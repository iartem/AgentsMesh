package internal

import (
	"net/http"

	"github.com/anthropics/agentsmesh/backend/internal/service/relay"
	"github.com/gin-gonic/gin"
)

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
