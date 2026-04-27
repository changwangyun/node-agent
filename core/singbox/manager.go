package singbox

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
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

type Manager struct {
	mu     sync.RWMutex
	cmd    *exec.Cmd
	cancel context.CancelFunc
	state  ProcessState
	cfg    *config.Config

	startTime time.Time
	crashCh   chan struct{}
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		cfg:     cfg,
		state:   StateStopped,
		crashCh: make(chan struct{}, 16),
	}
}

func (m *Manager) Start() error {
	m.mu.Lock()
	if m.state == StateRunning || m.state == StateStarting {
		m.mu.Unlock()
		return fmt.Errorf("sing-box is already running or starting")
	}
	m.state = StateStarting
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
		return fmt.Errorf("sing-box config not found: %s", configPath)
	}

	cmd := exec.CommandContext(ctx, binary, "run", "-c", configPath)
	cmd.Dir = m.cfg.SingBox.WorkDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	m.mu.Lock()
	m.cmd = cmd
	m.cancel = cancel
	m.mu.Unlock()

	if err := cmd.Start(); err != nil {
		cancel()
		m.setState(StateCrashed)
		return fmt.Errorf("start sing-box: %w", err)
	}

	m.setState(StateRunning)
	m.startTime = time.Now()

	go m.monitorProcess(ctx)

	return nil
}

func (m *Manager) monitorProcess(ctx context.Context) {
	err := m.cmd.Wait()

	m.mu.Lock()
	wasRunning := m.state == StateRunning
	m.mu.Unlock()

	if wasRunning {
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
		_ = syscall.Kill(-m.cmd.Process.Pid, syscall.SIGTERM)

		done := make(chan error, 1)
		go func() {
			_, _ = m.cmd.Process.Wait()
			done <- nil
		}()

		select {
		case <-done:
		case <-time.After(10 * time.Second):
			_ = syscall.Kill(-m.cmd.Process.Pid, syscall.SIGKILL)
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

func (m *Manager) CrashChannel() <-chan struct{} {
	return m.crashCh
}

func (m *Manager) setState(s ProcessState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = s
}
