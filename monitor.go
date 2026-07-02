package main

import (
	"net/http"
	"time"
)

type ProbeResult struct {
	TargetID   string    `json:"target_id"`
	StatusCode int       `json:"status_code"`
	LatencyMs  int64     `json:"latency_ms"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	CheckedAt  time.Time `json:"checked_at"`
}

func Probe(target Target) ProbeResult {
	result := ProbeResult{
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
