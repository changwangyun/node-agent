package configgen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateHysteria2(t *testing.T) {
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

	if err := gen.GenerateAndWrite(req); err != nil {
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

	if len(cfg.Inbounds) == 0 {
		t.Fatal("expected at least 1 inbound")
	}
	if cfg.Inbounds[0].Type != "tun" {
		t.Errorf("expected tun inbound, got %s", cfg.Inbounds[0].Type)
	}

	found := false
	for _, ob := range cfg.Outbounds {
		if ob.Type == "hysteria2" {
			found = true
			if ob.Server != "hk1.example.com" {
				t.Errorf("expected hk1.example.com, got %s", ob.Server)
			}
			if ob.ServerPort != 443 {
				t.Errorf("expected 443, got %d", ob.ServerPort)
			}
			if ob.Password != "test-password" {
				t.Errorf("expected test-password, got %s", ob.Password)
			}
			if ob.TLS == nil || !ob.TLS.Enabled {
				t.Error("TLS should be enabled for hysteria2")
			}
		}
	}
	if !found {
		t.Fatal("hysteria2 outbound not found")
	}
}

func TestGenerateVLESS(t *testing.T) {
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

	if err := gen.GenerateAndWrite(req); err != nil {
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

	found := false
	for _, ob := range cfg.Outbounds {
		if ob.Type == "vless" {
			found = true
			if ob.UUID != "a3482e88-686a-4a58-8126-99c9df64b7bf" {
				t.Errorf("expected uuid, got %s", ob.UUID)
			}
			if ob.Flow != "xtls-rprx-vision" {
				t.Errorf("expected xtls-rprx-vision, got %s", ob.Flow)
			}
		}
	}
	if !found {
		t.Fatal("vless outbound not found")
	}
}

func TestGenerateReality(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	gen := NewGenerator(cfgPath)

	req := &DeployRequest{
		UserID:          "user-003",
		NodeID:          "node-003",
		Protocol:        "reality",
		Server:          "us1.example.com",
		Port:            443,
		Password:        "reality-pass",
		UUID:            "a3482e88-686a-4a58-8126-99c9df64b7bf",
		SNI:             "www.microsoft.com",
		RealityPubKey:   "test-public-key",
		RealityShortID:  "6ba85179930d344f",
	}

	if err := gen.GenerateAndWrite(req); err != nil {
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

	found := false
	for _, ob := range cfg.Outbounds {
		if ob.Type == "vless" && ob.TLS != nil && ob.TLS.Reality != nil {
			found = true
			if !ob.TLS.Reality.Enabled {
				t.Error("Reality should be enabled")
			}
			if ob.TLS.Reality.PublicKey != "test-public-key" {
				t.Errorf("expected test-public-key, got %s", ob.TLS.Reality.PublicKey)
			}
		}
	}
	if !found {
		t.Fatal("reality outbound not found")
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

	err := gen.GenerateAndWrite(req)
	if err == nil {
		t.Fatal("expected error for unsupported protocol")
	}
}

func TestGenerateMixedInbound(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	gen := NewGenerator(cfgPath)

	req := &DeployRequest{
		UserID:      "user-005",
		NodeID:      "node-005",
		Protocol:    "hysteria2",
		Server:      "example.com",
		Port:        443,
		Password:    "pass",
		InboundType: "mixed",
	}

	if err := gen.GenerateAndWrite(req); err != nil {
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

	if cfg.Inbounds[0].Type != "mixed" {
		t.Errorf("expected mixed inbound, got %s", cfg.Inbounds[0].Type)
	}
	if cfg.Inbounds[0].ListenPort != 2080 {
		t.Errorf("expected 2080, got %d", cfg.Inbounds[0].ListenPort)
	}
}

func TestGetDeploys(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	gen := NewGenerator(cfgPath)

	req1 := &DeployRequest{UserID: "u1", NodeID: "n1", Protocol: "hysteria2", Server: "a.com", Port: 443, Password: "p1"}
	req2 := &DeployRequest{UserID: "u2", NodeID: "n2", Protocol: "vless", Server: "b.com", Port: 443, Password: "p2"}

	if err := gen.GenerateAndWrite(req1); err != nil {
		t.Fatalf("generate req1: %v", err)
	}
	if err := gen.GenerateAndWrite(req2); err != nil {
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
	if err := gen.GenerateAndWrite(req); err != nil {
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
	if err := gen.GenerateAndWrite(req); err != nil {
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
