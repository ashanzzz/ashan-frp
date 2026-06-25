package http

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"ashan-frp/internal/config"
	"ashan-frp/internal/http/handlers"
	"ashan-frp/internal/http/middleware"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/web"
)

//go:embed openapi.json
var openapiSpec []byte

type Server struct {
	cfg  config.Config
	db   *gorm.DB
	repo *repository.Repository
	gin  *gin.Engine
}

func New(cfg config.Config, db *gorm.DB, repo *repository.Repository) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	s := &Server{cfg: cfg, db: db, repo: repo, gin: r}
	s.routes()
	return s
}

func (s *Server) Run(ctx context.Context) error {
	srv := &http.Server{Addr: s.cfg.HTTPAddr, Handler: s.gin}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	log.Printf("[ashan-frp] listening on %s", s.cfg.HTTPAddr)
	if err := srv.ListenAndServe(); err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) routes() {
	r := s.gin
	authH := handlers.NewAuthHandler(s.cfg, s.repo)
	tunnelH := handlers.NewTunnelHandler(s.cfg, s.repo)
	settingsH := handlers.NewSettingsHandler(s.repo)
	dashH := handlers.NewDashboardHandler(s.cfg, s.repo)

	r.GET("/api/v1/health", dashH.Health)
	r.GET("/api/v1/version", dashH.Version)
	r.POST("/api/v1/auth/login", authH.Login)

	api := r.Group("/api/v1")
	api.Use(middleware.AuthMiddleware(s.repo))
	{
		api.POST("/auth/logout", authH.Logout)
		api.GET("/auth/me", authH.Me)
		api.POST("/auth/password/change", authH.ChangePassword)
		api.GET("/auth/tokens", authH.ListTokens)
		api.POST("/auth/tokens/:id/revoke", authH.RevokeToken)
		api.GET("/tunnels", tunnelH.List)
		api.POST("/tunnels", tunnelH.Create)
		api.GET("/tunnels/:id", tunnelH.Get)
		api.PATCH("/tunnels/:id", tunnelH.Update)
		api.DELETE("/tunnels/:id", tunnelH.Delete)
		api.POST("/tunnels/:id/provision", tunnelH.Provision)
		api.GET("/settings", settingsH.Get)
		api.PATCH("/settings", settingsH.Update)
		api.GET("/dashboard", dashH.Dashboard)
		api.GET("/jobs", dashH.GetJobs)
		api.GET("/jobs/:id", dashH.GetJob)
		api.GET("/audit", dashH.GetAuditLogs)
		api.GET("/events/stream", dashH.EventsStream)
		api.GET("/openapi.json", func(c *gin.Context) { c.Data(http.StatusOK, "application/json; charset=utf-8", openapiSpec) })
		api.GET("/docs", func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", docsPage) })
	}

	uiFS, _ := web.FS()
	r.GET("/ui/*filepath", func(c *gin.Context) {
		path := c.Param("filepath")
		if path == "/" || path == "" {
			path = "/index.html"
		}
		if _, err := uiFS.Open(path[1:]); err == nil {
			c.FileFromFS(path, http.FS(uiFS))
			return
		}
		c.FileFromFS("/index.html", http.FS(uiFS))
	})
	r.GET("/", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/ui/") })
}

var docsPage = []byte(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><title>Ashan FRP API Docs</title><style>body{font-family:system-ui;margin:32px;line-height:1.6;background:#0d0e12;color:#e3e4e8}a{color:#4a9eed}pre{background:#16171f;padding:16px;border-radius:8px;overflow:auto}</style></head><body><h1>Ashan FRP API Docs</h1><p>OpenAPI: <a href="/api/v1/openapi.json">/api/v1/openapi.json</a></p><pre id="spec">loading...</pre><script>fetch("/api/v1/openapi.json").then(r=>r.text()).then(t=>document.getElementById("spec").textContent=t)</script></body></html>`)

var _ embed.FS