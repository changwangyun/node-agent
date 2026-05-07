package xboard

import (
	"fmt"
	"log"
	"sync"
	"time"

	"node-agent/config"
	"node-agent/core/configgen"
	"node-agent/core/singbox"
	"node-agent/core/stats"
)

type XboardSync struct {
	cfg       *config.Config
	mgr       *singbox.Manager
	generator *configgen.Generator
	collector stats.Collector
	client    *XboardClient

	stopCh chan struct{}
	mu     sync.Mutex

	lastUserMap map[int]*UserInfo
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
		log.Printf("[xboard] failed to get node info: %v", err)
		return
	}

	users, err := s.client.GetUserList()
	if err != nil {
		log.Printf("[xboard] failed to get user list: %v", err)
		return
	}

	s.deployNode(nodeInfo, users)
	s.reportTraffic()
}

func (s *XboardSync) deployNode(nodeInfo *NodeInfo, users []UserInfo) {
	currentUserMap := make(map[int]*UserInfo)
	for i := range users {
		currentUserMap[users[i].ID] = &users[i]
	}

	deployedUsers := s.generator.GetDeployedUsers()

	needUpdate := len(deployedUsers) != len(users)
	if !needUpdate {
		for _, u := range users {
			if _, exists := deployedUsers[u.UUID]; !exists {
				needUpdate = true
				break
			}
		}
	}

	if !needUpdate {
		s.lastUserMap = currentUserMap
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

	log.Printf("[xboard] config updated: %d users deployed", len(users))
	s.lastUserMap = currentUserMap
}

func (s *XboardSync) buildDeployRequest(u UserInfo, nodeInfo *NodeInfo) *configgen.DeployRequest {
	req := &configgen.DeployRequest{
		UserID:   u.UUID,
		NodeID:   fmt.Sprintf("%d", s.cfg.Xboard.NodeID),
		Protocol: s.cfg.Xboard.NodeType,
		Server:   nodeInfo.Host,
		Port:     nodeInfo.Port,
		Password: u.UUID,
		UUID:     u.UUID,
	}

	if nodeInfo.SNI != "" {
		req.SNI = nodeInfo.SNI
	}

	if s.cfg.Xboard.NodeType == "hysteria2" {
		req.UpMbps = 100
		req.DownMbps = 200
		if u.SpeedLimit > 0 {
			req.DownMbps = int(u.SpeedLimit / 1000000)
			if req.DownMbps < 10 {
				req.DownMbps = 10
			}
			req.UpMbps = req.DownMbps
		}
	}

	if nodeInfo.RealityPrivateKey != "" {
		req.RealityPrivateKey = nodeInfo.RealityPrivateKey
		req.RealityShortID = nodeInfo.RealityShortID
		req.RealityDest = nodeInfo.RealityDest
		if nodeInfo.RealityDestPort > 0 {
			req.RealityDestPort = nodeInfo.RealityDestPort
		}
	}

	return req
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
		trafficData[us.UserID] = [2]int64{us.Upload, us.Download}
	}

	if err := s.client.ReportUserTraffic(trafficData); err != nil {
		log.Printf("[xboard] failed to report traffic: %v", err)
	}
}

func (s *XboardSync) v2rayStats() *stats.V2RayStatsCollector {
	if mc, ok := s.collector.(*stats.MultiCollector); ok {
		return mc.GetV2RayStats()
	}
	return nil
}

type NodeInfo struct {
	Host              string        `json:"host"`
	Port              int           `json:"port"`
	ServerName        string        `json:"server_name"`
	SNI               string        `json:"sni"`
	Transport         string        `json:"transport"`
	Network           string        `json:"network"`
	TLS               int           `json:"tls"`
	RealityPrivateKey string        `json:"private_key"`
	RealityShortID    string        `json:"short_id"`
	RealityDest       string        `json:"dest"`
	RealityDestPort   int           `json:"dest_port"`
	SpeedLimit        int64         `json:"speed_limit"`
	Routes            []RouteConfig `json:"routes"`
}

type RouteConfig struct {
	Match       []string `json:"match"`
	Action      string   `json:"action"`
	ActionValue string   `json:"action_value"`
}

type UserInfo struct {
	ID         int     `json:"id"`
	UUID       string  `json:"uuid"`
	SpeedLimit float64 `json:"speed_limit"`
}
