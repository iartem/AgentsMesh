package v1

import (
	"net/http"
	"strconv"

	"github.com/anthropics/agentsmesh/backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

// ArchiveChannel archives a channel
// POST /api/v1/organizations/:slug/channels/:id/archive
func (h *ChannelHandler) ArchiveChannel(c *gin.Context) {
	channelID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel ID"})
		return
	}

	ch, err := h.channelService.GetChannel(c.Request.Context(), channelID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if ch.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.channelService.ArchiveChannel(c.Request.Context(), channelID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to archive channel"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Channel archived"})
}

// UnarchiveChannel unarchives a channel
// POST /api/v1/organizations/:slug/channels/:id/unarchive
func (h *ChannelHandler) UnarchiveChannel(c *gin.Context) {
	channelID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel ID"})
		return
	}

	ch, err := h.channelService.GetChannel(c.Request.Context(), channelID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Channel not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if ch.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.channelService.UnarchiveChannel(c.Request.Context(), channelID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unarchive channel"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Channel unarchived"})
}
