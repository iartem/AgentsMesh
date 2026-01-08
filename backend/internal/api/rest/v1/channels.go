package v1

import (
	"net/http"
	"strconv"

	"github.com/anthropics/agentmesh/backend/internal/middleware"
	"github.com/anthropics/agentmesh/backend/internal/service/channel"
	"github.com/gin-gonic/gin"
)

// ChannelHandler handles channel-related requests
type ChannelHandler struct {
	channelService *channel.Service
}

// NewChannelHandler creates a new channel handler
func NewChannelHandler(channelService *channel.Service) *ChannelHandler {
	return &ChannelHandler{
		channelService: channelService,
	}
}

// ListChannelsRequest represents channel list request
type ListChannelsRequest struct {
	TeamID       *int64 `form:"team_id"`
	RepositoryID *int64 `form:"repository_id"`
	TicketID     *int64 `form:"ticket_id"`
	IncludeArchived bool `form:"include_archived"`
}

// ListChannels lists channels
// GET /api/v1/organizations/:slug/channels
func (h *ChannelHandler) ListChannels(c *gin.Context) {
	var req ListChannelsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	limit := 50
	offset := 0

	channels, total, err := h.channelService.ListChannels(c.Request.Context(), tenant.OrganizationID, req.TeamID, req.IncludeArchived, limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list channels"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"channels": channels, "total": total})
}

// CreateChannelRequest represents channel creation request
type CreateChannelRequest struct {
	Name         string `json:"name" binding:"required,min=2,max=100"`
	Description  string `json:"description"`
	Document     string `json:"document"`
	TeamID       *int64 `json:"team_id"`
	RepositoryID *int64 `json:"repository_id"`
	TicketID     *int64 `json:"ticket_id"`
}

// CreateChannel creates a new channel
// POST /api/v1/organizations/:slug/channels
func (h *ChannelHandler) CreateChannel(c *gin.Context) {
	var req CreateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	var desc *string
	if req.Description != "" {
		desc = &req.Description
	}

	ch, err := h.channelService.CreateChannel(c.Request.Context(), &channel.CreateChannelRequest{
		OrganizationID:  tenant.OrganizationID,
		TeamID:          req.TeamID,
		Name:            req.Name,
		Description:     desc,
		RepositoryID:    req.RepositoryID,
		TicketID:        req.TicketID,
		CreatedByUserID: &tenant.UserID,
	})
	if err != nil {
		if err == channel.ErrDuplicateName {
			c.JSON(http.StatusConflict, gin.H{"error": "Channel name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create channel"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"channel": ch})
}

// GetChannel returns channel by ID
// GET /api/v1/organizations/:slug/channels/:id
func (h *ChannelHandler) GetChannel(c *gin.Context) {
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

	c.JSON(http.StatusOK, gin.H{"channel": ch})
}

// UpdateChannelRequest represents channel update request
type UpdateChannelRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Document    string `json:"document"`
}

// UpdateChannel updates a channel
// PUT /api/v1/organizations/:slug/channels/:id
func (h *ChannelHandler) UpdateChannel(c *gin.Context) {
	channelID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel ID"})
		return
	}

	var req UpdateChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	var name, description, document *string
	if req.Name != "" {
		name = &req.Name
	}
	if req.Description != "" {
		description = &req.Description
	}
	if req.Document != "" {
		document = &req.Document
	}

	ch, err = h.channelService.UpdateChannel(c.Request.Context(), channelID, name, description, document)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update channel"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"channel": ch})
}

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

// ListMessages lists channel messages
// GET /api/v1/organizations/:slug/channels/:id/messages
func (h *ChannelHandler) ListMessages(c *gin.Context) {
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

	limit := 50
	if l := c.Query("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}

	messages, err := h.channelService.GetMessages(c.Request.Context(), channelID, nil, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list messages"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"messages": messages})
}

// SendMessageRequest represents message send request
type SendMessageRequest struct {
	Content    string `json:"content" binding:"required"`
	SessionKey string `json:"session_key"`
}

// SendMessage sends a message to a channel
// POST /api/v1/organizations/:slug/channels/:id/messages
func (h *ChannelHandler) SendMessage(c *gin.Context) {
	channelID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel ID"})
		return
	}

	var req SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	if ch.IsArchived {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot send messages to archived channel"})
		return
	}

	var sessionKey *string
	if req.SessionKey != "" {
		sessionKey = &req.SessionKey
	}

	msg, err := h.channelService.SendMessage(c.Request.Context(), channelID, sessionKey, &tenant.UserID, "text", req.Content, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send message"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": msg})
}

// JoinSession joins a session to a channel
// POST /api/v1/organizations/:slug/channels/:id/sessions
func (h *ChannelHandler) JoinSession(c *gin.Context) {
	channelID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel ID"})
		return
	}

	var req struct {
		SessionKey string `json:"session_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	if err := h.channelService.JoinChannel(c.Request.Context(), channelID, req.SessionKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join channel"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session joined channel"})
}

// LeaveSession removes a session from a channel
// DELETE /api/v1/organizations/:slug/channels/:id/sessions/:session_key
func (h *ChannelHandler) LeaveSession(c *gin.Context) {
	channelID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel ID"})
		return
	}

	sessionKey := c.Param("session_key")

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

	if err := h.channelService.LeaveChannel(c.Request.Context(), channelID, sessionKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to leave channel"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session left channel"})
}

// ListChannelSessions returns sessions joined to a channel
// GET /api/v1/organizations/:slug/channels/:id/sessions
func (h *ChannelHandler) ListChannelSessions(c *gin.Context) {
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

	sessions, err := h.channelService.GetChannelSessions(c.Request.Context(), channelID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    len(sessions),
	})
}
