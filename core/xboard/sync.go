package xboard

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"node-agent/config"
	"node-agent/core/configgen"
	"node-agent/core/singbox"
	"node-agent/core/stats"
	"node-agent/utils"

	"github.com/shirou/gopsutil/v3/mem"
)

type XboardSync struct {
	cfg       *config.Config
	mgr       *singbox.Manager
	generator *configgen.Generator
	collector stats.Collector
	client    *XboardClient

	stopCh chan struct{}
	mu     sync.Mutex

	lastUserMap  map[int]*UserInfo
	lastNodeInfo *NodeInfo

	lastTraffic map[string][2]int64
}

func NewXboardSync(
	cfg *config.Config,
	mgr *singbox.Manager,
	generator *configgen.Generator,
	collector stats.Collector,
) *XboardSync {
	xcfg := cfg.Xboard
	client := NewXboardClient(xcfg.APIHost, xcfg.APIKey, xcfg.NodeID, xcfg.NodeType, xcfg.Timeout)

	return &XboardSync{
		cfg:         cfg,
		mgr:         mgr,
		generator:   generator,
		collector:   collector,
		client:      client,
		stopCh:      make(chan struct{}),
		lastUserMap: make(map[int]*UserInfo),
		lastTraffic: make(map[string][2]int64),
	}
}

func (s *XboardSync) Start() {
	interval := time.Duration(s.cfg.Xboard.SyncInterval) * time.Second
	if interval < 10*time.Second {
		interval = 60 * time.Second
	}

	s.syncOnce()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.syncOnce()
		case <-s.stopCh:
			return
		}
	}
}

func (s *XboardSync) Stop() {
	close(s.stopCh)
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

	s.reportTraffic()
	s.reportNodeStatus()
}

func (s *XboardSync) deployNode(nodeInfo *NodeInfo, users []UserInfo) {
	currentUserMap := make(map[int]*UserInfo)
	for i := range users {
		currentUserMap[users[i].ID] = &users[i]
	}

	configChanged := s.isNodeConfigChanged(nodeInfo)
	usersChanged := s.isUsersChanged(users)

	if !configChanged && !usersChanged {
		s.lastUserMap = currentUserMap
		s.lastNodeInfo = nodeInfo
		return
	}

	var reqs []configgen.DeployRequest
	for _, u := range users {
		req := s.buildDeployRequest(u, nodeInfo)
		reqs = append(reqs, *req)
	}

	if err := s.generator.SetUsers(reqs); err != nil {
		log.Printf("[xboard] failed to set users: %v", err)
		return
	}

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

	reason := ""
	if configChanged {
		reason = "config changed"
	} else {
		reason = "users changed"
	}
	log.Printf("[xboard] config updated: %d users deployed (%s)", len(users), reason)
	s.lastUserMap = currentUserMap
	s.lastNodeInfo = nodeInfo
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
		old.RotationInterval != newInfo.RotationInterval
}

func (s *XboardSync) isUsersChanged(users []UserInfo) bool {
	deployedUsers := s.generator.GetDeployedUsers()
	if len(deployedUsers) != len(users) {
		return true
	}
	for _, u := range users {
		if _, exists := deployedUsers[u.UUID]; !exists {
			return true
		}
	}
	return false
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
		UserID:   u.UUID,
		NodeID:   fmt.Sprintf("%d", s.cfg.Xboard.NodeID),
		Protocol: s.mapNodeType(s.cfg.Xboard.NodeType),
		Server:   nodeInfo.Host,
		Port:     nodeInfo.Port,
		Password: password,
		UUID:     uuid,
	}

	if nodeInfo.SNI != "" {
		req.SNI = nodeInfo.SNI
	}

	switch s.cfg.Xboard.NodeType {
	case "hysteria2", "hysteria":
		req.Protocol = "hysteria2"
		req.UpMbps = 100
		req.DownMbps = 200
		if u.SpeedLimit > 0 {
			req.DownMbps = int(u.SpeedLimit / 1000000)
			if req.DownMbps < 10 {
				req.DownMbps = 10
			}
			req.UpMbps = req.DownMbps
		}
		if nodeInfo.SpeedLimit > 0 {
			nodeLimit := int(nodeInfo.SpeedLimit / 1000000)
			if nodeLimit > 0 && nodeLimit < req.DownMbps {
				req.DownMbps = nodeLimit
				req.UpMbps = nodeLimit
			}
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

	if u.SpeedLimit > 0 && req.Protocol != "hysteria2" {
		speedBps := int64(u.SpeedLimit)
		_ = speedBps
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

	trafficData := make(map[string][2]int64)
	for _, us := range allTraffic {
		last, exists := s.lastTraffic[us.UserID]
		var delta [2]int64
		if exists {
			delta[0] = us.Upload - last[0]
			delta[1] = us.Download - last[1]
			if delta[0] < 0 {
				delta[0] = 0
			}
			if delta[1] < 0 {
				delta[1] = 0
			}
		} else {
			delta[0] = us.Upload
			delta[1] = us.Download
		}

		if delta[0] > 0 || delta[1] > 0 {
			trafficData[us.UserID] = delta
		}

		s.lastTraffic[us.UserID] = [2]int64{us.Upload, us.Download}
	}

	if len(trafficData) == 0 {
		return
	}

	if err := s.client.ReportUserTraffic(trafficData); err != nil {
		log.Printf("[xboard] failed to report traffic: %v", err)
	} else {
		log.Printf("[xboard] reported traffic for %d users", len(trafficData))
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

	status := &NodeStatus{
		CPU: cpuPercent,
		Mem: MemoryStatus{
			Total: int64(memTotalMB) * 1024 * 1024,
			Used:  int64(memUsedMB) * 1024 * 1024,
		},
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

type NodeInfo struct {
	Host               string                 `json:"host"`
	Port               int                    `json:"port"`
	ServerName         string                 `json:"server_name"`
	SNI                string                 `json:"sni"`
	Transport          string                 `json:"transport"`
	Network            string                 `json:"network"`
	TLS                int                    `json:"tls"`
	SpeedLimit         int64                  `json:"speed_limit"`
	PushInterval       int                    `json:"push_interval"`
	PullInterval       int                    `json:"pull_interval"`
	Routes             []RouteConfig          `json:"routes"`
	NodeSecret         string                 `json:"node_secret"`
	RotationInterval   int                    `json:"rotation_interval"`

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

func (n *NodeInfo) MarshalJSON() ([]byte, error) {
	type Alias NodeInfo
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(n),
	})
}
