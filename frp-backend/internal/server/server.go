package server

import (
    "context"
    "encoding/json"
    "errors"
    "io"
    "io/fs"
    "log"
    "mime"
    "net/http"
    "os"
    "path"
    "strings"
    "time"

    "ashan-frp/internal/config"
    "ashan-frp/internal/domain"
    "ashan-frp/internal/events"
    "ashan-frp/internal/runtime/frpc"
    "ashan-frp/internal/store"
    "ashan-frp/internal/web"
)

type Server struct {
    cfg    config.Config
    store  *store.Store
    broker *events.Broker
    frpc   *frpc.Manager
    mux    *http.ServeMux
    uiFS   fs.FS
}

func New(cfg config.Config) (*Server, error) {
    st, err := store.Load(cfg.StateFile)
    if err != nil {
        return nil, err
    }
    uiFS, err := web.FS()
    if err != nil {
        uiFS = os.DirFS(".")
    }
    s := &Server{
        cfg:    cfg,
        store:  st,
        broker: events.NewBroker(),
        frpc:   frpc.NewManager("0.54.0-go"),
        mux:    http.NewServeMux(),
        uiFS:   uiFS,
    }
    s.routes()
    return s, nil
}

func (s *Server) Run(ctx context.Context) error {
    srv := &http.Server{
        Addr:    s.cfg.HTTPAddr,
        Handler: s.mux,
    }

    go func() {
        <-ctx.Done()
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        _ = srv.Shutdown(shutdownCtx)
    }()

    log.Printf("[ashan-frp] listening on %s", s.cfg.HTTPAddr)
    err := srv.ListenAndServe()
    if errors.Is(err, http.ErrServerClosed) {
        return nil
    }
    return err
}

func (s *Server) routes() {
    mux := s.mux
    mux.HandleFunc("GET /", s.handleRoot)
    mux.HandleFunc("GET /ui/", s.handleUI)
    mux.HandleFunc("GET /api/v1/version", s.handleVersion)
    mux.HandleFunc("GET /api/v1/health", s.handleHealth)
    mux.HandleFunc("GET /api/openapi.json", s.handleOpenAPI)
    mux.HandleFunc("GET /api/docs", s.handleDocs)
    mux.HandleFunc("GET /api/v1/events/stream", s.handleEventsStream)

    mux.HandleFunc("GET /api/v1/nodes", s.handleNodesList)
    mux.HandleFunc("POST /api/v1/nodes", s.handleNodesCreate)
    mux.HandleFunc("GET /api/v1/nodes/{id}", s.handleNodeGet)
    mux.HandleFunc("PATCH /api/v1/nodes/{id}", s.handleNodePatch)
    mux.HandleFunc("POST /api/v1/nodes/{id}/actions/check", s.handleNodeCheck)
    mux.HandleFunc("POST /api/v1/nodes/{id}/actions/archive", s.handleNodeArchive)

    mux.HandleFunc("GET /api/v1/tunnels", s.handleTunnelsList)
    mux.HandleFunc("POST /api/v1/tunnels", s.handleTunnelsCreate)
    mux.HandleFunc("GET /api/v1/tunnels/{id}", s.handleTunnelGet)
    mux.HandleFunc("PATCH /api/v1/tunnels/{id}", s.handleTunnelPatch)
    mux.HandleFunc("POST /api/v1/tunnels/{id}/actions/apply", s.handleTunnelApply)
    mux.HandleFunc("POST /api/v1/tunnels/{id}/actions/start", s.handleTunnelStart)
    mux.HandleFunc("POST /api/v1/tunnels/{id}/actions/stop", s.handleTunnelStop)
    mux.HandleFunc("POST /api/v1/tunnels/{id}/actions/recreate", s.handleTunnelRecreate)
    mux.HandleFunc("POST /api/v1/tunnels/{id}/actions/archive", s.handleTunnelArchive)

    mux.HandleFunc("GET /api/v1/website-mappings", s.handleWebsiteMappingsList)
    mux.HandleFunc("POST /api/v1/website-mappings", s.handleWebsiteMappingsCreate)
    mux.HandleFunc("GET /api/v1/website-mappings/{id}", s.handleWebsiteMappingGet)
    mux.HandleFunc("PATCH /api/v1/website-mappings/{id}", s.handleWebsiteMappingPatch)
    mux.HandleFunc("POST /api/v1/website-mappings/{id}/actions/sync", s.handleWebsiteMappingSync)
    mux.HandleFunc("POST /api/v1/website-mappings/{id}/actions/reapply", s.handleWebsiteMappingSync)
    mux.HandleFunc("POST /api/v1/website-mappings/{id}/actions/enable-https", s.handleWebsiteMappingEnableHTTPS)
    mux.HandleFunc("POST /api/v1/website-mappings/{id}/actions/disable-https", s.handleWebsiteMappingDisableHTTPS)
    mux.HandleFunc("POST /api/v1/website-mappings/{id}/actions/archive", s.handleWebsiteMappingArchive)

    mux.HandleFunc("GET /api/v1/settings", s.handleSettingsGet)
    mux.HandleFunc("PATCH /api/v1/settings", s.handleSettingsPatch)

    mux.HandleFunc("GET /api/v1/jobs", s.handleJobsList)
    mux.HandleFunc("GET /api/v1/jobs/{id}", s.handleJobGet)

    mux.HandleFunc("GET /api/v1/frpc/runtime", s.handleRuntimeGet)
    mux.HandleFunc("POST /api/v1/frpc/runtime/actions/start", s.handleRuntimeStart)
    mux.HandleFunc("POST /api/v1/frpc/runtime/actions/stop", s.handleRuntimeStop)
    mux.HandleFunc("POST /api/v1/frpc/runtime/actions/restart", s.handleRuntimeRestart)
    mux.HandleFunc("POST /api/v1/frpc/runtime/actions/reload", s.handleRuntimeReload)
    mux.HandleFunc("POST /api/v1/frpc/runtime/actions/switch-node", s.handleRuntimeSwitchNode)
}

func (s *Server) requestMeta(r *http.Request, job *domain.JobSummary) domain.ResponseMeta {
    requestID := r.Header.Get("X-Request-ID")
    if requestID == "" {
        requestID = domain.NewID("req")
    }
    traceID := r.Header.Get("X-Trace-ID")
    if traceID == "" {
        traceID = domain.NewID("trc")
    }
    return domain.ResponseMeta{RequestID: requestID, TraceID: traceID, Job: job}
}

func (s *Server) writeEnvelope(w http.ResponseWriter, status int, meta domain.ResponseMeta, data any, errInfo *domain.APIError) {
    w.Header().Set("Content-Type", "application/json; charset=utf-8")
    w.Header().Set("X-Request-ID", meta.RequestID)
    w.Header().Set("X-Trace-ID", meta.TraceID)
    w.WriteHeader(status)
    enc := json.NewEncoder(w)
    enc.SetEscapeHTML(false)
    _ = enc.Encode(domain.ResponseEnvelope{Data: data, Meta: meta, Error: errInfo})
}

func (s *Server) writeError(w http.ResponseWriter, r *http.Request, status int, code, message string, retryable bool, details any) {
    meta := s.requestMeta(r, nil)
    s.writeEnvelope(w, status, meta, nil, &domain.APIError{Code: code, Message: message, Retryable: retryable, Details: details})
}

func (s *Server) decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
    r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
    defer r.Body.Close()
    dec := json.NewDecoder(r.Body)
    dec.DisallowUnknownFields()
    if err := dec.Decode(dst); err != nil {
        s.writeError(w, r, http.StatusBadRequest, "INVALID_JSON", err.Error(), false, nil)
        return false
    }
    if dec.More() {
        s.writeError(w, r, http.StatusBadRequest, "INVALID_JSON", "body must contain a single JSON object", false, nil)
        return false
    }
    return true
}

func (s *Server) serveJSON(w http.ResponseWriter, r *http.Request, status int, data any, job *domain.JobSummary) {
    s.writeEnvelope(w, status, s.requestMeta(r, job), data, nil)
}

func (s *Server) serveText(w http.ResponseWriter, r *http.Request, status int, contentType string, body []byte) {
    meta := s.requestMeta(r, nil)
    w.Header().Set("Content-Type", contentType)
    w.Header().Set("X-Request-ID", meta.RequestID)
    w.Header().Set("X-Trace-ID", meta.TraceID)
    w.WriteHeader(status)
    _, _ = w.Write(body)
}

func (s *Server) handleRoot(w http.ResponseWriter, r *http.Request) {
    http.Redirect(w, r, "/ui/", http.StatusMovedPermanently)
}

func (s *Server) handleDocs(w http.ResponseWriter, r *http.Request) {
    body := []byte(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Ashan FRP API Docs</title>
<style>body{font-family:system-ui,-apple-system,BlinkMacSystemFont,sans-serif;margin:32px;line-height:1.6;background:#0d0e12;color:#e3e4e8}a{color:#4a9eed}pre{background:#16171f;padding:16px;border-radius:8px;overflow:auto}</style>
</head><body><h1>Ashan FRP API Docs</h1><p>OpenAPI JSON: <a href="/api/openapi.json">/api/openapi.json</a></p><p>This page is intentionally lightweight for the first Go rewrite slice.</p><pre id="spec">loading…</pre><script>fetch('/api/openapi.json').then(r=>r.text()).then(t=>document.getElementById('spec').textContent=t).catch(e=>document.getElementById('spec').textContent=String(e))</script></body></html>`)
    s.serveText(w, r, http.StatusOK, "text/html; charset=utf-8", body)
}

func (s *Server) handleOpenAPI(w http.ResponseWriter, r *http.Request) {
    s.serveText(w, r, http.StatusOK, "application/json; charset=utf-8", openAPISpec)
}

func (s *Server) handleUI(w http.ResponseWriter, r *http.Request) {
    rel := strings.TrimPrefix(r.URL.Path, "/ui/")
    if rel == "" || rel == "." || rel == "/" {
        rel = "index.html"
    }
    if rel == "index.html" {
        if b, err := fs.ReadFile(s.uiFS, "index.html"); err == nil {
            s.serveText(w, r, http.StatusOK, "text/html; charset=utf-8", b)
            return
        }
    }
    if b, err := fs.ReadFile(s.uiFS, rel); err == nil {
        contentType := mime.TypeByExtension(path.Ext(rel))
        if contentType == "" {
            contentType = "application/octet-stream"
        }
        s.serveText(w, r, http.StatusOK, contentType, b)
        return
    }
    if strings.Contains(rel, ".") {
        s.writeError(w, r, http.StatusNotFound, "UI_ASSET_NOT_FOUND", "embedded UI asset missing", false, nil)
        return
    }
    if b, err := fs.ReadFile(s.uiFS, "index.html"); err == nil {
        s.serveText(w, r, http.StatusOK, "text/html; charset=utf-8", b)
        return
    }
    s.writeError(w, r, http.StatusNotFound, "UI_ASSET_NOT_FOUND", "embedded UI asset missing", false, nil)
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
    s.serveJSON(w, r, http.StatusOK, map[string]any{
        "version":   s.cfg.Version,
        "engine":    "go-stdlib-embedded",
        "status":    "healthy",
        "app_name":  s.cfg.AppName,
        "api_base":  s.cfg.APIBasePath,
        "ui_base":   s.cfg.UIBasePath,
    }, nil)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
    snapshot := s.store.Snapshot()
    runtime := s.frpc.Summary()
    s.serveJSON(w, r, http.StatusOK, map[string]any{
        "status":               "healthy",
        "nodes":                len(snapshot.Nodes),
        "tunnels":              len(snapshot.Tunnels),
        "website_mappings":     len(snapshot.WebsiteMappings),
        "jobs":                 len(snapshot.Jobs),
        "events":               len(snapshot.Events),
        "frpc_runtime":         runtime,
        "data_file":            s.cfg.StateFile,
    }, nil)
}

func (s *Server) publish(evt domain.Event) domain.Event {
    persisted, err := s.store.AppendEvent(evt)
    if err != nil {
        log.Printf("[event] append failed: %v", err)
        return evt
    }
    s.broker.Publish(persisted)
    return persisted
}

func (s *Server) emit(kind, channel, level, message string, job *domain.Job, subject *domain.SubjectRef, payload any, apiErr *domain.APIError) domain.Event {
    evt := domain.Event{
        SchemaVersion: 1,
        Channel:       channel,
        Kind:          kind,
        Level:         level,
        Message:       message,
        TraceID:       domain.NewID("trc"),
        CreatedAt:     time.Now().UTC(),
    }
    if job != nil {
        summary := job.Summary()
        evt.Job = &summary
    }
    evt.Subject = subject
    evt.Payload = payload
    evt.Error = apiErr
    return s.publish(evt)
}

func (s *Server) enqueueJob(kind, targetType, targetID, title, channel string, payload any, subject *domain.SubjectRef, process func() (any, *domain.APIError)) domain.JobSummary {
    job, err := s.store.CreateJob(kind, targetType, targetID, title, channel, payload)
    if err != nil {
        log.Printf("[job] create failed: %v", err)
        return domain.JobSummary{}
    }
    s.emit("job.created", channel, "info", title, &job, subject, payload, nil)

    go func(jobID string) {
        time.Sleep(120 * time.Millisecond)
        if _, err := s.store.UpdateJob(jobID, func(j *domain.Job) error {
            j.Status = domain.JobStatusRunning
            return nil
        }); err == nil {
            updated, _ := s.store.GetJob(jobID)
            s.emit("job.started", channel, "info", title, &updated, subject, payload, nil)
        }

        result, apiErr := process()
        if apiErr != nil {
            updated, _ := s.store.UpdateJob(jobID, func(j *domain.Job) error {
                j.Status = domain.JobStatusFailed
                j.Error = apiErr
                j.Result = nil
                return nil
            })
            s.emit("job.failed", channel, "error", apiErr.Message, &updated, subject, payload, apiErr)
            return
        }

        updated, _ := s.store.UpdateJob(jobID, func(j *domain.Job) error {
            j.Status = domain.JobStatusSucceeded
            j.Error = nil
            j.Result = result
            return nil
        })
        s.emit("job.succeeded", channel, "info", title, &updated, subject, result, nil)
    }(job.ID)

    summary := job.Summary()
    return summary
}

func (s *Server) handleEventsStream(w http.ResponseWriter, r *http.Request) {
    channel := r.URL.Query().Get("channel")
    cursor := r.URL.Query().Get("cursor")

    meta := s.requestMeta(r, nil)
    w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")
    w.Header().Set("X-Request-ID", meta.RequestID)
    w.Header().Set("X-Trace-ID", meta.TraceID)

    flusher, ok := w.(http.Flusher)
    if !ok {
        s.writeError(w, r, http.StatusInternalServerError, "SSE_UNSUPPORTED", "streaming response is not supported", false, nil)
        return
    }

    writeEvent := func(evt domain.Event) error {
        if _, err := io.WriteString(w, "id: "+evt.Cursor+"\n"); err != nil {
            return err
        }
        payload, _ := json.Marshal(evt)
        if _, err := io.WriteString(w, "data: "+string(payload)+"\n\n"); err != nil {
            return err
        }
        flusher.Flush()
        return nil
    }

    for _, evt := range s.store.ListEvents(channel, cursor, 200) {
        if err := writeEvent(evt); err != nil {
            return
        }
    }

    sub, unsubscribe := s.broker.Subscribe(channel)
    defer unsubscribe()

    heartbeat := time.NewTicker(15 * time.Second)
    defer heartbeat.Stop()

    notify := r.Context().Done()
    for {
        select {
        case <-notify:
            return
        case evt, ok := <-sub:
            if !ok {
                return
            }
            if err := writeEvent(evt); err != nil {
                return
            }
        case <-heartbeat.C:
            _, _ = io.WriteString(w, ": ping\n\n")
            flusher.Flush()
        }
    }
}
