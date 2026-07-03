package config

import (
	"encoding/json"
	"fmt"
	"os"
	"time"
)

type Target struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	URL             string            `json:"url"`
	Method          string            `json:"method"`
	IntervalSeconds int               `json:"interval_seconds"`
	TimeoutSeconds  int               `json:"timeout_seconds"`
	Headers         map[string]string `json:"headers,omitempty"`
}

type Config struct {
	Targets []Target    `json:"targets"`
	Alert   AlertConfig `json:"alert"`
}

type AlertConfig struct {
	WebhookURL      string `json:"webhook_url"`
	Threshold       int    `json:"threshold"`
	CooldownMinutes int    `json:"cooldown_minutes"`
}

type ProbeResult struct {
	TargetID   string    `json:"target_id"`
	StatusCode int       `json:"status_code"`
	LatencyMs  int64     `json:"latency_ms"`
	Success    bool      `json:"success"`
	Error      string    `json:"error,omitempty"`
	CheckedAt  time.Time `json:"checked_at"`
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	for i := range cfg.Targets {
		if cfg.Targets[i].Method == "" {
			cfg.Targets[i].Method = "GET"
		}
		if cfg.Targets[i].IntervalSeconds <= 0 {
			cfg.Targets[i].IntervalSeconds = 30
		}
		if cfg.Targets[i].TimeoutSeconds <= 0 {
			cfg.Targets[i].TimeoutSeconds = 10
		}
	}
	return &cfg, nil
}

func SaveConfig(path string, cfg *Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(path, data, 0644)
}
