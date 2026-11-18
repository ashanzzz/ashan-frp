package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"ashan-frp/internal/config"
	"ashan-frp/internal/domain"
	"ashan-frp/internal/frpc"
	"ashan-frp/internal/integration/chmlfrp"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

// FrpcHandler exposes the FRPC Runtime Manager over HTTP.
type FrpcHandler struct {
	cfg  config.Config
	mgr  *frpc.Manager
	repo *repository.Repository
	key  []byte
}

func NewFrpcHandler(cfg config.Config, mgr *frpc.Manager, repo *repository.Repository, key ...[]byte) *FrpcHandler {
	var k []byte
	if len(key) > 0 && len(key[0]) > 0 {
		k = key[0]
	}
	return &FrpcHandler{cfg: cfg, mgr: mgr, repo: repo, key: k}
}

// prepareRenderer configures the manager's renderer with node & auth settings.
func (h *FrpcHandler) prepareRenderer(tunnels []domain.Tunnel) error {
	serverAddr := ""
	serverPort := 7000
	user := ""
	authToken := "ChmlFrpToken"

	// 1. Get user token from chmlfrp credentials
	if cred, err := h.repo.FindCredentialByProvider("chmlfrp"); err == nil && cred != nil && cred.EncryptedSecret != "" {
		if rawSecret, err := security.Decrypt(cred.EncryptedSecret, h.key); err == nil {
			var tokenData struct {
				AccessToken string `json:"access_token"`
				UserToken   string `json:"user_token"`
			}
			if json.Unmarshal(rawSecret, &tokenData) == nil {
				if tokenData.UserToken != "" {
					user = tokenData.UserToken
				}
				// If user token is empty but access token exists, fetch from userinfo
				if user == "" && tokenData.AccessToken != "" {
					client := chmlfrp.NewClient("token", tokenData.AccessToken)
					if uToken, err := client.GetUserInfo(); err == nil && uToken != "" {
						user = uToken
						tokenData.UserToken = uToken
						if updatedSecret, err := json.Marshal(tokenData); err == nil {
							if enc, err := security.Encrypt(updatedSecret, h.key); err == nil {
								cred.EncryptedSecret = enc
								_ = h.repo.UpsertCredential(cred)
							}
						}
					}
				}
			}
		}
	}

	// 2. Discover node server address
	nodeName := ""
	for _, t := range tunnels {
		if t.ChmlfrpNode != "" {
			nodeName = t.ChmlfrpNode
			break
		}
	}
	if nodeName != "" {
		nodes, _ := h.repo.ListNodes(repository.NodeFilter{})
		for _, n := range nodes {
			if n.DisplayName == nodeName || n.CanonicalName == nodeName {
				if n.RealIP != "" {
					serverAddr = n.RealIP
				} else if n.EndpointURL != "" {
					serverAddr = n.EndpointURL
				}
				break
			}
		}
	}

	// Default fallback to Beijing Multiline if not resolved
	if serverAddr == "" {
		serverAddr = "82.156.215.91"
	}

	h.mgr.Renderer().SetServer(serverAddr, serverPort, user, authToken)
	return nil
}

// Status returns the current FRPC runtime status.
// GET /api/v1/frpc/runtime
func (h *FrpcHandler) Status(c *gin.Context) {
	status := h.mgr.Status()
	lastError := h.mgr.LastError()
	lastCheck := h.mgr.LastHealthcheck()
	statusStr, healthReason := h.mgr.Healthcheck()

	c.JSON(http.StatusOK, domain.ResponseEnvelope{
		Data: map[string]any{
			"status":           string(status),
			"health_status":    string(statusStr),
			"health_reason":    healthReason,
			"last_error":       lastError,
			"last_healthcheck": lastCheck,
			"binary_path":      h.mgr.BinaryPath(),
			"version":          h.mgr.GetVersion(),
		},
	})
}

// Start starts the frpc subprocess.
// POST /api/v1/frpc/start
func (h *FrpcHandler) Start(c *gin.Context) {
	tunnels, err := h.repo.ListTunnels(repository.TunnelFilter{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INTERNAL", Message: "Failed to list tunnels"},
		})
		return
	}
	enabled := make([]domain.Tunnel, 0, len(tunnels))
	for _, t := range tunnels {
		if t.DesiredState == domain.TunnelDesiredEnabled {
			enabled = append(enabled, t)
		}
	}
	_ = h.prepareRenderer(enabled)
	if err := h.mgr.Start(c.Request.Context(), enabled); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "START_FAILED", Message: err.Error()},
		})
		return
	}
	h.audit("frpc.start", c)
	c.JSON(http.StatusOK, domain.ResponseEnvelope{
		Data: map[string]string{"message": "frpc start requested"},
	})
}

// Stop stops the frpc subprocess.
// POST /api/v1/frpc/stop
func (h *FrpcHandler) Stop(c *gin.Context) {
	if err := h.mgr.Stop(); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "STOP_FAILED", Message: err.Error()},
		})
		return
	}
	h.audit("frpc.stop", c)
	c.JSON(http.StatusOK, domain.ResponseEnvelope{
		Data: map[string]string{"message": "frpc stopped"},
	})
}

// Restart stops then starts the frpc subprocess.
// POST /api/v1/frpc/restart
func (h *FrpcHandler) Restart(c *gin.Context) {
	tunnels, err := h.repo.ListTunnels(repository.TunnelFilter{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INTERNAL", Message: "Failed to list tunnels"},
		})
		return
	}
	enabled := make([]domain.Tunnel, 0, len(tunnels))
	for _, t := range tunnels {
		if t.DesiredState == domain.TunnelDesiredEnabled {
			enabled = append(enabled, t)
		}
	}
	_ = h.prepareRenderer(enabled)
	if err := h.mgr.Restart(c.Request.Context(), enabled); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "RESTART_FAILED", Message: err.Error()},
		})
		return
	}
	h.audit("frpc.restart", c)
	c.JSON(http.StatusOK, domain.ResponseEnvelope{
		Data: map[string]string{"message": "frpc restart requested"},
	})
}

// Reload reloads the frpc configuration.
// POST /api/v1/frpc/reload
func (h *FrpcHandler) Reload(c *gin.Context) {
	tunnels, err := h.repo.ListTunnels(repository.TunnelFilter{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INTERNAL", Message: "Failed to list tunnels"},
		})
		return
	}
	enabled := make([]domain.Tunnel, 0, len(tunnels))
	for _, t := range tunnels {
		if t.DesiredState == domain.TunnelDesiredEnabled {
			enabled = append(enabled, t)
		}
	}
	if err := h.mgr.Reload(c.Request.Context(), enabled); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "RELOAD_FAILED", Message: err.Error()},
		})
		return
	}
	h.audit("frpc.reload", c)
	c.JSON(http.StatusOK, domain.ResponseEnvelope{
		Data: map[string]string{"message": "frpc reload requested"},
	})
}

// GetConfig returns the generated frpc.toml configuration content.
// GET /api/v1/frpc/config
func (h *FrpcHandler) GetConfig(c *gin.Context) {
	content, err := h.mgr.GetConfigContent()
	if err != nil {
		c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]string{"config": "# 暂无已生成的 FRPC 配置文件"}})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]string{"config": content}})
}

// GetLogs returns recent stdout/stderr logs from the frpc process.
// GET /api/v1/frpc/logs
func (h *FrpcHandler) GetLogs(c *gin.Context) {
	logs, err := h.mgr.ReadLogs(500)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "READ_LOGS_FAILED", Message: err.Error()},
		})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]string{"logs": logs}})
}

func (h *FrpcHandler) audit(action string, c *gin.Context) {
	_ = h.repo.CreateAuditLog(&domain.AuditLog{ID: domain.NewID("aud"), AccountID: c.GetString("account_id"), AccountName: c.GetString("account_name"), Action: action, ResourceType: "frpc", ResourceID: "runtime", RequestID: c.GetString("request_id"), TraceID: c.GetString("trace_id"), Outcome: "success", IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")})
}
