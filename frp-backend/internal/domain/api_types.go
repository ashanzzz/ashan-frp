package domain

import "time"

type CredentialStatus struct {
	Configured         bool       `json:"configured"`
	MaskHint           string     `json:"mask_hint,omitempty"`
	CredentialRef      string     `json:"credential_ref,omitempty"`
	CredentialRevision int        `json:"credential_revision,omitempty"`
	Identifier         string     `json:"identifier,omitempty"`
	LastVerified       *time.Time `json:"last_verified_at,omitempty"`
	LastError          string     `json:"last_error,omitempty"`
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

type SystemMetrics struct {
	Goroutines       int    `json:"goroutines"`
	MemoryAllocMB    uint64 `json:"memory_alloc_mb"`
	MemorySysMB      uint64 `json:"memory_sys_mb"`
	SQLiteOpenConns  int    `json:"sqlite_open_conns"`
	SQLiteInUseConns int    `json:"sqlite_in_use_conns"`
	UptimeSeconds    int64  `json:"uptime_seconds"`
}

type HealthInfo struct {
	Status          string        `json:"status"`
	Tunnels         int           `json:"tunnels"`
	Jobs            int           `json:"jobs"`
	QueuedJobs      int           `json:"queued_jobs"`
	RunningJobs     int           `json:"running_jobs"`
	FailedJobs      int           `json:"failed_jobs"`
	Nodes           int           `json:"nodes"`
	WebsiteMappings int           `json:"website_mappings"`
	SyncStates      int           `json:"sync_states"`
	SystemMetrics   SystemMetrics `json:"system_metrics"`
}

type DashboardData struct {
	Version         VersionInfo `json:"version"`
	Health          HealthInfo  `json:"health"`
	Tunnels         []Tunnel    `json:"tunnels"`
	Jobs            []Job       `json:"jobs"`
	Settings        SettingsDTO `json:"settings"`
	RecentAuditLogs []AuditLog  `json:"recent_audit_logs"`
}
