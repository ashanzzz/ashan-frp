package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"ashan-frp/internal/config"
	"ashan-frp/internal/domain"
	"ashan-frp/internal/http"
	"ashan-frp/internal/repository"
)

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil { log.Fatalf("failed to create data dir: %v", err) }

	db, err := gorm.Open(sqlite.Open(cfg.DatabaseDSN+"?_journal_mode=WAL&_busy_timeout=5000"), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil { log.Fatalf("failed to open database: %v", err) }

	if err := db.AutoMigrate(&domain.Account{}, &domain.AuthToken{}, &domain.UpstreamCredential{}, &domain.Tunnel{}, &domain.Job{}, &domain.AuditLog{}, &domain.Snapshot{}, &domain.SyncState{}, &domain.Setting{}); err != nil { log.Fatalf("auto-migrate failed: %v", err) }
	bootstrapAdmin(db, cfg)

	repo := repository.New(db)
	srv := http.New(cfg, db, repo)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	log.Printf("[ashan-frp] v%s starting on %s", cfg.Version, cfg.HTTPAddr)
	if err := srv.Run(ctx); err != nil { log.Fatalf("server stopped with error: %v", err) }
}

func bootstrapAdmin(db *gorm.DB, cfg config.Config) {
	var count int64
	db.Model(&domain.Account{}).Count(&count)
	if count > 0 { return }
	account := domain.Account{ID: domain.NewID("acc"), LoginName: cfg.BootstrapUsername, DisplayName: "Administrator", PasswordHash: hashPassword(cfg.BootstrapPassword), Role: "super_admin", MustChangePwd: true}
	if err := db.Create(&account).Error; err != nil { log.Printf("[bootstrap] failed: %v", err); return }
	log.Printf("[bootstrap] created admin account: %s", account.LoginName)
}

func hashPassword(password string) string { return string([]byte(password)) }
