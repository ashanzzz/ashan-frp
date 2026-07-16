package admincli

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"gorm.io/gorm"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

var (
	ErrAccountNotFound  = errors.New("administrator account not found")
	ErrNotAdministrator = errors.New("account is not an administrator")
	ErrPasswordTooShort = errors.New("password must contain at least 8 characters")
	ErrUsernameInvalid  = errors.New("username must contain 1 to 64 characters")
	ErrUsernameTaken    = errors.New("username is already in use")
)

type ResetRequest struct {
	Username    string
	NewUsername string
	NewPassword string
}

type ResetResult struct {
	AccountID     string
	LoginName     string
	RevokedTokens int64
}

func ListAdministrators(repo *repository.Repository) ([]domain.Account, error) {
	return repo.ListAdminAccounts()
}

func ResetPassword(repo *repository.Repository, request ResetRequest) (ResetResult, error) {
	username, err := validateUsername(request.Username)
	if err != nil {
		return ResetResult{}, err
	}
	if utf8.RuneCountInString(request.NewPassword) < 8 {
		return ResetResult{}, ErrPasswordTooShort
	}

	account, err := repo.FindAccountByLogin(username)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ResetResult{}, ErrAccountNotFound
	}
	if err != nil {
		return ResetResult{}, fmt.Errorf("find administrator: %w", err)
	}
	if account.Role != "admin" && account.Role != "super_admin" {
		return ResetResult{}, ErrNotAdministrator
	}

	newUsername := username
	if strings.TrimSpace(request.NewUsername) != "" {
		newUsername, err = validateUsername(request.NewUsername)
		if err != nil {
			return ResetResult{}, err
		}
	}
	if newUsername != username {
		existing, lookupErr := repo.FindAccountByLogin(newUsername)
		if lookupErr == nil && existing.ID != account.ID {
			return ResetResult{}, ErrUsernameTaken
		}
		if lookupErr != nil && !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return ResetResult{}, fmt.Errorf("check new username: %w", lookupErr)
		}
	}

	hash, err := security.HashPassword(request.NewPassword)
	if err != nil {
		return ResetResult{}, fmt.Errorf("hash password: %w", err)
	}
	revoked, err := repo.ResetAdminCredentials(repository.AccountCredentialReset{
		AccountID:    account.ID,
		LoginName:    newUsername,
		PasswordHash: hash,
	})
	if err != nil {
		return ResetResult{}, fmt.Errorf("reset administrator credentials: %w", err)
	}
	return ResetResult{AccountID: account.ID, LoginName: newUsername, RevokedTokens: revoked}, nil
}

func validateUsername(value string) (string, error) {
	value = strings.TrimSpace(value)
	length := utf8.RuneCountInString(value)
	if length == 0 || length > 64 {
		return "", ErrUsernameInvalid
	}
	return value, nil
}
