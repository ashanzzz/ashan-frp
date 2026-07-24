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

func isManagedComment(comment string) bool {
	c := strings.ToLower(strings.TrimSpace(comment))
	return strings.HasPrefix(c, "ashan-frp managed:") || strings.Contains(c, "1panel")
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
	record, err := client.CreateDNSRecord(input, "ashan-frp managed: manual")
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
		if !isManagedComment(currentRecord.Comment) {
			c.JSON(http.StatusForbidden, domain.ResponseEnvelope{Error: &domain.APIError{Code: "ERR_NOT_MANAGED", Message: "原生 DNS 记录受保护，不可直接修改。请先将其标记为 ashan-frp 管理。"}})
			return
		}
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
		if !isManagedComment(currentRecord.Comment) {
			c.JSON(http.StatusForbidden, domain.ResponseEnvelope{Error: &domain.APIError{Code: "ERR_NOT_MANAGED", Message: "原生 DNS 记录受保护，不可直接删除。请先将其标记为 ashan-frp 管理。"}})
			return
		}
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

func (h *DNSHandler) Claim(c *gin.Context) {
	recordID := strings.TrimSpace(c.Param("id"))
	client, credential, err := h.client()
	if err != nil {
		h.respondCredentialError(c, credential, err)
		return
	}
	currentRecord, getErr := client.GetRecord(recordID)
	if getErr != nil || currentRecord == nil {
		h.respondCredentialError(c, credential, fmt.Errorf("Cloudflare DNS record read failed: %w", getErr))
		return
	}
	newComment := "ashan-frp managed: manual"
	if currentRecord.Comment != "" && !strings.Contains(currentRecord.Comment, "ashan-frp managed") {
		newComment = currentRecord.Comment + " (ashan-frp managed: manual)"
	}
	input := domain.DNSRecordInput{
		Type:    currentRecord.Type,
		Name:    currentRecord.Name,
		Content: currentRecord.Content,
		Proxied: &currentRecord.Proxied,
		TTL:     currentRecord.TTL,
	}
	record, err := client.UpdateDNSRecord(recordID, input, newComment)
	if err != nil {
		h.respondCredentialError(c, credential, fmt.Errorf("Cloudflare DNS claim failed: %w", err))
		return
	}
	h.markCredential(credential, nil)
	h.audit(c, "cloudflare_dns.claim", record)
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: record})
}

func (h *DNSHandler) Unclaim(c *gin.Context) {
	recordID := strings.TrimSpace(c.Param("id"))
	client, credential, err := h.client()
	if err != nil {
		h.respondCredentialError(c, credential, err)
		return
	}
	currentRecord, getErr := client.GetRecord(recordID)
	if getErr != nil || currentRecord == nil {
		h.respondCredentialError(c, credential, fmt.Errorf("Cloudflare DNS record read failed: %w", getErr))
		return
	}
	newComment := strings.TrimSpace(strings.ReplaceAll(currentRecord.Comment, "ashan-frp managed: manual", ""))
	newComment = strings.TrimSpace(strings.ReplaceAll(newComment, "(ashan-frp managed: manual)", ""))
	input := domain.DNSRecordInput{
		Type:    currentRecord.Type,
		Name:    currentRecord.Name,
		Content: currentRecord.Content,
		Proxied: &currentRecord.Proxied,
		TTL:     currentRecord.TTL,
	}
	record, err := client.UpdateDNSRecord(recordID, input, newComment)
	if err != nil {
		h.respondCredentialError(c, credential, fmt.Errorf("Cloudflare DNS unclaim failed: %w", err))
		return
	}
	h.markCredential(credential, nil)
	h.audit(c, "cloudflare_dns.unclaim", record)
	c.JSON(http.StatusOK, domain.ResponseEnvelope{Data: record})
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
	cli := h.clientFactory(string(secret), credential.Identifier)
	if !strings.Contains(credential.Identifier, ".") && len(credential.Identifier) >= 20 {
		if lz, ok := cli.(interface{ ListZones() ([]domain.CFZone, error) }); ok {
			if zones, err := lz.ListZones(); err == nil {
				for _, z := range zones {
					if z.ID == credential.Identifier {
						credential.Identifier = z.Name
						_ = h.repo.UpsertCredential(credential)
						break
					}
				}
			}
		}
	}
	return cli, credential, nil
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
