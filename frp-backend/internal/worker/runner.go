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
	"ashan-frp/internal/security"
)

type Runner struct {
	db           *gorm.DB
	repo         *repository.Repository
	key          []byte
	pollInterval time.Duration
	stopCh       chan struct{}
}

func NewRunner(db *gorm.DB, repo *repository.Repository, key []byte) *Runner {
	return &Runner{db: db, repo: repo, key: key, pollInterval: 5 * time.Second, stopCh: make(chan struct{})}
}

func (r *Runner) Start() {
	log.Printf("[worker] started")
	go r.loop()
}

func (r *Runner) Stop() { close(r.stopCh) }

func (r *Runner) loop() {
	ticker := time.NewTicker(r.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.processNext()
		}
	}
}

func (r *Runner) processNext() {
	jobs, err := r.repo.ListJobs(repository.JobFilter{Status: domain.JobStatusQueued})
	if err == nil && len(jobs) > 0 {
		r.execute(&jobs[len(jobs)-1])
		return
	}

	jobs, err = r.repo.ListJobs(repository.JobFilter{Status: domain.JobStatusFailed})
	if err != nil {
		return
	}
	for i := range jobs {
		if !jobs[i].Retryable || jobs[i].AttemptCount >= jobs[i].MaxAttempts {
			continue
		}
		if jobs[i].NextRetryAt != nil && time.Now().Before(*jobs[i].NextRetryAt) {
			continue
		}
		r.execute(&jobs[i])
		return
	}
}

func (r *Runner) execute(job *domain.Job) {
	now := time.Now()
	job.Status = domain.JobStatusRunning
	job.StartedAt = &now
	job.ErrorMessage = ""
	_ = r.repo.UpdateJob(job)

	handlers.PublishEvent(domain.Event{
		Channel: "subject:job:" + job.ID,
		Kind:    "job.started",
		Level:   "info",
		Message: job.Title,
		Job:     jobSummary(job),
	})

	var result string
	var err2 error
	switch job.Kind {
	case "provision_tunnel":
		result, err2 = r.provisionTunnel(job)
	case "deprovision_tunnel":
		result, err2 = r.deprovisionTunnel(job)
	case "reconcile":
		result, err2 = r.reconcile(job)
	default:
		err2 = fmt.Errorf("unknown kind: %s", job.Kind)
	}

	done := time.Now()
	if err2 != nil {
		job.Status = domain.JobStatusFailed
		job.ErrorMessage = err2.Error()
		job.CompletedAt = &done
		job.NextRetryAt = nil
		if job.Retryable && job.AttemptCount < job.MaxAttempts {
			job.AttemptCount++
			job.Status = domain.JobStatusRetryWait
			retry := time.Now().Add(time.Duration(job.AttemptCount) * 30 * time.Second)
			job.NextRetryAt = &retry
		}
		_ = r.repo.UpdateJob(job)
		publishJobEvent(job, "job.failed", "error", err2.Error(), &domain.APIError{Code: "JOB_FAILED", Message: err2.Error(), Retryable: job.Retryable && job.AttemptCount < job.MaxAttempts})
		if job.Status == domain.JobStatusRetryWait {
			publishJobEvent(job, "job.retry_scheduled", "warn", "job scheduled for retry", nil)
		}
		return
	}

	job.Status = domain.JobStatusSucceeded
	job.ResultJSON = result
	job.CompletedAt = &done
	job.NextRetryAt = nil
	_ = r.repo.UpdateJob(job)
	publishJobEvent(job, "job.succeeded", "info", job.Title, nil)
}


func publishJobEvent(job *domain.Job, kind, level, message string, apiErr *domain.APIError) {
		evt := domain.Event{
			Channel: "subject:job:" + job.ID,
			Kind:    kind,
			Level:   level,
			Message: message,
			Job:     jobSummary(job),
		}
	if apiErr != nil {
		evt.Error = apiErr
	}
	handlers.PublishEvent(evt)
}

func jobSummary(job *domain.Job) *domain.JobSummary {
	if job == nil {
		return nil
	}
	return &domain.JobSummary{
		ID:         job.ID,
		Status:     job.Status,
		Channel:    "subject:job:" + job.ID,
		Kind:       job.Kind,
		TargetType: job.TargetType,
		TargetID:   job.TargetID,
	}
}

func (r *Runner) provisionTunnel(job *domain.Job) (string, error) {
	var p map[string]any
	if err := json.Unmarshal([]byte(job.PayloadJSON), &p); err != nil {
		return "", err
	}
	tid, _ := p["tunnel_id"].(string)
	fd, _ := p["full_domain"].(string)
	proto, _ := p["protocol"].(string)
	node, _ := p["chmlfrp_node"].(string)
	if tid == "" || fd == "" || proto == "" {
		return "", fmt.Errorf("invalid payload")
	}
	t, err := r.repo.FindTunnelByID(tid)
	if err != nil {
		return "", err
	}
	cf, _ := r.repo.FindCredentialByProvider("cloudflare")
	op, _ := r.repo.FindCredentialByProvider("onepanel")
	ch, _ := r.repo.FindCredentialByProvider("chmlfrp")
	if cf != nil && cf.EncryptedSecret != "" {
		sec, err := security.Decrypt(cf.EncryptedSecret, r.key)
		if err != nil {
			return "", fmt.Errorf("decrypt cloudflare credential: %w", err)
		}
		c := cloudflare.NewClient(string(sec), cf.Identifier)
		rec, e := c.CreateRecord(fd, "CNAME", fd, t.CFProxied, tid)
		if e != nil {
			return "", e
		}
		t.CFRecordID = rec.ID
	}
	if (proto == "http" || proto == "https") && op != nil && op.EncryptedSecret != "" {
		sec, err := security.Decrypt(op.EncryptedSecret, r.key)
		if err != nil {
			return "", fmt.Errorf("decrypt onepanel credential: %w", err)
		}
		o := onepanel.NewClient(op.Identifier, string(sec))
		wid, e := o.CreateWebsite(fd, fmt.Sprintf("127.0.0.1:%d", t.LocalPort), proto == "https")
		if e != nil {
			return "", e
		}
		t.OnePanelWebsiteID = wid
		t.OnePanelSSLEnabled = proto == "https"
		t.OnePanelProxyTarget = fmt.Sprintf("127.0.0.1:%d", t.LocalPort)
	}
	if ch != nil && ch.EncryptedSecret != "" {
		sec, err := security.Decrypt(ch.EncryptedSecret, r.key)
		if err != nil {
			return "", fmt.Errorf("decrypt chmlfrp credential: %w", err)
		}
		secStr := string(sec)
		if node == "" {
			ns, err := chmlfrp.NewClient(ch.Identifier, secStr).GetNodes()
			if err != nil {
				return "", err
			}
			if len(ns) > 0 {
				node = ns[0].Name
			}
		}
		cc := chmlfrp.NewClient(ch.Identifier, secStr)
		chid, e := cc.CreateTunnel(domain.ChmlFrpCreateTunnelReq{TunnelName: t.ChmlfrpTunnelName, Node: node, PortType: proto, LocalIP: t.LocalIP, LocalPort: t.LocalPort, RemotePort: t.RemotePort, BandDomain: fd, Encryption: true, Compression: true})
		if e != nil {
			return "", e
		}
		t.ChmlfrpTunnelID = chid
		t.ChmlfrpNode = node
	}
	t.ActualState = "healthy"
	t.StateReason = "Provisioned by " + job.ID
	if err := r.repo.UpdateTunnel(t); err != nil {
		return "", err
	}
	if err := r.repo.UpsertSyncState(&domain.SyncState{ID: domain.NewID("syn"), LocalResourceType: "tunnel", LocalResourceID: tid, ExternalProvider: "chmlfrp", ExternalID: t.ChmlfrpTunnelID, Status: "synced", LastCheckedAt: time.Now()}); err != nil {
		return "", err
	}
	publishJobEvent(job, "tunnel.provisioned", "info", fmt.Sprintf("Tunnel %s ready", t.ProjectName), nil)
	rj, _ := json.Marshal(map[string]any{"tunnel_id": tid, "full_domain": fd})
	return string(rj), nil
}

func (r *Runner) deprovisionTunnel(job *domain.Job) (string, error) {
	var p map[string]any
	if err := json.Unmarshal([]byte(job.PayloadJSON), &p); err != nil {
		return "", err
	}
	t, _ := r.repo.FindTunnelByID(fmt.Sprint(p["tunnel_id"]))
	if t == nil {
		return "", fmt.Errorf("not found")
	}
	cf, _ := r.repo.FindCredentialByProvider("cloudflare")
	op, _ := r.repo.FindCredentialByProvider("onepanel")
	ch, _ := r.repo.FindCredentialByProvider("chmlfrp")
	if ch != nil && t.ChmlfrpTunnelID != "" {
		s, err := security.Decrypt(ch.EncryptedSecret, r.key)
		if err != nil {
			return "", fmt.Errorf("decrypt chmlfrp credential: %w", err)
		}
		if err := chmlfrp.NewClient(ch.Identifier, string(s)).DeleteTunnel(t.ChmlfrpTunnelID); err != nil {
			return "", err
		}
	}
	if cf != nil && t.CFRecordID != "" {
		s, err := security.Decrypt(cf.EncryptedSecret, r.key)
		if err != nil {
			return "", fmt.Errorf("decrypt cloudflare credential: %w", err)
		}
		if err := cloudflare.NewClient(string(s), cf.Identifier).DeleteRecord(t.CFRecordID); err != nil {
			return "", err
		}
	}
	if op != nil && t.OnePanelWebsiteID > 0 {
		s, err := security.Decrypt(op.EncryptedSecret, r.key)
		if err != nil {
			return "", fmt.Errorf("decrypt onepanel credential: %w", err)
		}
		if err := onepanel.NewClient(op.Identifier, string(s)).DeleteWebsite(t.OnePanelWebsiteID); err != nil {
			return "", err
		}
	}
	t.ActualState = "offline"
	if err := r.repo.UpdateTunnel(t); err != nil {
		return "", err
	}
	publishJobEvent(job, "tunnel.deprovisioned", "info", "Tunnel deprovisioned", nil)
	return "deprovisioned", nil
}

func (r *Runner) reconcile(job *domain.Job) (string, error) {
	ts, _ := r.repo.ListTunnels(repository.TunnelFilter{})
	ch, _ := r.repo.FindCredentialByProvider("chmlfrp")
	cfc, _ := r.repo.FindCredentialByProvider("cloudflare")
	diffs := make([]map[string]any, 0)
	if ch != nil && ch.EncryptedSecret != "" {
		s, err := security.Decrypt(ch.EncryptedSecret, r.key)
		if err != nil {
			return "", fmt.Errorf("decrypt chmlfrp credential: %w", err)
		}
		cc := chmlfrp.NewClient(ch.Identifier, string(s))
		rt, err := cc.GetTunnels()
		if err != nil {
			return "", err
		}
		ln := map[string]bool{}
		for _, t := range ts {
			if t.ChmlfrpTunnelName != "" {
				ln[t.ChmlfrpTunnelName] = true
			}
		}
		for _, rt := range rt {
			if len(rt.Name) >= 10 && rt.Name[:10] == "[ashan-frp]" && !ln[rt.Name] {
				diffs = append(diffs, map[string]any{"type": "orphaned_chmlfrp", "name": rt.Name, "id": rt.ID})
			}
		}
	}
	if cfc != nil && cfc.EncryptedSecret != "" {
		s, err := security.Decrypt(cfc.EncryptedSecret, r.key)
		if err != nil {
			return "", fmt.Errorf("decrypt cloudflare credential: %w", err)
		}
		cfcli := cloudflare.NewClient(string(s), cfc.Identifier)
		cr, err := cfcli.FindRecordsByTag()
		if err != nil {
			return "", err
		}
		ld := map[string]bool{}
		for _, t := range ts {
			if t.FullDomain != "" {
				ld[t.FullDomain] = true
			}
		}
		for _, r := range cr {
			if !ld[r.Name] {
				diffs = append(diffs, map[string]any{"type": "orphaned_cf", "name": r.Name, "id": r.ID})
			}
		}
	}
	rj, err := json.Marshal(map[string]any{"tunnels_checked": len(ts), "diffs": len(diffs), "items": diffs})
	if err != nil {
		return "", err
	}
	publishJobEvent(job, "sync.reconciled", "info", "Reconciliation completed", nil)
	return string(rj), nil
}

