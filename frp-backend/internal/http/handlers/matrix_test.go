package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
)

func TestMatrixHandler_GetMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupHandlerDB(t)
	repo := repository.New(db)
	key := []byte("01234567890123456789012345678901")

	// Create test node
	require.NoError(t, repo.CreateNode(&domain.Node{
		ID:           "node_bj",
		DisplayName:  "北京多线",
		RealIP:       "82.156.215.91",
		Status:       domain.NodeStatusActive,
		HealthStatus: domain.HealthOnline,
	}))

	// Create test tunnel
	require.NoError(t, repo.CreateTunnel(&domain.Tunnel{
		ID:                "tun_drive",
		Name:              "drive",
		FullDomain:        "drive.335356119.xyz",
		Protocol:          "https",
		LocalIP:           "192.168.8.11",
		LocalPort:         30004,
		ChmlfrpNode:       "北京多线",
		ChmlfrpTunnelName: "drive",
		DesiredState:      "enabled",
		ActualState:       "running",
	}))

	h := NewMatrixHandler(repo, key)
	r := gin.New()
	r.GET("/api/v1/matrix", h.GetMatrix)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/matrix", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var env domain.ResponseEnvelope
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	data := env.Data.(map[string]any)
	assert.Equal(t, float64(1), data["total"])

	items := data["matrix"].([]any)
	assert.Len(t, items, 1)

	item := items[0].(map[string]any)
	assert.Equal(t, "drive", item["name"])
	assert.Equal(t, "drive.335356119.xyz", item["full_domain"])
	assert.Equal(t, "HTTPS", item["protocol"])
	assert.Equal(t, float64(30004), item["local_port"])

	chml := item["chmlfrp"].(map[string]any)
	assert.Equal(t, "北京多线", chml["node_name"])
	assert.Equal(t, "82.156.215.91", chml["node_ip"])
	assert.Equal(t, true, chml["node_online"])
}
