package singbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"node-agent/config"
)

type ProcessState int

const (
	StateStopped ProcessState = iota
	StateRunning
	StateStarting
	StateStopping
	StateCrashed
)

func (s ProcessState) String() string {
	switch s {
	case StateRunning:
		return "running"
	case StateStarting:
		return "starting"
	case StateStopping:
		return "stopping"
	case StateCrashed:
		return "crashed"
	default:
		return "stopped"
	}
}

const maxLogLines = 200

type Manager struct {
	mu        sync.RWMutex
	cmd       *exec.Cmd
	cancel    context.CancelFunc
	state     ProcessState
	cfg       *config.Config
	hasConfig bool

	startTime   time.Time
	crashCh     chan struct{}
	lastError   string
	crashTime   time.Time
	logBuffer   *RingBuffer
}

type RingBuffer struct {
	mu     sync.RWMutex
	lines  []string
	size   int
	head   int
	count  int
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		lines: make([]string, size),
		size:  size,
	}
}

func (r *RingBuffer) Write(line string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.lines[r.head] = line
	r.head = (r.head + 1) % r.size
	if r.count < r.size {
		r.count++
	}
}

func (r *RingBuffer) Lines(n int) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if n > r.count {
		n = r.count
	}
	result := make([]string, 0, n)
	start := r.head - n
	if start < 0 {
		start += r.size
	}
	for i := 0; i < n; i++ {
		idx := (start + i) % r.size
		result = append(result, r.lines[idx])
	}
	return result
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:       cfg,
		state:     StateStopped,
		crashCh:   make(chan struct{}, 16),
		logBuffer: NewRingBuffer(maxLogLines),
	}
}

func (m *Manager) Start() error {
	m.mu.Lock()
	if m.state == StateRunning || m.state == StateStarting {
		m.mu.Unlock()
		return fmt.Errorf("sing-box is already running or starting")
	}
	m.state = StateStarting
	m.lastError = ""
	m.mu.Unlock()

	ctx, cancel := context.WithCancel(context.Background())

	binary := m.cfg.SingBox.BinaryPath
	configPath := m.cfg.SingBox.ConfigPath

	if _, err := os.Stat(binary); err != nil {
		cancel()
		m.setState(StateStopped)
		return fmt.Errorf("sing-box binary not found: %s", binary)
	}

	if _, err := os.Stat(configPath); err != nil {
		cancel()
		m.setState(StateStopped)
		return fmt.Errorf("sing-box config not found: %s (deploy a config first via POST /deploy)", configPath)
	}

	m.hasConfig = true

	cmd := exec.CommandContext(ctx, binary, "run", "-c", configPath)
	cmd.Dir = m.cfg.SingBox.WorkDir
	setSysProcAttr(cmd)

	var stderrBuf bytes.Buffer
	cmd.Stdout = &logWriter{prefix: "[singbox:out] ", buf: m.logBuffer, fallback: os.Stdout}
	cmd.Stderr = &logWriter{prefix: "[singbox:err] ", buf: m.logBuffer, fallback: os.Stderr, captureErr: &stderrBuf}

	m.mu.Lock()
	m.cmd = cmd
	m.cancel = cancel
	m.mu.Unlock()

	if err := cmd.Start(); err != nil {
		cancel()
		m.mu.Lock()
		m.lastError = fmt.Sprintf("start failed: %v", err)
		m.crashTime = time.Now()
		m.mu.Unlock()
		m.setState(StateCrashed)
		return fmt.Errorf("start sing-box: %w", err)
	}

	m.setState(StateRunning)
	m.startTime = time.Now()

	go m.monitorProcess(ctx, &stderrBuf)

	return nil
}

type logWriter struct {
	prefix    string
	buf       *RingBuffer
	fallback  *os.File
	captureErr *bytes.Buffer
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	line := string(p)
	w.buf.Write(w.prefix + line)
	if w.captureErr != nil {
		w.captureErr.Write(p)
	}
	return w.fallback.Write(p)
}

func (m *Manager) monitorProcess(ctx context.Context, stderrBuf *bytes.Buffer) {
	err := m.cmd.Wait()

	m.mu.Lock()
	wasRunning := m.state == StateRunning
	m.mu.Unlock()

	if wasRunning {
		errMsg := "unknown reason"
		if err != nil {
			errMsg = err.Error()
		}
		if stderrBuf.Len() > 0 {
			stderr := stderrBuf.String()
			lastLines := lastNLines(stderr, 5)
			errMsg = errMsg + "\nRecent stderr:\n" + lastLines
		}

		m.mu.Lock()
		m.lastError = errMsg
		m.crashTime = time.Now()
		m.mu.Unlock()

		m.setState(StateCrashed)
		select {
		case m.crashCh <- struct{}{}:
		default:
		}
	} else {
		m.setState(StateStopped)
	}

	if err != nil && ctx.Err() == nil {
		fmt.Printf("[singbox] process exited unexpectedly: %v\n", err)
	}
}

func lastNLines(s string, n int) string {
	count := 0
	idx := len(s)
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '\n' {
			count++
			if count >= n {
				idx = i + 1
				break
			}
		}
	}
	if idx > 0 && idx < len(s) {
		return s[idx:]
	}
	return s
}

func (m *Manager) Stop() error {
	m.mu.Lock()
	if m.state != StateRunning && m.state != StateCrashed {
		m.mu.Unlock()
		return fmt.Errorf("sing-box is not running")
	}
	m.state = StateStopping
	m.mu.Unlock()

	if m.cancel != nil {
		m.cancel()
	}

	if m.cmd != nil && m.cmd.Process != nil {
		_ = sendSigterm(m.cmd.Process.Pid)

		done := make(chan error, 1)
		go func() {
			_, _ = m.cmd.Process.Wait()
			done <- nil
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = sendSigkill(m.cmd.Process.Pid)
		}
	}

	m.setState(StateStopped)
	return nil
}

func (m *Manager) Restart() error {
	if m.IsRunning() {
		if err := m.Stop(); err != nil {
			return fmt.Errorf("stop sing-box: %w", err)
		}
		time.Sleep(500 * time.Millisecond)
	}
	return m.Start()
}

func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state == StateRunning
}

func (m *Manager) GetState() ProcessState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

func (m *Manager) GetUptime() time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state != StateRunning {
		return 0
	}
	return time.Since(m.startTime)
}

func (m *Manager) GetPID() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.cmd != nil && m.cmd.Process != nil {
		return m.cmd.Process.Pid
	}
	return 0
}

func (m *Manager) GetLastError() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastError
}

func (m *Manager) GetCrashTime() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.crashTime
}

func (m *Manager) GetLogs(n int) []string {
	if n <= 0 || n > maxLogLines {
		n = 50
	}
	return m.logBuffer.Lines(n)
}

func (m *Manager) CrashChannel() <-chan struct{} {
	return m.crashCh
}

func (m *Manager) ConfigExists() bool {
	if m.hasConfig {
		return true
	}
	_, err := os.Stat(m.cfg.SingBox.ConfigPath)
	if err == nil {
		m.hasConfig = true
		return true
	}
	return false
}

func (m *Manager) setState(s ProcessState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = s
}
