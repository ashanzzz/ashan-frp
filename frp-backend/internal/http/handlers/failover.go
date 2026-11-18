package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
)

// FailoverSupervisorEngine defines the interface required by FailoverHandler.
type FailoverSupervisorEngine interface {
	GetStatus() domain.FailoverStatus
	GetSettings() domain.FailoverSettings
	SaveSettings(domain.FailoverSettings) error
	TriggerFailoverManual(reason string) (*domain.FailoverStatus, error)
	OptimizeWeeklyBestNode(reason string, force bool) (*domain.FailoverStatus, error)
}

type FailoverHandler struct {
	supervisor FailoverSupervisorEngine
	repo       *repository.Repository
}

func NewFailoverHandler(supervisor FailoverSupervisorEngine, repo *repository.Repository) *FailoverHandler {
	return &FailoverHandler{supervisor: supervisor, repo: repo}
}

// GetStatus returns the current failover status and tracked node health.
// GET /api/v1/failover/status
func (h *FailoverHandler) GetStatus(c *gin.Context) {
	if h.supervisor == nil {
		c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: domain.FailoverStatus{}})
		return
	}
	st := h.supervisor.GetStatus()
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: st})
}

// GetSettings returns failover configuration.
// GET /api/v1/failover/settings
func (h *FailoverHandler) GetSettings(c *gin.Context) {
	if h.supervisor == nil {
		c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: domain.FailoverSettings{AutoFailoverEnabled: true, OfflineThresholdMin: 10, HeartbeatIntervalSec: 30, WeeklyOptimizeEnabled: true}})
		return
	}
	settings := h.supervisor.GetSettings()
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: settings})
}

// UpdateSettings modifies failover configuration.
// POST /api/v1/failover/settings
func (h *FailoverHandler) UpdateSettings(c *gin.Context) {
	if h.supervisor == nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{Error: &domain.APIError{Code: "UNAVAILABLE", Message: "Failover supervisor not initialized"}})
		return
	}
	var input domain.FailoverSettings
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "INVALID_REQUEST", Message: err.Error()}})
		return
	}
	if input.OfflineThresholdMin <= 0 {
		input.OfflineThresholdMin = 10
	}
	if input.HeartbeatIntervalSec <= 0 {
		input.HeartbeatIntervalSec = 30
	}
	if input.WeeklyOptimizeSchedule == "" {
		input.WeeklyOptimizeSchedule = "Monday 03:30"
	}
	if input.MinImprovementPct <= 0 {
		input.MinImprovementPct = 15
	}
	if err := h.supervisor.SaveSettings(input); err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{Error: &domain.APIError{Code: "DB_ERROR", Message: err.Error()}})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: input})
}

// TriggerManual triggers manual failover test.
// POST /api/v1/failover/trigger
func (h *FailoverHandler) TriggerManual(c *gin.Context) {
	if h.supervisor == nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{Error: &domain.APIError{Code: "UNAVAILABLE", Message: "Failover supervisor not initialized"}})
		return
	}
	st, err := h.supervisor.TriggerFailoverManual("Manual operator trigger from console")
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{Error: &domain.APIError{Code: "FAILOVER_FAILED", Message: err.Error()}, Data: st})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: st})
}

// OptimizeWeekly triggers weekly best-node speed test and optimization switch.
// POST /api/v1/failover/optimize
func (h *FailoverHandler) OptimizeWeekly(c *gin.Context) {
	if h.supervisor == nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{Error: &domain.APIError{Code: "UNAVAILABLE", Message: "Failover supervisor not initialized"}})
		return
	}
	var input struct {
		Force  bool   `json:"force"`
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&input)
	if input.Reason == "" {
		input.Reason = "Manual operator optimization trigger from console"
	}
	st, err := h.supervisor.OptimizeWeeklyBestNode(input.Reason, input.Force)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{Error: &domain.APIError{Code: "OPTIMIZATION_FAILED", Message: err.Error()}, Data: st})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: st})
}

// GetLogs returns failover-specific audit logs.
// GET /api/v1/failover/logs
func (h *FailoverHandler) GetLogs(c *gin.Context) {
	logs, _, err := h.repo.ListAuditLogsFiltered(repository.AuditFilter{
		Action: "failover",
		Limit:  50,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{Error: &domain.APIError{Code: "DB_ERROR", Message: err.Error()}})
		return
	}
	if logs == nil {
		logs = []domain.AuditLog{}
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]any{"logs": logs}})
}
