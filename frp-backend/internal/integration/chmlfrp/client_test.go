package chmlfrp

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"ashan-frp/internal/domain"
	"github.com/stretchr/testify/require"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func newTestClient(rt http.RoundTripper) *Client {
	client := NewClient("user", "pass")
	client.token = "token"
	client.userID = 42
	client.http = &http.Client{Transport: rt}
	return client
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestClient_GetNodes_returnsErrorOnTransportFailure(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return nil, errors.New("boom")
	}))

	_, err := client.GetNodes()
	require.Error(t, err)
	require.Contains(t, err.Error(), "boom")
}

func TestClient_Login_returnsErrorWhenCodeIsNotSuccess(t *testing.T) {
	client := NewClient("user", "pass")
	client.http = &http.Client{Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		require.Equal(t, "/api/login.php", req.URL.Path)
		return response(http.StatusOK, `{"code":500,"token":"abc","userid":7,"error":"bad"}`), nil
	})}

	err := client.Login()
	require.Error(t, err)
}

func TestClient_GetTunnels_returnsErrorOnInvalidJSON(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, "not-json"), nil
	}))

	_, err := client.GetTunnels()
	require.Error(t, err)
}

func TestClient_CreateTunnel_returnsErrorWhenTunnelLookupFails(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/api/tunnel.php":
			return response(http.StatusOK, "created"), nil
		case "/api/usertunnel.php":
			return nil, errors.New("lookup failed")
		default:
			return nil, errors.New("unexpected request")
		}
	}))

	_, err := client.CreateTunnel(domain.ChmlFrpCreateTunnelReq{
		TunnelName: "demo",
		PortType:   "tcp",
		Node:       "1",
		LocalIP:    "127.0.0.1",
		LocalPort:  8080,
		RemotePort: 9000,
	})
	require.Error(t, err)
}

func TestClient_DeleteTunnel_returnsErrorOnHTTPStatus(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusInternalServerError, "fail"), nil
	}))

	err := client.DeleteTunnel("123")
	require.Error(t, err)
}

func TestClient_GetConfig_returnsErrorOnInvalidJSON(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, "not-json"), nil
	}))

	_, err := client.GetConfig("node-a")
	require.Error(t, err)
}

func TestClient_GetConfig_returnsErrorWhenAPISignalsFailure(t *testing.T) {
	client := newTestClient(roundTripperFunc(func(*http.Request) (*http.Response, error) {
		return response(http.StatusOK, `{"success":false,"message":"bad"}`), nil
	}))

	_, err := client.GetConfig("node-a")
	require.Error(t, err)
}
