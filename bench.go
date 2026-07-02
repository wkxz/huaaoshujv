package main

import (
	"net/http"
	"sync"
	"time"
)

type BenchConfig struct {
	URL             string            `json:"url"`
	Method          string            `json:"method"`
	Headers         map[string]string `json:"headers,omitempty"`
	Concurrency     int               `json:"concurrency"`
	DurationSeconds int               `json:"duration_seconds"`
	TotalRequests   int               `json:"total_requests"`
}

type BenchRecord struct {
	StatusCode int           `json:"status_code"`
	Latency    time.Duration `json:"-"`
	LatencyMs  float64       `json:"latency_ms"`
	Error      string        `json:"error,omitempty"`
}

func RunBench(cfg BenchConfig) []BenchRecord {
	if cfg.Method == "" {
		cfg.Method = "GET"
	}
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 10
	}

	var records []BenchRecord
	var mu sync.Mutex
	var wg sync.WaitGroup

	client := &http.Client{Timeout: 30 * time.Second}

	done := make(chan struct{})
	if cfg.DurationSeconds > 0 {
		go func() {
			time.Sleep(time.Duration(cfg.DurationSeconds) * time.Second)
			close(done)
		}()
	}

	requestCount := make(chan struct{}, cfg.Concurrency)
	totalSent := 0
	var countMu sync.Mutex

	shouldStop := func() bool {
		select {
		case <-done:
			return true
		default:
		}
		if cfg.TotalRequests > 0 {
			countMu.Lock()
			defer countMu.Unlock()
			if totalSent >= cfg.TotalRequests {
				return true
			}
			totalSent++
		}
		return false
	}

	for i := 0; i < cfg.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if shouldStop() {
					return
				}
				requestCount <- struct{}{}

				record := sendRequest(client, cfg)

				mu.Lock()
				records = append(records, record)
				mu.Unlock()

				<-requestCount
			}
		}()
	}

	wg.Wait()
	return records
}

func sendRequest(client *http.Client, cfg BenchConfig) BenchRecord {
	req, err := http.NewRequest(cfg.Method, cfg.URL, nil)
	if err != nil {
		return BenchRecord{Error: err.Error()}
	}
	for k, v := range cfg.Headers {
		req.Header.Set(k, v)
	}

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start)

	if err != nil {
		return BenchRecord{
			Latency:   latency,
			LatencyMs: float64(latency.Microseconds()) / 1000.0,
			Error:     err.Error(),
		}
	}
	resp.Body.Close()

	return BenchRecord{
		StatusCode: resp.StatusCode,
		Latency:    latency,
		LatencyMs:  float64(latency.Microseconds()) / 1000.0,
	}
}
