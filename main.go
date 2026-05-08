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
	"node-agent/core/xboard"
	"node-agent/middleware"
)

var Version = "dev"

func main() {
	configPath := flag.String("config", "/etc/node-agent/config.json", "path to config file")
	showVersion := flag.Bool("version", false, "show version")
	flag.Parse()

	if *showVersion {
		fmt.Printf("node-agent %s\n", Version)
		os.Exit(0)
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("[main] load config: %v", err)
	}

	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		log.Fatalf("[main] create data dir: %v", err)
	}

	mgr := singbox.NewManager(cfg)
	generator := configgen.NewGenerator(cfg.SingBox.ConfigPath)
	generator.SetClashAPI(cfg.SingBox.ClashAPIAddr, cfg.SingBox.ClashAPISecret)
	generator.SetV2RayAPI(cfg.SingBox.V2RayAPIAddr)

	sbVer, err := mgr.GetVersion()
	if err != nil {
		log.Printf("[main] failed to get sing-box version: %v", err)
	} else if sbVer == "unknown" {
		log.Printf("[main] sing-box version: unknown (compiled without version info)")
		generator.SetSingboxVersion(sbVer)
	} else {
		log.Printf("[main] sing-box version: %s", sbVer)
		generator.SetSingboxVersion(sbVer)
	}

	singboxCollector := stats.NewSingBoxStatsCollector(cfg.SingBox.ClashAPIAddr, cfg.SingBox.ClashAPISecret)
	fallbackCollector := stats.NewFallbackCollector()
	multiCollector := stats.NewMultiCollector(singboxCollector, fallbackCollector)

	v2rayStatsCollector := stats.NewV2RayStatsCollector(cfg.SingBox.V2RayAPIAddr)
	multiCollector.SetV2RayStats(v2rayStatsCollector)

	limiter := device.NewDeviceLimiter(&cfg.DeviceLimit)

	var reporter *heartbeat.Reporter
	if !cfg.IsXboardMode() {
		reporter = heartbeat.NewReporter(cfg, mgr, multiCollector, limiter)
	}

	handler := controller.NewHandler(mgr, generator, multiCollector, v2rayStatsCollector, limiter, reporter)

	mux := http.NewServeMux()

	mux.HandleFunc("/deploy", withMethods(handler.Deploy, http.MethodPost))
	mux.HandleFunc("/deploy/remove", withMethods(handler.RemoveUser, http.MethodPost))
	mux.HandleFunc("/status", withMethods(handler.Status, http.MethodGet))
	mux.HandleFunc("/stats", withMethods(handler.GetStats, http.MethodGet))
	mux.HandleFunc("/heartbeat", withMethods(handler.Heartbeat, http.MethodGet))

	mux.HandleFunc("/restart", withMethods(handler.Restart, http.MethodPost))
	mux.HandleFunc("/stop", withMethods(handler.Stop, http.MethodPost))
	mux.HandleFunc("/start", withMethods(handler.Start, http.MethodPost))

	mux.HandleFunc("/device/register", withMethods(handler.RegisterDevice, http.MethodPost))
	mux.HandleFunc("/session/acquire", withMethods(handler.AcquireSession, http.MethodPost))
	mux.HandleFunc("/session/release", withMethods(handler.ReleaseSession, http.MethodPost))
	mux.HandleFunc("/logs", withMethods(handler.GetLogs, http.MethodGet))
	mux.HandleFunc("/traffic/user", withMethods(handler.GetUserTraffic, http.MethodGet))
	mux.HandleFunc("/client-config", withMethods(handler.ClientConfig, http.MethodGet))
	mux.HandleFunc("/online", withMethods(handler.GetOnlineUsers, http.MethodGet))

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	var h http.Handler = mux
	h = middleware.TokenAuth(cfg)(h)
	h = middleware.IPWhitelist(cfg)(h)
	h = middleware.CORS(cfg)(h)
	h = middleware.Logging(h)
	h = middleware.Recovery(h)

	go startWatchdog(mgr, cfg)

	var xboardSync *xboard.XboardSync
	if cfg.IsXboardMode() {
		xboardSync = xboard.NewXboardSync(cfg, mgr, generator, multiCollector, limiter)
		go func() {
			log.Printf("[xboard] starting sync, interval=%ds", cfg.Xboard.SyncInterval)
			xboardSync.Start()
		}()
	} else {
		go func() {
			log.Printf("[heartbeat] starting reporter, interval=%ds", cfg.HeartbeatInterval)
			reporter.Start()
		}()
	}

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
	if xboardSync != nil {
		xboardSync.Stop()
	}
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

	crashCount := 0
	for range ticker.C {
		if !mgr.IsRunning() && mgr.ConfigExists() {
			state := mgr.GetState()
			if state == singbox.StateCrashed {
				crashCount++
				log.Printf("[watchdog] sing-box crashed (crash #%d), attempting restart...", crashCount)

				errMsg := mgr.GetLastError()
				if crashCount >= 3 && containsUnsupportedField(errMsg) {
					log.Printf("[watchdog] sing-box version may be too old for current config features")
					log.Printf("[watchdog] please upgrade sing-box to >= v1.10.0: https://github.com/SagerNet/sing-box/releases")
					log.Printf("[watchdog] or run: curl -fsSL https://raw.githubusercontent.com/changwangyun/node-agent/main/deploy/install.sh | bash")
					crashCount = 0
					time.Sleep(30 * time.Second)
					continue
				}

				if err := mgr.Start(); err != nil {
					log.Printf("[watchdog] restart failed: %v", err)
				} else {
					log.Println("[watchdog] sing-box restarted successfully")
					crashCount = 0
				}
			}
		} else {
			crashCount = 0
		}
	}
}

func containsUnsupportedField(errMsg string) bool {
	return contains(errMsg, "initial_packet_size") ||
		contains(errMsg, "bbr_profile") ||
		contains(errMsg, "disable_path_mtu_discovery")
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
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
