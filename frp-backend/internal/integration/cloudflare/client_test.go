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
