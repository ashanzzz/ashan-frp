package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "modernc.org/sqlite"

	"ashan-frp/internal/config"
	"ashan-frp/internal/domain"
	"ashan-frp/internal/frpc"
	"ashan-frp/internal/http"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
	"ashan-frp/internal/worker"
)

func main() {
	cfg := config.Load()
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil { log.Fatalf("failed to create data dir: %v", err) }

	db, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: cfg.DatabaseDSN + "?_journal_mode=WAL&_busy_timeout=5000"}), &gorm.Config{Logger: logger.Default.LogMode(logger.Warn)})
	if err != nil { log.Fatalf("failed to open database: %v", err) }

	if err := db.AutoMigrate(&domain.Account{}, &domain.AuthToken{}, &domain.UpstreamCredential{}, &domain.Node{}, &domain.Tunnel{}, &domain.Job{}, &domain.AuditLog{}, &domain.Snapshot{}, &domain.SyncState{}, &domain.Setting{}, &domain.WebsiteMapping{}); err != nil { log.Fatalf("auto-migrate failed: %v", err) }
	bootstrapAdmin(db, cfg)

	repo := repository.New(db)

	// FRPC Runtime Manager — manages the embedded frpc subprocess.
	frpcWorkDir := filepath.Join(cfg.DataDir, "frpc")
	frpcRenderer := frpc.NewConfigRenderer("", 0, "")
	frpcMgr := frpc.NewManager(frpcWorkDir, frpcRenderer)
	frpcMgr.OnStatusChange(func(s frpc.Status, reason string) {
		if reason != "" {
			log.Printf("[frpc] status=%s reason=%s", s, reason)
		} else {
			log.Printf("[frpc] status=%s", s)
		}
	})

	// Job Runner — processes queued/failed jobs.
	key := security.DeriveEncryptionKey(cfg.EncryptionKey)
	runner := worker.NewRunner(db, repo, key)
	runner.Start()
	defer runner.Stop()

	srv := http.New(cfg, db, repo, frpcMgr)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	log.Printf("[ashan-frp] v%s starting on %s", cfg.Version, cfg.HTTPAddr)
	if err := srv.Run(ctx); err != nil { log.Fatalf("server stopped with error: %v", err) }
}

func bootstrapAdmin(db *gorm.DB, cfg config.Config) {
	var count int64
	db.Model(&domain.Account{}).Count(&count)
	if count > 0 { return }
	hash, err := security.HashPassword(cfg.BootstrapPassword)
	if err != nil { log.Printf("[bootstrap] failed to hash password: %v", err); return }
	account := domain.Account{ID: domain.NewID("acc"), LoginName: cfg.BootstrapUsername, DisplayName: "Administrator", PasswordHash: hash, Role: "super_admin", MustChangePwd: true}
	if err := db.Create(&account).Error; err != nil { log.Printf("[bootstrap] failed: %v", err); return }
	log.Printf("[bootstrap] created admin account: %s", account.LoginName)
}
