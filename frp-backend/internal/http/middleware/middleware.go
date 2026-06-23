package middleware

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
)

func AuthMiddleware(repo *repository.Repository) gin.HandlerFunc {
	publicPaths := map[string]bool{
		"/api/v1/health":     true,
		"/api/v1/version":    true,
		"/api/v1/auth/login": true,
	}
	return func(c *gin.Context) {
		if publicPaths[c.Request.URL.Path] {
			c.Next()
			return
		}
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenHash := authHeader[7:]
			token, err := repo.FindAuthTokenByHash(tokenHash)
			if err != nil || !token.IsValid() {
				c.AbortWithStatusJSON(http.StatusUnauthorized, domain.ResponseEnvelope{
					Error: &domain.APIError{Code: "UNAUTHORIZED", Message: "Invalid or expired token"},
				})
				return
			}
			_ = repo.TouchAuthToken(token.ID)
			acc, _ := repo.FindAccountByID(token.AccountID)
			if acc != nil {
				c.Set("account", acc)
				c.Set("account_id", acc.ID)
				c.Set("account_role", acc.Role)
			}
			c.Next()
			return
		}
		cookie, err := c.Cookie("ashan_frp_session")
		if err == nil && cookie != "" {
			token, err := repo.FindAuthTokenByHash(cookie)
			if err == nil && token.IsValid() {
				_ = repo.TouchAuthToken(token.ID)
				acc, _ := repo.FindAccountByID(token.AccountID)
				if acc != nil {
					c.Set("account", acc)
					c.Set("account_id", acc.ID)
					c.Set("account_role", acc.Role)
				}
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "UNAUTHORIZED", Message: "Authentication required"},
		})
	}
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = domain.NewID("req")
		}
		traceID := c.GetHeader("X-Trace-ID")
		if traceID == "" {
			traceID = domain.NewID("trc")
		}
		c.Set("request_id", requestID)
		c.Set("trace_id", traceID)
		c.Header("X-Request-ID", requestID)
		c.Header("X-Trace-ID", traceID)
		c.Next()
	}
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		_ = time.Since(start)
	}
}
