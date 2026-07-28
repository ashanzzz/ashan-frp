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
	// The management console accepts ChmlFrp API Tokens only. The upstream
	// account name is display metadata; every non-empty second argument is sent
	// as the token so saved real account names never get mistaken for passwords.
	c := &Client{username: username, password: passwordOrToken, http: &http.Client{Timeout: 30 * time.Second}}
	if strings.TrimSpace(passwordOrToken) != "" {
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

// GetCurrentUser validates the configured token with ChmlFrp and returns the
// account identity that actually owns it. Form data keeps the provider secret
// out of the outbound URL.
func (c *Client) GetCurrentUser() (*domain.ChmlFrpUserInfo, error) {
	if err := c.ensureLogin(); err != nil {
		return nil, err
	}
	resp, err := c.http.PostForm(V2BaseURL+"/userinfo", url.Values{"token": {c.token}})
	if err != nil {
		return nil, fmt.Errorf("chmlfrp get current user: %w", err)
	}
	body, err := readBody(resp, "chmlfrp get current user")
	if err != nil {
		return nil, err
	}
	var result domain.ChmlFrpUserInfoResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("chmlfrp get current user parse: %w", err)
	}
	if result.Code != http.StatusOK || (!strings.EqualFold(result.State, "success") && result.State != "") || result.Data.Username == "" {
		return nil, fmt.Errorf("chmlfrp get current user failed: %s", firstNonEmpty(result.Error, result.Message, result.Msg, "credential rejected"))
	}
	return &result.Data, nil
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
				Code int                  `json:"code"`
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
				Code int                    `json:"code"`
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

func (c *Client) LoginV2() error {
	if c.token == "" {
		return fmt.Errorf("chmlfrp login v2: token is empty")
	}
	u := fmt.Sprintf("%s/login?access_token=%s", V2BaseURL, url.QueryEscape(c.token))
	resp, err := c.http.Get(u)
	if err != nil {
		return fmt.Errorf("chmlfrp login v2 request: %w", err)
	}
	body, err := readBody(resp, "chmlfrp login v2")
	if err != nil {
		return err
	}
	var result struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			ID int `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &result); err == nil && result.Code == 200 && result.Data.ID > 0 {
		c.userID = result.Data.ID
		return nil
	}
	return nil
}

func (c *Client) GetNodeInfo(node string) (*domain.ChmlFrpNodeInfoResp, error) {
	if err := c.ensureLogin(); err != nil {
		return nil, err
	}
	u := fmt.Sprintf("%s/nodeinfo?token=%s&node=%s", V2BaseURL, url.QueryEscape(c.token), url.QueryEscape(node))
	resp, err := c.http.Get(u)
	if err != nil {
		return nil, fmt.Errorf("chmlfrp nodeinfo request: %w", err)
	}
	body, err := readBody(resp, "chmlfrp nodeinfo")
	if err != nil {
		return nil, err
	}
	var result domain.ChmlFrpNodeInfoResp
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("chmlfrp nodeinfo parse: %w", err)
	}
	return &result, nil
}

func (c *Client) CreateTunnel(params domain.ChmlFrpCreateTunnelReq) (string, error) {
	if err := c.ensureLogin(); err != nil {
		return "", err
	}
	name := params.TunnelName
	if !strings.HasPrefix(name, "[ashan-frp]") {
		name = "[ashan-frp]" + name
	}

	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		currentName := name
		if attempt > 0 {
			currentName = name + "-" + randSuffix(4)
		}

		// Ground Truth Spec: POST https://cf-v2.uapis.cn/create_tunnel with JSON body (NO Authorization header)
		v2Body := domain.ChmlFrpV2CreateTunnelBody{
			Token:       c.token,
			TunnelName:  currentName,
			Node:        params.Node,
			LocalIP:     params.LocalIP,
			PortType:    params.PortType,
			LocalPort:   params.LocalPort,
			RemotePort:  params.RemotePort,
			BandDomain:  params.BandDomain,
			Encryption:  params.Encryption,
			Compression: params.Compression,
			ExtraParams: params.ExtraParams,
		}
		jsonBytes, _ := json.Marshal(v2Body)

		req, err := http.NewRequest("POST", V2BaseURL+"/create_tunnel", strings.NewReader(string(jsonBytes)))
		if err == nil {
			req.Header.Set("Content-Type", "application/json")
			resp, rErr := c.http.Do(req)
			if rErr == nil {
				body, readErr := readBody(resp, "chmlfrp create tunnel v2")
				if readErr == nil && !bodySignalsFailure(body) {
					tunnels, tErr := c.GetTunnels()
					if tErr != nil {
						lastErr = fmt.Errorf("chmlfrp lookup tunnels v2: %w", tErr)
						continue
					}
					for _, tunnel := range tunnels {
						if tunnel.Name == currentName {
							return tunnel.ID, nil
						}
					}
					return currentName, nil
				} else if readErr == nil {
					lastErr = fmt.Errorf("chmlfrp create tunnel v2 rejected: %s", strings.TrimSpace(string(body)))
				}
			}
		}

		// Fallback to V1 form-encoded endpoint if V2 fails
		form := url.Values{
			"token": {c.token}, "userid": {strconv.Itoa(c.userID)}, "type": {params.PortType},
			"node": {params.Node}, "name": {currentName}, "ap": {params.ExtraParams},
			"localip": {params.LocalIP}, "nport": {strconv.Itoa(params.LocalPort)},
			"encryption": {boolStr(params.Encryption)}, "compression": {boolStr(params.Compression)},
		}
		if params.PortType == "tcp" || params.PortType == "udp" {
			form.Set("dorp", strconv.Itoa(params.RemotePort))
		} else {
			form.Set("dorp", params.BandDomain)
			form.Set("domainNameLabel", "custom")
		}
		resp, err := c.http.PostForm(V1BaseURL+"/tunnel.php", form)
		if err != nil {
			lastErr = fmt.Errorf("chmlfrp create tunnel v1: %w", err)
			continue
		}
		body, err := readBody(resp, "chmlfrp create tunnel v1")
		if err != nil {
			lastErr = err
			continue
		}
		if bodySignalsFailure(body) {
			lastErr = fmt.Errorf("chmlfrp create tunnel v1 rejected: %s", strings.TrimSpace(string(body)))
			continue
		}
		tunnels, err := c.GetTunnels()
		if err != nil {
			lastErr = fmt.Errorf("chmlfrp lookup tunnels v1: %w", err)
			continue
		}
		for _, tunnel := range tunnels {
			if tunnel.Name == currentName {
				return tunnel.ID, nil
			}
		}
		return currentName, nil
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

	// Ground Truth Spec: MUST BE GET https://cf-v2.uapis.cn/delete_tunnel?tunnelid={id} with Bearer Auth Header
	v2URL := fmt.Sprintf("%s/delete_tunnel?tunnelid=%s", V2BaseURL, url.QueryEscape(tunnelID))
	req, err := http.NewRequest("GET", v2URL, nil)
	if err == nil {
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		resp, rErr := c.http.Do(req)
		if rErr == nil {
			body, readErr := readBody(resp, "chmlfrp delete tunnel v2")
			if readErr == nil {
				bodyStr := strings.TrimSpace(string(body))
				if !bodySignalsFailure(body) || strings.Contains(bodyStr, "不存在") || strings.Contains(bodyStr, "不属于") {
					return nil
				}
			}
		}
	}

	// Fallback to V1 deletetl.php
	resp, err := c.http.PostForm(V1BaseURL+"/deletetl.php", url.Values{"token": {c.token}, "userid": {strconv.Itoa(c.userID)}, "nodeid": {tunnelID}})
	if err != nil {
		return fmt.Errorf("chmlfrp delete tunnel: %w", err)
	}
	body, err := readBody(resp, "chmlfrp delete tunnel")
	if err != nil {
		return err
	}
	bodyStr := strings.TrimSpace(string(body))
	if bodySignalsFailure(body) && !strings.Contains(bodyStr, "不存在") && !strings.Contains(bodyStr, "不属于") {
		return fmt.Errorf("chmlfrp delete tunnel failed: %s", bodyStr)
	}
	return nil
}

func (c *Client) GetConfig(node string) (string, error) {
	if err := c.ensureLogin(); err != nil {
		return "", err
	}

	// Ground Truth Spec: GET https://cf-v2.uapis.cn/tunnel_config?token={access_token}&node={node_name}
	v2URL := fmt.Sprintf("%s/tunnel_config?token=%s&node=%s", V2BaseURL, url.QueryEscape(c.token), url.QueryEscape(node))
	req, err := http.NewRequest("GET", v2URL, nil)
	if err == nil {
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		resp, rErr := c.http.Do(req)
		if rErr == nil && resp.StatusCode == http.StatusOK {
			body, readErr := readBody(resp, "chmlfrp get config v2")
			if readErr == nil && len(body) > 0 && !bodySignalsFailure(body) {
				var result struct {
					Code    int    `json:"code"`
					Msg     string `json:"msg"`
					Success bool   `json:"success"`
					Message string `json:"message"`
					Data    string `json:"data"`
				}
				if json.Unmarshal(body, &result) == nil {
					if result.Code == 200 || result.Success {
						return firstNonEmpty(result.Data, result.Message, result.Msg), nil
					}
				} else if strings.Contains(string(body), "[common]") || strings.Contains(string(body), "server_addr") {
					return string(body), nil
				}
			}
		}
	}

	// Fallback to V1 frpconfig.php
	resp, err := c.http.PostForm(V1BaseURL+"/frpconfig.php", url.Values{"usertoken": {c.token}, "node": {node}})
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
	if !result.Success || bodySignalsFailure(body) {
		return "", fmt.Errorf("chmlfrp get config failed: %s", firstNonEmpty(result.Message, result.Msg, string(body)))
	}
	return firstNonEmpty(result.Data, result.Message, result.Msg), nil
}
