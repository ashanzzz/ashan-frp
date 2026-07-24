package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/integration/cloudflare"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

type cloudflareDNSClient interface {
	ListRecords() ([]domain.CFDNSRecord, error)
	GetRecord(string) (*domain.CFDNSRecord, error)
	CreateDNSRecord(domain.DNSRecordInput, string) (*domain.CFDNSRecord, error)
	UpdateDNSRecord(string, domain.DNSRecordInput, string) (*domain.CFDNSRecord, error)
	DeleteRecord(string) error
}

type DNSHandler struct {
	repo          *repository.Repository
	key           []byte
	clientFactory func(string, string) cloudflareDNSClient
}

func NewDNSHandler(repo *repository.Repository, key []byte) *DNSHandler {
	return &DNSHandler{repo: repo, key: key, clientFactory: func(token, zone string) cloudflareDNSClient { return cloudflare.NewClient(token, zone) }}
}

func (h *DNSHandler) List(c *gin.Context) {
	client, credential, err := h.client()
	if err != nil {
		h.respondCredentialError(c, credential, err)
		return
	}
	records, err := client.ListRecords()
	if err != nil {
		h.respondCredentialError(c, credential, fmt.Errorf("Cloudflare DNS read failed: %w", err))
		return
	}
	h.markCredential(credential, nil)
	if records == nil {
		records = []domain.CFDNSRecord{}
	}
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]any{"records": records, "zone": credential.Identifier, "total": len(records)}})
}

func (h *DNSHandler) Create(c *gin.Context) {
	var input domain.DNSRecordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "INVALID_REQUEST", Message: err.Error()}})
		return
	}
	client, credential, err := h.client()
	if err != nil {
		h.respondCredentialError(c, credential, err)
		return
	}
	record, err := client.CreateDNSRecord(input, "")
	if err != nil {
		h.respondCredentialError(c, credential, fmt.Errorf("Cloudflare DNS create failed: %w", err))
		return
	}
	h.markCredential(credential, nil)
	h.audit(c, "cloudflare_dns.create", record)
	c.JSON(http.StatusCreated, domain.ResponseEnvelope{Data: record})
}

func (h *DNSHandler) Update(c *gin.Context) {
	var input domain.DNSRecordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "INVALID_REQUEST", Message: err.Error()}})
		return
	}
	client, credential, err := h.client()
	if err != nil {
		h.respondCredentialError(c, credential, err)
		return
	}
	recordID := strings.TrimSpace(c.Param("id"))
	comment := ""
	currentRecord, getErr := client.GetRecord(recordID)
	if getErr != nil {
		h.respondCredentialError(c, credential, fmt.Errorf("Cloudflare DNS read before update failed: %w", getErr))
		return
	}
	if currentRecord != nil {
		comment = currentRecord.Comment
	}
	record, err := client.UpdateDNSRecord(recordID, input, comment)
	if err != nil {
		h.respondCredentialError(c, credential, fmt.Errorf("Cloudflare DNS update failed: %w", err))
		return
	}
	h.markCredential(credential, nil)
	h.audit(c, "cloudflare_dns.update", record)
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: record})
}

func (h *DNSHandler) Delete(c *gin.Context) {
	recordID := strings.TrimSpace(c.Param("id"))
	if recordID == "" {
		c.JSON(http.StatusBadRequest, domain.ResponseEnvelope{Error: &domain.APIError{Code: "INVALID_REQUEST", Message: "DNS record ID is required"}})
		return
	}
	client, credential, err := h.client()
	if err != nil {
		h.respondCredentialError(c, credential, err)
		return
	}
	record := &domain.CFDNSRecord{ID: recordID}
	currentRecord, getErr := client.GetRecord(recordID)
	if getErr != nil {
		h.respondCredentialError(c, credential, fmt.Errorf("Cloudflare DNS read before delete failed: %w", getErr))
		return
	}
	if currentRecord != nil {
		record = currentRecord
	}
	if err := client.DeleteRecord(recordID); err != nil {
		h.respondCredentialError(c, credential, fmt.Errorf("Cloudflare DNS delete failed: %w", err))
		return
	}
	h.markCredential(credential, nil)
	h.audit(c, "cloudflare_dns.delete", record)
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: map[string]string{"message": "DNS record deleted"}})
}

func (h *DNSHandler) client() (cloudflareDNSClient, *domain.UpstreamCredential, error) {
	credential, err := h.repo.FindCredentialByProvider("cloudflare")
	if err != nil || credential == nil || credential.EncryptedSecret == "" || strings.TrimSpace(credential.Identifier) == "" {
		return nil, credential, fmt.Errorf("Cloudflare API Token and Zone must be configured first")
	}
	secret, err := security.Decrypt(credential.EncryptedSecret, h.key)
	if err != nil {
		return nil, credential, fmt.Errorf("Cloudflare credential could not be read")
	}
	return h.clientFactory(string(secret), credential.Identifier), credential, nil
}

func (h *DNSHandler) respondCredentialError(c *gin.Context, credential *domain.UpstreamCredential, err error) {
	h.markCredential(credential, err)
	status := http.StatusBadGateway
	if credential == nil || strings.Contains(err.Error(), "must be configured") {
		status = http.StatusPreconditionFailed
	}
	c.JSON(status, domain.ResponseEnvelope{Error: &domain.APIError{Code: "CLOUDFLARE_DNS_FAILED", Message: err.Error()}})
}

func (h *DNSHandler) markCredential(credential *domain.UpstreamCredential, operationErr error) {
	if credential == nil {
		return
	}
	now := time.Now()
	credential.UpdatedAt = now
	if operationErr != nil {
		credential.LastError = operationErr.Error()
	} else {
		credential.LastVerifiedAt = &now
		credential.LastError = ""
	}
	_ = h.repo.UpsertCredential(credential)
}

func (h *DNSHandler) audit(c *gin.Context, action string, record *domain.CFDNSRecord) {
	accountID := c.GetString("account_id")
	account, _ := h.repo.FindAccountByID(accountID)
	accountName := ""
	if account != nil {
		accountName = account.LoginName
	}
	detail := fmt.Sprintf(`{"type":%q,"name":%q}`, record.Type, record.Name)
	_ = h.repo.CreateAuditLog(&domain.AuditLog{ID: domain.NewID("aud"), AccountID: accountID, AccountName: accountName, Action: action, ResourceType: "cloudflare_dns", ResourceID: record.ID, DetailJSON: detail, RequestID: c.GetString("request_id"), TraceID: c.GetString("trace_id"), Outcome: "success", IPAddress: c.ClientIP(), UserAgent: c.GetHeader("User-Agent")})
}
