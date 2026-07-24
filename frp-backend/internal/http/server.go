package http

import (
	"context"
	"embed"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"ashan-frp/internal/config"
	"ashan-frp/internal/frpc"
	"ashan-frp/internal/http/handlers"
	"ashan-frp/internal/http/middleware"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
	"ashan-frp/internal/web"
)

//go:embed openapi.json
var openapiSpec []byte

type Server struct {
	cfg     config.Config
	db      *gorm.DB
	repo    *repository.Repository
	frpcMgr *frpc.Manager
	gin     *gin.Engine
}

func New(cfg config.Config, db *gorm.DB, repo *repository.Repository, frpcMgr *frpc.Manager) *Server {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestID())
	r.Use(middleware.Logger())
	s := &Server{cfg: cfg, db: db, repo: repo, frpcMgr: frpcMgr, gin: r}
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
	err := srv.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) routes() {
	r := s.gin
	authH := handlers.NewAuthHandler(s.cfg, s.repo)
	tunnelH := handlers.NewTunnelHandler(s.cfg, s.repo)
	settingsH := handlers.NewSettingsHandler(s.repo, security.DeriveEncryptionKey(s.cfg.EncryptionKey))
	dnsH := handlers.NewDNSHandler(s.repo, security.DeriveEncryptionKey(s.cfg.EncryptionKey))
	dashH := handlers.NewDashboardHandler(s.cfg, s.repo)
	nodeH := handlers.NewNodeHandler(s.repo)
	frpcH := handlers.NewFrpcHandler(s.cfg, s.frpcMgr, s.repo)
	webH := handlers.NewWebsiteMappingHandler(s.repo)

	r.GET("/api/v1/health", dashH.Health)
	r.GET("/api/v1/version", dashH.Version)
	r.GET("/api/openapi.json", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/api/v1/openapi.json") })
	r.GET("/api/docs", func(c *gin.Context) { c.Redirect(http.StatusMovedPermanently, "/api/v1/docs") })
	r.POST("/api/v1/auth/login", authH.Login)
	r.GET("/api/v1/auth/session", authH.Session)
	r.GET("/favicon.ico", func(c *gin.Context) { c.Status(http.StatusNoContent) })

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
		api.POST("/settings/integrations/cloudflare/verify", settingsH.VerifyCloudflare)
		api.POST("/settings/integrations/cloudflare/zones", settingsH.ListCloudflareZones)
		api.POST("/settings/integrations/chmlfrp/oauth/start", settingsH.StartChmlFrpOAuth)
		api.POST("/settings/integrations/chmlfrp/oauth/poll", settingsH.PollChmlFrpOAuth)
		api.GET("/dns/records", dnsH.List)
		api.POST("/dns/records", dnsH.Create)
		api.PATCH("/dns/records/:id", dnsH.Update)
		api.DELETE("/dns/records/:id", dnsH.Delete)
		api.POST("/dns/records/:id/claim", dnsH.Claim)
		api.POST("/dns/records/:id/unclaim", dnsH.Unclaim)
		api.GET("/dashboard", dashH.Dashboard)
		api.GET("/jobs", dashH.GetJobs)
		api.GET("/jobs/:id", dashH.GetJob)
		api.GET("/audit", dashH.GetAuditLogs)
		api.GET("/events/stream", dashH.EventsStream)
		api.GET("/nodes", nodeH.List)
		api.GET("/nodes/:id", nodeH.Get)
		api.POST("/nodes", nodeH.Create)
		api.PATCH("/nodes/:id", nodeH.Update)
		api.POST("/nodes/sync", nodeH.Sync)
		api.GET("/frpc/runtime", frpcH.Status)
		api.POST("/frpc/start", frpcH.Start)
		api.POST("/frpc/stop", frpcH.Stop)
		api.POST("/frpc/restart", frpcH.Restart)
		api.POST("/frpc/reload", frpcH.Reload)
		api.GET("/frpc/config", frpcH.GetConfig)
		api.GET("/frpc/logs", frpcH.GetLogs)
		api.GET("/website-mappings", webH.List)
		api.POST("/website-mappings", webH.Create)
		api.GET("/website-mappings/:id", webH.Get)
		api.PATCH("/website-mappings/:id", webH.Update)
		api.POST("/website-mappings/:id/sync", webH.Sync)
		api.GET("/openapi.json", func(c *gin.Context) { c.Data(http.StatusOK, "application/json; charset=utf-8", openapiSpec) })
		api.GET("/docs", func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", docsPage) })
	}

	uiFS, _ := web.FS()
	uiIndex, _ := fs.ReadFile(uiFS, "index.html")
	uiAppJS, _ := fs.ReadFile(uiFS, "app.js")
	uiStyles, _ := fs.ReadFile(uiFS, "styles.css")
	serveUIIndex := func(c *gin.Context) { c.Data(http.StatusOK, "text/html; charset=utf-8", uiIndex) }
	serveAppJS := func(c *gin.Context) { c.Data(http.StatusOK, "application/javascript; charset=utf-8", uiAppJS) }
	serveStyles := func(c *gin.Context) { c.Data(http.StatusOK, "text/css; charset=utf-8", uiStyles) }

	// UI entry. We resolve the embedded file bytes and reply with c.Data so
	// net/http's ServeFileFS cannot trigger its implicit directory redirect
	// (which otherwise causes a 301 loop on /ui/ and /ui/index.html).
	r.GET("/ui", serveUIIndex)
	r.GET("/ui/", serveUIIndex)
	r.GET("/ui/index.html", serveUIIndex)
	r.GET("/ui/styles.css", serveStyles)
	r.GET("/ui/app.js", serveAppJS)
	r.GET("/styles.css", serveStyles)
	r.GET("/app.js", serveAppJS)
	r.NoRoute(func(c *gin.Context) {
		if strings.HasPrefix(c.Request.URL.Path, "/ui") {
			serveUIIndex(c)
			return
		}
		c.JSON(http.StatusNotFound, gin.H{
			"error": gin.H{
				"code":    "not_found",
				"message": "resource not found",
			},
		})
	})
}

var docsPage = []byte(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><title>Ashan FRP API Docs</title><style>body{font-family:system-ui;margin:32px;line-height:1.6;background:#0d0e12;color:#e3e4e8}a{color:#4a9eed}pre{background:#16171f;padding:16px;border-radius:8px;overflow:auto}</style></head><body><h1>Ashan FRP API Docs</h1><p>OpenAPI: <a href="/api/v1/openapi.json">/api/v1/openapi.json</a></p><pre id="spec">loading...</pre><script>fetch("/api/v1/openapi.json").then(r=>r.text()).then(t=>document.getElementById("spec").textContent=t)</script></body></html>`)

var _ embed.FS
