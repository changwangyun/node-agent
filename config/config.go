package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type Config struct {
	mu sync.RWMutex

	NodeID   string `json:"node_id"`
	APIPort  int    `json:"api_port"`
	APIToken string `json:"api_token"`
	LogLevel string `json:"log_level"`
	DataDir  string `json:"data_dir"`

	PanelType string `json:"panel_type"`

	SingBox SingBoxConfig `json:"singbox"`

	ControlPlane ControlPlaneConfig `json:"control_plane"`

	Xboard XboardConfig `json:"xboard,omitempty"`

	DeviceLimit DeviceLimitConfig `json:"device_limit"`

	IPWhitelist []string `json:"ip_whitelist"`

	CORS CORSConfig `json:"cors"`

	HeartbeatInterval int `json:"heartbeat_interval"`

	WatchdogInterval int `json:"watchdog_interval"`
}

type SingBoxConfig struct {
	BinaryPath     string `json:"binary_path"`
	ConfigPath     string `json:"config_path"`
	WorkDir        string `json:"work_dir"`
	ClashAPIAddr   string `json:"clash_api_addr"`
	ClashAPISecret string `json:"clash_api_secret"`
	V2RayAPIAddr   string `json:"v2ray_api_addr"`
}

type ControlPlaneConfig struct {
	URL           string `json:"url"`
	Token         string `json:"token"`
	NodeID        string `json:"node_id"`
	Timeout       int    `json:"timeout"`
	HeartbeatPath string `json:"heartbeat_path,omitempty"`
}

type DeviceLimitConfig struct {
	MaxDevices    int `json:"max_devices"`
	MaxConcurrent int `json:"max_concurrent"`
}

type CORSConfig struct {
	Enabled        bool     `json:"enabled"`
	AllowedOrigins []string `json:"allowed_origins"`
}

type XboardConfig struct {
	APIHost          string `json:"api_host"`
	APIKey           string `json:"api_key"`
	NodeID           string `json:"node_id"`
	NodeType         string `json:"node_type"`
	SyncInterval     int    `json:"sync_interval"`
	Timeout          int    `json:"timeout"`
	DeviceLimit      int    `json:"device_limit"`
	NodeSecret       string `json:"node_secret"`
	RotationInterval int    `json:"rotation_interval"`
}

var (
	globalCfg *Config
)

func DefaultConfig() *Config {
	return &Config{
		NodeID:   "node-001",
		APIPort:  8080,
		APIToken: "change-me-in-production",
		LogLevel: "info",
		DataDir:  "/var/lib/node-agent",
		SingBox: SingBoxConfig{
			BinaryPath:     "/usr/local/bin/sing-box",
			ConfigPath:     "/etc/sing-box/config.json",
			WorkDir:        "/etc/sing-box",
			ClashAPIAddr:   "127.0.0.1:9090",
			ClashAPISecret: "node-agent-stats",
			V2RayAPIAddr:   "127.0.0.1:10001",
		},
		ControlPlane: ControlPlaneConfig{
			URL:     "http://127.0.0.1:8000",
			Token:   "",
			NodeID:  "node-001",
			Timeout: 10,
		},
		DeviceLimit: DeviceLimitConfig{
			MaxDevices:    3,
			MaxConcurrent: 5,
		},
		IPWhitelist: []string{},
		CORS: CORSConfig{
			Enabled:        true,
			AllowedOrigins: []string{"*"},
		},
		HeartbeatInterval: 10,
		WatchdogInterval:  5,
	}
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := DefaultConfig()
			if writeErr := cfg.Save(path); writeErr != nil {
				return nil, fmt.Errorf("create default config: %w", writeErr)
			}
			globalCfg = cfg
			return cfg, nil
		}
		return nil, fmt.Errorf("read config file: %w", err)
	}

	cfg := DefaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	if cfg.ControlPlane.Token == "" {
		cfg.ControlPlane.Token = cfg.APIToken
	}

	globalCfg = cfg
	return cfg, nil
}

func Get() *Config {
	if globalCfg == nil {
		globalCfg = DefaultConfig()
	}
	return globalCfg
}

func Set(cfg *Config) {
	globalCfg = cfg
}

func (c *Config) Save(path string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.ControlPlane.Token == "" {
		c.ControlPlane.Token = c.APIToken
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	dir := filepath.Dir(path)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write config file: %w", err)
	}
	return nil
}

func (c *Config) GetAPIAddr() string {
	return fmt.Sprintf(":%d", c.APIPort)
}

func (c *Config) GetControlPlaneURL() string {
	return c.ControlPlane.URL
}

func (c *Config) IsXboardMode() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.PanelType == "xboard" || c.Xboard.APIHost != ""
}

func (c *Config) IsIPWhitelisted(ip string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.IPWhitelist) == 0 {
		return true
	}
	for _, allowed := range c.IPWhitelist {
		if allowed == ip || allowed == "0.0.0.0/0" {
			return true
		}
	}
	return false
}
