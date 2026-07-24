package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/integration/chmlfrp"
	"ashan-frp/internal/integration/cloudflare"
	"ashan-frp/internal/integration/onepanel"
	"ashan-frp/internal/observability"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

type cloudflareVerifier interface {
	VerifyToken() error
	ResolveZone() error
	ListRecords() ([]domain.CFDNSRecord, error)
	ListZones() ([]domain.CFZone, error)
}

type SettingsHandler struct {
	repo          *repository.Repository
	key           []byte
	clientFactory func(token, zone string) cloudflareVerifier
}

func NewSettingsHandler(repo *repository.Repository, key []byte) *SettingsHandler {
	return &SettingsHandler{
		repo: repo,
		key:  key,
		clientFactory: func(token, zone string) cloudflareVerifier {
			return cloudflare.NewClient(token, zone)
		},
	}
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

	h.upsertCredential(c, "chmlfrp", req.Integrations.ChmlFrp.Username, req.Integrations.ChmlFrp.Password)
	h.upsertCredential(c, "onepanel", req.Integrations.OnePanel.BaseURL, req.Integrations.OnePanel.APIToken)
	cfZone := req.Integrations.Cloudflare.ZoneName
	cfToken := req.Integrations.Cloudflare.APIToken
	if cfZone == "" {
		tokenToUse := cfToken
		if tokenToUse == "" {
			cred, _ := h.repo.FindCredentialByProvider("cloudflare")
			if cred != nil && cred.EncryptedSecret != "" {
				sec, _ := security.Decrypt(cred.EncryptedSecret, h.key)
				tokenToUse = string(sec)
			}
		}
		if tokenToUse != "" {
			cli := h.clientFactory(tokenToUse, "")
			zones, err := cli.ListZones()
			if err == nil {
				if len(zones) == 1 {
					req.Integrations.Cloudflare.ZoneName = zones[0].Name
				} else if len(zones) > 1 {
					c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "MULTIPLE_ZONES", Message: "该 Token 关联了多个 Zone，请在界面点击加载并选择一个"}})
					return
				} else {
					c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "NO_ZONES", Message: "该 Token 没有关联任何可用的 Zone"}})
					return
				}
			} else {
				c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "CLOUDFLARE_ERROR", Message: "自动获取 Zone 失败: " + err.Error()}})
				return
			}
		}
	}
	h.upsertCredential(c, "cloudflare", req.Integrations.Cloudflare.ZoneName, req.Integrations.Cloudflare.APIToken)
	h.verifyIntegrations(c, req.Integrations)

	h.auditSettingsUpdate(c)
	settings, _ := h.repo.GetAllSettings()
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: h.settingsMapToView(settings)})
}

func (h *SettingsHandler) auditSettingsUpdate(c *gin.Context) {
	_ = h.repo.CreateAuditLog(&domain.AuditLog{ID: domain.NewID("aud"), AccountID: c.GetString("account_id"), AccountName: c.GetString("account_name"), Action: "settings.update", ResourceType: "settings", ResourceID: "global", RequestID: c.GetString("request_id"), TraceID: c.GetString("trace_id"), Outcome: "success", IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")})
}

func (h *SettingsHandler) verifyIntegrations(c *gin.Context, integrations domain.IntegrationSettings) {
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
	_, _ = h.verifyCloudflareCredential(c)

}

type cloudflareVerificationStep struct {
	Name       string `json:"name"`
	Outcome    string `json:"outcome"`
	ErrorCode  string `json:"error_code,omitempty"`
	HTTPStatus int    `json:"http_status,omitempty"`
	DurationMS int64  `json:"duration_ms"`
}
type cloudflareVerificationResult struct {
	VerificationID string                       `json:"verification_id"`
	RequestID      string                       `json:"request_id,omitempty"`
	CredentialRef  string                       `json:"credential_ref,omitempty"`
	Steps          []cloudflareVerificationStep `json:"steps"`
}

func (h *SettingsHandler) VerifyCloudflare(c *gin.Context) {
	result, err := h.verifyCloudflareCredential(c)
	settings, _ := h.repo.GetAllSettings()
	view := h.settingsMapToView(settings)
	message := "Cloudflare Token and Zone access verified"
	if err != nil {
		message = err.Error()
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]any{"valid": err == nil, "message": message, "cloudflare": view.Integrations.Cloudflare, "verification_id": result.VerificationID, "request_id": result.RequestID, "credential_ref": result.CredentialRef, "steps": result.Steps}})
}

func (h *SettingsHandler) verifyCloudflareCredential(c *gin.Context) (cloudflareVerificationResult, error) {
	result := cloudflareVerificationResult{VerificationID: domain.NewID("ver"), Steps: []cloudflareVerificationStep{}}
	if c != nil {
		result.RequestID = c.GetString("request_id")
	}
	var cred *domain.UpstreamCredential
	var secret []byte
	run := func(name string, operation func() error) error {
		started := time.Now()
		err := operation()
		code, status := cloudflareError(err)
		outcome := "success"
		level := slog.LevelInfo
		if err != nil {
			outcome = "failure"
			level = slog.LevelWarn
		}
		step := cloudflareVerificationStep{Name: name, Outcome: outcome, ErrorCode: code, HTTPStatus: status, DurationMS: time.Since(started).Milliseconds()}
		result.Steps = append(result.Steps, step)
		slog.Log(c.Request.Context(), level, "integration.verification.step", "component", "integration", "event", "verification.step", "provider", "cloudflare", "operation", name, "verification_id", result.VerificationID, "request_id", result.RequestID, "credential_ref", result.CredentialRef, "outcome", outcome, "error_code", code, "http_status", status, "duration_ms", step.DurationMS)
		return err
	}
	ctx := c
	if ctx == nil {
		ctx = &gin.Context{}
	}
	err := run("credential.load", func() error {
		found, e := h.repo.FindCredentialByProvider("cloudflare")
		if e != nil || found == nil || found.EncryptedSecret == "" {
			return fmt.Errorf("CLOUDFLARE_NOT_CONFIGURED: Cloudflare API Token is not configured")
		}
		cred = found
		result.CredentialRef = found.CredentialRef
		return nil
	})
	if err == nil {
		err = run("credential.decrypt", func() error {
			value, e := security.Decrypt(cred.EncryptedSecret, h.key)
			if e != nil {
				return fmt.Errorf("CLOUDFLARE_CREDENTIAL_DECRYPT_FAILED: stored Cloudflare credential could not be read")
			}
			secret = value
			return nil
		})
	}
	var client cloudflareVerifier
	if err == nil {
		client = h.clientFactory(string(secret), cred.Identifier)
		err = run("cloudflare.token.verify", client.VerifyToken)
	}
	if err == nil {
		err = run("cloudflare.zone.resolve", client.ResolveZone)
	}
	if err == nil {
		err = run("cloudflare.dns.read", func() error { _, e := client.ListRecords(); return e })
	}
	now := time.Now()
	if cred != nil {
		cred.UpdatedAt = now
		if err != nil {
			cred.LastVerifiedAt = nil
			cred.LastError = err.Error()
		} else {
			cred.LastVerifiedAt = &now
			cred.LastError = ""
		}
		if saveErr := h.repo.UpsertCredential(cred); saveErr != nil && err == nil {
			err = fmt.Errorf("CLOUDFLARE_STATUS_SAVE_FAILED: save verification status")
		}
		h.auditVerification(ctx, result, cred, err)
	}
	return result, err
}

func cloudflareError(err error) (string, int) {
	if err == nil {
		return "", 0
	}
	text := err.Error()
	switch {
	case errors.Is(err, cloudflare.ErrTokenInvalid) || strings.Contains(text, "http 401") || strings.Contains(text, "TOKEN_INVALID"):
		return "CLOUDFLARE_TOKEN_INVALID", 401
	case errors.Is(err, cloudflare.ErrTokenUnconfigured) || strings.Contains(text, "not configured"):
		return "CLOUDFLARE_NOT_CONFIGURED", 0
	case errors.Is(err, cloudflare.ErrZoneNotFound) || strings.Contains(text, "zone not found"):
		return "CLOUDFLARE_ZONE_NOT_FOUND", 404
	case strings.Contains(strings.ToLower(text), "timeout"):
		return "CLOUDFLARE_TIMEOUT", 0
	default:
		return "CLOUDFLARE_REQUEST_FAILED", 0
	}
}
func (h *SettingsHandler) auditVerification(c *gin.Context, result cloudflareVerificationResult, cred *domain.UpstreamCredential, operationErr error) {
	outcome := "success"
	code := ""
	if operationErr != nil {
		outcome = "failure"
		code, _ = cloudflareError(operationErr)
	}
	detail, _ := json.Marshal(map[string]any{"provider": "cloudflare", "zone": cred.Identifier, "verification_id": result.VerificationID, "credential_revision": cred.Revision, "token_mask": cred.MaskHint, "steps": result.Steps})
	var durationMS int64
	for _, step := range result.Steps {
		durationMS += step.DurationMS
	}
	accountID, accountName, ip, userAgent, traceID := "", "", "", "", ""
	if c != nil {
		accountID = c.GetString("account_id")
		accountName = c.GetString("account_name")
		ip = c.ClientIP()
		userAgent = c.GetHeader("User-Agent")
		traceID = c.GetString("trace_id")
	}
	_ = h.repo.CreateAuditLog(&domain.AuditLog{ID: domain.NewID("aud"), AccountID: accountID, AccountName: accountName, Action: "cloudflare.credential.verify", ResourceType: "credential", ResourceID: cred.ID, DetailJSON: string(detail), RequestID: result.RequestID, TraceID: traceID, Outcome: outcome, DurationMS: durationMS, ErrorCode: code, CredentialRef: cred.CredentialRef, IPAddress: ip, UserAgent: userAgent})
}

func (h *SettingsHandler) upsertCredential(c *gin.Context, provider, identifier, secret string) {
	started := time.Now()
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
			cred.MaskHint = observability.TokenMask(secret)
			cred.CredentialRef = observability.CredentialRef(secret, h.key)
			cred.Revision++
			cred.LastError = ""
		}
	}
	cred.UpdatedAt = time.Now()
	_ = h.repo.UpsertCredential(cred)
	slog.Info("credential.saved", "component", "settings", "event", "credential.save", "provider", provider, "identifier", cred.Identifier, "credential_ref", cred.CredentialRef, "credential_revision", cred.Revision, "token_mask", cred.MaskHint, "request_id", c.GetString("request_id"), "account_id", c.GetString("account_id"))
	if provider == "cloudflare" {
		detail, _ := json.Marshal(map[string]any{"provider": provider, "zone": cred.Identifier, "token_mask": cred.MaskHint, "credential_revision": cred.Revision})
		_ = h.repo.CreateAuditLog(&domain.AuditLog{ID: domain.NewID("aud"), AccountID: c.GetString("account_id"), AccountName: c.GetString("account_name"), Action: "cloudflare.credential.save", ResourceType: "credential", ResourceID: cred.ID, DetailJSON: string(detail), RequestID: c.GetString("request_id"), TraceID: c.GetString("trace_id"), Outcome: "success", DurationMS: time.Since(started).Milliseconds(), CredentialRef: cred.CredentialRef, IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")})
	}
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
			identifier := cred.Identifier
			// If the stored identifier looks like a Zone ID (32-char hex, no dots),
			// try to resolve it to a domain name for display.
			if identifier != "" && !strings.Contains(identifier, ".") && len(identifier) >= 20 {
				if cred.EncryptedSecret != "" {
					if sec, decErr := security.Decrypt(cred.EncryptedSecret, h.key); decErr == nil {
						cli := h.clientFactory(string(sec), "")
						if zones, zErr := cli.ListZones(); zErr == nil {
							for _, z := range zones {
								if z.ID == identifier {
									identifier = z.Name
									cred.Identifier = z.Name
									_ = h.repo.UpsertCredential(&cred)
									break
								}
							}
						}
					}
				}
			}
			if view.Integrations.Cloudflare.ZoneName == "" {
				view.Integrations.Cloudflare.ZoneName = identifier
			}
			view.Integrations.Cloudflare.HasAPIToken = cred.EncryptedSecret != ""
			// Return plaintext API token for personal-use display.
			if cred.EncryptedSecret != "" {
				if sec, decErr := security.Decrypt(cred.EncryptedSecret, h.key); decErr == nil {
					view.Integrations.Cloudflare.APIToken = string(sec)
				}
			}
			view.Integrations.Cloudflare.UpdatedAt = cred.UpdatedAt
			view.Integrations.Cloudflare.LastValidatedAt = cred.LastVerifiedAt
			view.Integrations.Cloudflare.LastErrorMessage = cred.LastError
			view.Integrations.Cloudflare.TokenMask = cred.MaskHint
			view.Integrations.Cloudflare.CredentialRef = cred.CredentialRef
			view.Integrations.Cloudflare.CredentialRevision = cred.Revision
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
		cs := domain.CredentialStatus{Configured: cred.EncryptedSecret != "", MaskHint: cred.MaskHint, CredentialRef: cred.CredentialRef, CredentialRevision: cred.Revision, Identifier: cred.Identifier, LastVerified: cred.LastVerifiedAt, LastError: cred.LastError}
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

func (h *SettingsHandler) ListCloudflareZones(c *gin.Context) {
	var input struct {
		Token string `json:"token"`
	}
	_ = c.ShouldBindJSON(&input)
	token := input.Token
	if token == "" {
		cred, _ := h.repo.FindCredentialByProvider("cloudflare")
		if cred != nil && cred.EncryptedSecret != "" {
			sec, err := security.Decrypt(cred.EncryptedSecret, h.key)
			if err == nil {
				token = string(sec)
			}
		}
	}
	if token == "" {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "INVALID_REQUEST", Message: "token is required or not configured"}})
		return
	}
	cli := h.clientFactory(token, "")
	zones, err := cli.ListZones()
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{Error: &domain.APIError{Code: "CLOUDFLARE_ERROR", Message: err.Error()}})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]any{"zones": zones}})
}
