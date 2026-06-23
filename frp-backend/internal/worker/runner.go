package worker

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
	"gorm.io/gorm"
	"ashan-frp/internal/domain"
	"ashan-frp/internal/http/handlers"
	"ashan-frp/internal/integration/chmlfrp"
	"ashan-frp/internal/integration/cloudflare"
	"ashan-frp/internal/integration/onepanel"
	"ashan-frp/internal/repository"
)

type Runner struct { db *gorm.DB; repo *repository.Repository; pollInterval time.Duration; stopCh chan struct{} }
func NewRunner(db *gorm.DB, repo *repository.Repository) *Runner { return &Runner{db: db, repo: repo, pollInterval: 5 * time.Second, stopCh: make(chan struct{})} }
func (r *Runner) Start() { log.Printf("[worker] started"); go r.loop() }
func (r *Runner) Stop() { close(r.stopCh) }
func (r *Runner) loop() { ticker := time.NewTicker(r.pollInterval); defer ticker.Stop(); for { select { case <-r.stopCh: return; case <-ticker.C: r.processNext() } } }

func (r *Runner) processNext() {
	jobs, _ := r.repo.ListJobs(repository.JobFilter{Status: "queued"})
	if len(jobs) == 0 {
		jobs, _ = r.repo.ListJobs(repository.JobFilter{Status: "failed"})
		for i := range jobs { if jobs[i].Retryable && jobs[i].AttemptCount < jobs[i].MaxAttempts { if jobs[i].NextRetryAt == nil || time.Now().After(*jobs[i].NextRetryAt) { r.execute(&jobs[i]); return } } }
		return
	}
	r.execute(&jobs[len(jobs)-1])
}

func (r *Runner) execute(job *domain.Job) {
	now := time.Now(); job.Status = "running"; job.StartedAt = &now; _ = r.repo.UpdateJob(job)
	handlers.PublishEvent(domain.Event{Channel: "subject:job:" + job.ID, Kind: "job.started", Level: "info", Message: job.Title, Job: &domain.JobSummary{ID: job.ID, Status: "running", Channel: "subject:job:" + job.ID}})
	var result string; var err2 error
	switch job.Kind {
	case "provision_tunnel": result, err2 = r.provisionTunnel(job)
	case "deprovision_tunnel": result, err2 = r.deprovisionTunnel(job)
	case "reconcile": result, err2 = r.reconcile(job)
	default: err2 = fmt.Errorf("unknown kind: %s", job.Kind)
	}
	done := time.Now()
	if err2 != nil { job.Status = "failed"; job.ErrorMessage = err2.Error(); job.CompletedAt = &done
		if job.Retryable && job.AttemptCount < job.MaxAttempts { job.AttemptCount++; job.Status = "queued"; retry := time.Now().Add(time.Duration(job.AttemptCount)*30*time.Second); job.NextRetryAt = &retry }
		handlers.PublishEvent(domain.Event{Channel: "subject:job:"+job.ID, Kind: "job.failed", Level: "error", Message: err2.Error(), Job: &domain.JobSummary{ID: job.ID, Status: "failed"}, Error: &domain.APIError{Code:"JOB_FAILED", Message: err2.Error()}})
	} else { job.Status = "succeeded"; job.ResultJSON = result; job.CompletedAt = &done
		handlers.PublishEvent(domain.Event{Channel: "subject:job:"+job.ID, Kind: "job.succeeded", Level: "info", Message: job.Title, Job: &domain.JobSummary{ID: job.ID, Status: "succeeded"}})
	}
	_ = r.repo.UpdateJob(job)
}

func (r *Runner) provisionTunnel(job *domain.Job) (string, error) {
	var p map[string]any; json.Unmarshal([]byte(job.PayloadJSON), &p)
	tid := p["tunnel_id"].(string); fd := p["full_domain"].(string); proto := p["protocol"].(string); node := p["chmlfrp_node"].(string)
	t, err := r.repo.FindTunnelByID(tid); if err != nil { return "", err }
	cf, _ := r.repo.FindCredentialByProvider("cloudflare"); op, _ := r.repo.FindCredentialByProvider("onepanel"); ch, _ := r.repo.FindCredentialByProvider("chmlfrp")
	if cf != nil && cf.EncryptedSecret != "" { c := cloudflare.NewClient(cf.EncryptedSecret, cf.Identifier); rec, e := c.CreateRecord(fd, "CNAME", fd, t.CFProxied, tid); if e != nil { log.Printf("[worker] CF: %v", e) } else { t.CFRecordID = rec.ID } }
	if (proto == "http" || proto == "https") && op != nil && op.EncryptedSecret != "" { o := onepanel.NewClient(op.Identifier, op.EncryptedSecret); wid, e := o.CreateWebsite(fd, fmt.Sprintf("127.0.0.1:%d", t.LocalPort), proto=="https"); if e != nil { log.Printf("[worker] 1Panel: %v", e) } else { t.OnePanelWebsiteID = wid; t.OnePanelSSLEnabled = proto=="https"; t.OnePanelProxyTarget = fmt.Sprintf("127.0.0.1:%d", t.LocalPort) } }
	if ch != nil && ch.EncryptedSecret != "" { if node == "" { ns, _ := chmlfrp.NewClient(ch.Identifier, ch.EncryptedSecret).GetNodes(); if len(ns) > 0 { node = ns[0].Name } }
		cc := chmlfrp.NewClient(ch.Identifier, ch.EncryptedSecret); chid, e := cc.CreateTunnel(domain.ChmlFrpCreateTunnelReq{TunnelName: t.ChmlfrpTunnelName, Node: node, PortType: proto, LocalIP: t.LocalIP, LocalPort: t.LocalPort, RemotePort: t.RemotePort, BandDomain: fd, Encryption: true, Compression: true})
		if e != nil { log.Printf("[worker] ChmlFrp: %v", e) } else { t.ChmlfrpTunnelID = chid; t.ChmlfrpNode = node }
	}
	t.ActualState = "healthy"; t.StateReason = "Provisioned by " + job.ID; _ = r.repo.UpdateTunnel(t)
	_ = r.repo.UpsertSyncState(&domain.SyncState{ID: domain.NewID("syn"), LocalResourceType: "tunnel", LocalResourceID: tid, ExternalProvider: "chmlfrp", ExternalID: t.ChmlfrpTunnelID, Status: "synced", LastCheckedAt: time.Now()})
	handlers.PublishEvent(domain.Event{Channel: "subject:tunnel:"+tid, Kind: "tunnel.provisioned", Level: "info", Message: fmt.Sprintf("Tunnel %s ready", t.ProjectName), Subject: &domain.SubjectRef{Type: "tunnel", ID: tid, Name: t.ProjectName}})
	rj, _ := json.Marshal(map[string]any{"tunnel_id": tid, "full_domain": fd}); return string(rj), nil
}

func (r *Runner) deprovisionTunnel(job *domain.Job) (string, error) {
	var p map[string]any; json.Unmarshal([]byte(job.PayloadJSON), &p)
	t, _ := r.repo.FindTunnelByID(p["tunnel_id"].(string)); if t == nil { return "", fmt.Errorf("not found") }
	cf, _ := r.repo.FindCredentialByProvider("cloudflare"); op, _ := r.repo.FindCredentialByProvider("onepanel"); ch, _ := r.repo.FindCredentialByProvider("chmlfrp")
	if ch != nil && t.ChmlfrpTunnelID != "" { chmlfrp.NewClient(ch.Identifier, ch.EncryptedSecret).DeleteTunnel(t.ChmlfrpTunnelID) }
	if cf != nil && t.CFRecordID != "" { cloudflare.NewClient(cf.EncryptedSecret, cf.Identifier).DeleteRecord(t.CFRecordID) }
	if op != nil && t.OnePanelWebsiteID > 0 { onepanel.NewClient(op.Identifier, op.EncryptedSecret).DeleteWebsite(t.OnePanelWebsiteID) }
	t.ActualState = "offline"; _ = r.repo.UpdateTunnel(t); return "deprovisioned", nil
}

func (r *Runner) reconcile(job *domain.Job) (string, error) {
	ts, _ := r.repo.ListTunnels(repository.TunnelFilter{}); ch, _ := r.repo.FindCredentialByProvider("chmlfrp"); cfc, _ := r.repo.FindCredentialByProvider("cloudflare")
	diffs := make([]map[string]any, 0)
	if ch != nil && ch.EncryptedSecret != "" {
		cc := chmlfrp.NewClient(ch.Identifier, ch.EncryptedSecret); rt, _ := cc.GetTunnels()
		ln := map[string]bool{}; for _, t := range ts { if t.ChmlfrpTunnelName != "" { ln[t.ChmlfrpTunnelName] = true } }
		for _, rt := range rt { if len(rt.Name) >= 10 && rt.Name[:10] == "[ashan-frp]" && !ln[rt.Name] { diffs = append(diffs, map[string]any{"type":"orphaned_chmlfrp","name":rt.Name,"id":rt.ID}) } }
	}
	if cfc != nil && cfc.EncryptedSecret != "" {
		cfcli := cloudflare.NewClient(cfc.EncryptedSecret, cfc.Identifier); cr, _ := cfcli.FindRecordsByTag()
		ld := map[string]bool{}; for _, t := range ts { if t.FullDomain != "" { ld[t.FullDomain] = true } }
		for _, r := range cr { if !ld[r.Name] { diffs = append(diffs, map[string]any{"type":"orphaned_cf","name":r.Name,"id":r.ID}) } }
	}
	rj, _ := json.Marshal(map[string]any{"tunnels_checked": len(ts), "diffs": len(diffs), "items": diffs}); return string(rj), nil
}
