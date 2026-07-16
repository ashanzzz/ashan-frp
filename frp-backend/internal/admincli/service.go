package admincli

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

var (
	ErrNoAdministrator  = errors.New("no administrator account found")
	ErrMultipleAdmins   = errors.New("multiple administrator accounts found")
	ErrPasswordTooShort = errors.New("password must contain at least 8 characters")
	ErrUsernameInvalid  = errors.New("username must contain 1 to 64 characters")
)

type ResetRequest struct {
	NewUsername string
	NewPassword string
}

type ResetResult struct {
	AccountID     string
	LoginName     string
	RevokedTokens int64
}

func ResetPassword(repo *repository.Repository, request ResetRequest) (ResetResult, error) {
	newUsername, err := validateUsername(request.NewUsername)
	if err != nil {
		return ResetResult{}, err
	}
	if utf8.RuneCountInString(request.NewPassword) < 8 {
		return ResetResult{}, ErrPasswordTooShort
	}

	accounts, err := repo.ListAdminAccounts()
	if err != nil {
		return ResetResult{}, fmt.Errorf("find administrator: %w", err)
	}
	if len(accounts) == 0 {
		return ResetResult{}, ErrNoAdministrator
	}
	if len(accounts) > 1 {
		return ResetResult{}, ErrMultipleAdmins
	}
	account := accounts[0]

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
