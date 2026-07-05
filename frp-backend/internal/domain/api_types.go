package domain

import "time"

type CredentialStatus struct {
	Configured  bool       `json:"configured"`
	MaskHint    string     `json:"mask_hint,omitempty"`
	Identifier  string     `json:"identifier,omitempty"`
	Secret      string     `json:"secret,omitempty"`
	LastVerified *time.Time `json:"last_verified_at,omitempty"`
	LastError   string     `json:"last_error,omitempty"`
}

type IntegrationStatusDTO struct {
	Chmlfrp    CredentialStatus `json:"chmlfrp"`
	OnePanel   CredentialStatus `json:"onepanel"`
	Cloudflare CredentialStatus `json:"cloudflare"`
}

type SettingsDTO struct {
	General      GeneralSettings      `json:"general"`
	Sync         SyncSettings         `json:"sync"`
	Queue        QueueSettings        `json:"queue"`
	FRPCRuntime  FRPCRuntimeSettings  `json:"frpc_runtime"`
	Integrations IntegrationStatusDTO `json:"integrations"`
}

type VersionInfo struct {
	Version string `json:"version"`
	Engine  string `json:"engine"`
	AppName string `json:"app_name"`
	Status  string `json:"status"`
	APIBase string `json:"api_base"`
	UIBase  string `json:"ui_base"`
}

type HealthInfo struct {
	Status  string `json:"status"`
	Tunnels int    `json:"tunnels"`
	Jobs    int    `json:"jobs"`
}

type DashboardData struct {
	Version         VersionInfo  `json:"version"`
	Health          HealthInfo   `json:"health"`
	Tunnels         []Tunnel     `json:"tunnels"`
	Jobs            []Job        `json:"jobs"`
	Settings        SettingsDTO  `json:"settings"`
	RecentAuditLogs []AuditLog   `json:"recent_audit_logs"`
}
