package domain

// ---- Account (GORM model) ----

type Account struct {
	ID             string     `json:"id" gorm:"primaryKey;size:20"`
	LoginName      string     `json:"login_name" gorm:"uniqueIndex;size:64;not null"`
	DisplayName    string     `json:"display_name" gorm:"size:128"`
	PasswordHash   string     `json:"-" gorm:"size:256;not null"`
	Role           string     `json:"role" gorm:"size:32;not null;default:admin"`
	MustChangePwd  bool       `json:"must_change_password" gorm:"default:true"`
	FailedAttempts int        `json:"-" gorm:"default:0"`
	LockedUntil    *time.Time `json:"-"`
	LastLoginAt    *time.Time `json:"last_login_at"`
	LastIP         string     `json:"-" gorm:"size:45"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (Account) TableName() string { return "accounts" }

// ---- AuthToken (session / API token) ----

type AuthToken struct {
	ID         string     `json:"id" gorm:"primaryKey;size:20"`
	AccountID  string     `json:"account_id" gorm:"index;size:20;not null"`
	TokenType  string     `json:"token_type" gorm:"size:16;not null"`
	TokenHash  string     `json:"-" gorm:"size:256;not null"`
	Scopes     string     `json:"scopes" gorm:"size:512"`
	UserAgent  string     `json:"user_agent,omitempty" gorm:"size:512"`
	IPAddress  string     `json:"ip_address,omitempty" gorm:"size:45"`
	ExpiresAt  time.Time  `json:"expires_at"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

func (AuthToken) TableName() string { return "auth_tokens" }
func (t AuthToken) IsExpired() bool { return time.Now().After(t.ExpiresAt) }
func (t AuthToken) IsRevoked() bool { return t.RevokedAt != nil }
func (t AuthToken) IsValid() bool   { return !t.IsExpired() && !t.IsRevoked() }

// ---- UpstreamCredential ----

type UpstreamCredential struct {
	ID              string     `json:"id" gorm:"primaryKey;size:20"`
	Provider        string     `json:"provider" gorm:"size:32;not null"`
	Identifier      string     `json:"identifier" gorm:"size:256"`
	EncryptedSecret string     `json:"-" gorm:"type:text"`
	MaskHint        string     `json:"mask_hint" gorm:"size:64"`
	LastVerifiedAt  *time.Time `json:"last_verified_at,omitempty"`
	LastError       string     `json:"last_error,omitempty" gorm:"size:512"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (UpstreamCredential) TableName() string { return "upstream_credentials" }

// ---- AuditLog ----

type AuditLog struct {
	ID           string    `json:"id" gorm:"primaryKey;size:20"`
	AccountID    string    `json:"account_id" gorm:"size:20;index"`
	AccountName  string    `json:"account_name" gorm:"size:64"`
	Action       string    `json:"action" gorm:"size:128;not null"`
	ResourceType string    `json:"resource_type" gorm:"size:32"`
	ResourceID   string    `json:"resource_id" gorm:"size:20;index"`
	DetailJSON   string    `json:"detail_json,omitempty" gorm:"type:text"`
	IPAddress    string    `json:"ip_address" gorm:"size:45"`
	UserAgent    string    `json:"user_agent" gorm:"size:512"`
	CreatedAt    time.Time `json:"created_at"`
}

func (AuditLog) TableName() string { return "audit_logs" }

// ---- Snapshot ----

type Snapshot struct {
	ID           string    `json:"id" gorm:"primaryKey;size:20"`
	Provider     string    `json:"provider" gorm:"size:32;not null;index"`
	ResourceType string    `json:"resource_type" gorm:"size:32;not null"`
	ExternalID   string    `json:"external_id" gorm:"size:64;index"`
	PayloadJSON  string    `json:"payload_json" gorm:"type:text"`
	Hash         string    `json:"hash" gorm:"size:64"`
	CreatedAt    time.Time `json:"created_at"`
}

func (Snapshot) TableName() string { return "snapshots" }

// ---- SyncState ----

type SyncState struct {
	ID                 string     `json:"id" gorm:"primaryKey;size:20"`
	LocalResourceType  string     `json:"local_resource_type" gorm:"size:32;not null;index"`
	LocalResourceID    string     `json:"local_resource_id" gorm:"size:20;not null;index"`
	ExternalProvider   string     `json:"external_provider" gorm:"size:32;not null"`
	ExternalID         string     `json:"external_id" gorm:"size:64"`
	Status             string     `json:"status" gorm:"size:32;not null"`
	DiffJSON           string     `json:"diff_json,omitempty" gorm:"type:text"`
	LastCheckedAt      time.Time  `json:"last_checked_at"`
	LastReconciledAt   *time.Time `json:"last_reconciled_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (SyncState) TableName() string { return "sync_states" }

// ---- Setting ----

type Setting struct {
	Key       string    `json:"key" gorm:"primaryKey;size:128"`
	ValueJSON string    `json:"value_json" gorm:"type:text;not null"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Setting) TableName() string { return "settings" }

// ---- Login / Auth DTO ----

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Mode     string `json:"mode"`
}

type LoginResponse struct {
	Account AccountAuth `json:"account"`
	Auth    AuthInfo    `json:"auth"`
}

type AccountAuth struct {
	ID        string `json:"id"`
	LoginName string `json:"login_name"`
	Role      string `json:"role"`
}

type AuthInfo struct {
	Token     string    `json:"token,omitempty"`
	Mode      string    `json:"mode"`
	ExpiresAt time.Time `json:"expires_at"`
}

type PasswordChangeRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required,min=8"`
}
