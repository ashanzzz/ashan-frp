package store

import (
    "bytes"
    "encoding/json"
    "errors"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "sync"
    "time"

    "ashan-frp/internal/domain"
)

type Store struct {
    mu    sync.RWMutex
    path  string
    seq   uint64
    state domain.State
}

func Load(path string) (*Store, error) {
    st := &Store{path: path}
    if err := st.loadOrSeed(); err != nil {
        return nil, err
    }
    return st, nil
}

func (s *Store) loadOrSeed() error {
    if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
        return err
    }
    raw, err := os.ReadFile(s.path)
    if err == nil && len(bytes.TrimSpace(raw)) > 0 {
        if err := json.Unmarshal(raw, &s.state); err != nil {
            return err
        }
        if s.state.Version == "" {
            s.state.Version = "0.1.0"
        }
        return nil
    }
    s.state = domain.SeedState()
    return s.saveLocked()
}

func (s *Store) saveLocked() error {
    data, err := json.MarshalIndent(s.state, "", "  ")
    if err != nil {
        return err
    }
    tmp := s.path + ".tmp"
    if err := os.WriteFile(tmp, data, 0o644); err != nil {
        return err
    }
    return os.Rename(tmp, s.path)
}

func (s *Store) Snapshot() domain.State {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.state
}

func (s *Store) Version() string {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.state.Version
}

func (s *Store) nextCursorLocked() string {
    s.seq++
    return fmt.Sprintf("%d-%06d", time.Now().UTC().UnixNano(), s.seq)
}

func (s *Store) appendEventLocked(evt domain.Event) domain.Event {
    if evt.Cursor == "" {
        evt.Cursor = s.nextCursorLocked()
    }
    if evt.CreatedAt.IsZero() {
        evt.CreatedAt = time.Now().UTC()
    }
    s.state.Events = append(s.state.Events, evt)
    if len(s.state.Events) > 500 {
        s.state.Events = s.state.Events[len(s.state.Events)-500:]
    }
    return evt
}

func (s *Store) AppendEvent(evt domain.Event) (domain.Event, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    evt = s.appendEventLocked(evt)
    return evt, s.saveLocked()
}

func (s *Store) ListEvents(channel, afterCursor string, limit int) []domain.Event {
    s.mu.RLock()
    defer s.mu.RUnlock()
    if limit <= 0 {
        limit = 100
    }
    var start int
    if afterCursor != "" {
        for i, evt := range s.state.Events {
            if evt.Cursor == afterCursor {
                start = i + 1
                break
            }
        }
    }
    out := make([]domain.Event, 0, limit)
    for i := start; i < len(s.state.Events); i++ {
        evt := s.state.Events[i]
        if channel != "" && evt.Channel != channel {
            continue
        }
        out = append(out, evt)
        if len(out) >= limit {
            break
        }
    }
    return out
}

func (s *Store) listJobsLocked(filter func(domain.Job) bool) []domain.Job {
    out := make([]domain.Job, 0, len(s.state.Jobs))
    for _, job := range s.state.Jobs {
        if filter == nil || filter(job) {
            out = append(out, job)
        }
    }
    sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
    return out
}

func (s *Store) ListJobs(status, kind, targetType, targetID string) []domain.Job {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.listJobsLocked(func(job domain.Job) bool {
        if status != "" && job.Status != status {
            return false
        }
        if kind != "" && job.Kind != kind {
            return false
        }
        if targetType != "" && job.TargetType != targetType {
            return false
        }
        if targetID != "" && job.TargetID != targetID {
            return false
        }
        return true
    })
}

func (s *Store) GetJob(id string) (domain.Job, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    for _, job := range s.state.Jobs {
        if job.ID == id {
            return job, true
        }
    }
    return domain.Job{}, false
}

func (s *Store) CreateJob(kind, targetType, targetID, title, channel string, payload any) (domain.Job, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    now := time.Now().UTC()
    job := domain.Job{
        ID:           domain.NewID("job"),
        Kind:         kind,
        TargetType:   targetType,
        TargetID:     targetID,
        Channel:      channel,
        Status:       domain.JobStatusQueued,
        AttemptCount: 1,
        MaxAttempts:  3,
        Title:        title,
        Payload:      payload,
        CreatedAt:    now,
        UpdatedAt:    now,
    }
    s.state.Jobs = append(s.state.Jobs, job)
    return job, s.saveLocked()
}

func (s *Store) UpdateJob(id string, fn func(*domain.Job) error) (domain.Job, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    for i := range s.state.Jobs {
        if s.state.Jobs[i].ID == id {
            if err := fn(&s.state.Jobs[i]); err != nil {
                return domain.Job{}, err
            }
            s.state.Jobs[i].UpdatedAt = time.Now().UTC()
            return s.state.Jobs[i], s.saveLocked()
        }
    }
    return domain.Job{}, errors.New("job not found")
}

func (s *Store) UpsertNode(id string, input domain.NodeInput) (domain.Node, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    now := time.Now().UTC()
    if id == "" {
        id = domain.NewID("node")
    }
    var node domain.Node
    found := false
    for i := range s.state.Nodes {
        if s.state.Nodes[i].ID == id {
            node = s.state.Nodes[i]
            found = true
            node.DisplayName = input.DisplayName
            node.Provider = input.Provider
            node.NodeType = input.NodeType
            node.EndpointURL = input.EndpointURL
            node.Region = input.Region
            node.Status = input.Status
            node.CanonicalName = input.CanonicalName
            node.Metadata = input.Metadata
            if node.CreatedAt.IsZero() {
                node.CreatedAt = now
            }
            node.UpdatedAt = now
            s.state.Nodes[i] = node
            break
        }
    }
    if !found {
        node = domain.Node{
            ID:            id,
            DisplayName:   input.DisplayName,
            Provider:      input.Provider,
            NodeType:      input.NodeType,
            EndpointURL:   input.EndpointURL,
            Region:        input.Region,
            Status:        pickString(input.Status, domain.NodeStatusActive),
            HealthStatus:  domain.HealthUnknown,
            CanonicalName: input.CanonicalName,
            Metadata:      input.Metadata,
            CreatedAt:     now,
            UpdatedAt:     now,
        }
        if node.HealthStatus == "" {
            node.HealthStatus = domain.HealthUnknown
        }
        s.state.Nodes = append(s.state.Nodes, node)
    }
    if node.HealthStatus == "" {
        node.HealthStatus = domain.HealthUnknown
    }
    if err := s.saveLocked(); err != nil {
        return domain.Node{}, err
    }
    return node, nil
}

func (s *Store) UpdateNode(id string, fn func(*domain.Node) error) (domain.Node, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    for i := range s.state.Nodes {
        if s.state.Nodes[i].ID == id {
            if err := fn(&s.state.Nodes[i]); err != nil {
                return domain.Node{}, err
            }
            s.state.Nodes[i].UpdatedAt = time.Now().UTC()
            return s.state.Nodes[i], s.saveLocked()
        }
    }
    return domain.Node{}, errors.New("node not found")
}

func (s *Store) GetNode(id string) (domain.Node, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    for _, node := range s.state.Nodes {
        if node.ID == id {
            return node, true
        }
    }
    return domain.Node{}, false
}

func (s *Store) ListNodes(filter domain.NodeListFilter) []domain.Node {
    s.mu.RLock()
    defer s.mu.RUnlock()
    out := make([]domain.Node, 0, len(s.state.Nodes))
    for _, node := range s.state.Nodes {
        if !filter.IncludeArchived && node.Status == domain.NodeStatusArchived {
            continue
        }
        if filter.Provider != "" && !strings.EqualFold(node.Provider, filter.Provider) {
            continue
        }
        if filter.Status != "" && !strings.EqualFold(node.Status, filter.Status) {
            continue
        }
        if filter.HealthStatus != "" && !strings.EqualFold(node.HealthStatus, filter.HealthStatus) {
            continue
        }
        if !node.Match(filter.Q) {
            continue
        }
        out = append(out, node)
    }
    sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
    return out
}

func (s *Store) UpsertTunnel(id string, input domain.TunnelInput) (domain.Tunnel, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    now := time.Now().UTC()
    if id == "" {
        id = domain.NewID("tun")
    }
    var tunnel domain.Tunnel
    found := false
    for i := range s.state.Tunnels {
        if s.state.Tunnels[i].ID == id {
            tunnel = s.state.Tunnels[i]
            found = true
            tunnel.NodeID = input.NodeID
            tunnel.Name = input.Name
            tunnel.TunnelType = input.TunnelType
            tunnel.DesiredState = pickString(input.DesiredState, domain.TunnelDesiredEnabled)
            tunnel.LocalIP = input.LocalIP
            tunnel.LocalPort = input.LocalPort
            tunnel.RemotePort = input.RemotePort
            tunnel.DNSDomainCNAME = input.DNSDomainCNAME
            tunnel.DNSProxied = input.DNSProxied
            tunnel.UpdatedAt = now
            s.state.Tunnels[i] = tunnel
            break
        }
    }
    if !found {
        tunnel = domain.Tunnel{
            ID:               id,
            NodeID:           input.NodeID,
            Name:             input.Name,
            TunnelType:       input.TunnelType,
            DesiredState:     pickString(input.DesiredState, domain.TunnelDesiredEnabled),
            LocalIP:          input.LocalIP,
            LocalPort:        input.LocalPort,
            RemotePort:       input.RemotePort,
            DNSDomainCNAME:   input.DNSDomainCNAME,
            DNSProxied:       input.DNSProxied,
            ActualState:      domain.TunnelActualPending,
            StateReason:      "awaiting apply",
            RuntimeKey:       fmt.Sprintf("frpc-%s-%d", input.NodeID, input.LocalPort),
            CreatedAt:        now,
            UpdatedAt:        now,
        }
        s.state.Tunnels = append(s.state.Tunnels, tunnel)
    }
    if err := s.saveLocked(); err != nil {
        return domain.Tunnel{}, err
    }
    return tunnel, nil
}

func (s *Store) UpdateTunnel(id string, fn func(*domain.Tunnel) error) (domain.Tunnel, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    for i := range s.state.Tunnels {
        if s.state.Tunnels[i].ID == id {
            if err := fn(&s.state.Tunnels[i]); err != nil {
                return domain.Tunnel{}, err
            }
            s.state.Tunnels[i].UpdatedAt = time.Now().UTC()
            return s.state.Tunnels[i], s.saveLocked()
        }
    }
    return domain.Tunnel{}, errors.New("tunnel not found")
}

func (s *Store) GetTunnel(id string) (domain.Tunnel, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    for _, tunnel := range s.state.Tunnels {
        if tunnel.ID == id {
            return tunnel, true
        }
    }
    return domain.Tunnel{}, false
}

func (s *Store) ListTunnels(filter domain.TunnelListFilter) []domain.Tunnel {
    s.mu.RLock()
    defer s.mu.RUnlock()
    out := make([]domain.Tunnel, 0, len(s.state.Tunnels))
    for _, tunnel := range s.state.Tunnels {
        if filter.NodeID != "" && tunnel.NodeID != filter.NodeID {
            continue
        }
        if filter.DesiredState != "" && !strings.EqualFold(tunnel.DesiredState, filter.DesiredState) {
            continue
        }
        if filter.ManualOverride && !tunnel.ManualOverride {
            continue
        }
        if !tunnel.Match(filter.Q) {
            continue
        }
        out = append(out, tunnel)
    }
    sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
    return out
}

func (s *Store) UpsertWebsiteMapping(id string, input domain.WebsiteMappingInput) (domain.WebsiteMapping, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    now := time.Now().UTC()
    if id == "" {
        id = domain.NewID("web")
    }
    var wm domain.WebsiteMapping
    found := false
    for i := range s.state.WebsiteMappings {
        if s.state.WebsiteMappings[i].ID == id {
            wm = s.state.WebsiteMappings[i]
            found = true
            wm.SourceKind = input.SourceKind
            wm.NodeID = input.NodeID
            wm.TunnelID = input.TunnelID
            wm.SourceExternalID = input.SourceExternalID
            wm.WebsiteAlias = input.WebsiteAlias
            wm.PrimaryDomain = input.PrimaryDomain
            wm.Domains = append([]string(nil), input.Domains...)
            wm.HTTPSEnabled = input.HTTPSEnabled
            wm.CertificateMode = input.CertificateMode
            wm.SSLCertificateRef = input.SSLCertificateRef
            wm.ProxyEnabled = input.ProxyEnabled
            wm.CacheEnabled = input.CacheEnabled
            wm.ProxyTarget = input.ProxyTarget
            wm.HTTPConfig = input.HTTPConfig
            wm.ConflictStrategy = input.ConflictStrategy
            wm.UpdatedAt = now
            s.state.WebsiteMappings[i] = wm
            break
        }
    }
    if !found {
        wm = domain.WebsiteMapping{
            ID:                id,
            SourceKind:        input.SourceKind,
            NodeID:            input.NodeID,
            TunnelID:          input.TunnelID,
            SourceExternalID:  input.SourceExternalID,
            WebsiteAlias:      input.WebsiteAlias,
            PrimaryDomain:     input.PrimaryDomain,
            Domains:           append([]string(nil), input.Domains...),
            HTTPSEnabled:      input.HTTPSEnabled,
            CertificateMode:   input.CertificateMode,
            SSLCertificateRef: input.SSLCertificateRef,
            ProxyEnabled:      input.ProxyEnabled,
            CacheEnabled:      input.CacheEnabled,
            ProxyTarget:       input.ProxyTarget,
            HTTPConfig:        input.HTTPConfig,
            ConflictStrategy:  input.ConflictStrategy,
            Status:            domain.WebsiteStatusPending,
            RuntimeKey:        fmt.Sprintf("web-%s", strings.ReplaceAll(input.PrimaryDomain, ".", "-")),
            CreatedAt:         now,
            UpdatedAt:         now,
        }
        s.state.WebsiteMappings = append(s.state.WebsiteMappings, wm)
    }
    if err := s.saveLocked(); err != nil {
        return domain.WebsiteMapping{}, err
    }
    return wm, nil
}

func (s *Store) UpdateWebsiteMapping(id string, fn func(*domain.WebsiteMapping) error) (domain.WebsiteMapping, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    for i := range s.state.WebsiteMappings {
        if s.state.WebsiteMappings[i].ID == id {
            if err := fn(&s.state.WebsiteMappings[i]); err != nil {
                return domain.WebsiteMapping{}, err
            }
            s.state.WebsiteMappings[i].UpdatedAt = time.Now().UTC()
            return s.state.WebsiteMappings[i], s.saveLocked()
        }
    }
    return domain.WebsiteMapping{}, errors.New("website mapping not found")
}

func (s *Store) GetWebsiteMapping(id string) (domain.WebsiteMapping, bool) {
    s.mu.RLock()
    defer s.mu.RUnlock()
    for _, wm := range s.state.WebsiteMappings {
        if wm.ID == id {
            return wm, true
        }
    }
    return domain.WebsiteMapping{}, false
}

func (s *Store) ListWebsiteMappings(filter domain.WebsiteMappingListFilter) []domain.WebsiteMapping {
    s.mu.RLock()
    defer s.mu.RUnlock()
    out := make([]domain.WebsiteMapping, 0, len(s.state.WebsiteMappings))
    for _, wm := range s.state.WebsiteMappings {
        if !filter.IncludeArchived && wm.Status == domain.WebsiteStatusArchived {
            continue
        }
        if filter.NodeID != "" && wm.NodeID != filter.NodeID {
            continue
        }
        if filter.HTTPSEnabled != nil && wm.HTTPSEnabled != *filter.HTTPSEnabled {
            continue
        }
        if filter.Status != "" && !strings.EqualFold(wm.Status, filter.Status) {
            continue
        }
        if !wm.Match(filter.Q) {
            continue
        }
        out = append(out, wm)
    }
    sort.SliceStable(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
    return out
}

func (s *Store) UpdateSettings(settings domain.Settings) (domain.Settings, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    now := time.Now().UTC()
    merged := mergeSettings(s.state.Settings, settings, now)
    merged.UpdatedAt = now
    s.state.Settings = merged
    if err := s.saveLocked(); err != nil {
        return domain.Settings{}, err
    }
    return maskSettings(merged), nil
}

func (s *Store) Settings() domain.Settings {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return maskSettings(s.state.Settings)
}

func (s *Store) UpdateChmlFrpCredentials(fn func(*domain.ChmlFrpCredentials) error) (domain.Settings, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if err := fn(&s.state.Settings.Integrations.ChmlFrp); err != nil {
        return domain.Settings{}, err
    }
    s.state.Settings.UpdatedAt = time.Now().UTC()
    if err := s.saveLocked(); err != nil {
        return domain.Settings{}, err
    }
    return maskSettings(s.state.Settings), nil
}

func (s *Store) UpdateOnePanelCredentials(fn func(*domain.OnePanelCredentials) error) (domain.Settings, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if err := fn(&s.state.Settings.Integrations.OnePanel); err != nil {
        return domain.Settings{}, err
    }
    s.state.Settings.UpdatedAt = time.Now().UTC()
    if err := s.saveLocked(); err != nil {
        return domain.Settings{}, err
    }
    return maskSettings(s.state.Settings), nil
}

func (s *Store) UpdateCloudflareCredentials(fn func(*domain.CloudflareCredentials) error) (domain.Settings, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if err := fn(&s.state.Settings.Integrations.Cloudflare); err != nil {
        return domain.Settings{}, err
    }
    s.state.Settings.UpdatedAt = time.Now().UTC()
    if err := s.saveLocked(); err != nil {
        return domain.Settings{}, err
    }
    return maskSettings(s.state.Settings), nil
}

func (s *Store) UpdateNodeStatus(id string, status, health string) (domain.Node, error) {
    return s.UpdateNode(id, func(node *domain.Node) error {
        if status != "" {
            node.Status = status
        }
        if health != "" {
            node.HealthStatus = health
        }
        return nil
    })
}

func (s *Store) UpdateTunnelState(id string, actualState, reason, errorCode, errorMessage string) (domain.Tunnel, error) {
    return s.UpdateTunnel(id, func(t *domain.Tunnel) error {
        if actualState != "" {
            t.ActualState = actualState
        }
        t.StateReason = reason
        t.LastErrorCode = errorCode
        t.LastErrorMessage = errorMessage
        now := time.Now().UTC()
        t.LastAppliedAt = &now
        return nil
    })
}

func (s *Store) UpdateWebsiteState(id string, status, reason, errorCode, errorMessage string) (domain.WebsiteMapping, error) {
    return s.UpdateWebsiteMapping(id, func(w *domain.WebsiteMapping) error {
        if status != "" {
            w.Status = status
        }
        w.LastErrorCode = errorCode
        w.LastErrorMessage = errorMessage
        now := time.Now().UTC()
        w.LastSyncedAt = &now
        return nil
    })
}

func (s *Store) SetRuntimeKeyForTunnel(id, runtimeKey string) (domain.Tunnel, error) {
    return s.UpdateTunnel(id, func(t *domain.Tunnel) error {
        t.RuntimeKey = runtimeKey
        return nil
    })
}

func mergeSettings(current, incoming domain.Settings, now time.Time) domain.Settings {
    merged := current
    merged.General = incoming.General
    merged.Sync = incoming.Sync
    merged.Queue = incoming.Queue
    merged.FRPCRuntime = incoming.FRPCRuntime

    merged.Integrations.ChmlFrp.Username = incoming.Integrations.ChmlFrp.Username
    if strings.TrimSpace(incoming.Integrations.ChmlFrp.Password) != "" {
        merged.Integrations.ChmlFrp.Password = incoming.Integrations.ChmlFrp.Password
        merged.Integrations.ChmlFrp.HasPassword = true
    }
    if merged.Integrations.ChmlFrp.Password != "" {
        merged.Integrations.ChmlFrp.HasPassword = true
    }
    merged.Integrations.ChmlFrp.UpdatedAt = now
    merged.Integrations.ChmlFrp.LastErrorMessage = ""

    if strings.TrimSpace(incoming.Integrations.OnePanel.BaseURL) != "" {
        merged.Integrations.OnePanel.BaseURL = incoming.Integrations.OnePanel.BaseURL
    }
    if strings.TrimSpace(incoming.Integrations.OnePanel.Entrance) != "" {
        merged.Integrations.OnePanel.Entrance = incoming.Integrations.OnePanel.Entrance
    }
    if strings.TrimSpace(incoming.Integrations.OnePanel.APIToken) != "" {
        merged.Integrations.OnePanel.APIToken = incoming.Integrations.OnePanel.APIToken
        merged.Integrations.OnePanel.HasAPIToken = true
    }
    if merged.Integrations.OnePanel.APIToken != "" {
        merged.Integrations.OnePanel.HasAPIToken = true
    }
    merged.Integrations.OnePanel.UpdatedAt = now
    merged.Integrations.OnePanel.LastErrorMessage = ""

    if strings.TrimSpace(incoming.Integrations.Cloudflare.APIToken) != "" {
        merged.Integrations.Cloudflare.APIToken = incoming.Integrations.Cloudflare.APIToken
        merged.Integrations.Cloudflare.HasAPIToken = true
    }
    if merged.Integrations.Cloudflare.APIToken != "" {
        merged.Integrations.Cloudflare.HasAPIToken = true
    }
    if strings.TrimSpace(incoming.Integrations.Cloudflare.ZoneID) != "" {
        merged.Integrations.Cloudflare.ZoneID = incoming.Integrations.Cloudflare.ZoneID
    }
    merged.Integrations.Cloudflare.UpdatedAt = now
    merged.Integrations.Cloudflare.LastErrorMessage = ""

    return merged
}

func maskSettings(settings domain.Settings) domain.Settings {
    masked := settings
    masked.Integrations.ChmlFrp.Password = ""
    masked.Integrations.ChmlFrp.HasPassword = settings.Integrations.ChmlFrp.Password != "" || settings.Integrations.ChmlFrp.HasPassword
    masked.Integrations.OnePanel.APIToken = ""
    masked.Integrations.OnePanel.HasAPIToken = settings.Integrations.OnePanel.APIToken != "" || settings.Integrations.OnePanel.HasAPIToken
    masked.Integrations.Cloudflare.APIToken = ""
    masked.Integrations.Cloudflare.HasAPIToken = settings.Integrations.Cloudflare.APIToken != "" || settings.Integrations.Cloudflare.HasAPIToken
    return masked
}

func pickString(value, fallback string) string {
    if strings.TrimSpace(value) == "" {
        return fallback
    }
    return value
}
