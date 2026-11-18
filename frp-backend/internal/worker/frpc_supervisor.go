package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"time"

	"ashan-frp/internal/domain"
	"ashan-frp/internal/frpc"
	"ashan-frp/internal/integration/chmlfrp"
	"ashan-frp/internal/repository"
	"ashan-frp/internal/security"
)

type FrpcSupervisor struct {
	repo           *repository.Repository
	mgr            *frpc.Manager
	key            []byte
	pollInterval   time.Duration
	stopCh         chan struct{}
	lastConfigHash string
}

func NewFrpcSupervisor(repo *repository.Repository, mgr *frpc.Manager, key []byte) *FrpcSupervisor {
	return &FrpcSupervisor{
		repo:         repo,
		mgr:          mgr,
		key:          key,
		pollInterval: 15 * time.Second,
		stopCh:       make(chan struct{}),
	}
}

func (s *FrpcSupervisor) Start() {
	slog.Info("frpc_supervisor.started")
	go s.loop()
}

func (s *FrpcSupervisor) Stop() {
	close(s.stopCh)
}

func (s *FrpcSupervisor) loop() {
	// First tick immediately after 2 seconds
	select {
	case <-s.stopCh:
		return
	case <-time.After(2 * time.Second):
		s.checkAndEnsureRunning()
	}

	ticker := time.NewTicker(s.pollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkAndEnsureRunning()
		}
	}
}

func (s *FrpcSupervisor) checkAndEnsureRunning() {
	status := s.mgr.Status()

	// Fetch enabled tunnels
	tunnels, err := s.repo.ListTunnels(repository.TunnelFilter{})
	if err != nil {
		slog.Error("frpc_supervisor.list_tunnels_failed", "error", err)
		return
	}

	enabled := make([]domain.Tunnel, 0, len(tunnels))
	for _, t := range tunnels {
		if t.DesiredState == domain.TunnelDesiredEnabled {
			enabled = append(enabled, t)
		}
	}

	if len(enabled) == 0 {
		return
	}

	// Prepare renderer settings
	s.prepareRenderer(enabled)

	// Calculate target config hash to detect any modifications
	cfg, err := s.mgr.Renderer().Render(enabled)
	var currentHash string
	if err == nil {
		raw, _ := json.Marshal(cfg)
		h := sha256.Sum256(raw)
		currentHash = hex.EncodeToString(h[:])
	}

	// If already running but configuration has changed (e.g. tunnels edited, ports modified, or node switched),
	// automatically reload/restart to apply the changes seamlessly.
	if status == frpc.StatusRunning {
		if s.lastConfigHash != "" && currentHash != "" && s.lastConfigHash != currentHash {
			slog.Info("frpc_supervisor.config_changed_reloading", "prev_hash", s.lastConfigHash, "new_hash", currentHash)
			if err := s.mgr.Restart(context.Background(), enabled); err != nil {
				slog.Error("frpc_supervisor.reload_failed", "error", err)
			} else {
				s.lastConfigHash = currentHash
				slog.Info("frpc_supervisor.reload_success")
			}
		}
		return
	}

	if status == frpc.StatusStarting || status == frpc.StatusRestarting {
		return
	}

	slog.Info("frpc_supervisor.recovering_client", "status", status, "enabled_tunnels", len(enabled))

	if err := s.mgr.Start(context.Background(), enabled); err != nil {
		slog.Error("frpc_supervisor.start_failed", "error", err)
	} else {
		s.lastConfigHash = currentHash
		slog.Info("frpc_supervisor.start_success")
	}
}

func (s *FrpcSupervisor) prepareRenderer(tunnels []domain.Tunnel) {
	serverAddr := ""
	serverPort := 7000
	user := ""
	authToken := "ChmlFrpToken"

	// 1. Get user token from chmlfrp credentials
	if cred, err := s.repo.FindCredentialByProvider("chmlfrp"); err == nil && cred != nil && cred.EncryptedSecret != "" {
		if rawSecret, err := security.Decrypt(cred.EncryptedSecret, s.key); err == nil {
			var tokenData struct {
				AccessToken string `json:"access_token"`
				UserToken   string `json:"user_token"`
			}
			if json.Unmarshal(rawSecret, &tokenData) == nil {
				if tokenData.UserToken != "" {
					user = tokenData.UserToken
				}
				if user == "" && tokenData.AccessToken != "" {
					client := chmlfrp.NewClient("token", tokenData.AccessToken)
					if uToken, err := client.GetUserInfo(); err == nil && uToken != "" {
						user = uToken
						tokenData.UserToken = uToken
						if updatedSecret, err := json.Marshal(tokenData); err == nil {
							if enc, err := security.Encrypt(updatedSecret, s.key); err == nil {
								cred.EncryptedSecret = enc
								_ = s.repo.UpsertCredential(cred)
							}
						}
					}
				}
			}
		}
	}

	// 2. Discover node server address
	nodeName := ""
	for _, t := range tunnels {
		if t.ChmlfrpNode != "" {
			nodeName = t.ChmlfrpNode
			break
		}
	}
	if nodeName != "" {
		nodes, _ := s.repo.ListNodes(repository.NodeFilter{})
		for _, n := range nodes {
			if n.DisplayName == nodeName || n.CanonicalName == nodeName {
				if n.RealIP != "" {
					serverAddr = n.RealIP
				} else if n.EndpointURL != "" {
					serverAddr = n.EndpointURL
				}
				break
			}
		}
	}

	if serverAddr == "" {
		serverAddr = "82.156.215.91"
	}

	s.mgr.Renderer().SetServer(serverAddr, serverPort, user, authToken)
}
