package monitor

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"http-monitor/internal/alert"
	"http-monitor/internal/config"
	"http-monitor/internal/store"
)

func Probe(target config.Target) config.ProbeResult {
	result := config.ProbeResult{
		TargetID:  target.ID,
		CheckedAt: time.Now(),
	}

	client := &http.Client{
		Timeout: time.Duration(target.TimeoutSeconds) * time.Second,
	}

	req, err := http.NewRequest(target.Method, target.URL, nil)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	for k, v := range target.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(req)
	result.LatencyMs = time.Since(start).Milliseconds()

	if err != nil {
		result.Error = err.Error()
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.Success = resp.StatusCode >= 200 && resp.StatusCode < 300
	return result
}

type Scheduler struct {
	targets  map[string]config.Target
	stopChs  map[string]chan struct{}
	resultCh chan config.ProbeResult
	store    *store.Store
	alertMgr *alert.AlertManager
	mu       sync.Mutex
}

func NewScheduler(s *store.Store, am *alert.AlertManager) *Scheduler {
	return &Scheduler{
		targets:  make(map[string]config.Target),
		stopChs:  make(map[string]chan struct{}),
		resultCh: make(chan config.ProbeResult, 100),
		store:    s,
		alertMgr: am,
	}
}

func (s *Scheduler) Start(targets []config.Target) {
	for _, t := range targets {
		s.AddTarget(t)
	}
	go s.processResults()
}

func (s *Scheduler) AddTarget(t config.Target) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, exists := s.stopChs[t.ID]; exists {
		close(ch)
	}

	s.targets[t.ID] = t
	stopCh := make(chan struct{})
	s.stopChs[t.ID] = stopCh

	go s.runProbe(t, stopCh)
}

func (s *Scheduler) RemoveTarget(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, exists := s.stopChs[id]; exists {
		close(ch)
		delete(s.stopChs, id)
		delete(s.targets, id)
	}
}

func (s *Scheduler) runProbe(t config.Target, stopCh chan struct{}) {
	s.resultCh <- Probe(t)

	ticker := time.NewTicker(time.Duration(t.IntervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			s.resultCh <- Probe(t)
		}
	}
}

func (s *Scheduler) processResults() {
	for result := range s.resultCh {
		s.store.SaveResult(result)
		s.alertMgr.Check(result)

		status := "✓"
		if !result.Success {
			status = "✗"
		}
		fmt.Printf("[%s] %s %s %dms (HTTP %d) %s\n",
			result.CheckedAt.Format("15:04:05"),
			status,
			result.TargetID,
			result.LatencyMs,
			result.StatusCode,
			result.Error,
		)
	}
}
