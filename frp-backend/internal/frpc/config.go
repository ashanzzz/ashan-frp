package frpc

import (
	"fmt"
	"strings"

	"ashan-frp/internal/domain"
)

// FRPCConfig is the TOML structure for frpc.
type FRPCConfig struct {
	ServerAddr     string            `toml:"server_addr"`
	ServerPort     int               `toml:"server_port"`
	AuthToken      string            `toml:"auth_token,omitempty"`
	LogFile        string            `toml:"log_file,omitempty"`
	LogLevel       string            `toml:"log_level"`
	LogMaxDays     int               `toml:"log_max_days"`
	Proxies        []FRPCProxyConfig `toml:"proxies"`
}

// FRPCProxyConfig is a single proxy entry in frpc.toml.
type FRPCProxyConfig struct {
	Name          string `toml:"name"`
	Type          string `toml:"type"`
	LocalIP       string `toml:"local_ip"`
	LocalPort     int    `toml:"local_port"`
	RemotePort    int    `toml:"remote_port,omitempty"`
	Subdomain     string `toml:"subdomain,omitempty"`
	CustomDomains string `toml:"custom_domains,omitempty"`
	Encryption    bool   `toml:"encryption"`
	Compression   bool   `toml:"compression"`
}

// ConfigRenderer generates frpc.toml from domain state.
type ConfigRenderer struct {
	serverAddr string
	serverPort int
	authToken  string
	logLevel   string
	logMaxDays int
}

// NewConfigRenderer creates a ConfigRenderer with defaults.
func NewConfigRenderer(serverAddr string, serverPort int, authToken string) *ConfigRenderer {
	return &ConfigRenderer{
		serverAddr: serverAddr,
		serverPort: serverPort,
		authToken:  authToken,
		logLevel:   "info",
		logMaxDays: 3,
	}
}

// SetLogLevel overrides the log level.
func (r *ConfigRenderer) SetLogLevel(level string) {
	r.logLevel = level
}

// SetLogMaxDays overrides log retention.
func (r *ConfigRenderer) SetLogMaxDays(days int) {
	r.logMaxDays = days
}

// Render generates the FRPCConfig from a list of enabled tunnels.
func (r *ConfigRenderer) Render(tunnels []domain.Tunnel) (*FRPCConfig, error) {
	if r.serverAddr == "" {
		return nil, fmt.Errorf("frpc config: server_addr is required")
	}
	if r.serverPort <= 0 || r.serverPort > 65535 {
		return nil, fmt.Errorf("frpc config: server_port must be 1-65535, got %d", r.serverPort)
	}

	cfg := &FRPCConfig{
		ServerAddr: r.serverAddr,
		ServerPort: r.serverPort,
		AuthToken:  r.authToken,
		LogLevel:   r.logLevel,
		LogMaxDays: r.logMaxDays,
	}

	for _, t := range tunnels {
		if t.DesiredState != domain.TunnelDesiredEnabled {
			continue
		}
		proxy := FRPCProxyConfig{
			Name:       proxyName(t),
			Type:       proxyType(t.Protocol),
			LocalIP:    t.LocalIP,
			LocalPort:  t.LocalPort,
			RemotePort: t.RemotePort,
			Encryption: true,
			Compression: true,
		}
		if proxy.Type == "http" || proxy.Type == "https" {
			proxy.Subdomain = t.Subdomain
			proxy.CustomDomains = t.FullDomain
			proxy.RemotePort = 0 // http/https proxies don't use remote_port
		}
		cfg.Proxies = append(cfg.Proxies, proxy)
	}

	return cfg, nil
}

// proxyName generates a safe proxy name from a tunnel.
func proxyName(t domain.Tunnel) string {
	name := t.ProjectName
	if name == "" {
		name = t.Name
	}
	if name == "" {
		name = t.ID
	}
	// Replace characters that are problematic in TOML keys.
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, ".", "_")
	return name
}

// proxyType normalizes the protocol to frpc proxy type.
func proxyType(protocol string) string {
	switch strings.ToLower(protocol) {
	case "tcp", "udp", "http", "https", "stcp", "xtcp":
		return strings.ToLower(protocol)
	default:
		return "tcp"
	}
}
