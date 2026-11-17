package chmlfrp

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestParseToken(t *testing.T) {
	// 1. Plain token
	raw := []byte("eyJhbGciOi...")
	token, oauthData, isOAuth := ParseToken(raw)
	require.False(t, isOAuth)
	require.Nil(t, oauthData)
	require.Equal(t, "eyJhbGciOi...", token)

	// 2. OAuth structured data
	data := OAuthTokenData{
		AccessToken:  "at_123456",
		RefreshToken: "rt_654321",
		ExpiresIn:    600,
		ExpiresAt:    time.Now().Add(10 * time.Minute),
	}
	encoded, _ := json.Marshal(data)
	token2, oauthData2, isOAuth2 := ParseToken(encoded)
	require.True(t, isOAuth2)
	require.NotNil(t, oauthData2)
	require.Equal(t, "at_123456", token2)
	require.Equal(t, "rt_654321", oauthData2.RefreshToken)
}

func TestEnsureValidToken_Refresh(t *testing.T) {
	// Mock HTTP client for token endpoint
	mgr := NewOAuth2Manager()
	mgr.http = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			require.Equal(t, "/oauth2/token", req.URL.Path)
			respJSON := `{"access_token":"new_at_999","refresh_token":"new_rt_888","token_type":"bearer","expires_in":600}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader(respJSON)),
				Header:     make(http.Header),
			}, nil
		}),
	}

	// Token expired 1 minute ago
	expiredData := OAuthTokenData{
		AccessToken:  "old_at",
		RefreshToken: "old_rt",
		ExpiresIn:    600,
		ExpiresAt:    time.Now().Add(-1 * time.Minute),
	}
	raw, _ := json.Marshal(expiredData)

	activeToken, updatedPayload, refreshed, err := mgr.EnsureValidToken(raw)
	require.NoError(t, err)
	require.True(t, refreshed)
	require.Equal(t, "new_at_999", activeToken)
	require.NotEmpty(t, updatedPayload)

	// Verify updated payload
	var updatedData OAuthTokenData
	err = json.Unmarshal(updatedPayload, &updatedData)
	require.NoError(t, err)
	require.Equal(t, "new_at_999", updatedData.AccessToken)
	require.Equal(t, "new_rt_888", updatedData.RefreshToken)
}

func TestNewClient_WithOAuthJSON(t *testing.T) {
	data := OAuthTokenData{
		AccessToken:  "token_from_json",
		RefreshToken: "refresh_from_json",
	}
	encoded, _ := json.Marshal(data)
	c := NewClient("oauth2_user", string(encoded))
	require.Equal(t, "token_from_json", c.token)
}
