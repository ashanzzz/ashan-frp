package chmlfrp

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"strings"
)

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
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

func bodySignalsFailure(body []byte) bool {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.Contains(lower, "error") || strings.Contains(lower, "fail") || strings.Contains(lower, "already") {
		return true
	}
	if !strings.HasPrefix(trimmed, "{") && !strings.HasPrefix(trimmed, "[") {
		return false
	}
	var generic map[string]any
	if err := json.Unmarshal(body, &generic); err != nil {
		return false
	}
	if success, ok := generic["success"].(bool); ok && !success {
		return true
	}
	if code, ok := generic["code"].(float64); ok && int(code) != http.StatusOK {
		return true
	}
	if state, ok := generic["state"].(string); ok && strings.EqualFold(strings.TrimSpace(state), "error") {
		return true
	}
	return false
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func randSuffix(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	buf := make([]byte, n)
	for i := range buf {
		buf[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return string(buf)
}
