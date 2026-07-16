package repository

import (
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"ashan-frp/internal/domain"
)

type AccountCredentialReset struct {
	AccountID    string
	LoginName    string
	PasswordHash string
}

func (r *Repository) ListAdminAccounts() ([]domain.Account, error) {
	var accounts []domain.Account
	err := r.db.Where("role IN ?", []string{"admin", "super_admin"}).Order("login_name ASC").Find(&accounts).Error
	return accounts, err
}

func (r *Repository) ResetAdminCredentials(input AccountCredentialReset) (int64, error) {
	var revoked int64
	err := r.db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&domain.Account{}).Where("id = ?", input.AccountID).Updates(map[string]any{
			"login_name":      input.LoginName,
			"password_hash":   input.PasswordHash,
			"must_change_pwd": false,
			"failed_attempts": 0,
			"locked_until":    nil,
		})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return gorm.ErrRecordNotFound
		}

		now := time.Now()
		revokeResult := tx.Model(&domain.AuthToken{}).
			Where("account_id = ? AND revoked_at IS NULL", input.AccountID).
			Update("revoked_at", now)
		if revokeResult.Error != nil {
			return revokeResult.Error
		}
		revoked = revokeResult.RowsAffected

		detail, err := json.Marshal(map[string]any{
			"source":         "cli",
			"revoked_tokens": revoked,
		})
		if err != nil {
			return err
		}
		return tx.Create(&domain.AuditLog{
			ID:           domain.NewID("aud"),
			AccountID:    input.AccountID,
			AccountName:  input.LoginName,
			Action:       "password.reset.cli",
			ResourceType: "account",
			ResourceID:   input.AccountID,
			DetailJSON:   string(detail),
		}).Error
	})
	return revoked, err
}
