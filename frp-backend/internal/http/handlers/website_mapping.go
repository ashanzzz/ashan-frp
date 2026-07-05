package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
)

type WebsiteMappingHandler struct {
	repo *repository.Repository
}

func NewWebsiteMappingHandler(repo *repository.Repository) *WebsiteMappingHandler {
	return &WebsiteMappingHandler{repo: repo}
}

// List returns all website mappings.
// GET /api/v1/website-mappings
func (h *WebsiteMappingHandler) List(c *gin.Context) {
	f := repository.WebsiteMappingFilter{
		NodeID: c.Query("node_id"),
		Status: c.Query("status"),
	}
	ws, err := h.repo.ListWebsiteMappings(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INTERNAL", Message: "Failed to list website mappings"},
		})
		return
	}
	if ws == nil {
		ws = []domain.WebsiteMapping{}
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]any{"website_mappings": ws}})
}

// Get returns a single website mapping.
// GET /api/v1/website-mappings/:id
func (h *WebsiteMappingHandler) Get(c *gin.Context) {
	w, err := h.repo.FindWebsiteMappingByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "NOT_FOUND", Message: "Website mapping not found"},
		})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: w})
}

// Create creates a new website mapping.
// POST /api/v1/website-mappings
func (h *WebsiteMappingHandler) Create(c *gin.Context) {
	var input domain.WebsiteMappingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INVALID_REQUEST", Message: err.Error()},
		})
		return
	}
	now := time.Now().UTC().Truncate(time.Second)
	w := &domain.WebsiteMapping{
		ID:               domain.NewID("web"),
		SourceKind:       input.SourceKind,
		NodeID:           input.NodeID,
		TunnelID:         input.TunnelID,
		SourceExternalID: input.SourceExternalID,
		WebsiteAlias:     input.WebsiteAlias,
		PrimaryDomain:    input.PrimaryDomain,
		Domains:          input.Domains,
		HTTPSEnabled:     input.HTTPSEnabled,
		CertificateMode:  input.CertificateMode,
		SSLCertificateRef: input.SSLCertificateRef,
		ProxyEnabled:     input.ProxyEnabled,
		CacheEnabled:     input.CacheEnabled,
		ProxyTarget:      input.ProxyTarget,
		HTTPConfig:       input.HTTPConfig,
		ConflictStrategy:  input.ConflictStrategy,
		Status:           domain.WebsiteStatusPending,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err := h.repo.CreateWebsiteMapping(w); err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INTERNAL", Message: "Failed to create website mapping"},
		})
		return
	}
	h.audit("website_mapping.create", "website_mapping", w.ID, c)
	c.JSON(http.StatusCreated, domain.ResponseEnvelope{Data: w})
}

// Update modifies an existing website mapping.
// PATCH /api/v1/website-mappings/:id
func (h *WebsiteMappingHandler) Update(c *gin.Context) {
	w, err := h.repo.FindWebsiteMappingByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "NOT_FOUND", Message: "Website mapping not found"},
		})
		return
	}
	var input domain.WebsiteMappingInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INVALID_REQUEST", Message: err.Error()},
		})
		return
	}
	if input.SourceKind != "" { w.SourceKind = input.SourceKind }
	if input.NodeID != "" { w.NodeID = input.NodeID }
	if input.TunnelID != "" { w.TunnelID = input.TunnelID }
	if input.SourceExternalID != "" { w.SourceExternalID = input.SourceExternalID }
	if input.WebsiteAlias != "" { w.WebsiteAlias = input.WebsiteAlias }
	if input.PrimaryDomain != "" { w.PrimaryDomain = input.PrimaryDomain }
	if input.Domains != nil { w.Domains = input.Domains }
	w.HTTPSEnabled = input.HTTPSEnabled
	if input.CertificateMode != "" { w.CertificateMode = input.CertificateMode }
	if input.SSLCertificateRef != "" { w.SSLCertificateRef = input.SSLCertificateRef }
	w.ProxyEnabled = input.ProxyEnabled
	w.CacheEnabled = input.CacheEnabled
	if input.ProxyTarget != "" { w.ProxyTarget = input.ProxyTarget }
	if input.HTTPConfig != nil { w.HTTPConfig = input.HTTPConfig }
	if input.ConflictStrategy != "" { w.ConflictStrategy = input.ConflictStrategy }
	w.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	w.Status = domain.WebsiteStatusPending

	if err := h.repo.UpdateWebsiteMapping(w); err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INTERNAL", Message: "Failed to update website mapping"},
		})
		return
	}
	h.audit("website_mapping.update", "website_mapping", w.ID, c)
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: w})
}

// Sync is a stub that would trigger async sync.
// POST /api/v1/website-mappings/:id/sync
func (h *WebsiteMappingHandler) Sync(c *gin.Context) {
	c.JSON(http.StatusOK, domain.ResponseEnvelope{
		Data: map[string]string{"message": "Website mapping sync queued"},
	})
}

func (h *WebsiteMappingHandler) audit(action, resType, resID string, c *gin.Context) {
	accID := c.GetString("account_id")
	acc, _ := h.repo.FindAccountByID(accID)
	name := ""
	if acc != nil {
		name = acc.LoginName
	}
	_ = h.repo.CreateAuditLog(&domain.AuditLog{
		ID:           domain.NewID("aud"),
		AccountID:    accID,
		AccountName:  name,
		Action:       action,
		ResourceType: resType,
		ResourceID:   resID,
		IPAddress:    c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
	})
}
