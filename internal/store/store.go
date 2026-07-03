package store

import (
	"encoding/json"
	"os"
	"sync"
	"time"

	"http-monitor/internal/config"
)

const maxResultsPerTarget = 100

type Store struct {
	mu      sync.RWMutex
	results map[string][]config.ProbeResult
	path    string
	saveCh  chan struct{}
}

func NewStore(path string) *Store {
	s := &Store{
		results: make(map[string][]config.ProbeResult),
		path:    path,
		saveCh:  make(chan struct{}, 1),
	}
	s.load()
	go s.persistLoop()
	return s
}

func (s *Store) SaveResult(r config.ProbeResult) {
	s.mu.Lock()
	s.results[r.TargetID] = append(s.results[r.TargetID], r)
	if len(s.results[r.TargetID]) > maxResultsPerTarget {
		s.results[r.TargetID] = s.results[r.TargetID][len(s.results[r.TargetID])-maxResultsPerTarget:]
	}
	s.mu.Unlock()

	select {
	case s.saveCh <- struct{}{}:
	default:
	}
}

func (s *Store) persistLoop() {
	for range s.saveCh {
		time.Sleep(2 * time.Second)
		for {
			select {
			case <-s.saveCh:
			default:
				goto flush
			}
		}
	flush:
		s.persist()
	}
}

func (s *Store) GetResults(targetID string) []config.ProbeResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := s.results[targetID]
	copied := make([]config.ProbeResult, len(results))
	copy(copied, results)
	return copied
}

func (s *Store) GetLatestResult(targetID string) *config.ProbeResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := s.results[targetID]
	if len(results) == 0 {
		return nil
	}
	r := results[len(results)-1]
	return &r
}

func (s *Store) GetAllLatest() map[string]*config.ProbeResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	latest := make(map[string]*config.ProbeResult)
	for id, results := range s.results {
		if len(results) > 0 {
			r := results[len(results)-1]
			latest[id] = &r
		}
	}
	return latest
}

func (s *Store) persist() {
	s.mu.RLock()
	data, err := json.MarshalIndent(s.results, "", "  ")
	s.mu.RUnlock()

	if err != nil {
		return
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return
	}
	os.Rename(tmp, s.path)
}

func (s *Store) load() {
	data, err := os.ReadFile(s.path)
	if err != nil {
		return
	}
	json.Unmarshal(data, &s.results)
}

func (s *Store) ForceSync() {
	s.persist()
}
