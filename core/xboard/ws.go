package xboard

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

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

	if w.connected {
		return nil
	}

	wsURL, err := w.buildWSURL()
	if err != nil {
		return fmt.Errorf("build ws url: %w", err)
	}

	header := http.Header{}
	header.Set("Authorization", "Bearer "+w.apiKey)
	header.Set("X-Node-ID", w.nodeID)
	header.Set("X-Node-Type", w.nodeType)

	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(wsURL, header)
	if err != nil {
		return fmt.Errorf("dial ws: %w", err)
	}

	w.conn = conn
	w.connected = true

	go w.readLoop()
	go w.pingLoop()

	log.Printf("[xboard-ws] connected to %s", wsURL)
	return nil
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

func (w *WSClient) buildWSURL() (string, error) {
	u, err := url.Parse(w.apiHost)
	if err != nil {
		return "", fmt.Errorf("parse api host: %w", err)
	}

	wsScheme := "ws"
	if u.Scheme == "https" {
		wsScheme = "wss"
	}

	wsURL := fmt.Sprintf("%s://%s/api/v2/server/handshake", wsScheme, u.Host)
	if u.Path != "" && u.Path != "/" {
		wsURL = fmt.Sprintf("%s://%s%s/api/v2/server/handshake", wsScheme, u.Host, u.Path)
	}

	return wsURL, nil
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

	go w.reconnect()
}

func (w *WSClient) reconnect() {
	delay := w.reconnectDelay

	for {
		select {
		case <-w.stopCh:
			return
		case <-time.After(delay):
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
