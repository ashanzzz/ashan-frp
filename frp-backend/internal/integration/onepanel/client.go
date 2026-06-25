package onepanel

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"ashan-frp/internal/domain"
)

type Client struct {
	baseURL  string
	apiToken string
	http     *http.Client
}

func NewClient(baseURL, apiToken string) *Client {
	if baseURL == "" { baseURL = "http://localhost:8080" }
	return &Client{baseURL: baseURL, apiToken: apiToken, http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) headers() map[string]string { return map[string]string{"Content-Type": "application/json", "1Panel-Token": c.apiToken} }

func (c *Client) CreateWebsite(domainName, proxyTarget string, enableSSL bool) (int, error) {
	payload := domain.OnePanelCreateWebsiteReq{PrimaryDomain: domainName, Type: "proxy", Proxy: proxyTarget, Domains: domainName}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", c.baseURL+"/api/v1/websites", bytes.NewReader(body))
	for k, v := range c.headers() { req.Header.Set(k, v) }
	resp, err := c.http.Do(req)
	if err != nil { return 0, err }
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var result domain.OnePanelResponse
	json.Unmarshal(respBody, &result)
	if result.Code != 200 { return 0, fmt.Errorf("1panel: %s (code=%d)", result.Message, result.Code) }
	var website domain.OnePanelWebsite
	json.Unmarshal(result.Data, &website)
	if enableSSL && website.ID > 0 { c.EnableSSL(website.ID, domainName) }
	return website.ID, nil
}

func (c *Client) EnableSSL(websiteID int, domainName string) error {
	payload := map[string]any{"websiteId": websiteID, "type": "http", "provider": "letsencrypt", "domains": []string{domainName}, "autoRenew": true}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", fmt.Sprintf("%s/api/v1/websites/ssl", c.baseURL), bytes.NewReader(body))
	for k, v := range c.headers() { req.Header.Set(k, v) }
	resp, _ := c.http.Do(req)
	resp.Body.Close()
	return nil
}

func (c *Client) DeleteWebsite(websiteID int) error {
	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/api/v1/websites/%d", c.baseURL, websiteID), nil)
	for k, v := range c.headers() { req.Header.Set(k, v) }
	resp, _ := c.http.Do(req)
	resp.Body.Close()
	return nil
}

func (c *Client) ListWebsites() ([]domain.OnePanelWebsite, error) {
	req, _ := http.NewRequest("GET", c.baseURL+"/api/v1/websites", nil)
	for k, v := range c.headers() { req.Header.Set(k, v) }
	resp, _ := c.http.Do(req)
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var result domain.OnePanelResponse
	json.Unmarshal(respBody, &result)
	var websites []domain.OnePanelWebsite
	json.Unmarshal(result.Data, &websites)
	return websites, nil
}

func (c *Client) TestConnection() error {
	req, _ := http.NewRequest("GET", c.baseURL+"/api/v1/dashboard/panel", nil)
	for k, v := range c.headers() { req.Header.Set(k, v) }
	resp, err := c.http.Do(req)
	if err != nil { return err }
	resp.Body.Close()
	return nil
}
