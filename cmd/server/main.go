package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"http-monitor/internal/alert"
	"http-monitor/internal/api"
	"http-monitor/internal/config"
	"http-monitor/internal/monitor"
	"http-monitor/internal/store"
)

func main() {
	cfgPath := "config.json"
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	fmt.Printf("已加载 %d 个监控目标:\n", len(cfg.Targets))
	for _, t := range cfg.Targets {
		fmt.Printf("  - [%s] %s (%s %s, 间隔%ds)\n", t.ID, t.Name, t.Method, t.URL, t.IntervalSeconds)
	}

	s := store.NewStore("data.json")

	threshold := cfg.Alert.Threshold
	if threshold <= 0 {
		threshold = 3
	}
	cooldown := cfg.Alert.CooldownMinutes
	if cooldown <= 0 {
		cooldown = 5
	}
	alertMgr := alert.NewAlertManager(cfg.Alert.WebhookURL, threshold, cooldown)

	scheduler := monitor.NewScheduler(s, alertMgr)
	scheduler.Start(cfg.Targets)

	mux := http.NewServeMux()

	a := api.NewAPI(s, scheduler, cfg, cfgPath)
	a.RegisterRoutes(mux)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	handler := loggingMiddleware(mux)

	addr := ":8080"
	server := &http.Server{Addr: addr, Handler: handler}

	go func() {
		fmt.Printf("HTTP Monitor started on %s\n", addr)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n正在关闭服务...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	server.Shutdown(ctx)
	s.ForceSync()
	fmt.Println("服务已关闭")
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.URL.Path == "/health" {
			start := time.Now()
			next.ServeHTTP(w, r)
			log.Printf("%s %s %v", r.Method, r.URL.Path, time.Since(start))
		} else {
			next.ServeHTTP(w, r)
		}
	})
}
