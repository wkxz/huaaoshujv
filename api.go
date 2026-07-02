package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

type API struct {
	store      *Store
	scheduler  *Scheduler
	cfg        *Config
	mu         sync.Mutex
	benchTasks map[string]*BenchTask
	benchMu    sync.Mutex
}

type BenchTask struct {
	ID        string       `json:"id"`
	Config    BenchConfig  `json:"config"`
	Status    string       `json:"status"`
	Report    *BenchReport `json:"report,omitempty"`
	StartedAt time.Time   `json:"started_at"`
	EndedAt   *time.Time  `json:"ended_at,omitempty"`
	Error     string       `json:"error,omitempty"`
}

func NewAPI(store *Store, scheduler *Scheduler, cfg *Config) *API {
	return &API{
		store:      store,
		scheduler:  scheduler,
		cfg:        cfg,
		benchTasks: make(map[string]*BenchTask),
	}
}

func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/status", a.handleStatus)
	mux.HandleFunc("/api/targets", a.handleTargets)
	mux.HandleFunc("/api/targets/", a.handleTargetByID)
	mux.HandleFunc("/api/bench", a.handleBenchList)
	mux.HandleFunc("/api/bench/", a.handleBenchByID)
}

func (a *API) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	latest := a.store.GetAllLatest()
	total := len(a.cfg.Targets)
	healthy := 0
	for _, r := range latest {
		if r.Success {
			healthy++
		}
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"total":    total,
		"healthy":  healthy,
		"unhealthy": total - healthy,
	})
}

func (a *API) handleTargets(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listTargets(w, r)
	case http.MethodPost:
		a.addTarget(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (a *API) listTargets(w http.ResponseWriter, r *http.Request) {
	latest := a.store.GetAllLatest()

	type TargetStatus struct {
		Target
		Latest *ProbeResult `json:"latest_probe"`
	}

	var result []TargetStatus
	for _, t := range a.cfg.Targets {
		ts := TargetStatus{Target: t, Latest: latest[t.ID]}
		result = append(result, ts)
	}

	writeJSON(w, http.StatusOK, result)
}

func (a *API) handleTargetByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/targets/")
	parts := strings.Split(path, "/")

	if len(parts) == 2 && parts[1] == "history" {
		a.handleTargetHistory(w, r, parts[0])
		return
	}

	if len(parts) == 1 {
		a.handleTargetDetail(w, r, parts[0])
		return
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func (a *API) handleTargetDetail(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		a.getTarget(w, r, id)
	case http.MethodPut:
		a.updateTarget(w, r, id)
	case http.MethodDelete:
		a.deleteTarget(w, r, id)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (a *API) getTarget(w http.ResponseWriter, r *http.Request, id string) {
	for _, t := range a.cfg.Targets {
		if t.ID == id {
			latest := a.store.GetLatestResult(id)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"target":       t,
				"latest_probe": latest,
			})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "target not found"})
}

func (a *API) handleTargetHistory(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	results := a.store.GetResults(id)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"target_id": id,
		"count":     len(results),
		"history":   results,
	})
}

func (a *API) addTarget(w http.ResponseWriter, r *http.Request) {
	var t Target
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if t.ID == "" || t.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id and url are required"})
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for _, existing := range a.cfg.Targets {
		if existing.ID == t.ID {
			writeJSON(w, http.StatusConflict, map[string]string{"error": "target id already exists"})
			return
		}
	}

	if t.Method == "" {
		t.Method = "GET"
	}
	if t.IntervalSeconds <= 0 {
		t.IntervalSeconds = 30
	}
	if t.TimeoutSeconds <= 0 {
		t.TimeoutSeconds = 10
	}

	a.cfg.Targets = append(a.cfg.Targets, t)
	SaveConfig("config.json", a.cfg)
	a.scheduler.AddTarget(t)

	writeJSON(w, http.StatusCreated, t)
}

func (a *API) updateTarget(w http.ResponseWriter, r *http.Request, id string) {
	var update Target
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	for i, t := range a.cfg.Targets {
		if t.ID == id {
			update.ID = id
			if update.URL == "" {
				update.URL = t.URL
			}
			if update.Method == "" {
				update.Method = t.Method
			}
			if update.Name == "" {
				update.Name = t.Name
			}
			if update.IntervalSeconds <= 0 {
				update.IntervalSeconds = t.IntervalSeconds
			}
			if update.TimeoutSeconds <= 0 {
				update.TimeoutSeconds = t.TimeoutSeconds
			}
			a.cfg.Targets[i] = update
			SaveConfig("config.json", a.cfg)
			a.scheduler.AddTarget(update)
			writeJSON(w, http.StatusOK, update)
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "target not found"})
}

func (a *API) deleteTarget(w http.ResponseWriter, r *http.Request, id string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for i, t := range a.cfg.Targets {
		if t.ID == id {
			a.cfg.Targets = append(a.cfg.Targets[:i], a.cfg.Targets[i+1:]...)
			SaveConfig("config.json", a.cfg)
			a.scheduler.RemoveTarget(id)
			writeJSON(w, http.StatusOK, map[string]string{"message": "deleted"})
			return
		}
	}
	writeJSON(w, http.StatusNotFound, map[string]string{"error": "target not found"})
}

func (a *API) handleBenchList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		a.listBenchTasks(w, r)
	case http.MethodPost:
		a.createBenchTask(w, r)
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
	}
}

func (a *API) listBenchTasks(w http.ResponseWriter, r *http.Request) {
	a.benchMu.Lock()
	defer a.benchMu.Unlock()

	tasks := make([]*BenchTask, 0, len(a.benchTasks))
	for _, t := range a.benchTasks {
		tasks = append(tasks, t)
	}
	writeJSON(w, http.StatusOK, tasks)
}

func (a *API) createBenchTask(w http.ResponseWriter, r *http.Request) {
	var cfg BenchConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}
	if cfg.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}
	if !strings.HasPrefix(cfg.URL, "http://") && !strings.HasPrefix(cfg.URL, "https://") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url must start with http:// or https://"})
		return
	}
	if cfg.DurationSeconds <= 0 && cfg.TotalRequests <= 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "duration_seconds or total_requests is required"})
		return
	}
	if cfg.Concurrency > 500 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "concurrency must be <= 500"})
		return
	}
	if cfg.DurationSeconds > 300 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "duration_seconds must be <= 300"})
		return
	}

	taskID := fmt.Sprintf("bench-%d", time.Now().UnixMilli())
	task := &BenchTask{
		ID:        taskID,
		Config:    cfg,
		Status:    "running",
		StartedAt: time.Now(),
	}

	a.benchMu.Lock()
	a.benchTasks[taskID] = task
	a.benchMu.Unlock()

	go func() {
		start := time.Now()
		records := RunBench(cfg)
		duration := time.Since(start)

		report := CalcReport(records, duration)
		now := time.Now()

		a.benchMu.Lock()
		task.Status = "completed"
		task.Report = &report
		task.EndedAt = &now
		a.benchMu.Unlock()
	}()

	writeJSON(w, http.StatusCreated, task)
}

func (a *API) handleBenchByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/bench/")

	a.benchMu.Lock()
	task, ok := a.benchTasks[id]
	a.benchMu.Unlock()

	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "bench task not found"})
		return
	}

	writeJSON(w, http.StatusOK, task)
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
