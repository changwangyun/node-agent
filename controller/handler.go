package controller

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"node-agent/core/configgen"
	"node-agent/core/device"
	"node-agent/core/heartbeat"
	"node-agent/core/singbox"
	"node-agent/core/stats"
	"node-agent/utils"
)

type Handler struct {
	mgr        *singbox.Manager
	generator  *configgen.Generator
	collector  stats.Collector
	v2rayStats *stats.V2RayStatsCollector
	limiter    *device.DeviceLimiter
	reporter   *heartbeat.Reporter
}

func NewHandler(
	mgr *singbox.Manager,
	generator *configgen.Generator,
	collector stats.Collector,
	v2rayStats *stats.V2RayStatsCollector,
	limiter *device.DeviceLimiter,
	reporter *heartbeat.Reporter,
) *Handler {
	return &Handler{
		mgr:        mgr,
		generator:  generator,
		collector:  collector,
		v2rayStats: v2rayStats,
		limiter:    limiter,
		reporter:   reporter,
	}
}

func (h *Handler) Deploy(w http.ResponseWriter, r *http.Request) {
	var req configgen.DeployRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	if req.UserID == "" || req.NodeID == "" || req.Protocol == "" {
		writeError(w, http.StatusBadRequest, "missing required fields: user_id, node_id, protocol")
		return
	}

	if req.Port == 0 {
		writeError(w, http.StatusBadRequest, "missing required field: port")
		return
	}

	if req.Server == "" {
		req.Server = "0.0.0.0"
	}

	clientCfg, err := h.generator.GenerateAndWrite(&req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "generate config failed: "+err.Error())
		return
	}

	if h.mgr.IsRunning() {
		if err := h.mgr.Restart(); err != nil {
			writeError(w, http.StatusInternalServerError, "restart sing-box failed: "+err.Error())
			return
		}
	} else {
		if err := h.mgr.Start(); err != nil {
			writeError(w, http.StatusInternalServerError, "start sing-box failed: "+err.Error())
			return
		}
	}

	response := map[string]interface{}{
		"success":       true,
		"message":       fmt.Sprintf("deployed %s config for user %s", req.Protocol, req.UserID),
		"node_id":       req.NodeID,
		"client_config": clientCfg,
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) ClientConfig(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing query parameter: user_id")
		return
	}

	clientCfg, err := h.generator.GetClientConfig(userID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, clientCfg)
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) {
	isRunning := h.mgr.IsRunning()
	state := h.mgr.GetState()
	uptime := h.mgr.GetUptime()
	pid := h.mgr.GetPID()
	lastError := h.mgr.GetLastError()
	crashTime := h.mgr.GetCrashTime()

	cpuPercent, _ := utils.GetCPUUsage()
	memPercent, memUsed, memTotal := utils.GetMemoryUsage()
	diskUsed, diskTotal := utils.GetDiskUsage()
	load1, load5, load15 := utils.GetLoadAvg()

	statsResult, _ := h.collector.GetStats()

	var activeConns int
	var trafficData stats.TrafficData
	var speedData stats.SpeedData
	var onlineDetails []*stats.OnlineUser
	var onlineUserCount int
	if statsResult != nil {
		activeConns = statsResult.Connections.ActiveConnections
		trafficData = statsResult.Traffic
		speedData = statsResult.Speed
		if len(statsResult.OnlineUsers) > 0 {
			onlineDetails = statsResult.OnlineUsers
			onlineUserCount = len(statsResult.OnlineUsers)
		}
	}

	nodeInfo := map[string]interface{}{
		"singbox_running": isRunning,
		"singbox_state":   state.String(),
		"uptime_seconds":  int64(uptime.Seconds()),
		"pid":             pid,
	}
	if lastError != "" {
		nodeInfo["last_error"] = lastError
		if !crashTime.IsZero() {
			nodeInfo["crash_time"] = crashTime.Format("2006-01-02T15:04:05Z07:00")
		}
	}

	response := map[string]interface{}{
		"node": nodeInfo,
		"system": map[string]interface{}{
			"cpu_percent":   cpuPercent,
			"mem_percent":   memPercent,
			"mem_used_mb":   memUsed,
			"mem_total_mb":  memTotal,
			"disk_used_gb":  diskUsed,
			"disk_total_gb": diskTotal,
			"load_1":        load1,
			"load_5":        load5,
			"load_15":       load15,
		},
		"traffic": map[string]interface{}{
			"upload":   trafficData.Upload,
			"download": trafficData.Download,
		},
		"speed": map[string]interface{}{
			"upload":   speedData.Upload,
			"download": speedData.Download,
		},
		"connections": map[string]interface{}{
			"active": activeConns,
		},
		"devices": map[string]interface{}{
			"online_users":  h.limiter.GetOnlineUserCount(),
			"total_devices": h.limiter.GetTotalDeviceCount(),
		},
	}

	if onlineUserCount > 0 {
		response["online_details"] = onlineDetails
		response["devices"].(map[string]interface{})["online_users"] = onlineUserCount
	} else if mc, ok := h.collector.(*stats.MultiCollector); ok {
		if users, err := mc.GetOnlineUsers(); err == nil && len(users) > 0 {
			response["online_details"] = users
			response["devices"].(map[string]interface{})["online_users"] = len(users)
		}
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	result, err := h.collector.GetStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "get stats failed: "+err.Error())
		return
	}

	response := map[string]interface{}{
		"traffic": map[string]interface{}{
			"upload":   result.Traffic.Upload,
			"download": result.Traffic.Download,
		},
		"speed": map[string]interface{}{
			"upload":   result.Speed.Upload,
			"download": result.Speed.Download,
		},
		"connections": map[string]interface{}{
			"active": result.Connections.ActiveConnections,
		},
	}

	if len(result.OnlineUsers) > 0 {
		response["online_users"] = len(result.OnlineUsers)
		response["online_details"] = result.OnlineUsers
	} else if mc, ok := h.collector.(*stats.MultiCollector); ok {
		if users, err := mc.GetOnlineUsers(); err == nil && len(users) > 0 {
			response["online_users"] = len(users)
			response["online_details"] = users
		}
	}

	if h.v2rayStats != nil && h.v2rayStats.IsEnabled() {
		if allTraffic, err := h.v2rayStats.GetAllUserTraffic(); err == nil {
			response["user_traffic"] = allTraffic
		}
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) GetOnlineUsers(w http.ResponseWriter, r *http.Request) {
	var onlineUsers []*stats.OnlineUser

	if mc, ok := h.collector.(*stats.MultiCollector); ok {
		if users, err := mc.GetOnlineUsers(); err == nil {
			onlineUsers = users
		}
	} else if sc, ok := h.collector.(*stats.SingBoxStatsCollector); ok {
		if users, err := sc.GetOnlineUsers(); err == nil {
			onlineUsers = users
		}
	}

	response := map[string]interface{}{
		"count":        len(onlineUsers),
		"online_users": onlineUsers,
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Handler) Heartbeat(w http.ResponseWriter, r *http.Request) {
	payload := h.reporter.GetPayload()
	writeJSON(w, http.StatusOK, payload)
}

func (h *Handler) Restart(w http.ResponseWriter, r *http.Request) {
	if err := h.mgr.Restart(); err != nil {
		writeError(w, http.StatusInternalServerError, "restart failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "sing-box restarted",
	})
}

func (h *Handler) Stop(w http.ResponseWriter, r *http.Request) {
	if err := h.mgr.Stop(); err != nil {
		writeError(w, http.StatusInternalServerError, "stop failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "sing-box stopped",
	})
}

func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	if err := h.mgr.Start(); err != nil {
		writeError(w, http.StatusInternalServerError, "start failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
		"message": "sing-box started",
	})
}

func (h *Handler) RegisterDevice(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   string `json:"user_id"`
		DeviceID string `json:"device_id"`
		IP       string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.limiter.RegisterDevice(req.UserID, req.DeviceID, req.IP)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{
			"allowed": false,
			"reason":  result.String(),
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"allowed":      true,
		"reason":       result.String(),
		"device_count": h.limiter.GetDeviceCount(req.UserID),
	})
}

func (h *Handler) AcquireSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.limiter.AcquireSession(req.UserID)
	if err != nil {
		writeJSON(w, http.StatusForbidden, map[string]interface{}{
			"allowed": false,
			"reason":  result.String(),
			"error":   err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"allowed":          true,
		"reason":           result.String(),
		"concurrent_count": h.limiter.GetConcurrentCount(req.UserID),
	})
}

func (h *Handler) ReleaseSession(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	h.limiter.ReleaseSession(req.UserID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

func (h *Handler) GetLogs(w http.ResponseWriter, r *http.Request) {
	n := 50
	if v := r.URL.Query().Get("lines"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil && parsed > 0 && parsed <= 200 {
			n = parsed
		}
	}

	logs := h.mgr.GetLogs(n)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"lines": logs,
		"count": len(logs),
	})
}

func (h *Handler) GetUserTraffic(w http.ResponseWriter, r *http.Request) {
	if h.v2rayStats == nil {
		writeError(w, http.StatusServiceUnavailable, "v2ray api not available: sing-box needs to be built with -tags with_v2ray_api")
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID != "" {
		ut, err := h.v2rayStats.GetUserTraffic(userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "query user traffic failed: "+err.Error())
			return
		}
		writeJSON(w, http.StatusOK, ut)
		return
	}

	all, err := h.v2rayStats.GetAllUserTraffic()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "query all user traffic failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"users": all,
		"count": len(all),
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]interface{}{
		"error": msg,
	})
}
