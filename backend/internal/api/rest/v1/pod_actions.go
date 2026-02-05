package v1

import (
	"net/http"

	"github.com/anthropics/agentsmesh/backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

// TerminatePod terminates a pod
// POST /api/v1/organizations/:slug/pods/:key/terminate
func (h *PodHandler) TerminatePod(c *gin.Context) {
	podKey := c.Param("key")

	pod, err := h.podService.GetPod(c.Request.Context(), podKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if pod.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Only creator or admin can terminate
	if pod.CreatedByID != tenant.UserID && tenant.UserRole == "member" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only pod creator or admin can terminate"})
		return
	}

	if err := h.podService.TerminatePod(c.Request.Context(), podKey); err != nil {
		if err == ErrPodTerminated {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Pod already terminated"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to terminate pod"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Pod terminated"})
}

// SendPromptRequest represents prompt sending request
type SendPromptRequest struct {
	Prompt string `json:"prompt" binding:"required"`
}

// SendPrompt sends a prompt to the pod
// POST /api/v1/organizations/:slug/pods/:key/send-prompt
func (h *PodHandler) SendPrompt(c *gin.Context) {
	podKey := c.Param("key")

	var req SendPromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	pod, err := h.podService.GetPod(c.Request.Context(), podKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Pod not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if pod.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if !pod.IsActive() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Pod is not active"})
		return
	}

	// TODO: Implement prompt sending via gRPC to runner
	// For now, return not implemented
	_ = req.Prompt
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Prompt sending via REST not implemented. Use terminal WebSocket."})
}
