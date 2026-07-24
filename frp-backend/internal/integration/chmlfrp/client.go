package chmlfrp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"ashan-frp/internal/domain"
)

const (
	V1BaseURL = "https://cf-v1.uapis.cn/api"
	V2BaseURL = "https://cf-v2.uapis.cn"
)

type Client struct {
	username string
	password string
	token    string
	userID   int
	http     *http.Client
}

func NewClient(username, passwordOrToken string) *Client {
	c := &Client{username: username, password: passwordOrToken, http: &http.Client{Timeout: 30 * time.Second}}
	if username == "oauth2_user" || username == "token" || strings.HasPrefix(passwordOrToken, "eyJ") || (len(username) == 0 && len(passwordOrToken) > 0) {
		c.token = passwordOrToken
	}
	return c
}

func (c *Client) ensureLogin() error {
	if c.token != "" {
		return nil
	}
	return c.Login()
}

func (c *Client) Login() error {
	resp, err := c.http.PostForm(V1BaseURL+"/login.php", url.Values{"username": {c.username}, "password": {c.password}})
	if err != nil {
		return fmt.Errorf("chmlfrp login: %w", err)
	}
	body, err := readBody(resp, "chmlfrp login")
	if err != nil {
		return err
	}
	var result domain.ChmlFrpLoginResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("chmlfrp login parse: %w", err)
	}
	if result.Code != 200 || result.Token == "" {
		return fmt.Errorf("chmlfrp login failed: code=%d: %s", result.Code, firstNonEmpty(result.Error, result.Msg, result.Message, string(body)))
	}
	c.token = result.Token
	c.userID = result.UserID
	return nil
}

func (c *Client) GetNodes() ([]domain.ChmlFrpNode, error) {
	if err := c.ensureLogin(); err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("GET", V2BaseURL+"/node", nil)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err == nil {
		body, rErr := readBody(resp, "chmlfrp get nodes v2")
		if rErr == nil {
			var result struct {
				Code int                 `json:"code"`
				Data []domain.ChmlFrpNode `json:"data"`
			}
			if json.Unmarshal(body, &result) == nil && len(result.Data) > 0 {
				return result.Data, nil
			}
			var nodes []domain.ChmlFrpNode
			if json.Unmarshal(body, &nodes) == nil && len(nodes) > 0 {
				return nodes, nil
			}
		}
	}

	resp, err = c.http.Get(V1BaseURL + "/unode.php")
	if err != nil {
		return nil, fmt.Errorf("chmlfrp get nodes: %w", err)
	}
	body, err := readBody(resp, "chmlfrp get nodes")
	if err != nil {
		return nil, err
	}
	var nodes []domain.ChmlFrpNode
	if err := json.Unmarshal(body, &nodes); err != nil {
		return nil, fmt.Errorf("chmlfrp get nodes parse: %w", err)
	}
	return nodes, nil
}

func (c *Client) GetTunnels() ([]domain.ChmlFrpTunnel, error) {
	if err := c.ensureLogin(); err != nil {
		return nil, err
	}
	req, _ := http.NewRequest("GET", V2BaseURL+"/tunnel", nil)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err == nil {
		body, rErr := readBody(resp, "chmlfrp get tunnels v2")
		if rErr == nil {
			var result struct {
				Code int                   `json:"code"`
				Data []domain.ChmlFrpTunnel `json:"data"`
			}
			if json.Unmarshal(body, &result) == nil && len(result.Data) > 0 {
				return result.Data, nil
			}
			var tunnels []domain.ChmlFrpTunnel
			if json.Unmarshal(body, &tunnels) == nil && len(tunnels) > 0 {
				return tunnels, nil
			}
		}
	}

	resp, err = c.http.PostForm(V1BaseURL+"/usertunnel.php", url.Values{"token": {c.token}})
	if err != nil {
		return nil, fmt.Errorf("chmlfrp get tunnels: %w", err)
	}
	body, err := readBody(resp, "chmlfrp get tunnels")
	if err != nil {
		return nil, fmt.Errorf("chmlfrp get tunnels read body: %w", err)
	}
	var tunnels []domain.ChmlFrpTunnel
	if err := json.Unmarshal(body, &tunnels); err != nil {
		return nil, fmt.Errorf("chmlfrp get tunnels parse: %w", err)
	}
	return tunnels, nil
}

func (c *Client) CreateTunnel(params domain.ChmlFrpCreateTunnelReq) (string, error) {
	if err := c.ensureLogin(); err != nil {
		return "", err
	}
	name := params.TunnelName
	if !strings.HasPrefix(name, "[ashan-frp]") {
		name = "[ashan-frp]" + name
	}
	form := url.Values{
		"token": {c.token}, "userid": {strconv.Itoa(c.userID)}, "type": {params.PortType},
		"node": {params.Node}, "name": {name}, "ap": {params.ExtraParams},
		"localip": {params.LocalIP}, "nport": {strconv.Itoa(params.LocalPort)},
		"encryption": {boolStr(params.Encryption)}, "compression": {boolStr(params.Compression)},
	}
	if params.PortType == "tcp" || params.PortType == "udp" {
		form.Set("dorp", strconv.Itoa(params.RemotePort))
	} else {
		form.Set("dorp", params.BandDomain)
		form.Set("domainNameLabel", "custom")
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			form.Set("name", name+"-"+randSuffix(4))
		}

		// Try V2 create_tunnel endpoint first
		req, _ := http.NewRequest("POST", V2BaseURL+"/create_tunnel", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		resp, err := c.http.Do(req)
		if err != nil || resp.StatusCode >= 400 {
			resp, err = c.http.PostForm(V1BaseURL+"/tunnel.php", form)
		}
		if err != nil {
			lastErr = fmt.Errorf("chmlfrp create tunnel: %w", err)
			continue
		}
		body, err := readBody(resp, "chmlfrp create tunnel")
		if err != nil {
			lastErr = err
			continue
		}
		if bodySignalsFailure(body) {
			lastErr = fmt.Errorf("chmlfrp create tunnel rejected: %s", strings.TrimSpace(string(body)))
			continue
		}
		tunnels, err := c.GetTunnels()
		if err != nil {
			lastErr = err
			continue
		}
		search := form.Get("name")
		for _, tunnel := range tunnels {
			if tunnel.Name == search {
				return tunnel.ID, nil
			}
		}
		return search, nil
	}
	if lastErr != nil {
		return "", fmt.Errorf("chmlfrp create failed after 3 retries: %w", lastErr)
	}
	return "", fmt.Errorf("chmlfrp create failed after 3 retries")
}

func (c *Client) DeleteTunnel(tunnelID string) error {
	if err := c.ensureLogin(); err != nil {
		return err
	}

	// Try V2 delete_tunnel endpoint first
	v2URL := fmt.Sprintf("%s/delete_tunnel?tunnelid=%s&token=%s", V2BaseURL, url.QueryEscape(tunnelID), url.QueryEscape(c.token))
	req, _ := http.NewRequest("POST", v2URL, nil)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err == nil {
		body, rErr := readBody(resp, "chmlfrp delete tunnel v2")
		if rErr == nil && !bodySignalsFailure(body) {
			return nil
		}
	}

	// Fallback to V1 deletetl.php
	resp, err = c.http.PostForm(V1BaseURL+"/deletetl.php", url.Values{"token": {c.token}, "userid": {strconv.Itoa(c.userID)}, "nodeid": {tunnelID}})
	if err != nil {
		return fmt.Errorf("chmlfrp delete tunnel: %w", err)
	}
	body, err := readBody(resp, "chmlfrp delete tunnel")
	if err != nil {
		return err
	}
	if bodySignalsFailure(body) {
		return fmt.Errorf("chmlfrp delete tunnel failed: %s", strings.TrimSpace(string(body)))
	}
	return nil
}

func (c *Client) GetConfig(node string) (string, error) {
	if err := c.ensureLogin(); err != nil {
		return "", err
	}

	// Try V2 tunnel_config endpoint first
	v2URL := fmt.Sprintf("%s/tunnel_config?token=%s&node=%s", V2BaseURL, url.QueryEscape(c.token), url.QueryEscape(node))
	req, _ := http.NewRequest("GET", v2URL, nil)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.http.Do(req)
	if err == nil && resp.StatusCode == http.StatusOK {
		body, rErr := readBody(resp, "chmlfrp get config v2")
		if rErr == nil && len(body) > 0 && !bodySignalsFailure(body) {
			var result domain.ChmlFrpConfigResponse
			if json.Unmarshal(body, &result) == nil && result.Success {
				return result.Message, nil
			}
		}
	}

	// Fallback to V1 frpconfig.php
	resp, err = c.http.PostForm(V1BaseURL+"/frpconfig.php", url.Values{"usertoken": {c.token}, "node": {node}})
	if err != nil {
		return "", fmt.Errorf("chmlfrp get config: %w", err)
	}
	body, err := readBody(resp, "chmlfrp get config")
	if err != nil {
		return "", err
	}
	var result domain.ChmlFrpConfigResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("chmlfrp get config parse: %w", err)
	}
	if !result.Success {
		return "", fmt.Errorf("chmlfrp get config failed: %s", firstNonEmpty(result.Message, result.Msg, string(body)))
	}
	return result.Message, nil
}
