package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	cfg, err := LoadConfig("config.json")
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	fmt.Printf("已加载 %d 个监控目标:\n", len(cfg.Targets))
	for _, t := range cfg.Targets {
		fmt.Printf("  - [%s] %s (%s %s, 间隔%ds)\n", t.ID, t.Name, t.Method, t.URL, t.IntervalSeconds)
	}

	store := NewStore("data.json")
	scheduler := NewScheduler(store)
	scheduler.Start(cfg.Targets)

	mux := http.NewServeMux()

	api := NewAPI(store, scheduler, cfg)
	api.RegisterRoutes(mux)

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}`))
	})

	addr := ":8080"
	fmt.Printf("HTTP Monitor started on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, mux))
}
