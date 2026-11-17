package worker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "modernc.org/sqlite"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

func setupRunnerTestDB(t *testing.T) (*gorm.DB, *repository.Repository) {
	t.Helper()
	db, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: ":memory:"}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.Account{}, &domain.AuthToken{}, &domain.UpstreamCredential{}, &domain.Tunnel{}, &domain.Job{}, &domain.AuditLog{}, &domain.Snapshot{}, &domain.SyncState{}, &domain.Setting{}, &domain.Node{}, &domain.WebsiteMapping{}))
	return db, repository.New(db)
}

func TestRunner_execute_sets_retry_wait_and_next_retry_when_job_kind_is_unknown(t *testing.T) {
	db, repo := setupRunnerTestDB(t)
	r := NewRunner(db, repo, []byte("secret"))
	job := &domain.Job{
		ID:           domain.NewID("job"),
		Kind:         "unknown_kind",
		Status:       domain.JobStatusQueued,
		Title:        "Unknown job",
		Retryable:    true,
		AttemptCount: 1,
		MaxAttempts:  5,
		CreatedBy:    domain.NewID("acc"),
	}
	require.NoError(t, db.Create(job).Error)

	// When
	r.execute(job)

	// Then
	var saved domain.Job
	require.NoError(t, db.First(&saved, "id = ?", job.ID).Error)
	require.Equal(t, domain.JobStatusRetryWait, saved.Status)
	require.NotNil(t, saved.NextRetryAt)
	require.Equal(t, 2, saved.AttemptCount)
	require.Equal(t, "unknown kind: unknown_kind", saved.ErrorMessage)
}

func TestRunner_execute_handles_node_refresh_job(t *testing.T) {
	db, repo := setupRunnerTestDB(t)
	r := NewRunner(db, repo, []byte("secret"))
	job := &domain.Job{
		ID:          domain.NewID("job"),
		Kind:        "node.refresh",
		TargetType:  "nodes",
		TargetID:    "nodes",
		Channel:     "subject:nodes",
		Status:      domain.JobStatusQueued,
		Title:       "Refresh nodes",
		Retryable:   false,
		CreatedBy:   domain.NewID("acc"),
		PayloadJSON: `{}`,
	}
	require.NoError(t, db.Create(job).Error)

	r.execute(job)

	var saved domain.Job
	require.NoError(t, db.First(&saved, "id = ?", job.ID).Error)
	require.Equal(t, domain.JobStatusFailed, saved.Status)
	require.Contains(t, saved.ErrorMessage, "chmlfrp not configured")
	require.NotNil(t, saved.CompletedAt)
}

func TestRunner_execute_handles_website_mapping_sync_job(t *testing.T) {
	db, repo := setupRunnerTestDB(t)
	r := NewRunner(db, repo, []byte("secret"))
	wm := &domain.WebsiteMapping{ID: "m1", SourceKind: "chmlfrp", NodeID: "node-1", PrimaryDomain: "sync.example", Domains: []string{"sync.example"}, Status: domain.WebsiteStatusPending}
	require.NoError(t, repo.CreateWebsiteMapping(wm))
	job := &domain.Job{
		ID:          domain.NewID("job"),
		Kind:        "website_mapping.sync",
		TargetType:  "website_mappings",
		TargetID:    wm.ID,
		Channel:     "subject:website-mappings",
		Status:      domain.JobStatusQueued,
		Title:       "Sync website mapping",
		Retryable:   false,
		CreatedBy:   domain.NewID("acc"),
		PayloadJSON: `{}`,
	}
	require.NoError(t, db.Create(job).Error)

	r.execute(job)

	var saved domain.Job
	require.NoError(t, db.First(&saved, "id = ?", job.ID).Error)
	require.Equal(t, domain.JobStatusSucceeded, saved.Status)
	require.NotNil(t, saved.CompletedAt)
}

type fakeRunnerCloudflareClient struct {
	records   map[string]*domain.CFDNSRecord
	deletedID string
}

func (f *fakeRunnerCloudflareClient) VerifyToken() error { return nil }
func (f *fakeRunnerCloudflareClient) GetRecord(id string) (*domain.CFDNSRecord, error) {
	if rec, ok := f.records[id]; ok {
		return rec, nil
	}
	return nil, fmt.Errorf("record not found")
}
func (f *fakeRunnerCloudflareClient) DeleteRecord(id string) error {
	f.deletedID = id
	return nil
}
func (f *fakeRunnerCloudflareClient) CreateDNSRecord(input domain.DNSRecordInput, comment string) (*domain.CFDNSRecord, error) {
	return &domain.CFDNSRecord{ID: "created", Name: input.Name, Type: input.Type, Content: input.Content, Comment: comment}, nil
}
func (f *fakeRunnerCloudflareClient) CreateRecord(name, recordType, content string, proxied bool, tunnelID string) (*domain.CFDNSRecord, error) {
	return &domain.CFDNSRecord{ID: "created", Name: name, Type: recordType, Content: content, Comment: "ashan-frp managed: tunnel " + tunnelID}, nil
}
func (f *fakeRunnerCloudflareClient) FindRecordsByTag() ([]domain.CFDNSRecord, error) {
	var list []domain.CFDNSRecord
	for _, r := range f.records {
		list = append(list, *r)
	}
	return list, nil
}

func TestRunner_deprovisionTunnel_skipsUnmanagedDNSRecord(t *testing.T) {
	db, repo := setupRunnerTestDB(t)
	key := []byte("0123456789abcdef0123456789abcdef")
	r := NewRunner(db, repo, key)

	encSecret, err := security.Encrypt([]byte("dummy-cf-token"), key)
	require.NoError(t, err)
	cfCred := &domain.UpstreamCredential{
		ID:              "cre_cf",
		Provider:        "cloudflare",
		Identifier:      "example.com",
		EncryptedSecret: encSecret,
	}
	require.NoError(t, repo.UpsertCredential(cfCred))

	fakeClient := &fakeRunnerCloudflareClient{
		records: map[string]*domain.CFDNSRecord{
			"rec_unmanaged": {ID: "rec_unmanaged", Name: "mail.example.com", Type: "A", Content: "1.1.1.1", Comment: "1panel-managed-site"},
			"rec_native":    {ID: "rec_native", Name: "root.example.com", Type: "A", Content: "1.1.1.1", Comment: ""},
			"rec_managed":   {ID: "rec_managed", Name: "app.example.com", Type: "CNAME", Content: "edge.cn", Comment: "ashan-frp managed: tunnel tun_3"},
		},
	}
	origFactory := newCloudflareClient
	defer func() { newCloudflareClient = origFactory }()
	newCloudflareClient = func(string, *domain.UpstreamCredential) runnerCloudflareClient {
		return fakeClient
	}

	// Case 1: 1panel record (unmanaged) -> skip DNS delete
	tun1 := &domain.Tunnel{ID: "tun_1", Name: "tun1", CFRecordID: "rec_unmanaged", ActualState: "running"}
	require.NoError(t, repo.CreateTunnel(tun1))
	job1 := &domain.Job{ID: "job_1", Kind: "deprovision_tunnel", TargetID: "tun_1", PayloadJSON: `{"tunnel_id":"tun_1"}`}
	res1, err1 := r.deprovisionTunnel(job1)
	require.NoError(t, err1)
	require.Equal(t, "deprovisioned", res1)
	require.Empty(t, fakeClient.deletedID, "Must NOT delete 1panel / unmanaged DNS record")

	// Case 2: Native record without comment -> skip DNS delete
	tun2 := &domain.Tunnel{ID: "tun_2", Name: "tun2", CFRecordID: "rec_native", ActualState: "running"}
	require.NoError(t, repo.CreateTunnel(tun2))
	job2 := &domain.Job{ID: "job_2", Kind: "deprovision_tunnel", TargetID: "tun_2", PayloadJSON: `{"tunnel_id":"tun_2"}`}
	res2, err2 := r.deprovisionTunnel(job2)
	require.NoError(t, err2)
	require.Equal(t, "deprovisioned", res2)
	require.Empty(t, fakeClient.deletedID, "Must NOT delete native DNS record")

	// Case 3: ashan-frp managed record -> delete DNS record
	tun3 := &domain.Tunnel{ID: "tun_3", Name: "tun3", CFRecordID: "rec_managed", ActualState: "running"}
	require.NoError(t, repo.CreateTunnel(tun3))
	job3 := &domain.Job{ID: "job_3", Kind: "deprovision_tunnel", TargetID: "tun_3", PayloadJSON: `{"tunnel_id":"tun_3"}`}
	res3, err3 := r.deprovisionTunnel(job3)
	require.NoError(t, err3)
	require.Equal(t, "deprovisioned", res3)
	require.Equal(t, "rec_managed", fakeClient.deletedID, "Must delete ashan-frp managed DNS record")
}
