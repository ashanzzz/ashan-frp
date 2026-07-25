package worker

import (
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/integration/chmlfrp"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

type NodeSyncWorker struct {
	repo         *repository.Repository
	key          []byte
	syncInterval time.Duration
	stopCh       chan struct{}
}

func NewNodeSyncWorker(repo *repository.Repository, key []byte) *NodeSyncWorker {
	return &NodeSyncWorker{
		repo:         repo,
		key:          key,
		syncInterval: 24 * time.Hour,
		stopCh:       make(chan struct{}),
	}
}

func (w *NodeSyncWorker) Start() {
	slog.Info("node_sync_worker.started")
	go w.loop()
}

func (w *NodeSyncWorker) Stop() {
	close(w.stopCh)
}

func (w *NodeSyncWorker) loop() {
	// Sync immediately on startup
	w.SyncChmlFrpNodes()
	w.MonitorInUseNodes()

	syncTicker := time.NewTicker(w.syncInterval)
	defer syncTicker.Stop()

	inUseTicker := time.NewTicker(5 * time.Minute)
	defer inUseTicker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-syncTicker.C:
			w.SyncChmlFrpNodes()
		case <-inUseTicker.C:
			w.MonitorInUseNodes()
		}
	}
}

func (w *NodeSyncWorker) MonitorInUseNodes() {
	inUseMap, err := w.repo.GetInUseNodeMap()
	if err != nil || len(inUseMap) == 0 {
		return
	}

	for nodeID, count := range inUseMap {
		node, nErr := w.repo.FindNodeByID(nodeID)
		if nErr != nil || node == nil {
			continue
		}

		targetIP := node.RealIP
		if targetIP == "" {
			targetIP = node.EndpointURL
		}
		if targetIP == "" {
			continue
		}

		res := chmlfrp.MeasureNodeSpeed(targetIP, 80)
		if res.Reachable {
			_ = w.repo.UpdateNodeSpeedTest(node.ID, node.IsPreferredNode, res.LatencyMS, res.SpeedMbps, res.RealIP)
			node.HealthStatus = domain.HealthOnline
			_ = w.repo.UpdateNode(node)
		} else {
			slog.Warn("node_sync_worker.in_use_node_offline_warning",
				"node_id", node.ID,
				"bound_tunnels", count,
				"error", res.Error,
			)
			node.HealthStatus = domain.HealthOffline
			_ = w.repo.UpdateNode(node)

			_ = w.repo.CreateAuditLog(&domain.AuditLog{
				ID:           domain.NewID("aud"),
				AccountID:    "system",
				AccountName:  "NodeSyncWorker",
				Action:       "node.in_use_warning_offline",
				ResourceType: "node",
				ResourceID:   node.ID,
				Outcome:      "failure",
				DetailJSON:   "【使用中节点失效警告】节点 " + node.ID + " 当前关联穿透隧道，但 TCP 测速超时离线，请关注！",
			})
		}
	}
}

func (w *NodeSyncWorker) SyncChmlFrpNodes() {
	token := ""
	cred, cErr := w.repo.FindCredentialByProvider("chmlfrp")
	if cErr == nil && cred != nil && cred.EncryptedSecret != "" {
		if dec, decErr := security.Decrypt(cred.EncryptedSecret, w.key); decErr == nil {
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
	nodes, err := client.GetNodes()
	if err != nil || len(nodes) == 0 {
		slog.Warn("node_sync_worker.sync_failed", "err", err)
		return
	}

	now := time.Now()
	for _, raw := range nodes {
		nodeID := strings.TrimSpace(raw.Name)
		if nodeID == "" {
			continue
		}
		webSupported := raw.Wed == "1" || strings.Contains(strings.ToLower(raw.Notes), "web") || raw.HTTPPort > 0
		notes := strings.TrimSpace(raw.Notes)
		fangyu := strings.TrimSpace(raw.Fangyu)

		existing, _ := w.repo.FindNodeByID(nodeID)
		if existing != nil {
			existing.DisplayName = raw.Name
			existing.Notes = notes
			existing.Fangyu = fangyu
			existing.WebSupported = webSupported
			existing.RealIP = raw.IP
			existing.EndpointURL = raw.IP
			existing.Region = raw.Area
			existing.UpdatedAt = now
			_ = w.repo.UpdateNode(existing)
		} else {
			newNode := &domain.Node{
				ID:              nodeID,
				DisplayName:     raw.Name,
				Provider:        "chmlfrp",
				NodeType:        "frp_node",
				EndpointURL:     raw.IP,
				Region:          raw.Area,
				Status:          domain.NodeStatusActive,
				HealthStatus:    domain.HealthUnknown,
				CanonicalName:   raw.Name,
				WebSupported:    webSupported,
				Notes:           notes,
				Fangyu:          fangyu,
				RealIP:          raw.IP,
				IsPreferredNode: false,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			_ = w.repo.CreateNode(newNode)
		}
	}
	slog.Info("node_sync_worker.synced", "count", len(nodes))

	w.EnsureEmergencyBestNodeSelection()
}

// EnsureEmergencyBestNodeSelection runs when no preferred node is online/valid.
func (w *NodeSyncWorker) EnsureEmergencyBestNodeSelection() {
	preferred, err := w.repo.ListPreferredNodes()
	if err == nil && len(preferred) > 0 {
		for _, n := range preferred {
			if n.HealthStatus == domain.HealthOnline {
				return // Preferred pool has valid online nodes, no emergency needed!
			}
		}
	}

	// Preferred pool has 0 online nodes! Scan candidate nodes with web_supported = true.
	candidates, cErr := w.repo.ListCandidateNodes()
	if cErr != nil || len(candidates) == 0 {
		return
	}

	var bestNode *domain.Node
	bestLatency := 99999

	for i := range candidates {
		c := &candidates[i]
		if c.WebSupported {
			target := c.RealIP
			if target == "" {
				target = c.EndpointURL
			}
			if target != "" {
				res := chmlfrp.MeasureNodeSpeed(target, 80)
				if res.Reachable {
					_ = w.repo.UpdateNodeSpeedTest(c.ID, false, res.LatencyMS, res.SpeedMbps, res.RealIP)
					if res.LatencyMS < bestLatency {
						bestLatency = res.LatencyMS
						bestNode = c
					}
				}
			}
		}
	}

	if bestNode != nil {
		slog.Warn("node_sync_worker.emergency_best_node_selected",
			"node_id", bestNode.ID,
			"latency_ms", bestNode.LatencyMS,
			"speed_mbps", bestNode.SpeedMbps,
		)
		bestNode.IsPreferredNode = true
		_ = w.repo.UpdateNode(bestNode)

		_ = w.repo.CreateAuditLog(&domain.AuditLog{
			ID:           domain.NewID("aud"),
			AccountID:    "system",
			AccountName:  "NodeSyncWorker",
			Action:       "node.emergency_auto_select",
			ResourceType: "node",
			ResourceID:   bestNode.ID,
			Outcome:      "success",
			DetailJSON:   "Automatically selected best candidate node " + bestNode.ID + " due to empty preferred pool",
		})
	}
}
