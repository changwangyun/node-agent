package xboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type HandshakeResponse struct {
	WebSocket struct {
		Enabled bool   `json:"enabled"`
		WsURL   string `json:"ws_url"`
	} `json:"websocket"`
}

type WSMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type WSConfigUpdate struct {
	NodeInfo *NodeInfo  `json:"node_info,omitempty"`
	Users    []UserInfo `json:"users,omitempty"`
}

type WSClient struct {
	apiHost  string
	apiKey   string
	nodeID   string
	nodeType string

	conn      *websocket.Conn
	mu        sync.Mutex
	connected bool
	stopCh    chan struct{}

	onConfigUpdate func(*NodeInfo, []UserInfo)
	onDisconnected func()

	reconnectDelay time.Duration
	maxDelay       time.Duration

	unsupported  bool
	notifiedOnce bool
	wsURL        string
}

func NewWSClient(apiHost, apiKey, nodeID, nodeType string) *WSClient {
	return &WSClient{
		apiHost:        apiHost,
		apiKey:         apiKey,
		nodeID:         nodeID,
		nodeType:       nodeType,
		stopCh:         make(chan struct{}),
		reconnectDelay: 2 * time.Second,
		maxDelay:       60 * time.Second,
	}
}

func (w *WSClient) SetOnConfigUpdate(fn func(*NodeInfo, []UserInfo)) {
	w.onConfigUpdate = fn
}

func (w *WSClient) SetOnDisconnected(fn func()) {
	w.onDisconnected = fn
}

func (w *WSClient) Connect() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.connected || w.unsupported {
		return nil
	}

	if w.wsURL == "" {
		hs, err := w.doHandshake()
		if err != nil {
			if !w.notifiedOnce {
				log.Printf("[xboard] WebSocket handshake failed: %v, trying direct connection", err)
				w.notifiedOnce = true
			}
			w.wsURL = normalizeWSURL(w.apiHost)
		} else if !hs.WebSocket.Enabled || hs.WebSocket.WsURL == "" {
			if !w.notifiedOnce {
				log.Printf("[xboard] WebSocket not enabled on panel, trying direct connection")
				w.notifiedOnce = true
			}
			w.wsURL = normalizeWSURL(w.apiHost)
		} else {
			w.wsURL = normalizeWSURL(hs.WebSocket.WsURL)
			log.Printf("[xboard] WebSocket URL from handshake: %s", w.wsURL)
		}
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+w.apiKey)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, resp, err := dialer.Dial(w.wsURL, header)
	if err != nil {
		if resp != nil {
			statusCode := resp.StatusCode
			resp.Body.Close()
			if statusCode == 404 || statusCode == 405 || statusCode == 501 {
				w.unsupported = true
				if !w.notifiedOnce {
					log.Printf("[xboard] WebSocket not supported by panel (HTTP %d), using REST polling only", statusCode)
					w.notifiedOnce = true
				}
				return nil
			}
		}

		fallbackURL := normalizeWSURL(w.apiHost)
		if fallbackURL != w.wsURL {
			log.Printf("[xboard] ws dial %s failed: %v, trying fallback %s", w.wsURL, err, fallbackURL)
			conn2, resp2, err2 := dialer.Dial(fallbackURL, header)
			if err2 == nil {
				if resp2 != nil {
					resp2.Body.Close()
				}
				w.wsURL = fallbackURL
				conn = conn2
				goto connected
			}
			if resp2 != nil {
				resp2.Body.Close()
			}
			log.Printf("[xboard] ws fallback also failed: %v", err2)
		}

		return fmt.Errorf("dial ws %s: %w", w.wsURL, err)
	}

connected:
	w.conn = conn
	w.connected = true

	go w.readLoop()
	go w.pingLoop()

	log.Printf("[xboard-ws] connected to %s", w.wsURL)
	return nil
}

func (w *WSClient) doHandshake() (*HandshakeResponse, error) {
	handshakeURL := w.apiHost + "/api/v2/server/handshake"

	payload := map[string]interface{}{
		"token":     w.apiKey,
		"node_id":   w.nodeID,
		"node_type": w.nodeType,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal handshake payload: %w", err)
	}

	req, err := http.NewRequest("POST", handshakeURL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create handshake request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("handshake request: %w", err)
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("handshake status %d: %s", resp.StatusCode, respBody)
	}

	var hs HandshakeResponse
	if err := json.NewDecoder(resp.Body).Decode(&hs); err != nil {
		return nil, fmt.Errorf("decode handshake: %w", err)
	}

	return &hs, nil
}

func normalizeWSURL(raw string) string {
	s := strings.TrimRight(raw, "/")
	switch {
	case strings.HasPrefix(s, "https://"):
		s = "wss://" + s[8:]
	case strings.HasPrefix(s, "http://"):
		s = "ws://" + s[7:]
	case strings.HasPrefix(s, "ws://"), strings.HasPrefix(s, "wss://"):
	default:
		s = "wss://" + s
	}
	return s
}

func (w *WSClient) Disconnect() {
	w.mu.Lock()
	defer w.mu.Unlock()

	if !w.connected {
		return
	}

	w.connected = false
	if w.conn != nil {
		w.conn.Close()
		w.conn = nil
	}
}

func (w *WSClient) Stop() {
	close(w.stopCh)
	w.Disconnect()
}

func (w *WSClient) IsConnected() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.connected
}

func (w *WSClient) readLoop() {
	defer func() {
		w.handleDisconnect()
	}()

	for {
		select {
		case <-w.stopCh:
			return
		default:
		}

		w.mu.Lock()
		conn := w.conn
		w.mu.Unlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[xboard-ws] read error: %v", err)
			}
			return
		}

		var msg WSMessage
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("[xboard-ws] failed to parse message: %v", err)
			continue
		}

		switch msg.Type {
		case "config_update", "user_update", "update":
			var update WSConfigUpdate
			if err := json.Unmarshal(msg.Data, &update); err != nil {
				log.Printf("[xboard-ws] failed to parse config update: %v", err)
				continue
			}
			if w.onConfigUpdate != nil {
				w.onConfigUpdate(update.NodeInfo, update.Users)
			}
		case "ping":
			w.sendPong()
		default:
			log.Printf("[xboard-ws] unknown message type: %s", msg.Type)
		}
	}
}

func (w *WSClient) pingLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.mu.Lock()
			conn := w.conn
			w.mu.Unlock()

			if conn != nil {
				conn.WriteMessage(websocket.PingMessage, nil)
			}
		case <-w.stopCh:
			return
		}
	}
}

func (w *WSClient) sendPong() {
	w.mu.Lock()
	conn := w.conn
	w.mu.Unlock()

	if conn != nil {
		conn.WriteMessage(websocket.PongMessage, nil)
	}
}

func (w *WSClient) handleDisconnect() {
	w.mu.Lock()
	w.connected = false
	if w.conn != nil {
		w.conn.Close()
		w.conn = nil
	}
	w.mu.Unlock()

	if w.onDisconnected != nil {
		w.onDisconnected()
	}

	if !w.unsupported {
		go w.reconnect()
	}
}

func (w *WSClient) reconnect() {
	delay := w.reconnectDelay

	for {
		select {
		case <-w.stopCh:
			return
		case <-time.After(delay):
		}

		if w.unsupported {
			return
		}

		log.Printf("[xboard-ws] reconnecting...")
		if err := w.Connect(); err == nil {
			return
		}

		delay *= 2
		if delay > w.maxDelay {
			delay = w.maxDelay
		}
		log.Printf("[xboard-ws] reconnect failed, retrying in %v", delay)
	}
}
