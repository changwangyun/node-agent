package xboard

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

type HandshakeResponse struct {
	WebSocket struct {
		Enabled bool   `json:"enabled"`
		WsURL   string `json:"ws_url"`
	} `json:"websocket"`
	Settings struct {
		PushInterval int `json:"push_interval"`
		PullInterval int `json:"pull_interval"`
	} `json:"settings"`
}

type wsMessage struct {
	Event     string          `json:"event"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp int64           `json:"timestamp,omitempty"`
}

type WSClient struct {
	apiHost  string
	apiKey   string
	nodeID   string
	nodeType string

	connected    atomic.Bool
	writeCh      chan wsMessage
	stopCh       chan struct{}
	mu           sync.Mutex
	conn         *websocket.Conn
	reconnecting atomic.Bool

	onConfigUpdate func(*NodeInfo, []UserInfo)
	onDisconnected func()
	onMetrics      func() map[string]interface{}

	handshakeDone bool
	handshakeResp *HandshakeResponse
	handshakeErr  error
	handshakeMu   sync.Mutex

	disconnectAt time.Time
	disconnectMu sync.RWMutex
}

func NewWSClient(apiHost, apiKey, nodeID, nodeType string) *WSClient {
	return &WSClient{
		apiHost:  apiHost,
		apiKey:   apiKey,
		nodeID:   nodeID,
		nodeType: nodeType,
		stopCh:   make(chan struct{}),
	}
}

func (w *WSClient) SetOnConfigUpdate(fn func(*NodeInfo, []UserInfo)) {
	w.onConfigUpdate = fn
}

func (w *WSClient) SetOnDisconnected(fn func()) {
	w.onDisconnected = fn
}

func (w *WSClient) SetOnMetrics(fn func() map[string]interface{}) {
	w.onMetrics = fn
}

func (w *WSClient) GetHandshakeResponse() (*HandshakeResponse, error) {
	w.handshakeMu.Lock()
	defer w.handshakeMu.Unlock()
	return w.handshakeResp, w.handshakeErr
}

func (w *WSClient) IsConnected() bool {
	return w.connected.Load()
}

func (w *WSClient) DisconnectDuration() time.Duration {
	w.disconnectMu.RLock()
	defer w.disconnectMu.RUnlock()
	if w.disconnectAt.IsZero() {
		return 0
	}
	return time.Since(w.disconnectAt)
}

func (w *WSClient) Connect() {
	if w.reconnecting.Swap(true) {
		return
	}
	go w.run()
}

func (w *WSClient) run() {
	defer w.reconnecting.Store(false)

	backoff := 2 * time.Second
	maxBackoff := 60 * time.Second

	for {
		start := time.Now()
		err := w.connect()
		wasConnected := w.connected.Swap(false)

		w.disconnectMu.Lock()
		if wasConnected || w.disconnectAt.IsZero() {
			w.disconnectAt = time.Now()
		}
		w.disconnectMu.Unlock()

		if wasConnected && w.onDisconnected != nil {
			w.onDisconnected()
		}

		if err != nil {
			if !wasConnected {
				log.Printf("[xboard] ws connect failed, using REST polling: %v", err)
			}
		}

		select {
		case <-w.stopCh:
			return
		default:
		}

		if time.Since(start) > 2*time.Minute {
			backoff = 2 * time.Second
		}

		jitter := time.Duration(rand.Int63n(int64(backoff / 5)))
		wait := backoff + jitter

		timer := time.NewTimer(wait)
		select {
		case <-w.stopCh:
			timer.Stop()
			return
		case <-timer.C:
		}

		if backoff < maxBackoff {
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}
	}
}

func (w *WSClient) connect() error {
	if err := w.ensureHandshake(); err != nil {
		return fmt.Errorf("handshake: %w", err)
	}

	hs := w.handshakeResp
	if hs == nil || !hs.WebSocket.Enabled || hs.WebSocket.WsURL == "" {
		return fmt.Errorf("websocket not enabled on panel")
	}

	wsURL := normalizeWSURL(hs.WebSocket.WsURL)

	candidates := []string{wsURL}
	if fallback := deriveWSURLFromAPIHost(w.apiHost); fallback != "" && fallback != wsURL {
		candidates = append(candidates, fallback)
	}

	var conn *websocket.Conn
	var dialErr error

	for _, candidate := range candidates {
		u, err := url.Parse(candidate)
		if err != nil {
			dialErr = fmt.Errorf("parse ws url: %w", err)
			continue
		}

		q := u.Query()
		q.Set("token", w.apiKey)
		q.Set("node_id", w.nodeID)
		u.RawQuery = q.Encode()

		log.Printf("[xboard] ws connecting to %s", u.String())

		dialer := websocket.Dialer{
			HandshakeTimeout: 15 * time.Second,
		}

		conn, _, err = dialer.Dial(u.String(), nil)
		if err == nil {
			wsURL = candidate
			dialErr = nil
			break
		}
		dialErr = fmt.Errorf("dial: %w", err)
		log.Printf("[xboard] ws dial failed for %s: %v", candidate, err)
	}

	if dialErr != nil {
		return dialErr
	}

	conn.SetReadLimit(10 << 20)

	var firstMsg wsMessage
	if err := conn.ReadJSON(&firstMsg); err != nil {
		conn.Close()
		return fmt.Errorf("read auth response: %w", err)
	}

	log.Printf("[xboard-ws] auth response: event=%s", firstMsg.Event)

	if firstMsg.Event == "error" {
		conn.Close()
		var errData struct {
			Message string `json:"message"`
		}
		if err := json.Unmarshal(firstMsg.Data, &errData); err != nil {
			return fmt.Errorf("auth failed (unable to parse error: %v)", err)
		}
		return fmt.Errorf("auth failed: %s", errData.Message)
	}

	if firstMsg.Event != "auth.success" {
		w.handleMessage(firstMsg)
	}

	w.mu.Lock()
	w.conn = conn
	w.mu.Unlock()

	w.connected.Store(true)

	w.disconnectMu.Lock()
	w.disconnectAt = time.Time{}
	w.disconnectMu.Unlock()

	writeCh := make(chan wsMessage, 16)
	w.writeCh = writeCh

	errCh := make(chan error, 1)
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			var msg wsMessage
			if err := conn.ReadJSON(&msg); err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			w.handleMessage(msg)
			if msg.Event == "ping" {
				select {
				case writeCh <- wsMessage{Event: "pong"}:
				default:
				}
			}
		}
	}()

	statusInterval := 60 * time.Second
	if hs.Settings.PushInterval > 0 {
		statusInterval = time.Duration(hs.Settings.PushInterval) * time.Second
	}
	reportTicker := time.NewTicker(statusInterval)
	defer reportTicker.Stop()

	log.Printf("[xboard-ws] connected to %s", wsURL)

	for {
		select {
		case <-w.stopCh:
			conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			conn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			<-done
			return nil
		case err := <-errCh:
			conn.Close()
			<-done
			return fmt.Errorf("read: %w", err)
		case <-reportTicker.C:
			if w.onMetrics != nil {
				stats := w.onMetrics()
				if stats != nil {
					data, _ := json.Marshal(stats)
					msg := wsMessage{
						Event:     "node.status",
						Data:      data,
						Timestamp: time.Now().Unix(),
					}
					select {
					case writeCh <- msg:
					default:
					}
				}
			}
		case msg := <-writeCh:
			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteJSON(msg); err != nil {
				conn.Close()
				<-done
				return fmt.Errorf("write: %w", err)
			}
		}
	}
}

func (w *WSClient) ensureHandshake() error {
	w.handshakeMu.Lock()
	if w.handshakeDone {
		w.handshakeMu.Unlock()
		return w.handshakeErr
	}
	w.handshakeMu.Unlock()

	w.handshakeMu.Lock()
	defer w.handshakeMu.Unlock()

	if w.handshakeDone {
		return w.handshakeErr
	}

	w.handshakeDone = true

	handshakeURL := w.apiHost + "/api/v2/server/handshake"

	payload := map[string]interface{}{
		"token":   w.apiKey,
		"node_id": w.nodeID,
	}
	if w.nodeType != "" {
		payload["node_type"] = w.nodeType
	}

	body, err := json.Marshal(payload)
	if err != nil {
		w.handshakeErr = fmt.Errorf("marshal handshake payload: %w", err)
		return w.handshakeErr
	}

	req, err := http.NewRequest("POST", handshakeURL, bytes.NewReader(body))
	if err != nil {
		w.handshakeErr = fmt.Errorf("create handshake request: %w", err)
		return w.handshakeErr
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		w.handshakeErr = fmt.Errorf("handshake request: %w", err)
		return w.handshakeErr
	}
	defer func() {
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		w.handshakeErr = fmt.Errorf("handshake status %d: %s", resp.StatusCode, respBody)
		return w.handshakeErr
	}

	var hs HandshakeResponse
	if err := json.NewDecoder(resp.Body).Decode(&hs); err != nil {
		w.handshakeErr = fmt.Errorf("decode handshake: %w", err)
		return w.handshakeErr
	}

	w.handshakeResp = &hs
	log.Printf("[xboard] handshake ok, ws_enabled=%v ws_url=%s push_interval=%d pull_interval=%d",
		hs.WebSocket.Enabled, hs.WebSocket.WsURL, hs.Settings.PushInterval, hs.Settings.PullInterval)
	return nil
}

func (w *WSClient) RefreshHandshake() error {
	w.handshakeMu.Lock()
	w.handshakeDone = false
	w.handshakeResp = nil
	w.handshakeErr = nil
	w.handshakeMu.Unlock()

	return w.ensureHandshake()
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

func deriveWSURLFromAPIHost(apiHost string) string {
	s := strings.TrimRight(apiHost, "/")
	switch {
	case strings.HasPrefix(s, "https://"):
		return "wss://" + s[8:]
	case strings.HasPrefix(s, "http://"):
		return "ws://" + s[7:]
	}
	return ""
}

func (w *WSClient) handleMessage(msg wsMessage) {
	switch msg.Event {
	case "sync.config":
		var payload struct {
			Config    NodeInfo `json:"config"`
			Timestamp int64    `json:"timestamp"`
			NodeID    int      `json:"node_id"`
		}
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			log.Printf("[xboard-ws] failed to parse sync.config: %v", err)
			return
		}
		if w.onConfigUpdate != nil {
			w.onConfigUpdate(&payload.Config, nil)
		}

	case "sync.users":
		var payload struct {
			Users     []UserInfo `json:"users"`
			Timestamp int64      `json:"timestamp"`
			NodeID    int        `json:"node_id"`
		}
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			log.Printf("[xboard-ws] failed to parse sync.users: %v", err)
			return
		}
		if w.onConfigUpdate != nil {
			w.onConfigUpdate(nil, payload.Users)
		}

	case "sync.user.delta":
		var payload struct {
			Action    string     `json:"action"`
			Users     []UserInfo `json:"users"`
			Timestamp int64      `json:"timestamp"`
			NodeID    int        `json:"node_id"`
		}
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			log.Printf("[xboard-ws] failed to parse sync.user.delta: %v", err)
			return
		}
		if w.onConfigUpdate != nil {
			w.onConfigUpdate(nil, payload.Users)
		}

	case "auth.success":
	case "pong":
	case "ping":
		w.sendPong()
	default:
		log.Printf("[xboard-ws] unknown event: %s", msg.Event)
	}
}

func (w *WSClient) SendDeviceReport(devices map[int][]string) {
	if !w.connected.Load() {
		return
	}

	payload := map[string]interface{}{
		"devices": devices,
	}

	d, err := json.Marshal(payload)
	if err != nil {
		return
	}

	msg := wsMessage{
		Event:     "report.devices",
		Data:      d,
		Timestamp: time.Now().Unix(),
	}

	select {
	case w.writeCh <- msg:
	default:
		log.Printf("[xboard-ws] write channel full, skipping device report")
	}
}

func (w *WSClient) SendNodeStatus(stats map[string]interface{}) {
	if !w.connected.Load() || stats == nil {
		return
	}

	data, _ := json.Marshal(stats)
	msg := wsMessage{
		Event:     "node.status",
		Data:      data,
		Timestamp: time.Now().Unix(),
	}

	select {
	case w.writeCh <- msg:
	default:
		log.Printf("[xboard-ws] write channel full, skipping node status")
	}
}

func (w *WSClient) sendPong() {
	if !w.connected.Load() {
		return
	}
	select {
	case w.writeCh <- wsMessage{Event: "pong"}:
	default:
	}
}

func (w *WSClient) Disconnect() {
	w.mu.Lock()
	if w.conn != nil {
		w.conn.Close()
		w.conn = nil
	}
	w.mu.Unlock()
	w.connected.Store(false)
}

func (w *WSClient) Stop() {
	close(w.stopCh)
	w.Disconnect()
}

func intIDToString(id string) string {
	if _, err := strconv.Atoi(id); err == nil {
		return id
	}
	return id
}
