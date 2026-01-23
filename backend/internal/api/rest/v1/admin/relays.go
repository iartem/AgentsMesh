package admin

import (
	"net/http"

	"github.com/anthropics/agentsmesh/backend/internal/domain/admin"
	"github.com/anthropics/agentsmesh/backend/internal/service/agentpod"
	adminservice "github.com/anthropics/agentsmesh/backend/internal/service/admin"
	"github.com/anthropics/agentsmesh/backend/internal/service/relay"
	"github.com/anthropics/agentsmesh/backend/internal/service/runner"

	"github.com/gin-gonic/gin"
)

// RelayHandler handles relay management requests
type RelayHandler struct {
	adminService  *adminservice.Service
	relayManager  *relay.Manager
	commandSender runner.RunnerCommandSender
	podService    *agentpod.PodService
}

// NewRelayHandler creates a new relay handler
func NewRelayHandler(
	adminSvc *adminservice.Service,
	relayMgr *relay.Manager,
	cmdSender runner.RunnerCommandSender,
	podSvc *agentpod.PodService,
) *RelayHandler {
	return &RelayHandler{
		adminService:  adminSvc,
		relayManager:  relayMgr,
		commandSender: cmdSender,
		podService:    podSvc,
	}
}

// RegisterRoutes registers relay management routes
func (h *RelayHandler) RegisterRoutes(rg *gin.RouterGroup) {
	relaysGroup := rg.Group("/relays")
	{
		relaysGroup.GET("", h.ListRelays)
		relaysGroup.GET("/stats", h.GetStats)
		relaysGroup.GET("/:id", h.GetRelay)
		relaysGroup.DELETE("/:id", h.ForceUnregister)

		// Session management
		relaysGroup.GET("/sessions", h.ListSessions)
		relaysGroup.POST("/sessions/migrate", h.MigrateSession)
		relaysGroup.POST("/sessions/bulk-migrate", h.BulkMigrateSessions)
	}
}

// logAction is a helper method for audit logging
func (h *RelayHandler) logAction(c *gin.Context, action admin.AuditAction, targetType admin.TargetType, targetID int64, oldData, newData interface{}) {
	LogAdminAction(c, h.adminService, action, targetType, targetID, oldData, newData)
}

// ListRelays returns all registered relays
func (h *RelayHandler) ListRelays(c *gin.Context) {
	if h.relayManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Relay manager not available"})
		return
	}

	relays := h.relayManager.GetRelays()

	c.JSON(http.StatusOK, gin.H{
		"data":  relays,
		"total": len(relays),
	})
}

// GetStats returns relay statistics
func (h *RelayHandler) GetStats(c *gin.Context) {
	if h.relayManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Relay manager not available"})
		return
	}

	stats := h.relayManager.GetStats()

	c.JSON(http.StatusOK, stats)
}

// GetRelay returns a specific relay by ID
func (h *RelayHandler) GetRelay(c *gin.Context) {
	if h.relayManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Relay manager not available"})
		return
	}

	relayID := c.Param("id")
	if relayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Relay ID is required"})
		return
	}

	relayInfo := h.relayManager.GetRelayByID(relayID)
	if relayInfo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Relay not found"})
		return
	}

	// Also get sessions on this relay
	sessions := h.relayManager.GetSessionsByRelay(relayID)

	c.JSON(http.StatusOK, gin.H{
		"relay":          relayInfo,
		"session_count":  len(sessions),
		"sessions":       sessions,
	})
}

// ForceUnregisterRequest is the request for force unregister
type ForceUnregisterRequest struct {
	MigrateSessions bool `json:"migrate_sessions"`
}

// ForceUnregister removes a relay and optionally migrates sessions
func (h *RelayHandler) ForceUnregister(c *gin.Context) {
	if h.relayManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Relay manager not available"})
		return
	}

	relayID := c.Param("id")
	if relayID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Relay ID is required"})
		return
	}

	// Get relay info before deletion for logging
	relayInfo := h.relayManager.GetRelayByID(relayID)
	if relayInfo == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Relay not found"})
		return
	}

	var req ForceUnregisterRequest
	_ = c.ShouldBindJSON(&req)

	// Force unregister
	affectedSessions := h.relayManager.ForceUnregister(relayID)

	// Log admin action
	h.logAction(c, admin.AuditActionDelete, admin.TargetType("relay"), 0,
		gin.H{"relay_id": relayID, "url": relayInfo.URL},
		gin.H{"affected_sessions": len(affectedSessions), "migrate": req.MigrateSessions})

	// Optionally migrate sessions
	if req.MigrateSessions && len(affectedSessions) > 0 && h.commandSender != nil && h.podService != nil {
		for _, session := range affectedSessions {
			pod, err := h.podService.GetPod(c.Request.Context(), session.PodKey)
			if err == nil && pod != nil && pod.RunnerID > 0 {
				_ = h.commandSender.SendUnsubscribeTerminal(c.Request.Context(), pod.RunnerID, session.PodKey)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":            "unregistered",
		"relay_id":          relayID,
		"affected_sessions": len(affectedSessions),
	})
}

// ListSessions returns all active sessions
func (h *RelayHandler) ListSessions(c *gin.Context) {
	if h.relayManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Relay manager not available"})
		return
	}

	relayID := c.Query("relay_id")

	var sessions []*relay.ActiveSession
	if relayID != "" {
		sessions = h.relayManager.GetSessionsByRelay(relayID)
	} else {
		sessions = h.relayManager.GetAllSessions()
	}

	c.JSON(http.StatusOK, gin.H{
		"data":  sessions,
		"total": len(sessions),
	})
}

// MigrateSessionRequest is the request for session migration
type MigrateSessionRequest struct {
	PodKey      string `json:"pod_key" binding:"required"`
	TargetRelay string `json:"target_relay" binding:"required"`
}

// MigrateSession migrates a single session to a new relay
func (h *RelayHandler) MigrateSession(c *gin.Context) {
	if h.relayManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Relay manager not available"})
		return
	}

	var req MigrateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get target relay
	targetRelay := h.relayManager.GetRelayByID(req.TargetRelay)
	if targetRelay == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Target relay not found"})
		return
	}

	if !targetRelay.Healthy {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Target relay is not healthy"})
		return
	}

	// Migrate the session
	newSession, oldRelayID, err := h.relayManager.MigrateSession(req.PodKey, targetRelay)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// Log admin action
	h.logAction(c, admin.AuditActionUpdate, admin.TargetType("relay_session"), 0,
		gin.H{"pod_key": req.PodKey, "relay": oldRelayID},
		gin.H{"pod_key": req.PodKey, "relay": req.TargetRelay})

	// Notify runner to reconnect
	if h.commandSender != nil && h.podService != nil {
		pod, err := h.podService.GetPod(c.Request.Context(), req.PodKey)
		if err == nil && pod != nil && pod.RunnerID > 0 {
			_ = h.commandSender.SendUnsubscribeTerminal(c.Request.Context(), pod.RunnerID, req.PodKey)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     "migrated",
		"session":    newSession,
		"from_relay": oldRelayID,
		"to_relay":   req.TargetRelay,
	})
}

// BulkMigrateRequest is the request for bulk session migration
type BulkMigrateRequest struct {
	SourceRelay string `json:"source_relay" binding:"required"`
	TargetRelay string `json:"target_relay" binding:"required"`
}

// BulkMigrateSessions migrates all sessions from one relay to another
func (h *RelayHandler) BulkMigrateSessions(c *gin.Context) {
	if h.relayManager == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Relay manager not available"})
		return
	}

	var req BulkMigrateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get target relay
	targetRelay := h.relayManager.GetRelayByID(req.TargetRelay)
	if targetRelay == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Target relay not found"})
		return
	}

	if !targetRelay.Healthy {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Target relay is not healthy"})
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

	// Log admin action
	h.logAction(c, admin.AuditActionUpdate, admin.TargetType("relay_bulk_migrate"), 0,
		gin.H{"source_relay": req.SourceRelay, "total": len(sessions)},
		gin.H{"target_relay": req.TargetRelay, "migrated": migrated, "failed": failed})

	c.JSON(http.StatusOK, gin.H{
		"status":   "completed",
		"total":    len(sessions),
		"migrated": migrated,
		"failed":   failed,
	})
}
