package onepanel

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
	return &Client{baseURL: "http://example.test", apiToken: "token", http: &http.Client{Transport: rt}}
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Status: http.StatusText(status), Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
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
