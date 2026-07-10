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

func TestWebsiteMappingHandler_Create(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewWebsiteMappingHandler(repo)

	input := domain.WebsiteMappingInput{
		SourceKind:    "chmlfrp",
		NodeID:        "node_hk",
		PrimaryDomain: "blog.335356119.xyz",
		Domains:       []string{"blog.335356119.xyz", "www.blog.335356119.xyz"},
		HTTPSEnabled:  true,
		ProxyEnabled:  true,
		ProxyTarget:   "http://localhost:3000",
		HTTPConfig:    map[string]any{"cache_ttl": 3600},
	}
	c, w := newGinContext(http.MethodPost, "/api/v1/website-mappings", input)
	h.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)

	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	require.Nil(t, env.Error, "unexpected error: %+v", env.Error)

	wm, ok := env.Data.(map[string]any)
	require.True(t, ok)
	assert.Contains(t, wm["id"], "web_")
	assert.Equal(t, "chmlfrp", wm["source_kind"])
	assert.Equal(t, "node_hk", wm["node_id"])
	assert.Equal(t, "blog.335356119.xyz", wm["primary_domain"])
	assert.Equal(t, true, wm["https_enabled"])
	assert.Equal(t, true, wm["proxy_enabled"])
	assert.Equal(t, "http://localhost:3000", wm["proxy_target"])
	assert.Equal(t, "pending", wm["status"])

	// Domains should round-trip via JSON column.
	domains, ok := wm["domains"].([]any)
	require.True(t, ok)
	assert.Len(t, domains, 2)
	assert.Equal(t, "blog.335356119.xyz", domains[0])

	// HTTPConfig should also round-trip.
	hcfg, ok := wm["http_config"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, float64(3600), hcfg["cache_ttl"])
}

func TestWebsiteMappingHandler_Create_InvalidJSON(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewWebsiteMappingHandler(repo)

	c, w := newGinContext(http.MethodPost, "/api/v1/website-mappings", "bad")
	h.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	require.NotNil(t, env.Error)
	assert.Equal(t, "INVALID_REQUEST", env.Error.Code)
}

func TestWebsiteMappingHandler_Create_DefaultsStatus(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewWebsiteMappingHandler(repo)

	input := domain.WebsiteMappingInput{
		SourceKind:    "manual",
		NodeID:        "node_xz",
		PrimaryDomain: "default.335356119.xyz",
		Domains:       []string{"default.335356119.xyz"},
	}
	c, w := newGinContext(http.MethodPost, "/api/v1/website-mappings", input)
	h.Create(c)

	assert.Equal(t, http.StatusCreated, w.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, w, &env)
	require.Nil(t, env.Error)
	wm := env.Data.(map[string]any)
	assert.Equal(t, "pending", wm["status"])
	assert.Equal(t, false, wm["https_enabled"])
	assert.Equal(t, false, wm["proxy_enabled"])
}

func TestWebsiteMappingHandler_Get_Found(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewWebsiteMappingHandler(repo)

	wm := domain.WebsiteMapping{ID: "web_1", SourceKind: "chmlfrp", NodeID: "n1", PrimaryDomain: "get.335356119.xyz", Domains: []string{"get.335356119.xyz"}, Status: domain.WebsiteStatusSynced}
	require.NoError(t, repo.CreateWebsiteMapping(&wm))

	c, rec := newGinContext(http.MethodGet, "/api/v1/website-mappings/web_1", nil)
	c.Params = gin.Params{{Key: "id", Value: "web_1"}}
	h.Get(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, rec, &env)
	require.Nil(t, env.Error)
	got := env.Data.(map[string]any)
	assert.Equal(t, "web_1", got["id"])
	assert.Equal(t, "get.335356119.xyz", got["primary_domain"])
	assert.Equal(t, string(domain.WebsiteStatusSynced), got["status"])
}

func TestWebsiteMappingHandler_Get_NotFound(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewWebsiteMappingHandler(repo)

	c, rec := newGinContext(http.MethodGet, "/api/v1/website-mappings/nonexistent", nil)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}
	h.Get(c)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, rec, &env)
	require.NotNil(t, env.Error)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

func TestWebsiteMappingHandler_List_Empty(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewWebsiteMappingHandler(repo)

	c, rec := newGinContext(http.MethodGet, "/api/v1/website-mappings", nil)
	h.List(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, rec, &env)
	require.Nil(t, env.Error)
	data := env.Data.(map[string]any)
	mappings, ok := data["website_mappings"].([]any)
	require.True(t, ok)
	assert.Empty(t, mappings)
}

func TestWebsiteMappingHandler_List_ReturnsCreated(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewWebsiteMappingHandler(repo)

	_ = repo.CreateWebsiteMapping(&domain.WebsiteMapping{ID: "web_a", SourceKind: "chmlfrp", NodeID: "n1", PrimaryDomain: "a.xyz", Domains: []string{"a.xyz"}, Status: domain.WebsiteStatusSynced})
	_ = repo.CreateWebsiteMapping(&domain.WebsiteMapping{ID: "web_b", SourceKind: "manual", NodeID: "n1", PrimaryDomain: "b.xyz", Domains: []string{"b.xyz"}, Status: domain.WebsiteStatusPending})

	c, rec := newGinContext(http.MethodGet, "/api/v1/website-mappings", nil)
	h.List(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, rec, &env)
	data := env.Data.(map[string]any)
	mappings := data["website_mappings"].([]any)
	assert.Len(t, mappings, 2)
}

func TestWebsiteMappingHandler_List_FilterByNodeID(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewWebsiteMappingHandler(repo)

	_ = repo.CreateWebsiteMapping(&domain.WebsiteMapping{ID: "web_a", SourceKind: "chmlfrp", NodeID: "node_hk", PrimaryDomain: "a.xyz", Domains: []string{"a.xyz"}, Status: domain.WebsiteStatusSynced})
	_ = repo.CreateWebsiteMapping(&domain.WebsiteMapping{ID: "web_b", SourceKind: "manual", NodeID: "node_xz", PrimaryDomain: "b.xyz", Domains: []string{"b.xyz"}, Status: domain.WebsiteStatusSynced})

	c, rec := newGinContext(http.MethodGet, "/api/v1/website-mappings?node_id=node_hk", nil)
	c.Request.URL.RawQuery = "node_id=node_hk"
	h.List(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, rec, &env)
	data := env.Data.(map[string]any)
	mappings := data["website_mappings"].([]any)
	assert.Len(t, mappings, 1)
	assert.Equal(t, "web_a", mappings[0].(map[string]any)["id"])
}

func TestWebsiteMappingHandler_List_FilterByStatus(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewWebsiteMappingHandler(repo)

	_ = repo.CreateWebsiteMapping(&domain.WebsiteMapping{ID: "web_a", SourceKind: "chmlfrp", NodeID: "n1", PrimaryDomain: "a.xyz", Domains: []string{"a.xyz"}, Status: domain.WebsiteStatusSynced})
	_ = repo.CreateWebsiteMapping(&domain.WebsiteMapping{ID: "web_b", SourceKind: "manual", NodeID: "n1", PrimaryDomain: "b.xyz", Domains: []string{"b.xyz"}, Status: domain.WebsiteStatusPending})

	c, rec := newGinContext(http.MethodGet, "/api/v1/website-mappings?status=synced", nil)
	c.Request.URL.RawQuery = "status=synced"
	h.List(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, rec, &env)
	data := env.Data.(map[string]any)
	mappings := data["website_mappings"].([]any)
	assert.Len(t, mappings, 1)
	assert.Equal(t, "web_a", mappings[0].(map[string]any)["id"])
}

func TestWebsiteMappingHandler_Update(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewWebsiteMappingHandler(repo)

	wm := domain.WebsiteMapping{ID: "web_upd", SourceKind: "chmlfrp", NodeID: "n1", PrimaryDomain: "old.xyz", Domains: []string{"old.xyz"}, Status: domain.WebsiteStatusSynced}
	require.NoError(t, repo.CreateWebsiteMapping(&wm))

	input := domain.WebsiteMappingInput{
		PrimaryDomain: "updated.xyz",
		Domains:       []string{"updated.xyz", "new.xyz"},
		HTTPSEnabled:  true,
	}
	c, rec := newGinContext(http.MethodPatch, "/api/v1/website-mappings/web_upd", input)
	c.Params = gin.Params{{Key: "id", Value: "web_upd"}}
	h.Update(c)

	assert.Equal(t, http.StatusOK, rec.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, rec, &env)
	require.Nil(t, env.Error)
	got := env.Data.(map[string]any)
	assert.Equal(t, "updated.xyz", got["primary_domain"])
	// SourceKind should be preserved.
	assert.Equal(t, "chmlfrp", got["source_kind"])
	// Status resets to pending on update.
	assert.Equal(t, "pending", got["status"])

	// Verify domains round-trip.
	domains, ok := got["domains"].([]any)
	require.True(t, ok)
	assert.Len(t, domains, 2)
}

func TestWebsiteMappingHandler_Update_NotFound(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewWebsiteMappingHandler(repo)

	input := domain.WebsiteMappingInput{PrimaryDomain: "ghost.xyz", Domains: []string{"ghost.xyz"}}
	c, rec := newGinContext(http.MethodPatch, "/api/v1/website-mappings/nonexistent", input)
	c.Params = gin.Params{{Key: "id", Value: "nonexistent"}}
	h.Update(c)

	assert.Equal(t, http.StatusNotFound, rec.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, rec, &env)
	require.NotNil(t, env.Error)
	assert.Equal(t, "NOT_FOUND", env.Error.Code)
}

func TestWebsiteMappingHandler_Update_InvalidJSON(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewWebsiteMappingHandler(repo)

	wm := domain.WebsiteMapping{ID: "web_inv", SourceKind: "manual", NodeID: "n1", PrimaryDomain: "inv.xyz", Domains: []string{"inv.xyz"}, Status: domain.WebsiteStatusSynced}
	require.NoError(t, repo.CreateWebsiteMapping(&wm))

	c, rec := newGinContext(http.MethodPatch, "/api/v1/website-mappings/web_inv", "bad")
	c.Params = gin.Params{{Key: "id", Value: "web_inv"}}
	h.Update(c)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, rec, &env)
	require.NotNil(t, env.Error)
	assert.Equal(t, "INVALID_REQUEST", env.Error.Code)
}

func TestWebsiteMappingHandler_Sync(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewWebsiteMappingHandler(repo)

	wm := domain.WebsiteMapping{ID: "m1", SourceKind: "chmlfrp", NodeID: "node-1", PrimaryDomain: "sync.example", Domains: []string{"sync.example"}, Status: domain.WebsiteStatusPending}
	require.NoError(t, repo.CreateWebsiteMapping(&wm))

	c, rec := newGinContext(http.MethodPost, "/api/v1/website-mappings/m1/sync", nil)
	c.Params = gin.Params{{Key: "id", Value: "m1"}}
	h.Sync(c)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	var env domain.ResponseEnvelope
	jsonDecodeResp(t, rec, &env)
	require.Nil(t, env.Error)
	require.NotNil(t, env.Meta.Job)
	assert.NotEmpty(t, env.Meta.Job.ID)
	assert.Equal(t, domain.JobStatusQueued, env.Meta.Job.Status)
	assert.Equal(t, "website_mapping.sync", env.Meta.Job.Kind)
	assert.Equal(t, "website_mappings", env.Meta.Job.TargetType)
	assert.Equal(t, wm.ID, env.Meta.Job.TargetID)
	assert.Equal(t, "subject:website-mappings", env.Meta.Job.Channel)

	var saved domain.Job
	require.NoError(t, db.First(&saved, "id = ?", env.Meta.Job.ID).Error)
	assert.Equal(t, env.Meta.Job.ID, saved.ID)
	assert.Equal(t, domain.JobStatusQueued, saved.Status)
	assert.Equal(t, "website_mapping.sync", saved.Kind)
	assert.Equal(t, "website_mappings", saved.TargetType)
	assert.Equal(t, wm.ID, saved.TargetID)
}

func TestWebsiteMappingHandler_CreateThenGet(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewWebsiteMappingHandler(repo)

	input := domain.WebsiteMappingInput{
		SourceKind:   "chmlfrp",
		NodeID:       "node_hk",
		PrimaryDomain: "rt.335356119.xyz",
		Domains:      []string{"rt.335356119.xyz"},
		HTTPSEnabled: true,
		HTTPConfig:   map[string]any{"redirect_www": true},
	}
	c, w := newGinContext(http.MethodPost, "/api/v1/website-mappings", input)
	h.Create(c)
	assert.Equal(t, http.StatusCreated, w.Code)

	var env domain.ResponseEnvelope
	json.Unmarshal(w.Body.Bytes(), &env)
	wmData := env.Data.(map[string]any)
	id := wmData["id"].(string)

	// GET back.
	c2, w2 := newGinContext(http.MethodGet, "/api/v1/website-mappings/"+id, nil)
	c2.Params = gin.Params{{Key: "id", Value: id}}
	h.Get(c2)
	assert.Equal(t, http.StatusOK, w2.Code)

	json.Unmarshal(w2.Body.Bytes(), &env)
	got := env.Data.(map[string]any)
	assert.Equal(t, id, got["id"])
	assert.Equal(t, "rt.335356119.xyz", got["primary_domain"])

	domains, ok := got["domains"].([]any)
	require.True(t, ok)
	assert.Len(t, domains, 1)
	assert.Equal(t, "rt.335356119.xyz", domains[0])

	hcfg, ok := got["http_config"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, true, hcfg["redirect_www"])
}

func TestWebsiteMappingHandler_Create_WrongType(t *testing.T) {
	db := setupHandlerDB(t)
	repo := repository.New(db)
	h := NewWebsiteMappingHandler(repo)

	// Send an integer where a string is expected — triggers ShouldBindJSON error.
	c, w := newGinContext(http.MethodPost, "/api/v1/website-mappings", map[string]any{
		"source_kind": 123,
		"primary_domain": "test.xyz",
	})
	h.Create(c)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
