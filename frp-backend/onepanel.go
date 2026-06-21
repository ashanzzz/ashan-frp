package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type OnePanelClient struct {
	BaseURL  string
	Entrance string
	Token    string
	Client   *http.Client
}

func NewOnePanelClient(baseURL, entrance, token string) *OnePanelClient {
	return &OnePanelClient{
		BaseURL:  baseURL,
		Entrance: entrance,
		Token:    token,
		Client:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (op *OnePanelClient) SyncReverseProxy(domain string, localPort int) error {
	if op.BaseURL == "" || op.Token == "" {
		return fmt.Errorf("1panel config incomplete")
	}
	url := fmt.Sprintf("%s/api/v1/websites", op.BaseURL)
	bodyMap := map[string]interface{}{
		"primaryDomain": domain,
		"type":          "proxy",
		"proxyTarget":   fmt.Sprintf("[http://127.0.0.1](http://127.0.0.1):%d", localPort),
		"protocol":      "HTTPS",
		"ssl":           "auto",
	}
	jsonBytes, _ := json.Marshal(bodyMap)
	
	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	h := hmac.New(sha256.New, []byte(op.Token))
	h.Write([]byte(timestamp))
	signature := hex.EncodeToString(h.Sum(nil))

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-1Panel-Timestamp", timestamp)
	req.Header.Set("X-1Panel-Signature", signature)

	resp, err := op.Client.Do(req)
	if err != nil { return err }
	defer resp.Body.Close()
	return nil
}
