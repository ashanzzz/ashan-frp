package repository

import (
	"database/sql"
	"time"

	"gorm.io/gorm"

	"ashan-frp/internal/domain"
)

type Repository struct{ db *gorm.DB }

func New(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) DBStats() (sql.DBStats, error) {
	sqlDB, err := r.db.DB()
	if err != nil {
		return sql.DBStats{}, err
	}
	return sqlDB.Stats(), nil
}

func (r *Repository) FindAccountByLogin(login string) (*domain.Account, error) {
	var a domain.Account
	err := r.db.Where("login_name = ?", login).First(&a).Error
	return &a, err
}
func (r *Repository) FindAccountByID(id string) (*domain.Account, error) {
	var a domain.Account
	err := r.db.First(&a, "id = ?", id).Error
	return &a, err
}
func (r *Repository) UpdateAccount(a *domain.Account) error     { return r.db.Save(a).Error }
func (r *Repository) CreateAuthToken(t *domain.AuthToken) error { return r.db.Create(t).Error }
func (r *Repository) FindAuthTokenByHash(hash string) (*domain.AuthToken, error) {
	var t domain.AuthToken
	err := r.db.Where("token_hash = ?", hash).First(&t).Error
	return &t, err
}
func (r *Repository) RevokeAuthToken(id string) error {
	now := time.Now()
	return r.db.Model(&domain.AuthToken{}).Where("id = ?", id).Update("revoked_at", now).Error
}
func (r *Repository) ListAuthTokensByAccount(accID string) ([]domain.AuthToken, error) {
	var tokens []domain.AuthToken
	err := r.db.Where("account_id = ?", accID).Order("created_at DESC").Find(&tokens).Error
	return tokens, err
}
func (r *Repository) TouchAuthToken(id string) error {
	now := time.Now()
	return r.db.Model(&domain.AuthToken{}).Where("id = ?", id).Update("last_used_at", now).Error
}
func (r *Repository) CreateTunnel(t *domain.Tunnel) error { return r.db.Create(t).Error }
func (r *Repository) FindTunnelByID(id string) (*domain.Tunnel, error) {
	var t domain.Tunnel
	err := r.db.First(&t, "id = ?", id).Error
	return &t, err
}
func (r *Repository) FindTunnelByDomain(fullDomain string) (*domain.Tunnel, error) {
	var t domain.Tunnel
	err := r.db.Where("full_domain = ?", fullDomain).First(&t).Error
	return &t, err
}
func (r *Repository) UpdateTunnel(t *domain.Tunnel) error { return r.db.Save(t).Error }
func (r *Repository) DeleteTunnel(id string) error {
	return r.db.Delete(&domain.Tunnel{}, "id = ?", id).Error
}
func (r *Repository) CountTunnels() (int64, error) {
	var c int64
	err := r.db.Model(&domain.Tunnel{}).Where("desired_state != ?", "archived").Count(&c).Error
	return c, err
}

type TunnelFilter struct {
	Status      string
	Protocol    string
	ChmlfrpNode string
}

func (r *Repository) ListTunnels(f TunnelFilter) ([]domain.Tunnel, error) {
	q := r.db.Model(&domain.Tunnel{})
	if f.Status != "" {
		q = q.Where("desired_state = ?", f.Status)
	}
	if f.Protocol != "" {
		q = q.Where("protocol = ?", f.Protocol)
	}
	if f.ChmlfrpNode != "" {
		q = q.Where("chmlfrp_node = ?", f.ChmlfrpNode)
	}
	var tunnels []domain.Tunnel
	err := q.Order("updated_at DESC").Find(&tunnels).Error
	return tunnels, err
}

func (r *Repository) ListFailoverTunnels() ([]domain.Tunnel, error) {
	var tunnels []domain.Tunnel
	err := r.db.Where("is_failover_pool = ?", true).Order("failover_priority ASC, created_at ASC").Find(&tunnels).Error
	return tunnels, err
}

func (r *Repository) UpdateFailoverPriority(id string, isPool bool, priority int) error {
	return r.db.Model(&domain.Tunnel{}).Where("id = ?", id).Updates(map[string]any{
		"is_failover_pool":  isPool,
		"failover_priority": priority,
		"updated_at":        time.Now(),
	}).Error
}

func (r *Repository) CreateJob(j *domain.Job) error { return r.db.Create(j).Error }
func (r *Repository) FindJobByID(id string) (*domain.Job, error) {
	var j domain.Job
	err := r.db.First(&j, "id = ?", id).Error
	return &j, err
}
func (r *Repository) UpdateJob(j *domain.Job) error { return r.db.Save(j).Error }
func (r *Repository) CountJobsByStatus(status string) (int64, error) {
	var c int64
	err := r.db.Model(&domain.Job{}).Where("status = ?", status).Count(&c).Error
	return c, err
}

type JobFilter struct {
	Status     string
	TargetType string
	TargetID   string
}

func (r *Repository) ListJobs(f JobFilter) ([]domain.Job, error) {
	q := r.db.Model(&domain.Job{})
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.TargetType != "" {
		q = q.Where("target_type = ?", f.TargetType)
	}
	if f.TargetID != "" {
		q = q.Where("target_id = ?", f.TargetID)
	}
	var jobs []domain.Job
	err := q.Order("created_at DESC").Limit(100).Find(&jobs).Error
	return jobs, err
}

func (r *Repository) CreateAuditLog(l *domain.AuditLog) error { return r.db.Create(l).Error }

type AuditFilter struct {
	Action      string
	Outcome     string
	Provider    string
	AccountName string
	RequestID   string
	From        *time.Time
	To          *time.Time
	Limit       int
	Offset      int
}

func (r *Repository) ListAuditLogs(limit int) ([]domain.AuditLog, error) {
	logs, _, err := r.ListAuditLogsFiltered(AuditFilter{Limit: limit})
	return logs, err
}

func (r *Repository) ListAuditLogsFiltered(filter AuditFilter) ([]domain.AuditLog, int64, error) {
	q := r.db.Model(&domain.AuditLog{})
	if filter.Action != "" {
		q = q.Where("action LIKE ?", "%"+filter.Action+"%")
	}
	if filter.Outcome != "" {
		q = q.Where("outcome = ?", filter.Outcome)
	}
	if filter.Provider != "" {
		q = q.Where("action LIKE ? OR detail_json LIKE ?", filter.Provider+"%", "%\"provider\":\""+filter.Provider+"\"%")
	}
	if filter.AccountName != "" {
		q = q.Where("account_name LIKE ?", "%"+filter.AccountName+"%")
	}
	if filter.RequestID != "" {
		q = q.Where("request_id = ?", filter.RequestID)
	}
	if filter.From != nil {
		q = q.Where("created_at >= ?", *filter.From)
	}
	if filter.To != nil {
		q = q.Where("created_at <= ?", *filter.To)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var logs []domain.AuditLog
	err := q.Order("created_at DESC").Limit(limit).Offset(filter.Offset).Find(&logs).Error
	return logs, total, err
}

func (r *Repository) DeleteAuditLogsBefore(cutoff time.Time) (int64, error) {
	result := r.db.Where("created_at < ?", cutoff).Delete(&domain.AuditLog{})
	return result.RowsAffected, result.Error
}

func (r *Repository) GetSetting(key string) (*domain.Setting, error) {
	var s domain.Setting
	err := r.db.First(&s, "key = ?", key).Error
	return &s, err
}
func (r *Repository) SetSetting(key, val string) error {
	s := domain.Setting{Key: key, ValueJSON: val, UpdatedAt: time.Now()}
	return r.db.Save(&s).Error
}
func (r *Repository) GetAllSettings() ([]domain.Setting, error) {
	var s []domain.Setting
	err := r.db.Find(&s).Error
	return s, err
}
func (r *Repository) FindCredentialByProvider(provider string) (*domain.UpstreamCredential, error) {
	var c domain.UpstreamCredential
	err := r.db.Where("provider = ?", provider).First(&c).Error
	return &c, err
}
func (r *Repository) UpsertCredential(c *domain.UpstreamCredential) error { return r.db.Save(c).Error }
func (r *Repository) ListCredentials() ([]domain.UpstreamCredential, error) {
	var creds []domain.UpstreamCredential
	err := r.db.Find(&creds).Error
	return creds, err
}
func (r *Repository) CreateSnapshot(s *domain.Snapshot) error { return r.db.Create(s).Error }
func (r *Repository) ListSnapshotsByProvider(provider string, limit int) ([]domain.Snapshot, error) {
	var snaps []domain.Snapshot
	err := r.db.Where("provider = ?", provider).Order("created_at DESC").Limit(limit).Find(&snaps).Error
	return snaps, err
}
func (r *Repository) UpsertSyncState(s *domain.SyncState) error { return r.db.Save(s).Error }
func (r *Repository) ListSyncStates() ([]domain.SyncState, error) {
	var states []domain.SyncState
	err := r.db.Find(&states).Error
	return states, err
}
func (r *Repository) FindSyncStateByLocal(lt, lid string) (*domain.SyncState, error) {
	var s domain.SyncState
	err := r.db.Where("local_resource_type = ? AND local_resource_id = ?", lt, lid).First(&s).Error
	return &s, err
}

func (r *Repository) CountNodes() (int64, error) {
	var c int64
	err := r.db.Model(&domain.Node{}).Where("status != ?", domain.NodeStatusArchived).Count(&c).Error
	return c, err
}

func (r *Repository) ListPreferredNodes() ([]domain.Node, error) {
	var nodes []domain.Node
	err := r.db.Where("is_preferred_node = ? AND status != ?", true, domain.NodeStatusArchived).Order("latency_ms ASC, updated_at DESC").Find(&nodes).Error
	return nodes, err
}

func (r *Repository) ListCandidateNodes() ([]domain.Node, error) {
	var nodes []domain.Node
	err := r.db.Where("is_preferred_node = ? AND status != ?", false, domain.NodeStatusArchived).Order("latency_ms ASC, updated_at DESC").Find(&nodes).Error
	return nodes, err
}

func (r *Repository) UpdateNodeSpeedTest(id string, isPreferred bool, latencyMS int, speedMbps float64, realIP string) error {
	now := time.Now()
	updates := map[string]any{
		"is_preferred_node":    isPreferred,
		"latency_ms":           latencyMS,
		"speed_mbps":           speedMbps,
		"last_speed_tested_at": now,
		"updated_at":           now,
	}
	if realIP != "" {
		updates["real_ip"] = realIP
	}
	return r.db.Model(&domain.Node{}).Where("id = ?", id).Updates(updates).Error
}
func (r *Repository) CountWebsiteMappings() (int64, error) {
	var c int64
	err := r.db.Model(&domain.WebsiteMapping{}).Where("status != ?", domain.WebsiteStatusArchived).Count(&c).Error
	return c, err
}
func (r *Repository) CountSyncStates() (int64, error) {
	var c int64
	err := r.db.Model(&domain.SyncState{}).Count(&c).Error
	return c, err
}

// ---- WebsiteMapping ----

type WebsiteMappingFilter struct {
	Q               string
	NodeID          string
	Status          string
	HTTPSEnabled    *bool
	IncludeArchived bool
}

func (r *Repository) CreateWebsiteMapping(w *domain.WebsiteMapping) error {
	w.SerializeJSON()
	return r.db.Create(w).Error
}

func (r *Repository) FindWebsiteMappingByID(id string) (*domain.WebsiteMapping, error) {
	var w domain.WebsiteMapping
	err := r.db.First(&w, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	w.DeserializeJSON()
	return &w, nil
}

func (r *Repository) UpdateWebsiteMapping(w *domain.WebsiteMapping) error {
	w.SerializeJSON()
	return r.db.Save(w).Error
}

func (r *Repository) DeleteWebsiteMapping(id string) error {
	return r.db.Delete(&domain.WebsiteMapping{}, "id = ?", id).Error
}

func (r *Repository) ListWebsiteMappings(f WebsiteMappingFilter) ([]domain.WebsiteMapping, error) {
	q := r.db.Model(&domain.WebsiteMapping{})
	if f.NodeID != "" {
		q = q.Where("node_id = ?", f.NodeID)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.HTTPSEnabled != nil {
		q = q.Where("https_enabled = ?", *f.HTTPSEnabled)
	}
	var ws []domain.WebsiteMapping
	err := q.Order("updated_at DESC").Find(&ws).Error
	if err != nil {
		return nil, err
	}
	for i := range ws {
		ws[i].DeserializeJSON()
	}
	return ws, nil
}

func (r *Repository) CountWebsiteMappingsLegacy() (int64, error) {
	var c int64
	err := r.db.Model(&domain.WebsiteMapping{}).Count(&c).Error
	return c, err
}

type NodeFilter struct {
	Q               string
	Provider        string
	Status          string
	HealthStatus    string
	IncludeArchived bool
}

func (r *Repository) CreateNode(n *domain.Node) error {
	n.SerializeJSON()
	return r.db.Create(n).Error
}

func (r *Repository) FindNodeByID(id string) (*domain.Node, error) {
	var n domain.Node
	err := r.db.First(&n, "id = ?", id).Error
	if err != nil {
		return nil, err
	}
	n.DeserializeJSON()
	return &n, nil
}

func (r *Repository) UpdateNode(n *domain.Node) error {
	n.SerializeJSON()
	return r.db.Save(n).Error
}

func (r *Repository) ListNodes(f NodeFilter) ([]domain.Node, error) {
	q := r.db.Model(&domain.Node{})
	if f.Q != "" {
		like := "%" + f.Q + "%"
		q = q.Where("display_name LIKE ? OR canonical_name LIKE ? OR endpoint_url LIKE ?", like, like, like)
	}
	if f.Provider != "" {
		q = q.Where("provider = ?", f.Provider)
	}
	if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	if f.HealthStatus != "" {
		q = q.Where("health_status = ?", f.HealthStatus)
	}
	var nodes []domain.Node
	err := q.Order("updated_at DESC").Find(&nodes).Error
	if err != nil {
		return nil, err
	}
	for i := range nodes {
		nodes[i].DeserializeJSON()
	}
	return nodes, nil
}

func (r *Repository) CreateEvent(e *domain.Event) error { return r.db.Create(e).Error }
func (r *Repository) ListEventsByChannel(channel string) ([]domain.Event, error) {
	var events []domain.Event
	err := r.db.Where("channel = ?", channel).Order("created_at ASC").Find(&events).Error
	return events, err
}

func (r *Repository) ListAccessibleJobEvents(channel, role string) ([]domain.Event, error) {
	events, err := r.ListEventsByChannel(channel)
	if err != nil {
		return nil, err
	}
	if role == "super_admin" || role == "admin" {
		return events, nil
	}
	if role == "viewer" {
		filtered := make([]domain.Event, 0, len(events))
		for _, evt := range events {
			if evt.Level == "error" || evt.Level == "warn" || evt.Kind == "job.failed" || evt.Kind == "job.retry_scheduled" {
				filtered = append(filtered, evt)
			}
		}
		return filtered, nil
	}
	return events, nil
}
