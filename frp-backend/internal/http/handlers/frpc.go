package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ashan-frp/internal/config"
	"ashan-frp/internal/domain"
	"ashan-frp/internal/frpc"
	"ashan-frp/internal/repository"
)

// FrpcHandler exposes the FRPC Runtime Manager over HTTP.
type FrpcHandler struct {
	cfg  config.Config
	mgr  *frpc.Manager
	repo *repository.Repository
}

func NewFrpcHandler(cfg config.Config, mgr *frpc.Manager, repo *repository.Repository) *FrpcHandler {
	return &FrpcHandler{cfg: cfg, mgr: mgr, repo: repo}
}

// Status returns the current FRPC runtime status.
// GET /api/v1/frpc/runtime
func (h *FrpcHandler) Status(c *gin.Context) {
	status := h.mgr.Status()
	lastError := h.mgr.LastError()
	lastCheck := h.mgr.LastHealthcheck()
	statusStr, healthReason := h.mgr.Healthcheck()

	c.JSON(http.StatusOK, domain.ResponseEnvelope{
		Data: map[string]any{
			"status":         string(status),
			"health_status":  string(statusStr),
			"health_reason":  healthReason,
			"last_error":     lastError,
			"last_healthcheck": lastCheck,
		},
	})
}

// Start starts the frpc subprocess.
// POST /api/v1/frpc/start
func (h *FrpcHandler) Start(c *gin.Context) {
	tunnels, err := h.repo.ListTunnels(repository.TunnelFilter{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INTERNAL", Message: "Failed to list tunnels"},
		})
		return
	}
	enabled := make([]domain.Tunnel, 0, len(tunnels))
	for _, t := range tunnels {
		if t.DesiredState == domain.TunnelDesiredEnabled {
			enabled = append(enabled, t)
		}
	}
	if err := h.mgr.Start(c.Request.Context(), enabled); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "START_FAILED", Message: err.Error()},
		})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{
		Data: map[string]string{"message": "frpc start requested"},
	})
}

// Stop stops the frpc subprocess.
// POST /api/v1/frpc/stop
func (h *FrpcHandler) Stop(c *gin.Context) {
	if err := h.mgr.Stop(); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "STOP_FAILED", Message: err.Error()},
		})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{
		Data: map[string]string{"message": "frpc stopped"},
	})
}

// Restart stops then starts the frpc subprocess.
// POST /api/v1/frpc/restart
func (h *FrpcHandler) Restart(c *gin.Context) {
	tunnels, err := h.repo.ListTunnels(repository.TunnelFilter{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INTERNAL", Message: "Failed to list tunnels"},
		})
		return
	}
	enabled := make([]domain.Tunnel, 0, len(tunnels))
	for _, t := range tunnels {
		if t.DesiredState == domain.TunnelDesiredEnabled {
			enabled = append(enabled, t)
		}
	}
	if err := h.mgr.Restart(c.Request.Context(), enabled); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "RESTART_FAILED", Message: err.Error()},
		})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{
		Data: map[string]string{"message": "frpc restart requested"},
	})
}
