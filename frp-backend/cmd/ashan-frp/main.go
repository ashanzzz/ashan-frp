package main

import (
    "context"
    "log"
    "os/signal"
    "syscall"

    "ashan-frp/internal/app"
    "ashan-frp/internal/config"
)

func main() {
    cfg := config.Load()
    server, err := app.New(cfg)
    if err != nil {
        log.Fatalf("bootstrap failed: %v", err)
    }

    ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
    defer stop()

    if err := server.Run(ctx); err != nil {
        log.Fatalf("server stopped with error: %v", err)
    }
}
