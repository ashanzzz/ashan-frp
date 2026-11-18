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

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
)

type mockFailoverSupervisor struct {
	settings domain.FailoverSettings
	status   domain.FailoverStatus
}

func (m *mockFailoverSupervisor) GetStatus() domain.FailoverStatus {
	return m.status
}

func (m *mockFailoverSupervisor) GetSettings() domain.FailoverSettings {
	return m.settings
}

func (m *mockFailoverSupervisor) SaveSettings(s domain.FailoverSettings) error {
	m.settings = s
	return nil
}

func (m *mockFailoverSupervisor) TriggerFailoverManual(reason string) (*domain.FailoverStatus, error) {
	st := m.status
	return &st, nil
}

func (m *mockFailoverSupervisor) OptimizeWeeklyBestNode(reason string, force bool) (*domain.FailoverStatus, error) {
	st := m.status
	return &st, nil
}

func setupFailoverHandlerTest(t *testing.T) (*repository.Repository, *mockFailoverSupervisor, *FailoverHandler) {
	t.Helper()
	db, err := gorm.Open(sqlite.New(sqlite.Config{DriverName: "sqlite", DSN: ":memory:"}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
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
	))
	repo := repository.New(db)
	mockSup := &mockFailoverSupervisor{
		settings: domain.FailoverSettings{AutoFailoverEnabled: true, OfflineThresholdMin: 10},
		status:   domain.FailoverStatus{AutoFailoverEnabled: true, TrackedNode: "北京多线", TrackedNodeOnline: true},
	}
	h := NewFailoverHandler(mockSup, repo)
	return repo, mockSup, h
}

func TestFailoverHandler_GetStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, _, h := setupFailoverHandlerTest(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/api/v1/failover/status", nil)

	h.GetStatus(c)
	require.Equal(t, http.StatusOK, w.Code)

	var resp domain.ResponseEnvelope
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Nil(t, resp.Error)
}

func TestFailoverHandler_UpdateSettings(t *testing.T) {
	gin.SetMode(gin.TestMode)
	_, _, h := setupFailoverHandlerTest(t)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	payload := domain.FailoverSettings{
		AutoFailoverEnabled:  true,
		OfflineThresholdMin:  45,
		HeartbeatIntervalSec: 20,
	}
	body, _ := json.Marshal(payload)
	c.Request = httptest.NewRequest("POST", "/api/v1/failover/settings", bytes.NewReader(body))

	h.UpdateSettings(c)
	require.Equal(t, http.StatusOK, w.Code)

	// Fetch settings to verify
	w2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(w2)
	c2.Request = httptest.NewRequest("GET", "/api/v1/failover/settings", nil)
	h.GetSettings(c2)
	require.Equal(t, http.StatusOK, w2.Code)
}
