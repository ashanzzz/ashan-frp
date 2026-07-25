package domain

import (
    "crypto/rand"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "strings"
    "time"
)

const (
    NodeStatusActive   = "active"
    NodeStatusDisabled = "disabled"
    NodeStatusArchived = "archived"

    HealthOnline   = "online"
    HealthDegraded = "degraded"
    HealthOffline  = "offline"
    HealthUnknown  = "unknown"

    TunnelDesiredEnabled  = "enabled"
    TunnelDesiredDisabled = "disabled"

    TunnelActualPending  = "pending"
    TunnelActualEnabled  = "enabled"
    TunnelActualDisabled = "disabled"
    TunnelActualError    = "error"

    WebsiteStatusPending  = "pending"
    WebsiteStatusSynced   = "synced"
    WebsiteStatusConflict = "conflict"
    WebsiteStatusArchived = "archived"

    JobStatusQueued     = "queued"
    JobStatusRunning    = "running"
    JobStatusRetryWait  = "retry_wait"
    JobStatusBlocked    = "blocked"
    JobStatusSucceeded  = "succeeded"
    JobStatusFailed     = "failed"
    JobStatusCanceled   = "canceled"
)

type APIError struct {
    Code      string `json:"code"`
    Message   string `json:"message"`
    Retryable bool   `json:"retryable"`
    Details   any    `json:"details,omitempty"`
}

type ResponseEnvelope struct {
    Data  any         `json:"data,omitempty"`
    Meta  ResponseMeta `json:"meta,omitempty"`
    Error *APIError   `json:"error,omitempty"`
}

type ResponseMeta struct {
    RequestID string       `json:"request_id,omitempty"`
    TraceID   string       `json:"trace_id,omitempty"`
    Job       *JobSummary  `json:"job,omitempty"`
}

type JobSummary struct {
    ID        string `json:"id"`
    Status    string `json:"status"`
    Channel   string `json:"channel"`
    Kind      string `json:"kind"`
    TargetType string `json:"target_type"`
    TargetID  string `json:"target_id"`
}

type SubjectRef struct {
    Type string `json:"type"`
    ID   string `json:"id"`
    Name string `json:"name,omitempty"`
}

type Event struct {
    SchemaVersion int         `json:"schema_version"`
    Channel       string      `json:"channel"`
    Kind          string      `json:"kind"`
    Cursor        string      `json:"cursor"`
    Level         string      `json:"level"`
    Message       string      `json:"message"`
    Job           *JobSummary `json:"job,omitempty" gorm:"-"`
    Subject       *SubjectRef `json:"subject,omitempty" gorm:"-"`
    Payload       any         `json:"payload,omitempty" gorm:"-"`
    Error         *APIError   `json:"error,omitempty" gorm:"-"`
    TraceID       string      `json:"trace_id,omitempty"`
    CreatedAt     time.Time   `json:"created_at"`
}

type Node struct {
	ID           string         `json:"id" gorm:"primaryKey;size:20"`
	DisplayName  string         `json:"display_name" gorm:"size:128"`
	Provider     string         `json:"provider" gorm:"size:32;index"`
	NodeType     string         `json:"node_type" gorm:"size:32"`
	EndpointURL  string         `json:"endpoint_url,omitempty" gorm:"size:512"`
	Region       string         `json:"region,omitempty" gorm:"size:64"`
	Status       string         `json:"status" gorm:"size:32"`
	HealthStatus string         `json:"health_status" gorm:"size:32"`
	CanonicalName string        `json:"canonical_name,omitempty" gorm:"size:128"`
	ExternalID   string         `json:"external_id,omitempty" gorm:"size:128"`
	Metadata     map[string]any  `json:"metadata,omitempty" gorm:"-"`
	MetadataJSON string         `json:"-" gorm:"type:text"`
	CreatedAt    time.Time       `json:"created_at"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

func (Node) TableName() string { return "nodes" }

func (n *Node) SerializeJSON() {
	if n.Metadata != nil {
		data, _ := json.Marshal(n.Metadata)
		n.MetadataJSON = string(data)
	}
}

func (n *Node) DeserializeJSON() {
	if n.MetadataJSON != "" {
		json.Unmarshal([]byte(n.MetadataJSON), &n.Metadata)
	}
}

type NodeInput struct {
    DisplayName   string         `json:"display_name"`
    Provider      string         `json:"provider"`
    NodeType      string         `json:"node_type"`
    EndpointURL   string         `json:"endpoint_url,omitempty"`
    Region        string         `json:"region,omitempty"`
    Status        string         `json:"status,omitempty"`
    CanonicalName string         `json:"canonical_name,omitempty"`
    Metadata      map[string]any  `json:"metadata,omitempty"`
}

type NodeListFilter struct {
    Q               string
    Provider        string
    Status          string
    HealthStatus    string
    IncludeArchived bool
}

type Tunnel struct {
	ID                 string     `json:"id"`
	NodeID             string     `json:"node_id"`
	ProjectName        string     `json:"project_name,omitempty" gorm:"size:128"`
	Subdomain          string     `json:"subdomain,omitempty" gorm:"size:128"`
	FullDomain         string     `json:"full_domain,omitempty" gorm:"index;size:256"`
	Protocol           string     `json:"protocol,omitempty" gorm:"size:16"`
	Name               string     `json:"name"`
	TunnelType         string     `json:"tunnel_type"`
	DesiredState       string     `json:"desired_state"`
	LocalIP            string     `json:"local_ip"`
	LocalPort          int        `json:"local_port"`
	RemotePort         int        `json:"remote_port,omitempty"`
	DNSDomainCNAME     string     `json:"dns_domain_cname,omitempty"`
	DNSProxied         bool       `json:"dns_proxied,omitempty"`
	ChmlfrpNode        string     `json:"chmlfrp_node,omitempty" gorm:"size:64"`
	ChmlfrpTunnelName  string     `json:"chmlfrp_tunnel_name,omitempty" gorm:"size:128"`
	ChmlfrpTunnelID    string     `json:"chmlfrp_tunnel_id,omitempty" gorm:"size:64"`
	CFProxied          bool       `json:"cf_proxied,omitempty"`
	CFRecordID         string     `json:"cf_record_id,omitempty" gorm:"size:64"`
	OnePanelWebsiteID   int        `json:"onepanel_website_id,omitempty"`
	OnePanelSSLEnabled  bool       `json:"onepanel_ssl_enabled,omitempty"`
	OnePanelProxyTarget string     `json:"onepanel_proxy_target,omitempty" gorm:"size:256"`
	IsFailoverPool      bool       `json:"is_failover_pool" gorm:"default:false"`
	FailoverPriority    int        `json:"failover_priority" gorm:"default:0"`
	LastHealthCheckedAt *time.Time `json:"last_health_checked_at,omitempty"`
	NodeRealIP          string     `json:"node_real_ip,omitempty" gorm:"size:64"`
	CreatedBy           string     `json:"created_by,omitempty" gorm:"size:20"`
	ActualState        string     `json:"actual_state"`
	StateReason        string     `json:"state_reason,omitempty"`
	ManualOverride     bool       `json:"manual_override,omitempty"`
	RuntimeKey         string     `json:"runtime_key,omitempty"`
	LastAppliedAt      *time.Time `json:"last_applied_at,omitempty"`
	LastErrorCode      string     `json:"last_error_code,omitempty"`
	LastErrorMessage   string     `json:"last_error_message,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

type TunnelInput struct {
	NodeID         string `json:"node_id"`
	Name           string `json:"name"`
	TunnelType     string `json:"tunnel_type"`
	DesiredState   string `json:"desired_state,omitempty"`
	LocalIP        string `json:"local_ip"`
	LocalPort      int    `json:"local_port"`
	RemotePort     int    `json:"remote_port,omitempty"`
	DNSDomainCNAME string `json:"dns_domain_cname,omitempty"`
	DNSProxied     bool   `json:"dns_proxied,omitempty"`
	ProjectName    string `json:"project_name,omitempty"`
	Subdomain      string `json:"subdomain,omitempty"`
	Protocol       string `json:"protocol,omitempty"`
	ChmlfrpNode    string `json:"chmlfrp_node,omitempty"`
	CFProxied      bool   `json:"cf_proxied,omitempty"`
}

type TunnelListFilter struct {
    Q              string
    NodeID         string
    DesiredState   string
    DiffStatus     string
    ManualOverride bool
}

type WebsiteMapping struct {
	ID                 string         `json:"id" gorm:"primaryKey;size:20"`
	SourceKind         string         `json:"source_kind" gorm:"size:32"`
	NodeID             string         `json:"node_id" gorm:"size:20;index"`
	TunnelID           string         `json:"tunnel_id,omitempty" gorm:"size:20"`
	SourceExternalID   string         `json:"source_external_id,omitempty" gorm:"size:128"`
	WebsiteAlias       string         `json:"website_alias,omitempty" gorm:"size:128"`
	PrimaryDomain      string         `json:"primary_domain" gorm:"size:256;index"`
	Domains            []string       `json:"domains" gorm:"-"`
	DomainsJSON        string         `json:"-" gorm:"type:text"`
	HTTPSEnabled       bool           `json:"https_enabled"`
	CertificateMode    string         `json:"certificate_mode,omitempty" gorm:"size:32"`
	SSLCertificateRef  string         `json:"ssl_certificate_ref,omitempty" gorm:"size:128"`
	ProxyEnabled       bool           `json:"proxy_enabled"`
	CacheEnabled       bool           `json:"cache_enabled"`
	ProxyTarget        string         `json:"proxy_target,omitempty" gorm:"size:256"`
	HTTPConfig         map[string]any `json:"http_config,omitempty" gorm:"-"`
	HTTPConfigJSON     string         `json:"-" gorm:"type:text"`
	ConflictStrategy   string         `json:"conflict_strategy,omitempty" gorm:"size:64"`
	Status             string         `json:"status" gorm:"size:32"`
	PanelWebsiteID     string         `json:"panel_website_id,omitempty" gorm:"size:64"`
	LastSyncedAt       *time.Time     `json:"last_synced_at,omitempty"`
	LastErrorCode      string         `json:"last_error_code,omitempty" gorm:"size:64"`
	LastErrorMessage   string         `json:"last_error_message,omitempty" gorm:"size:512"`
	RuntimeKey         string         `json:"runtime_key,omitempty" gorm:"size:128"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
}

func (WebsiteMapping) TableName() string { return "website_mappings" }

// SerializeJSON converts Go fields to JSON strings for GORM storage.
func (w *WebsiteMapping) SerializeJSON() {
	if w.Domains != nil {
		data, _ := json.Marshal(w.Domains)
		w.DomainsJSON = string(data)
	}
	if w.HTTPConfig != nil {
		data, _ := json.Marshal(w.HTTPConfig)
		w.HTTPConfigJSON = string(data)
	}
}

// DeserializeJSON converts JSON strings back to Go fields after GORM load.
func (w *WebsiteMapping) DeserializeJSON() {
	if w.DomainsJSON != "" {
		json.Unmarshal([]byte(w.DomainsJSON), &w.Domains)
	}
	if w.HTTPConfigJSON != "" {
		json.Unmarshal([]byte(w.HTTPConfigJSON), &w.HTTPConfig)
	}
}

type WebsiteMappingInput struct {
    SourceKind        string         `json:"source_kind"`
    NodeID            string         `json:"node_id"`
    TunnelID          string         `json:"tunnel_id,omitempty"`
    SourceExternalID  string         `json:"source_external_id,omitempty"`
    WebsiteAlias      string         `json:"website_alias,omitempty"`
    PrimaryDomain     string         `json:"primary_domain"`
    Domains           []string       `json:"domains"`
    HTTPSEnabled      bool           `json:"https_enabled"`
    CertificateMode   string         `json:"certificate_mode,omitempty"`
    SSLCertificateRef string         `json:"ssl_certificate_ref,omitempty"`
    ProxyEnabled      bool           `json:"proxy_enabled"`
    CacheEnabled      bool           `json:"cache_enabled"`
    ProxyTarget       string         `json:"proxy_target,omitempty"`
    HTTPConfig        map[string]any `json:"http_config,omitempty"`
    ConflictStrategy  string         `json:"conflict_strategy,omitempty"`
}

type WebsiteMappingListFilter struct {
    Q              string
    NodeID         string
    HTTPSEnabled   *bool
    Status         string
    IncludeArchived bool
}

type GeneralSettings struct {
    DefaultLogLines    int    `json:"default_log_lines"`
    DataRetentionDays  int    `json:"data_retention_days"`
    DefaultRefreshMode  string `json:"default_refresh_mode"`
}

type SyncSettings struct {
    HealthcheckInterval   string `json:"healthcheck_interval"`
    SyncPollInterval      string `json:"sync_poll_interval"`
    DiffStrategy          string `json:"diff_strategy"`
    ManualOverridePriority string `json:"manual_override_priority"`
}

type QueueSettings struct {
    MaxAttempts           int    `json:"max_attempts"`
    RetryBackoff          string `json:"retry_backoff"`
    StalledJobPolicy      string `json:"stalled_job_policy"`
    ArchiveRetentionDays  int    `json:"archive_retention_days"`
}

type FRPCRuntimeSettings struct {
    Enabled             bool   `json:"frpc_enabled"`
    BinarySource        string `json:"frpc_binary_source"`
    BinaryVersion       string `json:"frpc_binary_version"`
    LogLevel            string `json:"frpc_log_level"`
    HealthcheckInterval string `json:"frpc_healthcheck_interval"`
    RestartBackoff      string `json:"frpc_restart_backoff"`
    AutoRecoverStrategy string `json:"auto_recover_strategy"`
    SwitchNodeStrategy  string `json:"switch_node_strategy"`
}

type ChmlFrpCredentials struct {
    Username         string     `json:"username,omitempty"`
    Password         string     `json:"password,omitempty"`
    HasPassword      bool       `json:"has_password,omitempty"`
    UpdatedAt        time.Time  `json:"updated_at,omitempty"`
    LastValidatedAt  *time.Time `json:"last_validated_at,omitempty"`
    LastErrorMessage string     `json:"last_error_message,omitempty"`
}

type OnePanelCredentials struct {
    BaseURL          string     `json:"base_url,omitempty"`
    Entrance         string     `json:"entrance,omitempty"`
    APIToken         string     `json:"api_token,omitempty"`
    HasAPIToken      bool       `json:"has_api_token,omitempty"`
    UpdatedAt        time.Time  `json:"updated_at,omitempty"`
    LastValidatedAt  *time.Time `json:"last_validated_at,omitempty"`
    LastErrorMessage string     `json:"last_error_message,omitempty"`
}

type CloudflareCredentials struct {
	APIToken         string     `json:"api_token,omitempty"`
	ZoneName         string     `json:"zone_name,omitempty"`
	HasAPIToken      bool       `json:"has_api_token,omitempty"`
	UpdatedAt        time.Time  `json:"updated_at,omitempty"`
	LastValidatedAt  *time.Time `json:"last_validated_at,omitempty"`
	LastErrorMessage string     `json:"last_error_message,omitempty"`
	TokenMask          string     `json:"token_mask,omitempty"`
	CredentialRef      string     `json:"credential_ref,omitempty"`
	CredentialRevision int        `json:"credential_revision,omitempty"`
}

type IntegrationSettings struct {
    ChmlFrp    ChmlFrpCredentials    `json:"chmlfrp"`
    OnePanel   OnePanelCredentials   `json:"onepanel"`
    Cloudflare CloudflareCredentials `json:"cloudflare"`
}

type Settings struct {
    General      GeneralSettings     `json:"general"`
    Sync         SyncSettings        `json:"sync"`
    Queue        QueueSettings       `json:"queue"`
    FRPCRuntime  FRPCRuntimeSettings `json:"frpc_runtime"`
    Integrations IntegrationSettings `json:"integrations"`
    UpdatedAt    time.Time           `json:"updated_at"`
}

type SettingsPatchRequest struct {
    General      GeneralSettings     `json:"general"`
    Sync         SyncSettings        `json:"sync"`
    Queue        QueueSettings       `json:"queue"`
    FRPCRuntime  FRPCRuntimeSettings `json:"frpc_runtime"`
    Integrations IntegrationSettings `json:"integrations"`
}

type Job struct {
	ID          string         `json:"id"`
	Kind        string         `json:"kind"`
	TargetType  string         `json:"target_type"`
	TargetID    string         `json:"target_id"`
	Channel     string         `json:"channel"`
	Status      string         `json:"status"`
	AttemptCount int           `json:"attempt_count"`
	MaxAttempts int            `json:"max_attempts"`
	Title       string         `json:"title"`
	PayloadJSON string         `json:"payload_json,omitempty" gorm:"type:text"`
	ResultJSON  string         `json:"result_json,omitempty" gorm:"type:text"`
	Retryable   bool           `json:"retryable,omitempty"`
	NextRetryAt *time.Time     `json:"next_retry_at,omitempty"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
	ErrorMessage string        `json:"error_message,omitempty"`
	CreatedBy   string         `json:"created_by,omitempty" gorm:"size:20"`
	Payload     any            `json:"payload,omitempty" gorm:"-"`
	Result      any            `json:"result,omitempty" gorm:"-"`
	Error       *APIError      `json:"error,omitempty" gorm:"-"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
}

type State struct {
    Version         string          `json:"version"`
    Nodes           []Node          `json:"nodes"`
    Tunnels         []Tunnel        `json:"tunnels"`
    WebsiteMappings []WebsiteMapping `json:"website_mappings"`
    Settings        Settings        `json:"settings"`
    Jobs            []Job           `json:"jobs"`
    Events          []Event         `json:"events"`
}

type RuntimeSummary struct {
    FRPCVersion       string    `json:"frpc_version"`
    EngineStatus      string    `json:"engine_status"`
    ActiveTunnelsCount int       `json:"active_tunnels_count"`
    ActiveTunnelIDs   []string  `json:"active_tunnel_ids"`
    LastAction        string    `json:"last_action,omitempty"`
    UpdatedAt         time.Time `json:"updated_at"`
}

func NewID(prefix string) string {
    b := make([]byte, 8)
    _, _ = rand.Read(b)
    return fmt.Sprintf("%s_%s", prefix, hex.EncodeToString(b))
}

func (j Job) Summary() JobSummary {
    return JobSummary{
        ID:        j.ID,
        Status:    j.Status,
        Channel:   j.Channel,
        Kind:      j.Kind,
        TargetType: j.TargetType,
        TargetID:  j.TargetID,
    }
}

func (n Node) Match(q string) bool {
    if q == "" {
        return true
    }
    q = strings.ToLower(q)
    fields := []string{n.ID, n.DisplayName, n.Provider, n.NodeType, n.EndpointURL, n.Region, n.Status, n.HealthStatus, n.CanonicalName, n.ExternalID}
    for _, f := range fields {
        if strings.Contains(strings.ToLower(f), q) {
            return true
        }
    }
    return false
}

func (t Tunnel) Match(q string) bool {
    if q == "" {
        return true
    }
    q = strings.ToLower(q)
    fields := []string{t.ID, t.NodeID, t.Name, t.TunnelType, t.DesiredState, t.LocalIP, t.DNSDomainCNAME, t.ActualState, t.StateReason, t.RuntimeKey}
    for _, f := range fields {
        if strings.Contains(strings.ToLower(f), q) {
            return true
        }
    }
    return false
}

func (w WebsiteMapping) Match(q string) bool {
    if q == "" {
        return true
    }
    q = strings.ToLower(q)
    fields := []string{w.ID, w.SourceKind, w.NodeID, w.TunnelID, w.SourceExternalID, w.WebsiteAlias, w.PrimaryDomain, w.Status, w.PanelWebsiteID, w.RuntimeKey}
    for _, f := range fields {
        if strings.Contains(strings.ToLower(f), q) {
            return true
        }
    }
    for _, d := range w.Domains {
        if strings.Contains(strings.ToLower(d), q) {
            return true
        }
    }
    return false
}

func SeedState() State {
    now := time.Now().UTC().Truncate(time.Second)
    older := now.Add(-24 * time.Hour)
    lastApplied := now.Add(-10 * time.Minute)
    lastSynced := now.Add(-5 * time.Minute)

    node1 := Node{
        ID:            "node_hk_bgp",
        DisplayName:   "香港 BGP 极速 [国际]",
        Provider:      "chmlfrp",
        NodeType:      "frp_node",
        EndpointURL:   "https://node-hk.example.com",
        Region:        "hongkong",
        Status:        NodeStatusActive,
        HealthStatus:  HealthOnline,
        CanonicalName: "hk-bgp-fast",
        Metadata:      map[string]any{"location": "中国香港", "load": 18, "tags": []string{"intl", "bgp"}},
        CreatedAt:     older,
        UpdatedAt:     now,
    }
    node1.SerializeJSON()

    node2 := Node{
        ID:            "node_xz_gf",
        DisplayName:   "徐州电信高防 [双线]",
        Provider:      "chmlfrp",
        NodeType:      "frp_node",
        EndpointURL:   "https://node-xz.example.com",
        Region:        "xuzhou",
        Status:        NodeStatusActive,
        HealthStatus:  HealthDegraded,
        CanonicalName: "xz-gf-dual",
        Metadata:      map[string]any{"location": "中国江苏", "load": 32, "tags": []string{"ddos", "dual"}},
        CreatedAt:     older,
        UpdatedAt:     now,
    }
    node2.SerializeJSON()

    tunnel1 := Tunnel{
        ID:               "tun_npm",
        NodeID:           node1.ID,
        Name:             "npm-registry",
        TunnelType:       "http",
        DesiredState:     TunnelDesiredEnabled,
        LocalIP:          "127.0.0.1",
        LocalPort:        3000,
        DNSDomainCNAME:   "npm.example.com",
        DNSProxied:       true,
        ActualState:      TunnelActualEnabled,
        StateReason:      "seeded",
        RuntimeKey:       "frpc-node_hk_bgp-3000",
        LastAppliedAt:    &lastApplied,
        CreatedAt:        older,
        UpdatedAt:        now,
    }

    tunnel2 := Tunnel{
        ID:               "tun_api",
        NodeID:           node2.ID,
        Name:             "api-gateway",
        TunnelType:       "tcp",
        DesiredState:     TunnelDesiredEnabled,
        LocalIP:          "127.0.0.1",
        LocalPort:        8080,
        RemotePort:       7001,
        ActualState:      TunnelActualPending,
        StateReason:      "awaiting apply",
        RuntimeKey:       "frpc-node_xz_gf-8080",
        CreatedAt:        older,
        UpdatedAt:        now,
    }

    website := WebsiteMapping{
		ID:                "web_npm",
		SourceKind:        "tunnel",
		NodeID:            node1.ID,
		TunnelID:          tunnel1.ID,
		WebsiteAlias:      "npm-site",
		PrimaryDomain:     "npm.example.com",
		Domains:           []string{"npm.example.com", "registry.example.com"},
		HTTPSEnabled:      true,
		CertificateMode:   "auto",
		ProxyEnabled:      true,
		CacheEnabled:      false,
		ProxyTarget:       "http://127.0.0.1:3000",
		HTTPConfig:        map[string]any{"body_size_limit": "20m", "read_timeout": "30s"},
		ConflictStrategy:   "pause_on_conflict",
		Status:            WebsiteStatusSynced,
		PanelWebsiteID:    "panel_web_001",
		LastSyncedAt:      &lastSynced,
		RuntimeKey:        "site-npm-example-com",
		CreatedAt:         older,
		UpdatedAt:         now,
	}
	website.SerializeJSON()

    settings := Settings{
        General: GeneralSettings{DefaultLogLines: 100, DataRetentionDays: 30, DefaultRefreshMode: "polling"},
        Sync: SyncSettings{HealthcheckInterval: "1m", SyncPollInterval: "10s", DiffStrategy: "pause_on_conflict", ManualOverridePriority: "manual_wins"},
        Queue: QueueSettings{MaxAttempts: 5, RetryBackoff: "30s", StalledJobPolicy: "mark_blocked", ArchiveRetentionDays: 30},
        FRPCRuntime: FRPCRuntimeSettings{Enabled: true, BinarySource: "embedded", BinaryVersion: "0.54.0", LogLevel: "info", HealthcheckInterval: "30s", RestartBackoff: "30s", AutoRecoverStrategy: "reload_then_restart", SwitchNodeStrategy: "prefer_healthy_low_load"},
        UpdatedAt: now,
    }

    evt := Event{
        SchemaVersion: 1,
        Channel:       "account:current",
        Kind:          "system.seeded",
        Cursor:        "seed-0001",
        Level:         "info",
        Message:       "seed state loaded",
        Subject:       &SubjectRef{Type: "system", ID: "bootstrap", Name: "bootstrap"},
        TraceID:       "trace-seed",
        CreatedAt:     now,
    }

    return State{
        Version:         "0.1.0",
        Nodes:           []Node{node1, node2},
        Tunnels:         []Tunnel{tunnel1, tunnel2},
        WebsiteMappings: []WebsiteMapping{website},
        Settings:        settings,
        Jobs:            []Job{},
        Events:          []Event{evt},
    }
}

