package frpc

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"ashan-frp/internal/domain"
)

// Status represents the FRPC process state.
type Status string

const (
	StatusStopped    Status = "stopped"
	StatusStarting   Status = "starting"
	StatusRunning    Status = "running"
	StatusDegraded   Status = "degraded"
	StatusRestarting Status = "restarting"
	StatusFailed     Status = "failed"
)

// Manager manages the frpc subprocess lifecycle.
type Manager struct {
	mu     sync.Mutex
	status Status

	workDir    string
	binaryPath string
	configPath string

	cmd    *exec.Cmd
	cancel context.CancelFunc

	renderer *ConfigRenderer

	healthcheckInterval time.Duration
	lastHealthcheck     time.Time
	lastError           string

	onStatusChange func(Status, string)
}

// NewManager creates a FRPC runtime manager.
func NewManager(workDir string, renderer *ConfigRenderer) *Manager {
	return &Manager{
		workDir:             workDir,
		binaryPath:          filepath.Join(workDir, "bin", "frpc"),
		configPath:          filepath.Join(workDir, "frpc.toml"),
		renderer:            renderer,
		healthcheckInterval: 30 * time.Second,
		status:              StatusStopped,
	}
}

// SetBinaryPath overrides the default frpc binary location.
func (m *Manager) SetBinaryPath(path string) {
	m.binaryPath = path
}

// SetHealthcheckInterval sets the healthcheck polling interval.
func (m *Manager) SetHealthcheckInterval(d time.Duration) {
	m.healthcheckInterval = d
}

// OnStatusChange registers a callback for status transitions.
func (m *Manager) OnStatusChange(fn func(Status, string)) {
	m.onStatusChange = fn
}

// Status returns the current FRPC process status.
func (m *Manager) Status() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.status
}

// Start generates config and launches the frpc subprocess.
func (m *Manager) Start(ctx context.Context, tunnels []domain.Tunnel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status == StatusRunning || m.status == StatusStarting {
		return fmt.Errorf("frpc manager: cannot start, already %s", m.status)
	}

	m.setStatus(StatusStarting, "generating config")

	cfg, err := m.renderer.Render(tunnels)
	if err != nil {
		m.setStatus(StatusFailed, err.Error())
		return fmt.Errorf("frpc manager: config render: %w", err)
	}

	if err := m.writeConfig(cfg); err != nil {
		m.setStatus(StatusFailed, err.Error())
		return fmt.Errorf("frpc manager: write config: %w", err)
	}

	if err := m.ensureBinaryLocked(); err != nil {
		m.setStatus(StatusFailed, err.Error())
		return fmt.Errorf("frpc manager: binary check: %w", err)
	}

	procCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel

	cmd := exec.CommandContext(procCtx, m.binaryPath, "-c", m.configPath)
	cmd.Dir = m.workDir
	cmd.Stdout = m.openLog("stdout.log")
	cmd.Stderr = m.openLog("stderr.log")

	if err := cmd.Start(); err != nil {
		cancel()
		m.setStatus(StatusFailed, err.Error())
		return fmt.Errorf("frpc manager: start process: %w", err)
	}

	m.cmd = cmd
	m.setStatus(StatusRunning, "process started")

	go m.monitor(procCtx, cmd)

	return nil
}

// Stop terminates the frpc subprocess.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status == StatusStopped {
		return nil
	}

	if m.cancel != nil {
		m.cancel()
		m.cancel = nil
	}

	if m.cmd != nil && m.cmd.Process != nil {
		m.cmd.Process.Signal(os.Interrupt)
		done := make(chan struct{})
		go func() {
			m.cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			m.cmd.Process.Kill()
		}
	}

	m.cmd = nil
	m.setStatus(StatusStopped, "stopped by request")
	return nil
}

// Restart stops and starts the frpc subprocess.
func (m *Manager) Restart(ctx context.Context, tunnels []domain.Tunnel) error {
	m.mu.Lock()
	prevStatus := m.status
	m.mu.Unlock()

	if prevStatus == StatusRunning || prevStatus == StatusDegraded {
		if err := m.Stop(); err != nil {
			log.Printf("[frpc] stop before restart: %v", err)
		}
	}

	m.mu.Lock()
	m.setStatus(StatusRestarting, "restarting")
	m.mu.Unlock()

	return m.Start(ctx, tunnels)
}

// Reload generates new config and sends SIGHUP to the running frpc.
func (m *Manager) Reload(ctx context.Context, tunnels []domain.Tunnel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.status != StatusRunning && m.status != StatusDegraded {
		return fmt.Errorf("frpc manager: cannot reload, status is %s", m.status)
	}

	cfg, err := m.renderer.Render(tunnels)
	if err != nil {
		return fmt.Errorf("frpc manager: config render: %w", err)
	}

	if err := m.writeConfig(cfg); err != nil {
		return fmt.Errorf("frpc manager: write config: %w", err)
	}

	if m.cmd != nil && m.cmd.Process != nil {
		if err := m.cmd.Process.Signal(os.Interrupt); err != nil {
			return fmt.Errorf("frpc manager: signal reload: %w", err)
		}
	}

	return nil
}

// Healthcheck returns the current health status.
func (m *Manager) Healthcheck() (Status, string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.lastHealthcheck = time.Now()

	if m.status == StatusStopped {
		return StatusStopped, "not running"
	}

	if m.cmd == nil || m.cmd.Process == nil {
		m.setStatus(StatusFailed, "process handle missing")
		return m.status, m.lastError
	}

	// Check if process is still alive.
	if m.cmd.ProcessState != nil && m.cmd.ProcessState.Exited() {
		exitCode := m.cmd.ProcessState.ExitCode()
		reason := fmt.Sprintf("process exited with code %d", exitCode)
		m.setStatus(StatusFailed, reason)
		return m.status, reason
	}

	return m.status, ""
}

// LastError returns the last recorded error.
func (m *Manager) LastError() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastError
}

// LastHealthcheck returns when the last healthcheck ran.
func (m *Manager) LastHealthcheck() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.lastHealthcheck
}

func (m *Manager) setStatus(s Status, reason string) {
	old := m.status
	m.status = s
	if reason != "" {
		m.lastError = reason
	}
	if old != s && m.onStatusChange != nil {
		m.onStatusChange(s, reason)
	}
}

func (m *Manager) writeConfig(cfg *FRPCConfig) error {
	if err := os.MkdirAll(m.workDir, 0o755); err != nil {
		return err
	}

	// Write a simple TOML representation.
	f, err := os.Create(m.configPath)
	if err != nil {
		return err
	}
	defer f.Close()

	fmt.Fprintf(f, "server_addr = %q\n", cfg.ServerAddr)
	fmt.Fprintf(f, "server_port = %d\n", cfg.ServerPort)
	if cfg.AuthToken != "" {
		fmt.Fprintf(f, "auth_token = %q\n", cfg.AuthToken)
	}
	fmt.Fprintf(f, "log_file = %q\n", filepath.Join(m.workDir, "logs", "frpc.log"))
	fmt.Fprintf(f, "log_level = %q\n", cfg.LogLevel)
	fmt.Fprintf(f, "log_max_days = %d\n", cfg.LogMaxDays)
	fmt.Fprintf(f, "\n")

	for _, p := range cfg.Proxies {
		fmt.Fprintf(f, "[[proxies]]\n")
		fmt.Fprintf(f, "name = %q\n", p.Name)
		fmt.Fprintf(f, "type = %q\n", p.Type)
		fmt.Fprintf(f, "local_ip = %q\n", p.LocalIP)
		fmt.Fprintf(f, "local_port = %d\n", p.LocalPort)
		if p.RemotePort > 0 {
			fmt.Fprintf(f, "remote_port = %d\n", p.RemotePort)
		}
		if p.Subdomain != "" {
			fmt.Fprintf(f, "subdomain = %q\n", p.Subdomain)
		}
		if p.CustomDomains != "" {
			fmt.Fprintf(f, "custom_domains = %q\n", p.CustomDomains)
		}
		fmt.Fprintf(f, "encryption = %v\n", p.Encryption)
		fmt.Fprintf(f, "compression = %v\n", p.Compression)
		fmt.Fprintf(f, "\n")
	}

	return nil
}

func (m *Manager) BinaryPath() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.binaryPath
}

func (m *Manager) GetVersion() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	_ = m.ensureBinaryLocked()
	if m.binaryPath == "" {
		return "v0.54.0 (embedded)"
	}
	cmd := exec.Command(m.binaryPath, "--version")
	out, err := cmd.Output()
	if err != nil {
		return "v0.54.0 (embedded)"
	}
	return strings.TrimSpace(string(out))
}

func (m *Manager) GetConfigContent() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, err := os.ReadFile(m.configPath)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (m *Manager) ReadLogs(maxLines int) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	logPath := filepath.Join(m.workDir, "logs", "stdout.log")
	b, err := os.ReadFile(logPath)
	if err != nil {
		logPath = filepath.Join(m.workDir, "logs", "frpc.log")
		b, err = os.ReadFile(logPath)
		if err != nil {
			return "暂无 FRPC 运行日志", nil
		}
	}
	lines := strings.Split(string(b), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	return strings.Join(lines, "\n"), nil
}

func (m *Manager) ensureBinary() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.ensureBinaryLocked()
}

func (m *Manager) ensureBinaryLocked() error {
	if _, err := os.Stat(m.binaryPath); err == nil {
		return nil
	}
	if path, err := exec.LookPath("frpc"); err == nil {
		m.binaryPath = path
		return nil
	}
	candidates := []string{
		"/usr/local/bin/frpc",
		"/usr/bin/frpc",
		"/app/bin/frpc",
		filepath.Join(m.workDir, "frpc"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			m.binaryPath = c
			return nil
		}
	}
	return fmt.Errorf("frpc binary not found at %s or PATH", m.binaryPath)
}

func (m *Manager) openLog(name string) *os.File {
	logDir := filepath.Join(m.workDir, "logs")
	os.MkdirAll(logDir, 0o755)
	f, err := os.OpenFile(filepath.Join(logDir, name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		log.Printf("[frpc] cannot open log %s: %v", name, err)
		return nil
	}
	return f
}

func (m *Manager) monitor(ctx context.Context, cmd *exec.Cmd) {
	err := cmd.Wait()

	m.mu.Lock()
	defer m.mu.Unlock()

	if ctx.Err() != nil {
		// Context was cancelled — intentional stop.
		return
	}

	if err != nil {
		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		reason := fmt.Sprintf("frpc exited unexpectedly (code %d): %v", exitCode, err)
		m.setStatus(StatusFailed, reason)
		log.Printf("[frpc] %s", reason)
	}
}
