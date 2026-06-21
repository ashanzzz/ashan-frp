package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ChmlClient struct {
	APIKey string
	Client *http.Client
}

func NewChmlClient(apiKey string) *ChmlClient {
	return &ChmlClient{
		APIKey: apiKey,
		Client: &http.Client{Timeout: 8 * time.Second},
}
}

func (cc *ChmlClient) LoginAndGetToken(username, password string) (string, error) {
	url := "[https://api.chmlfrp.cn/v1/auth/login](https://api.chmlfrp.cn/v1/auth/login)"
	payload := map[string]string{"username": username, "password": password}
	jsonBytes, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBytes))
	if err != nil { return "", err }
	req.Header.Set("Content-Type", "application/json")

	resp, err := cc.Client.Do(req)
	if err != nil { return "", err }
	defer resp.Body.Close()

	var res struct {
		Success bool `json:"success"`
		Data    struct { Token string `json:"token"` } `json:"data"`
	}
	json.NewDecoder(resp.Body).Decode(&res)
	if !res.Success { return "", fmt.Errorf("auth failed") }
	return res.Data.Token, nil
}
