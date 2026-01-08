package v1

import (
	"log"
	"net/http"
	"strconv"

	"github.com/anthropics/agentmesh/backend/internal/middleware"
	"github.com/anthropics/agentmesh/backend/internal/service/agent"
	"github.com/anthropics/agentmesh/backend/internal/service/runner"
	"github.com/anthropics/agentmesh/backend/internal/service/session"
	"github.com/gin-gonic/gin"
)

// SessionHandler handles session-related requests
type SessionHandler struct {
	sessionService     *session.Service
	runnerService      *runner.Service
	agentService       *agent.Service
	runnerConnMgr      *runner.ConnectionManager
	sessionCoordinator *runner.SessionCoordinator
	terminalRouter     interface{} // *runner.TerminalRouter, optional
}

// NewSessionHandler creates a new session handler
func NewSessionHandler(sessionService *session.Service, runnerService *runner.Service, agentService *agent.Service) *SessionHandler {
	return &SessionHandler{
		sessionService: sessionService,
		runnerService:  runnerService,
		agentService:   agentService,
	}
}

// SetRunnerConnectionManager sets the runner connection manager
func (h *SessionHandler) SetRunnerConnectionManager(cm *runner.ConnectionManager) {
	h.runnerConnMgr = cm
}

// SetSessionCoordinator sets the session coordinator for session lifecycle management
func (h *SessionHandler) SetSessionCoordinator(sc *runner.SessionCoordinator) {
	h.sessionCoordinator = sc
}

// SetTerminalRouter sets the terminal router for terminal operations
func (h *SessionHandler) SetTerminalRouter(tr interface{}) {
	h.terminalRouter = tr
}

// ListSessionsRequest represents session list request
type ListSessionsRequest struct {
	TeamID *int64 `form:"team_id"`
	Status string `form:"status"`
	Limit  int    `form:"limit"`
	Offset int    `form:"offset"`
}

// ListSessions lists sessions
// GET /api/v1/organizations/:slug/sessions
func (h *SessionHandler) ListSessions(c *gin.Context) {
	var req ListSessionsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	limit := req.Limit
	if limit == 0 {
		limit = 20
	}

	// If not admin, filter by user's teams
	var teamID *int64
	if tenant.UserRole == "member" {
		// Use first team ID or req.TeamID if specified
		if req.TeamID != nil {
			teamID = req.TeamID
		} else if len(tenant.TeamIDs) > 0 {
			teamID = &tenant.TeamIDs[0]
		}
	} else {
		teamID = req.TeamID
	}

	sessions, total, err := h.sessionService.ListSessions(
		c.Request.Context(),
		tenant.OrganizationID,
		teamID,
		req.Status,
		limit,
		req.Offset,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"total":    total,
		"limit":    limit,
		"offset":   req.Offset,
	})
}

// CreateSessionRequest represents session creation request
type CreateSessionRequest struct {
	RunnerID          int64   `json:"runner_id" binding:"required"`
	AgentTypeID       *int64  `json:"agent_type_id"`
	CustomAgentTypeID *int64  `json:"custom_agent_type_id"`
	TeamID            *int64  `json:"team_id"`
	RepositoryID      *int64  `json:"repository_id"`
	TicketID          *int64  `json:"ticket_id"`
	InitialPrompt     string  `json:"initial_prompt"`
	BranchName        *string `json:"branch_name"`
}

// CreateSession creates a new session
// POST /api/v1/organizations/:slug/sessions
func (h *SessionHandler) CreateSession(c *gin.Context) {
	var req CreateSessionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	// Check team membership if team is specified
	if req.TeamID != nil {
		found := false
		for _, tid := range tenant.TeamIDs {
			if tid == *req.TeamID {
				found = true
				break
			}
		}
		if !found && tenant.UserRole == "member" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this team"})
			return
		}
	}

	// Create session record in database
	sess, err := h.sessionService.CreateSession(c.Request.Context(), &session.CreateSessionRequest{
		OrganizationID:    tenant.OrganizationID,
		TeamID:            req.TeamID,
		RunnerID:          req.RunnerID,
		AgentTypeID:       req.AgentTypeID,
		CustomAgentTypeID: req.CustomAgentTypeID,
		RepositoryID:      req.RepositoryID,
		TicketID:          req.TicketID,
		CreatedByID:       tenant.UserID,
		InitialPrompt:     req.InitialPrompt,
		BranchName:        req.BranchName,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	// Send create_session command to runner via SessionCoordinator
	if h.sessionCoordinator != nil {
		// Get permission mode from session settings (default to "plan" for plan mode)
		permissionMode := "plan"
		if sess.PermissionMode != nil {
			permissionMode = *sess.PermissionMode
		}

		createReq := &runner.CreateSessionRequest{
			SessionID:      sess.SessionKey,
			InitialCommand: "claude", // Default command to run Claude Code CLI
			InitialPrompt:  req.InitialPrompt,
			PermissionMode: permissionMode,
			WorkingDir:     "", // Runner will use default workspace
		}

		// Set ticket identifier if ticket is specified (for worktree creation)
		if req.TicketID != nil {
			// TODO: Fetch ticket identifier from database
			createReq.TicketIdentifier = ""
		}

		// Get repository path if specified
		if req.RepositoryID != nil {
			// TODO: Fetch repository path from database and set as WorkingDir
		}

		// Log the request
		log.Printf("[sessions] Sending create_session to runner %d for session %s", req.RunnerID, sess.SessionKey)

		if err := h.sessionCoordinator.CreateSession(c.Request.Context(), req.RunnerID, createReq); err != nil {
			// Log the error but don't fail - session is created, runner might be offline
			log.Printf("[sessions] Failed to send create_session: %v", err)
			c.JSON(http.StatusCreated, gin.H{
				"session": sess,
				"warning": "Session created but runner communication failed: " + err.Error(),
			})
			return
		}
		log.Printf("[sessions] create_session sent successfully for session %s", sess.SessionKey)
	} else {
		log.Printf("[sessions] SessionCoordinator is nil, cannot send create_session command")
	}

	c.JSON(http.StatusCreated, gin.H{"session": sess})
}

// GetSession returns session by key
// GET /api/v1/organizations/:slug/sessions/:key
func (h *SessionHandler) GetSession(c *gin.Context) {
	sessionKey := c.Param("key")

	sess, err := h.sessionService.GetSession(c.Request.Context(), sessionKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if sess.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check team access if member
	if tenant.UserRole == "member" && sess.TeamID != nil {
		found := false
		for _, tid := range tenant.TeamIDs {
			if tid == *sess.TeamID {
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this session"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"session": sess})
}

// TerminateSession terminates a session
// POST /api/v1/organizations/:slug/sessions/:key/terminate
func (h *SessionHandler) TerminateSession(c *gin.Context) {
	sessionKey := c.Param("key")

	sess, err := h.sessionService.GetSession(c.Request.Context(), sessionKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if sess.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Only creator or admin can terminate
	if sess.CreatedByID != tenant.UserID && tenant.UserRole == "member" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only session creator or admin can terminate"})
		return
	}

	if err := h.sessionService.TerminateSession(c.Request.Context(), sessionKey); err != nil {
		if err == session.ErrSessionTerminated {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Session already terminated"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to terminate session"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Session terminated"})
}

// GetSessionConnection returns connection info for session
// GET /api/v1/organizations/:slug/sessions/:key/connect
func (h *SessionHandler) GetSessionConnection(c *gin.Context) {
	sessionKey := c.Param("key")

	sess, err := h.sessionService.GetSession(c.Request.Context(), sessionKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if sess.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if !sess.IsActive() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session is not active"})
		return
	}

	// Return WebSocket connection URL
	c.JSON(http.StatusOK, gin.H{
		"session_key": sessionKey,
		"ws_url":      "/api/v1/ws/terminal/" + sessionKey,
		"status":      sess.Status,
	})
}

// ListSessionsByTicket lists sessions for a ticket
// GET /api/v1/organizations/:slug/tickets/:id/sessions
func (h *SessionHandler) ListSessionsByTicket(c *gin.Context) {
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid ticket ID"})
		return
	}

	sessions, err := h.sessionService.GetSessionsByTicket(c.Request.Context(), ticketID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list sessions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

// GetConnectionInfo returns connection info for session (alias for GetSessionConnection)
// GET /api/v1/organizations/:slug/sessions/:key/connect
func (h *SessionHandler) GetConnectionInfo(c *gin.Context) {
	h.GetSessionConnection(c)
}

// SendPromptRequest represents prompt sending request
type SendPromptRequest struct {
	Prompt string `json:"prompt" binding:"required"`
}

// SendPrompt sends a prompt to the session
// POST /api/v1/organizations/:slug/sessions/:key/send-prompt
func (h *SessionHandler) SendPrompt(c *gin.Context) {
	sessionKey := c.Param("key")

	var req SendPromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sess, err := h.sessionService.GetSession(c.Request.Context(), sessionKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if sess.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if !sess.IsActive() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session is not active"})
		return
	}

	// TODO: Implement WebSocket-based prompt sending to runner
	// For now, return not implemented
	_ = req.Prompt
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Prompt sending via REST not implemented. Use WebSocket terminal."})
}

// TerminalRouterInterface defines the interface for terminal router operations
type TerminalRouterInterface interface {
	GetRecentOutput(sessionID string, lines int) []byte
	GetAllScrollbackData(sessionID string) []byte
	RouteInput(sessionID string, data []byte) error
	RouteResize(sessionID string, cols, rows int) error
}

// ObserveTerminalRequest represents terminal observation request
type ObserveTerminalRequest struct {
	Lines int `form:"lines"`
}

// ObserveTerminal returns recent terminal output for observation
// GET /api/v1/organizations/:slug/sessions/:key/terminal/observe
func (h *SessionHandler) ObserveTerminal(c *gin.Context) {
	sessionKey := c.Param("key")

	var req ObserveTerminalRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sess, err := h.sessionService.GetSession(c.Request.Context(), sessionKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if sess.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check team access if member
	if tenant.UserRole == "member" && sess.TeamID != nil {
		found := false
		for _, tid := range tenant.TeamIDs {
			if tid == *sess.TeamID {
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this session"})
			return
		}
	}

	// Get terminal output from router if available
	if h.terminalRouter == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Terminal router not available"})
		return
	}

	tr, ok := h.terminalRouter.(TerminalRouterInterface)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Terminal router interface not implemented"})
		return
	}

	lines := req.Lines
	if lines <= 0 {
		lines = 100 // Default to last 100 lines
	}

	var output []byte
	if lines == -1 {
		output = tr.GetAllScrollbackData(sessionKey)
	} else {
		output = tr.GetRecentOutput(sessionKey, lines)
	}

	c.JSON(http.StatusOK, gin.H{
		"session_key": sessionKey,
		"output":      string(output),
		"status":      sess.Status,
		"agent_status": sess.AgentStatus,
	})
}

// TerminalInputRequest represents terminal input request
type TerminalInputRequest struct {
	Input string `json:"input" binding:"required"`
}

// SendTerminalInput sends input to the terminal
// POST /api/v1/organizations/:slug/sessions/:key/terminal/input
func (h *SessionHandler) SendTerminalInput(c *gin.Context) {
	sessionKey := c.Param("key")

	var req TerminalInputRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sess, err := h.sessionService.GetSession(c.Request.Context(), sessionKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if sess.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if !sess.IsActive() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session is not active"})
		return
	}

	// Check team access if member
	if tenant.UserRole == "member" && sess.TeamID != nil {
		found := false
		for _, tid := range tenant.TeamIDs {
			if tid == *sess.TeamID {
				found = true
				break
			}
		}
		if !found {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access denied to this session"})
			return
		}
	}

	if h.terminalRouter == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Terminal router not available"})
		return
	}

	tr, ok := h.terminalRouter.(TerminalRouterInterface)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Terminal router interface not implemented"})
		return
	}

	if err := tr.RouteInput(sessionKey, []byte(req.Input)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send input: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Input sent"})
}

// TerminalResizeRequest represents terminal resize request
type TerminalResizeRequest struct {
	Cols int `json:"cols" binding:"required,min=1"`
	Rows int `json:"rows" binding:"required,min=1"`
}

// ResizeTerminal resizes the terminal
// POST /api/v1/organizations/:slug/sessions/:key/terminal/resize
func (h *SessionHandler) ResizeTerminal(c *gin.Context) {
	sessionKey := c.Param("key")

	var req TerminalResizeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	sess, err := h.sessionService.GetSession(c.Request.Context(), sessionKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Session not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if sess.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if !sess.IsActive() {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Session is not active"})
		return
	}

	if h.terminalRouter == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Terminal router not available"})
		return
	}

	tr, ok := h.terminalRouter.(TerminalRouterInterface)
	if !ok {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Terminal router interface not implemented"})
		return
	}

	if err := tr.RouteResize(sessionKey, req.Cols, req.Rows); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to resize terminal: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Terminal resized"})
}
