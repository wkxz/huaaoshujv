package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type AlertManager struct {
	webhookURL    string
	threshold     int
	cooldown      time.Duration
	failCounts    map[string]int
	alertedAt     map[string]time.Time
	wasDown       map[string]bool
	mu            sync.Mutex
}

func NewAlertManager(webhookURL string, threshold int, cooldownMinutes int) *AlertManager {
	return &AlertManager{
		webhookURL: webhookURL,
		threshold:  threshold,
		cooldown:   time.Duration(cooldownMinutes) * time.Minute,
		failCounts: make(map[string]int),
		alertedAt:  make(map[string]time.Time),
		wasDown:    make(map[string]bool),
	}
}

func (am *AlertManager) Check(result ProbeResult) {
	am.mu.Lock()
	defer am.mu.Unlock()

	if result.Success {
		if am.wasDown[result.TargetID] {
			am.sendRecovery(result)
			am.wasDown[result.TargetID] = false
		}
		am.failCounts[result.TargetID] = 0
		return
	}

	am.failCounts[result.TargetID]++

	if am.failCounts[result.TargetID] >= am.threshold {
		if last, ok := am.alertedAt[result.TargetID]; ok && time.Since(last) < am.cooldown {
			return
		}
		am.sendAlert(result)
		am.alertedAt[result.TargetID] = time.Now()
		am.wasDown[result.TargetID] = true
	}
}

type WebhookPayload struct {
	Type      string `json:"type"`
	TargetID  string `json:"target_id"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Details   string `json:"details,omitempty"`
}

func (am *AlertManager) sendAlert(result ProbeResult) {
	payload := WebhookPayload{
		Type:      "alert",
		TargetID:  result.TargetID,
		Message:   fmt.Sprintf("目标 %s 连续 %d 次探测失败", result.TargetID, am.threshold),
		Timestamp: time.Now().Format(time.RFC3339),
		Details:   fmt.Sprintf("status_code=%d, error=%s", result.StatusCode, result.Error),
	}
	am.send(payload)
	fmt.Printf("[ALERT] %s 连续 %d 次失败，已触发告警\n", result.TargetID, am.threshold)
}

func (am *AlertManager) sendRecovery(result ProbeResult) {
	payload := WebhookPayload{
		Type:      "recovery",
		TargetID:  result.TargetID,
		Message:   fmt.Sprintf("目标 %s 已恢复正常", result.TargetID),
		Timestamp: time.Now().Format(time.RFC3339),
	}
	am.send(payload)
	fmt.Printf("[RECOVERY] %s 已恢复正常\n", result.TargetID)
}

func (am *AlertManager) send(payload WebhookPayload) {
	if am.webhookURL == "" {
		return
	}
	data, _ := json.Marshal(payload)
	go http.Post(am.webhookURL, "application/json", bytes.NewReader(data))
}
