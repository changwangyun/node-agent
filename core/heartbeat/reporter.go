package heartbeat

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"node-agent/config"
	"node-agent/core/device"
	"node-agent/core/singbox"
	"node-agent/core/stats"
	"node-agent/utils"
)

type HeartbeatPayload struct {
	NodeID    string        `json:"node_id"`
	Timestamp int64         `json:"timestamp"`
	Status    NodeStatus    `json:"status"`
	Traffic   TrafficReport `json:"traffic"`
	Online    OnlineReport  `json:"online"`
	System    SystemReport  `json:"system"`
}

type NodeStatus struct {
	SingBoxRunning bool   `json:"singbox_running"`
	SingBoxState   string `json:"singbox_state"`
	Uptime         int64  `json:"uptime_seconds"`
	PID            int    `json:"pid"`
}

type TrafficReport struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
}

type OnlineReport struct {
	UserCount      int `json:"user_count"`
	DeviceCount    int `json:"device_count"`
	ActiveSessions int `json:"active_sessions"`
}

type SystemReport struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemPercent float64 `json:"mem_percent"`
	MemUsed    uint64  `json:"mem_used_mb"`
	MemTotal   uint64  `json:"mem_total_mb"`
	DiskUsed   uint64  `json:"disk_used_gb"`
	DiskTotal  uint64  `json:"disk_total_gb"`
	Load1      float64 `json:"load_1"`
	Load5      float64 `json:"load_5"`
	Load15     float64 `json:"load_15"`
}

type Reporter struct {
	mu        sync.RWMutex
	cfg       *config.Config
	mgr       *singbox.Manager
	collector stats.Collector
	limiter   *device.DeviceLimiter
	client    *http.Client
	stopCh    chan struct{}
	running   bool
}

func NewReporter(
	cfg *config.Config,
	mgr *singbox.Manager,
	collector stats.Collector,
	limiter *device.DeviceLimiter,
) *Reporter {
	return &Reporter{
		cfg:       cfg,
		mgr:       mgr,
		collector: collector,
		limiter:   limiter,
		client: &http.Client{
			Timeout: time.Duration(cfg.ControlPlane.Timeout) * time.Second,
		},
		stopCh: make(chan struct{}),
	}
}

func (r *Reporter) Start() {
	r.mu.Lock()
	if r.running {
		r.mu.Unlock()
		return
	}
	r.running = true
	r.mu.Unlock()

	interval := time.Duration(r.cfg.HeartbeatInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	r.sendHeartbeat()

	for {
		select {
		case <-ticker.C:
			r.sendHeartbeat()
		case <-r.stopCh:
			return
		}
	}
}

func (r *Reporter) Stop() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.running {
		close(r.stopCh)
		r.running = false
		r.stopCh = make(chan struct{})
	}
}

func (r *Reporter) sendHeartbeat() {
	payload := r.buildPayload()

	data, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[heartbeat] marshal payload error: %v", err)
		return
	}

	heartbeatPath := r.cfg.ControlPlane.HeartbeatPath
	if heartbeatPath == "" {
		heartbeatPath = "/api/v1/node/heartbeat"
	}
	url := fmt.Sprintf("%s%s", r.cfg.GetControlPlaneURL(), heartbeatPath)
	req, err := http.NewRequest("POST", url, bytes.NewReader(data))
	if err != nil {
		log.Printf("[heartbeat] create request error: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Node-Token", r.cfg.ControlPlane.Token)
	req.Header.Set("X-Node-ID", r.cfg.ControlPlane.NodeID)

	resp, err := r.client.Do(req)
	if err != nil {
		log.Printf("[heartbeat] send error: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("[heartbeat] server returned status: %d", resp.StatusCode)
	}
}

func (r *Reporter) buildPayload() *HeartbeatPayload {
	isRunning := r.mgr.IsRunning()
	state := r.mgr.GetState()
	uptime := r.mgr.GetUptime()
	pid := r.mgr.GetPID()

	var trafficReport TrafficReport
	var onlineUserCount int
	statsResult, err := r.collector.GetStats()
	if err == nil && statsResult != nil {
		trafficReport = TrafficReport{
			Upload:   statsResult.Traffic.Upload,
			Download: statsResult.Traffic.Download,
		}
		if len(statsResult.OnlineUsers) > 0 {
			onlineUserCount = len(statsResult.OnlineUsers)
		}
	}

	onlineReport := OnlineReport{
		UserCount:      r.limiter.GetOnlineUserCount(),
		DeviceCount:    r.limiter.GetTotalDeviceCount(),
		ActiveSessions: r.limiter.GetOnlineUserCount(),
	}

	if onlineUserCount > 0 {
		onlineReport.UserCount = onlineUserCount
	}

	cpuPercent, _ := utils.GetCPUUsage()
	memPercent, memUsed, memTotal := utils.GetMemoryUsage()
	diskUsed, diskTotal := utils.GetDiskUsage()
	load1, load5, load15 := utils.GetLoadAvg()

	return &HeartbeatPayload{
		NodeID:    r.cfg.NodeID,
		Timestamp: time.Now().Unix(),
		Status: NodeStatus{
			SingBoxRunning: isRunning,
			SingBoxState:   state.String(),
			Uptime:         int64(uptime.Seconds()),
			PID:            pid,
		},
		Traffic: trafficReport,
		Online:  onlineReport,
		System: SystemReport{
			CPUPercent: cpuPercent,
			MemPercent: memPercent,
			MemUsed:    memUsed,
			MemTotal:   memTotal,
			DiskUsed:   diskUsed,
			DiskTotal:  diskTotal,
			Load1:      load1,
			Load5:      load5,
			Load15:     load15,
		},
	}
}

func (r *Reporter) GetPayload() *HeartbeatPayload {
	return r.buildPayload()
}
