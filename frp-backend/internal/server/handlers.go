package server

import (
    "net/http"
    "strconv"
    "strings"
    "time"

    "ashan-frp/internal/domain"
)

func (s *Server) handleNodesList(w http.ResponseWriter, r *http.Request) {
    includeArchived := strings.EqualFold(r.URL.Query().Get("include_archived"), "true")
    nodes := s.store.ListNodes(domain.NodeListFilter{
        Q:               r.URL.Query().Get("q"),
        Provider:        r.URL.Query().Get("provider"),
        Status:          r.URL.Query().Get("status"),
        HealthStatus:    r.URL.Query().Get("health_status"),
        IncludeArchived: includeArchived,
    })
    s.serveJSON(w, r, http.StatusOK, map[string]any{"nodes": nodes}, nil)
}

func (s *Server) handleNodesCreate(w http.ResponseWriter, r *http.Request) {
    var input domain.NodeInput
    if !s.decodeJSON(w, r, &input) {
        return
    }
    if strings.TrimSpace(input.DisplayName) == "" || strings.TrimSpace(input.Provider) == "" || strings.TrimSpace(input.NodeType) == "" {
        s.writeError(w, r, http.StatusBadRequest, "NODE_VALIDATION_FAILED", "display_name, provider 和 node_type 不能为空", false, nil)
        return
    }
    node, err := s.store.UpsertNode("", input)
    if err != nil {
        s.writeError(w, r, http.StatusInternalServerError, "NODE_SAVE_FAILED", err.Error(), false, nil)
        return
    }
    s.emit("node.created", "subject:node:"+node.ID, "info", "node created", nil, &domain.SubjectRef{Type: "node", ID: node.ID, Name: node.DisplayName}, node, nil)
    s.serveJSON(w, r, http.StatusCreated, node, nil)
}

func (s *Server) handleNodeGet(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    node, ok := s.store.GetNode(id)
    if !ok {
        s.writeError(w, r, http.StatusNotFound, "NODE_NOT_FOUND", "node not found", false, map[string]any{"id": id})
        return
    }
    s.serveJSON(w, r, http.StatusOK, node, nil)
}

func (s *Server) handleNodePatch(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    var input domain.NodeInput
    if !s.decodeJSON(w, r, &input) {
        return
    }
    if strings.TrimSpace(input.DisplayName) == "" || strings.TrimSpace(input.Provider) == "" || strings.TrimSpace(input.NodeType) == "" {
        s.writeError(w, r, http.StatusBadRequest, "NODE_VALIDATION_FAILED", "display_name, provider 和 node_type 不能为空", false, nil)
        return
    }
    node, err := s.store.UpsertNode(id, input)
    if err != nil {
        s.writeError(w, r, http.StatusNotFound, "NODE_NOT_FOUND", err.Error(), false, map[string]any{"id": id})
        return
    }
    s.emit("node.updated", "subject:node:"+node.ID, "info", "node updated", nil, &domain.SubjectRef{Type: "node", ID: node.ID, Name: node.DisplayName}, node, nil)
    s.serveJSON(w, r, http.StatusOK, node, nil)
}

func (s *Server) handleNodeCheck(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    node, ok := s.store.GetNode(id)
    if !ok {
        s.writeError(w, r, http.StatusNotFound, "NODE_NOT_FOUND", "node not found", false, map[string]any{"id": id})
        return
    }
    subject := &domain.SubjectRef{Type: "node", ID: node.ID, Name: node.DisplayName}
    summary := s.enqueueJob(
        "node.check",
        "node",
        node.ID,
        "node check: "+node.DisplayName,
        "subject:node:"+node.ID,
        map[string]any{"endpoint_url": node.EndpointURL},
        subject,
        func() (any, *domain.APIError) {
            checked, err := s.store.UpdateNodeStatus(node.ID, domain.NodeStatusActive, domain.HealthOnline)
            if err != nil {
                return nil, &domain.APIError{Code: "NODE_UPDATE_FAILED", Message: err.Error(), Retryable: true}
            }
            return map[string]any{"node": checked, "checked_at": time.Now().UTC()}, nil
        },
    )
    s.serveJSON(w, r, http.StatusAccepted, map[string]any{"accepted": true}, &summary)
}

func (s *Server) handleNodeArchive(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    node, err := s.store.UpdateNode(id, func(n *domain.Node) error {
        n.Status = domain.NodeStatusArchived
        n.HealthStatus = domain.HealthUnknown
        return nil
    })
    if err != nil {
        s.writeError(w, r, http.StatusNotFound, "NODE_NOT_FOUND", err.Error(), false, map[string]any{"id": id})
        return
    }
    s.emit("node.archived", "subject:node:"+node.ID, "info", "node archived", nil, &domain.SubjectRef{Type: "node", ID: node.ID, Name: node.DisplayName}, node, nil)
    s.serveJSON(w, r, http.StatusOK, node, nil)
}

func (s *Server) handleTunnelsList(w http.ResponseWriter, r *http.Request) {
    manual := strings.EqualFold(r.URL.Query().Get("manual_override"), "true")
    tunnels := s.store.ListTunnels(domain.TunnelListFilter{
        Q:              r.URL.Query().Get("q"),
        NodeID:         r.URL.Query().Get("node_id"),
        DesiredState:   r.URL.Query().Get("desired_state"),
        DiffStatus:     r.URL.Query().Get("diff_status"),
        ManualOverride: manual,
    })
    s.serveJSON(w, r, http.StatusOK, map[string]any{"tunnels": tunnels}, nil)
}

func (s *Server) handleTunnelsCreate(w http.ResponseWriter, r *http.Request) {
    var input domain.TunnelInput
    if !s.decodeJSON(w, r, &input) {
        return
    }
    if strings.TrimSpace(input.NodeID) == "" || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.TunnelType) == "" || strings.TrimSpace(input.LocalIP) == "" || input.LocalPort <= 0 {
        s.writeError(w, r, http.StatusBadRequest, "TUNNEL_VALIDATION_FAILED", "node_id, name, tunnel_type, local_ip 和 local_port 不能为空", false, nil)
        return
    }
    tunnel, err := s.store.UpsertTunnel("", input)
    if err != nil {
        s.writeError(w, r, http.StatusInternalServerError, "TUNNEL_SAVE_FAILED", err.Error(), false, nil)
        return
    }
    s.emit("tunnel.created", "subject:tunnel:"+tunnel.ID, "info", "tunnel created", nil, &domain.SubjectRef{Type: "tunnel", ID: tunnel.ID, Name: tunnel.Name}, tunnel, nil)
    s.serveJSON(w, r, http.StatusCreated, tunnel, nil)
}

func (s *Server) handleTunnelGet(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    tunnel, ok := s.store.GetTunnel(id)
    if !ok {
        s.writeError(w, r, http.StatusNotFound, "TUNNEL_NOT_FOUND", "tunnel not found", false, map[string]any{"id": id})
        return
    }
    s.serveJSON(w, r, http.StatusOK, tunnel, nil)
}

func (s *Server) handleTunnelPatch(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    var input domain.TunnelInput
    if !s.decodeJSON(w, r, &input) {
        return
    }
    if strings.TrimSpace(input.NodeID) == "" || strings.TrimSpace(input.Name) == "" || strings.TrimSpace(input.TunnelType) == "" || strings.TrimSpace(input.LocalIP) == "" || input.LocalPort <= 0 {
        s.writeError(w, r, http.StatusBadRequest, "TUNNEL_VALIDATION_FAILED", "node_id, name, tunnel_type, local_ip 和 local_port 不能为空", false, nil)
        return
    }
    tunnel, err := s.store.UpsertTunnel(id, input)
    if err != nil {
        s.writeError(w, r, http.StatusNotFound, "TUNNEL_NOT_FOUND", err.Error(), false, map[string]any{"id": id})
        return
    }
    s.emit("tunnel.updated", "subject:tunnel:"+tunnel.ID, "info", "tunnel updated", nil, &domain.SubjectRef{Type: "tunnel", ID: tunnel.ID, Name: tunnel.Name}, tunnel, nil)
    s.serveJSON(w, r, http.StatusOK, tunnel, nil)
}

func (s *Server) handleTunnelApply(w http.ResponseWriter, r *http.Request) {
    s.handleTunnelJobAction(w, r, "tunnel.apply", "apply", func(tunnel domain.Tunnel) (any, *domain.APIError) {
        applied, err := s.store.UpdateTunnelState(tunnel.ID, domain.TunnelActualEnabled, "applied to runtime", "", "")
        if err != nil {
            return nil, &domain.APIError{Code: "TUNNEL_UPDATE_FAILED", Message: err.Error(), Retryable: true}
        }
        runtime := s.frpc.Start(tunnel.ID)
        return map[string]any{"tunnel": applied, "runtime": runtime}, nil
    })
}

func (s *Server) handleTunnelStart(w http.ResponseWriter, r *http.Request) {
    s.handleTunnelJobAction(w, r, "tunnel.start", "start", func(tunnel domain.Tunnel) (any, *domain.APIError) {
        applied, err := s.store.UpdateTunnelState(tunnel.ID, domain.TunnelActualEnabled, "started", "", "")
        if err != nil {
            return nil, &domain.APIError{Code: "TUNNEL_UPDATE_FAILED", Message: err.Error(), Retryable: true}
        }
        runtime := s.frpc.Start(tunnel.ID)
        return map[string]any{"tunnel": applied, "runtime": runtime}, nil
    })
}

func (s *Server) handleTunnelStop(w http.ResponseWriter, r *http.Request) {
    s.handleTunnelJobAction(w, r, "tunnel.stop", "stop", func(tunnel domain.Tunnel) (any, *domain.APIError) {
        applied, err := s.store.UpdateTunnelState(tunnel.ID, domain.TunnelActualDisabled, "stopped", "", "")
        if err != nil {
            return nil, &domain.APIError{Code: "TUNNEL_UPDATE_FAILED", Message: err.Error(), Retryable: true}
        }
        runtime := s.frpc.Stop(tunnel.ID)
        return map[string]any{"tunnel": applied, "runtime": runtime}, nil
    })
}

func (s *Server) handleTunnelRecreate(w http.ResponseWriter, r *http.Request) {
    s.handleTunnelJobAction(w, r, "tunnel.recreate", "recreate", func(tunnel domain.Tunnel) (any, *domain.APIError) {
        runtime := s.frpc.Restart(tunnel.ID)
        applied, err := s.store.UpdateTunnelState(tunnel.ID, domain.TunnelActualEnabled, "recreated", "", "")
        if err != nil {
            return nil, &domain.APIError{Code: "TUNNEL_UPDATE_FAILED", Message: err.Error(), Retryable: true}
        }
        return map[string]any{"tunnel": applied, "runtime": runtime}, nil
    })
}

func (s *Server) handleTunnelArchive(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    tunnel, err := s.store.UpdateTunnel(id, func(t *domain.Tunnel) error {
        t.DesiredState = domain.TunnelDesiredDisabled
        t.ActualState = domain.TunnelActualDisabled
        t.StateReason = "archived"
        return nil
    })
    if err != nil {
        s.writeError(w, r, http.StatusNotFound, "TUNNEL_NOT_FOUND", err.Error(), false, map[string]any{"id": id})
        return
    }
    s.emit("tunnel.archived", "subject:tunnel:"+tunnel.ID, "info", "tunnel archived", nil, &domain.SubjectRef{Type: "tunnel", ID: tunnel.ID, Name: tunnel.Name}, tunnel, nil)
    s.serveJSON(w, r, http.StatusOK, tunnel, nil)
}

func (s *Server) handleTunnelJobAction(w http.ResponseWriter, r *http.Request, kind, action string, process func(domain.Tunnel) (any, *domain.APIError)) {
    id := r.PathValue("id")
    tunnel, ok := s.store.GetTunnel(id)
    if !ok {
        s.writeError(w, r, http.StatusNotFound, "TUNNEL_NOT_FOUND", "tunnel not found", false, map[string]any{"id": id})
        return
    }
    subject := &domain.SubjectRef{Type: "tunnel", ID: tunnel.ID, Name: tunnel.Name}
    summary := s.enqueueJob(
        kind,
        "tunnel",
        tunnel.ID,
        action+": "+tunnel.Name,
        "subject:tunnel:"+tunnel.ID,
        map[string]any{"action": action},
        subject,
        func() (any, *domain.APIError) {
            return process(tunnel)
        },
    )
    s.serveJSON(w, r, http.StatusAccepted, map[string]any{"accepted": true}, &summary)
}

func (s *Server) handleWebsiteMappingsList(w http.ResponseWriter, r *http.Request) {
    var httpsEnabled *bool
    if v := r.URL.Query().Get("https_enabled"); v != "" {
        if parsed, err := strconv.ParseBool(v); err == nil {
            httpsEnabled = &parsed
        }
    }
    mappings := s.store.ListWebsiteMappings(domain.WebsiteMappingListFilter{
        Q:               r.URL.Query().Get("q"),
        NodeID:          r.URL.Query().Get("node_id"),
        HTTPSEnabled:    httpsEnabled,
        Status:          r.URL.Query().Get("status"),
        IncludeArchived: strings.EqualFold(r.URL.Query().Get("include_archived"), "true"),
    })
    s.serveJSON(w, r, http.StatusOK, map[string]any{"website_mappings": mappings}, nil)
}

func (s *Server) handleWebsiteMappingsCreate(w http.ResponseWriter, r *http.Request) {
    var input domain.WebsiteMappingInput
    if !s.decodeJSON(w, r, &input) {
        return
    }
    if strings.TrimSpace(input.SourceKind) == "" || strings.TrimSpace(input.NodeID) == "" || strings.TrimSpace(input.PrimaryDomain) == "" || len(input.Domains) == 0 {
        s.writeError(w, r, http.StatusBadRequest, "WEBSITE_MAPPING_VALIDATION_FAILED", "source_kind, node_id, primary_domain, domains 不能为空", false, nil)
        return
    }
    wm, err := s.store.UpsertWebsiteMapping("", input)
    if err != nil {
        s.writeError(w, r, http.StatusInternalServerError, "WEBSITE_MAPPING_SAVE_FAILED", err.Error(), false, nil)
        return
    }
    s.emit("website_mapping.created", "subject:website_mapping:"+wm.ID, "info", "website mapping created", nil, &domain.SubjectRef{Type: "website_mapping", ID: wm.ID, Name: wm.PrimaryDomain}, wm, nil)
    s.serveJSON(w, r, http.StatusCreated, wm, nil)
}

func (s *Server) handleWebsiteMappingGet(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    wm, ok := s.store.GetWebsiteMapping(id)
    if !ok {
        s.writeError(w, r, http.StatusNotFound, "WEBSITE_MAPPING_NOT_FOUND", "website mapping not found", false, map[string]any{"id": id})
        return
    }
    s.serveJSON(w, r, http.StatusOK, wm, nil)
}

func (s *Server) handleWebsiteMappingPatch(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    var input domain.WebsiteMappingInput
    if !s.decodeJSON(w, r, &input) {
        return
    }
    if strings.TrimSpace(input.SourceKind) == "" || strings.TrimSpace(input.NodeID) == "" || strings.TrimSpace(input.PrimaryDomain) == "" || len(input.Domains) == 0 {
        s.writeError(w, r, http.StatusBadRequest, "WEBSITE_MAPPING_VALIDATION_FAILED", "source_kind, node_id, primary_domain, domains 不能为空", false, nil)
        return
    }
    wm, err := s.store.UpsertWebsiteMapping(id, input)
    if err != nil {
        s.writeError(w, r, http.StatusNotFound, "WEBSITE_MAPPING_NOT_FOUND", err.Error(), false, map[string]any{"id": id})
        return
    }
    s.emit("website_mapping.updated", "subject:website_mapping:"+wm.ID, "info", "website mapping updated", nil, &domain.SubjectRef{Type: "website_mapping", ID: wm.ID, Name: wm.PrimaryDomain}, wm, nil)
    s.serveJSON(w, r, http.StatusOK, wm, nil)
}

func (s *Server) handleWebsiteMappingSync(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    wm, ok := s.store.GetWebsiteMapping(id)
    if !ok {
        s.writeError(w, r, http.StatusNotFound, "WEBSITE_MAPPING_NOT_FOUND", "website mapping not found", false, map[string]any{"id": id})
        return
    }
    subject := &domain.SubjectRef{Type: "website_mapping", ID: wm.ID, Name: wm.PrimaryDomain}
    summary := s.enqueueJob(
        "website_mapping.sync",
        "website_mapping",
        wm.ID,
        "sync website mapping: "+wm.PrimaryDomain,
        "subject:website_mapping:"+wm.ID,
        map[string]any{"primary_domain": wm.PrimaryDomain},
        subject,
        func() (any, *domain.APIError) {
            updated, err := s.store.UpdateWebsiteState(wm.ID, domain.WebsiteStatusSynced, "synced", "", "")
            if err != nil {
                return nil, &domain.APIError{Code: "WEBSITE_MAPPING_UPDATE_FAILED", Message: err.Error(), Retryable: true}
            }
            if updated.PanelWebsiteID == "" {
                _ = updated
                _, _ = s.store.UpdateWebsiteMapping(wm.ID, func(item *domain.WebsiteMapping) error {
                    item.PanelWebsiteID = "panel_" + strings.ReplaceAll(wm.PrimaryDomain, ".", "_")
                    return nil
                })
                updated, _ = s.store.GetWebsiteMapping(wm.ID)
            }
            return map[string]any{"website_mapping": updated, "synced_at": time.Now().UTC()}, nil
        },
    )
    s.serveJSON(w, r, http.StatusAccepted, map[string]any{"accepted": true}, &summary)
}

func (s *Server) handleWebsiteMappingEnableHTTPS(w http.ResponseWriter, r *http.Request) {
    s.handleWebsiteHTTPSAction(w, r, true)
}

func (s *Server) handleWebsiteMappingDisableHTTPS(w http.ResponseWriter, r *http.Request) {
    s.handleWebsiteHTTPSAction(w, r, false)
}

func (s *Server) handleWebsiteHTTPSAction(w http.ResponseWriter, r *http.Request, enabled bool) {
    id := r.PathValue("id")
    wm, ok := s.store.GetWebsiteMapping(id)
    if !ok {
        s.writeError(w, r, http.StatusNotFound, "WEBSITE_MAPPING_NOT_FOUND", "website mapping not found", false, map[string]any{"id": id})
        return
    }
    action := "disable-https"
    kind := "website_mapping.disable_https"
    level := "info"
    message := "disable HTTPS: " + wm.PrimaryDomain
    if enabled {
        action = "enable-https"
        kind = "website_mapping.enable_https"
        message = "enable HTTPS: " + wm.PrimaryDomain
    }
    subject := &domain.SubjectRef{Type: "website_mapping", ID: wm.ID, Name: wm.PrimaryDomain}
    summary := s.enqueueJob(
        kind,
        "website_mapping",
        wm.ID,
        action+": "+wm.PrimaryDomain,
        "subject:website_mapping:"+wm.ID,
        map[string]any{"enabled": enabled},
        subject,
        func() (any, *domain.APIError) {
            updated, err := s.store.UpdateWebsiteMapping(wm.ID, func(item *domain.WebsiteMapping) error {
                item.HTTPSEnabled = enabled
                item.Status = domain.WebsiteStatusSynced
                now := time.Now().UTC()
                item.LastSyncedAt = &now
                return nil
            })
            if err != nil {
                return nil, &domain.APIError{Code: "WEBSITE_MAPPING_UPDATE_FAILED", Message: err.Error(), Retryable: true}
            }
            return map[string]any{"website_mapping": updated}, nil
        },
    )
    s.serveJSON(w, r, http.StatusAccepted, map[string]any{"accepted": true, "action": action, "level": level, "message": message}, &summary)
}

func (s *Server) handleWebsiteMappingArchive(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    wm, err := s.store.UpdateWebsiteMapping(id, func(item *domain.WebsiteMapping) error {
        item.Status = domain.WebsiteStatusArchived
        return nil
    })
    if err != nil {
        s.writeError(w, r, http.StatusNotFound, "WEBSITE_MAPPING_NOT_FOUND", err.Error(), false, map[string]any{"id": id})
        return
    }
    s.emit("website_mapping.archived", "subject:website_mapping:"+wm.ID, "info", "website mapping archived", nil, &domain.SubjectRef{Type: "website_mapping", ID: wm.ID, Name: wm.PrimaryDomain}, wm, nil)
    s.serveJSON(w, r, http.StatusOK, wm, nil)
}

func (s *Server) handleSettingsGet(w http.ResponseWriter, r *http.Request) {
    s.serveJSON(w, r, http.StatusOK, s.store.Settings(), nil)
}

func (s *Server) handleSettingsPatch(w http.ResponseWriter, r *http.Request) {
    var settings domain.Settings
    if !s.decodeJSON(w, r, &settings) {
        return
    }
    updated, err := s.store.UpdateSettings(settings)
    if err != nil {
        s.writeError(w, r, http.StatusInternalServerError, "SETTINGS_SAVE_FAILED", err.Error(), false, nil)
        return
    }
    s.emit("settings.updated", "account:current", "info", "settings updated", nil, &domain.SubjectRef{Type: "settings", ID: "current", Name: "settings"}, updated, nil)
    s.serveJSON(w, r, http.StatusOK, updated, nil)
}

func (s *Server) handleJobsList(w http.ResponseWriter, r *http.Request) {
    jobs := s.store.ListJobs(
        r.URL.Query().Get("status"),
        r.URL.Query().Get("kind"),
        r.URL.Query().Get("target_type"),
        r.URL.Query().Get("target_id"),
    )
    s.serveJSON(w, r, http.StatusOK, map[string]any{"jobs": jobs}, nil)
}

func (s *Server) handleJobGet(w http.ResponseWriter, r *http.Request) {
    id := r.PathValue("id")
    job, ok := s.store.GetJob(id)
    if !ok {
        s.writeError(w, r, http.StatusNotFound, "JOB_NOT_FOUND", "job not found", false, map[string]any{"id": id})
        return
    }
    s.serveJSON(w, r, http.StatusOK, job, nil)
}

func (s *Server) handleRuntimeGet(w http.ResponseWriter, r *http.Request) {
    s.serveJSON(w, r, http.StatusOK, s.frpc.Summary(), nil)
}

func (s *Server) handleRuntimeStart(w http.ResponseWriter, r *http.Request) {
    s.runtimeAction(w, r, "runtime.start", "start", func() any { return s.frpc.Start("runtime") })
}

func (s *Server) handleRuntimeStop(w http.ResponseWriter, r *http.Request) {
    s.runtimeAction(w, r, "runtime.stop", "stop", func() any { return s.frpc.Stop("runtime") })
}

func (s *Server) handleRuntimeRestart(w http.ResponseWriter, r *http.Request) {
    s.runtimeAction(w, r, "runtime.restart", "restart", func() any { return s.frpc.Restart("runtime") })
}

func (s *Server) handleRuntimeReload(w http.ResponseWriter, r *http.Request) {
    s.runtimeAction(w, r, "runtime.reload", "reload", func() any { return s.frpc.Reload() })
}

func (s *Server) handleRuntimeSwitchNode(w http.ResponseWriter, r *http.Request) {
    var body struct { NodeID string `json:"node_id"` }
    if !s.decodeJSON(w, r, &body) {
        return
    }
    if strings.TrimSpace(body.NodeID) == "" {
        s.writeError(w, r, http.StatusBadRequest, "NODE_ID_REQUIRED", "node_id 不能为空", false, nil)
        return
    }
    s.runtimeAction(w, r, "runtime.switch-node", "switch-node", func() any { return s.frpc.SwitchNode(body.NodeID) })
}
func (s *Server) runtimeAction(w http.ResponseWriter, r *http.Request, kind, action string, run func() any) {
    payload := map[string]any{"action": action}
    summary := s.enqueueJob(kind, "runtime", "frpc", "frpc runtime "+action, "runtime:frpc", payload, &domain.SubjectRef{Type: "runtime", ID: "frpc", Name: "frpc"}, func() (any, *domain.APIError) {
        return run(), nil
    })
    s.serveJSON(w, r, http.StatusAccepted, map[string]any{"accepted": true, "action": action}, &summary)
}
