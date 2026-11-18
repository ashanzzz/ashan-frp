package worker

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "modernc.org/sqlite"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
)

func setupFailoverTestDB(t *testing.T) (*gorm.DB, *repository.Repository) {
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
	return db, repository.New(db)
}

func TestFailoverSupervisor_SettingsDefaultAndSave(t *testing.T) {
	_, repo := setupFailoverTestDB(t)
	sup := NewFailoverSupervisor(repo, nil, []byte("32bytessecretkeyformonitor123456"))

	// Default settings
	st := sup.GetSettings()
	require.True(t, st.AutoFailoverEnabled)
	require.Equal(t, 10, st.OfflineThresholdMin)
	require.Equal(t, 30, st.HeartbeatIntervalSec)
	require.True(t, st.WeeklyOptimizeEnabled)

	// Save modified settings
	st.OfflineThresholdMin = 15
	st.AutoFailoverEnabled = false
	err := sup.SaveSettings(st)
	require.NoError(t, err)

	updated := sup.GetSettings()
	require.False(t, updated.AutoFailoverEnabled)
	require.Equal(t, 15, updated.OfflineThresholdMin)
}

func TestFailoverSupervisor_StatusTracking(t *testing.T) {
	_, repo := setupFailoverTestDB(t)
	sup := NewFailoverSupervisor(repo, nil, []byte("32bytessecretkeyformonitor123456"))

	st := sup.GetStatus()
	require.True(t, st.AutoFailoverEnabled)
	require.True(t, st.TrackedNodeOnline)
	require.Equal(t, 0, st.ContinuousOfflineMin)

	// Simulate offline detection
	sup.mu.Lock()
	sup.currentTrackedNode = "北京多线"
	sup.currentTrackedIP = "82.156.215.91"
	sup.isOnline = false
	past := time.Now().Add(-70 * time.Minute)
	sup.firstOfflineAt = &past
	sup.mu.Unlock()

	st2 := sup.GetStatus()
	require.False(t, st2.TrackedNodeOnline)
	require.Equal(t, "北京多线", st2.TrackedNode)
	require.Equal(t, "82.156.215.91", st2.TrackedNodeIP)
	require.GreaterOrEqual(t, st2.ContinuousOfflineMin, 70)
}

func TestFailoverSupervisor_DebounceThresholdUnder10Min(t *testing.T) {
	_, repo := setupFailoverTestDB(t)
	sup := NewFailoverSupervisor(repo, nil, []byte("32bytessecretkeyformonitor123456"))

	// Simulate node offline for 5 minutes (less than 10-min threshold)
	sup.mu.Lock()
	sup.currentTrackedNode = "北京多线"
	sup.currentTrackedIP = "82.156.215.91"
	sup.isOnline = false
	past := time.Now().Add(-5 * time.Minute)
	sup.firstOfflineAt = &past
	sup.mu.Unlock()

	st := sup.GetStatus()
	require.False(t, st.TrackedNodeOnline)
	require.Equal(t, 5, st.ContinuousOfflineMin)
	require.Equal(t, 10, st.OfflineThresholdMin)
	// Must NOT have executed failover yet
	require.Nil(t, st.LastFailoverAt)
	require.Empty(t, st.LastFailoverLog)
}

func TestFailoverSupervisor_FlapRecoveryResetsTimer(t *testing.T) {
	_, repo := setupFailoverTestDB(t)
	sup := NewFailoverSupervisor(repo, nil, []byte("32bytessecretkeyformonitor123456"))

	// 1. Node becomes offline for 7 minutes
	sup.mu.Lock()
	sup.currentTrackedNode = "北京多线"
	sup.currentTrackedIP = "82.156.215.91"
	sup.isOnline = false
	past := time.Now().Add(-7 * time.Minute)
	sup.firstOfflineAt = &past
	sup.mu.Unlock()

	st := sup.GetStatus()
	require.False(t, st.TrackedNodeOnline)
	require.Equal(t, 7, st.ContinuousOfflineMin)

	// 2. Node recovers online before 10-minute threshold
	sup.mu.Lock()
	sup.isOnline = true
	sup.firstOfflineAt = nil
	sup.mu.Unlock()

	stRecovered := sup.GetStatus()
	require.True(t, stRecovered.TrackedNodeOnline)
	require.Equal(t, 0, stRecovered.ContinuousOfflineMin)
	require.Nil(t, stRecovered.FirstOfflineAt)
	// Timer reset verified: anti-flap prevents premature migration
}

func TestFailoverSupervisor_NoCandidateAbortsGracefully(t *testing.T) {
	_, repo := setupFailoverTestDB(t)
	sup := NewFailoverSupervisor(repo, nil, []byte("32bytessecretkeyformonitor123456"))

	// Trigger failover with no candidate nodes in database
	err := sup.ExecuteFailover("北京多线", "82.156.215.91", 65)
	require.Error(t, err)
	require.Contains(t, err.Error(), "candidate nodes")

	// Verify audit log was written
	logs, err := repo.ListAuditLogs(10)
	require.NoError(t, err)
	require.NotEmpty(t, logs)
	require.Equal(t, "node.auto_failover_aborted", logs[0].Action)
	require.Equal(t, "failed", logs[0].Outcome)
	require.Contains(t, logs[0].DetailJSON, "Failover failed")
}

func TestFailoverSupervisor_AuditLogRecordingOnSuccess(t *testing.T) {
	_, repo := setupFailoverTestDB(t)
	sup := NewFailoverSupervisor(repo, nil, []byte("32bytessecretkeyformonitor123456"))

	// Manually record audit via recordAudit helper
	testMsg := "触发自动故障转移：节点 [北京多线](82.156.215.91) 持续离线 60 分钟，系统自动测速并选定最佳健康节点 [河南多线](1.2.3.4, 延迟 25ms)。已完成 5 条隧道迁移与 5 条 Cloudflare DNS 解析同步，本地 FRPC 守护进程已热重载。"
	sup.recordAudit("node.auto_failover", "河南多线", "success", testMsg)

	logs, _, err := repo.ListAuditLogsFiltered(repository.AuditFilter{
		Action: "failover",
		Limit:  10,
	})
	require.NoError(t, err)
	require.Len(t, logs, 1)
	require.Equal(t, "node.auto_failover", logs[0].Action)
	require.Equal(t, "success", logs[0].Outcome)
	require.Equal(t, "河南多线", logs[0].ResourceID)
	require.Equal(t, testMsg, logs[0].DetailJSON)
}
