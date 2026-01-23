package internal

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/infra/acme"
	"github.com/anthropics/agentsmesh/backend/internal/service/agentpod"
	"github.com/anthropics/agentsmesh/backend/internal/service/relay"
	"github.com/anthropics/agentsmesh/backend/internal/service/runner"
	"github.com/gin-gonic/gin"
)

// RelayHandler handles internal relay API endpoints
type RelayHandler struct {
	relayManager  *relay.Manager
	dnsService    *relay.DNSService
	acmeManager   *acme.Manager
	commandSender runner.RunnerCommandSender
	podService    *agentpod.PodService
	logger        *slog.Logger
}

// NewRelayHandler creates a new relay handler
func NewRelayHandler(relayManager *relay.Manager, dnsService *relay.DNSService, acmeManager *acme.Manager, commandSender runner.RunnerCommandSender, podService *agentpod.PodService) *RelayHandler {
	return &RelayHandler{
		relayManager:  relayManager,
		dnsService:    dnsService,
		acmeManager:   acmeManager,
		commandSender: commandSender,
		podService:    podService,
		logger:        slog.With("component", "relay_handler"),
	}
}

// RegisterRequest is the relay registration request
type RegisterRequest struct {
	RelayID     string `json:"relay_id" binding:"required"`
	RelayName   string `json:"relay_name"`                      // Name for DNS auto-registration (e.g., "us-east-1")
	IP          string `json:"ip"`                              // Public IP for DNS auto-registration
	URL         string `json:"url"`                             // Public URL for browsers (optional if DNS auto)
	InternalURL string `json:"internal_url"`                    // Internal URL for runners (Docker network)
	Region      string `json:"region"`
	Capacity    int    `json:"capacity"`
}

// RegisterResponse is the relay registration response
type RegisterResponse struct {
	Status      string `json:"status"`
	URL         string `json:"url,omitempty"`          // Generated URL (if DNS auto-registration)
	InternalURL string `json:"internal_url,omitempty"` // Echoed back for confirmation
	DNSCreated  bool   `json:"dns_created,omitempty"`  // Whether DNS record was created

	// TLS certificate (if ACME is enabled)
	TLSCert   string `json:"tls_cert,omitempty"`   // PEM encoded certificate chain
	TLSKey    string `json:"tls_key,omitempty"`    // PEM encoded private key
	TLSExpiry string `json:"tls_expiry,omitempty"` // Certificate expiry time (RFC3339)
}

// HeartbeatRequest is the relay heartbeat request
type HeartbeatRequest struct {
	RelayID     string  `json:"relay_id" binding:"required"`
	Connections int     `json:"connections"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	LatencyMs   int     `json:"latency_ms,omitempty"` // Heartbeat round-trip latency in milliseconds
}

// SessionClosedRequest is the session closed notification
type SessionClosedRequest struct {
	PodKey    string `json:"pod_key" binding:"required"`
	SessionID string `json:"session_id"`
}

// UnregisterRequest is the relay unregistration request (graceful shutdown)
type UnregisterRequest struct {
	RelayID string `json:"relay_id" binding:"required"`
	Reason  string `json:"reason,omitempty"` // shutdown, maintenance, etc.
}

// Register handles relay registration
// POST /api/internal/relays/register
func (h *RelayHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	response := RegisterResponse{
		Status: "registered",
	}

	url := req.URL
	dnsCreated := false

	// Handle DNS auto-registration if relay_name and IP provided and DNS service enabled
	if req.RelayName != "" && req.IP != "" && h.dnsService != nil && h.dnsService.IsEnabled() {
		// Create/update DNS record
		if err := h.dnsService.CreateRecord(c.Request.Context(), req.RelayName, req.IP); err != nil {
			h.logger.Error("Failed to create DNS record",
				"relay_name", req.RelayName,
				"ip", req.IP,
				"error", err)
			// Don't fail registration, just log the error
			// Relay can still work if URL is provided manually
		} else {
			// Generate URL from DNS
			url = h.dnsService.GenerateRelayURL(req.RelayName)
			dnsCreated = true
			h.logger.Info("DNS record created for relay",
				"relay_name", req.RelayName,
				"ip", req.IP,
				"url", url)
		}
	}

	// Validate that we have a URL (either provided or generated)
	if url == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "url is required when DNS auto-registration is not available",
		})
		return
	}

	info := &relay.RelayInfo{
		ID:          req.RelayID,
		URL:         url,
		InternalURL: req.InternalURL,
		Region:      req.Region,
		Capacity:    req.Capacity,
	}

	if info.Capacity == 0 {
		info.Capacity = 1000 // Default capacity
	}

	if info.Region == "" {
		info.Region = "default"
	}

	if err := h.relayManager.Register(info); err != nil {
		h.logger.Error("Failed to register relay", "relay_id", req.RelayID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register relay"})
		return
	}

	h.logger.Info("Relay registered",
		"relay_id", req.RelayID,
		"url", url,
		"region", req.Region,
		"dns_created", dnsCreated)

	response.URL = url
	response.InternalURL = req.InternalURL
	response.DNSCreated = dnsCreated

	// Include TLS certificate if ACME is enabled and certificate is available
	if h.acmeManager != nil {
		cert, key, expiry, err := h.acmeManager.GetCertificatePEM()
		if err == nil && cert != "" {
			response.TLSCert = cert
			response.TLSKey = key
			response.TLSExpiry = expiry.Format(time.RFC3339)
			h.logger.Info("TLS certificate included in registration response",
				"relay_id", req.RelayID,
				"cert_expiry", expiry)
		} else if err != nil {
			h.logger.Warn("ACME certificate not available",
				"relay_id", req.RelayID,
				"error", err)
		}
	}

	c.JSON(http.StatusOK, response)
}

// Heartbeat handles relay heartbeat
// POST /api/internal/relays/heartbeat
func (h *RelayHandler) Heartbeat(c *gin.Context) {
	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.relayManager.HeartbeatWithLatency(req.RelayID, req.Connections, req.CPUUsage, req.MemoryUsage, req.LatencyMs); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Unregister handles graceful relay unregistration
// POST /api/internal/relays/unregister
func (h *RelayHandler) Unregister(c *gin.Context) {
	var req UnregisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get relay info before unregistering
	relay := h.relayManager.GetRelayByID(req.RelayID)
	if relay == nil {
		// Relay not found, but that's OK for unregister (idempotent)
		h.logger.Info("Unregister request for unknown relay",
			"relay_id", req.RelayID,
			"reason", req.Reason)
		c.JSON(http.StatusOK, gin.H{"status": "not_found"})
		return
	}

	// Gracefully unregister - mark as offline but don't remove
	affectedSessions := h.relayManager.GracefulUnregister(req.RelayID, req.Reason)

	h.logger.Info("Relay gracefully unregistered",
		"relay_id", req.RelayID,
		"reason", req.Reason,
		"affected_sessions", len(affectedSessions))

	// Notify runners to reconnect affected pods to other relays
	if len(affectedSessions) > 0 && h.commandSender != nil && h.podService != nil {
		migratedCount := 0
		for _, session := range affectedSessions {
			pod, err := h.podService.GetPod(c.Request.Context(), session.PodKey)
			if err != nil || pod == nil || pod.RunnerID == 0 {
				continue
			}
			// Send unsubscribe to trigger reconnection to new relay
			if err := h.commandSender.SendUnsubscribeTerminal(c.Request.Context(), pod.RunnerID, session.PodKey); err != nil {
				h.logger.Warn("Failed to send unsubscribe for session migration",
					"pod_key", session.PodKey,
					"error", err)
			} else {
				migratedCount++
			}
		}
		h.logger.Info("Session migration triggered after graceful unregister",
			"total", len(affectedSessions),
			"migrated", migratedCount)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":            "unregistered",
		"reason":            req.Reason,
		"affected_sessions": len(affectedSessions),
	})
}

// SessionClosed handles session closed notification from relay
// POST /api/internal/relays/session-closed
func (h *RelayHandler) SessionClosed(c *gin.Context) {
	var req SessionClosedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.logger.Info("Session closed notification received",
		"pod_key", req.PodKey,
		"session_id", req.SessionID)

	// Get the pod info to find runner
	session := h.relayManager.GetSession(req.PodKey)
	if session == nil {
		// Session already removed
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
		return
	}

	// Get pod to find runner ID
	if h.podService != nil && h.commandSender != nil {
		pod, err := h.podService.GetPod(c.Request.Context(), req.PodKey)
		if err == nil && pod != nil && pod.RunnerID > 0 {
			if err := h.commandSender.SendUnsubscribeTerminal(c.Request.Context(), pod.RunnerID, req.PodKey); err != nil {
				h.logger.Warn("Failed to send unsubscribe terminal",
					"pod_key", req.PodKey,
					"runner_id", pod.RunnerID,
					"error", err)
			}
		}
	}

	// Remove the session
	h.relayManager.RemoveSession(req.PodKey)

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Stats returns relay statistics
// GET /api/internal/relays/stats
func (h *RelayHandler) Stats(c *gin.Context) {
	stats := h.relayManager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// List returns all registered relays
// GET /api/internal/relays
func (h *RelayHandler) List(c *gin.Context) {
	relays := h.relayManager.GetRelays()
	c.JSON(http.StatusOK, gin.H{"relays": relays})
}

// Get returns a specific relay by ID
// GET /api/internal/relays/:relay_id
func (h *RelayHandler) Get(c *gin.Context) {
	relayID := c.Param("relay_id")
	if relayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "relay_id is required"})
		return
	}

	relay := h.relayManager.GetRelayByID(relayID)
	if relay == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "relay not found"})
		return
	}

	c.JSON(http.StatusOK, relay)
}

// ForceUnregisterRequest is the request for force unregister
type ForceUnregisterRequest struct {
	MigrateSessions bool `json:"migrate_sessions"` // Whether to migrate affected sessions
}

// MigrateSessionRequest is the request for session migration
type MigrateSessionRequest struct {
	PodKey       string `json:"pod_key" binding:"required"`
	TargetRelay  string `json:"target_relay" binding:"required"` // Target relay ID
}

// BulkMigrateRequest is the request for bulk session migration
type BulkMigrateRequest struct {
	SourceRelay string `json:"source_relay" binding:"required"` // Source relay ID
	TargetRelay string `json:"target_relay" binding:"required"` // Target relay ID
}

// ForceUnregister removes a relay and optionally migrates sessions
// DELETE /api/internal/relays/:relay_id
func (h *RelayHandler) ForceUnregister(c *gin.Context) {
	relayID := c.Param("relay_id")
	if relayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "relay_id is required"})
		return
	}

	// Check if relay exists
	relay := h.relayManager.GetRelayByID(relayID)
	if relay == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "relay not found"})
		return
	}

	var req ForceUnregisterRequest
	// Parse optional body, ignore errors for empty body
	_ = c.ShouldBindJSON(&req)

	// Force unregister and get affected sessions
	affectedSessions := h.relayManager.ForceUnregister(relayID)

	h.logger.Info("Relay force unregistered",
		"relay_id", relayID,
		"affected_sessions", len(affectedSessions),
		"migrate_sessions", req.MigrateSessions)

	// Optionally notify runners to reconnect affected pods
	if req.MigrateSessions && len(affectedSessions) > 0 && h.commandSender != nil && h.podService != nil {
		migratedCount := 0
		for _, session := range affectedSessions {
			pod, err := h.podService.GetPod(c.Request.Context(), session.PodKey)
			if err != nil || pod == nil || pod.RunnerID == 0 {
				continue
			}
			// Send unsubscribe to trigger reconnection to new relay
			if err := h.commandSender.SendUnsubscribeTerminal(c.Request.Context(), pod.RunnerID, session.PodKey); err != nil {
				h.logger.Warn("Failed to send unsubscribe for session migration",
					"pod_key", session.PodKey,
					"error", err)
			} else {
				migratedCount++
			}
		}
		h.logger.Info("Session migration triggered",
			"total", len(affectedSessions),
			"migrated", migratedCount)
	}

	c.JSON(http.StatusOK, gin.H{
		"status":            "unregistered",
		"affected_sessions": len(affectedSessions),
	})
}

// ListSessions returns all active sessions
// GET /api/internal/relays/sessions
func (h *RelayHandler) ListSessions(c *gin.Context) {
	relayID := c.Query("relay_id")

	var sessions []*relay.ActiveSession
	if relayID != "" {
		sessions = h.relayManager.GetSessionsByRelay(relayID)
	} else {
		sessions = h.relayManager.GetAllSessions()
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// MigrateSession migrates a single session to a new relay
// POST /api/internal/relays/sessions/migrate
func (h *RelayHandler) MigrateSession(c *gin.Context) {
	var req MigrateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get target relay
	targetRelay := h.relayManager.GetRelayByID(req.TargetRelay)
	if targetRelay == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "target relay not found"})
		return
	}

	if !targetRelay.Healthy {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target relay is not healthy"})
		return
	}

	// Migrate the session
	newSession, oldRelayID, err := h.relayManager.MigrateSession(req.PodKey, targetRelay)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Notify runner to reconnect
	if h.commandSender != nil && h.podService != nil {
		pod, err := h.podService.GetPod(c.Request.Context(), req.PodKey)
		if err == nil && pod != nil && pod.RunnerID > 0 {
			if err := h.commandSender.SendUnsubscribeTerminal(c.Request.Context(), pod.RunnerID, req.PodKey); err != nil {
				h.logger.Warn("Failed to send unsubscribe for migration",
					"pod_key", req.PodKey,
					"error", err)
			}
		}
	}

	h.logger.Info("Session migrated",
		"pod_key", req.PodKey,
		"from_relay", oldRelayID,
		"to_relay", req.TargetRelay)

	c.JSON(http.StatusOK, gin.H{
		"status":     "migrated",
		"session":    newSession,
		"from_relay": oldRelayID,
		"to_relay":   req.TargetRelay,
	})
}

// BulkMigrateSessions migrates all sessions from one relay to another
// POST /api/internal/relays/sessions/bulk-migrate
func (h *RelayHandler) BulkMigrateSessions(c *gin.Context) {
	var req BulkMigrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get target relay
	targetRelay := h.relayManager.GetRelayByID(req.TargetRelay)
	if targetRelay == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "target relay not found"})
		return
	}

	if !targetRelay.Healthy {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target relay is not healthy"})
		return
	}

	// Get all sessions on source relay
	sessions := h.relayManager.GetSessionsByRelay(req.SourceRelay)
	if len(sessions) == 0 {
		c.JSON(http.StatusOK, gin.H{
			"status":   "no_sessions",
			"migrated": 0,
		})
		return
	}

	// Migrate each session
	migrated := 0
	failed := 0
	for _, session := range sessions {
		_, _, err := h.relayManager.MigrateSession(session.PodKey, targetRelay)
		if err != nil {
			failed++
			continue
		}

		// Notify runner to reconnect
		if h.commandSender != nil && h.podService != nil {
			pod, err := h.podService.GetPod(c.Request.Context(), session.PodKey)
			if err == nil && pod != nil && pod.RunnerID > 0 {
				_ = h.commandSender.SendUnsubscribeTerminal(c.Request.Context(), pod.RunnerID, session.PodKey)
			}
		}
		migrated++
	}

	h.logger.Info("Bulk session migration completed",
		"source_relay", req.SourceRelay,
		"target_relay", req.TargetRelay,
		"total", len(sessions),
		"migrated", migrated,
		"failed", failed)

	c.JSON(http.StatusOK, gin.H{
		"status":   "completed",
		"total":    len(sessions),
		"migrated": migrated,
		"failed":   failed,
	})
}

// RelayRouterDeps holds dependencies for relay routes
type RelayRouterDeps struct {
	RelayManager   *relay.Manager
	DNSService     *relay.DNSService
	ACMEManager    *acme.Manager
	CommandSender  runner.RunnerCommandSender
	PodService     *agentpod.PodService
	InternalSecret string
}

// RegisterRelayRoutes registers relay API routes
func RegisterRelayRoutes(router *gin.RouterGroup, deps *RelayRouterDeps) {
	handler := NewRelayHandler(deps.RelayManager, deps.DNSService, deps.ACMEManager, deps.CommandSender, deps.PodService)

	// Internal API authentication middleware
	router.Use(InternalAPIAuth(deps.InternalSecret))

	router.POST("/register", handler.Register)
	router.POST("/heartbeat", handler.Heartbeat)
	router.POST("/unregister", handler.Unregister)
	router.POST("/session-closed", handler.SessionClosed)
	router.GET("/stats", handler.Stats)
	router.GET("", handler.List)
	router.GET("/:relay_id", handler.Get)
	router.DELETE("/:relay_id", handler.ForceUnregister)

	// Session management
	router.GET("/sessions", handler.ListSessions)
	router.POST("/sessions/migrate", handler.MigrateSession)
	router.POST("/sessions/bulk-migrate", handler.BulkMigrateSessions)
}

// InternalAPIAuth is middleware for internal API authentication
func InternalAPIAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("X-Internal-Secret")
		if auth != secret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}
