package worker

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "modernc.org/sqlite"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
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
