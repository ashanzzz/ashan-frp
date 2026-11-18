package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/integration/chmlfrp"
	"ashan-frp/internal/integration/cloudflare"
	"ashan-frp/internal/integration/onepanel"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

type MatrixItem struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Protocol   string `json:"protocol"`
	FullDomain string `json:"full_domain"`
	LocalIP    string `json:"local_ip"`
	LocalPort  int    `json:"local_port"`
	RemotePort int    `json:"remote_port"`

	// 1Panel 反代与后端目标
	OnePanel struct {
		Configured   bool      `json:"configured"`
		WebsiteID    int       `json:"website_id,omitempty"`
		Alias        string    `json:"alias,omitempty"`
		ListenPort   int       `json:"listen_port,omitempty"`
		ProxyTarget  string    `json:"proxy_target,omitempty"`
		SSLEnabled   bool      `json:"ssl_enabled"`
		SSLStatus    string    `json:"ssl_status,omitempty"`
		SSLExpireAt  time.Time `json:"ssl_expire_at,omitempty"`
		SSLDaysLeft  int       `json:"ssl_days_left,omitempty"`
		SSLAutoRenew bool      `json:"ssl_auto_renew"`
	} `json:"onepanel"`

	// ChmlFrp 节点与穿透
	ChmlFrp struct {
		Configured  bool   `json:"configured"`
		NodeName    string `json:"node_name,omitempty"`
		NodeIP      string `json:"node_ip,omitempty"`
		NodeOnline  bool   `json:"node_online"`
		TunnelID    string `json:"tunnel_id,omitempty"`
		TunnelName  string `json:"tunnel_name,omitempty"`
		Synced      bool   `json:"synced"`
		ActualState string `json:"actual_state"`
	} `json:"chmlfrp"`

	// Cloudflare DNS 状态
	Cloudflare struct {
		Configured bool   `json:"configured"`
		RecordID   string `json:"record_id,omitempty"`
		DNSIP      string `json:"dns_ip,omitempty"`
		Proxied    bool   `json:"proxied"`
		IsAligned  bool   `json:"is_aligned"`
	} `json:"cloudflare"`

	// 端到端健康状态
	Status      string `json:"status"` // "healthy", "degraded", "offline"
	StateReason string `json:"state_reason,omitempty"`
}

type MatrixHandler struct {
	repo *repository.Repository
	key  []byte
}

func NewMatrixHandler(repo *repository.Repository, key []byte) *MatrixHandler {
	return &MatrixHandler{repo: repo, key: key}
}

// BuildMatrix assembles unified matrix items from repository and upstream sources.
func (h *MatrixHandler) BuildMatrix(syncUpstream bool) ([]MatrixItem, error) {
	tunnels, err := h.repo.ListTunnels(repository.TunnelFilter{})
	if err != nil {
		return nil, err
	}

	nodes, _ := h.repo.ListNodes(repository.NodeFilter{})
	nodeMap := make(map[string]domain.Node)
	for _, n := range nodes {
		nodeMap[n.DisplayName] = n
		nodeMap[n.CanonicalName] = n
		if n.RealIP != "" {
			nodeMap[n.RealIP] = n
		}
	}

	// 1. OnePanel Websites & SSL cache
	var opWebsites []domain.OnePanelWebsite
	var opCerts []domain.OnePanelSSL
	if cred, err := h.repo.FindCredentialByProvider("onepanel"); err == nil && cred != nil && cred.EncryptedSecret != "" {
		if dec, decErr := security.Decrypt(cred.EncryptedSecret, h.key); decErr == nil {
			client := onepanel.NewClient(cred.Identifier, string(dec))
			if ws, wsErr := client.ListWebsites(); wsErr == nil {
				for i := range ws {
					if ws[i].Proxy == "" && ws[i].ID > 0 {
						if det, dErr := client.GetWebsite(ws[i].ID); dErr == nil && det != nil {
							ws[i].Proxy = det.Proxy
						}
					}
				}
				opWebsites = ws
			}
			if certs, certErr := client.ListCertificates(""); certErr == nil {
				opCerts = certs
			}
		}
	}

	opMap := make(map[string]domain.OnePanelWebsite)
	for _, ws := range opWebsites {
		if ws.Alias != "" {
			opMap[strings.ToLower(ws.Alias)] = ws
		}
		if ws.PrimaryDomain != "" {
			opMap[strings.ToLower(ws.PrimaryDomain)] = ws
		}
	}

	certDomainMap := make(map[string]domain.OnePanelSSL)
	for _, c := range opCerts {
		if c.PrimaryDomain != "" {
			certDomainMap[strings.ToLower(c.PrimaryDomain)] = c
		}
	}

	// 2. Cloudflare DNS records cache
	cfRecordsMap := make(map[string]domain.CFDNSRecord)
	if cred, err := h.repo.FindCredentialByProvider("cloudflare"); err == nil && cred != nil && cred.EncryptedSecret != "" {
		if dec, decErr := security.Decrypt(cred.EncryptedSecret, h.key); decErr == nil {
			token := string(dec)
			cfClient := cloudflare.NewClient(token, cred.ZoneID)
			if recs, recErr := cfClient.ListRecords(); recErr == nil {
				for _, r := range recs {
					cfRecordsMap[strings.ToLower(r.Name)] = r
				}
			}
		}
	}

	// 3. ChmlFrp remote tunnels (optional sync)
	chmlTunnelsMap := make(map[string]domain.ChmlFrpTunnel)
	if syncUpstream {
		if cred, err := h.repo.FindCredentialByProvider("chmlfrp"); err == nil && cred != nil && cred.EncryptedSecret != "" {
			if dec, decErr := security.Decrypt(cred.EncryptedSecret, h.key); decErr == nil {
				oauthMgr := chmlfrp.NewOAuth2Manager()
				activeToken, updatedPayload, refreshed, _ := oauthMgr.EnsureValidToken(dec)
				if refreshed && len(updatedPayload) > 0 {
					if enc, encErr := security.Encrypt(updatedPayload, h.key); encErr == nil {
						cred.EncryptedSecret = enc
						now := time.Now()
						cred.UpdatedAt = now
						cred.LastVerifiedAt = &now
						_ = h.repo.UpsertCredential(cred)
					}
				}
				chClient := chmlfrp.NewClient("token", activeToken)
				if rts, rtErr := chClient.GetTunnels(); rtErr == nil {
					for _, rt := range rts {
						chmlTunnelsMap[strings.ToLower(rt.GetName())] = rt
					}
				}
			}
		}
	}

	items := make([]MatrixItem, 0, len(tunnels))
	now := time.Now()

	for _, t := range tunnels {
		item := MatrixItem{
			ID:          t.ID,
			Name:        t.Name,
			Protocol:    strings.ToUpper(t.Protocol),
			FullDomain:  t.FullDomain,
			LocalIP:     t.LocalIP,
			LocalPort:   t.LocalPort,
			RemotePort:  t.RemotePort,
			Status:      "healthy",
			StateReason: t.StateReason,
		}
		if item.Protocol == "" {
			item.Protocol = "HTTPS"
		}

		// Match 1Panel Website
		nameKey := strings.ToLower(t.Name)
		domainKey := strings.ToLower(t.FullDomain)
		var matchedWS *domain.OnePanelWebsite
		if ws, ok := opMap[nameKey]; ok {
			matchedWS = &ws
		} else if ws, ok := opMap[domainKey]; ok {
			matchedWS = &ws
		}

		if matchedWS != nil {
			item.OnePanel.Configured = true
			item.OnePanel.WebsiteID = matchedWS.ID
			item.OnePanel.Alias = matchedWS.Alias
			item.OnePanel.ProxyTarget = matchedWS.Proxy
			item.OnePanel.SSLEnabled = matchedWS.Protocol == "HTTPS" || matchedWS.SSLStatus == "success"
			item.OnePanel.SSLStatus = matchedWS.SSLStatus

			// Extract listen port
			if strings.Contains(matchedWS.PrimaryDomain, ":") {
				parts := strings.Split(matchedWS.PrimaryDomain, ":")
				fmt.Sscanf(parts[1], "%d", &item.OnePanel.ListenPort)
			}
			if item.OnePanel.ListenPort == 0 && t.LocalPort > 0 {
				item.OnePanel.ListenPort = t.LocalPort
			}

			// Match SSL cert
			var matchedCert *domain.OnePanelSSL
			if cert, ok := certDomainMap[domainKey]; ok {
				matchedCert = &cert
			}
			if matchedCert != nil {
				item.OnePanel.SSLExpireAt = matchedCert.ExpireDate
				item.OnePanel.SSLAutoRenew = matchedCert.AutoRenew
				if !matchedCert.ExpireDate.IsZero() {
					item.OnePanel.SSLDaysLeft = int(matchedCert.ExpireDate.Sub(now).Hours() / 24)
				}
				if matchedCert.Status != "" {
					item.OnePanel.SSLStatus = matchedCert.Status
				}
			}
		} else if t.LocalPort > 0 {
			item.OnePanel.ListenPort = t.LocalPort
		}

		// Match ChmlFrp
		item.ChmlFrp.NodeName = t.ChmlfrpNode
		if item.ChmlFrp.NodeName == "" {
			item.ChmlFrp.NodeName = "北京多线"
		}
		item.ChmlFrp.TunnelName = t.ChmlfrpTunnelName
		item.ChmlFrp.TunnelID = t.ChmlfrpTunnelID
		item.ChmlFrp.ActualState = t.ActualState
		item.ChmlFrp.Configured = true
		item.ChmlFrp.Synced = t.ChmlfrpTunnelID != "" || t.StateReason == "Synced from ChmlFrp API"

		if rt, ok := chmlTunnelsMap[nameKey]; ok {
			item.ChmlFrp.TunnelID = rt.GetID()
			item.ChmlFrp.Synced = true
			if rt.Node != "" {
				item.ChmlFrp.NodeName = rt.Node
			}
		}

		// Node info
		var targetNodeIP string
		if n, ok := nodeMap[item.ChmlFrp.NodeName]; ok {
			item.ChmlFrp.NodeOnline = n.HealthStatus == domain.HealthOnline || n.Status == domain.NodeStatusActive
			item.ChmlFrp.NodeIP = n.RealIP
			if item.ChmlFrp.NodeIP == "" {
				item.ChmlFrp.NodeIP = n.EndpointURL
			}
			targetNodeIP = item.ChmlFrp.NodeIP
		}
		if targetNodeIP == "" {
			targetNodeIP = "82.156.215.91"
			item.ChmlFrp.NodeIP = targetNodeIP
			item.ChmlFrp.NodeOnline = true
		}

		// Match Cloudflare DNS
		if cfRec, ok := cfRecordsMap[domainKey]; ok {
			item.Cloudflare.Configured = true
			item.Cloudflare.RecordID = cfRec.ID
			item.Cloudflare.DNSIP = cfRec.Content
			item.Cloudflare.Proxied = cfRec.Proxied
			item.Cloudflare.IsAligned = strings.TrimSpace(cfRec.Content) == strings.TrimSpace(targetNodeIP)
		}

		// Evaluate comprehensive status
		if !item.ChmlFrp.NodeOnline {
			item.Status = "degraded"
		} else if item.Cloudflare.Configured && !item.Cloudflare.IsAligned {
			item.Status = "degraded"
		} else if t.ActualState != "running" && t.ActualState != "enabled" && t.ActualState != "" {
			item.Status = "degraded"
		} else {
			item.Status = "healthy"
		}

		items = append(items, item)
	}

	return items, nil
}

// GetMatrix handles GET /api/v1/matrix
func (h *MatrixHandler) GetMatrix(c *gin.Context) {
	syncUpstream := c.Query("sync") == "true"
	items, err := h.BuildMatrix(syncUpstream)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "MATRIX_BUILD_FAILED", Message: err.Error()},
		})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{
		Data: map[string]any{
			"matrix": items,
			"total":  len(items),
		},
	})
}

// SyncMatrix handles POST /api/v1/matrix/sync
func (h *MatrixHandler) SyncMatrix(c *gin.Context) {
	items, err := h.BuildMatrix(true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "MATRIX_SYNC_FAILED", Message: err.Error()},
		})
		return
	}

	// Also update local tunnel table with enriched info
	for _, item := range items {
		t, err := h.repo.FindTunnelByID(item.ID)
		if err == nil && t != nil {
			if item.OnePanel.WebsiteID > 0 {
				t.OnePanelWebsiteID = item.OnePanel.WebsiteID
			}
			t.OnePanelSSLEnabled = item.OnePanel.SSLEnabled
			if item.OnePanel.ProxyTarget != "" {
				t.OnePanelProxyTarget = item.OnePanel.ProxyTarget
			}
			if item.Cloudflare.RecordID != "" {
				t.CFRecordID = item.Cloudflare.RecordID
				t.CFProxied = item.Cloudflare.Proxied
			}
			if item.ChmlFrp.TunnelID != "" {
				t.ChmlfrpTunnelID = item.ChmlFrp.TunnelID
			}
			_ = h.repo.UpdateTunnel(t)
		}
	}

	c.JSON(http.StatusOK, domain.ResponseEnvelope{
		Data: map[string]any{
			"message": "Full-chain matrix synchronization completed successfully",
			"matrix":  items,
			"total":   len(items),
		},
	})
}
