package chmlfrp

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
	"ashan-frp/internal/domain"
)

const V1BaseURL = "https://cf-v1.uapis.cn/api"

type Client struct { username string; password string; token string; userID int; http *http.Client }

func NewClient(username, password string) *Client { return &Client{username: username, password: password, http: &http.Client{Timeout: 30 * time.Second}} }

func (c *Client) Login() error {
	resp, err := c.http.PostForm(V1BaseURL+"/login.php", url.Values{"username": {c.username}, "password": {c.password}})
	if err != nil { return fmt.Errorf("chmlfrp login: %w", err) }
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result domain.ChmlFrpLoginResponse
	json.Unmarshal(body, &result)
	if result.Code != 200 && result.Token == "" { return fmt.Errorf("chmlfrp login failed: %s", result.Error) }
	c.token = result.Token; c.userID = result.UserID
	return nil
}

func (c *Client) GetNodes() ([]domain.ChmlFrpNode, error) {
	if c.token == "" { c.Login() }
	resp, _ := c.http.Get(V1BaseURL + "/unode.php"); defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var nodes []domain.ChmlFrpNode; json.Unmarshal(body, &nodes)
	return nodes, nil
}

func (c *Client) GetTunnels() ([]domain.ChmlFrpTunnel, error) {
	if c.token == "" { c.Login() }
	resp, _ := c.http.PostForm(V1BaseURL+"/usertunnel.php", url.Values{"token": {c.token}})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var tunnels []domain.ChmlFrpTunnel; json.Unmarshal(body, &tunnels)
	return tunnels, nil
}

func (c *Client) CreateTunnel(params domain.ChmlFrpCreateTunnelReq) (string, error) {
	if c.token == "" { c.Login() }
	name := params.TunnelName
	if !strings.HasPrefix(name, "[ashan-frp]") { name = "[ashan-frp]" + name }
	form := url.Values{
		"token": {c.token}, "userid": {strconv.Itoa(c.userID)}, "type": {params.PortType},
		"node": {params.Node}, "name": {name}, "ap": {params.ExtraParams},
		"localip": {params.LocalIP}, "nport": {strconv.Itoa(params.LocalPort)},
		"encryption": {boolStr(params.Encryption)}, "compression": {boolStr(params.Compression)},
	}
	if params.PortType == "tcp" || params.PortType == "udp" { form.Set("dorp", strconv.Itoa(params.RemotePort)) } else { form.Set("dorp", params.BandDomain); form.Set("domainNameLabel", "custom") }
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 { form.Set("name", name+"-"+randSuffix(4)) }
		resp, _ := c.http.PostForm(V1BaseURL+"/tunnel.php", form)
		body, _ := io.ReadAll(resp.Body); resp.Body.Close()
		if strings.Contains(string(body), "error") || strings.Contains(string(body), "e") || strings.Contains(string(body), "already") { continue }
		tunnels, _ := c.GetTunnels()
		search := form.Get("name")
		for _, t := range tunnels { if t.Name == search { return t.ID, nil } }
		return search, nil
	}
	return "", fmt.Errorf("chmlfrp create failed after 3 retries")
}

func (c *Client) DeleteTunnel(tunnelID string) error {
	if c.token == "" { c.Login() }
	resp, _ := c.http.PostForm(V1BaseURL+"/deletetl.php", url.Values{"token": {c.token}, "userid": {strconv.Itoa(c.userID)}, "nodeid": {tunnelID}})
	defer resp.Body.Close()
	return nil
}

func (c *Client) GetConfig(node string) (string, error) {
	if c.token == "" { c.Login() }
	resp, _ := c.http.PostForm(V1BaseURL+"/frpconfig.php", url.Values{"usertoken": {c.token}, "node": {node}})
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result domain.ChmlFrpConfigResponse; json.Unmarshal(body, &result)
	if !result.Success { return string(body), nil }
	return result.Message, nil
}

func boolStr(b bool) string { if b { return "true" }; return "false" }
func randSuffix(n int) string { const l = "abcdefghijklmnopqrstuvwxyz0123456789"; b := make([]byte, n); for i := range b { b[i] = l[rand.Intn(len(l))] }; return string(b) }
