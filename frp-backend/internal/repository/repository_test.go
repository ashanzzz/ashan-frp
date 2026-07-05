package repository

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	_ "modernc.org/sqlite"

	"ashan-frp/internal/domain"
)

// setupTestDB creates an in-memory SQLite database with all tables auto-migrated.
func setupTestDB(t *testing.T) *gorm.DB {
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
	)
	require.NoError(t, err)
	return db
}

func Test_Repository_FindAccountByLogin(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	acc := domain.Account{
		ID:           domain.NewID("acc"),
		LoginName:    "testuser",
		DisplayName:  "Test User",
		PasswordHash: "hash:abc123",
		Role:         "admin",
	}
	err := db.Create(&acc).Error
	require.NoError(t, err)

	t.Run("returns account when login exists", func(t *testing.T) {
		// When
		found, err := repo.FindAccountByLogin("testuser")

		// Then
		require.NoError(t, err)
		require.NotNil(t, found)
		assert.Equal(t, acc.ID, found.ID)
		assert.Equal(t, "testuser", found.LoginName)
	})

	t.Run("returns error when login does not exist", func(t *testing.T) {
		// When
		_, err := repo.FindAccountByLogin("nonexistent")

		// Then
		require.Error(t, err)
	})
}

func Test_Repository_FindAccountByID(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	acc := domain.Account{
		ID:           domain.NewID("acc"),
		LoginName:    "findme",
		PasswordHash: "hash:abc",
		Role:         "admin",
	}
	err := db.Create(&acc).Error
	require.NoError(t, err)

	t.Run("returns account when ID exists", func(t *testing.T) {
		found, err := repo.FindAccountByID(acc.ID)
		require.NoError(t, err)
		assert.Equal(t, acc.LoginName, found.LoginName)
	})

	t.Run("returns error when ID does not exist", func(t *testing.T) {
		_, err := repo.FindAccountByID("nonexistent_id")
		require.Error(t, err)
	})
}

func Test_Repository_UpdateAccount(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	acc := domain.Account{
		ID:          domain.NewID("acc"),
		LoginName:   "update_me",
		PasswordHash: "hash:old",
		Role:        "admin",
	}
	err := db.Create(&acc).Error
	require.NoError(t, err)

	// When
	acc.DisplayName = "Updated Name"
	err = repo.UpdateAccount(&acc)
	require.NoError(t, err)

	// Then
	var saved domain.Account
	err = db.First(&saved, "id = ?", acc.ID).Error
	require.NoError(t, err)
	assert.Equal(t, "Updated Name", saved.DisplayName)
}

func Test_Repository_CreateAuthToken_FindAuthTokenByHash(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	acc := domain.Account{
		ID:           domain.NewID("acc"),
		LoginName:    "tokenuser",
		PasswordHash: "hash:tok",
		Role:         "admin",
	}
	require.NoError(t, db.Create(&acc).Error)

	token := &domain.AuthToken{
		ID:        domain.NewID("tok"),
		AccountID: acc.ID,
		TokenType: "session",
		TokenHash: "tok_" + domain.NewID("x"),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	t.Run("creates and finds by hash", func(t *testing.T) {
		err := repo.CreateAuthToken(token)
		require.NoError(t, err)

		found, err := repo.FindAuthTokenByHash(token.TokenHash)
		require.NoError(t, err)
		assert.Equal(t, token.ID, found.ID)
	})

	t.Run("find returns error for unknown hash", func(t *testing.T) {
		_, err := repo.FindAuthTokenByHash("tok_unknown")
		require.Error(t, err)
	})
}

func Test_Repository_RevokeAuthToken(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	acc := domain.Account{
		ID: domain.NewID("acc"), LoginName: "revoker", PasswordHash: "hash:r", Role: "admin",
	}
	require.NoError(t, db.Create(&acc).Error)

	token := &domain.AuthToken{
		ID:        domain.NewID("tok"),
		AccountID: acc.ID,
		TokenType: "session",
		TokenHash: "tok_revoke_test",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	require.NoError(t, db.Create(token).Error)

	// When
	err := repo.RevokeAuthToken(token.ID)
	require.NoError(t, err)

	// Then
	var saved domain.AuthToken
	err = db.First(&saved, "id = ?", token.ID).Error
	require.NoError(t, err)
	require.NotNil(t, saved.RevokedAt)
	assert.True(t, saved.IsRevoked())
	assert.False(t, saved.IsValid())
}

func Test_Repository_ListAuthTokensByAccount(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	acc := domain.Account{
		ID: domain.NewID("acc"), LoginName: "listtok", PasswordHash: "hash:lt", Role: "admin",
	}
	require.NoError(t, db.Create(&acc).Error)

	for i := 0; i < 3; i++ {
		tok := &domain.AuthToken{
			ID:        domain.NewID("tok"),
			AccountID: acc.ID,
			TokenType: "session",
			TokenHash: "tok_list_" + domain.NewID("x"),
			ExpiresAt: time.Now().Add(24 * time.Hour),
		}
		require.NoError(t, db.Create(tok).Error)
	}

	// When
	tokens, err := repo.ListAuthTokensByAccount(acc.ID)
	require.NoError(t, err)
	assert.Len(t, tokens, 3)
}

func Test_Repository_TouchAuthToken(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	acc := domain.Account{
		ID: domain.NewID("acc"), LoginName: "touch", PasswordHash: "hash:t", Role: "admin",
	}
	require.NoError(t, db.Create(&acc).Error)
	token := &domain.AuthToken{
		ID:        domain.NewID("tok"),
		AccountID: acc.ID,
		TokenType: "session",
		TokenHash: "tok_touch",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	require.NoError(t, db.Create(token).Error)

	// When
	time.Sleep(1 * time.Millisecond)
	err := repo.TouchAuthToken(token.ID)
	require.NoError(t, err)

	// Then
	var saved domain.AuthToken
	err = db.First(&saved, "id = ?", token.ID).Error
	require.NoError(t, err)
	require.NotNil(t, saved.LastUsedAt)
	assert.True(t, saved.LastUsedAt.After(token.CreatedAt))
}

func Test_Repository_CreateTunnel_FindTunnelByID(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	tunnel := &domain.Tunnel{
		ID:            domain.NewID("tun"),
		ProjectName:   "test-proj",
		Subdomain:     "test",
		FullDomain:    "test.335356119.xyz",
		Protocol:      "http",
		LocalIP:       "127.0.0.1",
		LocalPort:     3000,
		DesiredState:  domain.TunnelDesiredEnabled,
		ActualState:   domain.TunnelActualPending,
		ChmlfrpNode:   "node_hk",
	}

	err := repo.CreateTunnel(tunnel)
	require.NoError(t, err)

	t.Run("find by ID returns tunnel", func(t *testing.T) {
		found, err := repo.FindTunnelByID(tunnel.ID)
		require.NoError(t, err)
		assert.Equal(t, tunnel.ProjectName, found.ProjectName)
	})

	t.Run("find by ID returns error for missing", func(t *testing.T) {
		_, err := repo.FindTunnelByID("nonexistent")
		require.Error(t, err)
	})
}

func Test_Repository_FindTunnelByDomain(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	tunnel := &domain.Tunnel{
		ID:           domain.NewID("tun"),
		ProjectName:  "domain-test",
		FullDomain:   "unique.335356119.xyz",
		Protocol:     "tcp",
		LocalIP:      "127.0.0.1",
		LocalPort:    4000,
		DesiredState: domain.TunnelDesiredEnabled,
		ActualState:  domain.TunnelActualPending,
	}
	require.NoError(t, repo.CreateTunnel(tunnel))

	t.Run("finds by exact domain", func(t *testing.T) {
		found, err := repo.FindTunnelByDomain("unique.335356119.xyz")
		require.NoError(t, err)
		assert.Equal(t, tunnel.ID, found.ID)
	})

	t.Run("error for unknown domain", func(t *testing.T) {
		_, err := repo.FindTunnelByDomain("nonexistent.335356119.xyz")
		require.Error(t, err)
	})
}

func Test_Repository_UpdateTunnel(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	tunnel := &domain.Tunnel{
		ID:           domain.NewID("tun"),
		ProjectName:  "update-me",
		Protocol:     "tcp",
		LocalIP:      "127.0.0.1",
		LocalPort:    5000,
		DesiredState: domain.TunnelDesiredEnabled,
		ActualState:  domain.TunnelActualPending,
	}
	require.NoError(t, repo.CreateTunnel(tunnel))

	// When
	tunnel.DesiredState = domain.TunnelDesiredDisabled
	tunnel.StateReason = "disabled by test"
	err := repo.UpdateTunnel(tunnel)
	require.NoError(t, err)

	// Then
	var saved domain.Tunnel
	err = db.First(&saved, "id = ?", tunnel.ID).Error
	require.NoError(t, err)
	assert.Equal(t, domain.TunnelDesiredDisabled, saved.DesiredState)
	assert.Equal(t, "disabled by test", saved.StateReason)
}

func Test_Repository_DeleteTunnel(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	tunnel := &domain.Tunnel{
		ID:           domain.NewID("tun"),
		ProjectName:  "delete-me",
		Protocol:     "tcp",
		LocalIP:      "127.0.0.1",
		LocalPort:    6000,
		DesiredState: domain.TunnelDesiredEnabled,
		ActualState:  domain.TunnelActualPending,
	}
	require.NoError(t, repo.CreateTunnel(tunnel))

	// When
	err := repo.DeleteTunnel(tunnel.ID)
	require.NoError(t, err)

	// Then
	var count int64
	db.Model(&domain.Tunnel{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func Test_Repository_CountTunnels(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	for i := 0; i < 3; i++ {
		tun := &domain.Tunnel{
			ID:           domain.NewID("tun"),
			ProjectName:  "count-test",
			Protocol:     "tcp",
			LocalIP:      "127.0.0.1",
			LocalPort:    7000 + i,
			DesiredState: domain.TunnelDesiredEnabled,
			ActualState:  domain.TunnelActualPending,
		}
		require.NoError(t, db.Create(tun).Error)
	}
	// Archived tunnel should not be counted
	archived := &domain.Tunnel{
		ID:           domain.NewID("tun"),
		ProjectName:  "archived",
		Protocol:     "tcp",
		LocalIP:      "127.0.0.1",
		LocalPort:    9999,
		DesiredState: "archived",
		ActualState:  "archived",
	}
	require.NoError(t, db.Create(archived).Error)

	// When
	count, err := repo.CountTunnels()
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)
}

func Test_Repository_ListTunnels(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	states := []string{domain.TunnelDesiredEnabled, domain.TunnelDesiredDisabled, domain.TunnelDesiredEnabled}
	for i, s := range states {
		tun := &domain.Tunnel{
			ID:           domain.NewID("tun"),
			ProjectName:  "list-test",
			Protocol:     "tcp",
			LocalIP:      "127.0.0.1",
			LocalPort:    8000 + i,
			DesiredState: s,
			ActualState:  domain.TunnelActualPending,
		}
		require.NoError(t, db.Create(tun).Error)
	}

	t.Run("returns all non-archived tunnels with empty filter", func(t *testing.T) {
		tunnels, err := repo.ListTunnels(TunnelFilter{})
		require.NoError(t, err)
		assert.Len(t, tunnels, 3)
	})

	t.Run("filters by status", func(t *testing.T) {
		enabled, err := repo.ListTunnels(TunnelFilter{Status: domain.TunnelDesiredEnabled})
		require.NoError(t, err)
		assert.Len(t, enabled, 2)
	})
}

func Test_Repository_CreateJob_FindJobByID(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	job := &domain.Job{
		ID:       domain.NewID("job"),
		Kind:     "test_job",
		Status:   domain.JobStatusQueued,
		Title:    "Test job",
		Retryable: true,
		MaxAttempts: 3,
	}
	err := repo.CreateJob(job)
	require.NoError(t, err)

	t.Run("find by ID returns job", func(t *testing.T) {
		found, err := repo.FindJobByID(job.ID)
		require.NoError(t, err)
		assert.Equal(t, job.Title, found.Title)
		assert.Equal(t, domain.JobStatusQueued, found.Status)
	})

	t.Run("find by ID returns error for missing", func(t *testing.T) {
		_, err := repo.FindJobByID("nonexistent")
		require.Error(t, err)
	})
}

func Test_Repository_UpdateJob(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	job := &domain.Job{
		ID:     domain.NewID("job"),
		Kind:   "update_test",
		Status: domain.JobStatusQueued,
		Title:  "Update me",
	}
	require.NoError(t, repo.CreateJob(job))

	job.Status = domain.JobStatusRunning
	now := time.Now()
	job.StartedAt = &now
	err := repo.UpdateJob(job)
	require.NoError(t, err)

	var saved domain.Job
	err = db.First(&saved, "id = ?", job.ID).Error
	require.NoError(t, err)
	assert.Equal(t, domain.JobStatusRunning, saved.Status)
	require.NotNil(t, saved.StartedAt)
}

func Test_Repository_CountJobsByStatus(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	statuses := []string{
		domain.JobStatusQueued, domain.JobStatusQueued, domain.JobStatusRunning,
		domain.JobStatusFailed, domain.JobStatusSucceeded,
	}
	for i, s := range statuses {
		j := &domain.Job{
			ID:     domain.NewID("job"),
			Kind:   "count_test",
			Status: s,
			Title:  "Job " + s,
		}
		require.NoError(t, db.Create(j).Error)
		_ = i
	}

	count, err := repo.CountJobsByStatus(domain.JobStatusQueued)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func Test_Repository_ListJobs(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	for i := 0; i < 5; i++ {
		j := &domain.Job{
			ID:     domain.NewID("job"),
			Kind:   "list_test",
			Status: domain.JobStatusQueued,
			Title:  "Job",
		}
		require.NoError(t, db.Create(j).Error)
	}

	t.Run("returns all queued jobs", func(t *testing.T) {
		jobs, err := repo.ListJobs(JobFilter{Status: domain.JobStatusQueued})
		require.NoError(t, err)
		assert.Len(t, jobs, 5)
	})

	t.Run("returns empty for unknown status", func(t *testing.T) {
		jobs, err := repo.ListJobs(JobFilter{Status: domain.JobStatusRunning})
		require.NoError(t, err)
		assert.Len(t, jobs, 0)
	})
}

func Test_Repository_CreateAuditLog_ListAuditLogs(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)

	log1 := &domain.AuditLog{
		ID:           domain.NewID("aud"),
		AccountID:    "acc_1",
		AccountName:  "user1",
		Action:       "tunnel.create",
		ResourceType: "tunnel",
		ResourceID:   "tun_1",
	}
	log2 := &domain.AuditLog{
		ID:           domain.NewID("aud"),
		AccountID:    "acc_2",
		AccountName:  "user2",
		Action:       "tunnel.delete",
		ResourceType: "tunnel",
		ResourceID:   "tun_2",
	}
	require.NoError(t, repo.CreateAuditLog(log1))
	require.NoError(t, repo.CreateAuditLog(log2))

	logs, err := repo.ListAuditLogs(10)
	require.NoError(t, err)
	assert.Len(t, logs, 2)
}

func Test_Repository_Setting_CRUD(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)

	t.Run("GetSetting returns error for missing key", func(t *testing.T) {
		_, err := repo.GetSetting("nonexistent")
		require.Error(t, err)
	})

	t.Run("SetSetting creates or updates", func(t *testing.T) {
		err := repo.SetSetting("test_key", `{"value":42}`)
		require.NoError(t, err)

		s, err := repo.GetSetting("test_key")
		require.NoError(t, err)
		assert.Equal(t, "test_key", s.Key)
		assert.Equal(t, `{"value":42}`, s.ValueJSON)
	})

	t.Run("SetSetting updates existing", func(t *testing.T) {
		err := repo.SetSetting("test_key", `{"value":99}`)
		require.NoError(t, err)

		s, err := repo.GetSetting("test_key")
		require.NoError(t, err)
		assert.Equal(t, `{"value":99}`, s.ValueJSON)
	})

	t.Run("GetAllSettings returns all", func(t *testing.T) {
		err := repo.SetSetting("key_a", `"a"`)
		require.NoError(t, err)
		err = repo.SetSetting("key_b", `"b"`)
		require.NoError(t, err)

		all, err := repo.GetAllSettings()
		require.NoError(t, err)
		// Should have key_a, key_b, test_key
		assert.True(t, len(all) >= 3)
	})
}

func Test_Repository_Credential_CRUD(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)

	t.Run("FindCredentialByProvider returns error for missing", func(t *testing.T) {
		_, err := repo.FindCredentialByProvider("chmlfrp")
		require.Error(t, err)
	})

	t.Run("UpsertCredential creates new", func(t *testing.T) {
		cred := &domain.UpstreamCredential{
			ID:              domain.NewID("cre"),
			Provider:        "chmlfrp",
			Identifier:      "myuser",
			EncryptedSecret: "secret",
			MaskHint:        "my***rp",
		}
		err := repo.UpsertCredential(cred)
		require.NoError(t, err)

		found, err := repo.FindCredentialByProvider("chmlfrp")
		require.NoError(t, err)
		assert.Equal(t, "myuser", found.Identifier)
	})

	t.Run("UpsertCredential updates existing", func(t *testing.T) {
		cred, err := repo.FindCredentialByProvider("chmlfrp")
		require.NoError(t, err)
		cred.Identifier = "newuser"
		err = repo.UpsertCredential(cred)
		require.NoError(t, err)

		found, err := repo.FindCredentialByProvider("chmlfrp")
		require.NoError(t, err)
		assert.Equal(t, "newuser", found.Identifier)
	})

	t.Run("ListCredentials returns all", func(t *testing.T) {
		creds, err := repo.ListCredentials()
		require.NoError(t, err)
		assert.True(t, len(creds) >= 1)
	})
}

func Test_Repository_Snapshot_CRUD(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	snap := &domain.Snapshot{
		ID:           domain.NewID("snp"),
		Provider:     "cloudflare",
		ResourceType: "dns_record",
		ExternalID:   "cf_rec_1",
		PayloadJSON:  `{"name":"test.example.com"}`,
	}
	err := repo.CreateSnapshot(snap)
	require.NoError(t, err)

	t.Run("ListSnapshotsByProvider returns snapshots", func(t *testing.T) {
		snaps, err := repo.ListSnapshotsByProvider("cloudflare", 10)
		require.NoError(t, err)
		assert.Len(t, snaps, 1)
		assert.Equal(t, "cf_rec_1", snaps[0].ExternalID)
	})

	t.Run("empty list for unknown provider", func(t *testing.T) {
		snaps, err := repo.ListSnapshotsByProvider("unknown", 10)
		require.NoError(t, err)
		assert.Len(t, snaps, 0)
	})
}

func Test_Repository_SyncState_CRUD(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	state := &domain.SyncState{
		ID:                domain.NewID("syn"),
		LocalResourceType: "tunnel",
		LocalResourceID:   "tun_abc",
		ExternalProvider:  "chmlfrp",
		ExternalID:        "chml_123",
		Status:            "synced",
		LastCheckedAt:     time.Now(),
	}
	err := repo.UpsertSyncState(state)
	require.NoError(t, err)

	t.Run("UpsertSyncState creates and FindSyncStateByLocal finds", func(t *testing.T) {
		found, err := repo.FindSyncStateByLocal("tunnel", "tun_abc")
		require.NoError(t, err)
		assert.Equal(t, "synced", found.Status)
		assert.Equal(t, "chml_123", found.ExternalID)
	})

	t.Run("FindSyncStateByLocal returns error for missing", func(t *testing.T) {
		_, err := repo.FindSyncStateByLocal("tunnel", "nonexistent")
		require.Error(t, err)
	})

	t.Run("UpsertSyncState updates existing", func(t *testing.T) {
		state.Status = "diff_found"
		err := repo.UpsertSyncState(state)
		require.NoError(t, err)

		found, _ := repo.FindSyncStateByLocal("tunnel", "tun_abc")
		assert.Equal(t, "diff_found", found.Status)
	})

	t.Run("ListSyncStates returns all", func(t *testing.T) {
		states, err := repo.ListSyncStates()
		require.NoError(t, err)
		assert.Len(t, states, 1)
	})
}

func Test_Repository_ListTunnels_filters_by_protocol_and_node(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)

	tunnels := []domain.Tunnel{
		{ID: domain.NewID("tun"), ProjectName: "a", Protocol: "http", LocalIP: "127.0.0.1", LocalPort: 1, DesiredState: domain.TunnelDesiredEnabled, ActualState: domain.TunnelActualPending, ChmlfrpNode: "hk"},
		{ID: domain.NewID("tun"), ProjectName: "b", Protocol: "tcp", LocalIP: "127.0.0.1", LocalPort: 2, DesiredState: domain.TunnelDesiredEnabled, ActualState: domain.TunnelActualPending, ChmlfrpNode: "hk"},
		{ID: domain.NewID("tun"), ProjectName: "c", Protocol: "tcp", LocalIP: "127.0.0.1", LocalPort: 3, DesiredState: domain.TunnelDesiredEnabled, ActualState: domain.TunnelActualPending, ChmlfrpNode: "xz"},
	}
	for _, tun := range tunnels {
		require.NoError(t, db.Create(&tun).Error)
	}

	t.Run("filter by protocol", func(t *testing.T) {
		result, err := repo.ListTunnels(TunnelFilter{Protocol: "tcp"})
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("filter by chmlfrp_node", func(t *testing.T) {
		result, err := repo.ListTunnels(TunnelFilter{ChmlfrpNode: "hk"})
		require.NoError(t, err)
		assert.Len(t, result, 2)
	})

	t.Run("filter by all fields", func(t *testing.T) {
		result, err := repo.ListTunnels(TunnelFilter{Status: domain.TunnelDesiredEnabled, Protocol: "tcp", ChmlfrpNode: "xz"})
		require.NoError(t, err)
		assert.Len(t, result, 1)
		assert.Equal(t, "c", result[0].ProjectName)
	})
}

func Test_Repository_ListJobs_filters_by_target(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)

	for i := 0; i < 3; i++ {
		j := &domain.Job{
			ID: domain.NewID("job"), Kind: "provision_tunnel",
			Status: domain.JobStatusQueued, Title: "Provision",
			TargetType: "tunnel", TargetID: "tun_abc",
		}
		require.NoError(t, db.Create(j).Error)
	}
	other := &domain.Job{
		ID: domain.NewID("job"), Kind: "reconcile",
		Status: domain.JobStatusQueued, Title: "Reconcile",
	}
	require.NoError(t, db.Create(other).Error)

	jobs, err := repo.ListJobs(JobFilter{TargetType: "tunnel", TargetID: "tun_abc"})
	require.NoError(t, err)
	assert.Len(t, jobs, 3)
}

func Test_Repository_FindAccountByLogin_is_case_sensitive(t *testing.T) {
	db := setupTestDB(t)
	repo := New(db)
	acc := domain.Account{
		ID: domain.NewID("acc"), LoginName: "ExactCase", PasswordHash: "hash:c", Role: "admin",
	}
	require.NoError(t, db.Create(&acc).Error)

	_, err := repo.FindAccountByLogin("exactcase")
	require.Error(t, err, "should be case-sensitive")

	found, err := repo.FindAccountByLogin("ExactCase")
	require.NoError(t, err)
	assert.Equal(t, acc.ID, found.ID)
}
