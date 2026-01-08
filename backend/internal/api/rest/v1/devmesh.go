package v1

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/anthropics/agentmesh/backend/internal/domain/devmesh"
	"github.com/anthropics/agentmesh/backend/internal/middleware"
	devmeshService "github.com/anthropics/agentmesh/backend/internal/service/devmesh"
	"github.com/anthropics/agentmesh/backend/internal/service/ticket"
	"github.com/gin-gonic/gin"
)

// DevMeshHandler handles DevMesh-related requests
type DevMeshHandler struct {
	devmeshService *devmeshService.Service
	ticketService  *ticket.Service
}

// NewDevMeshHandler creates a new DevMesh handler
func NewDevMeshHandler(ds *devmeshService.Service, ts *ticket.Service) *DevMeshHandler {
	return &DevMeshHandler{
		devmeshService: ds,
		ticketService:  ts,
	}
}

// GetTopology returns the DevMesh topology for the organization
// GET /api/v1/organizations/:slug/devmesh/topology
func (h *DevMeshHandler) GetTopology(c *gin.Context) {
	tenant := middleware.GetTenant(c)

	var teamID *int64
	if teamIDStr := c.Query("team_id"); teamIDStr != "" {
		if id, err := strconv.ParseInt(teamIDStr, 10, 64); err == nil {
			teamID = &id
		}
	}

	slog.Debug("GetTopology called", "org_id", tenant.OrganizationID, "team_id", teamID)

	topology, err := h.devmeshService.GetTopology(c.Request.Context(), tenant.OrganizationID, teamID)
	if err != nil {
		slog.Error("Failed to get topology", "error", err, "org_id", tenant.OrganizationID)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get topology: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"topology": topology})
}

// CreateSessionForTicketRequest represents the request to create a session for a ticket
type CreateSessionForTicketRequest struct {
	RunnerID       int64  `json:"runner_id" binding:"required"`
	InitialPrompt  string `json:"initial_prompt"`
	Model          string `json:"model"`
	PermissionMode string `json:"permission_mode"`
	ThinkLevel     string `json:"think_level"`
}

// CreateSessionForTicket creates a new session for a ticket
// POST /api/v1/organizations/:slug/tickets/:identifier/sessions
func (h *DevMeshHandler) CreateSessionForTicket(c *gin.Context) {
	identifier := c.Param("identifier")
	tenant := middleware.GetTenant(c)

	var req CreateSessionForTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the ticket
	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	// Create session
	session, err := h.devmeshService.CreateSessionForTicket(c.Request.Context(), &devmesh.CreateSessionForTicketRequest{
		OrganizationID: tenant.OrganizationID,
		TeamID:         t.TeamID,
		TicketID:       t.ID,
		RunnerID:       req.RunnerID,
		CreatedByID:    tenant.UserID,
		InitialPrompt:  req.InitialPrompt,
		Model:          req.Model,
		PermissionMode: req.PermissionMode,
		ThinkLevel:     req.ThinkLevel,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "Session created successfully",
		"session": session,
	})
}

// GetTicketSessions returns sessions for a ticket
// GET /api/v1/organizations/:slug/tickets/:identifier/sessions
func (h *DevMeshHandler) GetTicketSessions(c *gin.Context) {
	identifier := c.Param("identifier")

	// Get the ticket
	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	// Get sessions
	activeOnly := c.Query("active") == "true"
	var sessions []devmesh.DevMeshNode
	if activeOnly {
		sessions, err = h.devmeshService.GetActiveSessionsForTicket(c.Request.Context(), t.ID)
	} else {
		sessions, err = h.devmeshService.GetSessionsForTicket(c.Request.Context(), t.ID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// BatchGetTicketSessionsRequest represents the batch request
type BatchGetTicketSessionsRequest struct {
	TicketIDs []int64 `json:"ticket_ids" binding:"required"`
}

// BatchGetTicketSessions returns sessions for multiple tickets
// POST /api/v1/organizations/:slug/tickets/batch-sessions
func (h *DevMeshHandler) BatchGetTicketSessions(c *gin.Context) {
	var req BatchGetTicketSessionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(req.TicketIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "ticket_ids cannot be empty"})
		return
	}

	if len(req.TicketIDs) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot query more than 100 tickets at once"})
		return
	}

	result, err := h.devmeshService.BatchGetTicketSessions(c.Request.Context(), req.TicketIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get sessions"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// JoinChannelRequest represents the request to join a channel
type JoinChannelRequest struct {
	SessionKey string `json:"session_key" binding:"required"`
}

// JoinChannel adds a session to a channel
// POST /api/v1/organizations/:slug/channels/:id/sessions
func (h *DevMeshHandler) JoinChannel(c *gin.Context) {
	channelIDStr := c.Param("id")
	channelID, err := strconv.ParseInt(channelIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel ID"})
		return
	}

	var req JoinChannelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.devmeshService.JoinChannel(c.Request.Context(), channelID, req.SessionKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to join channel"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session joined channel successfully"})
}

// LeaveChannel removes a session from a channel
// DELETE /api/v1/organizations/:slug/channels/:id/sessions/:session_key
func (h *DevMeshHandler) LeaveChannel(c *gin.Context) {
	channelIDStr := c.Param("id")
	channelID, err := strconv.ParseInt(channelIDStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid channel ID"})
		return
	}

	sessionKey := c.Param("session_key")

	if err := h.devmeshService.LeaveChannel(c.Request.Context(), channelID, sessionKey); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to leave channel"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session left channel successfully"})
}
