package handlers

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"ashan-frp/internal/domain"
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
}

func (f *fakeCloudflareVerifier) VerifyToken() error                    { return f.tokenErr }
func (f *fakeCloudflareVerifier) ResolveZone() error                    { return f.zoneErr }
func (f *fakeCloudflareVerifier) ListRecords() ([]domain.CFDNSRecord, error) { return []domain.CFDNSRecord{}, f.listErr }
func (f *fakeCloudflareVerifier) ListZones() ([]domain.CFZone, error) { return []domain.CFZone{}, f.listErr }

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
}
