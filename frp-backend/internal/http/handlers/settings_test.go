package handlers

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
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
