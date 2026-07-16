package cloudflare

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"ashan-frp/internal/domain"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func newTestClient(rt http.RoundTripper) *Client {
	return &Client{apiToken: "token", zoneID: "zone", http: &http.Client{Transport: rt}}
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestClient_VerifyToken_succeedsWhenAPIConfirmsToken(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, http.MethodGet, req.Method)
		require.Equal(t, "/client/v4/user/tokens/verify", req.URL.Path)
		require.Equal(t, "Bearer token", req.Header.Get("Authorization"))
		return response(http.StatusOK, `{"success":true}`), nil
	}))

	require.NoError(t, client.VerifyToken())
}

func TestClient_VerifyToken_returnsErrorWhenAPIReportsFailure(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"success":false,"errors":[{"code":1000,"message":"bad"}]}`), nil
	}))

	err := client.VerifyToken()
	require.Error(t, err)
}

func TestClient_ListRecords_returnsErrorWhenAPIReportsFailure(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"success":false,"errors":[{"code":1000,"message":"bad"}]}`), nil
	}))

	_, err := client.ListRecords()
	require.Error(t, err)
}

func TestClient_DeleteRecord_returnsErrorOnHTTPStatus(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusInternalServerError, "fail"), nil
	}))

	err := client.DeleteRecord("record-1")
	require.Error(t, err)
}

func TestClient_ListRecords_returnsErrorOnTransportFailure(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	}))

	_, err := client.ListRecords()
	require.Error(t, err)
}

func TestClient_ValidateTokenAndZone_checksTokenAndDNSRead(t *testing.T) {
	requests := make([]string, 0, 2)
	client := newTestClient(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		requests = append(requests, req.URL.Path)
		switch req.URL.Path {
		case "/client/v4/user/tokens/verify":
			return response(http.StatusOK, `{"success":true}`), nil
		case "/client/v4/zones/zone/dns_records":
			return response(http.StatusOK, `{"success":true,"result":[]}`), nil
		default:
			return response(http.StatusNotFound, `{"success":false}`), nil
		}
	}))

	require.NoError(t, client.ValidateTokenAndZone())
	require.Equal(t, []string{"/client/v4/user/tokens/verify", "/client/v4/zones/zone/dns_records"}, requests)
}

func TestClient_ValidateTokenAndZone_requiresZone(t *testing.T) {
	client := &Client{apiToken: "token", http: &http.Client{}}
	require.ErrorContains(t, client.ValidateTokenAndZone(), "Zone name or Zone ID is required")
}

func TestClient_ListRecords_paginatesAllPages(t *testing.T) {
	calls := 0
	client := newTestClient(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			require.Contains(t, req.URL.RawQuery, "page=1")
			return response(http.StatusOK, `{"success":true,"result":[{"id":"one","name":"one.example.com","type":"A","content":"192.0.2.1"}],"result_info":{"page":1,"total_pages":2}}`), nil
		}
		require.Contains(t, req.URL.RawQuery, "page=2")
		return response(http.StatusOK, `{"success":true,"result":[{"id":"two","name":"two.example.com","type":"AAAA","content":"2001:db8::1"}],"result_info":{"page":2,"total_pages":2}}`), nil
	}))
	records, err := client.ListRecords()
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, 2, calls)
}

func TestClient_CreateDNSRecord_encodesMXAndCAA(t *testing.T) {
	priority := 10
	requests := make([]map[string]any, 0, 2)
	client := newTestClient(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		var payload map[string]any
		require.NoError(t, json.NewDecoder(req.Body).Decode(&payload))
		requests = append(requests, payload)
		return response(http.StatusOK, `{"success":true,"result":{"id":"rec"}}`), nil
	}))
	_, err := client.CreateDNSRecord(domain.DNSRecordInput{Type: "MX", Name: "example.com", Content: "mail.example.com", TTL: 300, Priority: &priority}, "")
	require.NoError(t, err)
	_, err = client.CreateDNSRecord(domain.DNSRecordInput{Type: "CAA", Name: "example.com", TTL: 300, CAA: &domain.CAARecordData{Flags: 0, Tag: "issue", Value: "letsencrypt.org"}}, "")
	require.NoError(t, err)
	require.Equal(t, float64(10), requests[0]["priority"])
	data := requests[1]["data"].(map[string]any)
	require.Equal(t, "issue", data["tag"])
}

func TestClient_CreateDNSRecord_rejectsProxyForMX(t *testing.T) {
	proxied := true
	client := newTestClient(roundTripperFunc(func(*http.Request) (*http.Response, error) { t.Fatal("request must not be sent"); return nil, nil }))
	_, err := client.CreateDNSRecord(domain.DNSRecordInput{Type: "MX", Name: "example.com", Content: "mail.example.com", TTL: 300, Proxied: &proxied}, "")
	require.Error(t, err)
}
