package internal

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

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

	response := HeartbeatResponse{Status: "ok"}

	// Include TLS certificate only if relay needs it and ACME is enabled
	if req.NeedCert && h.acmeManager != nil {
		cert, key, expiry, err := h.acmeManager.GetCertificatePEM()
		if err == nil && cert != "" {
			response.TLSCert = cert
			response.TLSKey = key
			response.TLSExpiry = expiry.Format(time.RFC3339)
			h.logger.Info("TLS certificate included in heartbeat response",
				"relay_id", req.RelayID,
				"cert_expiry", expiry)
		}
	}

	c.JSON(http.StatusOK, response)
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
