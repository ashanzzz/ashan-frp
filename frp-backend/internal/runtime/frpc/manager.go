package frpc

import (
    "sort"
    "sync"
    "time"

    "ashan-frp/internal/domain"
)

type Manager struct {
    mu           sync.RWMutex
    version      string
    engineStatus string
    active       map[string]time.Time
    lastAction   string
    updatedAt    time.Time
}

func NewManager(version string) *Manager {
    return &Manager{
        version:      version,
        engineStatus: "running",
        active:       map[string]time.Time{},
        updatedAt:    time.Now().UTC(),
    }
}

func (m *Manager) Start(tunnelID string) domain.RuntimeSummary {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.active[tunnelID] = time.Now().UTC()
    m.engineStatus = "running"
    m.lastAction = "start:" + tunnelID
    m.updatedAt = time.Now().UTC()
    return m.summaryLocked()
}

func (m *Manager) Stop(tunnelID string) domain.RuntimeSummary {
    m.mu.Lock()
    defer m.mu.Unlock()
    delete(m.active, tunnelID)
    if len(m.active) == 0 {
        m.engineStatus = "stopped"
    }
    m.lastAction = "stop:" + tunnelID
    m.updatedAt = time.Now().UTC()
    return m.summaryLocked()
}

func (m *Manager) Restart(tunnelID string) domain.RuntimeSummary {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.active[tunnelID] = time.Now().UTC()
    m.engineStatus = "running"
    m.lastAction = "restart:" + tunnelID
    m.updatedAt = time.Now().UTC()
    return m.summaryLocked()
}

func (m *Manager) StartProcess() domain.RuntimeSummary {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.engineStatus = "running"
    m.lastAction = "start"
    m.updatedAt = time.Now().UTC()
    return m.summaryLocked()
}

func (m *Manager) StopProcess() domain.RuntimeSummary {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.engineStatus = "stopped"
    m.lastAction = "stop"
    m.updatedAt = time.Now().UTC()
    return m.summaryLocked()
}

func (m *Manager) RestartProcess() domain.RuntimeSummary {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.engineStatus = "running"
    m.lastAction = "restart"
    m.updatedAt = time.Now().UTC()
    return m.summaryLocked()
}

func (m *Manager) Reload() domain.RuntimeSummary {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.lastAction = "reload"
    m.updatedAt = time.Now().UTC()
    return m.summaryLocked()
}

func (m *Manager) SwitchNode(nodeID string) domain.RuntimeSummary {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.lastAction = "switch-node:" + nodeID
    m.updatedAt = time.Now().UTC()
    return m.summaryLocked()
}

func (m *Manager) Summary() domain.RuntimeSummary {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.summaryLocked()
}

func (m *Manager) summaryLocked() domain.RuntimeSummary {
    ids := make([]string, 0, len(m.active))
    for id := range m.active {
        ids = append(ids, id)
    }
    sort.Strings(ids)
    return domain.RuntimeSummary{
        FRPCVersion:       m.version,
        EngineStatus:      m.engineStatus,
        ActiveTunnelsCount: len(ids),
        ActiveTunnelIDs:   ids,
        LastAction:        m.lastAction,
        UpdatedAt:         m.updatedAt,
    }
}
