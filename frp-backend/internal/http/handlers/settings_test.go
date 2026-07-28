package handlers

import (
	"bytes"
	"errors"
	"log/slog"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/integration/cloudflare"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

func TestSettingsHandler_VerifyCloudflare_reportsUnconfiguredWithoutSecrets(t *testing.T) {
	db := setupHandlerDB(t)
	h := NewSettingsHandler(repository.New(db), []byte("settings-test-key"))
	ctx, recorder := newGinContext(http.MethodPost, "/api/v1/settings/integrations/cloudflare/verify", nil)

	h.VerifyCloudflare(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var envelope domain.ResponseEnvelope
	jsonDecodeResp(t, recorder, &envelope)
	payload := envelope.Data.(map[string]any)
	require.Equal(t, false, payload["valid"])
	require.Contains(t, payload["message"], "not configured")
	require.NotContains(t, recorder.Body.String(), "EncryptedSecret")
}

func TestDNSHandler_List_requiresCloudflareConfiguration(t *testing.T) {
	db := setupHandlerDB(t)
	h := NewDNSHandler(repository.New(db), []byte("dns-test-key"))
	ctx, recorder := newGinContext(http.MethodGet, "/api/v1/dns/records", nil)
	h.List(ctx)
	require.Equal(t, http.StatusPreconditionFailed, recorder.Code)
	var envelope domain.ResponseEnvelope
	jsonDecodeResp(t, recorder, &envelope)
	require.NotNil(t, envelope.Error)
	require.Contains(t, envelope.Error.Message, "configured")
}

type fakeCloudflareVerifier struct {
	tokenErr error
	zoneErr  error
	listErr  error
	zones    []domain.CFZone
}

func (f *fakeCloudflareVerifier) VerifyToken() error { return f.tokenErr }
func (f *fakeCloudflareVerifier) ResolveZone() error { return f.zoneErr }
func (f *fakeCloudflareVerifier) ListRecords() ([]domain.CFDNSRecord, error) {
	return []domain.CFDNSRecord{}, f.listErr
}
func (f *fakeCloudflareVerifier) ListZones() ([]domain.CFZone, error) {
	return f.zones, f.listErr
}

func TestSettingsHandler_VerifyCloudflare_withConfiguredSecret_success(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	key := []byte("0123456789abcdef0123456789abcdef")
	enc, err := security.Encrypt([]byte("test-token"), key)
	require.NoError(t, err)
	require.NoError(t, repo.UpsertCredential(&domain.UpstreamCredential{
		ID:              "cre_cf",
		Provider:        "cloudflare",
		Identifier:      "example.com",
		EncryptedSecret: enc,
		MaskHint:        "test****",
		CredentialRef:   "ref_123",
	}))

	h := NewSettingsHandler(repo, key)
	fake := &fakeCloudflareVerifier{}
	h.clientFactory = func(token, zone string) cloudflareVerifier {
		require.Equal(t, "test-token", token)
		require.Equal(t, "example.com", zone)
		return fake
	}

	ctx, recorder := newGinContext(http.MethodPost, "/api/v1/settings/integrations/cloudflare/verify", nil)
	h.VerifyCloudflare(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var envelope domain.ResponseEnvelope
	jsonDecodeResp(t, recorder, &envelope)
	payload := envelope.Data.(map[string]any)
	require.Equal(t, true, payload["valid"])
	require.Contains(t, payload["message"], "verified")
	require.NotContains(t, recorder.Body.String(), "test-token")
}

func TestSettingsHandler_ConfigureCloudflare_singleZoneVerifiesAndSavesPlaintext(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	key := []byte("0123456789abcdef0123456789abcdef")
	h := NewSettingsHandler(repo, key)
	h.credentialClientFactory = func(credentials cloudflare.Credentials, zone string) cloudflareVerifier {
		require.Equal(t, cloudflare.AuthMethodAPIToken, credentials.AuthMethod)
		require.Equal(t, "cfut_test-token", credentials.Secret)
		if zone == "" {
			return &fakeCloudflareVerifier{zones: []domain.CFZone{{ID: "zone-1", Name: "example.com"}}}
		}
		require.Equal(t, "zone-1", zone)
		return &fakeCloudflareVerifier{}
	}

	ctx, recorder := newGinContext(http.MethodPost, "/api/v1/settings/integrations/cloudflare/configure", map[string]any{
		"secret": "cfut_test-token",
	})
	h.ConfigureCloudflare(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	var envelope domain.ResponseEnvelope
	jsonDecodeResp(t, recorder, &envelope)
	payload := envelope.Data.(map[string]any)
	require.Equal(t, "saved", payload["status"])
	require.Equal(t, "cfut_test-token", payload["secret"])
	require.Equal(t, "example.com", payload["zone_name"])

	credential, err := repo.FindCredentialByProvider("cloudflare")
	require.NoError(t, err)
	require.Equal(t, cloudflare.AuthMethodAPIToken, credential.AuthMethod)
	require.Equal(t, "zone-1", credential.ZoneID)
	decrypted, err := security.Decrypt(credential.EncryptedSecret, key)
	require.NoError(t, err)
	require.Equal(t, "cfut_test-token", string(decrypted))
	require.NotContains(t, credential.EncryptedSecret, "cfut_test-token")
	integrationSetting, err := repo.GetSetting("integrations")
	require.NoError(t, err)
	require.NotContains(t, integrationSetting.ValueJSON, "cfut_test-token")
}

func TestSettingsHandler_ConfigureCloudflareLogsOnlySafeCredentialIdentity(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	key := []byte("0123456789abcdef0123456789abcdef")
	h := NewSettingsHandler(repo, key)
	h.credentialClientFactory = func(credentials cloudflare.Credentials, zone string) cloudflareVerifier {
		if zone == "" {
			return &fakeCloudflareVerifier{zones: []domain.CFZone{{ID: "zone-1", Name: "example.com"}}}
		}
		return &fakeCloudflareVerifier{}
	}

	var logs bytes.Buffer
	previousLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() { slog.SetDefault(previousLogger) })

	ctx, recorder := newGinContext(http.MethodPost, "/api/v1/settings/integrations/cloudflare/configure", map[string]any{
		"secret": "cfut_log_safe_test_token",
	})
	h.ConfigureCloudflare(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, logs.String(), "credential.saved")
	require.Contains(t, logs.String(), "credential_ref")
	require.NotContains(t, logs.String(), "cfut_log_safe_test_token")
}

func TestSettingsHandler_ConfigureCloudflare_multipleZonesDoesNotSaveBeforeSelection(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewSettingsHandler(repo, []byte("0123456789abcdef0123456789abcdef"))
	h.credentialClientFactory = func(credentials cloudflare.Credentials, zone string) cloudflareVerifier {
		require.Equal(t, cloudflare.AuthMethodGlobalAPIKey, credentials.AuthMethod)
		require.Equal(t, "owner@example.com", credentials.Email)
		require.Empty(t, zone)
		return &fakeCloudflareVerifier{zones: []domain.CFZone{
			{ID: "zone-a", Name: "a.example"},
			{ID: "zone-b", Name: "b.example"},
		}}
	}

	ctx, recorder := newGinContext(http.MethodPost, "/api/v1/settings/integrations/cloudflare/configure", map[string]any{
		"secret": "cfk_global-key",
		"email":  "owner@example.com",
	})
	h.ConfigureCloudflare(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var envelope domain.ResponseEnvelope
	jsonDecodeResp(t, recorder, &envelope)
	payload := envelope.Data.(map[string]any)
	require.Equal(t, "zone_selection_required", payload["status"])
	_, err := repo.FindCredentialByProvider("cloudflare")
	require.Error(t, err)
}

func TestSettingsHandler_ConfigureCloudflare_selectedZoneVerifiesAndSaves(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	key := []byte("0123456789abcdef0123456789abcdef")
	h := NewSettingsHandler(repo, key)
	h.credentialClientFactory = func(credentials cloudflare.Credentials, zone string) cloudflareVerifier {
		require.Equal(t, cloudflare.AuthMethodGlobalAPIKey, credentials.AuthMethod)
		require.Equal(t, "owner@example.com", credentials.Email)
		if zone == "" {
			return &fakeCloudflareVerifier{zones: []domain.CFZone{
				{ID: "zone-a", Name: "a.example"},
				{ID: "zone-b", Name: "b.example"},
			}}
		}
		require.Equal(t, "zone-b", zone)
		return &fakeCloudflareVerifier{}
	}

	ctx, recorder := newGinContext(http.MethodPost, "/api/v1/settings/integrations/cloudflare/configure", map[string]any{
		"secret":  "cfk_global-key",
		"email":   "owner@example.com",
		"zone_id": "zone-b",
	})
	h.ConfigureCloudflare(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	credential, err := repo.FindCredentialByProvider("cloudflare")
	require.NoError(t, err)
	require.Equal(t, "zone-b", credential.ZoneID)
	require.Equal(t, "b.example", credential.Identifier)
	require.Equal(t, cloudflare.AuthMethodGlobalAPIKey, credential.AuthMethod)
}

func TestSettingsHandler_ConfigureCloudflare_dnsReadFailureDoesNotSave(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewSettingsHandler(repo, []byte("0123456789abcdef0123456789abcdef"))
	h.credentialClientFactory = func(credentials cloudflare.Credentials, zone string) cloudflareVerifier {
		if zone == "" {
			return &fakeCloudflareVerifier{zones: []domain.CFZone{{ID: "zone-1", Name: "example.com"}}}
		}
		return &fakeCloudflareVerifier{listErr: errors.New("forbidden")}
	}

	ctx, recorder := newGinContext(http.MethodPost, "/api/v1/settings/integrations/cloudflare/configure", map[string]any{
		"secret": "cfut_test-token",
	})
	h.ConfigureCloudflare(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	_, err := repo.FindCredentialByProvider("cloudflare")
	require.Error(t, err)
	_, err = repo.GetSetting("integrations")
	require.Error(t, err)
}

func TestSettingsHandler_GetReturnsFullPlaintextProviderSecrets(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	key := []byte("0123456789abcdef0123456789abcdef")
	for _, item := range []struct {
		provider   string
		identifier string
		secret     string
	}{
		{provider: "cloudflare", identifier: "example.com", secret: "full-cloudflare-secret"},
		{provider: "chmlfrp", identifier: "oauth2_user", secret: "full-chmlfrp-secret"},
	} {
		encrypted, err := security.Encrypt([]byte(item.secret), key)
		require.NoError(t, err)
		require.NoError(t, repo.UpsertCredential(&domain.UpstreamCredential{
			ID: domain.NewID("cre"), Provider: item.provider, Identifier: item.identifier, EncryptedSecret: encrypted,
		}))
	}

	h := NewSettingsHandler(repo, key)
	ctx, recorder := newGinContext(http.MethodGet, "/api/v1/settings", nil)
	h.Get(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	require.Contains(t, recorder.Body.String(), `"api_token":"full-cloudflare-secret"`)
	require.Contains(t, recorder.Body.String(), `"password":"full-chmlfrp-secret"`)
}

func TestSettingsHandler_UpdatePreservesCloudflareConfigurationAndDisablesLegacyWrites(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	key := []byte("0123456789abcdef0123456789abcdef")
	encrypted, err := security.Encrypt([]byte("existing-secret"), key)
	require.NoError(t, err)
	require.NoError(t, repo.UpsertCredential(&domain.UpstreamCredential{
		ID: "cre_cf", Provider: "cloudflare", Identifier: "example.com", AuthMethod: cloudflare.AuthMethodAPIToken,
		ZoneID: "zone-existing", EncryptedSecret: encrypted,
	}))
	require.NoError(t, repo.SetSetting("integrations", `{"cloudflare":{"auth_method":"api_token","zone_id":"zone-existing","zone_name":"example.com"},"chmlfrp":{},"onepanel":{}}`))

	h := NewSettingsHandler(repo, key)
	h.clientFactory = func(token, zone string) cloudflareVerifier {
		t.Fatal("generic settings PATCH must not probe or rewrite Cloudflare credentials")
		return nil
	}
	h.credentialClientFactory = func(credentials cloudflare.Credentials, zone string) cloudflareVerifier {
		t.Fatal("generic settings PATCH must not probe or rewrite Cloudflare credentials")
		return nil
	}
	ctx, recorder := newGinContext(http.MethodPatch, "/api/v1/settings", map[string]any{
		"general": map[string]any{}, "sync": map[string]any{}, "queue": map[string]any{}, "frpc_runtime": map[string]any{},
		"integrations": map[string]any{
			"chmlfrp": map[string]any{}, "onepanel": map[string]any{},
			"cloudflare": map[string]any{
				"auth_method": "global_api_key", "account_email": "attacker@example.com",
				"zone_id": "zone-replaced", "zone_name": "replaced.example",
			},
		},
	})
	h.Update(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
	require.NotContains(t, recorder.Body.String(), "existing-secret")
	credential, err := repo.FindCredentialByProvider("cloudflare")
	require.NoError(t, err)
	require.Equal(t, cloudflare.AuthMethodAPIToken, credential.AuthMethod)
	require.Equal(t, "zone-existing", credential.ZoneID)
	require.Equal(t, "example.com", credential.Identifier)
	setting, err := repo.GetSetting("integrations")
	require.NoError(t, err)
	require.Contains(t, setting.ValueJSON, `"zone_id":"zone-existing"`)
	require.NotContains(t, setting.ValueJSON, "attacker@example.com")
	require.NotContains(t, setting.ValueJSON, "zone-replaced")
}
