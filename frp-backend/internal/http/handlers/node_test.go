package handlers

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
)

func TestNodeHandler_Create(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	input := domain.NodeInput{
		DisplayName: "us-east-1",
		Provider:    "aws",
		NodeType:    "ec2",
		EndpointURL: "https://node.example.com:8080",
		Region:      "us-east-1",
		Metadata:    map[string]any{"tier": "production"},
	}
	c, w := newGinContext(http.MethodPost, "/api/v1/nodes", input)
	h.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	require.Nil(t, env.Error, "unexpected error: %+v", env.Error)

	node, ok := env.Data.(map[string]any)
	require.True(t, ok, "expected map, got %T", env.Data)

	assert.Contains(t, node["id"], "node_")
	assert.Equal(t, "us-east-1", node["display_name"])
	assert.Equal(t, "aws", node["provider"])
	assert.Equal(t, "ec2", node["node_type"])
	assert.Equal(t, "us-east-1", node["region"])
	assert.Equal(t, string(domain.NodeStatusActive), node["status"])
	assert.Equal(t, string(domain.HealthUnknown), node["health_status"])
	assert.NotEmpty(t, node["created_at"])
	assert.NotEmpty(t, node["updated_at"])

	// Verify metadata round-trips.
	meta, ok := node["metadata"].(map[string]any)
	require.True(t, ok, "metadata should be map")
	assert.Equal(t, "production", meta["tier"])
}

func TestNodeHandler_Create_DefaultsStatusToActive(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	input := domain.NodeInput{
		DisplayName: "default-status",
		Provider:    "aws",
		NodeType:    "ec2",
	}
	c, w := newGinContext(http.MethodPost, "/api/v1/nodes", input)
	h.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	node := env.Data.(map[string]any)
	assert.Equal(t, string(domain.NodeStatusActive), node["status"])
}

func TestNodeHandler_Create_ExplicitStatus(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	input := domain.NodeInput{
		DisplayName: "explicit-status",
		Provider:    "aws",
		NodeType:    "ec2",
		Status:      string(domain.NodeStatusDisabled),
	}
	c, w := newGinContext(http.MethodPost, "/api/v1/nodes", input)
	h.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	node := env.Data.(map[string]any)
	assert.Equal(t, string(domain.NodeStatusDisabled), node["status"])
}

func TestNodeHandler_Create_InvalidJSON(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	c, w := newGinContext(http.MethodPost, "/api/v1/nodes", "not-json")
	h.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	require.NotNil(t, env.Error)
	assert.Equal(t, "INVALID_REQUEST", env.Error.Code)
}

func TestNodeHandler_List_Empty(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	c, w := newGinContext(http.MethodGet, "/api/v1/nodes", nil)
	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	require.Nil(t, env.Error)

	data, ok := env.Data.(map[string]any)
	require.True(t, ok)
	nodes, ok := data["nodes"].([]any)
	require.True(t, ok)
	assert.Empty(t, nodes)
}

func TestNodeHandler_List_ReturnsCreated(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	// Seed two nodes.
	_ = repo.CreateNode(&domain.Node{ID: "node_a", DisplayName: "A", Provider: "aws", NodeType: "ec2", Status: domain.NodeStatusActive})
	_ = repo.CreateNode(&domain.Node{ID: "node_b", DisplayName: "B", Provider: "gcp", NodeType: "vm", Status: domain.NodeStatusDisabled})

	c, w := newGinContext(http.MethodGet, "/api/v1/nodes", nil)
	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	data := env.Data.(map[string]any)
	nodes := data["nodes"].([]any)
	assert.Len(t, nodes, 2)
}

func TestNodeHandler_List_FilterByProvider(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	_ = repo.CreateNode(&domain.Node{ID: "node_a", DisplayName: "A", Provider: "aws", NodeType: "ec2"})
	_ = repo.CreateNode(&domain.Node{ID: "node_b", DisplayName: "B", Provider: "gcp", NodeType: "vm"})

	c, w := newGinContext(http.MethodGet, "/api/v1/nodes?provider=aws", nil)
	c.Request.URL.RawQuery = "provider=aws"
	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	data := env.Data.(map[string]any)
	nodes := data["nodes"].([]any)
	assert.Len(t, nodes, 1)
	assert.Equal(t, "node_a", nodes[0].(map[string]any)["id"])
}

func TestNodeHandler_List_FilterByQ(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	_ = repo.CreateNode(&domain.Node{ID: "node_x", DisplayName: "match-me", Provider: "aws", NodeType: "ec2", EndpointURL: "https://x.com"})
	_ = repo.CreateNode(&domain.Node{ID: "node_y", DisplayName: "skip-me", Provider: "gcp", NodeType: "vm", EndpointURL: "https://y.com"})

	c, w := newGinContext(http.MethodGet, "/api/v1/nodes?q=match", nil)
	c.Request.URL.RawQuery = "q=match"
	h.List(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	data := env.Data.(map[string]any)
	nodes := data["nodes"].([]any)
	assert.Len(t, nodes, 1)
	assert.Equal(t, "node_x", nodes[0].(map[string]any)["id"])
}

func TestNodeHandler_Get_Found(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	_ = repo.CreateNode(&domain.Node{ID: "node_1", DisplayName: "test", Provider: "aws", NodeType: "ec2"})

	c, w := newGinContext(http.MethodGet, "/api/v1/nodes/node_1", nil)
	c.Params = gin.Params{{Key: "id", Value: "node_1"}}
	h.Get(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	require.Nil(t, env.Error)
	node := env.Data.(map[string]any)
	assert.Equal(t, "node_1", node["id"])
}

func TestNodeHandler_Get_NotFound(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	c, w := newGinContext(http.MethodGet, "/api/v1/nodes/nonexistent", nil)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}
	h.Get(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	require.NotNil(t, env.Error)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

func TestNodeHandler_Update(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	_ = repo.CreateNode(&domain.Node{ID: "node_u1", DisplayName: "old", Provider: "aws", NodeType: "ec2", Status: domain.NodeStatusActive})

	input := domain.NodeInput{DisplayName: "new"}
	c, w := newGinContext(http.MethodPatch, "/api/v1/nodes/node_u1", input)
	c.Params = gin.Params{{Key: "id", Value: "node_u1"}}
	h.Update(c)

	assert.Equal(t, http.StatusOK, w.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	require.Nil(t, env.Error)
	node := env.Data.(map[string]any)
	assert.Equal(t, "new", node["display_name"])
	// Provider should be preserved.
	assert.Equal(t, "aws", node["provider"])
}

func TestNodeHandler_Update_NotFound(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	input := domain.NodeInput{DisplayName: "ghost"}
	c, w := newGinContext(http.MethodPatch, "/api/v1/nodes/ghost", input)
	c.Params = gin.Params{{Key: "id", Value: "ghost"}}
	h.Update(c)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	require.NotNil(t, env.Error)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

func TestNodeHandler_Update_InvalidJSON(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	_ = repo.CreateNode(&domain.Node{ID: "node_u2", DisplayName: "x", Provider: "aws", NodeType: "ec2"})

	c, w := newGinContext(http.MethodPatch, "/api/v1/nodes/node_u2", "bad")
	c.Params = gin.Params{{Key: "id", Value: "node_u2"}}
	h.Update(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	require.NotNil(t, env.Error)
	assert.Equal(t, "INVALID_REQUEST", env.Error.Code)
}

func TestNodeHandler_Sync(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	c, w := newGinContext(http.MethodPost, "/api/v1/nodes/sync", nil)
	h.Sync(c)

	assert.Equal(t, http.StatusAccepted, w.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	require.Nil(t, env.Error)
	require.NotNil(t, env.Meta.Job)
	assert.NotEmpty(t, env.Meta.Job.ID)
	assert.Equal(t, domain.JobStatusQueued, env.Meta.Job.Status)
	assert.Equal(t, "node.refresh", env.Meta.Job.Kind)
	assert.Equal(t, "nodes", env.Meta.Job.TargetType)
	assert.Equal(t, "nodes", env.Meta.Job.TargetID)
	assert.Equal(t, "subject:nodes", env.Meta.Job.Channel)

	var saved domain.Job
	require.NoError(t, db.First(&saved, "id = ?", env.Meta.Job.ID).Error)
	assert.Equal(t, env.Meta.Job.ID, saved.ID)
	assert.Equal(t, domain.JobStatusQueued, saved.Status)
	assert.Equal(t, "node.refresh", saved.Kind)
	assert.Equal(t, "nodes", saved.TargetType)
	assert.Equal(t, "nodes", saved.TargetID)
}

func TestNodeHandler_Create_RequiresJSON(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	// JSON with wrong type for required string field.
	c, w := newGinContext(http.MethodPost, "/api/v1/nodes", map[string]any{
		"display_name": 123,
	})
	h.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ---------- round-trip: create + get ----------

func TestNodeHandler_CreateThenGet(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewNodeHandler(repo)

	input := domain.NodeInput{
		DisplayName: "rt-node",
		Provider:    "azure",
		NodeType:    "vmss",
		Metadata: map[string]any{
			"os":   "linux",
			"size": "Standard_D2s_v3",
		},
	}
	c, w := newGinContext(http.MethodPost, "/api/v1/nodes", input)
	h.Create(c)
	assert.Equal(t, http.StatusCreated, w.Code)

	var env domain.ResponseEnvelope
	json.Unmarshal(w.Body.Bytes(), &env)
	nodeData := env.Data.(map[string]any)
	id := nodeData["id"].(string)

	// GET
	c2, w2 := newGinContext(http.MethodGet, "/api/v1/nodes/"+id, nil)
	c2.Params = gin.Params{{Key: "id", Value: id}}
	h.Get(c2)
	assert.Equal(t, http.StatusOK, w2.Code)

	json.Unmarshal(w2.Body.Bytes(), &env)
	got := env.Data.(map[string]any)
	assert.Equal(t, id, got["id"])
	assert.Equal(t, "rt-node", got["display_name"])

	meta := got["metadata"].(map[string]any)
	assert.Equal(t, "linux", meta["os"])
	assert.Equal(t, "Standard_D2s_v3", meta["size"])
}
