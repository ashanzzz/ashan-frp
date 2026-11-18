package worker

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/frpc"
	"ashan-frp/internal/integration/chmlfrp"
	"ashan-frp/internal/integration/cloudflare"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

const SettingKeyFailover = "failover_settings"

// FailoverSupervisor monitors active FRP node health, tracks continuous offline duration,
// migrates tunnels on failure (after 10 minutes offline threshold), and conducts weekly early-morning
// speed tests to optimize toward nodes with faster download throughput and lower latency.
type FailoverSupervisor struct {
	repo         *repository.Repository
	frpcMgr      *frpc.Manager
	key          []byte
	pollInterval time.Duration
	stopCh       chan struct{}
	mu           sync.RWMutex

	// In-memory runtime tracking
	currentTrackedNode     string
	currentTrackedIP       string
	isOnline               bool
	firstOfflineAt         *time.Time
	lastCheckedAt          time.Time
	lastFailoverAt         *time.Time
	lastFailoverLog        string
	lastOptimizeAt         *time.Time
	lastOptimizeLog        string
	candidateBestNode      string
	candidateBestLatency   int
	candidateBestSpeedMbps float64
}

// NewFailoverSupervisor creates a new auto-failover supervisor.
func NewFailoverSupervisor(repo *repository.Repository, frpcMgr *frpc.Manager, key []byte) *FailoverSupervisor {
	return &FailoverSupervisor{
		repo:         repo,
		frpcMgr:      frpcMgr,
		key:          key,
		pollInterval: 30 * time.Second,
		stopCh:       make(chan struct{}),
		isOnline:     true,
	}
}

// NewFailoverMonitor preserves backwards compatibility.
func NewFailoverMonitor(repo *repository.Repository, key []byte) *FailoverSupervisor {
	return NewFailoverSupervisor(repo, nil, key)
}

func (s *FailoverSupervisor) Start() {
	slog.Info("failover_supervisor.started")
	go s.loop()
}

func (s *FailoverSupervisor) Stop() {
	close(s.stopCh)
}

func (s *FailoverSupervisor) loop() {
	// Execute immediately on startup
	s.CheckHeartbeat()

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.CheckHeartbeat()
			s.checkWeeklyOptimizeSchedule()
		}
	}
}

// checkWeeklyOptimizeSchedule executes early Monday morning weekly optimization (03:30 AM).
func (s *FailoverSupervisor) checkWeeklyOptimizeSchedule() {
	settings := s.GetSettings()
	if !settings.WeeklyOptimizeEnabled {
		return
	}
	now := time.Now()
	// Target: Monday 03:30 ~ 03:35 AM
	if now.Weekday() == time.Monday && now.Hour() == 3 && now.Minute() >= 30 && now.Minute() <= 35 {
		s.mu.RLock()
		lastOpt := s.lastOptimizeAt
		s.mu.RUnlock()
		if lastOpt == nil || time.Since(*lastOpt) > 20*time.Hour {
			slog.Info("failover.weekly_optimize_timer_triggered", "time", now)
			_, _ = s.OptimizeWeeklyBestNode("Weekly Monday 03:30 AM scheduled optimization", false)
		}
	}
}

// GetSettings retrieves failover settings from SQLite, defaulting to auto_failover_enabled = true, 10m threshold.
func (s *FailoverSupervisor) GetSettings() domain.FailoverSettings {
	def := domain.FailoverSettings{
		AutoFailoverEnabled:    true,
		OfflineThresholdMin:    10,
		HeartbeatIntervalSec:   30,
		WeeklyOptimizeEnabled:  true,
		WeeklyOptimizeSchedule: "Monday 03:30",
		MinImprovementPct:      15,
	}
	setting, err := s.repo.GetSetting(SettingKeyFailover)
	if err != nil || setting == nil || setting.ValueJSON == "" {
		return def
	}
	var res domain.FailoverSettings
	if err := json.Unmarshal([]byte(setting.ValueJSON), &res); err != nil {
		return def
	}
	if res.OfflineThresholdMin <= 0 {
		res.OfflineThresholdMin = 10
	}
	if res.HeartbeatIntervalSec <= 0 {
		res.HeartbeatIntervalSec = 30
	}
	if res.MinImprovementPct <= 0 {
		res.MinImprovementPct = 15
	}
	if res.WeeklyOptimizeSchedule == "" {
		res.WeeklyOptimizeSchedule = "Monday 03:30"
	}
	return res
}

// SaveSettings persists failover settings to SQLite.
func (s *FailoverSupervisor) SaveSettings(settings domain.FailoverSettings) error {
	bytes, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	return s.repo.SetSetting(SettingKeyFailover, string(bytes))
}

// GetStatus returns the current failover engine status and tracked node health.
func (s *FailoverSupervisor) GetStatus() domain.FailoverStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	settings := s.GetSettings()
	continuousMin := 0
	if s.firstOfflineAt != nil {
		continuousMin = int(time.Since(*s.firstOfflineAt).Minutes())
	}

	return domain.FailoverStatus{
		AutoFailoverEnabled:    settings.AutoFailoverEnabled,
		TrackedNode:            s.currentTrackedNode,
		TrackedNodeIP:          s.currentTrackedIP,
		TrackedNodeOnline:      s.isOnline,
		FirstOfflineAt:         s.firstOfflineAt,
		ContinuousOfflineMin:   continuousMin,
		OfflineThresholdMin:    settings.OfflineThresholdMin,
		WeeklyOptimizeEnabled:  settings.WeeklyOptimizeEnabled,
		LastCheckedAt:          s.lastCheckedAt,
		LastFailoverAt:         s.lastFailoverAt,
		LastFailoverLog:        s.lastFailoverLog,
		LastOptimizeAt:         s.lastOptimizeAt,
		LastOptimizeLog:        s.lastOptimizeLog,
		CandidateBestNode:      s.candidateBestNode,
		CandidateBestLatency:   s.candidateBestLatency,
		CandidateBestSpeedMbps: s.candidateBestSpeedMbps,
	}
}

// CheckHeartbeat runs a single health check probe on the currently used FRP node.
func (s *FailoverSupervisor) CheckHeartbeat() {
	settings := s.GetSettings()
	now := time.Now()

	s.mu.Lock()
	s.lastCheckedAt = now
	s.mu.Unlock()

	// 1. Find active tunnels in repository
	tunnels, err := s.repo.ListTunnels(repository.TunnelFilter{})
	if err != nil || len(tunnels) == 0 {
		return
	}

	// Identify which ChmlFrp node is actively serving running tunnels
	var activeNodeName string
	var activeNodeIP string
	for _, t := range tunnels {
		if (t.ActualState == "running" || t.DesiredState == domain.TunnelDesiredEnabled) && t.ChmlfrpNode != "" {
			activeNodeName = t.ChmlfrpNode
			activeNodeIP = t.NodeRealIP
			break
		}
	}
	if activeNodeName == "" {
		return
	}

	// If node IP is empty, lookup node record or resolve DNS
	if activeNodeIP == "" {
		if node, _ := s.repo.FindNodeByID(activeNodeName); node != nil {
			activeNodeIP = node.RealIP
			if activeNodeIP == "" {
				activeNodeIP = node.EndpointURL
			}
		}
	}
	if activeNodeIP == "" {
		if addrs, err := net.LookupHost(activeNodeName + ".ip.chmlfrp.cn"); err == nil && len(addrs) > 0 {
			activeNodeIP = addrs[0]
		}
	}

	s.mu.Lock()
	s.currentTrackedNode = activeNodeName
	s.currentTrackedIP = activeNodeIP
	s.mu.Unlock()

	if !settings.AutoFailoverEnabled {
		s.mu.Lock()
		s.firstOfflineAt = nil
		s.mu.Unlock()
		return
	}

	// 2. Probe connectivity to the active node
	probeSuccess := false
	if activeNodeIP != "" {
		// Probe TCP 7000 (standard ChmlFrp FRP control port)
		conn, err := net.DialTimeout("tcp", net.JoinHostPort(activeNodeIP, "7000"), 3*time.Second)
		if err == nil {
			_ = conn.Close()
			probeSuccess = true
		} else {
			// Fallback: probe TCP 80
			conn80, err80 := net.DialTimeout("tcp", net.JoinHostPort(activeNodeIP, "80"), 2*time.Second)
			if err80 == nil {
				_ = conn80.Close()
				probeSuccess = true
			}
		}
	}

	// 3. Fallback verification via ChmlFrp upstream API GetNodeInfo
	if !probeSuccess {
		chClient, cErr := s.getChmlFrpClient()
		if cErr == nil && chClient != nil {
			if info, iErr := chClient.GetNodeInfo(activeNodeName); iErr == nil && info != nil && info.Data.State == "online" {
				probeSuccess = true
			}
		}
	}

	if probeSuccess {
		s.mu.Lock()
		if !s.isOnline && s.firstOfflineAt != nil {
			slog.Info("failover.node_recovered_online",
				"node", activeNodeName,
				"ip", activeNodeIP,
				"was_offline_since", s.firstOfflineAt,
			)
		}
		s.isOnline = true
		s.firstOfflineAt = nil
		s.mu.Unlock()
		return
	}

	// Probe Failed: Node is OFFLINE
	s.mu.Lock()
	s.isOnline = false
	if s.firstOfflineAt == nil {
		s.firstOfflineAt = &now
		slog.Warn("failover.node_offline_detected",
			"node", activeNodeName,
			"ip", activeNodeIP,
			"detected_at", now,
		)
	}
	continuousMin := int(now.Sub(*s.firstOfflineAt).Minutes())
	s.mu.Unlock()

	slog.Warn("failover.node_offline_heartbeat",
		"node", activeNodeName,
		"ip", activeNodeIP,
		"continuous_minutes", continuousMin,
		"threshold_minutes", settings.OfflineThresholdMin,
	)

	// Trigger failover if offline duration reaches or exceeds threshold (default 10 mins)
	if continuousMin >= settings.OfflineThresholdMin {
		slog.Error("failover.threshold_reached_initiating_failover",
			"node", activeNodeName,
			"ip", activeNodeIP,
			"continuous_minutes", continuousMin,
			"threshold_minutes", settings.OfflineThresholdMin,
		)
		if err := s.ExecuteFailover(activeNodeName, activeNodeIP, continuousMin); err != nil {
			slog.Error("failover.execution_failed", "error", err)
		}
	}
}

// TriggerFailoverManual manually triggers failover migration for testing or emergency.
func (s *FailoverSupervisor) TriggerFailoverManual(reason string) (*domain.FailoverStatus, error) {
	s.mu.RLock()
	node := s.currentTrackedNode
	ip := s.currentTrackedIP
	s.mu.RUnlock()

	if node == "" {
		node = "北京多线"
		ip = "82.156.215.91"
	}
	err := s.ExecuteFailover(node, ip, 0)
	st := s.GetStatus()
	return &st, err
}

// ExecuteFailover coordinates the end-to-end failover: speed tests candidate nodes,
// selects the lowest latency online node, updates ChmlFrp tunnels, updates Cloudflare DNS,
// and restarts the local FRPC process.
func (s *FailoverSupervisor) ExecuteFailover(failedNode, failedIP string, continuousMin int) error {
	now := time.Now()
	slog.Info("failover.start_execution", "failed_node", failedNode, "failed_ip", failedIP)

	// Step 1: Discover best candidate node
	candidates, err := s.repo.ListNodes(repository.NodeFilter{})
	if err != nil || len(candidates) == 0 {
		logMsg := fmt.Sprintf("Failover failed: Tracked node %s (%s) offline %d min, but 0 candidate nodes in database", failedNode, failedIP, continuousMin)
		s.recordAudit("node.auto_failover_aborted", failedNode, "failed", logMsg)
		return fmt.Errorf("%s", logMsg)
	}

	type CandidateResult struct {
		node    domain.Node
		latency int
		speed   float64
	}
	var viable []CandidateResult
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Test candidates concurrently
	for _, n := range candidates {
		// Filter: must support web, must not be the failed node
		if !n.WebSupported || n.DisplayName == failedNode || n.ID == failedNode || n.RealIP == failedIP {
			continue
		}
		targetIP := n.RealIP
		if targetIP == "" {
			targetIP = n.EndpointURL
		}
		if targetIP == "" {
			continue
		}

		wg.Add(1)
		go func(node domain.Node, ip string) {
			defer wg.Done()
			res := chmlfrp.MeasureNodeSpeed(ip, 80)
			if res.Reachable && res.LatencyMS > 0 {
				mu.Lock()
				viable = append(viable, CandidateResult{node: node, latency: res.LatencyMS, speed: res.SpeedMbps})
				mu.Unlock()
			}
		}(n, targetIP)
	}
	wg.Wait()

	if len(viable) == 0 {
		logMsg := fmt.Sprintf("Failover failed: Tracked node %s (%s) offline %d min, but 0 candidate nodes reachable", failedNode, failedIP, continuousMin)
		s.recordAudit("node.auto_failover_aborted", failedNode, "failed", logMsg)
		return fmt.Errorf("%s", logMsg)
	}

	// Sort viable candidates by lowest latency
	sort.Slice(viable, func(i, j int) bool {
		return viable[i].latency < viable[j].latency
	})
	best := viable[0]
	bestNode := best.node
	bestLatency := best.latency

	s.mu.Lock()
	s.candidateBestNode = bestNode.DisplayName
	s.candidateBestLatency = bestLatency
	s.candidateBestSpeedMbps = best.speed
	s.mu.Unlock()

	slog.Info("failover.best_candidate_selected",
		"node", bestNode.DisplayName,
		"ip", bestNode.RealIP,
		"latency_ms", bestLatency,
	)

	// Step 2 & 3 & 4: Migrate tunnels, DNS, and FRPC
	migratedTunnels, dnsUpdatedCount, mErr := s.MigrateToNode(bestNode, failedNode, failedIP)
	if mErr != nil {
		return mErr
	}

	// Step 5: Save Audit Log, reset firstOfflineAt, update lastFailoverAt
	summaryLog := fmt.Sprintf("触发自动故障转移：节点 [%s](%s) 持续离线 %d 分钟，系统自动测速并选定最佳健康节点 [%s](%s, 延迟 %dms)。已完成 %d 条隧道迁移与 %d 条 Cloudflare DNS 解析同步，本地 FRPC 守护进程已热重载。",
		failedNode, failedIP, continuousMin, bestNode.DisplayName, bestNode.RealIP, bestLatency, len(migratedTunnels), dnsUpdatedCount)

	s.recordAudit("node.auto_failover", bestNode.DisplayName, "success", summaryLog)

	s.mu.Lock()
	s.firstOfflineAt = nil
	s.isOnline = true
	s.currentTrackedNode = bestNode.DisplayName
	s.currentTrackedIP = bestNode.RealIP
	s.lastFailoverAt = &now
	s.lastFailoverLog = summaryLog
	s.mu.Unlock()

	slog.Info("failover.completed_successfully", "summary", summaryLog)
	return nil
}

// OptimizeWeeklyBestNode conducts comprehensive speed tests on candidate nodes (evaluating download speed
// and TCP latency), performs pre-switch logging/notification, hot-migrates all core tunnels to the superior node,
// executes post-switch domain verification, and performs automatic self-healing rollback if any endpoint breaks.
func (s *FailoverSupervisor) OptimizeWeeklyBestNode(reason string, force bool) (*domain.FailoverStatus, error) {
	now := time.Now()
	settings := s.GetSettings()

	s.mu.RLock()
	currentNodeName := s.currentTrackedNode
	currentNodeIP := s.currentTrackedIP
	s.mu.RUnlock()

	if currentNodeName == "" {
		currentNodeName = "北京多线"
		currentNodeIP = "82.156.215.91"
	}

	slog.Info("failover.weekly_optimize_start", "current_node", currentNodeName, "current_ip", currentNodeIP, "reason", reason)

	// Step 1: Measure current node baseline
	currSpeed := chmlfrp.MeasureNodeSpeed(currentNodeIP, 80)
	currLatency := currSpeed.LatencyMS
	if currLatency <= 0 {
		currLatency = 999
	}
	currThroughput := currSpeed.SpeedMbps

	// Step 2: Concurrently probe all candidate nodes supporting Web
	candidates, err := s.repo.ListNodes(repository.NodeFilter{})
	if err != nil || len(candidates) == 0 {
		return nil, fmt.Errorf("weekly optimize: no candidate nodes in database: %w", err)
	}

	type ScoredCandidate struct {
		node    domain.Node
		latency int
		speed   float64
		score   float64
	}
	var viable []ScoredCandidate
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, n := range candidates {
		if !n.WebSupported {
			continue
		}
		targetIP := n.RealIP
		if targetIP == "" {
			targetIP = n.EndpointURL
		}
		if targetIP == "" {
			continue
		}

		wg.Add(1)
		go func(node domain.Node, ip string) {
			defer wg.Done()
			res := chmlfrp.MeasureNodeSpeed(ip, 80)
			if res.Reachable && res.LatencyMS > 0 {
				// Score favors higher download throughput while penalizing high latency
				// Score = (SpeedMbps * 5.0) - LatencyMS
				score := (res.SpeedMbps * 5.0) - float64(res.LatencyMS)
				mu.Lock()
				viable = append(viable, ScoredCandidate{
					node:    node,
					latency: res.LatencyMS,
					speed:   res.SpeedMbps,
					score:   score,
				})
				mu.Unlock()
			}
		}(n, targetIP)
	}
	wg.Wait()

	if len(viable) == 0 {
		return nil, fmt.Errorf("weekly optimize: no reachable candidate nodes during speed test")
	}

	// Sort candidates by highest score
	sort.Slice(viable, func(i, j int) bool {
		return viable[i].score > viable[j].score
	})
	best := viable[0]
	bestNode := best.node

	// Update cached best candidate
	s.mu.Lock()
	s.candidateBestNode = bestNode.DisplayName
	s.candidateBestLatency = best.latency
	s.candidateBestSpeedMbps = best.speed
	s.mu.Unlock()

	// Calculate improvement percentage over current node
	currScore := (currThroughput * 5.0) - float64(currLatency)
	improvementPct := 0.0
	if currScore > 0 {
		improvementPct = ((best.score - currScore) / currScore) * 100.0
	} else if best.score > currScore {
		improvementPct = 100.0
	}

	minRequiredPct := float64(settings.MinImprovementPct)
	if minRequiredPct <= 0 {
		minRequiredPct = 15.0
	}

	// If current node is already the best or candidate does not meet the improvement threshold
	if !force && (bestNode.DisplayName == currentNodeName || bestNode.RealIP == currentNodeIP || improvementPct < minRequiredPct) {
		msg := fmt.Sprintf("周度优选巡检完成：当前节点 [%s](%s) 表现优异（延迟 %dms，下载速度 %.1f Mbps）。候选最优节点 [%s](%s, 延迟 %dms, 速度 %.1f Mbps) 性能提升幅度 (%.1f%%) 未达 %d%% 切换门槛，保持当前节点稳定运行。",
			currentNodeName, currentNodeIP, currLatency, currThroughput, bestNode.DisplayName, bestNode.RealIP, best.latency, best.speed, improvementPct, int(minRequiredPct))

		s.recordAudit("node.weekly_optimize_skipped", currentNodeName, "success", msg)

		s.mu.Lock()
		s.lastOptimizeAt = &now
		s.lastOptimizeLog = msg
		s.mu.Unlock()

		st := s.GetStatus()
		return &st, nil
	}

	// Candidate is significantly faster: initiate proactive migration
	preLog := fmt.Sprintf("启动周度节点优选切换：原节点 [%s](%s, 延迟 %dms, 速度 %.1f Mbps) -> 优选节点 [%s](%s, 延迟 %dms, 速度 %.1f Mbps, 性能提升 %.1f%%)。",
		currentNodeName, currentNodeIP, currLatency, currThroughput, bestNode.DisplayName, bestNode.RealIP, best.latency, best.speed, improvementPct)
	slog.Info("failover.weekly_optimize_migrating", "detail", preLog)
	s.recordAudit("node.weekly_optimize_started", bestNode.DisplayName, "running", preLog)

	// Save previous node for self-healing rollback
	prevNode := domain.Node{
		DisplayName: currentNodeName,
		RealIP:      currentNodeIP,
	}

	migratedTunnels, dnsCount, mErr := s.MigrateToNode(bestNode, currentNodeName, currentNodeIP)
	if mErr != nil {
		s.recordAudit("node.weekly_optimize_aborted", bestNode.DisplayName, "failed", mErr.Error())
		return nil, mErr
	}

	// Give FRPC & DNS 2 seconds to establish connection
	time.Sleep(2 * time.Second)

	// Step 4: Verify public domains. If broken, trigger automatic self-healing rollback!
	probeResults, vErr := s.VerifyPublicDomains()
	if vErr != nil {
		slog.Error("failover.weekly_optimize_failed_broken_initiating_rollback", "error", vErr)
		// Automatic Self-Healing Rollback
		_, _, rbErr := s.MigrateToNode(prevNode, bestNode.DisplayName, bestNode.RealIP)
		rollbackMsg := fmt.Sprintf("⚠️ 周度节点切换验证异常（探测失败: %s）。系统已自动启动自愈修复并回滚至原节点 [%s](%s)！回滚结果: %v",
			vErr.Error(), prevNode.DisplayName, prevNode.RealIP, rbErr)
		s.recordAudit("node.weekly_optimize_failback", prevNode.DisplayName, "failed", rollbackMsg)

		s.mu.Lock()
		s.lastOptimizeAt = &now
		s.lastOptimizeLog = rollbackMsg
		s.currentTrackedNode = prevNode.DisplayName
		s.currentTrackedIP = prevNode.RealIP
		s.mu.Unlock()

		st := s.GetStatus()
		return &st, fmt.Errorf("%s", rollbackMsg)
	}

	// Verification succeeded: all 5 domains are 200 OK
	successLog := fmt.Sprintf("周度节点优选切换圆满成功！已平滑切换至更高性能节点 [%s](%s)，连接延迟 %dms，下载速度 %.1f Mbps（综合性能提升 %.1f%%）。5 大核心公网域名均以 200 OK 正常响应，已同步迁移 %d 条隧道与 %d 条 Cloudflare 解析。",
		bestNode.DisplayName, bestNode.RealIP, best.latency, best.speed, improvementPct, len(migratedTunnels), dnsCount)

	s.recordAudit("node.weekly_optimize_success", bestNode.DisplayName, "success", successLog)

	s.mu.Lock()
	s.lastOptimizeAt = &now
	s.lastOptimizeLog = successLog
	s.currentTrackedNode = bestNode.DisplayName
	s.currentTrackedIP = bestNode.RealIP
	s.mu.Unlock()

	slog.Info("failover.weekly_optimize_completed", "probes", probeResults, "summary", successLog)
	st := s.GetStatus()
	return &st, nil
}

// MigrateToNode updates all affected tunnels in ChmlFrp, synchronizes Cloudflare DNS A records,
// and reloads the local FRPC daemon.
func (s *FailoverSupervisor) MigrateToNode(targetNode domain.Node, fromNodeName, fromNodeIP string) ([]domain.Tunnel, int, error) {
	chClient, err := s.getChmlFrpClient()
	if err != nil {
		return nil, 0, fmt.Errorf("failover: failed to get chmlfrp client: %w", err)
	}

	tunnels, err := s.repo.ListTunnels(repository.TunnelFilter{})
	if err != nil {
		return nil, 0, fmt.Errorf("failover: failed to list tunnels: %w", err)
	}

	var migratedTunnels []domain.Tunnel
	for _, t := range tunnels {
		if fromNodeName == "" || t.ChmlfrpNode == fromNodeName || t.NodeRealIP == fromNodeIP || fromNodeName == "*" {
			if t.ChmlfrpTunnelID != "" {
				updErr := chClient.UpdateTunnel(t.ChmlfrpTunnelID, t.ChmlfrpTunnelName, targetNode.DisplayName, t.LocalIP, t.Protocol, t.LocalPort, t.FullDomain, t.RemotePort)
				if updErr != nil {
					slog.Error("failover.chmlfrp_update_tunnel_error", "tunnel", t.Name, "error", updErr)
				}
			}
			t.ChmlfrpNode = targetNode.DisplayName
			t.NodeRealIP = targetNode.RealIP
			_ = s.repo.UpdateTunnel(&t)
			migratedTunnels = append(migratedTunnels, t)
		}
	}

	// Update Cloudflare DNS A records
	cfClient := s.getCloudflareClient()
	dnsUpdatedCount := 0
	if cfClient != nil && targetNode.RealIP != "" {
		cfRecords, cfErr := cfClient.ListRecords()
		if cfErr == nil {
			for _, t := range migratedTunnels {
				if t.FullDomain == "" {
					continue
				}
				for _, rec := range cfRecords {
					if strings.EqualFold(rec.Name, t.FullDomain) && rec.Type == "A" {
						proxied := t.CFProxied
						comment := rec.Comment
						if !strings.HasPrefix(strings.ToLower(comment), "ashan-frp managed:") {
							comment = "ashan-frp managed: " + t.ID
						}
						_, uErr := cfClient.UpdateDNSRecord(rec.ID, domain.DNSRecordInput{
							Type:    "A",
							Name:    t.FullDomain,
							Content: targetNode.RealIP,
							Proxied: &proxied,
							TTL:     1,
						}, comment)
						if uErr == nil {
							dnsUpdatedCount++
							slog.Info("failover.dns_record_updated", "domain", t.FullDomain, "new_ip", targetNode.RealIP)
						} else {
							slog.Error("failover.dns_record_update_failed", "domain", t.FullDomain, "error", uErr)
						}
						break
					}
				}
			}
		}
	}

	// Reload / restart FRPC
	if s.frpcMgr != nil && targetNode.RealIP != "" {
		user, authToken := s.getChmlFrpUserAndAuthToken()
		s.frpcMgr.Renderer().SetServer(targetNode.RealIP, 7000, user, authToken)
		allTunnels, _ := s.repo.ListTunnels(repository.TunnelFilter{})
		if rErr := s.frpcMgr.Restart(context.Background(), allTunnels); rErr != nil {
			slog.Error("failover.frpc_restart_failed", "error", rErr)
		} else {
			slog.Info("failover.frpc_restarted_successfully", "new_server", targetNode.RealIP)
		}
	}

	return migratedTunnels, dnsUpdatedCount, nil
}

// VerifyPublicDomains performs active end-to-end HTTPS probing on all 5 production endpoints.
func (s *FailoverSupervisor) VerifyPublicDomains() (map[string]bool, error) {
	domains := []string{
		"https://drive.335356119.xyz",
		"https://erpnext.335356119.xyz",
		"https://unraid.335356119.xyz",
		"https://aiapi.335356119.xyz",
		"https://pw.335356119.xyz",
	}

	client := &http.Client{
		Timeout: 6 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	results := make(map[string]bool)
	var unreachable []string

	for _, u := range domains {
		req, _ := http.NewRequest("GET", u, nil)
		req.Header.Set("User-Agent", "ashan-frp-watchdog")
		resp, err := client.Do(req)
		if err != nil || (resp.StatusCode < 200 || resp.StatusCode >= 400) {
			errMsg := "unreachable"
			if err != nil {
				errMsg = err.Error()
			} else {
				errMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
			}
			slog.Warn("failover.domain_probe_failed", "url", u, "error", errMsg)
			results[u] = false
			unreachable = append(unreachable, fmt.Sprintf("%s (%s)", u, errMsg))
		} else {
			results[u] = true
			resp.Body.Close()
		}
	}

	if len(unreachable) > 0 {
		return results, fmt.Errorf("public domains unreachable: %s", strings.Join(unreachable, ", "))
	}
	return results, nil
}

func (s *FailoverSupervisor) recordAudit(action, resourceID, outcome, details string) {
	_ = s.repo.CreateAuditLog(&domain.AuditLog{
		ID:           domain.NewID("aud"),
		AccountID:    "system",
		AccountName:  "FailoverSupervisor",
		Action:       action,
		ResourceType: "node",
		ResourceID:   resourceID,
		Outcome:      outcome,
		DetailJSON:   details,
	})
}

func (s *FailoverSupervisor) getChmlFrpClient() (*chmlfrp.Client, error) {
	token := ""
	cred, cErr := s.repo.FindCredentialByProvider("chmlfrp")
	if cErr == nil && cred != nil && cred.EncryptedSecret != "" {
		if dec, decErr := security.Decrypt(cred.EncryptedSecret, s.key); decErr == nil {
			oauthMgr := chmlfrp.NewOAuth2Manager()
			activeToken, updatedPayload, refreshed, _ := oauthMgr.EnsureValidToken(dec)
			if refreshed && len(updatedPayload) > 0 {
				if enc, encErr := security.Encrypt(updatedPayload, s.key); encErr == nil {
					cred.EncryptedSecret = enc
					now := time.Now()
					cred.UpdatedAt = now
					cred.LastVerifiedAt = &now
					_ = s.repo.UpsertCredential(cred)
				}
			}
			if activeToken != "" {
				token = activeToken
			} else {
				token = string(dec)
			}
		}
	}
	if token == "" {
		return nil, fmt.Errorf("no chmlfrp credential configured")
	}
	return chmlfrp.NewClient("token", token), nil
}

func (s *FailoverSupervisor) getCloudflareClient() *cloudflare.Client {
	cf, err := s.repo.FindCredentialByProvider("cloudflare")
	if err != nil || cf == nil || cf.EncryptedSecret == "" {
		return nil
	}
	rawSecret, err := security.Decrypt(cf.EncryptedSecret, s.key)
	if err != nil {
		return nil
	}
	zone := cf.ZoneID
	if zone == "" {
		zone = cf.Identifier
	}
	return cloudflare.NewClientWithCredentials(cloudflare.Credentials{
		AuthMethod: cf.AuthMethod,
		Secret:     string(rawSecret),
		Email:      cf.AccountEmail,
	}, zone)
}

func (s *FailoverSupervisor) getChmlFrpUserAndAuthToken() (string, string) {
	user := ""
	authToken := "ChmlFrpToken"
	if cred, err := s.repo.FindCredentialByProvider("chmlfrp"); err == nil && cred != nil && cred.EncryptedSecret != "" {
		if rawSecret, err := security.Decrypt(cred.EncryptedSecret, s.key); err == nil {
			var tokenData struct {
				AccessToken string `json:"access_token"`
				UserToken   string `json:"user_token"`
			}
			if json.Unmarshal(rawSecret, &tokenData) == nil {
				if tokenData.UserToken != "" {
					user = tokenData.UserToken
				}
				if user == "" && tokenData.AccessToken != "" {
					client := chmlfrp.NewClient("token", tokenData.AccessToken)
					if uToken, err := client.GetUserInfo(); err == nil && uToken != "" {
						user = uToken
					}
				}
			}
		}
	}
	return user, authToken
}
