package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"ashan-frp/internal/config"
	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
)

type DashboardHandler struct {
	cfg  config.Config
	repo *repository.Repository
}

func NewDashboardHandler(cfg config.Config, repo *repository.Repository) *DashboardHandler {
	return &DashboardHandler{cfg: cfg, repo: repo}
}

func (h *DashboardHandler) Version(c *gin.Context) {
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: domain.VersionInfo{
		Version: h.cfg.Version, Engine: "gin-gorm-sqlite", AppName: h.cfg.AppName,
		Status: "healthy", APIBase: h.cfg.APIBasePath, UIBase: h.cfg.UIBasePath,
	}})
}

func (h *DashboardHandler) Health(c *gin.Context) {
	tc, _ := h.repo.CountTunnels()
	qc, _ := h.repo.CountJobsByStatus("queued")
	rc, _ := h.repo.CountJobsByStatus("running")
	fc, _ := h.repo.CountJobsByStatus("failed")
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: domain.HealthInfo{
		Status: "healthy", Tunnels: int(tc), Jobs: int(qc + rc + fc),
	}})
}

func (h *DashboardHandler) Dashboard(c *gin.Context) {
	tunnels, _ := h.repo.ListTunnels(repository.TunnelFilter{})
	jobs, _ := h.repo.ListJobs(repository.JobFilter{})
	settings, _ := h.repo.GetAllSettings()
	auditLogs, _ := h.repo.ListAuditLogs(20)
	sh := &SettingsHandler{repo: h.repo}
	dto := sh.settingsMapToDTO(settings)
	creds, _ := h.repo.ListCredentials()
	for _, cred := range creds {
		cs := domain.CredentialStatus{Configured: cred.EncryptedSecret != "", MaskHint: cred.MaskHint, Identifier: cred.Identifier, LastVerified: cred.LastVerifiedAt, LastError: cred.LastError}
		switch cred.Provider {
		case "chmlfrp":
			dto.Integrations.Chmlfrp = cs
		case "onepanel":
			dto.Integrations.OnePanel = cs
		case "cloudflare":
			dto.Integrations.Cloudflare = cs
		}
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: domain.DashboardData{
		Version:         domain.VersionInfo{Version: h.cfg.Version, Engine: "gin-gorm-sqlite", AppName: h.cfg.AppName, Status: "healthy", APIBase: h.cfg.APIBasePath, UIBase: h.cfg.UIBasePath},
		Health:          domain.HealthInfo{Status: "healthy", Tunnels: len(tunnels), Jobs: len(jobs)},
		Tunnels:         tunnels,
		Jobs:            jobs,
		Settings:        dto,
		RecentAuditLogs: auditLogs,
	}})
}

func (h *DashboardHandler) GetJobs(c *gin.Context) {
	jobs, _ := h.repo.ListJobs(repository.JobFilter{Status: c.Query("status"), TargetType: c.Query("target_type"), TargetID: c.Query("target_id")})
	if jobs == nil {
		jobs = []domain.Job{}
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]any{"jobs": jobs}})
}

func (h *DashboardHandler) GetJob(c *gin.Context) {
	j, err := h.repo.FindJobByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, domain.ResponseEnvelope{Error: &domain.APIError{Code: "NOT_FOUND", Message: "Job not found"}})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: j})
}

func (h *DashboardHandler) GetAuditLogs(c *gin.Context) {
	logs, _ := h.repo.ListAuditLogs(100)
	if logs == nil {
		logs = []domain.AuditLog{}
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]any{"audit_logs": logs}})
}

type sseBroker struct {
	mu          sync.RWMutex
	subscribers map[string][]chan domain.Event
	history     []domain.Event
}

var broker = &sseBroker{subscribers: make(map[string][]chan domain.Event)}

func (b *sseBroker) Subscribe(channel string) chan domain.Event {
	b.mu.Lock()
	defer b.mu.Unlock()
	ch := make(chan domain.Event, 100)
	b.subscribers[channel] = append(b.subscribers[channel], ch)
	return ch
}

func (b *sseBroker) Unsubscribe(channel string, ch chan domain.Event) {
	b.mu.Lock()
	defer b.mu.Unlock()
	subs := b.subscribers[channel]
	for i, sub := range subs {
		if sub == ch {
			b.subscribers[channel] = append(subs[:i], subs[i+1:]...)
			close(ch)
			return
		}
	}
}

func (b *sseBroker) Publish(evt domain.Event) {
	b.mu.Lock()
	b.history = append(b.history, evt)
	if len(b.history) > 200 {
		b.history = b.history[len(b.history)-200:]
	}
	subs := make([]chan domain.Event, 0)
	if list, ok := b.subscribers[evt.Channel]; ok {
		subs = append(subs, list...)
	}
	if list, ok := b.subscribers[""]; ok {
		subs = append(subs, list...)
	}
	b.mu.Unlock()
	for _, ch := range subs {
		select {
		case ch <- evt:
		default:
		}
	}
}

func (b *sseBroker) History(channel, afterCursor string, limit int) []domain.Event {
	b.mu.RLock()
	defer b.mu.RUnlock()
	start := 0
	if afterCursor != "" {
		for i, evt := range b.history {
			if evt.Cursor == afterCursor {
				start = i + 1
				break
			}
		}
	}
	out := make([]domain.Event, 0, limit)
	for i := start; i < len(b.history); i++ {
		evt := b.history[i]
		if channel != "" && evt.Channel != channel {
			continue
		}
		out = append(out, evt)
		if len(out) >= limit {
			break
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

func PublishEvent(evt domain.Event) {
	if evt.Cursor == "" {
		evt.Cursor = domain.NewID("evt")
	}
	if evt.CreatedAt.IsZero() {
		evt.CreatedAt = time.Now().UTC()
	}
	if evt.SchemaVersion == 0 {
		evt.SchemaVersion = 1
	}
	broker.Publish(evt)
}

func (h *DashboardHandler) EventsStream(c *gin.Context) {
	channel := c.Query("channel")
	cursor := c.Query("cursor")
	c.Header("Content-Type", "text/event-stream; charset=utf-8")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{Error: &domain.APIError{Code: "SSE_UNSUPPORTED", Message: "Streaming not supported"}})
		return
	}
	for _, evt := range broker.History(channel, cursor, 100) {
		writeSSE(c.Writer, flusher, evt)
	}
	sub := broker.Subscribe(channel)
	defer broker.Unsubscribe(channel, sub)
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case evt, ok := <-sub:
			if !ok {
				return
			}
			writeSSE(c.Writer, flusher, evt)
		case <-heartbeat.C:
			c.Writer.WriteString(": ping\n\n")
			flusher.Flush()
		}
	}
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, evt domain.Event) {
	w.Write([]byte("id: " + evt.Cursor + "\n"))
	payload, _ := json.Marshal(evt)
	w.Write([]byte("data: " + string(payload) + "\n\n"))
	flusher.Flush()
}
