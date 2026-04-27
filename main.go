package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"node-agent/config"
	"node-agent/controller"
	"node-agent/core/configgen"
	"node-agent/core/device"
	"node-agent/core/heartbeat"
	"node-agent/core/singbox"
	"node-agent/core/stats"
	"node-agent/middleware"
)

func main() {
	configPath := flag.String("config", "/etc/node-agent/config.json", "path to config file")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[main] load config: %v", err)
	}

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Fatalf("[main] create data dir: %v", err)
	}

	mgr := singbox.NewManager(cfg)
	generator := configgen.NewGenerator(cfg.SingBox.ConfigPath)

	singboxCollector := stats.NewSingBoxStatsCollector("0.0.0.0:9090", "node-agent-stats")
	fallbackCollector := stats.NewFallbackCollector()
	multiCollector := stats.NewMultiCollector(singboxCollector, fallbackCollector)

	limiter := device.NewDeviceLimiter(&cfg.DeviceLimit)
	reporter := heartbeat.NewReporter(cfg, mgr, multiCollector, limiter)

	handler := controller.NewHandler(mgr, generator, multiCollector, limiter, reporter)

	mux := http.NewServeMux()

	mux.HandleFunc("/deploy", withMethods(handler.Deploy, http.MethodPost))
	mux.HandleFunc("/status", withMethods(handler.Status, http.MethodGet))
	mux.HandleFunc("/stats", withMethods(handler.GetStats, http.MethodGet))
	mux.HandleFunc("/heartbeat", withMethods(handler.Heartbeat, http.MethodGet))

	mux.HandleFunc("/restart", withMethods(handler.Restart, http.MethodPost))
	mux.HandleFunc("/stop", withMethods(handler.Stop, http.MethodPost))
	mux.HandleFunc("/start", withMethods(handler.Start, http.MethodPost))

	mux.HandleFunc("/device/register", withMethods(handler.RegisterDevice, http.MethodPost))
	mux.HandleFunc("/session/acquire", withMethods(handler.AcquireSession, http.MethodPost))
	mux.HandleFunc("/session/release", withMethods(handler.ReleaseSession, http.MethodPost))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	var h http.Handler = mux
	h = middleware.TokenAuth(cfg)(h)
	h = middleware.IPWhitelist(cfg)(h)
	h = middleware.Logging(h)
	h = middleware.Recovery(h)

	go startWatchdog(mgr, cfg)
	go func() {
		log.Printf("[heartbeat] starting reporter, interval=%ds", cfg.HeartbeatInterval)
		reporter.Start()
	}()

	cleanupTicker := time.NewTicker(5 * time.Minute)
	go func() {
		for range cleanupTicker.C {
			limiter.CleanupStale(30 * time.Minute)
		}
	}()

	server := &http.Server{
		Addr:         cfg.GetAPIAddr(),
		Handler:      h,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		log.Printf("[main] node-agent starting on %s", cfg.GetAPIAddr())
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[main] http server error: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	log.Printf("[main] received signal: %v, shutting down...", sig)

	reporter.Stop()
	cleanupTicker.Stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("[main] server shutdown error: %v", err)
	}

	if mgr.IsRunning() {
		if err := mgr.Stop(); err != nil {
			log.Printf("[main] stop sing-box error: %v", err)
		}
	}

	log.Println("[main] node-agent stopped")
}

func startWatchdog(mgr *singbox.Manager, cfg *config.Config) {
	interval := time.Duration(cfg.WatchdogInterval) * time.Second
	if interval < 1*time.Second {
		interval = 5 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		if !mgr.IsRunning() {
			state := mgr.GetState()
			if state == singbox.StateCrashed {
				log.Println("[watchdog] sing-box crashed, attempting restart...")
				if err := mgr.Start(); err != nil {
					log.Printf("[watchdog] restart failed: %v", err)
				} else {
					log.Println("[watchdog] sing-box restarted successfully")
				}
			}
		}
	}
}

func withMethods(h http.HandlerFunc, methods ...string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		for _, m := range methods {
			if r.Method == m {
				h(w, r)
				return
			}
		}
		w.Header().Set("Allow", fmt.Sprintf("%v", methods))
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
