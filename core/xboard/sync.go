package xboard

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"node-agent/config"
	"node-agent/core/configgen"
	"node-agent/core/device"
	"node-agent/core/singbox"
	"node-agent/core/stats"
	"node-agent/utils"

	"github.com/shirou/gopsutil/v3/mem"
)

type FlexibleInt int

func (f *FlexibleInt) UnmarshalJSON(data []byte) error {
	if len(data) >= 2 && (data[0] == '"' || data[0] == '\'') {
		var s string
		if err := json.Unmarshal(data, &s); err == nil {
			if v, err := strconv.Atoi(s); err == nil {
				*f = FlexibleInt(v)
				return nil
			}
		}
	}
	var v int
	if err := json.Unmarshal(data, &v); err == nil {
		*f = FlexibleInt(v)
	}
	return nil
}

type XboardSync struct {
	cfg       *config.Config
	mgr       *singbox.Manager
	generator *configgen.Generator
	collector stats.Collector
	limiter   *device.DeviceLimiter
	client    *XboardClient
	tracker   *Tracker
	wsClient  *WSClient

	stopCh chan struct{}
	mu     sync.Mutex

	lastUserMap   map[int]*UserInfo
	lastNodeInfo  *NodeInfo
	lastAliveUIDs map[int]bool

	wsConnected bool
}

func NewXboardSync(
	cfg *config.Config,
	mgr *singbox.Manager,
	generator *configgen.Generator,
	collector stats.Collector,
	limiter *device.DeviceLimiter,
) *XboardSync {
	xcfg := cfg.Xboard
	client := NewXboardClient(xcfg.APIHost, xcfg.APIKey, xcfg.NodeID.String(), xcfg.NodeType, xcfg.Timeout)
	wsClient := NewWSClient(xcfg.APIHost, xcfg.APIKey, xcfg.NodeID.String(), xcfg.NodeType)

	s := &XboardSync{
		cfg:         cfg,
		mgr:         mgr,
		generator:   generator,
		collector:   collector,
		limiter:     limiter,
		client:      client,
		tracker:     NewTracker(),
		wsClient:    wsClient,
		stopCh:      make(chan struct{}),
		lastUserMap: make(map[int]*UserInfo),
	}

	wsClient.SetOnConfigUpdate(s.onWSConfigUpdate)
	wsClient.SetOnDisconnected(s.onWSDisconnected)
	wsClient.SetOnMetrics(s.collectMetrics)

	return s
}

func (s *XboardSync) Start() {
	interval := time.Duration(s.cfg.Xboard.SyncInterval) * time.Second
	if interval < 10*time.Second {
		interval = 60 * time.Second
	}

	s.syncOnce()

	s.wsClient.Connect()

	wsDiscoveryInterval := 5 * time.Minute
	ticker := time.NewTicker(interval)
	wsDiscoveryTicker := time.NewTicker(wsDiscoveryInterval)
	defer ticker.Stop()
	defer wsDiscoveryTicker.Stop()

	for {
		select {
		case <-ticker.C:
			if s.wsClient.IsConnected() {
				s.reportAlive()
				s.reportTraffic()
				s.reportNodeStatus()
				continue
			}
			s.syncOnce()
		case <-wsDiscoveryTicker.C:
			s.wsDiscovery()
		case <-s.stopCh:
			return
		}
	}
}

func (s *XboardSync) Stop() {
	close(s.stopCh)
	s.wsClient.Stop()
}

func (s *XboardSync) onWSConfigUpdate(nodeInfo *NodeInfo, users []UserInfo) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.wsConnected = true

	if nodeInfo != nil {
		s.deployNode(nodeInfo, users)
	} else if len(users) > 0 && s.lastNodeInfo != nil {
		s.deployNode(s.lastNodeInfo, users)
	}

	log.Printf("[xboard] ws config update received")
}

func (s *XboardSync) onWSDisconnected() {
	s.mu.Lock()
	s.wsConnected = false
	s.mu.Unlock()
	log.Printf("[xboard] ws disconnected, falling back to REST polling")
}

func (s *XboardSync) syncOnce() {
	s.mu.Lock()
	defer s.mu.Unlock()

	nodeInfo, err := s.client.GetNodeInfo()
	if err != nil {
		if err.Error() != "not modified" {
			log.Printf("[xboard] failed to get node info: %v", err)
		}
		nodeInfo = s.lastNodeInfo
	}

	users, err := s.client.GetUserList()
	if err != nil {
		if err.Error() != "not modified" {
			log.Printf("[xboard] failed to get user list: %v", err)
		}
		return
	}

	if nodeInfo != nil {
		s.deployNode(nodeInfo, users)
	}

	s.reportAlive()
	s.reportTraffic()
	s.reportNodeStatus()
}

func (s *XboardSync) deployNode(nodeInfo *NodeInfo, users []UserInfo) {
	// If port is missing (0), sing-box will bind a random port, which breaks panel expectations.
	if nodeInfo == nil || int(nodeInfo.Port) <= 0 {
		log.Printf("[xboard] ERROR: invalid/missing node port in config (port=%v). Refusing to deploy to avoid random listen_port.", func() interface{} {
			if nodeInfo == nil {
				return nil
			}
			return int(nodeInfo.Port)
		}())
		return
	}

	currentUserMap := make(map[int]*UserInfo)
	for i := range users {
		currentUserMap[users[i].ID] = &users[i]
	}

	configChanged := s.isNodeConfigChanged(nodeInfo)

	if configChanged || s.lastNodeInfo == nil {
		s.fullDeploy(nodeInfo, users)
	} else {
		s.incrementalDeploy(nodeInfo, users)
	}

	s.lastUserMap = currentUserMap
	s.lastNodeInfo = nodeInfo
}

func (s *XboardSync) fullDeploy(nodeInfo *NodeInfo, users []UserInfo) {
	var reqs []configgen.DeployRequest
	for _, u := range users {
		req := s.buildDeployRequest(u, nodeInfo)
		reqs = append(reqs, *req)
	}

	if err := s.generator.SetUsers(reqs); err != nil {
		log.Printf("[xboard] failed to set users: %v", err)
		return
	}

	s.reloadKernel()
	s.updateLimiter(users)
	log.Printf("[xboard] full deploy: %d users", len(users))
}

func (s *XboardSync) incrementalDeploy(nodeInfo *NodeInfo, users []UserInfo) {
	toAdd, toRemove := s.computeUserDiff(users)
	if len(toAdd) == 0 && len(toRemove) == 0 {
		return
	}

	if len(toRemove) > 0 {
		removeReqs := make([]configgen.DeployRequest, 0, len(toRemove))
		for _, u := range toRemove {
			removeReqs = append(removeReqs, configgen.DeployRequest{UserID: u.UUID})
		}
		if err := s.generator.RemoveUsers(removeReqs); err != nil {
			log.Printf("[xboard] failed to remove users: %v", err)
			s.fullDeploy(nodeInfo, users)
			return
		}
		log.Printf("[xboard] removed %d users", len(toRemove))
	}

	if len(toAdd) > 0 {
		addReqs := make([]configgen.DeployRequest, 0, len(toAdd))
		for _, u := range toAdd {
			req := s.buildDeployRequest(*u, nodeInfo)
			addReqs = append(addReqs, *req)
		}
		if err := s.generator.AddUsers(addReqs); err != nil {
			log.Printf("[xboard] failed to add users: %v", err)
			s.fullDeploy(nodeInfo, users)
			return
		}
		log.Printf("[xboard] added %d users", len(toAdd))
	}

	s.reloadKernel()
	s.updateLimiter(users)
}

func (s *XboardSync) computeUserDiff(newUsers []UserInfo) (toAdd, toRemove []*UserInfo) {
	newMap := make(map[string]*UserInfo, len(newUsers))
	for i := range newUsers {
		newMap[newUsers[i].UUID] = &newUsers[i]
	}

	deployed := s.generator.GetDeployRequests()

	for uuid := range deployed {
		if _, exists := newMap[uuid]; !exists {
			for _, u := range s.lastUserMap {
				if u.UUID == uuid {
					toRemove = append(toRemove, u)
					break
				}
			}
		}
	}

	for uuid, u := range newMap {
		existing, exists := deployed[uuid]
		if !exists {
			toAdd = append(toAdd, u)
			continue
		}
		if existing.DeviceLimit != u.DeviceLimit {
			toAdd = append(toAdd, u)
			continue
		}
		if int64(existing.UpMbps*1000000) != int64(u.SpeedLimit) && int64(existing.DownMbps*1000000) != int64(u.SpeedLimit) {
			toAdd = append(toAdd, u)
			continue
		}
	}

	return
}

func (s *XboardSync) reloadKernel() {
	if s.mgr.IsRunning() {
		if err := s.mgr.Reload(); err != nil {
			log.Printf("[xboard] reload failed, trying restart: %v", err)
			if err := s.mgr.Restart(); err != nil {
				log.Printf("[xboard] restart failed: %v", err)
			}
		}
	} else {
		if err := s.mgr.Start(); err != nil {
			log.Printf("[xboard] start sing-box failed: %v", err)
		}
	}
}

func (s *XboardSync) updateLimiter(users []UserInfo) {
	if s.limiter == nil {
		return
	}
	limits := make(map[string]int, len(users))
	for _, u := range users {
		if u.DeviceLimit > 0 {
			limits[u.UUID] = u.DeviceLimit
		}
	}
	s.limiter.UpdateLimits(limits)
}

func (s *XboardSync) isNodeConfigChanged(newInfo *NodeInfo) bool {
	if s.lastNodeInfo == nil {
		return true
	}
	old := s.lastNodeInfo
	return old.Host != newInfo.Host ||
		old.Port != newInfo.Port ||
		old.SNI != newInfo.SNI ||
		old.TLS != newInfo.TLS ||
		old.Network != newInfo.Network ||
		old.Transport != newInfo.Transport ||
		old.RealityPrivateKey != newInfo.RealityPrivateKey ||
		old.RealityShortID != newInfo.RealityShortID ||
		old.RealityDest != newInfo.RealityDest ||
		old.SpeedLimit != newInfo.SpeedLimit ||
		old.NodeSecret != newInfo.NodeSecret ||
		old.RotationInterval != newInfo.RotationInterval ||
		old.Masquerade != newInfo.Masquerade ||
		old.ObfsType != newInfo.ObfsType ||
		old.ObfsPass != newInfo.ObfsPass ||
		old.CertPath != newInfo.CertPath ||
		old.KeyPath != newInfo.KeyPath ||
		old.ACMEDomain != newInfo.ACMEDomain ||
		old.ACMEEmail != newInfo.ACMEEmail
}

func (s *XboardSync) buildDeployRequest(u UserInfo, nodeInfo *NodeInfo) *configgen.DeployRequest {
	password := u.UUID
	uuid := u.UUID

	dynamicEnabled := nodeInfo.RotationInterval > 0 && nodeInfo.NodeSecret != ""
	if dynamicEnabled && u.DynamicPassword != "" {
		password = u.DynamicPassword
		if s.cfg.Xboard.NodeType == "vless" || s.cfg.Xboard.NodeType == "reality" {
			uuid = u.DynamicPassword
		}
	}

	req := &configgen.DeployRequest{
		UserID:      u.UUID,
		NodeID:      s.cfg.Xboard.NodeID.String(),
		Protocol:    s.mapNodeType(s.cfg.Xboard.NodeType),
		Server:      nodeInfo.Host,
		Port:        int(nodeInfo.Port),
		Password:    password,
		UUID:        uuid,
		DeviceLimit: u.DeviceLimit,
	}

	if nodeInfo.SNI != "" {
		req.SNI = nodeInfo.SNI
	}

	switch s.cfg.Xboard.NodeType {
	case "hysteria2", "hysteria":
		req.Protocol = "hysteria2"
		if nodeInfo.SpeedLimit > 0 {
			nodeLimit := int(nodeInfo.SpeedLimit / 1000000)
			if nodeLimit < 1 {
				nodeLimit = 1
			}
			req.UpMbps = nodeLimit
			req.DownMbps = nodeLimit
		} else {
			req.UpMbps = 100
			req.DownMbps = 200
		}
		if u.SpeedLimit > 0 {
			userLimit := int(u.SpeedLimit / 1000000)
			if userLimit < 1 {
				userLimit = 1
			}
			if userLimit < req.DownMbps {
				req.DownMbps = userLimit
				req.UpMbps = userLimit
			}
		}
		if nodeInfo.Masquerade != "" {
			req.Masquerade = nodeInfo.Masquerade
		}
		if nodeInfo.TLS == 1 {
			if nodeInfo.ACMEDomain != "" {
				req.ACMEDomain = nodeInfo.ACMEDomain
				req.ACMEEmail = nodeInfo.ACMEEmail
			} else if nodeInfo.CertPath != "" {
				req.TLSCertPath = nodeInfo.CertPath
				req.TLSKeyPath = nodeInfo.KeyPath
			}
		}
		if nodeInfo.ObfsType != "" {
			req.ObfsType = nodeInfo.ObfsType
			req.ObfsPass = nodeInfo.ObfsPass
		}

	case "vless":
		req.Protocol = "vless"
		if nodeInfo.TLS == 1 {
			if nodeInfo.ACMEDomain != "" {
				req.ACMEDomain = nodeInfo.ACMEDomain
				req.ACMEEmail = nodeInfo.ACMEEmail
			} else if nodeInfo.CertPath != "" {
				req.TLSCertPath = nodeInfo.CertPath
				req.TLSKeyPath = nodeInfo.KeyPath
			}
		}
		if nodeInfo.Network == "ws" {
			req.ObfsType = "ws"
			req.ObfsPass = nodeInfo.WSHost
			if nodeInfo.WSPath != "" {
				req.SNI = nodeInfo.WSPath
			}
		}
		if nodeInfo.Network == "grpc" && nodeInfo.GRPCServiceName != "" {
			req.ObfsType = "grpc"
			req.ObfsPass = nodeInfo.GRPCServiceName
		}

	case "reality":
		req.Protocol = "vless"
		if nodeInfo.RealityPrivateKey != "" {
			req.RealityPrivateKey = nodeInfo.RealityPrivateKey
		}
		if nodeInfo.RealityPublicKey != "" {
			req.RealityPublicKey = nodeInfo.RealityPublicKey
		}
		if nodeInfo.RealityShortID != "" {
			req.RealityShortID = nodeInfo.RealityShortID
		}
		if nodeInfo.RealityDest != "" {
			req.RealityDest = nodeInfo.RealityDestHost
			if nodeInfo.RealityDestPort > 0 {
				req.RealityDestPort = nodeInfo.RealityDestPort
			}
		}

	case "trojan":
		req.Protocol = "trojan"
		if nodeInfo.TLS == 1 {
			if nodeInfo.ACMEDomain != "" {
				req.ACMEDomain = nodeInfo.ACMEDomain
				req.ACMEEmail = nodeInfo.ACMEEmail
			} else if nodeInfo.CertPath != "" {
				req.TLSCertPath = nodeInfo.CertPath
				req.TLSKeyPath = nodeInfo.KeyPath
			}
		}
		if nodeInfo.ObfsType != "" {
			req.ObfsType = nodeInfo.ObfsType
			req.ObfsPass = nodeInfo.ObfsPass
		}
		if nodeInfo.Network == "ws" {
			req.ObfsType = "ws"
			req.ObfsPass = nodeInfo.WSHost
			if nodeInfo.WSPath != "" {
				req.SNI = nodeInfo.WSPath
			}
		}

	case "shadowsocks":
		req.Protocol = "shadowsocks"
	}

	if dynamicEnabled {
		prevPassword := s.computePrevWindowPassword(u.UUID, nodeInfo)
		if prevPassword != "" {
			req.PrevPassword = prevPassword
			if s.cfg.Xboard.NodeType == "vless" || s.cfg.Xboard.NodeType == "reality" {
				req.PrevUUID = formatDynamicUUID(prevPassword)
			}
		}
	}

	return req
}

func (s *XboardSync) computePrevWindowPassword(userUUID string, nodeInfo *NodeInfo) string {
	interval := int64(nodeInfo.RotationInterval)
	if interval <= 0 {
		return ""
	}
	prevWindow := (time.Now().Unix() / interval) - 1
	return computeDynamicPassword(userUUID, nodeInfo.NodeSecret, prevWindow)
}

func computeDynamicPassword(userUUID, nodeSecret string, timeWindow int64) string {
	message := fmt.Sprintf("%s:%d", nodeSecret, timeWindow)
	mac := hmac.New(sha256.New, []byte(userUUID))
	mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil)[:16])
}

func formatDynamicUUID(hash string) string {
	raw, err := base64.RawURLEncoding.DecodeString(hash)
	if err != nil || len(raw) < 16 {
		return hash
	}
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		binary.BigEndian.Uint32(raw[0:4]),
		binary.BigEndian.Uint16(raw[4:6]),
		binary.BigEndian.Uint16(raw[6:8]),
		binary.BigEndian.Uint16(raw[8:10]),
		raw[10:16])
}

func (s *XboardSync) mapNodeType(nodeType string) string {
	switch nodeType {
	case "hysteria", "hysteria2":
		return "hysteria2"
	case "vless":
		return "vless"
	case "reality":
		return "vless"
	case "trojan":
		return "trojan"
	case "vmess", "v2ray":
		return "vless"
	case "shadowsocks":
		return "shadowsocks"
	default:
		return nodeType
	}
}

func (s *XboardSync) reportAlive() {
	uuidToID := make(map[string]int)
	for id, info := range s.lastUserMap {
		uuidToID[info.UUID] = id
	}

	ipData := make(map[string][]string)
	currentIDs := make(map[int]bool)

	if mc, ok := s.collector.(*stats.MultiCollector); ok {
		if users, err := mc.GetOnlineUsers(); err == nil {
			for _, u := range users {
				if strings.HasSuffix(u.UserID, "@prev") {
					continue
				}
				id, ok := uuidToID[u.UserID]
				if !ok {
					continue
				}
				idStr := strconv.Itoa(id)
				currentIDs[id] = true
				if u.IP != "" {
					ipData[idStr] = append(ipData[idStr], u.IP)
				}
			}
		}
	}

	for id := range s.lastAliveUIDs {
		if !currentIDs[id] {
			idStr := strconv.Itoa(id)
			ipData[idStr] = []string{}
		}
	}

	s.lastAliveUIDs = currentIDs

	if err := s.client.ReportAliveWithIPs(ipData); err != nil {
		log.Printf("[xboard] failed to report alive: %v", err)
	}
}

func (s *XboardSync) reportTraffic() {
	v2ray := s.v2rayStats()
	if v2ray == nil {
		return
	}

	allTraffic, err := v2ray.GetAllUserTraffic()
	if err != nil {
		log.Printf("[xboard] failed to get user traffic: %v", err)
		return
	}

	if len(allTraffic) == 0 {
		return
	}

	cumTraffic := make(map[string][2]int64, len(allTraffic))
	for _, us := range allTraffic {
		if strings.HasPrefix(us.UserID, "outbound:") {
			continue
		}
		cumTraffic[us.UserID] = [2]int64{us.Upload, us.Download}
	}

	s.tracker.Process(cumTraffic, nil)

	if !s.tracker.HasTraffic() {
		return
	}

	trafficData := s.tracker.FlushTraffic()
	if len(trafficData) == 0 {
		return
	}

	uuidToID := make(map[string]int, len(s.lastUserMap))
	for id, info := range s.lastUserMap {
		uuidToID[info.UUID] = id
	}

	intTrafficData := make(map[int][2]int64, len(trafficData))
	for uuid, data := range trafficData {
		if id, ok := uuidToID[uuid]; ok {
			intTrafficData[id] = data
		} else {
			log.Printf("[xboard] traffic for unknown user %s, skipping", uuid)
		}
	}

	if len(intTrafficData) == 0 {
		return
	}

	if err := s.client.ReportUserTraffic(intTrafficData); err != nil {
		log.Printf("[xboard] failed to report traffic: %v", err)
		for id, data := range intTrafficData {
			uuid := ""
			for u, i := range uuidToID {
				if i == id {
					uuid = u
					break
				}
			}
			if uuid != "" {
				trafficData[uuid] = data
			}
		}
		s.tracker.RestoreTraffic(trafficData)
	} else {
		log.Printf("[xboard] reported traffic for %d users", len(intTrafficData))
	}
}

func (s *XboardSync) reportNodeStatus() {
	cpuPercent, err := utils.GetCPUUsage()
	if err != nil {
		log.Printf("[xboard] failed to get cpu usage: %v", err)
		return
	}

	memPercent, memUsedMB, memTotalMB := utils.GetMemoryUsage()
	_ = memPercent

	swapUsedMB, swapTotalMB := uint64(0), uint64(0)
	if v, err := mem.SwapMemory(); err == nil {
		swapUsedMB = v.Used / 1024 / 1024
		swapTotalMB = v.Total / 1024 / 1024
	}

	diskUsedGB, diskTotalGB := utils.GetDiskUsage()
	netIn, netOut := utils.GetNetSpeed()
	gcMetrics := utils.GetGCMetrics()

	status := &NodeStatus{
		CPU: cpuPercent,
		Mem: MemoryStatus{
			Total: int64(memTotalMB) * 1024 * 1024,
			Used:  int64(memUsedMB) * 1024 * 1024,
		},
		NetInSpeed:  netIn,
		NetOutSpeed: netOut,
		Goroutines:  gcMetrics.Goroutines,
		NumGC:       gcMetrics.NumGC,
		LastPauseMS: gcMetrics.LastPauseMS,
	}

	if swapTotalMB > 0 {
		status.Swap = MemoryStatus{
			Total: int64(swapTotalMB) * 1024 * 1024,
			Used:  int64(swapUsedMB) * 1024 * 1024,
		}
	}

	if diskTotalGB > 0 {
		status.Disk = MemoryStatus{
			Total: int64(diskTotalGB) * 1024 * 1024 * 1024,
			Used:  int64(diskUsedGB) * 1024 * 1024 * 1024,
		}
	}

	if err := s.client.ReportStatus(status); err != nil {
		log.Printf("[xboard] failed to report status: %v", err)
	}
}

func (s *XboardSync) v2rayStats() *stats.V2RayStatsCollector {
	if mc, ok := s.collector.(*stats.MultiCollector); ok {
		return mc.GetV2RayStats()
	}
	return nil
}

func (s *XboardSync) collectMetrics() map[string]interface{} {
	cpuPercent, err := utils.GetCPUUsage()
	if err != nil {
		cpuPercent = 0
	}

	_, memUsedMB, _ := utils.GetMemoryUsage()

	diskUsedGB, _ := utils.GetDiskUsage()
	netIn, netOut := utils.GetNetSpeed()

	// Get active connections from stats collector
	activeConnections := 0
	if connData, err := s.collector.GetConnections(); err == nil && connData != nil {
		activeConnections = connData.ActiveConnections
	}

	// Get cumulative traffic from stats collector
	var totalIn, totalOut int64
	if traffic, err := s.collector.GetTraffic(); err == nil && traffic != nil {
		totalIn = traffic.Upload
		totalOut = traffic.Download
	}

	// Match Xboard panel expected format:
	// node_id, cpu, memory(MB), disk(GB), online, network_in(bytes), network_out(bytes),
	// network_in_speed(bytes/s), network_out_speed(bytes/s)
	result := map[string]interface{}{
		"node_id":           s.cfg.Xboard.NodeID,
		"api":               1, // API is running (we're in the metrics collector, so API is alive)
		"kernel":            s.mgr.IsRunning(),
		"cpu":               cpuPercent,
		"memory":            float64(memUsedMB),
		"disk":              float64(diskUsedGB),
		"online":            activeConnections,
		"network_in":        totalIn,
		"network_out":       totalOut,
		"network_in_speed":  netIn,
		"network_out_speed": netOut,
	}

	return result
}

func (s *XboardSync) wsDiscovery() {
	if s.wsClient.IsConnected() {
		return
	}

	if err := s.wsClient.RefreshHandshake(); err != nil {
		log.Printf("[xboard] ws discovery handshake failed: %v", err)
		return
	}

	hs, err := s.wsClient.GetHandshakeResponse()
	if err != nil || hs == nil {
		return
	}

	if hs.WebSocket.Enabled && hs.WebSocket.WsURL != "" {
		log.Printf("[xboard] ws discovery: websocket available, triggering reconnect")
		s.wsClient.Connect()
	}
}

type NodeInfo struct {
	Host             string        `json:"host"`
	Port             FlexibleInt   `json:"port"`
	ServerPort       FlexibleInt   `json:"server_port"`
	ServerName       string        `json:"server_name"`
	SNI              string        `json:"sni"`
	Transport        string        `json:"transport"`
	Network          string        `json:"network"`
	TLS              int           `json:"tls"`
	SpeedLimit       int64         `json:"speed_limit"`
	PushInterval     int           `json:"push_interval"`
	PullInterval     int           `json:"pull_interval"`
	Routes           []RouteConfig `json:"routes"`
	NodeSecret       string        `json:"node_secret"`
	RotationInterval int           `json:"rotation_interval"`

	RealityPrivateKey string `json:"private_key"`
	RealityPublicKey  string `json:"public_key"`
	RealityShortID    string `json:"short_id"`
	RealityShortIDV2  string `json:"short_id_v2"`
	RealityDest       string `json:"dest"`
	RealityDestHost   string `json:"dest_host"`
	RealityDestPort   int    `json:"dest_port"`

	CertPath   string `json:"cert_path"`
	KeyPath    string `json:"key_path"`
	ACMEDomain string `json:"acme_domain"`
	ACMEEmail  string `json:"acme_email"`

	WSPath          string                 `json:"ws_path"`
	WSHost          string                 `json:"ws_host"`
	GRPCServiceName string                 `json:"grpc_service_name"`
	Masquerade      string                 `json:"masquerade"`
	ObfsType        string                 `json:"obfs_type"`
	ObfsPass        string                 `json:"obfs_password"`
	NetworkSettings map[string]interface{} `json:"network_settings"`
}

type RouteConfig struct {
	Match       []string `json:"match"`
	Action      string   `json:"action"`
	ActionValue string   `json:"action_value"`
}

type UserInfo struct {
	ID                int     `json:"id"`
	UUID              string  `json:"uuid"`
	SpeedLimit        float64 `json:"speed_limit"`
	DeviceLimit       int     `json:"device_limit"`
	DynamicPassword   string  `json:"dynamic_password"`
	PasswordExpiresAt int64   `json:"password_expires_at"`
}
