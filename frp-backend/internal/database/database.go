package database

import (
	"os"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "modernc.org/sqlite"

	"ashan-frp/internal/config"
	"ashan-frp/internal/domain"
	"ashan-frp/internal/security"
)

func Open(cfg config.Config) (*gorm.DB, error) {
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		return nil, err
	}

	dsn := cfg.DatabaseDSN
	separator := "?"
	if strings.Contains(dsn, "?") {
		separator = "&"
	}
	dsn += separator + "_journal_mode=WAL&_busy_timeout=5000"

	db, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: dsn}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	// SQLite serializes writes. A single shared connection prevents concurrent
	// session last-used updates from causing transient authentication failures.
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	if err := db.AutoMigrate(
		&domain.Account{},
		&domain.AuthToken{},
		&domain.UpstreamCredential{},
		&domain.Node{},
		&domain.Tunnel{},
		&domain.Job{},
		&domain.Event{},
		&domain.AuditLog{},
		&domain.Snapshot{},
		&domain.SyncState{},
		&domain.Setting{},
		&domain.WebsiteMapping{},
	); err != nil {
		return nil, err
	}

	return db, nil
}

func BootstrapAdmin(db *gorm.DB, cfg config.Config) (*domain.Account, error) {
	var count int64
	if err := db.Model(&domain.Account{}).Count(&count).Error; err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, nil
	}

	hash, err := security.HashPassword(cfg.BootstrapPassword)
	if err != nil {
		return nil, err
	}
	account := domain.Account{
		ID:            domain.NewID("acc"),
		LoginName:     cfg.BootstrapUsername,
		DisplayName:   "Administrator",
		PasswordHash:  hash,
		Role:          "super_admin",
		MustChangePwd: true,
	}
	if err := db.Create(&account).Error; err != nil {
		return nil, err
	}
	return &account, nil
}
