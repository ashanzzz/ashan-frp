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
