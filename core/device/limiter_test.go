package device

import (
	"testing"
	"time"

	"node-agent/config"
)

func TestNewDeviceLimiter(t *testing.T) {
	cfg := &config.DeviceLimitConfig{MaxDevices: 3, MaxConcurrent: 5}
	limiter := NewDeviceLimiter(cfg)
	if limiter == nil {
		t.Fatal("limiter should not be nil")
	}
}

func TestRegisterDevice(t *testing.T) {
	cfg := &config.DeviceLimitConfig{MaxDevices: 2, MaxConcurrent: 5}
	limiter := NewDeviceLimiter(cfg)

	result, err := limiter.RegisterDevice("user-1", "device-A", "10.0.0.1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != RegisterAllowed {
		t.Errorf("expected RegisterAllowed, got %s", result)
	}

	result, err = limiter.RegisterDevice("user-1", "device-B", "10.0.0.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != RegisterAllowed {
		t.Errorf("expected RegisterAllowed, got %s", result)
	}

	result, err = limiter.RegisterDevice("user-1", "device-C", "10.0.0.3")
	if err == nil {
		t.Fatal("expected error for exceeding device limit")
	}
	if result != RegisterDeviceLimitExceeded {
		t.Errorf("expected RegisterDeviceLimitExceeded, got %s", result)
	}
}

func TestRegisterSameDevice(t *testing.T) {
	cfg := &config.DeviceLimitConfig{MaxDevices: 1, MaxConcurrent: 5}
	limiter := NewDeviceLimiter(cfg)

	result, _ := limiter.RegisterDevice("user-1", "device-A", "10.0.0.1")
	if result != RegisterAllowed {
		t.Errorf("first registration should be allowed")
	}

	result, err := limiter.RegisterDevice("user-1", "device-A", "10.0.0.2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != RegisterAllowed {
		t.Errorf("same device re-registration should be allowed")
	}
}

func TestDifferentUsersIndependent(t *testing.T) {
	cfg := &config.DeviceLimitConfig{MaxDevices: 1, MaxConcurrent: 5}
	limiter := NewDeviceLimiter(cfg)

	result1, _ := limiter.RegisterDevice("user-1", "device-A", "10.0.0.1")
	result2, _ := limiter.RegisterDevice("user-2", "device-X", "10.0.0.2")

	if result1 != RegisterAllowed || result2 != RegisterAllowed {
		t.Error("different users should have independent device limits")
	}
}

func TestAcquireAndReleaseSession(t *testing.T) {
	cfg := &config.DeviceLimitConfig{MaxDevices: 5, MaxConcurrent: 3}
	limiter := NewDeviceLimiter(cfg)

	for i := 0; i < 3; i++ {
		result, err := limiter.AcquireSession("user-1")
		if err != nil {
			t.Fatalf("session %d: unexpected error: %v", i+1, err)
		}
		if result != RegisterAllowed {
			t.Errorf("session %d should be allowed", i+1)
		}
	}

	result, err := limiter.AcquireSession("user-1")
	if err == nil {
		t.Fatal("expected error for exceeding concurrent limit")
	}
	if result != RegisterConcurrentLimitExceeded {
		t.Errorf("expected RegisterConcurrentLimitExceeded, got %s", result)
	}

	limiter.ReleaseSession("user-1")

	result, err = limiter.AcquireSession("user-1")
	if err != nil {
		t.Fatalf("session after release: unexpected error: %v", err)
	}
	if result != RegisterAllowed {
		t.Errorf("session after release should be allowed")
	}
}

func TestReleaseSessionNonExistent(t *testing.T) {
	cfg := &config.DeviceLimitConfig{MaxDevices: 5, MaxConcurrent: 5}
	limiter := NewDeviceLimiter(cfg)
	limiter.ReleaseSession("nonexistent")
}

func TestGetDeviceCount(t *testing.T) {
	cfg := &config.DeviceLimitConfig{MaxDevices: 5, MaxConcurrent: 5}
	limiter := NewDeviceLimiter(cfg)

	if limiter.GetDeviceCount("user-1") != 0 {
		t.Error("expected 0 devices initially")
	}

	limiter.RegisterDevice("user-1", "device-A", "10.0.0.1")
	if limiter.GetDeviceCount("user-1") != 1 {
		t.Errorf("expected 1 device, got %d", limiter.GetDeviceCount("user-1"))
	}

	limiter.RegisterDevice("user-1", "device-B", "10.0.0.2")
	if limiter.GetDeviceCount("user-1") != 2 {
		t.Errorf("expected 2 devices, got %d", limiter.GetDeviceCount("user-1"))
	}
}

func TestGetConcurrentCount(t *testing.T) {
	cfg := &config.DeviceLimitConfig{MaxDevices: 5, MaxConcurrent: 5}
	limiter := NewDeviceLimiter(cfg)

	if limiter.GetConcurrentCount("user-1") != 0 {
		t.Error("expected 0 sessions initially")
	}

	limiter.AcquireSession("user-1")
	limiter.AcquireSession("user-1")
	if limiter.GetConcurrentCount("user-1") != 2 {
		t.Errorf("expected 2 sessions, got %d", limiter.GetConcurrentCount("user-1"))
	}
}

func TestRemoveDevice(t *testing.T) {
	cfg := &config.DeviceLimitConfig{MaxDevices: 5, MaxConcurrent: 5}
	limiter := NewDeviceLimiter(cfg)

	limiter.RegisterDevice("user-1", "device-A", "10.0.0.1")
	limiter.RegisterDevice("user-1", "device-B", "10.0.0.2")

	limiter.RemoveDevice("user-1", "device-A")
	if limiter.GetDeviceCount("user-1") != 1 {
		t.Errorf("expected 1 device after removal, got %d", limiter.GetDeviceCount("user-1"))
	}
}

func TestGetOnlineUserCount(t *testing.T) {
	cfg := &config.DeviceLimitConfig{MaxDevices: 5, MaxConcurrent: 5}
	limiter := NewDeviceLimiter(cfg)

	if limiter.GetOnlineUserCount() != 0 {
		t.Error("expected 0 online users initially")
	}

	limiter.AcquireSession("user-1")
	limiter.AcquireSession("user-2")
	if limiter.GetOnlineUserCount() != 2 {
		t.Errorf("expected 2 online users, got %d", limiter.GetOnlineUserCount())
	}
}

func TestGetTotalDeviceCount(t *testing.T) {
	cfg := &config.DeviceLimitConfig{MaxDevices: 5, MaxConcurrent: 5}
	limiter := NewDeviceLimiter(cfg)

	limiter.RegisterDevice("user-1", "device-A", "10.0.0.1")
	limiter.RegisterDevice("user-2", "device-X", "10.0.0.2")

	if limiter.GetTotalDeviceCount() != 2 {
		t.Errorf("expected 2 total devices, got %d", limiter.GetTotalDeviceCount())
	}
}

func TestCleanupStale(t *testing.T) {
	cfg := &config.DeviceLimitConfig{MaxDevices: 5, MaxConcurrent: 5}
	limiter := NewDeviceLimiter(cfg)

	limiter.RegisterDevice("user-1", "device-A", "10.0.0.1")

	limiter.mu.Lock()
	for _, dev := range limiter.devices["user-1"] {
		dev.LastSeen = time.Now().Add(-2 * time.Hour)
	}
	limiter.mu.Unlock()

	limiter.CleanupStale(1 * time.Hour)

	if limiter.GetDeviceCount("user-1") != 0 {
		t.Error("stale devices should be cleaned up")
	}
}

func TestUpdateConfig(t *testing.T) {
	cfg := &config.DeviceLimitConfig{MaxDevices: 5, MaxConcurrent: 5}
	limiter := NewDeviceLimiter(cfg)

	newCfg := &config.DeviceLimitConfig{MaxDevices: 1, MaxConcurrent: 1}
	limiter.UpdateConfig(newCfg)

	result, err := limiter.RegisterDevice("user-1", "device-A", "10.0.0.1")
	if result != RegisterAllowed {
		t.Error("first device should be allowed with new config")
	}

	result, err = limiter.RegisterDevice("user-1", "device-B", "10.0.0.2")
	if err == nil {
		t.Error("second device should exceed new limit")
	}
}

func TestGetUserDevices(t *testing.T) {
	cfg := &config.DeviceLimitConfig{MaxDevices: 5, MaxConcurrent: 5}
	limiter := NewDeviceLimiter(cfg)

	limiter.RegisterDevice("user-1", "device-A", "10.0.0.1")
	limiter.RegisterDevice("user-1", "device-B", "10.0.0.2")

	devices := limiter.GetUserDevices("user-1")
	if len(devices) != 2 {
		t.Errorf("expected 2 devices, got %d", len(devices))
	}

	devices = limiter.GetUserDevices("nonexistent")
	if devices != nil {
		t.Error("expected nil for nonexistent user")
	}
}
