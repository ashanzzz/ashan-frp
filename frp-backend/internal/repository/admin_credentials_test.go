package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ashan-frp/internal/domain"
)

func TestResetAdminCredentialsRollsBackOnUsernameConflict(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	original := domain.Account{ID: domain.NewID("acc"), LoginName: "original-admin", PasswordHash: "old-hash", Role: "admin", FailedAttempts: 5, MustChangePwd: true}
	occupied := domain.Account{ID: domain.NewID("acc"), LoginName: "occupied-admin", PasswordHash: "other-hash", Role: "admin"}
	require.NoError(t, db.Create(&original).Error)
	require.NoError(t, db.Create(&occupied).Error)
	token := domain.AuthToken{ID: domain.NewID("tok"), AccountID: original.ID, TokenType: "session", TokenHash: "tok_rollback", ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, db.Create(&token).Error)

	_, err := repo.ResetAdminCredentials(AccountCredentialReset{
		AccountID:    original.ID,
		LoginName:    occupied.LoginName,
		PasswordHash: "new-hash",
	})
	require.Error(t, err)

	var saved domain.Account
	require.NoError(t, db.First(&saved, "id = ?", original.ID).Error)
	assert.Equal(t, original.LoginName, saved.LoginName)
	assert.Equal(t, "old-hash", saved.PasswordHash)
	assert.Equal(t, 5, saved.FailedAttempts)
	assert.True(t, saved.MustChangePwd)

	var savedToken domain.AuthToken
	require.NoError(t, db.First(&savedToken, "id = ?", token.ID).Error)
	assert.Nil(t, savedToken.RevokedAt)
	var auditCount int64
	require.NoError(t, db.Model(&domain.AuditLog{}).Where("action = ?", "password.reset.cli").Count(&auditCount).Error)
	assert.Zero(t, auditCount)
}
