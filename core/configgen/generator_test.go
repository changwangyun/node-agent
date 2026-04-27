package configgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateHysteria2Inbound(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	gen := NewGenerator(cfgPath)

	req := &DeployRequest{
		UserID:   "user-001",
		NodeID:   "node-001",
		Protocol: "hysteria2",
		Server:   "hk1.example.com",
		Port:     443,
		Password: "test-password",
		SNI:      "hk1.example.com",
	}

	clientCfg, err := gen.GenerateAndWrite(req)
	if err != nil {
		t.Fatalf("generate and write: %v", err)
	}

	if clientCfg == nil {
		t.Fatal("expected client config result")
	}
	if clientCfg.Protocol != "hysteria2" {
		t.Errorf("expected protocol hysteria2, got %s", clientCfg.Protocol)
	}
	if clientCfg.URI == "" {
		t.Error("expected non-empty URI")
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var cfg SingBoxConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if len(cfg.Inbounds) == 0 {
		t.Fatal("expected at least 1 inbound")
	}
	if cfg.Inbounds[0].Type != "hysteria2" {
		t.Errorf("expected hysteria2 inbound, got %s", cfg.Inbounds[0].Type)
	}
	if cfg.Inbounds[0].Listen != "0.0.0.0" {
		t.Errorf("expected 0.0.0.0 listen, got %s", cfg.Inbounds[0].Listen)
	}
	if cfg.Inbounds[0].ListenPort != 443 {
		t.Errorf("expected port 443, got %d", cfg.Inbounds[0].ListenPort)
	}
	if len(cfg.Inbounds[0].Users) == 0 || cfg.Inbounds[0].Users[0].Password != "test-password" {
		t.Error("expected user with password test-password")
	}
	if cfg.Inbounds[0].TLS == nil || !cfg.Inbounds[0].TLS.Enabled {
		t.Error("TLS should be enabled for hysteria2")
	}
	if cfg.Inbounds[0].TLS.ServerName != "hk1.example.com" {
		t.Errorf("expected SNI hk1.example.com, got %s", cfg.Inbounds[0].TLS.ServerName)
	}

	found := false
	for _, ob := range cfg.Outbounds {
		if ob.Type == "direct" {
			found = true
		}
	}
	if !found {
		t.Fatal("direct outbound not found")
	}

	if cfg.Route.Final != "direct" {
		t.Errorf("expected route final=direct, got %s", cfg.Route.Final)
	}
}

func TestGenerateVLESSInbound(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	gen := NewGenerator(cfgPath)

	req := &DeployRequest{
		UserID:   "user-002",
		NodeID:   "node-002",
		Protocol: "vless",
		Server:   "jp1.example.com",
		Port:     443,
		Password: "vless-pass",
		UUID:     "a3482e88-686a-4a58-8126-99c9df64b7bf",
		SNI:      "jp1.example.com",
	}

	clientCfg, err := gen.GenerateAndWrite(req)
	if err != nil {
		t.Fatalf("generate and write: %v", err)
	}
	if clientCfg.Protocol != "vless" {
		t.Errorf("expected protocol vless, got %s", clientCfg.Protocol)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var cfg SingBoxConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if cfg.Inbounds[0].Type != "vless" {
		t.Errorf("expected vless inbound, got %s", cfg.Inbounds[0].Type)
	}
	if len(cfg.Inbounds[0].Users) == 0 || cfg.Inbounds[0].Users[0].UUID != "a3482e88-686a-4a58-8126-99c9df64b7bf" {
		t.Error("expected user with UUID")
	}
	if cfg.Inbounds[0].Users[0].Flow != "xtls-rprx-vision" {
		t.Errorf("expected xtls-rprx-vision flow, got %s", cfg.Inbounds[0].Users[0].Flow)
	}
	if cfg.Inbounds[0].TLS == nil || !cfg.Inbounds[0].TLS.Enabled {
		t.Error("TLS should be enabled for vless")
	}
}

func TestGenerateRealityInbound(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	gen := NewGenerator(cfgPath)

	req := &DeployRequest{
		UserID:            "user-003",
		NodeID:            "node-003",
		Protocol:          "reality",
		Server:            "us1.example.com",
		Port:              443,
		Password:          "reality-pass",
		UUID:              "a3482e88-686a-4a58-8126-99c9df64b7bf",
		SNI:               "www.microsoft.com",
		RealityPrivateKey: "test-private-key",
		RealityShortID:    "6ba85179930d344f",
		RealityDest:       "www.microsoft.com",
		RealityDestPort:   443,
	}

	clientCfg, err := gen.GenerateAndWrite(req)
	if err != nil {
		t.Fatalf("generate and write: %v", err)
	}
	if clientCfg.Protocol != "reality" {
		t.Errorf("expected protocol reality, got %s", clientCfg.Protocol)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var cfg SingBoxConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if cfg.Inbounds[0].Type != "vless" {
		t.Errorf("expected vless inbound, got %s", cfg.Inbounds[0].Type)
	}
	if cfg.Inbounds[0].TLS == nil || cfg.Inbounds[0].TLS.Reality == nil {
		t.Fatal("expected Reality TLS config")
	}
	if !cfg.Inbounds[0].TLS.Reality.Enabled {
		t.Error("Reality should be enabled")
	}
	if cfg.Inbounds[0].TLS.Reality.PrivateKey != "test-private-key" {
		t.Errorf("expected test-private-key, got %s", cfg.Inbounds[0].TLS.Reality.PrivateKey)
	}
	if cfg.Inbounds[0].TLS.Reality.Handshake == nil {
		t.Fatal("expected Reality handshake config")
	}
	if cfg.Inbounds[0].TLS.Reality.Handshake.Server != "www.microsoft.com" {
		t.Errorf("expected handshake server www.microsoft.com, got %s", cfg.Inbounds[0].TLS.Reality.Handshake.Server)
	}
}

func TestGenerateRealityMissingPrivateKey(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	gen := NewGenerator(cfgPath)

	req := &DeployRequest{
		UserID:   "user-003",
		NodeID:   "node-003",
		Protocol: "reality",
		Server:   "us1.example.com",
		Port:     443,
		Password: "reality-pass",
	}

	_, err := gen.GenerateAndWrite(req)
	if err == nil {
		t.Fatal("expected error for missing reality_private_key")
	}
}

func TestGenerateUnsupportedProtocol(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	gen := NewGenerator(cfgPath)

	req := &DeployRequest{
		UserID:   "user-004",
		NodeID:   "node-004",
		Protocol: "unsupported",
		Server:   "example.com",
		Port:     443,
		Password: "pass",
	}

	_, err := gen.GenerateAndWrite(req)
	if err == nil {
		t.Fatal("expected error for unsupported protocol")
	}
}

func TestGenerateHysteria2WithTLS(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	gen := NewGenerator(cfgPath)

	req := &DeployRequest{
		UserID:      "user-006",
		NodeID:      "node-006",
		Protocol:    "hysteria2",
		Server:      "example.com",
		Port:        443,
		Password:    "pass",
		SNI:         "example.com",
		TLSCertPath: "/etc/ssl/cert.pem",
		TLSKeyPath:  "/etc/ssl/key.pem",
		UpMbps:      100,
		DownMbps:    200,
	}

	_, err := gen.GenerateAndWrite(req)
	if err != nil {
		t.Fatalf("generate and write: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	var cfg SingBoxConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if cfg.Inbounds[0].TLS.CertificatePath != "/etc/ssl/cert.pem" {
		t.Errorf("expected cert path, got %s", cfg.Inbounds[0].TLS.CertificatePath)
	}
	if cfg.Inbounds[0].TLS.KeyPath != "/etc/ssl/key.pem" {
		t.Errorf("expected key path, got %s", cfg.Inbounds[0].TLS.KeyPath)
	}
	if cfg.Inbounds[0].UpMbps != 100 {
		t.Errorf("expected up_mbps 100, got %d", cfg.Inbounds[0].UpMbps)
	}
	if cfg.Inbounds[0].DownMbps != 200 {
		t.Errorf("expected down_mbps 200, got %d", cfg.Inbounds[0].DownMbps)
	}
}

func TestGetDeploys(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	gen := NewGenerator(cfgPath)

	req1 := &DeployRequest{UserID: "u1", NodeID: "n1", Protocol: "hysteria2", Server: "a.com", Port: 443, Password: "p1"}
	req2 := &DeployRequest{UserID: "u2", NodeID: "n2", Protocol: "vless", Server: "b.com", Port: 443, Password: "p2", UUID: "test-uuid"}

	if _, err := gen.GenerateAndWrite(req1); err != nil {
		t.Fatalf("generate req1: %v", err)
	}
	if _, err := gen.GenerateAndWrite(req2); err != nil {
		t.Fatalf("generate req2: %v", err)
	}

	deploys := gen.GetDeploys()
	if len(deploys) != 2 {
		t.Errorf("expected 2 deploys, got %d", len(deploys))
	}
	if _, ok := deploys["u1"]; !ok {
		t.Error("expected u1 deploy")
	}
	if _, ok := deploys["u2"]; !ok {
		t.Error("expected u2 deploy")
	}
}

func TestRemoveDeploy(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	gen := NewGenerator(cfgPath)

	req := &DeployRequest{UserID: "u1", NodeID: "n1", Protocol: "hysteria2", Server: "a.com", Port: 443, Password: "p1"}
	if _, err := gen.GenerateAndWrite(req); err != nil {
		t.Fatalf("generate: %v", err)
	}

	if err := gen.RemoveDeploy("u1"); err != nil {
		t.Fatalf("remove deploy: %v", err)
	}

	deploys := gen.GetDeploys()
	if len(deploys) != 0 {
		t.Errorf("expected 0 deploys after removal, got %d", len(deploys))
	}
}

func TestAtomicWrite(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	gen := NewGenerator(cfgPath)

	req := &DeployRequest{UserID: "u1", NodeID: "n1", Protocol: "hysteria2", Server: "a.com", Port: 443, Password: "p1"}
	if _, err := gen.GenerateAndWrite(req); err != nil {
		t.Fatalf("generate: %v", err)
	}

	tmpFile := cfgPath + ".tmp"
	if _, err := os.Stat(tmpFile); !os.IsNotExist(err) {
		t.Error("temp file should be cleaned up after atomic write")
	}

	if _, err := os.Stat(cfgPath); err != nil {
		t.Errorf("config file should exist: %v", err)
	}
}

func TestClientConfigResult(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	gen := NewGenerator(cfgPath)

	req := &DeployRequest{
		UserID:   "user-001",
		NodeID:   "node-001",
		Protocol: "hysteria2",
		Server:   "1.2.3.4",
		Port:     443,
		Password: "test-password",
		SNI:      "example.com",
	}

	clientCfg, err := gen.GenerateAndWrite(req)
	if err != nil {
		t.Fatalf("generate and write: %v", err)
	}

	if clientCfg.URI == "" {
		t.Error("expected non-empty URI")
	}
	if clientCfg.SingBoxConfig == "" {
		t.Error("expected non-empty sing-box client config")
	}
	if clientCfg.Server != "1.2.3.4" {
		t.Errorf("expected server 1.2.3.4, got %s", clientCfg.Server)
	}
	if clientCfg.Port != 443 {
		t.Errorf("expected port 443, got %d", clientCfg.Port)
	}

	var clientJSON SingBoxConfig
	if err := json.Unmarshal([]byte(clientCfg.SingBoxConfig), &clientJSON); err != nil {
		t.Fatalf("unmarshal client config: %v", err)
	}
	if len(clientJSON.Inbounds) == 0 || clientJSON.Inbounds[0].Type != "tun" {
		t.Error("expected tun inbound in client config")
	}
	if len(clientJSON.Outbounds) < 1 {
		t.Error("expected at least 1 outbound in client config")
	}
	if clientJSON.Route.Final != "proxy" {
		t.Errorf("expected client route final=proxy, got %s", clientJSON.Route.Final)
	}
}

func TestGetClientConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	gen := NewGenerator(cfgPath)

	req := &DeployRequest{
		UserID:   "user-001",
		NodeID:   "node-001",
		Protocol: "hysteria2",
		Server:   "1.2.3.4",
		Port:     443,
		Password: "test-password",
		SNI:      "example.com",
	}

	if _, err := gen.GenerateAndWrite(req); err != nil {
		t.Fatalf("generate and write: %v", err)
	}

	clientCfg, err := gen.GetClientConfig("user-001")
	if err != nil {
		t.Fatalf("get client config: %v", err)
	}
	if clientCfg.URI == "" {
		t.Error("expected non-empty URI from GetClientConfig")
	}

	_, err = gen.GetClientConfig("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent user")
	}
}
