package admincli

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "modernc.org/sqlite"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

func setupAdminRepo(t *testing.T) (*gorm.DB, *repository.Repository) {
	t.Helper()
	db, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: ":memory:"}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.Account{}, &domain.AuthToken{}, &domain.AuditLog{}))
	return db, repository.New(db)
}

func createAccount(t *testing.T, db *gorm.DB, username, role, password string) domain.Account {
	t.Helper()
	hash, err := security.HashPassword(password)
	require.NoError(t, err)
	account := domain.Account{ID: domain.NewID("acc"), LoginName: username, PasswordHash: hash, Role: role, MustChangePwd: true, FailedAttempts: 5}
	lockedUntil := time.Now().Add(10 * time.Minute)
	account.LockedUntil = &lockedUntil
	require.NoError(t, db.Create(&account).Error)
	return account
}

func TestResetPasswordUpdatesUniqueAdministratorAndRevokesTokens(t *testing.T) {
	db, repo := setupAdminRepo(t)
	account := createAccount(t, db, "old-admin", "super_admin", "old-password")
	active := domain.AuthToken{ID: domain.NewID("tok"), AccountID: account.ID, TokenType: "session", TokenHash: "tok_active", ExpiresAt: time.Now().Add(time.Hour)}
	alreadyRevoked := domain.AuthToken{ID: domain.NewID("tok"), AccountID: account.ID, TokenType: "api", TokenHash: "tok_revoked", ExpiresAt: time.Now().Add(time.Hour)}
	revokedAt := time.Now().Add(-time.Minute)
	alreadyRevoked.RevokedAt = &revokedAt
	require.NoError(t, db.Create(&active).Error)
	require.NoError(t, db.Create(&alreadyRevoked).Error)

	result, err := ResetPassword(repo, ResetRequest{NewUsername: "root-admin", NewPassword: "new-password"})
	require.NoError(t, err)
	assert.Equal(t, "root-admin", result.LoginName)
	assert.EqualValues(t, 1, result.RevokedTokens)

	var saved domain.Account
	require.NoError(t, db.First(&saved, "id = ?", account.ID).Error)
	assert.Equal(t, "root-admin", saved.LoginName)
	assert.Zero(t, saved.FailedAttempts)
	assert.Nil(t, saved.LockedUntil)
	assert.False(t, saved.MustChangePwd)
	assert.True(t, security.VerifyPassword("new-password", saved.PasswordHash))
	assert.False(t, security.VerifyPassword("old-password", saved.PasswordHash))

	var savedActive domain.AuthToken
	require.NoError(t, db.First(&savedActive, "id = ?", active.ID).Error)
	require.NotNil(t, savedActive.RevokedAt)
	var savedRevoked domain.AuthToken
	require.NoError(t, db.First(&savedRevoked, "id = ?", alreadyRevoked.ID).Error)
	require.NotNil(t, savedRevoked.RevokedAt)
	assert.WithinDuration(t, revokedAt, *savedRevoked.RevokedAt, time.Second)

	var audit domain.AuditLog
	require.NoError(t, db.Where("action = ?", "password.reset.cli").First(&audit).Error)
	assert.Equal(t, account.ID, audit.AccountID)
	assert.Equal(t, "root-admin", audit.AccountName)
	assert.NotContains(t, audit.DetailJSON, "new-password")
	assert.NotContains(t, audit.DetailJSON, saved.PasswordHash)
}

func TestResetPasswordRejectsMissingOrMultipleAdministrators(t *testing.T) {
	t.Run("no administrator", func(t *testing.T) {
		_, repo := setupAdminRepo(t)
		_, err := ResetPassword(repo, ResetRequest{NewUsername: "admin", NewPassword: "new-password"})
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrNoAdministrator))
	})

	t.Run("multiple administrators", func(t *testing.T) {
		db, repo := setupAdminRepo(t)
		first := createAccount(t, db, "first-admin", "admin", "old-password")
		createAccount(t, db, "second-admin", "super_admin", "old-password")
		token := domain.AuthToken{ID: domain.NewID("tok"), AccountID: first.ID, TokenType: "session", TokenHash: "tok_multiple", ExpiresAt: time.Now().Add(time.Hour)}
		require.NoError(t, db.Create(&token).Error)

		_, err := ResetPassword(repo, ResetRequest{NewUsername: "new-admin", NewPassword: "new-password"})
		require.Error(t, err)
		assert.True(t, errors.Is(err, ErrMultipleAdmins))

		var saved domain.Account
		require.NoError(t, db.First(&saved, "id = ?", first.ID).Error)
		assert.Equal(t, "first-admin", saved.LoginName)
		assert.True(t, security.VerifyPassword("old-password", saved.PasswordHash))
		var savedToken domain.AuthToken
		require.NoError(t, db.First(&savedToken, "id = ?", token.ID).Error)
		assert.Nil(t, savedToken.RevokedAt)
	})
}

func TestResetPasswordValidation(t *testing.T) {
	db, repo := setupAdminRepo(t)
	createAccount(t, db, "admin", "admin", "old-password")
	for _, test := range []struct {
		name    string
		request ResetRequest
		target  error
	}{
		{name: "short password", request: ResetRequest{NewUsername: "admin", NewPassword: "short"}, target: ErrPasswordTooShort},
		{name: "empty username", request: ResetRequest{NewUsername: " ", NewPassword: "new-password"}, target: ErrUsernameInvalid},
		{name: "long username", request: ResetRequest{NewUsername: strings.Repeat("a", 65), NewPassword: "new-password"}, target: ErrUsernameInvalid},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, err := ResetPassword(repo, test.request)
			require.Error(t, err)
			assert.True(t, errors.Is(err, test.target))
		})
	}
}
