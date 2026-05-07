package xboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type XboardClient struct {
	apiHost  string
	apiKey   string
	nodeID   int
	nodeType string
	timeout  time.Duration
	client   *http.Client
}

func NewXboardClient(apiHost, apiKey string, nodeID int, nodeType string, timeout int) *XboardClient {
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
	}
}

func (c *XboardClient) GetNodeInfo() (*NodeInfo, error) {
	url := fmt.Sprintf("%s/api/v1/server/UniProxy/config?node_id=%d&node_type=%s&token=%s",
		c.apiHost, c.nodeID, c.nodeType, c.apiKey)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request config: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == 304 {
		return nil, fmt.Errorf("not modified")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	data, ok := raw["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing data field")
	}

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

	if tlsSettings, ok := data["tls_settings"].(map[string]interface{}); ok {
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
	}

	if v, ok := data["speed_limit"].(float64); ok {
		nodeInfo.SpeedLimit = int64(v)
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

func (c *XboardClient) GetUserList() ([]UserInfo, error) {
	url := fmt.Sprintf("%s/api/v1/server/UniProxy/user?node_id=%d&node_type=%s&token=%s",
		c.apiHost, c.nodeID, c.nodeType, c.apiKey)

	resp, err := c.client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("request user list: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode == 304 {
		return nil, fmt.Errorf("not modified")
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("server returned %d: %s", resp.StatusCode, string(body))
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	data, ok := raw["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format: missing data array")
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
			users = append(users, u)
		}
	}

	return users, nil
}

func (c *XboardClient) ReportUserTraffic(trafficData map[string][2]int64) error {
	url := fmt.Sprintf("%s/api/v1/server/UniProxy/push?node_id=%d&node_type=%s&token=%s",
		c.apiHost, c.nodeID, c.nodeType, c.apiKey)

	body, err := json.Marshal(trafficData)
	if err != nil {
		return fmt.Errorf("marshal traffic data: %w", err)
	}

	resp, err := c.client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("push traffic: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, string(respBody))
	}

	return nil
}
