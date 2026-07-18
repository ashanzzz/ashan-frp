package middleware

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

func AuthMiddleware(repo *repository.Repository) gin.HandlerFunc {
	publicPaths := map[string]bool{"/api/v1/health": true, "/api/v1/version": true, "/api/v1/auth/login": true}
	return func(c *gin.Context) {
		if publicPaths[c.Request.URL.Path] {
			c.Next()
			return
		}
		if authHeader := c.GetHeader("Authorization"); len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			token, err := repo.FindAuthTokenByHash(security.HashToken(authHeader[7:]))
			if err == nil && token.IsValid() {
				_ = repo.TouchAuthToken(token.ID)
				if acc, _ := repo.FindAccountByID(token.AccountID); acc != nil {
					c.Set("account", acc)
					c.Set("account_id", acc.ID)
					c.Set("account_name", acc.LoginName)
					c.Set("account_role", acc.Role)
				}
				c.Next()
				return
			}
			c.AbortWithStatusJSON(http.StatusUnauthorized, domain.ResponseEnvelope{Error: &domain.APIError{Code: "UNAUTHORIZED", Message: "Invalid or expired token"}})
			return
		}
		if cookie, err := c.Cookie("ashan_frp_session"); err == nil && cookie != "" {
			if token, err := repo.FindAuthTokenByHash(security.HashToken(cookie)); err == nil && token.IsValid() {
				_ = repo.TouchAuthToken(token.ID)
				if acc, _ := repo.FindAccountByID(token.AccountID); acc != nil {
					c.Set("account", acc)
					c.Set("account_id", acc.ID)
					c.Set("account_role", acc.Role)
				}
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, domain.ResponseEnvelope{Error: &domain.APIError{Code: "UNAUTHORIZED", Message: "Authentication required"}})
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
		slog.Debug("http.request.start", "component", "http", "event", "request.start", "request_id", c.GetString("request_id"), "trace_id", c.GetString("trace_id"), "method", c.Request.Method, "route", c.FullPath(), "source_ip", c.ClientIP())
		defer func() {
			if recovered := recover(); recovered != nil {
				slog.Error("http.request.panic", "component", "http", "event", "request.panic", "request_id", c.GetString("request_id"), "trace_id", c.GetString("trace_id"), "account_id", c.GetString("account_id"), "method", c.Request.Method, "route", c.FullPath(), "duration_ms", time.Since(start).Milliseconds(), "outcome", "failure", "error_code", "PANIC")
				panic(recovered)
			}
			status := c.Writer.Status()
			level := slog.LevelInfo
			if status >= 500 {
				level = slog.LevelError
			} else if status >= 400 {
				level = slog.LevelWarn
			}
			if c.Request.URL.Path == "/api/v1/health" && status < 400 {
				level = slog.LevelDebug
			}
			attrs := []any{"component", "http", "event", "request.complete", "request_id", c.GetString("request_id"), "trace_id", c.GetString("trace_id"), "account_id", c.GetString("account_id"), "account_name", c.GetString("account_name"), "role", c.GetString("account_role"), "source_ip", c.ClientIP(), "method", c.Request.Method, "route", c.FullPath(), "status", status, "duration_ms", time.Since(start).Milliseconds(), "response_bytes", c.Writer.Size(), "outcome", map[bool]string{true: "success", false: "failure"}[status < 400]}
			slog.Log(c.Request.Context(), level, "http.request.complete", attrs...)
		}()
		c.Next()
	}
}
