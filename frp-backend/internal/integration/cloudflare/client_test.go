package cloudflare

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
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
