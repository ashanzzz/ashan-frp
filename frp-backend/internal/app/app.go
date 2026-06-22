package app

import (
    "context"

    "ashan-frp/internal/config"
    "ashan-frp/internal/server"
)

type App struct {
    server *server.Server
}

func New(cfg config.Config) (*App, error) {
    srv, err := server.New(cfg)
    if err != nil {
        return nil, err
    }
    return &App{server: srv}, nil
}

func (a *App) Run(ctx context.Context) error {
    return a.server.Run(ctx)
}
