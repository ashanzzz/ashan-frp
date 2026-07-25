package worker

import (
	"encoding/json"
	"log/slog"
	"time"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/integration/chmlfrp"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

type FailoverMonitor struct {
	repo         *repository.Repository
	key          []byte
	pollInterval time.Duration
	stopCh       chan struct{}
}

func NewFailoverMonitor(repo *repository.Repository, key []byte) *FailoverMonitor {
	return &FailoverMonitor{
		repo:         repo,
		key:          key,
		pollInterval: 30 * time.Second,
		stopCh:       make(chan struct{}),
	}
}

func (m *FailoverMonitor) Start() {
	slog.Info("failover_monitor.started")
	go m.loop()
}

func (m *FailoverMonitor) Stop() {
	close(m.stopCh)
}

func (m *FailoverMonitor) loop() {
	ticker := time.NewTicker(m.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.CheckFailoverPool()
		}
	}
}

func (m *FailoverMonitor) CheckFailoverPool() {
	tunnels, err := m.repo.ListFailoverTunnels()
	if err != nil || len(tunnels) == 0 {
		return
	}

	token := ""
	cred, cErr := m.repo.FindCredentialByProvider("chmlfrp")
	if cErr == nil && cred != nil && cred.EncryptedSecret != "" {
		if dec, decErr := security.Decrypt(cred.EncryptedSecret, m.key); decErr == nil {
			var creds domain.ChmlFrpCredentials
			if json.Unmarshal(dec, &creds) == nil && creds.Password != "" {
				token = creds.Password
			} else {
				token = string(dec)
			}
		}
	}
	if token == "" {
		return
	}

	client := chmlfrp.NewClient("token", token)
	now := time.Now()

	var firstOnline *domain.Tunnel
	var primaryTunnel *domain.Tunnel

	for i := range tunnels {
		t := &tunnels[i]
		if i == 0 {
			primaryTunnel = t
		}
		if t.ChmlfrpNode != "" {
			info, err := client.GetNodeInfo(t.ChmlfrpNode)
			if err == nil && info != nil && info.Data.State == "online" {
				t.NodeRealIP = info.Data.RealIP
				t.LastHealthCheckedAt = &now
				_ = m.repo.UpdateTunnel(t)
				if firstOnline == nil {
					firstOnline = t
				}
			} else {
				t.LastHealthCheckedAt = &now
				_ = m.repo.UpdateTunnel(t)
			}
		}
	}

	// If primary tunnel (#1) is offline/degraded, but a backup tunnel in the pool is online, failover automatically!
	if primaryTunnel != nil && firstOnline != nil && primaryTunnel.ID != firstOnline.ID {
		slog.Warn("failover_monitor.failover_triggered",
			"primary_id", primaryTunnel.ID,
			"primary_node", primaryTunnel.ChmlfrpNode,
			"failover_id", firstOnline.ID,
			"failover_node", firstOnline.ChmlfrpNode,
		)
		primaryTunnel.DesiredState = "disabled"
		firstOnline.DesiredState = "enabled"
		_ = m.repo.UpdateTunnel(primaryTunnel)
		_ = m.repo.UpdateTunnel(firstOnline)

		_ = m.repo.CreateAuditLog(&domain.AuditLog{
			ID:           domain.NewID("aud"),
			AccountID:    "system",
			AccountName:  "AutoFailoverMonitor",
			Action:       "tunnel.auto_failover",
			ResourceType: "tunnel",
			ResourceID:   firstOnline.ID,
			Outcome:      "success",
			DetailJSON:   "Failover triggered from " + primaryTunnel.ChmlfrpNode + " to " + firstOnline.ChmlfrpNode,
		})
	}
}
