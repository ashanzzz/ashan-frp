package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"ashan-frp/internal/config"
	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
)

type TunnelHandler struct {
	cfg  config.Config
	repo *repository.Repository
}

func NewTunnelHandler(cfg config.Config, repo *repository.Repository) *TunnelHandler {
	return &TunnelHandler{cfg: cfg, repo: repo}
}

func (h *TunnelHandler) List(c *gin.Context) {
	tunnels, _ := h.repo.ListTunnels(repository.TunnelFilter{
		Status: c.Query("status"), Protocol: c.Query("protocol"), ChmlfrpNode: c.Query("chmlfrp_node"),
	})
	if tunnels == nil {
		tunnels = []domain.Tunnel{}
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]any{"tunnels": tunnels}})
}

func (h *TunnelHandler) Get(c *gin.Context) {
	t, err := h.repo.FindTunnelByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, domain.ResponseEnvelope{Error: &domain.APIError{Code: "NOT_FOUND", Message: "Tunnel not found"}})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: t})
}

func (h *TunnelHandler) Create(c *gin.Context) {
	var input domain.TunnelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "INVALID_REQUEST", Message: err.Error()}})
		return
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = strings.TrimSpace(input.ProjectName)
	}
	if name == "" {
		name = strings.TrimSpace(input.Subdomain)
	}
	if name == "" && strings.TrimSpace(input.DNSDomainCNAME) != "" {
		name = strings.TrimSpace(input.DNSDomainCNAME)
	}
	tunnelType := strings.TrimSpace(input.TunnelType)
	if tunnelType == "" {
		tunnelType = strings.TrimSpace(input.Protocol)
	}
	if tunnelType == "" {
		tunnelType = "tcp"
	}
	desiredState := strings.TrimSpace(input.DesiredState)
	if desiredState == "" {
		desiredState = "enabled"
	}
	subdomain := strings.TrimSpace(input.Subdomain)
	if subdomain == "" {
		subdomain = strings.TrimSpace(input.ProjectName)
	}
	fullDomain := subdomain + "." + h.cfg.BaseDomain
	if subdomain == "" {
		fullDomain = strings.TrimSpace(input.DNSDomainCNAME)
		if fullDomain == "" {
			fullDomain = strings.TrimSpace(input.ProjectName)
		}
		if fullDomain == "" {
			fullDomain = name
		}
	}
	existing, _ := h.repo.FindTunnelByDomain(fullDomain)
	if existing != nil {
		c.JSON(http.StatusConflict, domain.ResponseEnvelope{Error: &domain.APIError{Code: "DUPLICATE_DOMAIN", Message: "Domain " + fullDomain + " already exists"}})
		return
	}
	localIP := input.LocalIP
	if localIP == "" {
		localIP = "127.0.0.1"
	}

	protocol := strings.TrimSpace(input.Protocol)
	if protocol == "" {
		protocol = tunnelType
	}
	tunnel := &domain.Tunnel{
		ID:                domain.NewID("tun"),
		NodeID:            input.NodeID,
		Name:              name,
		TunnelType:        tunnelType,
		ProjectName:       input.ProjectName,
		Subdomain:         subdomain,
		FullDomain:        fullDomain,
		Protocol:          protocol,
		LocalIP:           localIP,
		LocalPort:         input.LocalPort,
		RemotePort:        input.RemotePort,
		DNSDomainCNAME:    input.DNSDomainCNAME,
		DNSProxied:        input.DNSProxied,
		DesiredState:      desiredState,
		ChmlfrpNode:       input.ChmlfrpNode,
		ChmlfrpTunnelName: "[ashan-frp]" + name,
		CFProxied:         input.CFProxied,
		ActualState:       "pending",
		StateReason:       "Provisional - awaiting full provisioning",
		CreatedBy:         c.GetString("account_id"),
	}
	if err := h.repo.CreateTunnel(tunnel); err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{Error: &domain.APIError{Code: "INTERNAL", Message: "Failed to create tunnel"}})
		return
	}

	h.audit("tunnel.create", "tunnel", tunnel.ID, c)
	c.JSON(http.StatusCreated, domain.ResponseEnvelope{
		Data: tunnel,
		Meta: domain.ResponseMeta{Job: &domain.JobSummary{ID: domain.NewID("job"), Status: "queued", Channel: "subject:tunnel:" + tunnel.ID}},
	})

}

func (h *TunnelHandler) Update(c *gin.Context) {
	t, err := h.repo.FindTunnelByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, domain.ResponseEnvelope{Error: &domain.APIError{Code: "NOT_FOUND", Message: "Tunnel not found"}})
		return
	}
	var input domain.TunnelInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "INVALID_REQUEST", Message: err.Error()}})
		return
	}
	if input.ProjectName != "" {
		t.ProjectName = input.ProjectName
	}
	if input.Subdomain != "" {
		t.Subdomain = input.Subdomain
		t.FullDomain = input.Subdomain + "." + h.cfg.BaseDomain
	}
	if input.DNSDomainCNAME != "" {
		t.DNSDomainCNAME = input.DNSDomainCNAME
	}
	if input.DNSProxied {
		t.DNSProxied = true
	}
	if input.Protocol != "" {
		t.Protocol = input.Protocol
		if t.TunnelType == "" {
			t.TunnelType = input.Protocol
		}
	}
	if input.LocalIP != "" {
		t.LocalIP = input.LocalIP
	}
	if input.LocalPort > 0 {
		t.LocalPort = input.LocalPort
	}
	if input.RemotePort > 0 {
		t.RemotePort = input.RemotePort
	}
	if input.ChmlfrpNode != "" {
		t.ChmlfrpNode = input.ChmlfrpNode
	}
	if input.CFProxied {
		t.CFProxied = true
	}
	t.ManualOverride = true
	_ = h.repo.UpdateTunnel(t)
	h.audit("tunnel.update", "tunnel", t.ID, c)
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: t})
}

func (h *TunnelHandler) Delete(c *gin.Context) {
	t, err := h.repo.FindTunnelByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, domain.ResponseEnvelope{Error: &domain.APIError{Code: "NOT_FOUND", Message: "Tunnel not found"}})
		return
	}
	t.DesiredState = "archived"
	_ = h.repo.UpdateTunnel(t)
	h.audit("tunnel.archive", "tunnel", t.ID, c)
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]string{"message": "Tunnel archived"}})
}

func (h *TunnelHandler) Provision(c *gin.Context) {
	t, err := h.repo.FindTunnelByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, domain.ResponseEnvelope{Error: &domain.APIError{Code: "NOT_FOUND", Message: "Tunnel not found"}})
		return
	}
	payload, _ := json.Marshal(map[string]any{
		"tunnel_id": t.ID, "full_domain": t.FullDomain, "protocol": t.Protocol,
		"local_port": t.LocalPort, "chmlfrp_node": t.ChmlfrpNode, "cf_proxied": t.CFProxied,
	})
	job := &domain.Job{
		ID: domain.NewID("job"), Kind: "provision_tunnel", TargetType: "tunnel", TargetID: t.ID,
		Status: "queued", Title: "Provision: " + t.ProjectName, PayloadJSON: string(payload),
		AttemptCount: 1, MaxAttempts: 5, Retryable: true, CreatedBy: c.GetString("account_id"),
	}
	if err := h.repo.CreateJob(job); err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{Error: &domain.APIError{Code: "INTERNAL", Message: "Failed to queue tunnel provisioning"}})
		return
	}
	t.ActualState = "provisioning"
	t.StateReason = "Job " + job.ID + " queued"
	if err := h.repo.UpdateTunnel(t); err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{Error: &domain.APIError{Code: "INTERNAL", Message: "Failed to persist tunnel state"}})
		return
	}
	c.JSON(http.StatusAccepted, domain.ResponseEnvelope{
		Data: t,
		Meta: domain.ResponseMeta{Job: &domain.JobSummary{ID: job.ID, Status: job.Status, Channel: "subject:tunnel:" + t.ID, Kind: job.Kind, TargetType: job.TargetType, TargetID: job.TargetID}},
	})
}

func (h *TunnelHandler) audit(action, resType, resID string, c *gin.Context) {
	accID := c.GetString("account_id")
	acc, _ := h.repo.FindAccountByID(accID)
	name := ""
	if acc != nil {
		name = acc.LoginName
	}
	_ = h.repo.CreateAuditLog(&domain.AuditLog{
		ID: domain.NewID("aud"), AccountID: accID, AccountName: name,
		Action: action, ResourceType: resType, ResourceID: resID,
		RequestID: c.GetString("request_id"), TraceID: c.GetString("trace_id"), Outcome: "success", IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent"),
	})
}
