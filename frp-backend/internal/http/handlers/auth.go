package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/argon2"

	"ashan-frp/internal/config"
	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
)

type AuthHandler struct {
	cfg  config.Config
	repo *repository.Repository
}

func NewAuthHandler(cfg config.Config, repo *repository.Repository) *AuthHandler {
	return &AuthHandler{cfg: cfg, repo: repo}
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req domain.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INVALID_REQUEST", Message: err.Error()},
		})
		return
	}
	acc, err := h.repo.FindAccountByLogin(req.Username)
	if err != nil {
		c.JSON(http.StatusUnauthorized, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "AUTH_FAILED", Message: "Invalid username or password"},
		})
		return
	}
	if acc.LockedUntil != nil && time.Now().Before(*acc.LockedUntil) {
		c.JSON(http.StatusTooManyRequests, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "ACCOUNT_LOCKED", Message: "Account temporarily locked"},
		})
		return
	}
	if !verifyPassword(req.Password, acc.PasswordHash) {
		acc.FailedAttempts++
		if acc.FailedAttempts >= 5 {
			lock := time.Now().Add(15 * time.Minute)
			acc.LockedUntil = &lock
		}
		_ = h.repo.UpdateAccount(acc)
		c.JSON(http.StatusUnauthorized, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "AUTH_FAILED", Message: "Invalid username or password"},
		})
		return
	}
	acc.FailedAttempts = 0
	acc.LockedUntil = nil
	now := time.Now()
	acc.LastLoginAt = &now
	acc.LastIP = c.ClientIP()
	_ = h.repo.UpdateAccount(acc)

	tokenValue := domain.NewID("tok") + domain.NewID("x")
	tokenHash := hashToken(tokenValue)
	token := &domain.AuthToken{
		ID:        domain.NewID("tok"),
		AccountID: acc.ID,
		TokenType: "session",
		TokenHash: tokenHash,
		IPAddress: c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	_ = h.repo.CreateAuthToken(token)
	c.SetCookie("ashan_frp_session", tokenHash, 86400, "/", "", false, true)

	h.audit(acc.ID, acc.LoginName, "login", "account", acc.ID, c)
	c.JSON(http.StatusOK, domain.ResponseEnvelope{
		Data: domain.LoginResponse{
			Account: domain.AccountAuth{ID: acc.ID, LoginName: acc.LoginName, Role: acc.Role},
			Auth:    domain.AuthInfo{Token: tokenValue, Mode: "session", ExpiresAt: token.ExpiresAt},
		},
	})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	cookie, _ := c.Cookie("ashan_frp_session")
	if cookie != "" {
		t, _ := h.repo.FindAuthTokenByHash(cookie)
		if t != nil {
			_ = h.repo.RevokeAuthToken(t.ID)
		}
	}
	c.SetCookie("ashan_frp_session", "", -1, "/", "", false, true)
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]string{"message": "Logged out"}})
}

func (h *AuthHandler) Me(c *gin.Context) {
	acc, _ := c.Get("account")
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: acc})
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	var req domain.PasswordChangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INVALID_REQUEST", Message: err.Error()},
		})
		return
	}
	accRaw, _ := c.Get("account")
	acc := accRaw.(*domain.Account)
	if !verifyPassword(req.OldPassword, acc.PasswordHash) {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INVALID_PASSWORD", Message: "Current password is incorrect"},
		})
		return
	}
	acc.PasswordHash = hashPassword(req.NewPassword)
	acc.MustChangePwd = false
	_ = h.repo.UpdateAccount(acc)
	h.audit(acc.ID, acc.LoginName, "password.change", "account", acc.ID, c)
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]string{"message": "Password changed"}})
}

func (h *AuthHandler) ListTokens(c *gin.Context) {
	accID := c.GetString("account_id")
	tokens, _ := h.repo.ListAuthTokensByAccount(accID)
	type dto struct {
		ID         string     "json:\"id\""
		TokenType  string     "json:\"token_type\""
		ExpiresAt  time.Time  "json:\"expires_at\""
		RevokedAt  *time.Time "json:\"revoked_at,omitempty\""
		LastUsedAt *time.Time "json:\"last_used_at,omitempty\""
	}
	dtos := make([]dto, 0, len(tokens))
	for _, t := range tokens {
		dtos = append(dtos, dto{ID: t.ID, TokenType: t.TokenType, ExpiresAt: t.ExpiresAt, RevokedAt: t.RevokedAt, LastUsedAt: t.LastUsedAt})
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: dtos})
}

func (h *AuthHandler) RevokeToken(c *gin.Context) {
	_ = h.repo.RevokeAuthToken(c.Param("id"))
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]string{"message": "Revoked"}})
}

func (h *AuthHandler) audit(accID, accName, action, resType, resID string, c *gin.Context) {
	_ = h.repo.CreateAuditLog(&domain.AuditLog{
		ID: domain.NewID("aud"), AccountID: accID, AccountName: accName,
		Action: action, ResourceType: resType, ResourceID: resID,
		IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
	})
}

func hashPassword(pwd string) string {
	return string(argon2.IDKey([]byte(pwd), []byte("ashan-frp-salt-v2"), 1, 64*1024, 4, 32))
}

func verifyPassword(pwd, hash string) bool {
	return hashPassword(pwd) == hash
}

func hashToken(tok string) string {
	return string(argon2.IDKey([]byte(tok), []byte("ashan-frp-token-salt"), 1, 64*1024, 4, 32))
}
