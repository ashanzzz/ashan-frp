package cloudflare

import (
	"ashan-frp/internal/domain"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	apiToken string
	zoneName string
	zoneID   string
	http     *http.Client
}

func NewClient(apiToken, zoneNameOrID string) *Client {
	trimmed := strings.TrimSpace(zoneNameOrID)
	return &Client{apiToken: apiToken, zoneName: trimmed, zoneID: trimmed, http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) tokenProbeURL() string {
	return "https://api.cloudflare.com/client/v4/user/tokens/verify"
}

func (c *Client) headers() map[string]string {
	return map[string]string{"Authorization": "Bearer " + c.apiToken, "Content-Type": "application/json"}
}

type tokenVerifyResponse struct {
	Success  bool                `json:"success"`
	Errors   []domain.CFAPIError `json:"errors,omitempty"`
	Messages []domain.CFAPIError `json:"messages,omitempty"`
}

func (c *Client) VerifyToken() error {
	req, err := http.NewRequest("GET", c.tokenProbeURL(), nil)
	if err != nil {
		return err
	}
	for k, v := range c.headers() {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("cloudflare verify token: %w", err)
	}
	body, err := readBody(resp, "cloudflare verify token")
	if err != nil {
		return err
	}
	var result tokenVerifyResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("cloudflare verify token parse: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("cloudflare verify token failed: %s", cloudflareFailureMessage(result.Errors, result.Messages))
	}
	return nil
}

func (c *Client) ValidateTokenAndZone() error {
	if strings.TrimSpace(c.apiToken) == "" {
		return fmt.Errorf("cloudflare API Token is empty")
	}
	if strings.TrimSpace(c.zoneID) == "" {
		return fmt.Errorf("cloudflare Zone name or Zone ID is required")
	}
	if err := c.VerifyToken(); err != nil {
		return err
	}
	if _, err := c.ListRecords(); err != nil {
		return fmt.Errorf("cloudflare verify Zone DNS read access: %w", err)
	}
	return nil
}

func (c *Client) resolveZoneID() (string, error) {
	if strings.TrimSpace(c.zoneID) == "" {
		return "", fmt.Errorf("cloudflare zone is empty")
	}
	if len(strings.TrimSpace(c.zoneID)) >= 20 && !strings.Contains(c.zoneID, ".") {
		return c.zoneID, nil
	}
	if strings.TrimSpace(c.zoneName) == "" {
		return c.zoneID, nil
	}
	req, err := http.NewRequest("GET", "https://api.cloudflare.com/client/v4/zones?name="+c.zoneName+"&per_page=1", nil)
	if err != nil {
		return "", err
	}
	for k, v := range c.headers() {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	body, err := readBody(resp, "cloudflare list zones")
	if err != nil {
		return "", err
	}
	var result struct {
		Success  bool                `json:"success"`
		Errors   []domain.CFAPIError `json:"errors,omitempty"`
		Messages []domain.CFAPIError `json:"messages,omitempty"`
		Result   []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"result,omitempty"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("cloudflare list zones parse: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("cloudflare list zones failed: %s", cloudflareFailureMessage(result.Errors, result.Messages))
	}
	if len(result.Result) == 0 {
		return "", fmt.Errorf("cloudflare zone not found: %s", c.zoneName)
	}
	c.zoneID = result.Result[0].ID
	return c.zoneID, nil
}

func (c *Client) ResolveZone() error { _, err := c.resolveZoneID(); return err }

func (c *Client) baseURL() (string, error) {
	zoneID, err := c.resolveZoneID()
	if err != nil {
		return "", err
	}
	return "https://api.cloudflare.com/client/v4/zones/" + zoneID + "/dns_records", nil
}

func readBody(resp *http.Response, op string) ([]byte, error) {
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("%s read: %w", op, err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		if op == "cloudflare verify token" && (resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized) {
			return nil, fmt.Errorf("CLOUDFLARE_TOKEN_INVALID: Cloudflare API Token is invalid or revoked")
		}
		if resp.StatusCode == http.StatusUnauthorized {
			return nil, fmt.Errorf("CLOUDFLARE_TOKEN_INVALID: Cloudflare API Token is invalid or revoked")
		}
		return nil, fmt.Errorf("%s failed: http %d", op, resp.StatusCode)
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

func joinAPIErrorMessages(items []domain.CFAPIError) string {
	if len(items) == 0 {
		return ""
	}
	messages := make([]string, 0, len(items))
	for _, item := range items {
		if message := strings.TrimSpace(item.Message); message != "" {
			messages = append(messages, message)
		}
	}
	return strings.Join(messages, "; ")
}

func cloudflareFailureMessage(errors, messages []domain.CFAPIError) string {
	return firstNonEmpty(joinAPIErrorMessages(errors), joinAPIErrorMessages(messages), "Cloudflare did not accept the request")
}

func (c *Client) ListRecords() ([]domain.CFDNSRecord, error) {
	url, err := c.baseURL()
	if err != nil {
		return nil, err
	}
	const perPage = 100
	records := make([]domain.CFDNSRecord, 0)
	for page := 1; ; page++ {
		req, err := http.NewRequest("GET", fmt.Sprintf("%s?per_page=%d&page=%d", url, perPage, page), nil)
		if err != nil {
			return nil, err
		}
		for key, value := range c.headers() {
			req.Header.Set(key, value)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return nil, err
		}
		body, err := readBody(resp, "cloudflare list records")
		if err != nil {
			return nil, err
		}
		var result domain.CFListResponse
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("cloudflare list records parse: %w", err)
		}
		if !result.Success {
			return nil, fmt.Errorf("cloudflare list records failed: %s", cloudflareFailureMessage(result.Errors, result.Messages))
		}
		records = append(records, result.Result...)
		if result.ResultInfo == nil || result.ResultInfo.TotalPages <= page || len(result.Result) == 0 {
			return records, nil
		}
	}
}
func (c *Client) CreateDNSRecord(input domain.DNSRecordInput, comment string) (*domain.CFDNSRecord, error) {
	payload, err := c.recordPayload(input, comment)
	if err != nil {
		return nil, err
	}
	url, err := c.baseURL()
	if err != nil {
		return nil, err
	}
	return c.writeRecord(http.MethodPost, url, payload, "cloudflare create record")
}

func (c *Client) UpdateDNSRecord(recordID string, input domain.DNSRecordInput, comment string) (*domain.CFDNSRecord, error) {
	if strings.TrimSpace(recordID) == "" {
		return nil, fmt.Errorf("cloudflare DNS record ID is required")
	}
	payload, err := c.recordPayload(input, comment)
	if err != nil {
		return nil, err
	}
	url, err := c.baseURL()
	if err != nil {
		return nil, err
	}
	return c.writeRecord(http.MethodPut, url+"/"+recordID, payload, "cloudflare update record")
}

func (c *Client) recordPayload(input domain.DNSRecordInput, comment string) (domain.CFCreateRecordReq, error) {
	recordType := strings.ToUpper(strings.TrimSpace(input.Type))
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return domain.CFCreateRecordReq{}, fmt.Errorf("cloudflare DNS record name is required")
	}
	if !supportedDNSRecordType(recordType) {
		return domain.CFCreateRecordReq{}, fmt.Errorf("cloudflare DNS record type %q is not supported", recordType)
	}
	if input.TTL != 1 && (input.TTL < 60 || input.TTL > 86400) {
		return domain.CFCreateRecordReq{}, fmt.Errorf("cloudflare DNS TTL must be automatic (1) or between 60 and 86400 seconds")
	}
	if input.Proxied != nil && recordType != "A" && recordType != "AAAA" && recordType != "CNAME" {
		return domain.CFCreateRecordReq{}, fmt.Errorf("cloudflare %s record does not support proxy settings", recordType)
	}
	payload := domain.CFCreateRecordReq{Type: recordType, Name: name, TTL: input.TTL, Comment: comment}
	if payload.TTL == 0 {
		payload.TTL = 1
	}
	switch recordType {
	case "A", "AAAA", "CNAME", "TXT":
		payload.Content = strings.TrimSpace(input.Content)
		if payload.Content == "" {
			return domain.CFCreateRecordReq{}, fmt.Errorf("cloudflare %s record content is required", recordType)
		}
		if input.Proxied != nil {
			payload.Proxied = *input.Proxied
		}
	case "MX":
		payload.Content = strings.TrimSpace(input.Content)
		if payload.Content == "" || input.Priority == nil || *input.Priority < 0 || *input.Priority > 65535 {
			return domain.CFCreateRecordReq{}, fmt.Errorf("cloudflare MX record requires content and a priority between 0 and 65535")
		}
		payload.Priority = input.Priority
	case "CAA":
		if input.CAA == nil || strings.TrimSpace(input.CAA.Tag) == "" || strings.TrimSpace(input.CAA.Value) == "" || input.CAA.Flags < 0 || input.CAA.Flags > 255 {
			return domain.CFCreateRecordReq{}, fmt.Errorf("cloudflare CAA record requires flags (0-255), tag, and value")
		}
		payload.Data = map[string]any{"flags": input.CAA.Flags, "tag": strings.TrimSpace(input.CAA.Tag), "value": strings.TrimSpace(input.CAA.Value)}
	}
	return payload, nil
}

func supportedDNSRecordType(recordType string) bool {
	switch recordType {
	case "A", "AAAA", "CNAME", "TXT", "MX", "CAA":
		return true
	default:
		return false
	}
}

func (c *Client) writeRecord(method, url string, payload domain.CFCreateRecordReq, op string) (*domain.CFDNSRecord, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for key, value := range c.headers() {
		req.Header.Set(key, value)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	responseBody, err := readBody(resp, op)
	if err != nil {
		return nil, err
	}
	var response domain.CFResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("%s parse: %w", op, err)
	}
	if !response.Success {
		return nil, fmt.Errorf("%s failed: %s", op, cloudflareFailureMessage(response.Errors, response.Messages))
	}
	var record domain.CFDNSRecord
	if err := json.Unmarshal(response.Result, &record); err != nil {
		return nil, fmt.Errorf("%s result parse: %w", op, err)
	}
	return &record, nil
}
func (c *Client) CreateRecord(name, recordType, content string, proxied bool, tunnelID string) (*domain.CFDNSRecord, error) {
	comment := fmt.Sprintf("ashan-frp managed: tunnel %s", tunnelID)
	url, err := c.baseURL()
	if err != nil {
		return nil, err
	}
	payload := domain.CFCreateRecordReq{Type: recordType, Name: name, Content: content, Proxied: proxied, TTL: 1, Comment: comment}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	for k, v := range c.headers() {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	respBody, err := readBody(resp, "cloudflare create record")
	if err != nil {
		return nil, err
	}
	var result domain.CFResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("cloudflare create record parse: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("cloudflare create record failed: %s", cloudflareFailureMessage(result.Errors, result.Messages))
	}
	var record domain.CFDNSRecord
	if err := json.Unmarshal(result.Result, &record); err != nil {
		return nil, fmt.Errorf("cloudflare create record result parse: %w", err)
	}
	return &record, nil
}

func (c *Client) DeleteRecord(recordID string) error {
	url, err := c.baseURL()
	if err != nil {
		return err
	}
	req, err := http.NewRequest("DELETE", url+"/"+recordID, nil)
	if err != nil {
		return err
	}
	for k, v := range c.headers() {
		req.Header.Set(k, v)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	body, err := readBody(resp, "cloudflare delete record")
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(body)) == "" {
		return nil
	}
	var result domain.CFResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("cloudflare delete record parse: %w", err)
	}
	if !result.Success {
		return fmt.Errorf("cloudflare delete record failed: %s", cloudflareFailureMessage(result.Errors, result.Messages))
	}
	return nil
}

func (c *Client) FindRecordByComment(tunnelID string) (*domain.CFDNSRecord, error) {
	records, err := c.ListRecords()
	if err != nil {
		return nil, err
	}
	tag := fmt.Sprintf("ashan-frp managed: tunnel %s", tunnelID)
	for _, r := range records {
		if r.Comment == tag {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (c *Client) FindRecordsByTag() ([]domain.CFDNSRecord, error) {
	records, err := c.ListRecords()
	if err != nil {
		return nil, err
	}
	var managed []domain.CFDNSRecord
	for _, r := range records {
		if len(r.Comment) >= 18 && r.Comment[:18] == "ashan-frp managed:" {
			managed = append(managed, r)
		}
	}
	return managed, nil
}
