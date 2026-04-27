package stats

import (
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
	Connections ConnectionData `json:"connections"`
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

	lastTraffic  *TrafficData
	totalTraffic *TrafficData
}

type clashTrafficResponse struct {
	Up   int64 `json:"up"`
	Down int64 `json:"down"`
}

type clashConnectionsResponse struct {
	Total       int                     `json:"total"`
	Connections []clashConnectionDetail `json:"connections"`
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
		totalTraffic: &TrafficData{},
		lastTraffic:  &TrafficData{},
	}
}

func (s *SingBoxStatsCollector) GetTraffic() (*TrafficData, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	url := fmt.Sprintf("%s/traffic", s.baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create traffic request: %w", err)
	}
	if s.secret != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.secret))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return s.totalTraffic, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return s.totalTraffic, nil
	}

	var traffic clashTrafficResponse
	if err := json.Unmarshal(body, &traffic); err != nil {
		return s.totalTraffic, nil
	}

	deltaUp := traffic.Up - s.lastTraffic.Upload
	deltaDown := traffic.Down - s.lastTraffic.Download

	if deltaUp > 0 {
		s.totalTraffic.Upload += deltaUp
		s.lastTraffic.Upload = traffic.Up
	}
	if deltaDown > 0 {
		s.totalTraffic.Download += deltaDown
		s.lastTraffic.Download = traffic.Down
	}

	return &TrafficData{
		Upload:   s.totalTraffic.Upload,
		Download: s.totalTraffic.Download,
	}, nil
}

func (s *SingBoxStatsCollector) GetConnections() (*ConnectionData, error) {
	url := fmt.Sprintf("%s/connections", s.baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create connections request: %w", err)
	}
	if s.secret != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.secret))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return &ConnectionData{ActiveConnections: 0}, nil
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return &ConnectionData{ActiveConnections: 0}, nil
	}

	var connResp clashConnectionsResponse
	if err := json.Unmarshal(body, &connResp); err != nil {
		return &ConnectionData{ActiveConnections: 0}, nil
	}

	return &ConnectionData{
		ActiveConnections: connResp.Total,
	}, nil
}

func (s *SingBoxStatsCollector) GetOnlineUsers() ([]*OnlineUser, error) {
	url := fmt.Sprintf("%s/connections", s.baseURL)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create connections request: %w", err)
	}
	if s.secret != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", s.secret))
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request clash api: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var connResp clashConnectionsResponse
	if err := json.Unmarshal(body, &connResp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	userMap := make(map[string]*OnlineUser)
	for _, conn := range connResp.Connections {
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
	return result, nil
}

func (s *SingBoxStatsCollector) GetStats() (*StatsResult, error) {
	traffic, err := s.GetTraffic()
	if err != nil {
		traffic = &TrafficData{}
	}

	connections, err := s.GetConnections()
	if err != nil {
		connections = &ConnectionData{}
	}

	return &StatsResult{
		Traffic:     *traffic,
		Connections: *connections,
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
	primary  Collector
	fallback Collector
}

func NewMultiCollector(primary, fallback Collector) *MultiCollector {
	return &MultiCollector{
		primary:  primary,
		fallback: fallback,
	}
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
