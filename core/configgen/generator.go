package configgen

import (
	"crypto/ecdh"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

	SNI string `json:"sni,omitempty"`

	ObfsType string `json:"obfs_type,omitempty"`
	ObfsPass string `json:"obfs_password,omitempty"`

	UpMbps   int `json:"up_mbps,omitempty"`
	DownMbps int `json:"down_mbps,omitempty"`

	TLSCertPath string `json:"tls_cert_path,omitempty"`
	TLSKeyPath  string `json:"tls_key_path,omitempty"`

	ACMEDomain string `json:"acme_domain,omitempty"`
	ACMEEmail  string `json:"acme_email,omitempty"`

	RealityPrivateKey string `json:"reality_private_key,omitempty"`
	RealityPublicKey  string `json:"reality_public_key,omitempty"`
	RealityShortID    string `json:"reality_short_id,omitempty"`
	RealityDest       string `json:"reality_dest,omitempty"`
	RealityDestPort   int    `json:"reality_dest_port,omitempty"`
}

type ClientConfigResult struct {
	SingBoxConfig string `json:"singbox_config"`
	URI           string `json:"uri"`
	Protocol      string `json:"protocol"`
	Server        string `json:"server"`
	Port          int    `json:"port"`
	Insecure      bool   `json:"insecure"`
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
	Type                  string           `json:"type"`
	Tag                   string           `json:"tag,omitempty"`
	Listen                string           `json:"listen,omitempty"`
	ListenPort            int              `json:"listen_port,omitempty"`
	Users                 []InboundUser    `json:"users,omitempty"`
	TLS                   *InboundTLS      `json:"tls,omitempty"`
	Transport             *TransportConfig `json:"transport,omitempty"`
	Obfs                  *ObfsConfig      `json:"obfs,omitempty"`
	Multiplex             *MultiplexConfig `json:"multiplex,omitempty"`
	UpMbps                int              `json:"up_mbps,omitempty"`
	DownMbps              int              `json:"down_mbps,omitempty"`
	IgnoreClientBandwidth bool             `json:"ignore_client_bandwidth,omitempty"`
	Masquerade            string           `json:"masquerade,omitempty"`
	BBRProfile            string           `json:"bbr_profile,omitempty"`
	InitialPacketSize     int              `json:"initial_packet_size,omitempty"`
	DisablePathMTUDisc    bool             `json:"disable_path_mtu_discovery,omitempty"`
	Address               []string         `json:"address,omitempty"`
	MTU                   int              `json:"mtu,omitempty"`
	AutoRoute             bool             `json:"auto_route,omitempty"`
	StrictRoute           bool             `json:"strict_route,omitempty"`
}

type InboundUser struct {
	Name     string `json:"name,omitempty"`
	UUID     string `json:"uuid,omitempty"`
	Password string `json:"password,omitempty"`
	Flow     string `json:"flow,omitempty"`
}

type InboundTLS struct {
	Enabled         bool        `json:"enabled"`
	ServerName      string      `json:"server_name,omitempty"`
	CertificatePath string      `json:"certificate_path,omitempty"`
	KeyPath         string      `json:"key_path,omitempty"`
	ALPN            []string    `json:"alpn,omitempty"`
	ACME            *ACMEConfig `json:"acme,omitempty"`
	Reality         *Reality    `json:"reality,omitempty"`
}

type ACMEConfig struct {
	Domain string `json:"domain"`
	Email  string `json:"email,omitempty"`
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

type ObfsConfig struct {
	Type     string `json:"type"`
	Password string `json:"password,omitempty"`
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
	UpMbps     int              `json:"up_mbps,omitempty"`
	DownMbps   int              `json:"down_mbps,omitempty"`
	TLS        *OutboundTLS     `json:"tls,omitempty"`
	Transport  *TransportConfig `json:"transport,omitempty"`
	Obfs       *ObfsConfig      `json:"obfs,omitempty"`
	Multiplex  *MultiplexConfig `json:"multiplex,omitempty"`
}

type OutboundTLS struct {
	Enabled    bool             `json:"enabled"`
	ServerName string           `json:"server_name,omitempty"`
	Insecure   bool             `json:"insecure,omitempty"`
	ALPN       []string         `json:"alpn,omitempty"`
	Reality    *OutboundReality `json:"reality,omitempty"`
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
	ExternalController string `json:"external_controller"`
	Secret             string `json:"secret,omitempty"`
}

type V2RayAPIConfig struct {
	Listen string         `json:"listen"`
	Stats  *V2RayAPIStats `json:"stats,omitempty"`
}

type V2RayAPIStats struct {
	Enabled   bool     `json:"enabled"`
	Inbounds  []string `json:"inbounds,omitempty"`
	Outbounds []string `json:"outbounds,omitempty"`
	Users     []string `json:"users,omitempty"`
}

type Generator struct {
	mu             sync.Mutex
	cfgPath        string
	current        *SingBoxConfig
	deploys        map[string]*DeployRequest
	clashAPIAddr   string
	clashAPISecret string
	v2rayAPIAddr   string
	singboxVersion string
}

func NewGenerator(cfgPath string) *Generator {
	return &Generator{
		cfgPath:        cfgPath,
		deploys:        make(map[string]*DeployRequest),
		clashAPIAddr:   "0.0.0.0:9090",
		clashAPISecret: "node-agent-stats",
		v2rayAPIAddr:   "127.0.0.1:10001",
	}
}

func (g *Generator) SetClashAPI(addr, secret string) {
	g.clashAPIAddr = addr
	g.clashAPISecret = secret
}

func (g *Generator) SetV2RayAPI(addr string) {
	g.v2rayAPIAddr = addr
}

func (g *Generator) SetSingboxVersion(ver string) {
	g.singboxVersion = ver
}

func (g *Generator) supportsInitialPacketSize() bool {
	if g.singboxVersion == "" || g.singboxVersion == "unknown" {
		return false
	}
	re := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
	matches := re.FindStringSubmatch(g.singboxVersion)
	if len(matches) < 4 {
		return false
	}
	major, _ := strconv.Atoi(matches[1])
	minor, _ := strconv.Atoi(matches[2])
	if major > 1 {
		return true
	}
	if major == 1 && minor >= 10 {
		return true
	}
	return false
}

func (g *Generator) GenerateAndWrite(req *DeployRequest) (*ClientConfigResult, error) {
	g.mu.Lock()
	defer g.mu.Unlock()

	g.deploys[req.UserID] = req

	cfg, clientCfg, err := g.rebuildConfig()
	if err != nil {
		return nil, fmt.Errorf("generate config: %w", err)
	}

	g.current = cfg

	if err := g.writeConfig(cfg); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	return clientCfg, nil
}

func (g *Generator) GetClientConfig(userID string) (*ClientConfigResult, error) {
	g.mu.Lock()
	req, ok := g.deploys[userID]
	g.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("deploy not found for user: %s", userID)
	}

	_, clientCfg, err := g.generateClientOnly(req)
	if err != nil {
		return nil, err
	}
	return clientCfg, nil
}

func (g *Generator) rebuildConfig() (*SingBoxConfig, *ClientConfigResult, error) {
	if len(g.deploys) == 0 {
		defaultCfg := g.generateDefault()
		return defaultCfg, nil, nil
	}

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
				ExternalController: g.clashAPIAddr,
				Secret:             g.clashAPISecret,
			},
			V2RayAPI: &V2RayAPIConfig{
				Listen: g.v2rayAPIAddr,
				Stats: &V2RayAPIStats{
					Enabled:   true,
					Outbounds: []string{"direct"},
				},
			},
		},
		Route: &RouteConfig{
			Rules: []RouteRule{
				{Action: "sniff"},
				{Protocol: []string{"dns"}, Action: "hijack-dns"},
			},
			DefaultDomainResolver: "google",
			Final:                 "direct",
		},
	}

	cfg.Outbounds = append(cfg.Outbounds, Outbound{
		Type: "direct",
		Tag:  "direct",
	})

	type inboundKey struct {
		protocol string
		port     int
	}
	inboundGroups := make(map[inboundKey][]*DeployRequest)

	var lastReq *DeployRequest
	for _, req := range g.deploys {
		key := inboundKey{protocol: req.Protocol, port: req.Port}
		inboundGroups[key] = append(inboundGroups[key], req)
		lastReq = req
	}

	for _, reqs := range inboundGroups {
		firstReq := reqs[0]

		sni := firstReq.SNI
		if sni == "" {
			sni = firstReq.Server
		}

		certPath := firstReq.TLSCertPath
		keyPath := firstReq.TLSKeyPath
		useACME := firstReq.ACMEDomain != ""

		if firstReq.Protocol != "reality" && !useACME && (certPath == "" || keyPath == "") {
			dir := filepath.Dir(g.cfgPath)
			certPath = filepath.Join(dir, "self-signed-cert.pem")
			keyPath = filepath.Join(dir, "self-signed-key.pem")
			if err := GenerateSelfSignedCert(certPath, keyPath, sni); err != nil {
				return nil, nil, fmt.Errorf("generate self-signed cert: %w", err)
			}
		}

		inbound, err := g.generateInbound(firstReq, certPath, keyPath, useACME)
		if err != nil {
			return nil, nil, err
		}

		if len(reqs) > 1 {
			existingUsers := make(map[string]bool)
			for _, u := range inbound.Users {
				existingUsers[u.Name] = true
			}
			for _, req := range reqs[1:] {
				if !existingUsers[req.UserID] {
					switch req.Protocol {
					case "hysteria2":
						inbound.Users = append(inbound.Users, InboundUser{
							Name:     req.UserID,
							Password: req.Password,
						})
					case "vless", "reality":
						uuid := req.UUID
						if uuid == "" {
							uuid = req.Password
						}
						inbound.Users = append(inbound.Users, InboundUser{
							Name: req.UserID,
							UUID: uuid,
							Flow: "xtls-rprx-vision",
						})
					case "trojan":
						inbound.Users = append(inbound.Users, InboundUser{
							Name:     req.UserID,
							Password: req.Password,
						})
					}
					existingUsers[req.UserID] = true
				}
			}
		}

		cfg.Inbounds = append(cfg.Inbounds, *inbound)
	}

	var allUserIDs []string
	var inboundTags []string
	for _, req := range g.deploys {
		allUserIDs = append(allUserIDs, req.UserID)
	}
	for _, ib := range cfg.Inbounds {
		if ib.Tag != "" {
			inboundTags = append(inboundTags, ib.Tag)
		}
	}
	cfg.Stats.V2RayAPI.Stats.Users = allUserIDs
	cfg.Stats.V2RayAPI.Stats.Inbounds = inboundTags

	var lastClientCfg *ClientConfigResult
	if lastReq != nil {
		useSelfSigned := lastReq.Protocol != "reality" && lastReq.ACMEDomain == "" && (lastReq.TLSCertPath == "" || lastReq.TLSKeyPath == "")
		clientCfg, err := g.buildClientConfig(lastReq, useSelfSigned)
		if err != nil {
			return nil, nil, fmt.Errorf("build client config: %w", err)
		}
		lastClientCfg = clientCfg
	}

	return cfg, lastClientCfg, nil
}

func (g *Generator) generate(req *DeployRequest) (*SingBoxConfig, *ClientConfigResult, error) {
	sni := req.SNI
	if sni == "" {
		sni = req.Server
	}

	certPath := req.TLSCertPath
	keyPath := req.TLSKeyPath
	useACME := req.ACMEDomain != ""
	useSelfSigned := false

	if req.Protocol != "reality" && !useACME && (certPath == "" || keyPath == "") {
		dir := filepath.Dir(g.cfgPath)
		certPath = filepath.Join(dir, "self-signed-cert.pem")
		keyPath = filepath.Join(dir, "self-signed-key.pem")

		if err := GenerateSelfSignedCert(certPath, keyPath, sni); err != nil {
			return nil, nil, fmt.Errorf("generate self-signed cert: %w", err)
		}
		useSelfSigned = true
	}

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
				ExternalController: g.clashAPIAddr,
				Secret:             g.clashAPISecret,
			},
			V2RayAPI: &V2RayAPIConfig{
				Listen: g.v2rayAPIAddr,
				Stats: &V2RayAPIStats{
					Enabled:   true,
					Outbounds: []string{"direct"},
				},
			},
		},
		Route: &RouteConfig{
			Rules: []RouteRule{
				{Action: "sniff"},
				{Protocol: []string{"dns"}, Action: "hijack-dns"},
			},
			DefaultDomainResolver: "google",
			Final:                 "direct",
		},
	}

	inbound, err := g.generateInbound(req, certPath, keyPath, useACME)
	if err != nil {
		return nil, nil, err
	}
	cfg.Inbounds = append(cfg.Inbounds, *inbound)

	inboundTags := []string{}
	for _, ib := range cfg.Inbounds {
		if ib.Tag != "" {
			inboundTags = append(inboundTags, ib.Tag)
		}
	}
	cfg.Stats.V2RayAPI.Stats.Users = []string{req.UserID}
	cfg.Stats.V2RayAPI.Stats.Inbounds = inboundTags

	cfg.Outbounds = append(cfg.Outbounds, Outbound{
		Type: "direct",
		Tag:  "direct",
	})

	clientCfg, err := g.buildClientConfig(req, useSelfSigned)
	if err != nil {
		return nil, nil, fmt.Errorf("build client config: %w", err)
	}

	return cfg, clientCfg, nil
}

func (g *Generator) generateClientOnly(req *DeployRequest) (*SingBoxConfig, *ClientConfigResult, error) {
	useSelfSigned := req.Protocol != "reality" && req.ACMEDomain == "" && (req.TLSCertPath == "" || req.TLSKeyPath == "")
	clientCfg, err := g.buildClientConfig(req, useSelfSigned)
	if err != nil {
		return nil, nil, err
	}
	return nil, clientCfg, nil
}

func (g *Generator) generateInbound(req *DeployRequest, certPath, keyPath string, useACME bool) (*Inbound, error) {
	switch req.Protocol {
	case "hysteria2":
		return g.generateHysteria2Inbound(req, certPath, keyPath, useACME)
	case "vless":
		return g.generateVLESSInbound(req, certPath, keyPath, useACME)
	case "reality":
		return g.generateRealityInbound(req)
	case "trojan":
		return g.generateTrojanInbound(req, certPath, keyPath, useACME)
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", req.Protocol)
	}
}

func (g *Generator) generateHysteria2Inbound(req *DeployRequest, certPath, keyPath string, useACME bool) (*Inbound, error) {
	sni := req.SNI
	if sni == "" {
		sni = req.Server
	}

	inbound := &Inbound{
		Type:       "hysteria2",
		Tag:        "hysteria2-in",
		Listen:     "0.0.0.0",
		ListenPort: req.Port,
		Users:      []InboundUser{{Name: req.UserID, Password: req.Password}},
		Masquerade: "https://www.bing.com",
		TLS:        &InboundTLS{Enabled: true, ServerName: sni, ALPN: []string{"h3"}},
	}

	if g.supportsInitialPacketSize() {
		inbound.InitialPacketSize = 1400
	}

	if useACME && req.ACMEDomain != "" {
		inbound.TLS.ACME = &ACMEConfig{
			Domain: req.ACMEDomain,
			Email:  req.ACMEEmail,
		}
	} else if certPath != "" && keyPath != "" {
		inbound.TLS.CertificatePath = certPath
		inbound.TLS.KeyPath = keyPath
	}

	if req.ObfsType != "" {
		inbound.Obfs = &ObfsConfig{
			Type:     req.ObfsType,
			Password: req.ObfsPass,
		}
	}

	if req.UpMbps > 0 || req.DownMbps > 0 {
		inbound.UpMbps = req.UpMbps
		inbound.DownMbps = req.DownMbps
		inbound.IgnoreClientBandwidth = true
	}

	return inbound, nil
}

func (g *Generator) generateVLESSInbound(req *DeployRequest, certPath, keyPath string, useACME bool) (*Inbound, error) {
	uuid := req.UUID
	if uuid == "" {
		uuid = req.Password
	}

	sni := req.SNI
	if sni == "" {
		sni = req.Server
	}

	inbound := &Inbound{
		Type:       "vless",
		Tag:        "vless-in",
		Listen:     "0.0.0.0",
		ListenPort: req.Port,
		Users: []InboundUser{
			{Name: req.UserID, UUID: uuid, Flow: "xtls-rprx-vision"},
		},
		TLS: &InboundTLS{
			Enabled:    true,
			ServerName: sni,
		},
	}

	if useACME && req.ACMEDomain != "" {
		inbound.TLS.ACME = &ACMEConfig{
			Domain: req.ACMEDomain,
			Email:  req.ACMEEmail,
		}
	} else if certPath != "" && keyPath != "" {
		inbound.TLS.CertificatePath = certPath
		inbound.TLS.KeyPath = keyPath
	}

	return inbound, nil
}

func (g *Generator) generateRealityInbound(req *DeployRequest) (*Inbound, error) {
	uuid := req.UUID
	if uuid == "" {
		uuid = req.Password
	}

	dest := req.RealityDest
	if dest == "" {
		dest = "www.microsoft.com"
	}
	destPort := req.RealityDestPort
	if destPort == 0 {
		destPort = 443
	}

	shortIDs := []string{""}
	if req.RealityShortID != "" {
		shortIDs = []string{req.RealityShortID}
	}

	privateKey := req.RealityPrivateKey
	if privateKey == "" {
		return nil, fmt.Errorf("reality_private_key is required for reality protocol")
	}

	sni := req.SNI
	if sni == "" {
		sni = dest
	}

	inbound := &Inbound{
		Type:       "vless",
		Tag:        "reality-in",
		Listen:     "0.0.0.0",
		ListenPort: req.Port,
		Users: []InboundUser{
			{Name: req.UserID, UUID: uuid, Flow: "xtls-rprx-vision"},
		},
		TLS: &InboundTLS{
			Enabled:    true,
			ServerName: sni,
			Reality: &Reality{
				Enabled:    true,
				PrivateKey: privateKey,
				ShortID:    shortIDs,
				Handshake: &Handshake{
					Server:     dest,
					ServerPort: destPort,
				},
			},
		},
	}

	return inbound, nil
}

func (g *Generator) generateTrojanInbound(req *DeployRequest, certPath, keyPath string, useACME bool) (*Inbound, error) {
	sni := req.SNI
	if sni == "" {
		sni = req.Server
	}

	inbound := &Inbound{
		Type:       "trojan",
		Tag:        "trojan-in",
		Listen:     "0.0.0.0",
		ListenPort: req.Port,
		Users:      []InboundUser{{Name: req.UserID, Password: req.Password}},
		TLS: &InboundTLS{
			Enabled:    true,
			ServerName: sni,
		},
	}

	if useACME && req.ACMEDomain != "" {
		inbound.TLS.ACME = &ACMEConfig{
			Domain: req.ACMEDomain,
			Email:  req.ACMEEmail,
		}
	} else if certPath != "" && keyPath != "" {
		inbound.TLS.CertificatePath = certPath
		inbound.TLS.KeyPath = keyPath
	}

	if req.ObfsType != "" {
		inbound.Transport = &TransportConfig{
			Type:    "ws",
			Path:    "/" + req.ObfsPass,
			Headers: map[string]string{},
		}
	}

	return inbound, nil
}

func (g *Generator) buildClientConfig(req *DeployRequest, insecure bool) (*ClientConfigResult, error) {
	sni := req.SNI
	if sni == "" {
		sni = req.Server
	}

	server := req.Server
	if server == "0.0.0.0" || server == "::" || server == "" {
		server = "YOUR_SERVER_IP"
	}

	var clientOutbound Outbound
	var uri string

	switch req.Protocol {
	case "hysteria2":
		clientOutbound = Outbound{
			Type:       "hysteria2",
			Tag:        "proxy",
			Server:     server,
			ServerPort: req.Port,
			Password:   req.Password,
			TLS: &OutboundTLS{
				Enabled:    true,
				ServerName: sni,
				Insecure:   insecure,
				ALPN:       []string{"h3"},
			},
		}
		if req.UpMbps > 0 {
			clientOutbound.UpMbps = req.UpMbps
		}
		if req.DownMbps > 0 {
			clientOutbound.DownMbps = req.DownMbps
		}
		if req.ObfsType != "" {
			clientOutbound.Obfs = &ObfsConfig{
				Type:     req.ObfsType,
				Password: req.ObfsPass,
			}
		}
		uri = g.buildHysteria2URI(req, server, sni, insecure)

	case "vless":
		uuid := req.UUID
		if uuid == "" {
			uuid = req.Password
		}
		clientOutbound = Outbound{
			Type:       "vless",
			Tag:        "proxy",
			Server:     server,
			ServerPort: req.Port,
			UUID:       uuid,
			Flow:       "xtls-rprx-vision",
			TLS: &OutboundTLS{
				Enabled:    true,
				ServerName: sni,
				Insecure:   insecure,
			},
		}
		uri = g.buildVLESSURI(req, server, sni, uuid, insecure)

	case "reality":
		uuid := req.UUID
		if uuid == "" {
			uuid = req.Password
		}
		publicKey := req.RealityPublicKey
		if publicKey == "" && req.RealityPrivateKey != "" {
			derived, err := deriveRealityPublicKey(req.RealityPrivateKey)
			if err == nil {
				publicKey = derived
			}
		}
		shortID := req.RealityShortID
		clientOutbound = Outbound{
			Type:       "vless",
			Tag:        "proxy",
			Server:     server,
			ServerPort: req.Port,
			UUID:       uuid,
			Flow:       "xtls-rprx-vision",
			TLS: &OutboundTLS{
				Enabled:    true,
				ServerName: sni,
				Reality: &OutboundReality{
					Enabled:   true,
					PublicKey: publicKey,
					ShortID:   shortID,
				},
			},
		}
		uri = g.buildRealityURI(req, server, sni, uuid, publicKey, shortID)

	case "trojan":
		clientOutbound = Outbound{
			Type:       "trojan",
			Tag:        "proxy",
			Server:     server,
			ServerPort: req.Port,
			Password:   req.Password,
			TLS: &OutboundTLS{
				Enabled:    true,
				ServerName: sni,
				Insecure:   insecure,
			},
		}
		if req.ObfsType != "" {
			clientOutbound.Transport = &TransportConfig{
				Type: "ws",
				Path: "/" + req.ObfsPass,
			}
		}
		uri = g.buildTrojanURI(req, server, sni, insecure)
	}

	clientCfg := &SingBoxConfig{
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
		Inbounds: []Inbound{
			{
				Type:        "tun",
				Tag:         "tun-in",
				Address:     []string{"172.19.0.1/30"},
				MTU:         4160,
				AutoRoute:   true,
				StrictRoute: true,
			},
		},
		Outbounds: []Outbound{
			clientOutbound,
			{Type: "direct", Tag: "direct"},
		},
		Route: &RouteConfig{
			Rules: []RouteRule{
				{Action: "sniff"},
				{Protocol: []string{"dns"}, Action: "hijack-dns"},
			},
			DefaultDomainResolver: "google",
			Final:                 "proxy",
		},
	}

	clientJSON, err := json.MarshalIndent(clientCfg, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal client config: %w", err)
	}

	return &ClientConfigResult{
		SingBoxConfig: string(clientJSON),
		URI:           uri,
		Protocol:      req.Protocol,
		Server:        server,
		Port:          req.Port,
		Insecure:      insecure,
	}, nil
}

func (g *Generator) buildHysteria2URI(req *DeployRequest, server, sni string, insecure bool) string {
	u := url.URL{
		Scheme: "hysteria2",
		User:   url.User(req.Password),
		Host:   fmt.Sprintf("%s:%d", server, req.Port),
	}
	q := u.Query()
	q.Set("sni", sni)
	if insecure {
		q.Set("insecure", "1")
	}
	if req.ObfsType != "" {
		q.Set("obfs", req.ObfsType)
		if req.ObfsPass != "" {
			q.Set("obfs-password", req.ObfsPass)
		}
	}
	u.RawQuery = q.Encode()
	u.Fragment = fmt.Sprintf("%s-%s", req.Protocol, req.NodeID)
	return u.String()
}

func (g *Generator) buildVLESSURI(req *DeployRequest, server, sni, uuid string, insecure bool) string {
	u := url.URL{
		Scheme: "vless",
		User:   url.User(uuid),
		Host:   fmt.Sprintf("%s:%d", server, req.Port),
	}
	q := u.Query()
	q.Set("encryption", "none")
	q.Set("flow", "xtls-rprx-vision")
	q.Set("security", "tls")
	q.Set("sni", sni)
	q.Set("type", "tcp")
	q.Set("fp", "chrome")
	if insecure {
		q.Set("allowInsecure", "1")
	}
	u.RawQuery = q.Encode()
	u.Fragment = fmt.Sprintf("vless-%s", req.NodeID)
	return u.String()
}

func (g *Generator) buildRealityURI(req *DeployRequest, server, sni, uuid, publicKey, shortID string) string {
	u := url.URL{
		Scheme: "vless",
		User:   url.User(uuid),
		Host:   fmt.Sprintf("%s:%d", server, req.Port),
	}
	q := u.Query()
	q.Set("encryption", "none")
	q.Set("flow", "xtls-rprx-vision")
	q.Set("security", "reality")
	q.Set("sni", sni)
	q.Set("type", "tcp")
	q.Set("fp", "chrome")
	if publicKey != "" {
		q.Set("pbk", publicKey)
	}
	if shortID != "" {
		q.Set("sid", shortID)
	}
	u.RawQuery = q.Encode()
	u.Fragment = fmt.Sprintf("reality-%s", req.NodeID)
	return u.String()
}

func (g *Generator) buildTrojanURI(req *DeployRequest, server, sni string, insecure bool) string {
	u := url.URL{
		Scheme: "trojan",
		User:   url.User(req.Password),
		Host:   fmt.Sprintf("%s:%d", server, req.Port),
	}
	q := u.Query()
	q.Set("security", "tls")
	q.Set("sni", sni)
	q.Set("type", "tcp")
	q.Set("fp", "chrome")
	if insecure {
		q.Set("allowInsecure", "1")
	}
	if req.ObfsType != "" {
		q.Set("type", "ws")
		q.Set("path", "/"+req.ObfsPass)
	}
	u.RawQuery = q.Encode()
	u.Fragment = fmt.Sprintf("trojan-%s", req.NodeID)
	return u.String()
}

func deriveRealityPublicKey(privateKeyB64 string) (string, error) {
	privateKeyBytes, err := base64.StdEncoding.DecodeString(privateKeyB64)
	if err != nil {
		return "", fmt.Errorf("decode private key: %w", err)
	}

	priv, err := ecdh.X25519().NewPrivateKey(privateKeyBytes)
	if err != nil {
		return "", fmt.Errorf("create private key: %w", err)
	}

	publicKeyBytes := priv.PublicKey().Bytes()
	return base64.StdEncoding.EncodeToString(publicKeyBytes), nil
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

	cfg, _, err := g.rebuildConfig()
	if err != nil {
		return fmt.Errorf("rebuild config after remove: %w", err)
	}
	g.current = cfg
	return g.writeConfig(cfg)
}

func (g *Generator) generateDefault() *SingBoxConfig {
	return &SingBoxConfig{
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
		Inbounds:  []Inbound{},
		Outbounds: []Outbound{{Type: "direct", Tag: "direct"}},
		Route: &RouteConfig{
			Rules: []RouteRule{
				{Action: "sniff"},
				{Protocol: []string{"dns"}, Action: "hijack-dns"},
			},
			DefaultDomainResolver: "google",
			Final:                 "direct",
		},
	}
}
