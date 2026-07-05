package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

type SettingsHandler struct {
	repo *repository.Repository
	key  []byte
}

func NewSettingsHandler(repo *repository.Repository, key []byte) *SettingsHandler {
	return &SettingsHandler{repo: repo, key: key}
}

func (h *SettingsHandler) Get(c *gin.Context) {
	settings, _ := h.repo.GetAllSettings()
	dto := h.settingsMapToDTO(settings)
	creds, _ := h.repo.ListCredentials()
	for _, cred := range creds {
		cs := domain.CredentialStatus{Configured: cred.EncryptedSecret != "", MaskHint: cred.MaskHint, Identifier: cred.Identifier, LastVerified: cred.LastVerifiedAt, LastError: cred.LastError}
		switch cred.Provider {
		case "chmlfrp": dto.Integrations.Chmlfrp = cs
		case "onepanel": dto.Integrations.OnePanel = cs
		case "cloudflare": dto.Integrations.Cloudflare = cs
		}
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: dto})
}

func (h *SettingsHandler) Update(c *gin.Context) {
	var dto domain.SettingsDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "INVALID_REQUEST", Message: err.Error()}})
		return
	}
	sections := map[string]any{"general": dto.General, "sync": dto.Sync, "queue": dto.Queue, "frpc_runtime": dto.FRPCRuntime}
	for key, val := range sections {
		vj, _ := json.Marshal(val)
		_ = h.repo.SetSetting(key, string(vj))
	}
	if dto.Integrations.Chmlfrp.Identifier != "" || dto.Integrations.Chmlfrp.MaskHint != "" {
		h.upsert("chmlfrp", &dto.Integrations.Chmlfrp)
	}
	if dto.Integrations.Cloudflare.Identifier != "" || dto.Integrations.Cloudflare.MaskHint != "" {
		h.upsert("cloudflare", &dto.Integrations.Cloudflare)
	}
	if dto.Integrations.OnePanel.Identifier != "" || dto.Integrations.OnePanel.MaskHint != "" {
		h.upsert("onepanel", &dto.Integrations.OnePanel)
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]string{"message": "Settings updated"}})
}

func (h *SettingsHandler) upsert(provider string, cs *domain.CredentialStatus) {
	cred, err := h.repo.FindCredentialByProvider(provider)
	if err != nil {
		cred = &domain.UpstreamCredential{ID: domain.NewID("cre"), Provider: provider}
	}
	if cs.Identifier != "" { cred.Identifier = cs.Identifier }
	if cs.MaskHint != "" { cred.MaskHint = cs.MaskHint }
	if cs.Secret != "" {
		enc, encErr := security.Encrypt([]byte(cs.Secret), h.key)
		if encErr == nil {
			cred.EncryptedSecret = enc
		}
	}
	cred.UpdatedAt = time.Now()
	_ = h.repo.UpsertCredential(cred)
}

func (h *SettingsHandler) settingsMapToDTO(settings []domain.Setting) domain.SettingsDTO {
	dto := domain.SettingsDTO{
		General: domain.GeneralSettings{DefaultLogLines: 100, DataRetentionDays: 30, DefaultRefreshMode: "polling"},
		Sync:    domain.SyncSettings{HealthcheckInterval: "1m", SyncPollInterval: "10s", DiffStrategy: "pause_on_conflict", ManualOverridePriority: "manual_wins"},
		Queue:   domain.QueueSettings{MaxAttempts: 5, RetryBackoff: "30s", StalledJobPolicy: "mark_blocked", ArchiveRetentionDays: 30},
		FRPCRuntime: domain.FRPCRuntimeSettings{Enabled: false, BinarySource: "embedded", BinaryVersion: "0.54.0", LogLevel: "info", HealthcheckInterval: "30s", RestartBackoff: "30s", AutoRecoverStrategy: "reload_then_restart", SwitchNodeStrategy: "prefer_healthy_low_load"},
	}
	for _, s := range settings {
		switch s.Key {
		case "general": json.Unmarshal([]byte(s.ValueJSON), &dto.General)
		case "sync": json.Unmarshal([]byte(s.ValueJSON), &dto.Sync)
		case "queue": json.Unmarshal([]byte(s.ValueJSON), &dto.Queue)
		case "frpc_runtime": json.Unmarshal([]byte(s.ValueJSON), &dto.FRPCRuntime)
		}
	}
	return dto
}
