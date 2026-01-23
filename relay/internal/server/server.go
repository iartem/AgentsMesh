package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/anthropics/agentsmesh/relay/internal/auth"
	"github.com/anthropics/agentsmesh/relay/internal/backend"
	"github.com/anthropics/agentsmesh/relay/internal/config"
	"github.com/anthropics/agentsmesh/relay/internal/session"
)

// Server is the main relay server
type Server struct {
	cfg            *config.Config
	httpServer     *http.Server
	sessionManager *session.Manager
	backendClient  *backend.Client
	handler        *Handler

	// Graceful shutdown control
	acceptingConnections bool

	logger *slog.Logger
}

// New creates a new relay server
func New(cfg *config.Config) *Server {
	// Create backend client with full configuration
	backendClient := backend.NewClientWithConfig(backend.ClientConfig{
		BaseURL:           cfg.Backend.URL,
		InternalAPISecret: cfg.Backend.InternalAPISecret,
		RelayID:           cfg.Relay.ID,
		RelayName:         cfg.Relay.Name,
		RelayURL:          cfg.Relay.URL,
		RelayInternalURL:  cfg.Relay.InternalURL,
		RelayRegion:       cfg.Relay.Region,
		RelayCapacity:     cfg.Relay.Capacity,
		AutoIP:            cfg.Relay.AutoIP,
	})

	// Create server instance first (for closure capture)
	s := &Server{
		cfg:                  cfg,
		backendClient:        backendClient,
		acceptingConnections: true,
		logger:               slog.With("component", "server"),
	}

	// Create callback for when all browsers leave a session
	// Uses closure to capture server instance instead of global variable
	onAllBrowsersGone := func(podKey string) {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Find session ID for this pod using server's session manager
		sessionID := ""
		if sess := s.sessionManager.GetSessionByPodKey(podKey); sess != nil {
			sessionID = sess.ID
		}

		if err := s.backendClient.NotifySessionClosed(ctx, podKey, sessionID); err != nil {
			s.logger.Warn("Failed to notify backend of session closed", "pod_key", podKey, "error", err)
		}
	}

	// Create session manager with full configuration
	managerCfg := session.ManagerConfig{
		KeepAliveDuration:        cfg.Session.KeepAliveDuration,
		MaxBrowsersPerPod:        cfg.Session.MaxBrowsersPerPod,
		RunnerReconnectTimeout:   cfg.Session.RunnerReconnectTimeout,
		BrowserReconnectTimeout:  cfg.Session.BrowserReconnectTimeout,
		PendingConnectionTimeout: cfg.Session.PendingConnectionTimeout,
		OutputBufferSize:         cfg.Session.OutputBufferSize,
		OutputBufferCount:        cfg.Session.OutputBufferCount,
	}
	s.sessionManager = session.NewManagerWithConfig(managerCfg, onAllBrowsersGone)

	// Create token validator
	tokenValidator := auth.NewTokenValidator(cfg.JWT.Secret, cfg.JWT.Issuer)

	// Create handler
	s.handler = NewHandler(s.sessionManager, tokenValidator)

	return s
}

// Start starts the relay server
func (s *Server) Start(ctx context.Context) error {
	// Register with backend
	if err := s.backendClient.Register(ctx); err != nil {
		return fmt.Errorf("failed to register with backend: %w", err)
	}

	// Start heartbeat loop
	go s.backendClient.StartHeartbeat(ctx, s.cfg.Backend.HeartbeatInterval, func() int {
		stats := s.sessionManager.Stats()
		return stats.ActiveSessions
	})

	// Set up HTTP routes
	mux := http.NewServeMux()
	mux.HandleFunc("/runner/terminal", s.handler.HandleRunnerWS)
	mux.HandleFunc("/browser/terminal", s.handler.HandleBrowserWS)
	mux.HandleFunc("/health", s.handler.HandleHealth)
	mux.HandleFunc("/stats", s.handler.HandleStats)

	s.httpServer = &http.Server{
		Addr:         s.cfg.Server.Address(),
		Handler:      mux,
		ReadTimeout:  s.cfg.Server.ReadTimeout,
		WriteTimeout: s.cfg.Server.WriteTimeout,
	}

	// Start server in goroutine
	errCh := make(chan error, 1)

	// Check if we should use TLS
	useTLS := s.cfg.Server.TLS.Enabled

	// Check if backend provided a TLS certificate (ACME)
	if s.backendClient.HasTLSCertificate() {
		certPEM, keyPEM := s.backendClient.GetTLSCertificate()
		cert, err := tls.X509KeyPair([]byte(certPEM), []byte(keyPEM))
		if err != nil {
			s.logger.Error("Failed to load TLS certificate from backend", "error", err)
		} else {
			s.httpServer.TLSConfig = &tls.Config{
				Certificates: []tls.Certificate{cert},
			}
			useTLS = true
			s.logger.Info("Using TLS certificate from backend (ACME)",
				"expiry", s.backendClient.GetTLSExpiry())
		}
	}

	if useTLS {
		if s.httpServer.TLSConfig != nil {
			// Use certificate from backend (already loaded in TLSConfig)
			s.logger.Info("Starting relay server with TLS (certificate from backend)",
				"address", s.cfg.Server.Address())
			go func() {
				// ListenAndServeTLS with empty cert/key paths uses TLSConfig
				if err := s.httpServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
					errCh <- err
				}
			}()
		} else {
			// Use certificate files from config
			s.logger.Info("Starting relay server with TLS",
				"address", s.cfg.Server.Address(),
				"cert_file", s.cfg.Server.TLS.CertFile)
			go func() {
				if err := s.httpServer.ListenAndServeTLS(
					s.cfg.Server.TLS.CertFile,
					s.cfg.Server.TLS.KeyFile,
				); err != nil && err != http.ErrServerClosed {
					errCh <- err
				}
			}()
		}
	} else {
		s.logger.Info("Starting relay server", "address", s.cfg.Server.Address())
		go func() {
			if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				errCh <- err
			}
		}()
	}

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for context cancellation, signal, or error
	select {
	case <-ctx.Done():
		return s.gracefulShutdown("context_cancelled")
	case sig := <-sigCh:
		return s.gracefulShutdown(fmt.Sprintf("signal_%s", sig))
	case err := <-errCh:
		return err
	}
}

// gracefulShutdown performs a graceful shutdown of the relay server
func (s *Server) gracefulShutdown(reason string) error {
	s.logger.Info("Starting graceful shutdown...", "reason", reason)

	// 1. Notify backend that this relay is going offline
	unregCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := s.backendClient.Unregister(unregCtx, reason); err != nil {
		s.logger.Warn("Failed to unregister from backend", "error", err)
	}
	cancel()

	// 2. Stop accepting new connections
	s.acceptingConnections = false
	s.logger.Info("Stopped accepting new connections")

	// 3. Wait for existing sessions to close (with timeout)
	gracePeriod := 30 * time.Second
	deadline := time.Now().Add(gracePeriod)

	for time.Now().Before(deadline) {
		stats := s.sessionManager.Stats()
		if stats.ActiveSessions == 0 {
			s.logger.Info("All sessions closed")
			break
		}
		s.logger.Info("Waiting for sessions to close",
			"remaining", stats.ActiveSessions,
			"time_left", time.Until(deadline).Round(time.Second))
		time.Sleep(1 * time.Second)
	}

	// 4. Shutdown HTTP server
	s.logger.Info("Shutting down HTTP server...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.logger.Error("HTTP server shutdown error", "error", err)
		return err
	}

	s.logger.Info("Graceful shutdown completed")
	return nil
}

// IsAcceptingConnections returns whether the server is accepting new connections
func (s *Server) IsAcceptingConnections() bool {
	return s.acceptingConnections
}

// Stats returns server statistics
func (s *Server) Stats() session.SessionStats {
	return s.sessionManager.Stats()
}
