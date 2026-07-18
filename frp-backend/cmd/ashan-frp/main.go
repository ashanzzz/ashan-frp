package main

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"ashan-frp/internal/config"
	"ashan-frp/internal/database"
	"ashan-frp/internal/frpc"
	"ashan-frp/internal/http"
	"ashan-frp/internal/observability"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
	"ashan-frp/internal/worker"
)

func main() {
	cfg := config.Load()
	logger, loggerCloser, loggerErr := observability.NewLogger(observability.Config{Level: cfg.LogLevel, FileEnabled: cfg.LogFileEnabled, FilePath: cfg.LogFilePath, MaxSizeMB: cfg.LogMaxSizeMB, MaxBackups: cfg.LogMaxBackups, RetentionDays: cfg.LogRetentionDays, Compress: cfg.LogCompress})
	if loggerErr != nil {
		log.Fatalf("initialize structured logger: %v", loggerErr)
	}
	defer loggerCloser.Close()
	slog.SetDefault(logger)
	log.SetOutput(slog.NewLogLogger(logger.Handler(), slog.LevelInfo).Writer())
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "admin":
			if err := runAdminCommand(cfg, os.Args[2:], os.Stdin, os.Stdout, os.Stderr, terminalPasswordPrompt(os.Stdin, os.Stdout)); err != nil {
				fmt.Fprintf(os.Stderr, "管理员命令失败：%v\n", err)
				os.Exit(1)
			}
			return
		case "serve":
		case "help", "-h", "--help":
			printRootUsage(os.Stdout)
			return
		default:
			fmt.Fprintf(os.Stderr, "未知命令：%s\n", os.Args[1])
			printRootUsage(os.Stderr)
			os.Exit(2)
		}
	}

	db, err := database.Open(cfg)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	account, err := database.BootstrapAdmin(db, cfg)
	if err != nil {
		log.Fatalf("bootstrap admin failed: %v", err)
	}
	if account != nil {
		log.Printf("[bootstrap] created admin account: %s", account.LoginName)
	}

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
	if err := srv.Run(ctx); err != nil {
		log.Fatalf("server stopped with error: %v", err)
	}
}
