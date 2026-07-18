package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
)

// NodeHandler manages nodes via GORM repository.
type NodeHandler struct {
	repo *repository.Repository
}

// NewNodeHandler creates a NodeHandler backed by the repository.
func NewNodeHandler(repo *repository.Repository) *NodeHandler {
	return &NodeHandler{repo: repo}
}

// List returns all nodes. Supports ?q=, ?provider=, ?status=, ?health_status= filters.
// GET /api/v1/nodes
func (h *NodeHandler) List(c *gin.Context) {
	f := repository.NodeFilter{
		Provider:     c.Query("provider"),
		Status:       c.Query("status"),
		HealthStatus: c.Query("health_status"),
	}
	nodes, err := h.repo.ListNodes(f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INTERNAL", Message: "Failed to list nodes"},
		})
		return
	}

	// Apply text query filter in Go since Match is not a SQL filter.
	q := c.Query("q")
	if q != "" {
		filtered := make([]domain.Node, 0, len(nodes))
		for _, n := range nodes {
			if n.Match(q) {
				filtered = append(filtered, n)
			}
		}
		nodes = filtered
	}

	if nodes == nil {
		nodes = []domain.Node{}
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]any{"nodes": nodes}})
}

// Get returns a single node by ID.
// GET /api/v1/nodes/:id
func (h *NodeHandler) Get(c *gin.Context) {
	n, err := h.repo.FindNodeByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "NOT_FOUND", Message: "Node not found"},
		})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: n})
}

// Create adds a new node.
// POST /api/v1/nodes
func (h *NodeHandler) Create(c *gin.Context) {
	var input domain.NodeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INVALID_REQUEST", Message: err.Error()},
		})
		return
	}
	now := time.Now().UTC().Truncate(time.Second)
	node := &domain.Node{
		ID:            domain.NewID("node"),
		DisplayName:   input.DisplayName,
		Provider:      input.Provider,
		NodeType:      input.NodeType,
		EndpointURL:   input.EndpointURL,
		Region:        input.Region,
		Status:        input.Status,
		HealthStatus:  domain.HealthUnknown,
		CanonicalName: input.CanonicalName,
		Metadata:      input.Metadata,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if node.Status == "" {
		node.Status = domain.NodeStatusActive
	}
	if err := h.repo.CreateNode(node); err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INTERNAL", Message: "Failed to create node"},
		})
		return
	}
	h.audit("node.create", node.ID, c)
	c.JSON(http.StatusCreated, domain.ResponseEnvelope{Data: node})
}

// Update modifies an existing node.
// PATCH /api/v1/nodes/:id
func (h *NodeHandler) Update(c *gin.Context) {
	n, err := h.repo.FindNodeByID(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "NOT_FOUND", Message: "Node not found"},
		})
		return
	}
	var input domain.NodeInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INVALID_REQUEST", Message: err.Error()},
		})
		return
	}
	if input.DisplayName != "" {
		n.DisplayName = input.DisplayName
	}
	if input.Provider != "" {
		n.Provider = input.Provider
	}
	if input.NodeType != "" {
		n.NodeType = input.NodeType
	}
	if input.EndpointURL != "" {
		n.EndpointURL = input.EndpointURL
	}
	if input.Region != "" {
		n.Region = input.Region
	}
	if input.Status != "" {
		n.Status = input.Status
	}
	if input.CanonicalName != "" {
		n.CanonicalName = input.CanonicalName
	}
	if input.Metadata != nil {
		n.Metadata = input.Metadata
	}
	n.UpdatedAt = time.Now().UTC().Truncate(time.Second)

	if err := h.repo.UpdateNode(n); err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INTERNAL", Message: "Failed to update node"},
		})
		return
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: n})
}

// Sync queues an async node refresh job.
// POST /api/v1/nodes/sync
func (h *NodeHandler) Sync(c *gin.Context) {
	payload, _ := json.Marshal(map[string]any{"scope": "all"})
	job := &domain.Job{
		ID:           domain.NewID("job"),
		Kind:         "node.refresh",
		TargetType:   "nodes",
		TargetID:     "nodes",
		Channel:      "subject:nodes",
		Status:       domain.JobStatusQueued,
		Title:        "Refresh nodes",
		PayloadJSON:  string(payload),
		AttemptCount: 1,
		MaxAttempts:  5,
		Retryable:    true,
		CreatedBy:    c.GetString("account_id"),
	}
	if err := h.repo.CreateJob(job); err != nil {
		c.JSON(http.StatusInternalServerError, domain.ResponseEnvelope{
			Error: &domain.APIError{Code: "INTERNAL", Message: "Failed to queue node sync"},
		})
		return
	}
	h.audit("node.sync", job.ID, c)
	c.JSON(http.StatusAccepted, domain.ResponseEnvelope{
		Data: map[string]string{"message": "Node sync queued"},
		Meta: domain.ResponseMeta{Job: &domain.JobSummary{
			ID:         job.ID,
			Status:     job.Status,
			Channel:    job.Channel,
			Kind:       job.Kind,
			TargetType: job.TargetType,
			TargetID:   job.TargetID,
		}},
	})
}

func (h *NodeHandler) audit(action, resourceID string, c *gin.Context) {
	_ = h.repo.CreateAuditLog(&domain.AuditLog{ID: domain.NewID("aud"), AccountID: c.GetString("account_id"), AccountName: c.GetString("account_name"), Action: action, ResourceType: "node", ResourceID: resourceID, RequestID: c.GetString("request_id"), TraceID: c.GetString("trace_id"), Outcome: "success", IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")})
}
