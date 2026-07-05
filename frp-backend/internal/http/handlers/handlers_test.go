package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "modernc.org/sqlite"

	"ashan-frp/internal/config"
	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
)

// setupHandlerDB creates an in-memory SQLite database with all domain tables
// auto-migrated, including Node and WebsiteMapping.
func setupHandlerDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: ":memory:"}), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	require.NoError(t, err)
	err = db.AutoMigrate(
		&domain.Account{},
		&domain.AuthToken{},
		&domain.UpstreamCredential{},
		&domain.Tunnel{},
		&domain.Job{},
		&domain.AuditLog{},
		&domain.Snapshot{},
		&domain.SyncState{},
		&domain.Setting{},
		&domain.Node{},
		&domain.WebsiteMapping{},
	)
	require.NoError(t, err)
	return db
}

// newGinContext returns a gin.Context backed by an httptest.ResponseRecorder
// with optional JSON body. Sets account_id for auth-dependent handlers.
func newGinContext(method, target string, body any) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request, _ = http.NewRequest(method, target, nil)
	if body != nil {
		data, _ := json.Marshal(body)
		c.Request, _ = http.NewRequest(method, target, bytes.NewReader(data))
		c.Request.Header.Set("Content-Type", "application/json")
	}
	// Set a test account so audit() calls do not panic on missing account_id.
	c.Set("account_id", "acc_test")
	return c, w
}

// jsonDecodeResp unmarshals the JSON response body into v.
func jsonDecodeResp(t *testing.T, w *httptest.ResponseRecorder, v any) {
	t.Helper()
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), v))
}

func newTunnelHandlerTestRepo(t *testing.T) (*repository.Repository, *gorm.DB) {
	t.Helper()
	db := setupHandlerDB(t)
	repo := repository.New(db)
	return repo, db
}

func seedTestAccount(t *testing.T, db *gorm.DB) domain.Account {
	t.Helper()
	acc := domain.Account{ID: domain.NewID("acc"), LoginName: "tester", PasswordHash: "hash:test", Role: "admin"}
	require.NoError(t, db.Create(&acc).Error)
	return acc
}

func Test_TunnelHandler_Provision_returns_real_job_summary_and_persists_state(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo, db := newTunnelHandlerTestRepo(t)
	acc := seedTestAccount(t, db)
	cfg := config.Config{BaseDomain: "example.com"}
	h := NewTunnelHandler(cfg, repo)
	tunnel := domain.Tunnel{
		ID:           domain.NewID("tun"),
		ProjectName:  "demo",
		Subdomain:    "demo",
		FullDomain:   "demo.example.com",
		Protocol:     "tcp",
		LocalIP:      "127.0.0.1",
		LocalPort:    7000,
		RemotePort:   17000,
		DesiredState: "enabled",
		ActualState:  "pending",
		CreatedBy:    acc.ID,
	}
	require.NoError(t, db.Create(&tunnel).Error)

	c, w := newGinContext(http.MethodPost, "/api/v1/tunnels/"+tunnel.ID+"/actions/provision", nil)
	c.Params = gin.Params{{Key: "id", Value: tunnel.ID}}
	c.Set("account_id", acc.ID)

	// When
	h.Provision(c)

	// Then
	require.Equal(t, http.StatusAccepted, w.Code)
	var envelope domain.ResponseEnvelope
	jsonDecodeResp(t, w, &envelope)
	require.NotNil(t, envelope.Meta.Job)
	require.NotEmpty(t, envelope.Meta.Job.ID)
	require.Equal(t, "queued", envelope.Meta.Job.Status)
	require.Equal(t, "provision_tunnel", envelope.Meta.Job.Kind)
	require.Equal(t, "tunnel", envelope.Meta.Job.TargetType)
	require.Equal(t, tunnel.ID, envelope.Meta.Job.TargetID)

	var savedTunnel domain.Tunnel
	require.NoError(t, db.First(&savedTunnel, "id = ?", tunnel.ID).Error)
	require.Equal(t, "provisioning", savedTunnel.ActualState)
	require.Equal(t, "Job "+envelope.Meta.Job.ID+" queued", savedTunnel.StateReason)

	var savedJob domain.Job
	require.NoError(t, db.First(&savedJob, "id = ?", envelope.Meta.Job.ID).Error)
	require.Equal(t, domain.JobStatusQueued, savedJob.Status)
	require.Equal(t, tunnel.ID, savedJob.TargetID)
	require.Equal(t, acc.ID, savedJob.CreatedBy)
}
