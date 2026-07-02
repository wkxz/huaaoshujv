package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

type API struct {
	store     *Store
	scheduler *Scheduler
	cfg       *Config
}

func NewAPI(store *Store, scheduler *Scheduler, cfg *Config) *API {
	return &API{store: store, scheduler: scheduler, cfg: cfg}
}

func (a *API) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/status", a.handleStatus)
	mux.HandleFunc("/api/targets", a.handleTargets)
	mux.HandleFunc("/api/targets/", a.handleTargetByID)
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
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

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
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}

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

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
