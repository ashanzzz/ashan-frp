package onepanel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"ashan-frp/internal/domain"
)

type Client struct {
	baseURL  string
	apiToken string
	http     *http.Client
}

func NewClient(baseURL, apiToken string) *Client {
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	return &Client{baseURL: baseURL, apiToken: apiToken, http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) headers() map[string]string {
	return map[string]string{"Content-Type": "application/json", "1Panel-Token": c.apiToken}
}

func readBody(resp *http.Response, op string) ([]byte, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s read: %w", op, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("%s failed: http %d: %s", op, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func validateResponse(body []byte, op string) error {
	if len(strings.TrimSpace(string(body))) == 0 {
		return nil
	}
	var result domain.OnePanelResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("%s parse: %w", op, err)
	}
	if result.Code != http.StatusOK {
		return fmt.Errorf("%s failed: code=%d: %s", op, result.Code, firstNonEmpty(result.Message, result.Msg, string(body)))
	}
	return nil
}

func (c *Client) CreateWebsite(domainName, proxyTarget string, enableSSL bool) (int, error) {
	payload := domain.OnePanelCreateWebsiteReq{PrimaryDomain: domainName, Type: "proxy", Proxy: proxyTarget, Domains: []string{domainName}}
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest("POST", c.baseURL+"/api/v1/websites", bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	for k, v := range c.headers() {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("1panel create website: %w", err)
	}
	respBody, err := readBody(resp, "1panel create website")
	if err != nil {
		return 0, err
	}
	var result domain.OnePanelResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return 0, fmt.Errorf("1panel create website parse: %w", err)
	}
	if err := validateResponse(respBody, "1panel create website"); err != nil {
		return 0, err
	}
	var website domain.OnePanelWebsite
	if err := json.Unmarshal(result.Data, &website); err != nil {
		return 0, fmt.Errorf("1panel create website result parse: %w", err)
	}
	if enableSSL && website.ID > 0 {
		if err := c.EnableSSL(website.ID, domainName); err != nil {
			return 0, err
		}
	}
	return website.ID, nil
}

func (c *Client) EnableSSL(websiteID int, domainName string) error {
	payload := map[string]any{"websiteId": websiteID, "type": "http", "provider": "letsencrypt", "domains": []string{domainName}, "autoRenew": true}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/websites/ssl", c.baseURL), bytes.NewReader(body))
	if err != nil {
		return err
	}
	for k, v := range c.headers() {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("1panel enable ssl: %w", err)
	}
	respBody, err := readBody(resp, "1panel enable ssl")
	if err != nil {
		return err
	}
	return validateResponse(respBody, "1panel enable ssl")
}

func (c *Client) DeleteWebsite(websiteID int) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/websites/%d", c.baseURL, websiteID), nil)
	if err != nil {
		return err
	}
	for k, v := range c.headers() {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("1panel delete website: %w", err)
	}
	respBody, err := readBody(resp, "1panel delete website")
	if err != nil {
		return err
	}
	return validateResponse(respBody, "1panel delete website")
}

func (c *Client) ListWebsites() ([]domain.OnePanelWebsite, error) {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/websites", nil)
	if err != nil {
		return nil, err
	}
	for k, v := range c.headers() {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("1panel list websites: %w", err)
	}
	respBody, err := readBody(resp, "1panel list websites")
	if err != nil {
		return nil, err
	}
	var result domain.OnePanelResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("1panel list websites parse: %w", err)
	}
	if err := validateResponse(respBody, "1panel list websites"); err != nil {
		return nil, err
	}
	var websites []domain.OnePanelWebsite
	if err := json.Unmarshal(result.Data, &websites); err != nil {
		return nil, fmt.Errorf("1panel list websites result parse: %w", err)
	}
	return websites, nil
}

func (c *Client) TestConnection() error {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v1/dashboard/panel", nil)
	if err != nil {
		return err
	}
	for k, v := range c.headers() {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("1panel test connection: %w", err)
	}
	respBody, err := readBody(resp, "1panel test connection")
	if err != nil {
		return err
	}
	return validateResponse(respBody, "1panel test connection")
}
