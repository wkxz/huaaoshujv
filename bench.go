package main

import (
	"net/http"
	"sort"
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

type BenchReport struct {
	TotalRequests  int                `json:"total_requests"`
	SuccessCount   int                `json:"success_count"`
	FailCount      int                `json:"fail_count"`
	QPS            float64            `json:"qps"`
	AvgLatencyMs   float64            `json:"avg_latency_ms"`
	MinLatencyMs   float64            `json:"min_latency_ms"`
	MaxLatencyMs   float64            `json:"max_latency_ms"`
	P50LatencyMs   float64            `json:"p50_latency_ms"`
	P95LatencyMs   float64            `json:"p95_latency_ms"`
	P99LatencyMs   float64            `json:"p99_latency_ms"`
	StatusCodeDist map[int]int        `json:"status_code_dist"`
	DurationMs     float64            `json:"duration_ms"`
}

func CalcReport(records []BenchRecord, duration time.Duration) BenchReport {
	report := BenchReport{
		TotalRequests:  len(records),
		StatusCodeDist: make(map[int]int),
		DurationMs:     float64(duration.Milliseconds()),
	}

	if len(records) == 0 {
		return report
	}

	var latencies []float64
	var totalLatency float64

	for _, r := range records {
		if r.Error == "" && r.StatusCode >= 200 && r.StatusCode < 300 {
			report.SuccessCount++
		} else {
			report.FailCount++
		}
		if r.StatusCode > 0 {
			report.StatusCodeDist[r.StatusCode]++
		}
		latencies = append(latencies, r.LatencyMs)
		totalLatency += r.LatencyMs
	}

	report.AvgLatencyMs = totalLatency / float64(len(records))

	if duration.Seconds() > 0 {
		report.QPS = float64(len(records)) / duration.Seconds()
	}

	sort.Float64s(latencies)
	report.MinLatencyMs = latencies[0]
	report.MaxLatencyMs = latencies[len(latencies)-1]
	report.P50LatencyMs = percentile(latencies, 50)
	report.P95LatencyMs = percentile(latencies, 95)
	report.P99LatencyMs = percentile(latencies, 99)

	return report
}

func percentile(sorted []float64, p int) float64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (p * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}
