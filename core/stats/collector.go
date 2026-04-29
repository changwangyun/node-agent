package stats

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

type TrafficData struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
}

type SpeedData struct {
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
}

type ConnectionData struct {
	ActiveConnections int `json:"active_connections"`
}

type OnlineUser struct {
	UserID   string `json:"user_id"`
	Inbound  string `json:"inbound"`
	IP       string `json:"ip"`
	Upload   int64  `json:"upload"`
	Download int64  `json:"download"`
}

type StatsResult struct {
	Traffic     TrafficData    `json:"traffic"`
	Speed       SpeedData      `json:"speed"`
	Connections ConnectionData `json:"connections"`
	OnlineUsers []*OnlineUser  `json:"online_users,omitempty"`
}

type Collector interface {
	GetTraffic() (*TrafficData, error)
	GetConnections() (*ConnectionData, error)
	GetStats() (*StatsResult, error)
}

type SingBoxStatsCollector struct {
	mu      sync.RWMutex
	baseURL string
	secret  string
	client  *http.Client

	lastConnTraffic *TrafficData
	totalTraffic    *TrafficData
}

type clashTrafficResponse struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

type clashConnectionsResponse struct {
	DownloadTotal int64                   `json:"downloadTotal"`
	UploadTotal   int64                   `json:"uploadTotal"`
	Connections   []clashConnectionDetail `json:"connections"`
	Memory        int64                   `json:"memory"`
}

type clashConnectionDetail struct {
	ID       string `json:"id"`
	Metadata struct {
		Network     string `json:"network"`
		Type        string `json:"type"`
		SourceIP    string `json:"sourceIP"`
		Host        string `json:"host"`
		Inbound     string `json:"inbound"`
		InboundUser string `json:"inboundUser"`
	} `json:"metadata"`
	Upload   int64 `json:"upload"`
	Download int64 `json:"download"`
}

func NewSingBoxStatsCollector(clashAPIAddr, secret string) *SingBoxStatsCollector {
	return &SingBoxStatsCollector{
		baseURL: fmt.Sprintf("http://%s", clashAPIAddr),
		secret:  secret,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		totalTraffic:    &TrafficData{},
		lastConnTraffic: &TrafficData{},
	}
}

func (s *SingBoxStatsCollector) doRequest(path string) ([]byte, error) {
	url := fmt.Sprintf("%s%s", s.baseURL, path)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	if s.secret != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.secret))
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

func (s *SingBoxStatsCollector) GetTraffic() (*TrafficData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	body, err := s.doRequest("/connections")
	if err != nil {
		return s.totalTraffic, nil
	}

	var connResp clashConnectionsResponse
	if err := json.Unmarshal(body, &connResp); err != nil {
		return s.totalTraffic, nil
	}

	currentUp := connResp.UploadTotal
	currentDown := connResp.DownloadTotal

	deltaUp := currentUp - s.lastConnTraffic.Upload
	deltaDown := currentDown - s.lastConnTraffic.Download

	if deltaUp > 0 {
		s.totalTraffic.Upload += deltaUp
	}
	if deltaDown > 0 {
		s.totalTraffic.Download += deltaDown
	}

	s.lastConnTraffic.Upload = currentUp
	s.lastConnTraffic.Download = currentDown

	return &TrafficData{
		Upload:   s.totalTraffic.Upload,
		Download: s.totalTraffic.Download,
	}, nil
}

func (s *SingBoxStatsCollector) GetSpeed() (*SpeedData, error) {
	url := fmt.Sprintf("%s/traffic", s.baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return &SpeedData{}, nil
	}
	if s.secret != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.secret))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return &SpeedData{}, nil
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	if scanner.Scan() {
		var traffic clashTrafficResponse
		if err := json.Unmarshal(scanner.Bytes(), &traffic); err == nil {
			return &SpeedData{
				Upload:   traffic.Up,
				Download: traffic.Down,
			}, nil
		}
	}

	return &SpeedData{}, nil
}

func (s *SingBoxStatsCollector) GetConnections() (*ConnectionData, error) {
	body, err := s.doRequest("/connections")
	if err != nil {
		return &ConnectionData{ActiveConnections: 0}, nil
	}

	var connResp clashConnectionsResponse
	if err := json.Unmarshal(body, &connResp); err != nil {
		return &ConnectionData{ActiveConnections: 0}, nil
	}

	return &ConnectionData{
		ActiveConnections: len(connResp.Connections),
	}, nil
}

func extractOnlineUsers(connections []clashConnectionDetail) []*OnlineUser {
	userMap := make(map[string]*OnlineUser)
	for _, conn := range connections {
		userID := conn.Metadata.InboundUser
		if userID == "" {
			continue
		}
		if existing, ok := userMap[userID]; ok {
			existing.Upload += conn.Upload
			existing.Download += conn.Download
		} else {
			userMap[userID] = &OnlineUser{
				UserID:   userID,
				Inbound:  conn.Metadata.Inbound,
				IP:       conn.Metadata.SourceIP,
				Upload:   conn.Upload,
				Download: conn.Download,
			}
		}
	}
	result := make([]*OnlineUser, 0, len(userMap))
	for _, u := range userMap {
		result = append(result, u)
	}
	return result
}

func (s *SingBoxStatsCollector) GetOnlineUsers() ([]*OnlineUser, error) {
	body, err := s.doRequest("/connections")
	if err != nil {
		return nil, fmt.Errorf("request clash api: %w", err)
	}

	var connResp clashConnectionsResponse
	if err := json.Unmarshal(body, &connResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return extractOnlineUsers(connResp.Connections), nil
}

func (s *SingBoxStatsCollector) GetStats() (*StatsResult, error) {
	s.mu.Lock()

	body, err := s.doRequest("/connections")
	if err != nil {
		s.mu.Unlock()
		speed, _ := s.GetSpeed()
		return &StatsResult{Speed: *speed}, nil
	}

	var connResp clashConnectionsResponse
	if err := json.Unmarshal(body, &connResp); err != nil {
		s.mu.Unlock()
		speed, _ := s.GetSpeed()
		return &StatsResult{Speed: *speed}, nil
	}

	currentUp := connResp.UploadTotal
	currentDown := connResp.DownloadTotal

	deltaUp := currentUp - s.lastConnTraffic.Upload
	deltaDown := currentDown - s.lastConnTraffic.Download
	if deltaUp > 0 {
		s.totalTraffic.Upload += deltaUp
	}
	if deltaDown > 0 {
		s.totalTraffic.Download += deltaDown
	}
	s.lastConnTraffic.Upload = currentUp
	s.lastConnTraffic.Download = currentDown

	traffic := TrafficData{
		Upload:   s.totalTraffic.Upload,
		Download: s.totalTraffic.Download,
	}
	connections := ConnectionData{
		ActiveConnections: len(connResp.Connections),
	}

	onlineUsers := extractOnlineUsers(connResp.Connections)

	s.mu.Unlock()

	speed, _ := s.GetSpeed()

	return &StatsResult{
		Traffic:     traffic,
		Speed:       *speed,
		Connections: connections,
		OnlineUsers: onlineUsers,
	}, nil
}

type FallbackCollector struct {
	mu            sync.RWMutex
	uploadBytes   int64
	downloadBytes int64
	connections   int
}

func NewFallbackCollector() *FallbackCollector {
	return &FallbackCollector{}
}

func (f *FallbackCollector) GetTraffic() (*TrafficData, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return &TrafficData{
		Upload:   f.uploadBytes,
		Download: f.downloadBytes,
	}, nil
}

func (f *FallbackCollector) GetConnections() (*ConnectionData, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	return &ConnectionData{
		ActiveConnections: f.connections,
	}, nil
}

func (f *FallbackCollector) GetStats() (*StatsResult, error) {
	traffic, _ := f.GetTraffic()
	connections, _ := f.GetConnections()
	return &StatsResult{
		Traffic:     *traffic,
		Connections: *connections,
	}, nil
}

func (f *FallbackCollector) AddUpload(bytes int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uploadBytes += bytes
}

func (f *FallbackCollector) AddDownload(bytes int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.downloadBytes += bytes
}

func (f *FallbackCollector) SetConnections(count int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connections = count
}

type MultiCollector struct {
	primary     Collector
	fallback    Collector
	v2rayStats  *V2RayStatsCollector
	lastTraffic map[string]*UserTraffic
}

func NewMultiCollector(primary, fallback Collector) *MultiCollector {
	return &MultiCollector{
		primary:     primary,
		fallback:    fallback,
		lastTraffic: make(map[string]*UserTraffic),
	}
}

func (m *MultiCollector) SetV2RayStats(v *V2RayStatsCollector) {
	m.v2rayStats = v
}

func (m *MultiCollector) GetTraffic() (*TrafficData, error) {
	data, err := m.primary.GetTraffic()
	if err != nil {
		return m.fallback.GetTraffic()
	}
	return data, nil
}

func (m *MultiCollector) GetConnections() (*ConnectionData, error) {
	data, err := m.primary.GetConnections()
	if err != nil {
		return m.fallback.GetConnections()
	}
	return data, nil
}

func (m *MultiCollector) GetStats() (*StatsResult, error) {
	data, err := m.primary.GetStats()
	if err != nil {
		return m.fallback.GetStats()
	}
	return data, nil
}

func (m *MultiCollector) GetOnlineUsers() ([]*OnlineUser, error) {
	if sc, ok := m.primary.(*SingBoxStatsCollector); ok {
		users, err := sc.GetOnlineUsers()
		if err == nil && len(users) > 0 {
			return users, nil
		}
	}

	if m.v2rayStats != nil {
		return m.getOnlineUsersFromV2Ray()
	}

	return nil, fmt.Errorf("no online user source available")
}

func (m *MultiCollector) getOnlineUsersFromV2Ray() ([]*OnlineUser, error) {
	allTraffic, err := m.v2rayStats.GetAllUserTraffic()
	if err != nil {
		return nil, fmt.Errorf("v2ray api: %w", err)
	}

	var result []*OnlineUser
	for _, ut := range allTraffic {
		if ut.Upload == 0 && ut.Download == 0 {
			continue
		}

		last, exists := m.lastTraffic[ut.UserID]
		isActive := !exists || ut.Upload > last.Upload || ut.Download > last.Download
		if isActive {
			result = append(result, &OnlineUser{
				UserID:   ut.UserID,
				Upload:   ut.Upload - last.Upload,
				Download: ut.Download - last.Download,
			})
		}
	}

	newSnapshot := make(map[string]*UserTraffic, len(allTraffic))
	for _, ut := range allTraffic {
		utCopy := *ut
		newSnapshot[ut.UserID] = &utCopy
	}
	m.lastTraffic = newSnapshot

	return result, nil
}
