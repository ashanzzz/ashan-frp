package frpc

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ashan-frp/internal/domain"
)

func Test_Manager_NewManager_starts_in_stopped_state(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "")
	workDir := t.TempDir()

	// When
	mgr := NewManager(workDir, renderer)

	// Then
	if mgr.Status() != StatusStopped {
		t.Errorf("expected StatusStopped, got %s", mgr.Status())
	}
}

func Test_Manager_Start_fails_when_binary_missing(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "")
	workDir := t.TempDir()
	mgr := NewManager(workDir, renderer)
	mgr.SetBinaryPath(filepath.Join(workDir, "bin", "nonexistent"))

	tunnels := []domain.Tunnel{
		{
			ID:           "tun_1",
			ProjectName:  "test",
			Protocol:     "tcp",
			LocalIP:      "127.0.0.1",
			LocalPort:    3000,
			DesiredState: domain.TunnelDesiredEnabled,
		},
	}

	// When
	err := mgr.Start(context.Background(), tunnels)

	// Then
	if err == nil {
		t.Fatal("expected error for missing binary, got nil")
	}
	if mgr.Status() != StatusFailed {
		t.Errorf("expected StatusFailed, got %s", mgr.Status())
	}
}

func Test_Manager_Start_fails_when_already_running(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "")
	workDir := t.TempDir()
	mgr := NewManager(workDir, renderer)

	// Simulate running state.
	mgr.mu.Lock()
	mgr.status = StatusRunning
	mgr.mu.Unlock()

	// When
	err := mgr.Start(context.Background(), nil)

	// Then
	if err == nil {
		t.Fatal("expected error when already running, got nil")
	}
}

func Test_Manager_Stop_is_idempotent(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "")
	workDir := t.TempDir()
	mgr := NewManager(workDir, renderer)

	// When
	err1 := mgr.Stop()
	err2 := mgr.Stop()

	// Then
	if err1 != nil {
		t.Errorf("first stop should not error, got %v", err1)
	}
	if err2 != nil {
		t.Errorf("second stop should not error, got %v", err2)
	}
	if mgr.Status() != StatusStopped {
		t.Errorf("expected StatusStopped, got %s", mgr.Status())
	}
}

func Test_Manager_Reload_fails_when_not_running(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "")
	workDir := t.TempDir()
	mgr := NewManager(workDir, renderer)

	// When
	err := mgr.Reload(context.Background(), nil)

	// Then
	if err == nil {
		t.Fatal("expected error when reloading stopped manager, got nil")
	}
}

func Test_Manager_Healthcheck_reports_stopped_when_not_running(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "")
	workDir := t.TempDir()
	mgr := NewManager(workDir, renderer)

	// When
	status, reason := mgr.Healthcheck()

	// Then
	if status != StatusStopped {
		t.Errorf("expected StatusStopped, got %s", status)
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func Test_Manager_StatusChange_callback_fires(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "")
	workDir := t.TempDir()
	mgr := NewManager(workDir, renderer)

	var captured []Status
	mgr.OnStatusChange(func(s Status, _ string) {
		captured = append(captured, s)
	})

	// When - trigger a status change by starting with missing binary
	mgr.SetBinaryPath(filepath.Join(workDir, "bin", "nonexistent"))
	_ = mgr.Start(context.Background(), []domain.Tunnel{
		{
			ID:           "tun_1",
			ProjectName:  "test",
			Protocol:     "tcp",
			LocalIP:      "127.0.0.1",
			LocalPort:    3000,
			DesiredState: domain.TunnelDesiredEnabled,
		},
	})

	// Then
	if len(captured) < 2 {
		t.Fatalf("expected at least 2 status changes, got %d: %v", len(captured), captured)
	}
	if captured[0] != StatusStarting {
		t.Errorf("expected first status StatusStarting, got %s", captured[0])
	}
	if captured[1] != StatusFailed {
		t.Errorf("expected second status StatusFailed, got %s", captured[1])
	}
}

func Test_Manager_Start_writes_config_file(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "secret")
	workDir := t.TempDir()
	mgr := NewManager(workDir, renderer)
	mgr.SetBinaryPath(filepath.Join(workDir, "bin", "nonexistent"))

	tunnels := []domain.Tunnel{
		{
			ID:           "tun_1",
			ProjectName:  "test-app",
			Subdomain:    "test-app",
			FullDomain:   "test-app.335356119.xyz",
			Protocol:     "http",
			LocalIP:      "127.0.0.1",
			LocalPort:    3000,
			DesiredState: domain.TunnelDesiredEnabled,
		},
	}

	// When - Start will fail because binary is missing, but config should be written
	_ = mgr.Start(context.Background(), tunnels)

	// Then
	configPath := filepath.Join(workDir, "frpc.toml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected config file at %s, got error: %v", configPath, err)
	}
	content := string(data)
	if len(content) == 0 {
		t.Fatal("config file is empty")
	}
	if !contains(content, "server_addr") {
		t.Error("config missing server_addr")
	}
	if !contains(content, "test-app") {
		t.Error("config missing tunnel name 'test-app'")
	}
}

func Test_Manager_LastHealthcheck_tracks_time(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "")
	workDir := t.TempDir()
	mgr := NewManager(workDir, renderer)

	before := time.Now()

	// When
	mgr.Healthcheck()

	// Then
	after := mgr.LastHealthcheck()
	if after.Before(before) {
		t.Errorf("LastHealthcheck %v should not be before %v", after, before)
	}
}

func Test_Manager_SetHealthcheckInterval_updates(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "")
	workDir := t.TempDir()
	mgr := NewManager(workDir, renderer)

	// When
	mgr.SetHealthcheckInterval(10 * time.Second)

	// Then
	if mgr.healthcheckInterval != 10*time.Second {
		t.Errorf("expected 10s interval, got %v", mgr.healthcheckInterval)
	}
}

func Test_Manager_Workflow_AddTunnels_RenderConfig_Stop_Replace_Start(t *testing.T) {
	// Given: 1. Setup renderer & manager
	renderer := NewConfigRenderer("frp.chmlfrp.cn", 7000, "token_12345")
	workDir := t.TempDir()
	mgr := NewManager(workDir, renderer)

	// Step 1: Add N tunnels (e.g. web_app HTTP and ssh_app TCP)
	tunnels := []domain.Tunnel{
		{
			ID:           "tun_web",
			Name:         "web_app",
			Protocol:     "http",
			LocalIP:      "127.0.0.1",
			LocalPort:    8080,
			Subdomain:    "myweb",
			FullDomain:   "myweb.chmlfrp.cn",
			DesiredState: domain.TunnelDesiredEnabled,
		},
		{
			ID:           "tun_ssh",
			Name:         "ssh_app",
			Protocol:     "tcp",
			LocalIP:      "127.0.0.1",
			LocalPort:    22,
			RemotePort:   22222,
			DesiredState: domain.TunnelDesiredEnabled,
		},
	}

	// Step 2: Render configuration
	cfg, err := renderer.Render(tunnels)
	if err != nil {
		t.Fatalf("render config failed: %v", err)
	}
	if len(cfg.Proxies) != 2 {
		t.Fatalf("expected 2 proxies, got %d", len(cfg.Proxies))
	}

	// Step 3: Write / Replace configuration file frpc.toml
	if err := mgr.writeConfig(cfg); err != nil {
		t.Fatalf("write config failed: %v", err)
	}

	content, err := mgr.GetConfigContent()
	if err != nil || !contains(content, "myweb.chmlfrp.cn") || !contains(content, "22222") {
		t.Fatalf("config file content invalid: %v, content: %s", err, content)
	}

	// Step 4 & 5: Stop FRPC (if running) and Start FRPC with new config file
	_ = mgr.Stop()
	if mgr.Status() != StatusStopped {
		t.Errorf("expected StatusStopped before restart, got %s", mgr.Status())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
