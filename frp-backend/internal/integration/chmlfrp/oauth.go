package chmlfrp

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"ashan-frp/internal/domain"
)

const (
	QZhuaAPIBase             = "https://account-api.qzhua.net"
	QZhuaDeviceCodeEndpoint  = QZhuaAPIBase + "/oauth2/device_authorization"
	QZhuaTokenEndpoint       = QZhuaAPIBase + "/oauth2/token"
	QZhuaScope               = "chmlfrp_api"
	DefaultClientID          = "019d534218e67f8a862056c1efb869db"
	DefaultClientSecret      = "0a98ee0b7c69daa4c4922bae9be5df95eff6"
)

type OAuth2Manager struct {
	http *http.Client
}

func NewOAuth2Manager() *OAuth2Manager {
	return &OAuth2Manager{http: &http.Client{Timeout: 30 * time.Second}}
}

func (m *OAuth2Manager) StartDeviceAuthorization(clientID, clientSecret string) (*domain.ChmlFrpDeviceAuthResp, error) {
	if clientID == "" {
		clientID = DefaultClientID
	}
	if clientSecret == "" {
		clientSecret = DefaultClientSecret
	}
	form := url.Values{
		"client_id":     {clientID},
		"client_secret": {clientSecret},
		"scope":         {QZhuaScope},
	}
	req, err := http.NewRequest("POST", QZhuaDeviceCodeEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := m.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chmlfrp device auth http request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("chmlfrp device auth read body: %w", err)
	}

	var result domain.ChmlFrpDeviceAuthResp
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("chmlfrp device auth parse: %w", err)
	}
	if result.DeviceCode == "" {
		return nil, fmt.Errorf("chmlfrp device auth failed: %s", string(body))
	}
	return &result, nil
}

func (m *OAuth2Manager) PollToken(clientID, clientSecret, deviceCode string) (*domain.ChmlFrpOAuthTokenResp, error) {
	if clientID == "" {
		clientID = DefaultClientID
	}
	if clientSecret == "" {
		clientSecret = DefaultClientSecret
	}
	form := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"device_code": {deviceCode},
		"client_id":   {clientID},
	}
	req, err := http.NewRequest("POST", QZhuaTokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := m.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chmlfrp poll token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("chmlfrp poll token read body: %w", err)
	}

	var result domain.ChmlFrpOAuthTokenResp
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("chmlfrp poll token parse: %w", err)
	}
	return &result, nil
}

func (m *OAuth2Manager) RefreshAccessToken(clientID, clientSecret, refreshToken string) (*domain.ChmlFrpOAuthTokenResp, error) {
	if clientID == "" {
		clientID = DefaultClientID
	}
	if clientSecret == "" {
		clientSecret = DefaultClientSecret
	}
	form := url.Values{
		"grant_type":    {"refresh_token"},
		"refresh_token": {refreshToken},
		"client_id":     {clientID},
	}
	req, err := http.NewRequest("POST", QZhuaTokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetBasicAuth(clientID, clientSecret)

	resp, err := m.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("chmlfrp refresh token request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("chmlfrp refresh token read body: %w", err)
	}

	var result domain.ChmlFrpOAuthTokenResp
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("chmlfrp refresh token parse: %w", err)
	}
	if result.AccessToken == "" {
		return nil, fmt.Errorf("chmlfrp refresh token failed: %s", string(body))
	}
	return &result, nil
}

type OAuthTokenData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	UserToken    string    `json:"user_token,omitempty"`
	TokenType    string    `json:"token_type,omitempty"`
	ExpiresIn    int       `json:"expires_in,omitempty"`
	ExpiresAt    time.Time `json:"expires_at"`
	Scope        string    `json:"scope,omitempty"`
}

// ParseToken extracts the access token and optional OAuthTokenData from decrypted bytes.
func ParseToken(decrypted []byte) (token string, oauthData *OAuthTokenData, isOAuth bool) {
	var data OAuthTokenData
	if err := json.Unmarshal(decrypted, &data); err == nil && data.AccessToken != "" {
		return data.AccessToken, &data, true
	}
	// Fallback check: ChmlFrpCredentials struct with Password
	var creds domain.ChmlFrpCredentials
	if err := json.Unmarshal(decrypted, &creds); err == nil && creds.Password != "" {
		return creds.Password, nil, false
	}
	return strings.TrimSpace(string(decrypted)), nil, false
}

// EnsureValidToken checks whether the token is an OAuth token needing refresh.
// If near expiry (within 2 minutes) or already expired, it automatically calls RefreshAccessToken.
// Returns:
//   - activeToken: valid access_token to use immediately
//   - updatedPayload: new JSON byte slice to re-encrypt into storage (nil if no refresh occurred)
//   - refreshed: true if a refresh network call succeeded
//   - err: error if refresh failed
func (m *OAuth2Manager) EnsureValidToken(decrypted []byte) (string, []byte, bool, error) {
	token, data, isOAuth := ParseToken(decrypted)
	if !isOAuth || data == nil || data.RefreshToken == "" {
		return token, nil, false, nil
	}

	// If token expires within 2 minutes or has already expired
	if time.Now().Add(2 * time.Minute).After(data.ExpiresAt) {
		newResp, err := m.RefreshAccessToken("", "", data.RefreshToken)
		if err != nil {
			return token, nil, false, fmt.Errorf("failed to refresh oauth token: %w", err)
		}
		data.AccessToken = newResp.AccessToken
		if newResp.RefreshToken != "" {
			data.RefreshToken = newResp.RefreshToken
		}
		if newResp.ExpiresIn > 0 {
			data.ExpiresIn = newResp.ExpiresIn
			data.ExpiresAt = time.Now().Add(time.Duration(newResp.ExpiresIn) * time.Second)
		}
		updatedPayload, _ := json.Marshal(data)
		return data.AccessToken, updatedPayload, true, nil
	}

	return data.AccessToken, nil, false, nil
}
