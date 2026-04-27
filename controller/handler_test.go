package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"node-agent/config"
	"node-agent/core/configgen"
	"node-agent/core/device"
	"node-agent/core/heartbeat"
	"node-agent/core/singbox"
	"node-agent/core/stats"
	"node-agent/middleware"
)

func setupTestHandler(t *testing.T) (*Handler, *config.Config) {
	t.Helper()

	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	sbCfgPath := filepath.Join(tmpDir, "singbox-config.json")

	cfg := config.DefaultConfig()
	cfg.APIToken = "test-integration-token"
	cfg.DataDir = tmpDir
	cfg.SingBox.ConfigPath = sbCfgPath
	cfg.SingBox.BinaryPath = "/bin/echo"

	if err := cfg.Save(cfgPath); err != nil {
		t.Fatalf("save config: %v", err)
	}
	config.Set(cfg)

	mgr := singbox.NewManager(cfg)
	generator := configgen.NewGenerator(sbCfgPath)

	singboxCollector := stats.NewSingBoxStatsCollector("127.0.0.1:1", "")
	fallbackCollector := stats.NewFallbackCollector()
	multiCollector := stats.NewMultiCollector(singboxCollector, fallbackCollector)

	limiter := device.NewDeviceLimiter(&cfg.DeviceLimit)
	reporter := heartbeat.NewReporter(cfg, mgr, multiCollector, limiter)

	handler := NewHandler(mgr, generator, multiCollector, nil, limiter, reporter)

	return handler, cfg
}

func setupTestServer(t *testing.T, handler *Handler, cfg *config.Config) *httptest.Server {
	t.Helper()

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
		w.Write([]byte("ok"))
	})

	var h http.Handler = mux
	h = middleware.TokenAuth(cfg)(h)
	h = middleware.Logging(h)
	h = middleware.Recovery(h)

	return httptest.NewServer(h)
}

func TestIntegrationHealthEndpoint(t *testing.T) {
	handler, cfg := setupTestHandler(t)
	server := setupTestServer(t, handler, cfg)
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/health", nil)
	req.Header.Set("X-Node-Token", "test-integration-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestIntegrationStatusUnauthorized(t *testing.T) {
	handler, cfg := setupTestHandler(t)
	server := setupTestServer(t, handler, cfg)
	defer server.Close()

	resp, err := http.Get(server.URL + "/status")
	if err != nil {
		t.Fatalf("status request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", resp.StatusCode)
	}
}

func TestIntegrationStatusWithToken(t *testing.T) {
	handler, cfg := setupTestHandler(t)
	server := setupTestServer(t, handler, cfg)
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/status", nil)
	req.Header.Set("X-Node-Token", "test-integration-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("status request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with valid token, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	nodeInfo, ok := result["node"].(map[string]interface{})
	if !ok {
		t.Fatal("missing node info in response")
	}
	if _, ok := nodeInfo["singbox_running"]; !ok {
		t.Error("missing singbox_running in node info")
	}
	if _, ok := nodeInfo["singbox_state"]; !ok {
		t.Error("missing singbox_state in node info")
	}
}

func TestIntegrationStatsEndpoint(t *testing.T) {
	handler, cfg := setupTestHandler(t)
	server := setupTestServer(t, handler, cfg)
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/stats", nil)
	req.Header.Set("X-Node-Token", "test-integration-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("stats request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := result["traffic"]; !ok {
		t.Error("missing traffic in stats response")
	}
	if _, ok := result["connections"]; !ok {
		t.Error("missing connections in stats response")
	}
}

func TestIntegrationHeartbeatEndpoint(t *testing.T) {
	handler, cfg := setupTestHandler(t)
	server := setupTestServer(t, handler, cfg)
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/heartbeat", nil)
	req.Header.Set("X-Node-Token", "test-integration-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("heartbeat request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if _, ok := result["node_id"]; !ok {
		t.Error("missing node_id in heartbeat response")
	}
}

func TestIntegrationDeviceRegister(t *testing.T) {
	handler, cfg := setupTestHandler(t)
	server := setupTestServer(t, handler, cfg)
	defer server.Close()

	body, _ := json.Marshal(map[string]string{
		"user_id":   "user-001",
		"device_id": "device-A",
		"ip":        "10.0.0.1",
	})

	req, _ := http.NewRequest("POST", server.URL+"/device/register", bytes.NewReader(body))
	req.Header.Set("X-Node-Token", "test-integration-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("register device request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if allowed, ok := result["allowed"].(bool); !ok || !allowed {
		t.Error("device should be allowed")
	}
}

func TestIntegrationSessionAcquireRelease(t *testing.T) {
	handler, cfg := setupTestHandler(t)
	server := setupTestServer(t, handler, cfg)
	defer server.Close()

	acquireBody, _ := json.Marshal(map[string]string{"user_id": "user-001"})

	req, _ := http.NewRequest("POST", server.URL+"/session/acquire", bytes.NewReader(acquireBody))
	req.Header.Set("X-Node-Token", "test-integration-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("acquire session request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	releaseBody, _ := json.Marshal(map[string]string{"user_id": "user-001"})

	req, _ = http.NewRequest("POST", server.URL+"/session/release", bytes.NewReader(releaseBody))
	req.Header.Set("X-Node-Token", "test-integration-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("release session request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestIntegrationDeployInvalidBody(t *testing.T) {
	handler, cfg := setupTestHandler(t)
	server := setupTestServer(t, handler, cfg)
	defer server.Close()

	req, _ := http.NewRequest("POST", server.URL+"/deploy", bytes.NewReader([]byte("invalid json")))
	req.Header.Set("X-Node-Token", "test-integration-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("deploy request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid body, got %d", resp.StatusCode)
	}
}

func TestIntegrationDeployMissingFields(t *testing.T) {
	handler, cfg := setupTestHandler(t)
	server := setupTestServer(t, handler, cfg)
	defer server.Close()

	body, _ := json.Marshal(map[string]string{"user_id": "u1"})

	req, _ := http.NewRequest("POST", server.URL+"/deploy", bytes.NewReader(body))
	req.Header.Set("X-Node-Token", "test-integration-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("deploy request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for missing fields, got %d", resp.StatusCode)
	}
}

func TestIntegrationMethodNotAllowed(t *testing.T) {
	handler, cfg := setupTestHandler(t)
	server := setupTestServer(t, handler, cfg)
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL+"/deploy", nil)
	req.Header.Set("X-Node-Token", "test-integration-token")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", resp.StatusCode)
	}
}

func TestIntegrationTokenViaQuery(t *testing.T) {
	handler, cfg := setupTestHandler(t)
	server := setupTestServer(t, handler, cfg)
	defer server.Close()

	resp, err := http.Get(server.URL + "/health?token=test-integration-token")
	if err != nil {
		t.Fatalf("health request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with query token, got %d", resp.StatusCode)
	}
}

func TestIntegrationDeployWritesConfig(t *testing.T) {
	handler, cfg := setupTestHandler(t)
	server := setupTestServer(t, handler, cfg)
	defer server.Close()

	deployReq := map[string]interface{}{
		"user_id":  "user-deploy-test",
		"node_id":  "node-001",
		"protocol": "hysteria2",
		"server":   "hk1.example.com",
		"port":     443,
		"password": "test-pass",
		"sni":      "hk1.example.com",
	}
	body, _ := json.Marshal(deployReq)

	req, _ := http.NewRequest("POST", server.URL+"/deploy", bytes.NewReader(body))
	req.Header.Set("X-Node-Token", "test-integration-token")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("deploy request: %v", err)
	}
	defer resp.Body.Close()

	if _, err := os.Stat(cfg.SingBox.ConfigPath); os.IsNotExist(err) {
		t.Error("sing-box config file should be created after deploy")
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
