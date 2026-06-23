package repository

import (
	"time"
	"gorm.io/gorm"
	"ashan-frp/internal/domain"
)

type Repository struct { db *gorm.DB }
func New(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) FindAccountByLogin(login string) (*domain.Account, error) { var a domain.Account; err := r.db.Where("login_name = ?", login).First(&a).Error; return &a, err }
func (r *Repository) FindAccountByID(id string) (*domain.Account, error) { var a domain.Account; err := r.db.First(&a, "id = ?", id).Error; return &a, err }
func (r *Repository) UpdateAccount(a *domain.Account) error { return r.db.Save(a).Error }
func (r *Repository) CreateAuthToken(t *domain.AuthToken) error { return r.db.Create(t).Error }
func (r *Repository) FindAuthTokenByHash(hash string) (*domain.AuthToken, error) { var t domain.AuthToken; err := r.db.Where("token_hash = ?", hash).First(&t).Error; return &t, err }
func (r *Repository) RevokeAuthToken(id string) error { now := time.Now(); return r.db.Model(&domain.AuthToken{}).Where("id = ?", id).Update("revoked_at", now).Error }
func (r *Repository) ListAuthTokensByAccount(accID string) ([]domain.AuthToken, error) { var tokens []domain.AuthToken; err := r.db.Where("account_id = ?", accID).Order("created_at DESC").Find(&tokens).Error; return tokens, err }
func (r *Repository) TouchAuthToken(id string) error { now := time.Now(); return r.db.Model(&domain.AuthToken{}).Where("id = ?", id).Update("last_used_at", now).Error }
func (r *Repository) CreateTunnel(t *domain.Tunnel) error { return r.db.Create(t).Error }
func (r *Repository) FindTunnelByID(id string) (*domain.Tunnel, error) { var t domain.Tunnel; err := r.db.First(&t, "id = ?", id).Error; return &t, err }
func (r *Repository) FindTunnelByDomain(domain string) (*domain.Tunnel, error) { var t domain.Tunnel; err := r.db.Where("full_domain = ?", domain).First(&t).Error; return &t, err }
func (r *Repository) UpdateTunnel(t *domain.Tunnel) error { return r.db.Save(t).Error }
func (r *Repository) DeleteTunnel(id string) error { return r.db.Delete(&domain.Tunnel{}, "id = ?", id).Error }
func (r *Repository) CountTunnels() (int64, error) { var c int64; err := r.db.Model(&domain.Tunnel{}).Where("desired_state != ?", "archived").Count(&c).Error; return c, err }

type TunnelFilter struct { Status string; Protocol string; ChmlfrpNode string }

func (r *Repository) ListTunnels(f TunnelFilter) ([]domain.Tunnel, error) {
	q := r.db.Model(&domain.Tunnel{})
	if f.Status != "" { q = q.Where("desired_state = ?", f.Status) }
	if f.Protocol != "" { q = q.Where("protocol = ?", f.Protocol) }
	if f.ChmlfrpNode != "" { q = q.Where("chmlfrp_node = ?", f.ChmlfrpNode) }
	var tunnels []domain.Tunnel; err := q.Order("updated_at DESC").Find(&tunnels).Error; return tunnels, err
}

func (r *Repository) CreateJob(j *domain.Job) error { return r.db.Create(j).Error }
func (r *Repository) FindJobByID(id string) (*domain.Job, error) { var j domain.Job; err := r.db.First(&j, "id = ?", id).Error; return &j, err }
func (r *Repository) UpdateJob(j *domain.Job) error { return r.db.Save(j).Error }
func (r *Repository) CountJobsByStatus(status string) (int64, error) { var c int64; err := r.db.Model(&domain.Job{}).Where("status = ?", status).Count(&c).Error; return c, err }

type JobFilter struct { Status string; TargetType string; TargetID string }

func (r *Repository) ListJobs(f JobFilter) ([]domain.Job, error) {
	q := r.db.Model(&domain.Job{})
	if f.Status != "" { q = q.Where("status = ?", f.Status) }
	if f.TargetType != "" { q = q.Where("target_type = ?", f.TargetType) }
	if f.TargetID != "" { q = q.Where("target_id = ?", f.TargetID) }
	var jobs []domain.Job; err := q.Order("created_at DESC").Limit(100).Find(&jobs).Error; return jobs, err
}

func (r *Repository) CreateAuditLog(l *domain.AuditLog) error { return r.db.Create(l).Error }
func (r *Repository) ListAuditLogs(limit int) ([]domain.AuditLog, error) { var logs []domain.AuditLog; err := r.db.Order("created_at DESC").Limit(limit).Find(&logs).Error; return logs, err }
func (r *Repository) GetSetting(key string) (*domain.Setting, error) { var s domain.Setting; err := r.db.First(&s, "key = ?", key).Error; return &s, err }
func (r *Repository) SetSetting(key, val string) error { s := domain.Setting{Key: key, ValueJSON: val, UpdatedAt: time.Now()}; return r.db.Save(&s).Error }
func (r *Repository) GetAllSettings() ([]domain.Setting, error) { var s []domain.Setting; err := r.db.Find(&s).Error; return s, err }
func (r *Repository) FindCredentialByProvider(provider string) (*domain.UpstreamCredential, error) { var c domain.UpstreamCredential; err := r.db.Where("provider = ?", provider).First(&c).Error; return &c, err }
func (r *Repository) UpsertCredential(c *domain.UpstreamCredential) error { return r.db.Save(c).Error }
func (r *Repository) ListCredentials() ([]domain.UpstreamCredential, error) { var creds []domain.UpstreamCredential; err := r.db.Find(&creds).Error; return creds, err }
func (r *Repository) CreateSnapshot(s *domain.Snapshot) error { return r.db.Create(s).Error }
func (r *Repository) ListSnapshotsByProvider(provider string, limit int) ([]domain.Snapshot, error) { var snaps []domain.Snapshot; err := r.db.Where("provider = ?", provider).Order("created_at DESC").Limit(limit).Find(&snaps).Error; return snaps, err }
func (r *Repository) UpsertSyncState(s *domain.SyncState) error { return r.db.Save(s).Error }
func (r *Repository) ListSyncStates() ([]domain.SyncState, error) { var states []domain.SyncState; err := r.db.Find(&states).Error; return states, err }
func (r *Repository) FindSyncStateByLocal(lt, lid string) (*domain.SyncState, error) { var s domain.SyncState; err := r.db.Where("local_resource_type = ? AND local_resource_id = ?", lt, lid).First(&s).Error; return &s, err }
