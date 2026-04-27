package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.NodeID != "node-001" {
		t.Errorf("expected node-001, got %s", cfg.NodeID)
	}
	if cfg.APIPort != 8080 {
		t.Errorf("expected 8080, got %d", cfg.APIPort)
	}
	if cfg.HeartbeatInterval != 10 {
		t.Errorf("expected 10, got %d", cfg.HeartbeatInterval)
	}
	if cfg.DeviceLimit.MaxDevices != 3 {
		t.Errorf("expected 3, got %d", cfg.DeviceLimit.MaxDevices)
	}
	if cfg.DeviceLimit.MaxConcurrent != 5 {
		t.Errorf("expected 5, got %d", cfg.DeviceLimit.MaxConcurrent)
	}
}

func TestLoadAndSave(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.NodeID != "node-001" {
		t.Errorf("expected node-001, got %s", cfg.NodeID)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}

	var loaded Config
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("unmarshal config: %v", err)
	}

	if loaded.APIPort != cfg.APIPort {
		t.Errorf("expected %d, got %d", cfg.APIPort, loaded.APIPort)
	}
}

func TestLoadExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")

	customCfg := DefaultConfig()
	customCfg.NodeID = "custom-node"
	customCfg.APIPort = 9090
	customCfg.APIToken = "test-token-123"
	if err := customCfg.Save(cfgPath); err != nil {
		t.Fatalf("save config: %v", err)
	}

	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.NodeID != "custom-node" {
		t.Errorf("expected custom-node, got %s", cfg.NodeID)
	}
	if cfg.APIPort != 9090 {
		t.Errorf("expected 9090, got %d", cfg.APIPort)
	}
	if cfg.APIToken != "test-token-123" {
		t.Errorf("expected test-token-123, got %s", cfg.APIToken)
	}
}

func TestGetAPIAddr(t *testing.T) {
	cfg := DefaultConfig()
	cfg.APIPort = 3000
	addr := cfg.GetAPIAddr()
	if addr != ":3000" {
		t.Errorf("expected :3000, got %s", addr)
	}
}

func TestIsIPWhitelisted(t *testing.T) {
	cfg := DefaultConfig()

	if !cfg.IsIPWhitelisted("1.2.3.4") {
		t.Error("empty whitelist should allow all IPs")
	}

	cfg.IPWhitelist = []string{"10.0.0.1", "192.168.1.100"}
	if !cfg.IsIPWhitelisted("10.0.0.1") {
		t.Error("10.0.0.1 should be whitelisted")
	}
	if cfg.IsIPWhitelisted("8.8.8.8") {
		t.Error("8.8.8.8 should not be whitelisted")
	}

	cfg.IPWhitelist = []string{"0.0.0.0/0"}
	if !cfg.IsIPWhitelisted("1.2.3.4") {
		t.Error("0.0.0.0/0 should allow all IPs")
	}
}

func TestGet(t *testing.T) {
	globalCfg = nil
	cfg := Get()
	if cfg == nil {
		t.Fatal("Get() should not return nil")
	}
	if cfg.APIPort != 8080 {
		t.Errorf("expected default APIPort 8080, got %d", cfg.APIPort)
	}
}
