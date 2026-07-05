package frpc

import (
	"testing"

	"ashan-frp/internal/domain"
)

func Test_ConfigRenderer_Render_returns_config_with_enabled_tunnels(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "secret-token")
	tunnels := []domain.Tunnel{
		{
			ID:           "tun_1",
			ProjectName:  "my-app",
			Subdomain:    "my-app",
			FullDomain:   "my-app.335356119.xyz",
			Protocol:     "http",
			LocalIP:      "127.0.0.1",
			LocalPort:    3000,
			DesiredState: domain.TunnelDesiredEnabled,
		},
		{
			ID:           "tun_2",
			ProjectName:  "api-gateway",
			Subdomain:    "api",
			FullDomain:   "api.335356119.xyz",
			Protocol:     "tcp",
			LocalIP:      "127.0.0.1",
			LocalPort:    8080,
			RemotePort:   7001,
			DesiredState: domain.TunnelDesiredEnabled,
		},
	}

	// When
	cfg, err := renderer.Render(tunnels)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.ServerAddr != "frp.example.com" {
		t.Errorf("expected server_addr 'frp.example.com', got %q", cfg.ServerAddr)
	}
	if cfg.ServerPort != 7000 {
		t.Errorf("expected server_port 7000, got %d", cfg.ServerPort)
	}
	if cfg.AuthToken != "secret-token" {
		t.Errorf("expected auth_token 'secret-token', got %q", cfg.AuthToken)
	}
	if len(cfg.Proxies) != 2 {
		t.Fatalf("expected 2 proxies, got %d", len(cfg.Proxies))
	}

	// HTTP proxy
	httpProxy := cfg.Proxies[0]
	if httpProxy.Name != "my-app" {
		t.Errorf("expected proxy name 'my-app', got %q", httpProxy.Name)
	}
	if httpProxy.Type != "http" {
		t.Errorf("expected proxy type 'http', got %q", httpProxy.Type)
	}
	if httpProxy.LocalPort != 3000 {
		t.Errorf("expected local_port 3000, got %d", httpProxy.LocalPort)
	}
	if httpProxy.Subdomain != "my-app" {
		t.Errorf("expected subdomain 'my-app', got %q", httpProxy.Subdomain)
	}
	if httpProxy.CustomDomains != "my-app.335356119.xyz" {
		t.Errorf("expected custom_domains 'my-app.335356119.xyz', got %q", httpProxy.CustomDomains)
	}
	if httpProxy.RemotePort != 0 {
		t.Errorf("expected remote_port 0 for http proxy, got %d", httpProxy.RemotePort)
	}
	if !httpProxy.Encryption {
		t.Error("expected encryption=true")
	}
	if !httpProxy.Compression {
		t.Error("expected compression=true")
	}

	// TCP proxy
	tcpProxy := cfg.Proxies[1]
	if tcpProxy.Name != "api-gateway" {
		t.Errorf("expected proxy name 'api-gateway', got %q", tcpProxy.Name)
	}
	if tcpProxy.Type != "tcp" {
		t.Errorf("expected proxy type 'tcp', got %q", tcpProxy.Type)
	}
	if tcpProxy.RemotePort != 7001 {
		t.Errorf("expected remote_port 7001, got %d", tcpProxy.RemotePort)
	}
}

func Test_ConfigRenderer_Render_skips_disabled_tunnels(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "")
	tunnels := []domain.Tunnel{
		{
			ID:           "tun_1",
			ProjectName:  "enabled-one",
			Protocol:     "tcp",
			LocalIP:      "127.0.0.1",
			LocalPort:    3000,
			DesiredState: domain.TunnelDesiredEnabled,
		},
		{
			ID:           "tun_2",
			ProjectName:  "disabled-one",
			Protocol:     "tcp",
			LocalIP:      "127.0.0.1",
			LocalPort:    4000,
			DesiredState: domain.TunnelDesiredDisabled,
		},
	}

	// When
	cfg, err := renderer.Render(tunnels)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Proxies) != 1 {
		t.Fatalf("expected 1 proxy, got %d", len(cfg.Proxies))
	}
	if cfg.Proxies[0].Name != "enabled-one" {
		t.Errorf("expected proxy 'enabled-one', got %q", cfg.Proxies[0].Name)
	}
}

func Test_ConfigRenderer_Render_returns_empty_proxies_for_no_tunnels(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "")

	// When
	cfg, err := renderer.Render(nil)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Proxies) != 0 {
		t.Errorf("expected 0 proxies, got %d", len(cfg.Proxies))
	}
}

func Test_ConfigRenderer_Render_rejects_empty_server_addr(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("", 7000, "")

	// When
	_, err := renderer.Render(nil)

	// Then
	if err == nil {
		t.Fatal("expected error for empty server_addr, got nil")
	}
}

func Test_ConfigRenderer_Render_rejects_invalid_server_port(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 0, "")

	// When
	_, err := renderer.Render(nil)

	// Then
	if err == nil {
		t.Fatal("expected error for port 0, got nil")
	}
}

func Test_ConfigRenderer_Render_uses_log_settings(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "")
	renderer.SetLogLevel("debug")
	renderer.SetLogMaxDays(7)

	// When
	cfg, err := renderer.Render(nil)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("expected log_level 'debug', got %q", cfg.LogLevel)
	}
	if cfg.LogMaxDays != 7 {
		t.Errorf("expected log_max_days 7, got %d", cfg.LogMaxDays)
	}
}

func Test_ConfigRenderer_Render_uses_name_fallback(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "")
	tunnels := []domain.Tunnel{
		{
			ID:           "tun_fallback",
			Name:         "legacy-name",
			Protocol:     "tcp",
			LocalIP:      "127.0.0.1",
			LocalPort:    3000,
			DesiredState: domain.TunnelDesiredEnabled,
		},
	}

	// When
	cfg, err := renderer.Render(tunnels)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Proxies[0].Name != "legacy-name" {
		t.Errorf("expected fallback name 'legacy-name', got %q", cfg.Proxies[0].Name)
	}
}

func Test_ConfigRenderer_Render_uses_id_as_last_resort_name(t *testing.T) {
	// Given
	renderer := NewConfigRenderer("frp.example.com", 7000, "")
	tunnels := []domain.Tunnel{
		{
			ID:           "tun_id_only",
			Protocol:     "tcp",
			LocalIP:      "127.0.0.1",
			LocalPort:    3000,
			DesiredState: domain.TunnelDesiredEnabled,
		},
	}

	// When
	cfg, err := renderer.Render(tunnels)

	// Then
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Proxies[0].Name != "tun_id_only" {
		t.Errorf("expected ID fallback name 'tun_id_only', got %q", cfg.Proxies[0].Name)
	}
}
