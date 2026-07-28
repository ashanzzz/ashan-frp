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

type chmlfrpUserClient interface {
	GetCurrentUser() (*domain.ChmlFrpUserInfo, error)
}

type SettingsHandler struct {
	repo                    *repository.Repository
	key                     []byte
	clientFactory           func(token, zone string) cloudflareVerifier
	credentialClientFactory func(credentials cloudflare.Credentials, zone string) cloudflareVerifier
	chmlfrpClientFactory    func(token string) chmlfrpUserClient
}

func NewSettingsHandler(repo *repository.Repository, key []byte) *SettingsHandler {
	return &SettingsHandler{
		repo: repo,
		key:  key,
		clientFactory: func(token, zone string) cloudflareVerifier {
			return cloudflare.NewClient(token, zone)
		},
		credentialClientFactory: func(credentials cloudflare.Credentials, zone string) cloudflareVerifier {
			return cloudflare.NewClientWithCredentials(credentials, zone)
		},
		chmlfrpClientFactory: func(token string) chmlfrpUserClient {
			return chmlfrp.NewClient("token", token)
		},
	}
}

func (h *SettingsHandler) Get(c *gin.Context) {
	settings, _ := h.repo.GetAllSettings()
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: h.settingsMapToView(settings)})
}

func (h *SettingsHandler) ConfigureCloudflare(c *gin.Context) {
	var input struct {
		Secret string `json:"secret" binding:"required"`
		Email  string `json:"email"`
		ZoneID string `json:"zone_id"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "INVALID_REQUEST", Message: "Cloudflare credential is required"}})
		return
	}
	input.Secret = strings.TrimSpace(input.Secret)
	input.Email = strings.TrimSpace(input.Email)
	input.ZoneID = strings.TrimSpace(input.ZoneID)
	if strings.HasPrefix(input.Secret, "cfk_") && input.Email == "" {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "CLOUDFLARE_EMAIL_REQUIRED", Message: "Global API Key requires the Cloudflare account email"}})
		return
	}

	var detected cloudflare.Credentials
	var zones []domain.CFZone
	var probeErr error
	for _, candidate := range cloudflare.AuthCandidates(input.Secret, input.Email) {
		if candidate.AuthMethod == cloudflare.AuthMethodGlobalAPIKey && candidate.Email == "" {
			continue
		}
		client := h.credentialClientFactory(candidate, "")
		if candidate.AuthMethod == cloudflare.AuthMethodAPIToken {
			if err := client.VerifyToken(); err != nil {
				probeErr = err
				continue
			}
		}
		candidateZones, err := client.ListZones()
		if err != nil {
			probeErr = err
			continue
		}
		detected = candidate
		zones = candidateZones
		probeErr = nil
		break
	}
	if detected.AuthMethod == "" {
		if input.Email == "" && !strings.HasPrefix(input.Secret, "cfut_") {
			c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "CLOUDFLARE_EMAIL_REQUIRED", Message: "API Token verification failed; enter the Cloudflare account email to test Global API Key authentication"}})
			return
		}
		_ = probeErr
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "CLOUDFLARE_CREDENTIAL_INVALID", Message: "Cloudflare credential verification failed"}})
		return
	}
	if len(zones) == 0 {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "CLOUDFLARE_NO_ZONES", Message: "No accessible Cloudflare zones were found"}})
		return
	}

	var selected *domain.CFZone
	if input.ZoneID != "" {
		for i := range zones {
			if zones[i].ID == input.ZoneID {
				selected = &zones[i]
				break
			}
		}
		if selected == nil {
			c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "CLOUDFLARE_ZONE_NOT_ACCESSIBLE", Message: "The selected zone is not accessible with this credential"}})
			return
		}
	} else if len(zones) == 1 {
		selected = &zones[0]
	} else {
		c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]any{
			"status":        "zone_selection_required",
			"auth_method":   detected.AuthMethod,
			"account_email": detected.Email,
			"zones":         zones,
		}})
		return
	}

	zoneClient := h.credentialClientFactory(detected, selected.ID)
	if _, err := zoneClient.ListRecords(); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "CLOUDFLARE_DNS_READ_FAILED", Message: "Cloudflare DNS read verification failed for the selected zone"}})
		return
	}

	encrypted, err := security.Encrypt([]byte(detected.Secret), h.key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{Error: &domain.APIError{Code: "CREDENTIAL_SAVE_FAILED", Message: "Cloudflare credential could not be encrypted"}})
		return
	}
	credential, err := h.repo.FindCredentialByProvider("cloudflare")
	if err != nil || credential == nil {
		credential = &domain.UpstreamCredential{ID: domain.NewID("cre"), Provider: "cloudflare"}
	}
	now := time.Now()
	credentialRef := observability.CredentialRef(detected.Secret, h.key)
	if credential.CredentialRef != credentialRef {
		credential.Revision++
	}
	credential.Identifier = selected.Name
	credential.AuthMethod = detected.AuthMethod
	credential.AccountEmail = detected.Email
	credential.ZoneID = selected.ID
	credential.EncryptedSecret = encrypted
	credential.MaskHint = observability.TokenMask(detected.Secret)
	credential.CredentialRef = credentialRef
	credential.LastVerifiedAt = &now
	credential.LastError = ""
	credential.UpdatedAt = now

	integrations := h.currentIntegrationSettings()
	integrations.Cloudflare = domain.CloudflareCredentials{
		AuthMethod:   detected.AuthMethod,
		AccountEmail: detected.Email,
		ZoneID:       selected.ID,
		ZoneName:     selected.Name,
	}
	integrations.ChmlFrp.Password = ""
	integrations.OnePanel.APIToken = ""
	settingsJSON, err := json.Marshal(integrations)
	if err != nil || h.repo.SaveCredentialAndSetting(credential, "integrations", string(settingsJSON)) != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{Error: &domain.APIError{Code: "CREDENTIAL_SAVE_FAILED", Message: "Verified Cloudflare configuration could not be saved"}})
		return
	}

	detail, _ := json.Marshal(map[string]any{
		"provider": "cloudflare", "auth_method": detected.AuthMethod, "zone": selected.Name,
		"credential_ref": credential.CredentialRef, "credential_revision": credential.Revision,
	})
	_ = h.repo.CreateAuditLog(&domain.AuditLog{ID: domain.NewID("aud"), AccountID: c.GetString("account_id"), AccountName: c.GetString("account_name"), Action: "cloudflare.credential.save", ResourceType: "credential", ResourceID: credential.ID, DetailJSON: string(detail), RequestID: c.GetString("request_id"), TraceID: c.GetString("trace_id"), Outcome: "success", CredentialRef: credential.CredentialRef, IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")})
	slog.Info("credential.saved", "component", "settings", "event", "credential.save", "provider", "cloudflare", "auth_method", detected.AuthMethod, "zone", selected.Name, "credential_ref", credential.CredentialRef, "credential_revision", credential.Revision, "token_mask", credential.MaskHint, "request_id", c.GetString("request_id"), "account_id", c.GetString("account_id"))

	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]any{
		"status": "saved", "auth_method": detected.AuthMethod, "account_email": detected.Email,
		"zone_id": selected.ID, "zone_name": selected.Name, "secret": detected.Secret,
	}})
}

func (h *SettingsHandler) currentIntegrationSettings() domain.IntegrationSettings {
	var integrations domain.IntegrationSettings
	setting, err := h.repo.GetSetting("integrations")
	if err == nil && setting != nil {
		_ = json.Unmarshal([]byte(setting.ValueJSON), &integrations)
	}
	return integrations
}

func (h *SettingsHandler) storedCloudflareClient(secret string, credential *domain.UpstreamCredential, zone string) cloudflareVerifier {
	authMethod := credential.AuthMethod
	if authMethod == "" || authMethod == cloudflare.AuthMethodAPIToken {
		return h.clientFactory(secret, zone)
	}
	return h.credentialClientFactory(cloudflare.Credentials{
		AuthMethod: authMethod,
		Secret:     secret,
		Email:      credential.AccountEmail,
	}, zone)
}

func (h *SettingsHandler) Update(c *gin.Context) {
	var req domain.SettingsPatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "INVALID_REQUEST", Message: err.Error()}})
		return
	}
	if strings.TrimSpace(req.Integrations.Cloudflare.APIToken) != "" {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "USE_CLOUDFLARE_CONFIGURE", Message: "Cloudflare credentials must be verified and saved through the configure endpoint"}})
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

	currentIntegrations := h.currentIntegrationSettings()
	integrations := req.Integrations
	// Cloudflare credential identity and Zone metadata are owned exclusively by
	// ConfigureCloudflare. A generic settings PATCH must not overwrite verified
	// values with client-controlled metadata.
	integrations.Cloudflare = currentIntegrations.Cloudflare
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

	if token := strings.TrimSpace(req.Integrations.ChmlFrp.Password); token != "" {
		account, err := h.saveChmlFrpCredential(c, token)
		if err != nil {
			c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "CHMLFRP_CREDENTIAL_INVALID", Message: "ChmlFrp Token verification failed"}})
			return
		}
		integrations.ChmlFrp.Username = account.Username
	}
	if vj, err := json.Marshal(integrations); err == nil {
		_ = h.repo.SetSetting("integrations", string(vj))
	}

	h.upsertCredential(c, "onepanel", req.Integrations.OnePanel.BaseURL, req.Integrations.OnePanel.APIToken)
	h.verifyIntegrations(c, req.Integrations)

	h.auditSettingsUpdate(c)
	settings, _ := h.repo.GetAllSettings()
	c.Header("Cache-Control", "no-store")
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: withoutProviderSecrets(h.settingsMapToView(settings))})
}

func (h *SettingsHandler) auditSettingsUpdate(c *gin.Context) {
	_ = h.repo.CreateAuditLog(&domain.AuditLog{ID: domain.NewID("aud"), AccountID: c.GetString("account_id"), AccountName: c.GetString("account_name"), Action: "settings.update", ResourceType: "settings", ResourceID: "global", RequestID: c.GetString("request_id"), TraceID: c.GetString("trace_id"), Outcome: "success", IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")})
}

func (h *SettingsHandler) verifyIntegrations(c *gin.Context, integrations domain.IntegrationSettings) {
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
}

func (h *SettingsHandler) saveChmlFrpCredential(c *gin.Context, token string) (*domain.ChmlFrpUserInfo, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("ChmlFrp Token is required")
	}
	account, err := h.chmlfrpClientFactory(token).GetCurrentUser()
	if err != nil {
		return nil, err
	}
	encrypted, err := security.Encrypt([]byte(token), h.key)
	if err != nil {
		return nil, fmt.Errorf("encrypt ChmlFrp Token: %w", err)
	}
	cred, err := h.repo.FindCredentialByProvider("chmlfrp")
	if err != nil || cred == nil {
		cred = &domain.UpstreamCredential{ID: domain.NewID("cre"), Provider: "chmlfrp"}
	}
	now := time.Now()
	credentialRef := observability.CredentialRef(token, h.key)
	if cred.CredentialRef != credentialRef {
		cred.Revision++
	}
	cred.Identifier = account.Username
	cred.AuthMethod = "api_token"
	cred.EncryptedSecret = encrypted
	cred.MaskHint = observability.TokenMask(token)
	cred.CredentialRef = credentialRef
	cred.LastVerifiedAt = &now
	cred.LastError = ""
	cred.UpdatedAt = now
	if err := h.repo.UpsertCredential(cred); err != nil {
		return nil, fmt.Errorf("save ChmlFrp Token: %w", err)
	}
	detail, _ := json.Marshal(map[string]any{
		"provider": "chmlfrp", "account": account.Username,
		"credential_ref": cred.CredentialRef, "credential_revision": cred.Revision,
	})
	_ = h.repo.CreateAuditLog(&domain.AuditLog{ID: domain.NewID("aud"), AccountID: c.GetString("account_id"), AccountName: c.GetString("account_name"), Action: "chmlfrp.credential.save", ResourceType: "credential", ResourceID: cred.ID, DetailJSON: string(detail), RequestID: c.GetString("request_id"), TraceID: c.GetString("trace_id"), Outcome: "success", CredentialRef: cred.CredentialRef, IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")})
	slog.Info("credential.saved", "component", "settings", "event", "credential.save", "provider", "chmlfrp", "account", account.Username, "credential_ref", cred.CredentialRef, "credential_revision", cred.Revision, "token_mask", cred.MaskHint, "request_id", c.GetString("request_id"), "account_id", c.GetString("account_id"))
	return account, nil
}

func (h *SettingsHandler) StartChmlFrpOAuth(c *gin.Context) {
	mgr := chmlfrp.NewOAuth2Manager()
	authResp, err := mgr.StartDeviceAuthorization("", "")
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "OAUTH_START_FAILED", Message: err.Error()}})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: authResp})
}

func (h *SettingsHandler) PollChmlFrpOAuth(c *gin.Context) {
	var req struct {
		DeviceCode string `json:"device_code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.DeviceCode == "" {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "INVALID_REQUEST", Message: "device_code is required"}})
		return
	}
	mgr := chmlfrp.NewOAuth2Manager()
	tokenResp, err := mgr.PollToken("", "", req.DeviceCode)
	if err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "OAUTH_POLL_FAILED", Message: err.Error()}})
		return
	}
	if tokenResp.Error != "" {
		c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]any{"status": "pending", "error": tokenResp.Error, "description": tokenResp.ErrorDesc}})
		return
	}
	if tokenResp.AccessToken != "" {
		// The browser immediately submits this access token to the authenticated
		// settings PATCH, where saveChmlFrpCredential verifies /userinfo and stores
		// the real upstream account identity with the encrypted token.
		c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]any{"status": "success", "token": tokenResp}})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]any{"status": "pending"}})
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
	view := withoutProviderSecrets(h.settingsMapToView(settings))
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
		client = h.storedCloudflareClient(string(secret), cred, cred.Identifier)
		err = run("cloudflare.credential.verify", client.VerifyToken)
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
			if client != nil {
				if zones, zErr := client.ListZones(); zErr == nil {
					for _, z := range zones {
						if z.ID == cred.Identifier || z.Name == cred.Identifier {
							cred.Identifier = z.Name
							settings, _ := h.repo.GetAllSettings()
							for _, s := range settings {
								if s.Key == "integrations" {
									var integ domain.IntegrationSettings
									if json.Unmarshal([]byte(s.ValueJSON), &integ) == nil {
										integ.Cloudflare.ZoneName = z.Name
										if vj, err := json.Marshal(integ); err == nil {
											_ = h.repo.SetSetting("integrations", string(vj))
										}
									}
								}
							}
							break
						}
					}
				}
			}
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
			// Credential identity is the verified upstream account. Historical records
			// used sentinel values (or even the token itself) as the identifier; never
			// display those as a fake current account.
			account := cred.Identifier
			view.Integrations.ChmlFrp.HasPassword = cred.EncryptedSecret != ""
			if cred.EncryptedSecret != "" {
				if secret, err := security.Decrypt(cred.EncryptedSecret, h.key); err == nil {
					view.Integrations.ChmlFrp.Password = string(secret)
					if account == "oauth2_user" || account == "token" || account == string(secret) {
						account = ""
					}
				}
			}
			view.Integrations.ChmlFrp.Username = account
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
						cli := h.storedCloudflareClient(string(sec), &cred, "")
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
			if identifier != "" {
				view.Integrations.Cloudflare.ZoneName = identifier
				settings, _ := h.repo.GetAllSettings()
				for _, s := range settings {
					if s.Key == "integrations" {
						var integ domain.IntegrationSettings
						if json.Unmarshal([]byte(s.ValueJSON), &integ) == nil {
							if integ.Cloudflare.ZoneName != identifier {
								integ.Cloudflare.ZoneName = identifier
								if vj, err := json.Marshal(integ); err == nil {
									_ = h.repo.SetSetting("integrations", string(vj))
								}
							}
						}
					}
				}
			}
			view.Integrations.Cloudflare.HasAPIToken = cred.EncryptedSecret != ""
			view.Integrations.Cloudflare.AuthMethod = cred.AuthMethod
			if view.Integrations.Cloudflare.AuthMethod == "" {
				view.Integrations.Cloudflare.AuthMethod = cloudflare.AuthMethodAPIToken
			}
			view.Integrations.Cloudflare.AccountEmail = cred.AccountEmail
			view.Integrations.Cloudflare.ZoneID = cred.ZoneID
			// Personal self-hosted product decision: authenticated settings view returns the full secret.
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

func withoutProviderSecrets(view domain.SettingsPatchRequest) domain.SettingsPatchRequest {
	view.Integrations.ChmlFrp.Password = ""
	view.Integrations.OnePanel.APIToken = ""
	view.Integrations.Cloudflare.APIToken = ""
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
