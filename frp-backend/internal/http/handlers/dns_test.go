package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

type fakeCloudflareDNSClient struct {
	records        []domain.CFDNSRecord
	created        domain.DNSRecordInput
	updated        domain.DNSRecordInput
	updatedID      string
	updatedComment string
	deletedID      string
	listErr        error
}

func (f *fakeCloudflareDNSClient) ListRecords() ([]domain.CFDNSRecord, error) {
	return f.records, f.listErr
}
func (f *fakeCloudflareDNSClient) GetRecord(id string) (*domain.CFDNSRecord, error) {
	if f.listErr != nil {
		return nil, f.listErr
	}
	for _, r := range f.records {
		if r.ID == id {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("record not found")
}
func (f *fakeCloudflareDNSClient) CreateDNSRecord(input domain.DNSRecordInput, _ string) (*domain.CFDNSRecord, error) {
	f.created = input
	return &domain.CFDNSRecord{ID: "rec_created", Name: input.Name, Type: input.Type, Content: input.Content}, nil
}
func (f *fakeCloudflareDNSClient) UpdateDNSRecord(id string, input domain.DNSRecordInput, comment string) (*domain.CFDNSRecord, error) {
	f.updatedID, f.updated, f.updatedComment = id, input, comment
	return &domain.CFDNSRecord{ID: id, Name: input.Name, Type: input.Type, Content: input.Content, Comment: comment}, nil
}
func (f *fakeCloudflareDNSClient) DeleteRecord(id string) error { f.deletedID = id; return nil }

func newDNSHandlerWithFake(t *testing.T) (*DNSHandler, *gorm.DB, *fakeCloudflareDNSClient) {
	t.Helper()
	db := setupHandlerDB(t)
	repo := repository.New(db)
	key := []byte("0123456789abcdef0123456789abcdef")
	secret, err := security.Encrypt([]byte("token"), key)
	require.NoError(t, err)
	require.NoError(t, repo.UpsertCredential(&domain.UpstreamCredential{ID: "cre_test", Provider: "cloudflare", Identifier: "example.com", EncryptedSecret: secret}))
	fake := &fakeCloudflareDNSClient{records: []domain.CFDNSRecord{{ID: "rec_managed", Name: "www.example.com", Type: "CNAME", Content: "edge.example.net", Comment: "ashan-frp managed: tunnel tun_1"}}}
	h := NewDNSHandler(repo, key)
	h.clientFactory = func(string, string) cloudflareDNSClient { return fake }
	return h, db, fake
}

func TestDNSHandler_DeleteStopsWhenRecordLookupFails(t *testing.T) {
	h, _, fake := newDNSHandlerWithFake(t)
	fake.listErr = errors.New("Cloudflare unavailable")
	ctx, recorder := newGinContext(http.MethodDelete, "/api/v1/dns/records/rec_managed", nil)
	ctx.Params = []gin.Param{{Key: "id", Value: "rec_managed"}}

	h.Delete(ctx)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.Empty(t, fake.deletedID)
}

func TestDNSHandler_CRUD_auditsOperationsAndPreservesManagedComment(t *testing.T) {
	h, db, fake := newDNSHandlerWithFake(t)
	createCtx, createRecorder := newGinContext(http.MethodPost, "/api/v1/dns/records", domain.DNSRecordInput{Type: "A", Name: "api.example.com", Content: "192.0.2.1", TTL: 300})
	h.Create(createCtx)
	require.Equal(t, http.StatusCreated, createRecorder.Code)
	require.Equal(t, "api.example.com", fake.created.Name)

	updateCtx, updateRecorder := newGinContext(http.MethodPatch, "/api/v1/dns/records/rec_managed", domain.DNSRecordInput{Type: "CNAME", Name: "www.example.com", Content: "new.example.net", TTL: 300})
	updateCtx.Params = []gin.Param{{Key: "id", Value: "rec_managed"}}
	h.Update(updateCtx)
	require.Equal(t, http.StatusOK, updateRecorder.Code)
	require.Equal(t, "rec_managed", fake.updatedID)
	require.Equal(t, "ashan-frp managed: tunnel tun_1", fake.updatedComment)

	deleteCtx, deleteRecorder := newGinContext(http.MethodDelete, "/api/v1/dns/records/rec_managed", nil)
	deleteCtx.Params = []gin.Param{{Key: "id", Value: "rec_managed"}}
	h.Delete(deleteCtx)
	require.Equal(t, http.StatusOK, deleteRecorder.Code)
	require.Equal(t, "rec_managed", fake.deletedID)

	var audits []domain.AuditLog
	require.NoError(t, db.Find(&audits).Error)
	require.Len(t, audits, 3)
	require.Equal(t, "cloudflare_dns.create", audits[0].Action)
	require.NotContains(t, audits[0].DetailJSON, "token")
}
