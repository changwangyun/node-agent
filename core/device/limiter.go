package device

import (
	"fmt"
	"sync"
	"time"

	"node-agent/config"
)

type DeviceInfo struct {
	DeviceID  string    `json:"device_id"`
	UserID    string    `json:"user_id"`
	IP        string    `json:"ip"`
	ConnectedAt time.Time `json:"connected_at"`
	LastSeen  time.Time `json:"last_seen"`
}

type DeviceLimiter struct {
	mu       sync.RWMutex
	cfg      *config.DeviceLimitConfig
	devices  map[string]map[string]*DeviceInfo
	sessions map[string]int
}

func NewDeviceLimiter(cfg *config.DeviceLimitConfig) *DeviceLimiter {
	return &DeviceLimiter{
		cfg:      cfg,
		devices:  make(map[string]map[string]*DeviceInfo),
		sessions: make(map[string]int),
	}
}

type RegisterResult int

const (
	RegisterAllowed RegisterResult = iota
	RegisterDeviceLimitExceeded
	RegisterConcurrentLimitExceeded
)

func (r RegisterResult) String() string {
	switch r {
	case RegisterAllowed:
		return "allowed"
	case RegisterDeviceLimitExceeded:
		return "device_limit_exceeded"
	case RegisterConcurrentLimitExceeded:
		return "concurrent_limit_exceeded"
	default:
		return "unknown"
	}
}

func (d *DeviceLimiter) RegisterDevice(userID, deviceID, ip string) (RegisterResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.devices[userID] == nil {
		d.devices[userID] = make(map[string]*DeviceInfo)
	}

	userDevices := d.devices[userID]

	if existing, ok := userDevices[deviceID]; ok {
		existing.LastSeen = time.Now()
		existing.IP = ip
		return RegisterAllowed, nil
	}

	if d.cfg.MaxDevices > 0 && len(userDevices) >= d.cfg.MaxDevices {
		return RegisterDeviceLimitExceeded, fmt.Errorf(
			"user %s has %d devices, limit is %d",
			userID, len(userDevices), d.cfg.MaxDevices,
		)
	}

	userDevices[deviceID] = &DeviceInfo{
		DeviceID:    deviceID,
		UserID:      userID,
		IP:          ip,
		ConnectedAt: time.Now(),
		LastSeen:    time.Now(),
	}

	return RegisterAllowed, nil
}

func (d *DeviceLimiter) AcquireSession(userID string) (RegisterResult, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	current := d.sessions[userID]
	if d.cfg.MaxConcurrent > 0 && current >= d.cfg.MaxConcurrent {
		return RegisterConcurrentLimitExceeded, fmt.Errorf(
			"user %s has %d concurrent sessions, limit is %d",
			userID, current, d.cfg.MaxConcurrent,
		)
	}

	d.sessions[userID] = current + 1
	return RegisterAllowed, nil
}

func (d *DeviceLimiter) ReleaseSession(userID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.sessions[userID] > 0 {
		d.sessions[userID]--
	}
	if d.sessions[userID] == 0 {
		delete(d.sessions, userID)
	}
}

func (d *DeviceLimiter) RemoveDevice(userID, deviceID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.devices[userID] != nil {
		delete(d.devices[userID], deviceID)
		if len(d.devices[userID]) == 0 {
			delete(d.devices, userID)
		}
	}
}

func (d *DeviceLimiter) GetDeviceCount(userID string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.devices[userID] == nil {
		return 0
	}
	return len(d.devices[userID])
}

func (d *DeviceLimiter) GetConcurrentCount(userID string) int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.sessions[userID]
}

func (d *DeviceLimiter) GetUserDevices(userID string) []*DeviceInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.devices[userID] == nil {
		return nil
	}

	result := make([]*DeviceInfo, 0, len(d.devices[userID]))
	for _, dev := range d.devices[userID] {
		result = append(result, dev)
	}
	return result
}

func (d *DeviceLimiter) GetOnlineUserCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.sessions)
}

func (d *DeviceLimiter) GetTotalDeviceCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()

	total := 0
	for _, userDevs := range d.devices {
		total += len(userDevs)
	}
	return total
}

func (d *DeviceLimiter) CleanupStale(timeout time.Duration) {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now()
	for userID, userDevs := range d.devices {
		for deviceID, dev := range userDevs {
			if now.Sub(dev.LastSeen) > timeout {
				delete(userDevs, deviceID)
			}
		}
		if len(userDevs) == 0 {
			delete(d.devices, userID)
		}
	}
}

func (d *DeviceLimiter) UpdateConfig(cfg *config.DeviceLimitConfig) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.cfg = cfg
}
