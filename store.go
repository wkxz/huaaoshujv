package main

import (
	"encoding/json"
	"os"
	"sync"
)

const maxResultsPerTarget = 100

type Store struct {
	mu      sync.RWMutex
	results map[string][]ProbeResult
	path    string
}

func NewStore(path string) *Store {
	s := &Store{
		results: make(map[string][]ProbeResult),
		path:    path,
	}
	s.load()
	return s
}

func (s *Store) SaveResult(r ProbeResult) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.results[r.TargetID] = append(s.results[r.TargetID], r)
	if len(s.results[r.TargetID]) > maxResultsPerTarget {
		s.results[r.TargetID] = s.results[r.TargetID][len(s.results[r.TargetID])-maxResultsPerTarget:]
	}

	go s.persist()
}

func (s *Store) GetResults(targetID string) []ProbeResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := s.results[targetID]
	copied := make([]ProbeResult, len(results))
	copy(copied, results)
	return copied
}

func (s *Store) GetLatestResult(targetID string) *ProbeResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := s.results[targetID]
	if len(results) == 0 {
		return nil
	}
	r := results[len(results)-1]
	return &r
}

func (s *Store) GetAllLatest() map[string]*ProbeResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	latest := make(map[string]*ProbeResult)
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
	defer s.mu.RUnlock()

	data, err := json.MarshalIndent(s.results, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(s.path, data, 0644)
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
