package configgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type DeployRequest struct {
	UserID   string `json:"user_id"`
	NodeID   string `json:"node_id"`
	Protocol string `json:"protocol"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Password string `json:"password"`

	UUID string `json:"uuid,omitempty"`

	SNI            string `json:"sni,omitempty"`
	RealityPubKey  string `json:"reality_public_key,omitempty"`
	RealityShortID string `json:"reality_short_id,omitempty"`

	InboundType string `json:"inbound_type,omitempty"`
}

type SingBoxConfig struct {
	Log       *LogConfig   `json:"log,omitempty"`
	DNS       *DNSConfig   `json:"dns,omitempty"`
	Inbounds  []Inbound    `json:"inbounds"`
	Outbounds []Outbound   `json:"outbounds"`
	Route     *RouteConfig `json:"route,omitempty"`
	Stats     *StatsConfig `json:"experimental,omitempty"`
}

type LogConfig struct {
	Level     string `json:"level"`
	Timestamp bool   `json:"timestamp"`
}

type DNSConfig struct {
	Servers []DNSServer `json:"servers"`
}

type DNSServer struct {
	Tag    string `json:"tag"`
	Type   string `json:"type"`
	Server string `json:"server,omitempty"`
	Detour string `json:"detour,omitempty"`
}

type Inbound struct {
	Type          string           `json:"type"`
	Tag           string           `json:"tag,omitempty"`
	Listen        string           `json:"listen,omitempty"`
	ListenPort    int              `json:"listen_port,omitempty"`
	Address       []string         `json:"address,omitempty"`
	MTU           int              `json:"mtu,omitempty"`
	AutoRoute     bool             `json:"auto_route,omitempty"`
	StrictRoute   bool             `json:"strict_route,omitempty"`
	Users         []InboundUser    `json:"users,omitempty"`
	TLS           *InboundTLS      `json:"tls,omitempty"`
	Multiplex     *MultiplexConfig `json:"multiplex,omitempty"`
}

type InboundUser struct {
	UUID string `json:"uuid"`
	Flow string `json:"flow,omitempty"`
}

type InboundTLS struct {
	Enabled    bool     `json:"enabled"`
	ServerName string   `json:"server_name,omitempty"`
	Reality    *Reality `json:"reality,omitempty"`
}

type Reality struct {
	Enabled    bool       `json:"enabled"`
	Handshake  *Handshake `json:"handshake,omitempty"`
	PrivateKey string     `json:"private_key,omitempty"`
	ShortID    []string   `json:"short_id,omitempty"`
}

type Handshake struct {
	Server     string `json:"server"`
	ServerPort int    `json:"server_port"`
}

type MultiplexConfig struct {
	Enabled bool `json:"enabled"`
}

type Outbound struct {
	Type       string           `json:"type"`
	Tag        string           `json:"tag,omitempty"`
	Server     string           `json:"server,omitempty"`
	ServerPort int              `json:"server_port,omitempty"`
	Password   string           `json:"password,omitempty"`
	UUID       string           `json:"uuid,omitempty"`
	Flow       string           `json:"flow,omitempty"`
	TLS        *OutboundTLS     `json:"tls,omitempty"`
	Transport  *TransportConfig `json:"transport,omitempty"`
	Multiplex  *MultiplexConfig `json:"multiplex,omitempty"`
}

type OutboundTLS struct {
	Enabled    bool   `json:"enabled"`
	ServerName string `json:"server_name,omitempty"`
	Insecure   bool   `json:"insecure,omitempty"`

	Reality *OutboundReality `json:"reality,omitempty"`
}

type OutboundReality struct {
	Enabled   bool   `json:"enabled"`
	PublicKey string `json:"public_key,omitempty"`
	ShortID   string `json:"short_id,omitempty"`
}

type TransportConfig struct {
	Type    string            `json:"type"`
	Path    string            `json:"path,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type RouteConfig struct {
	Rules                 []RouteRule `json:"rules,omitempty"`
	DefaultDomainResolver string      `json:"default_domain_resolver,omitempty"`
	Final                 string      `json:"final"`
}

type RouteRule struct {
	Protocol []string `json:"protocol,omitempty"`
	Action   string   `json:"action,omitempty"`
	Outbound string   `json:"outbound,omitempty"`
}

type StatsConfig struct {
	ClashAPI *ClashAPIConfig `json:"clash_api,omitempty"`
	V2RayAPI *V2RayAPIConfig `json:"v2ray_api,omitempty"`
}

type ClashAPIConfig struct {
	Listen string `json:"listen"`
	Secret string `json:"secret"`
}

type V2RayAPIConfig struct {
	Listen string `json:"listen"`
}

type Generator struct {
	mu      sync.Mutex
	cfgPath string
	current *SingBoxConfig
	deploys map[string]*DeployRequest
}

func NewGenerator(cfgPath string) *Generator {
	return &Generator{
		cfgPath: cfgPath,
		deploys: make(map[string]*DeployRequest),
	}
}

func (g *Generator) GenerateAndWrite(req *DeployRequest) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	cfg, err := g.generate(req)
	if err != nil {
		return fmt.Errorf("generate config: %w", err)
	}

	g.deploys[req.UserID] = req
	g.current = cfg

	if err := g.writeConfig(cfg); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	return nil
}

func (g *Generator) generate(req *DeployRequest) (*SingBoxConfig, error) {
	cfg := &SingBoxConfig{
		Log: &LogConfig{
			Level:     "info",
			Timestamp: true,
		},
		DNS: &DNSConfig{
			Servers: []DNSServer{
				{Tag: "google", Type: "tls", Server: "8.8.8.8"},
				{Tag: "local", Type: "udp", Server: "223.5.5.5"},
			},
		},
		Stats: &StatsConfig{
			ClashAPI: &ClashAPIConfig{
				Listen: "0.0.0.0:9090",
				Secret: "node-agent-stats",
			},
			V2RayAPI: &V2RayAPIConfig{
				Listen: "127.0.0.1:10001",
			},
		},
		Route: &RouteConfig{
			Rules: []RouteRule{
				{
					Action: "sniff",
				},
				{
					Protocol: []string{"dns"},
					Action:   "hijack-dns",
				},
			},
			DefaultDomainResolver: "google",
			Final:                 "proxy",
		},
	}

	inboundType := req.InboundType
	if inboundType == "" {
		inboundType = "tun"
	}

	switch inboundType {
	case "tun":
		cfg.Inbounds = append(cfg.Inbounds, Inbound{
			Type:      "tun",
			Tag:       "tun-in",
			Address:   []string{"172.19.0.1/30"},
			MTU:       9000,
			AutoRoute: true,
			StrictRoute: true,
		})
	case "mixed":
		cfg.Inbounds = append(cfg.Inbounds, Inbound{
			Type:       "mixed",
			Tag:        "mixed-in",
			Listen:     "0.0.0.0",
			ListenPort: 2080,
		})
	}

	outbound, err := g.generateOutbound(req)
	if err != nil {
		return nil, err
	}
	cfg.Outbounds = append(cfg.Outbounds, *outbound)

	cfg.Outbounds = append(cfg.Outbounds, Outbound{
		Type: "direct",
		Tag:  "direct",
	})

	return cfg, nil
}

func (g *Generator) generateOutbound(req *DeployRequest) (*Outbound, error) {
	switch req.Protocol {
	case "hysteria2":
		return &Outbound{
			Type:       "hysteria2",
			Tag:        "proxy",
			Server:     req.Server,
			ServerPort: req.Port,
			Password:   req.Password,
			TLS: &OutboundTLS{
				Enabled:    true,
				ServerName: req.SNI,
				Insecure:   req.SNI == "",
			},
		}, nil

	case "vless":
		uuid := req.UUID
		if uuid == "" {
			uuid = req.Password
		}
		return &Outbound{
			Type:       "vless",
			Tag:        "proxy",
			Server:     req.Server,
			ServerPort: req.Port,
			UUID:       uuid,
			Flow:       "xtls-rprx-vision",
			TLS: &OutboundTLS{
				Enabled:    true,
				ServerName: req.SNI,
				Insecure:   false,
			},
		}, nil

	case "reality":
		uuid := req.UUID
		if uuid == "" {
			uuid = req.Password
		}
		return &Outbound{
			Type:       "vless",
			Tag:        "proxy",
			Server:     req.Server,
			ServerPort: req.Port,
			UUID:       uuid,
			Flow:       "xtls-rprx-vision",
			TLS: &OutboundTLS{
				Enabled:    true,
				ServerName: req.SNI,
				Reality: &OutboundReality{
					Enabled:   true,
					PublicKey: req.RealityPubKey,
					ShortID:   req.RealityShortID,
				},
			},
		}, nil

	default:
		return nil, fmt.Errorf("unsupported protocol: %s", req.Protocol)
	}
}

func (g *Generator) writeConfig(cfg *SingBoxConfig) error {
	dir := filepath.Dir(g.cfgPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create config dir: %w", err)
		}
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	tmpFile := g.cfgPath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0644); err != nil {
		return fmt.Errorf("write temp config: %w", err)
	}

	if err := os.Rename(tmpFile, g.cfgPath); err != nil {
		return fmt.Errorf("rename config: %w", err)
	}

	return nil
}

func (g *Generator) GetCurrent() *SingBoxConfig {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.current
}

func (g *Generator) GetDeploys() map[string]*DeployRequest {
	g.mu.Lock()
	defer g.mu.Unlock()
	result := make(map[string]*DeployRequest, len(g.deploys))
	for k, v := range g.deploys {
		result[k] = v
	}
	return result
}

func (g *Generator) RemoveDeploy(userID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	delete(g.deploys, userID)

	if len(g.deploys) == 0 {
		defaultCfg := g.generateDefault()
		g.current = defaultCfg
		return g.writeConfig(defaultCfg)
	}

	return nil
}

func (g *Generator) generateDefault() *SingBoxConfig {
	return &SingBoxConfig{
		Log: &LogConfig{
			Level:     "info",
			Timestamp: true,
		},
		Inbounds: []Inbound{
			{
				Type:      "tun",
				Tag:       "tun-in",
				Address:   []string{"172.19.0.1/30"},
				MTU:       9000,
				AutoRoute: true,
				StrictRoute: true,
			},
		},
		Outbounds: []Outbound{
			{Type: "direct", Tag: "direct"},
		},
		Route: &RouteConfig{
			DefaultDomainResolver: "google",
			Final:                 "direct",
		},
	}
}
