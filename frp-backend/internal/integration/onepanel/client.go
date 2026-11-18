package onepanel

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
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
		baseURL = "http://192.168.8.11:10086"
	}
	baseURL = strings.TrimRight(baseURL, "/")
	return &Client{baseURL: baseURL, apiToken: apiToken, http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) headers() map[string]string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	hash := md5.Sum([]byte(fmt.Sprintf("1panel%s%s", c.apiToken, ts)))
	token := hex.EncodeToString(hash[:])

	return map[string]string{
		"Content-Type":     "application/json",
		"Accept":           "application/json, text/plain, */*",
		"1Panel-Token":     token,
		"1Panel-Timestamp": ts,
	}
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

// CreateProxyWebsite creates a reverse proxy website in 1Panel v2 with a custom port and optional SSL.
func (c *Client) CreateProxyWebsite(alias, proxyTarget string, listenPort int, enableSSL bool, sslID int) (int, error) {
	if listenPort <= 0 {
		listenPort = 443
	}
	payload := domain.OnePanelCreateWebsiteReq{
		PrimaryDomain:  alias,
		Alias:          alias,
		Type:           "proxy",
		AppType:        "installed",
		WebSiteGroupID: 1,
		Proxy:          proxyTarget,
		ProxyType:      "",
		Domains: []domain.OnePanelWebsiteDomain{
			{
				Domain: "192.168.8.11",
				Port:   listenPort,
			},
		},
		EnableSSL:    enableSSL,
		WebsiteSSLID: sslID,
		Remark:       fmt.Sprintf("%d %s", listenPort, alias),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	req, err := http.NewRequest("POST", c.baseURL+"/api/v2/websites", bytes.NewReader(body))
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

	// 1Panel v2 may return website object or ID
	var website domain.OnePanelWebsite
	if len(result.Data) > 0 {
		if err := json.Unmarshal(result.Data, &website); err == nil && website.ID > 0 {
			return website.ID, nil
		}
	}

	// Lookup website ID by alias if not directly returned in Data
	websites, err := c.ListWebsites()
	if err == nil {
		for _, w := range websites {
			if w.Alias == alias || w.PrimaryDomain == alias {
				return w.ID, nil
			}
		}
	}
	return 0, nil
}

// CreateWebsite provides backwards-compatibility for existing callers.
func (c *Client) CreateWebsite(domainName, proxyTarget string, enableSSL bool) (int, error) {
	return c.CreateProxyWebsite(domainName, proxyTarget, 443, enableSSL, 0)
}

// EnableSSL applies or binds an existing SSL certificate to a website.
func (c *Client) EnableSSL(websiteID int, domainName string) error {
	payload := map[string]any{
		"websiteId": websiteID,
		"type":      "http",
		"provider":  "letsencrypt",
		"domains":   []string{domainName},
		"autoRenew": true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", c.baseURL+"/api/v2/websites/ssl", bytes.NewReader(body))
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

// ApplySSLCertificate applies for a new SSL certificate using Cloudflare DNS verification.
func (c *Client) ApplySSLCertificate(primaryDomain string, dnsAccountID, acmeAccountID int) (*domain.OnePanelSSL, error) {
	if dnsAccountID <= 0 {
		dnsAccountID = 1
	}
	if acmeAccountID <= 0 {
		acmeAccountID = 2
	}
	payload := domain.OnePanelSSLCreateReq{
		PrimaryDomain: primaryDomain,
		Provider:      "dnsAccount",
		AcmeAccountID: acmeAccountID,
		DNSAccountID:  dnsAccountID,
		AutoRenew:     true,
		KeyType:       "EC256",
		Apply:         true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.baseURL+"/api/v2/websites/ssl", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range c.headers() {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("1panel apply ssl: %w", err)
	}
	respBody, err := readBody(resp, "1panel apply ssl")
	if err != nil {
		return nil, err
	}
	var result domain.OnePanelResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("1panel apply ssl parse: %w", err)
	}
	if err := validateResponse(respBody, "1panel apply ssl"); err != nil {
		return nil, err
	}
	var cert domain.OnePanelSSL
	if err := json.Unmarshal(result.Data, &cert); err != nil {
		return nil, fmt.Errorf("1panel apply ssl result parse: %w", err)
	}
	return &cert, nil
}

// DeleteWebsite deletes a website by ID in 1Panel v2.
func (c *Client) DeleteWebsite(websiteID int) error {
	payload := domain.OnePanelDeleteWebsiteReq{
		ID:          websiteID,
		ForceDelete: true,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequest("POST", c.baseURL+"/api/v2/websites/del", bytes.NewReader(body))
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

// ListWebsites retrieves all websites from 1Panel v2.
func (c *Client) ListWebsites() ([]domain.OnePanelWebsite, error) {
	payload := domain.OnePanelWebsiteSearchReq{
		Page:     1,
		PageSize: 100,
		OrderBy:  "created_at",
		Order:    "descending",
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.baseURL+"/api/v2/websites/search", bytes.NewReader(body))
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
	var searchResult domain.OnePanelWebsiteSearchResult
	if err := json.Unmarshal(result.Data, &searchResult); err != nil {
		return nil, fmt.Errorf("1panel list websites result parse: %w", err)
	}
	return searchResult.Items, nil
}

// GetWebsite retrieves details of a specific website by ID.
func (c *Client) GetWebsite(websiteID int) (*domain.OnePanelWebsite, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/api/v2/websites/%d", c.baseURL, websiteID), nil)
	if err != nil {
		return nil, err
	}
	for k, v := range c.headers() {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("1panel get website: %w", err)
	}
	respBody, err := readBody(resp, "1panel get website")
	if err != nil {
		return nil, err
	}
	var result domain.OnePanelResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("1panel get website parse: %w", err)
	}
	if err := validateResponse(respBody, "1panel get website"); err != nil {
		return nil, err
	}
	var website domain.OnePanelWebsite
	if err := json.Unmarshal(result.Data, &website); err != nil {
		return nil, fmt.Errorf("1panel get website result parse: %w", err)
	}
	return &website, nil
}

// ListCertificates retrieves SSL certificates matching the name filter.
func (c *Client) ListCertificates(name string) ([]domain.OnePanelSSL, error) {
	payload := map[string]any{
		"page":     1,
		"pageSize": 50,
		"name":     name,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", c.baseURL+"/api/v2/websites/ssl/search", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range c.headers() {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("1panel list certificates: %w", err)
	}
	respBody, err := readBody(resp, "1panel list certificates")
	if err != nil {
		return nil, err
	}
	var result domain.OnePanelResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("1panel list certificates parse: %w", err)
	}
	if err := validateResponse(respBody, "1panel list certificates"); err != nil {
		return nil, err
	}
	var searchResult domain.OnePanelSSLSearchResult
	if err := json.Unmarshal(result.Data, &searchResult); err != nil {
		return nil, fmt.Errorf("1panel list certificates result parse: %w", err)
	}
	return searchResult.Items, nil
}

// TestConnection tests reachability and authentication of the 1Panel v2 API.
func (c *Client) TestConnection() error {
	req, err := http.NewRequest("GET", c.baseURL+"/api/v2/dashboard/base/os", nil)
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
