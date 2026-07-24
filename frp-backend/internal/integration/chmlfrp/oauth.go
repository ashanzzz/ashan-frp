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
