package database

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ashan-frp/internal/config"
	"ashan-frp/internal/domain"
	"ashan-frp/internal/security"
)

func TestOpenAndBootstrapAdminDoesNotOverwriteExistingAccount(t *testing.T) {
	dataDir := t.TempDir()
	cfg := config.Config{
		DataDir:           dataDir,
		DatabaseDSN:       "file:" + filepath.Join(dataDir, "state.db"),
		BootstrapUsername: "admin",
		BootstrapPassword: "initial-password",
	}
	db, err := Open(cfg)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	created, err := BootstrapAdmin(db, cfg)
	require.NoError(t, err)
	require.NotNil(t, created)
	assert.True(t, security.VerifyPassword("initial-password", created.PasswordHash))

	cfg.BootstrapPassword = "replacement-password"
	createdAgain, err := BootstrapAdmin(db, cfg)
	require.NoError(t, err)
	assert.Nil(t, createdAgain)

	var saved domain.Account
	require.NoError(t, db.Where("login_name = ?", "admin").First(&saved).Error)
	assert.True(t, security.VerifyPassword("initial-password", saved.PasswordHash))
	assert.False(t, security.VerifyPassword("replacement-password", saved.PasswordHash))
	assert.True(t, db.Migrator().HasTable(&domain.Event{}))
}
