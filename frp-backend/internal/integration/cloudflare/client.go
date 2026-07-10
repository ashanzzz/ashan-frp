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

func (c *Client) headers() map[string]string {
	return map[string]string{"Authorization": "Bearer " + c.apiToken, "Content-Type": "application/json"}
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
		Success bool `json:"success"`
		Errors []domain.CFAPIError `json:"errors,omitempty"`
		Messages []domain.CFAPIError `json:"messages,omitempty"`
		Result []struct { ID string `json:"id"`; Name string `json:"name"` } `json:"result,omitempty"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("cloudflare list zones parse: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("cloudflare list zones failed: %s", firstNonEmpty(joinAPIErrorMessages(result.Errors), joinAPIErrorMessages(result.Messages), strings.TrimSpace(string(body))))
	}
	if len(result.Result) == 0 {
		return "", fmt.Errorf("cloudflare zone not found: %s", c.zoneName)
	}
	c.zoneID = result.Result[0].ID
	return c.zoneID, nil
}

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

func (c *Client) ListRecords() ([]domain.CFDNSRecord, error) {
	url, err := c.baseURL()
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", url+"?per_page=500", nil)
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
	body, err := readBody(resp, "cloudflare list records")
	if err != nil {
		return nil, err
	}
	var result domain.CFListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("cloudflare list records parse: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("cloudflare list records failed: %s", firstNonEmpty(joinAPIErrorMessages(result.Errors), joinAPIErrorMessages(result.Messages), strings.TrimSpace(string(body))))
	}
	return result.Result, nil
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
		return nil, fmt.Errorf("cloudflare create record failed: %s", firstNonEmpty(joinAPIErrorMessages(result.Errors), joinAPIErrorMessages(result.Messages), strings.TrimSpace(string(respBody))))
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
		return fmt.Errorf("cloudflare delete record failed: %s", firstNonEmpty(joinAPIErrorMessages(result.Errors), joinAPIErrorMessages(result.Messages), strings.TrimSpace(string(body))))
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
