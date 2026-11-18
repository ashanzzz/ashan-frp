package onepanel

import (
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func newTestClient(rt http.RoundTripper) *Client {
	return &Client{baseURL: "http://example.test", apiToken: "test_secret_key", http: &http.Client{Transport: rt}}
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestClient_Headers_IncludesMD5Signature(t *testing.T) {
	client := NewClient("http://192.168.8.11:10086", "my_api_key")
	headers := client.headers()

	require.NotEmpty(t, headers["1Panel-Token"])
	require.NotEmpty(t, headers["1Panel-Timestamp"])
	require.Equal(t, "application/json", headers["Content-Type"])

	ts := headers["1Panel-Timestamp"]
	expectedHash := md5.Sum([]byte(fmt.Sprintf("1panel%s%s", "my_api_key", ts)))
	expectedToken := hex.EncodeToString(expectedHash[:])
	require.Equal(t, expectedToken, headers["1Panel-Token"])
}

func TestClient_CreateProxyWebsite_Success(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/v2/websites", req.URL.Path)
		require.Equal(t, "POST", req.Method)

		var payload map[string]any
		err := json.NewDecoder(req.Body).Decode(&payload)
		require.NoError(t, err)

		require.Equal(t, "drive", payload["primaryDomain"])
		require.Equal(t, "proxy", payload["type"])
		require.Equal(t, "http://192.168.8.11:8666", payload["proxy"])

		domains, ok := payload["domains"].([]any)
		require.True(t, ok)
		require.Len(t, domains, 1)

		d0 := domains[0].(map[string]any)
		require.Equal(t, float64(30004), d0["port"])

		return response(http.StatusOK, `{"code":200,"message":"success","data":{"id":10}}`), nil
	}))

	id, err := client.CreateProxyWebsite("drive", "http://192.168.8.11:8666", 30004, true, 16)
	require.NoError(t, err)
	require.Equal(t, 10, id)
}

func TestClient_CreateWebsite_returnsErrorOnInvalidJSON(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, "not-json"), nil
	}))

	_, err := client.CreateWebsite("demo.example", "127.0.0.1:8080", false)
	require.Error(t, err)
}

func TestClient_ListWebsites_returnsErrorWhenAPIReportsFailure(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"code":500,"message":"bad"}`), nil
	}))

	_, err := client.ListWebsites()
	require.Error(t, err)
}

func TestClient_EnableSSL_returnsErrorOnHTTPStatus(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusInternalServerError, `{"code":500,"message":"bad"}`), nil
	}))

	err := client.EnableSSL(1, "demo.example")
	require.Error(t, err)
}

func TestClient_DeleteWebsite_Success(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/v2/websites/del", req.URL.Path)
		require.Equal(t, "POST", req.Method)
		return response(http.StatusOK, `{"code":200,"message":"success"}`), nil
	}))

	err := client.DeleteWebsite(10)
	require.NoError(t, err)
}

func TestClient_DeleteWebsite_returnsErrorOnTransportFailure(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	}))

	err := client.DeleteWebsite(1)
	require.Error(t, err)
}

func TestClient_TestConnection_returnsErrorWhenAPIReportsFailure(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"code":500,"message":"bad"}`), nil
	}))

	err := client.TestConnection()
	require.Error(t, err)
}
