package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/integration/chmlfrp"
	"ashan-frp/internal/integration/cloudflare"
	"ashan-frp/internal/integration/onepanel"
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
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: h.settingsMapToView(settings)})
}

func (h *SettingsHandler) Update(c *gin.Context) {
	var req domain.SettingsPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "INVALID_REQUEST", Message: err.Error()}})
		return
	}

	sections := map[string]any{
		"general":      req.General,
		"sync":         req.Sync,
		"queue":        req.Queue,
		"frpc_runtime": req.FRPCRuntime,
	}
	for key, val := range sections {
		vj, _ := json.Marshal(val)
		_ = h.repo.SetSetting(key, string(vj))
	}

	integrations := req.Integrations
	integrations.ChmlFrp.Password = ""
	integrations.ChmlFrp.HasPassword = false
	integrations.ChmlFrp.LastValidatedAt = nil
	integrations.ChmlFrp.LastErrorMessage = ""
	integrations.OnePanel.APIToken = ""
	integrations.OnePanel.HasAPIToken = false
	integrations.OnePanel.LastValidatedAt = nil
	integrations.OnePanel.LastErrorMessage = ""
	integrations.Cloudflare.APIToken = ""
	integrations.Cloudflare.HasAPIToken = false
	integrations.Cloudflare.LastValidatedAt = nil
	integrations.Cloudflare.LastErrorMessage = ""
	if vj, err := json.Marshal(integrations); err == nil {
		_ = h.repo.SetSetting("integrations", string(vj))
	}

	h.upsertCredential("chmlfrp", req.Integrations.ChmlFrp.Username, req.Integrations.ChmlFrp.Password)
	h.upsertCredential("onepanel", req.Integrations.OnePanel.BaseURL, req.Integrations.OnePanel.APIToken)
	h.upsertCredential("cloudflare", req.Integrations.Cloudflare.ZoneName, req.Integrations.Cloudflare.APIToken)
	h.verifyIntegrations(req.Integrations)

	settings, _ := h.repo.GetAllSettings()
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: h.settingsMapToView(settings)})
}

func (h *SettingsHandler) verifyIntegrations(integrations domain.IntegrationSettings) {
	if integrations.ChmlFrp.Username != "" && integrations.ChmlFrp.Password != "" {
		cred, err := h.repo.FindCredentialByProvider("chmlfrp")
		if err == nil && cred != nil {
			now := time.Now()
			if err := chmlfrp.NewClient(integrations.ChmlFrp.Username, integrations.ChmlFrp.Password).Login(); err != nil {
				cred.LastError = err.Error()
			} else {
				cred.LastVerifiedAt = &now
				cred.LastError = ""
			}
			cred.UpdatedAt = now
			_ = h.repo.UpsertCredential(cred)
		}
	}
	if integrations.OnePanel.BaseURL != "" && integrations.OnePanel.APIToken != "" {
		cred, err := h.repo.FindCredentialByProvider("onepanel")
		if err == nil && cred != nil {
			now := time.Now()
			if err := onepanel.NewClient(integrations.OnePanel.BaseURL, integrations.OnePanel.APIToken).TestConnection(); err != nil {
				cred.LastError = err.Error()
			} else {
				cred.LastVerifiedAt = &now
				cred.LastError = ""
			}
			cred.UpdatedAt = now
			_ = h.repo.UpsertCredential(cred)
		}
	}
	cloudflareZone := integrations.Cloudflare.ZoneName
	if cloudflareZone == "" {
		cloudflareZone = integrations.Cloudflare.ZoneID
	}
	if cloudflareZone != "" && integrations.Cloudflare.APIToken != "" {
		cred, err := h.repo.FindCredentialByProvider("cloudflare")
		if err == nil && cred != nil {
			now := time.Now()
			if _, err := cloudflare.NewClient(integrations.Cloudflare.APIToken, cloudflareZone).ListRecords(); err != nil {
				cred.LastError = err.Error()
			} else {
				cred.LastVerifiedAt = &now
				cred.LastError = ""
			}
			cred.UpdatedAt = now
			_ = h.repo.UpsertCredential(cred)
		}
	}
}

func (h *SettingsHandler) upsertCredential(provider, identifier, secret string) {
	if identifier == "" && secret == "" {
		return
	}
	cred, err := h.repo.FindCredentialByProvider(provider)
	if err != nil {
		cred = &domain.UpstreamCredential{ID: domain.NewID("cre"), Provider: provider}
	}
	if identifier != "" {
		cred.Identifier = identifier
	}
	if secret != "" {
		enc, encErr := security.Encrypt([]byte(secret), h.key)
		if encErr == nil {
			cred.EncryptedSecret = enc
			cred.LastError = ""
		}
	}
	cred.UpdatedAt = time.Now()
	_ = h.repo.UpsertCredential(cred)
}

func (h *SettingsHandler) settingsMapToView(settings []domain.Setting) domain.SettingsPatchRequest {
	view := domain.SettingsPatchRequest{
		General:      domain.GeneralSettings{DefaultLogLines: 100, DataRetentionDays: 30, DefaultRefreshMode: "polling"},
		Sync:         domain.SyncSettings{HealthcheckInterval: "1m", SyncPollInterval: "10s", DiffStrategy: "pause_on_conflict", ManualOverridePriority: "manual_wins"},
		Queue:        domain.QueueSettings{MaxAttempts: 5, RetryBackoff: "30s", StalledJobPolicy: "mark_blocked", ArchiveRetentionDays: 30},
		FRPCRuntime:  domain.FRPCRuntimeSettings{Enabled: false, BinarySource: "embedded", BinaryVersion: "0.54.0", LogLevel: "info", HealthcheckInterval: "30s", RestartBackoff: "30s", AutoRecoverStrategy: "reload_then_restart", SwitchNodeStrategy: "prefer_healthy_low_load"},
		Integrations: domain.IntegrationSettings{},
	}
	for _, s := range settings {
		switch s.Key {
		case "general":
			_ = json.Unmarshal([]byte(s.ValueJSON), &view.General)
		case "sync":
			_ = json.Unmarshal([]byte(s.ValueJSON), &view.Sync)
		case "queue":
			_ = json.Unmarshal([]byte(s.ValueJSON), &view.Queue)
		case "frpc_runtime":
			_ = json.Unmarshal([]byte(s.ValueJSON), &view.FRPCRuntime)
		case "integrations":
			_ = json.Unmarshal([]byte(s.ValueJSON), &view.Integrations)
		}
	}

	creds, _ := h.repo.ListCredentials()
	for _, cred := range creds {
		switch cred.Provider {
		case "chmlfrp":
			if view.Integrations.ChmlFrp.Username == "" {
				view.Integrations.ChmlFrp.Username = cred.Identifier
			}
			view.Integrations.ChmlFrp.HasPassword = cred.EncryptedSecret != ""
			view.Integrations.ChmlFrp.UpdatedAt = cred.UpdatedAt
			view.Integrations.ChmlFrp.LastValidatedAt = cred.LastVerifiedAt
			view.Integrations.ChmlFrp.LastErrorMessage = cred.LastError
		case "onepanel":
			if view.Integrations.OnePanel.BaseURL == "" {
				view.Integrations.OnePanel.BaseURL = cred.Identifier
			}
			view.Integrations.OnePanel.HasAPIToken = cred.EncryptedSecret != ""
			view.Integrations.OnePanel.UpdatedAt = cred.UpdatedAt
			view.Integrations.OnePanel.LastValidatedAt = cred.LastVerifiedAt
			view.Integrations.OnePanel.LastErrorMessage = cred.LastError
		case "cloudflare":
			if view.Integrations.Cloudflare.ZoneName == "" {
				view.Integrations.Cloudflare.ZoneName = cred.Identifier
			}
			if view.Integrations.Cloudflare.ZoneID == "" {
				view.Integrations.Cloudflare.ZoneID = cred.Identifier
			}
			view.Integrations.Cloudflare.HasAPIToken = cred.EncryptedSecret != ""
			view.Integrations.Cloudflare.UpdatedAt = cred.UpdatedAt
			view.Integrations.Cloudflare.LastValidatedAt = cred.LastVerifiedAt
			view.Integrations.Cloudflare.LastErrorMessage = cred.LastError
		}
	}
	return view
}

func (h *SettingsHandler) settingsMapToDTO(settings []domain.Setting) domain.SettingsDTO {
	dto := domain.SettingsDTO{
		General:     domain.GeneralSettings{DefaultLogLines: 100, DataRetentionDays: 30, DefaultRefreshMode: "polling"},
		Sync:        domain.SyncSettings{HealthcheckInterval: "1m", SyncPollInterval: "10s", DiffStrategy: "pause_on_conflict", ManualOverridePriority: "manual_wins"},
		Queue:       domain.QueueSettings{MaxAttempts: 5, RetryBackoff: "30s", StalledJobPolicy: "mark_blocked", ArchiveRetentionDays: 30},
		FRPCRuntime: domain.FRPCRuntimeSettings{Enabled: false, BinarySource: "embedded", BinaryVersion: "0.54.0", LogLevel: "info", HealthcheckInterval: "30s", RestartBackoff: "30s", AutoRecoverStrategy: "reload_then_restart", SwitchNodeStrategy: "prefer_healthy_low_load"},
	}
	for _, s := range settings {
		switch s.Key {
		case "general":
			_ = json.Unmarshal([]byte(s.ValueJSON), &dto.General)
		case "sync":
			_ = json.Unmarshal([]byte(s.ValueJSON), &dto.Sync)
		case "queue":
			_ = json.Unmarshal([]byte(s.ValueJSON), &dto.Queue)
		case "frpc_runtime":
			_ = json.Unmarshal([]byte(s.ValueJSON), &dto.FRPCRuntime)
		}
	}

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
	return dto
}
