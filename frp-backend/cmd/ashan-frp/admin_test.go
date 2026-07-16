package main

import (
	"bytes"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "modernc.org/sqlite"

	"ashan-frp/internal/config"
	"ashan-frp/internal/database"
	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

func setupCommandRepo(t *testing.T) (*gorm.DB, *repository.Repository, domain.Account) {
	t.Helper()
	db, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: ":memory:"}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&domain.Account{}, &domain.AuthToken{}, &domain.AuditLog{}))
	hash, err := security.HashPassword("old-password")
	require.NoError(t, err)
	account := domain.Account{ID: domain.NewID("acc"), LoginName: "old-admin", PasswordHash: hash, Role: "super_admin"}
	require.NoError(t, db.Create(&account).Error)
	return db, repository.New(db), account
}

func TestExecuteAdminResetPasswordFromStdin(t *testing.T) {
	db, repo, account := setupCommandRepo(t)
	token := domain.AuthToken{ID: domain.NewID("tok"), AccountID: account.ID, TokenType: "session", TokenHash: "tok_command", ExpiresAt: time.Now().Add(time.Hour)}
	require.NoError(t, db.Create(&token).Error)
	secret := "new-password"
	var output bytes.Buffer
	err := executeAdminCommand(repo, []string{"reset-password", "--new-username", "new-admin", "--password-stdin"}, strings.NewReader(secret+"\n"), &output, &bytes.Buffer{}, nil)
	require.NoError(t, err)
	assert.Contains(t, output.String(), "密码已重置")
	assert.NotContains(t, output.String(), secret)

	var saved domain.Account
	require.NoError(t, db.First(&saved, "id = ?", account.ID).Error)
	assert.Equal(t, "new-admin", saved.LoginName)
	assert.True(t, security.VerifyPassword(secret, saved.PasswordHash))
}

func TestExecuteAdminInteractiveResetPromptsForNewUsernameAndPassword(t *testing.T) {
	db, repo, account := setupCommandRepo(t)
	answers := []string{"new-password", "new-password"}
	prompt := func(string) (string, error) {
		answer := answers[0]
		answers = answers[1:]
		return answer, nil
	}
	var output bytes.Buffer
	err := executeAdminCommand(repo, []string{"reset-password"}, strings.NewReader("new-admin\n"), &output, &bytes.Buffer{}, prompt)
	require.NoError(t, err)
	assert.Contains(t, output.String(), "请输入新的管理员用户名：")

	var saved domain.Account
	require.NoError(t, db.First(&saved, "id = ?", account.ID).Error)
	assert.Equal(t, "new-admin", saved.LoginName)
	assert.True(t, security.VerifyPassword("new-password", saved.PasswordHash))
}

func TestExecuteAdminInteractiveConfirmationAndAutomationValidation(t *testing.T) {
	_, repo, _ := setupCommandRepo(t)
	answers := []string{"new-password", "different-password"}
	prompt := func(string) (string, error) {
		answer := answers[0]
		answers = answers[1:]
		return answer, nil
	}
	err := executeAdminCommand(repo, []string{"reset-password"}, strings.NewReader("new-admin\n"), &bytes.Buffer{}, &bytes.Buffer{}, prompt)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "不一致")

	err = executeAdminCommand(repo, []string{"reset-password", "--password-stdin"}, strings.NewReader("new-password\n"), &bytes.Buffer{}, &bytes.Buffer{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "--new-username")
}

func TestExecuteAdminRejectsOldAndUnsafeFlags(t *testing.T) {
	_, repo, _ := setupCommandRepo(t)
	secret := "do-not-print-this"
	var errorOutput bytes.Buffer
	err := executeAdminCommand(repo, []string{"reset-password", "--username", "old-admin"}, strings.NewReader(""), &bytes.Buffer{}, &errorOutput, nil)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "old-admin")

	errorOutput.Reset()
	err = executeAdminCommand(repo, []string{"reset-password", "--password", secret}, strings.NewReader(""), &bytes.Buffer{}, &errorOutput, nil)
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secret)
	assert.NotContains(t, errorOutput.String(), secret)
}

func TestRunAdminResetDoesNotBootstrapAnAccount(t *testing.T) {
	dataDir := t.TempDir()
	cfg := config.Config{DataDir: dataDir, DatabaseDSN: "file:" + filepath.Join(dataDir, "state.db"), BootstrapUsername: "admin", BootstrapPassword: "must-not-be-used"}
	err := runAdminCommand(cfg, []string{"reset-password", "--new-username", "new-admin", "--password-stdin"}, strings.NewReader("new-password\n"), &bytes.Buffer{}, &bytes.Buffer{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "未找到管理员")

	db, err := database.Open(cfg)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	var count int64
	require.NoError(t, db.Model(&domain.Account{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestReadPasswordLine(t *testing.T) {
	value, err := readPasswordLine(strings.NewReader("secret with spaces\r\nignored"))
	require.NoError(t, err)
	assert.Equal(t, "secret with spaces", value)
	_, err = readPasswordLine(strings.NewReader(""))
	assert.True(t, errors.Is(err, io.EOF))
}
