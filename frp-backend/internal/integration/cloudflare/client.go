package cloudflare

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"ashan-frp/internal/domain"
)

type Client struct { apiToken string; zoneID string; http *http.Client }

func NewClient(apiToken, zoneID string) *Client { return &Client{apiToken: apiToken, zoneID: zoneID, http: &http.Client{Timeout: 30 * time.Second}} }
func (c *Client) baseURL() string { return "https://api.cloudflare.com/client/v4/zones/" + c.zoneID + "/dns_records" }
func (c *Client) headers() map[string]string { return map[string]string{"Authorization": "Bearer " + c.apiToken, "Content-Type": "application/json"} }

func (c *Client) ListRecords() ([]domain.CFDNSRecord, error) {
	req, _ := http.NewRequest("GET", c.baseURL()+"?per_page=500", nil)
	for k, v := range c.headers() { req.Header.Set(k, v) }
	resp, _ := c.http.Do(req); defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result domain.CFListResponse; json.Unmarshal(body, &result)
	return result.Result, nil
}

func (c *Client) CreateRecord(name, recordType, content string, proxied bool, tunnelID string) (*domain.CFDNSRecord, error) {
	comment := fmt.Sprintf("ashan-frp managed: tunnel %s", tunnelID)
	payload := domain.CFCreateRecordReq{Type: recordType, Name: name, Content: content, Proxied: proxied, TTL: 1, Comment: comment}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("POST", c.baseURL(), bytes.NewReader(body))
	for k, v := range c.headers() { req.Header.Set(k, v) }
	resp, _ := c.http.Do(req); defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	var result domain.CFResponse; json.Unmarshal(respBody, &result)
	if !result.Success { return nil, fmt.Errorf("cf: %d", len(result.Errors)) }
	var record domain.CFDNSRecord; json.Unmarshal(result.Result, &record)
	return &record, nil
}

func (c *Client) DeleteRecord(recordID string) error {
	req, _ := http.NewRequest("DELETE", c.baseURL()+"/"+recordID, nil)
	for k, v := range c.headers() { req.Header.Set(k, v) }
	resp, _ := c.http.Do(req); resp.Body.Close()
	return nil
}

func (c *Client) FindRecordByComment(tunnelID string) (*domain.CFDNSRecord, error) {
	records, _ := c.ListRecords()
	tag := fmt.Sprintf("ashan-frp managed: tunnel %s", tunnelID)
	for _, r := range records { if r.Comment == tag { return &r, nil } }
	return nil, fmt.Errorf("not found")
}

func (c *Client) FindRecordsByTag() ([]domain.CFDNSRecord, error) {
	records, _ := c.ListRecords()
	var managed []domain.CFDNSRecord
	for _, r := range records { if len(r.Comment) >= 18 && r.Comment[:18] == "ashan-frp managed:" { managed = append(managed, r) } }
	return managed, nil
}
