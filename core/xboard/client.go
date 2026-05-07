package xboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type XboardClient struct {
	apiHost  string
	apiKey   string
	nodeID   string
	nodeType string
	timeout  time.Duration
	client   *http.Client

	mu    sync.RWMutex
	etags map[string]string
}

func NewXboardClient(apiHost, apiKey string, nodeID string, nodeType string, timeout int) *XboardClient {
	t := time.Duration(timeout) * time.Second
	if t < 5*time.Second {
		t = 30 * time.Second
	}
	return &XboardClient{
		apiHost:  apiHost,
		apiKey:   apiKey,
		nodeID:   nodeID,
		nodeType: nodeType,
		timeout:  t,
		client: &http.Client{
			Timeout: t,
		},
		etags: make(map[string]string),
	}
}

func (c *XboardClient) buildURL(path string) string {
	return fmt.Sprintf("%s/api/v1/server/UniProxy%s?node_id=%s&node_type=%s&token=%s",
		c.apiHost, path, c.nodeID, c.nodeType, c.apiKey)
}

func (c *XboardClient) doGet(url string, etagKey string) (*http.Response, []byte, bool, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, false, fmt.Errorf("create request: %w", err)
	}

	c.mu.RLock()
	if etag, ok := c.etags[etagKey]; ok {
		req.Header.Set("If-None-Match", etag)
	}
	c.mu.RUnlock()

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, nil, false, fmt.Errorf("request: %w", err)
	}

	if resp.StatusCode == 304 {
		resp.Body.Close()
		return nil, nil, true, nil
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, nil, false, fmt.Errorf("read response: %w", err)
	}

	if etag := resp.Header.Get("ETag"); etag != "" {
		c.mu.Lock()
		c.etags[etagKey] = etag
		c.mu.Unlock()
	}

	return resp, body, false, nil
}

func (c *XboardClient) doPost(url string, payload interface{}) error {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("marshal payload: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	resp, err := c.client.Post(url, "application/json", bodyReader)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}

func (c *XboardClient) GetNodeInfo() (*NodeInfo, error) {
	url := c.buildURL("/config")

	resp, body, notModified, err := c.doGet(url, "config")
	if err != nil {
		return nil, fmt.Errorf("request config: %w", err)
	}
	if notModified {
		return nil, fmt.Errorf("not modified")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	data := raw

	nodeInfo := &NodeInfo{}

	if v, ok := data["host"].(string); ok {
		nodeInfo.Host = v
	}
	if v, ok := data["server"].(string); ok && nodeInfo.Host == "" {
		nodeInfo.Host = v
	}
	if v, ok := data["port"].(float64); ok {
		nodeInfo.Port = int(v)
	}
	if v, ok := data["server_port"].(float64); ok && nodeInfo.Port == 0 {
		nodeInfo.Port = int(v)
	}
	if v, ok := data["server_name"].(string); ok {
		nodeInfo.ServerName = v
		nodeInfo.SNI = v
	}
	if v, ok := data["sni"].(string); ok {
		nodeInfo.SNI = v
	}
	if v, ok := data["network"].(string); ok {
		nodeInfo.Network = v
	}
	if v, ok := data["transport"].(string); ok {
		nodeInfo.Transport = v
	}
	if v, ok := data["tls"].(float64); ok {
		nodeInfo.TLS = int(v)
	}
	if v, ok := data["speed_limit"].(float64); ok {
		nodeInfo.SpeedLimit = int64(v)
	}
	if v, ok := data["node_secret"].(string); ok {
		nodeInfo.NodeSecret = v
	}
	if v, ok := data["rotation_interval"].(float64); ok {
		nodeInfo.RotationInterval = int(v)
	}
	if v, ok := data["masquerade"].(string); ok {
		nodeInfo.Masquerade = v
	}
	if v, ok := data["obfs_type"].(string); ok {
		nodeInfo.ObfsType = v
	}
	if v, ok := data["obfs_password"].(string); ok {
		nodeInfo.ObfsPass = v
	}

	if tlsSettings, ok := data["tls_settings"].(map[string]interface{}); ok {
		parseTLSSettings(tlsSettings, nodeInfo)
	}

	if protocolSettings, ok := data["protocol_settings"].(map[string]interface{}); ok {
		parseProtocolSettings(protocolSettings, nodeInfo)
	}

	if networkSettings, ok := data["network_settings"].(map[string]interface{}); ok {
		parseNetworkSettings(networkSettings, nodeInfo)
	}

	if baseConfig, ok := data["base_config"].(map[string]interface{}); ok {
		if v, ok := baseConfig["push_interval"].(float64); ok {
			nodeInfo.PushInterval = int(v)
		}
		if v, ok := baseConfig["pull_interval"].(float64); ok {
			nodeInfo.PullInterval = int(v)
		}
	}

	if routes, ok := data["routes"].([]interface{}); ok {
		for _, r := range routes {
			if routeMap, ok := r.(map[string]interface{}); ok {
				rc := RouteConfig{}
				if action, ok := routeMap["action"].(string); ok {
					rc.Action = action
				}
				if actionValue, ok := routeMap["action_value"].(string); ok {
					rc.ActionValue = actionValue
				}
				if matches, ok := routeMap["match"].([]interface{}); ok {
					for _, m := range matches {
						if ms, ok := m.(string); ok {
							rc.Match = append(rc.Match, ms)
						}
					}
				}
				nodeInfo.Routes = append(nodeInfo.Routes, rc)
			}
		}
	}

	return nodeInfo, nil
}

func parseTLSSettings(tlsSettings map[string]interface{}, nodeInfo *NodeInfo) {
	if v, ok := tlsSettings["private_key"].(string); ok {
		nodeInfo.RealityPrivateKey = v
	}
	if v, ok := tlsSettings["short_id"].(string); ok {
		nodeInfo.RealityShortID = v
	}
	if v, ok := tlsSettings["dest"].(string); ok {
		nodeInfo.RealityDest = v
	}
	if v, ok := tlsSettings["server_name"].(string); ok && nodeInfo.SNI == "" {
		nodeInfo.SNI = v
	}
	if v, ok := tlsSettings["public_key"].(string); ok {
		nodeInfo.RealityPublicKey = v
	}
	if v, ok := tlsSettings["short_id_v2"].(string); ok {
		nodeInfo.RealityShortIDV2 = v
	}
	if destStr, ok := tlsSettings["dest"].(string); ok {
		nodeInfo.RealityDest = destStr
		if host, portStr, err := parseHostPort(destStr); err == nil {
			nodeInfo.RealityDestHost = host
			nodeInfo.RealityDestPort = portStr
		}
	}
	if v, ok := tlsSettings["cert_path"].(string); ok {
		nodeInfo.CertPath = v
	}
	if v, ok := tlsSettings["key_path"].(string); ok {
		nodeInfo.KeyPath = v
	}
	if v, ok := tlsSettings["acme_domain"].(string); ok {
		nodeInfo.ACMEDomain = v
	}
	if v, ok := tlsSettings["acme_email"].(string); ok {
		nodeInfo.ACMEEmail = v
	}
}

func parseProtocolSettings(ps map[string]interface{}, nodeInfo *NodeInfo) {
	if v, ok := ps["obfs_type"].(string); ok && nodeInfo.ObfsType == "" {
		nodeInfo.ObfsType = v
	}
	if v, ok := ps["obfs_password"].(string); ok && nodeInfo.ObfsPass == "" {
		nodeInfo.ObfsPass = v
	}
	if v, ok := ps["masquerade"].(string); ok && nodeInfo.Masquerade == "" {
		nodeInfo.Masquerade = v
	}
	if v, ok := ps["network"].(string); ok && nodeInfo.Network == "" {
		nodeInfo.Network = v
	}
	if v, ok := ps["transport"].(string); ok && nodeInfo.Transport == "" {
		nodeInfo.Transport = v
	}
}

func parseHostPort(s string) (string, int, error) {
	host := s
	port := 443
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == ':' {
			host = s[:i]
			p, err := strconv.Atoi(s[i+1:])
			if err != nil {
				return s, 443, err
			}
			port = p
			break
		}
		if s[i] == ']' {
			break
		}
	}
	return host, port, nil
}

func parseNetworkSettings(networkSettings map[string]interface{}, nodeInfo *NodeInfo) {
	nodeInfo.NetworkSettings = make(map[string]interface{})
	for k, v := range networkSettings {
		nodeInfo.NetworkSettings[k] = v
	}
	if v, ok := networkSettings["path"].(string); ok {
		nodeInfo.WSPath = v
	}
	if v, ok := networkSettings["host"].(string); ok {
		nodeInfo.WSHost = v
	}
	if v, ok := networkSettings["service_name"].(string); ok {
		nodeInfo.GRPCServiceName = v
	}
}

func (c *XboardClient) GetUserList() ([]UserInfo, error) {
	url := c.buildURL("/user")

	resp, body, notModified, err := c.doGet(url, "user")
	if err != nil {
		return nil, fmt.Errorf("request user list: %w", err)
	}
	if notModified {
		return nil, fmt.Errorf("not modified")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	data, ok := raw["users"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing users array")
	}

	var users []UserInfo
	for _, item := range data {
		if userMap, ok := item.(map[string]interface{}); ok {
			u := UserInfo{}
			if v, ok := userMap["id"].(float64); ok {
				u.ID = int(v)
			}
			if v, ok := userMap["uuid"].(string); ok {
				u.UUID = v
			}
			if v, ok := userMap["speed_limit"].(float64); ok {
				u.SpeedLimit = v
			}
			if v, ok := userMap["device_limit"].(float64); ok {
				u.DeviceLimit = int(v)
			}
			if v, ok := userMap["dynamic_password"].(string); ok {
				u.DynamicPassword = v
			}
			if v, ok := userMap["password_expires_at"].(float64); ok {
				u.PasswordExpiresAt = int64(v)
			}
			users = append(users, u)
		}
	}

	return users, nil
}

func (c *XboardClient) ReportUserTraffic(trafficData map[string][2]int64) error {
	url := c.buildURL("/push")
	return c.doPost(url, trafficData)
}

func (c *XboardClient) ReportAliveWithIPs(payload interface{}) error {
	url := c.buildURL("/alive")
	return c.doPost(url, payload)
}

type NodeStatus struct {
	CPU         float64      `json:"cpu"`
	Mem         MemoryStatus `json:"mem"`
	Swap        MemoryStatus `json:"swap,omitempty"`
	Disk        MemoryStatus `json:"disk,omitempty"`
	NetInSpeed  float64      `json:"net_in_speed,omitempty"`
	NetOutSpeed float64      `json:"net_out_speed,omitempty"`
	Goroutines  int          `json:"goroutines,omitempty"`
	NumGC       uint32       `json:"num_gc,omitempty"`
	LastPauseMS float64      `json:"last_pause_ms,omitempty"`
}

type MemoryStatus struct {
	Total int64 `json:"total"`
	Used  int64 `json:"used"`
}

func (c *XboardClient) ReportStatus(status *NodeStatus) error {
	url := c.buildURL("/status")
	return c.doPost(url, status)
}

func (c *XboardClient) GetAliveList() (map[string]int, error) {
	url := c.buildURL("/alivelist")

	resp, body, notModified, err := c.doGet(url, "alivelist")
	if err != nil {
		return nil, fmt.Errorf("request alivelist: %w", err)
	}
	if notModified {
		return nil, fmt.Errorf("not modified")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	result := make(map[string]int)
	if data, ok := raw["data"].(map[string]interface{}); ok {
		if alive, ok := data["alive"].(map[string]interface{}); ok {
			for k, v := range alive {
				if f, ok := v.(float64); ok {
					result[k] = int(f)
				}
			}
		}
	}

	return result, nil
}
