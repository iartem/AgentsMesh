package v1

import (
	"log"
	"net/http"
	"strconv"

	"github.com/anthropics/agentmesh/backend/internal/middleware"
	"github.com/anthropics/agentmesh/backend/internal/service/agent"
	"github.com/anthropics/agentmesh/backend/internal/service/gitprovider"
	"github.com/anthropics/agentmesh/backend/internal/service/repository"
	"github.com/anthropics/agentmesh/backend/internal/service/runner"
	"github.com/anthropics/agentmesh/backend/internal/service/session"
	"github.com/anthropics/agentmesh/backend/internal/service/sshkey"
	"github.com/anthropics/agentmesh/backend/internal/service/ticket"
	"github.com/gin-gonic/gin"
)

// SessionHandler handles session-related requests
type SessionHandler struct {
	sessionService     *session.Service
	runnerService      *runner.Service
	agentService       *agent.Service
	repositoryService  *repository.Service
	ticketService      *ticket.Service
	gitProviderService *gitprovider.Service
	sshKeyService      *sshkey.Service
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

// SetRepositoryService sets the repository service for repository lookups
func (h *SessionHandler) SetRepositoryService(rs *repository.Service) {
	h.repositoryService = rs
}

// SetTicketService sets the ticket service for ticket lookups
func (h *SessionHandler) SetTicketService(ts *ticket.Service) {
	h.ticketService = ts
}

// SetGitProviderService sets the git provider service for git token lookups
func (h *SessionHandler) SetGitProviderService(gps *gitprovider.Service) {
	h.gitProviderService = gps
}

// SetSSHKeyService sets the SSH key service for SSH private key lookups
func (h *SessionHandler) SetSSHKeyService(sks *sshkey.Service) {
	h.sshKeyService = sks
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

	// TeamID is deprecated - all resources are visible to organization members
	sessions, total, err := h.sessionService.ListSessions(
		c.Request.Context(),
		tenant.OrganizationID,
		req.TeamID, // Kept for backward compatibility, may be nil
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
	RepositoryURL     *string `json:"repository_url"`      // Direct repository URL (takes precedence over repository_id)
	TicketID          *int64  `json:"ticket_id"`
	TicketIdentifier  *string `json:"ticket_identifier"`   // Direct ticket identifier (takes precedence over ticket_id)
	InitialPrompt     string  `json:"initial_prompt"`
	BranchName        *string `json:"branch_name"`
	PermissionMode    *string `json:"permission_mode"`     // "plan", "default", or "bypassPermissions"

	// PluginConfig allows advanced users to pass additional configuration to sandbox plugins
	// Fields: init_script, init_timeout, env_vars
	PluginConfig map[string]interface{} `json:"plugin_config"`
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

	// TeamID is deprecated - all resources are visible to organization members

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
		// Get permission mode from request or session settings (default to "plan")
		permissionMode := "plan"
		if req.PermissionMode != nil {
			permissionMode = *req.PermissionMode
		} else if sess.PermissionMode != nil {
			permissionMode = *sess.PermissionMode
		}

		// Build PluginConfig for Runner's Sandbox plugins
		pluginConfig := h.buildPluginConfig(c, &req)

		createReq := &runner.CreateSessionRequest{
			SessionID:      sess.SessionKey,
			InitialCommand: "claude", // Default command to run Claude Code CLI
			InitialPrompt:  req.InitialPrompt,
			PermissionMode: permissionMode,
			PluginConfig:   pluginConfig,
		}

		// Log the request
		log.Printf("[sessions] Sending create_session to runner %d for session %s with plugin_config: %v",
			req.RunnerID, sess.SessionKey, pluginConfig)

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

	// All organization members can access sessions (Team-based access control removed)
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

	// All organization members can access sessions (Team-based access control removed)

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

	// All organization members can access sessions (Team-based access control removed)

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

// buildPluginConfig builds the PluginConfig map for Runner's Sandbox plugins
// It resolves repository_id -> repository_url, ticket_id -> ticket_identifier,
// and fetches git_token or ssh_private_key from the associated GitProvider
func (h *SessionHandler) buildPluginConfig(c *gin.Context, req *CreateSessionRequest) map[string]interface{} {
	config := make(map[string]interface{})

	// 1. Resolve Repository URL
	// Priority: repository_url > repository_id
	if req.RepositoryURL != nil && *req.RepositoryURL != "" {
		config["repository_url"] = *req.RepositoryURL
	} else if req.RepositoryID != nil && h.repositoryService != nil {
		repo, err := h.repositoryService.GetByID(c.Request.Context(), *req.RepositoryID)
		if err == nil && repo != nil {
			// Get clone URL from repository
			cloneURL, err := h.repositoryService.GetCloneURL(c.Request.Context(), *req.RepositoryID)
			if err == nil {
				config["repository_url"] = cloneURL
			}

			// Get credentials from GitProvider (if available)
			if h.gitProviderService != nil && repo.GitProviderID > 0 {
				provider, err := h.gitProviderService.GetByID(c.Request.Context(), repo.GitProviderID)
				if err == nil && provider != nil {
					// Check if this is an SSH Provider
					if provider.IsSSHProvider() {
						// Get SSH private key for authentication
						if provider.SSHKeyID != nil && h.sshKeyService != nil {
							privateKey, err := h.sshKeyService.GetPrivateKey(c.Request.Context(), *provider.SSHKeyID)
							if err == nil && privateKey != "" {
								config["ssh_private_key"] = privateKey
							}
						}
					} else {
						// HTTPS-based provider: use bot token
						if provider.BotTokenEncrypted != nil {
							// Note: Token is encrypted, Runner should handle decryption or
							// we need to decrypt here if encryption service is available
							// For now, we'll pass the encrypted token and let Runner handle it
							// TODO: Implement proper token decryption
							config["git_token"] = *provider.BotTokenEncrypted
						}
					}
				}
			}
		}
	}

	// 2. Resolve Branch Name
	if req.BranchName != nil && *req.BranchName != "" {
		config["branch"] = *req.BranchName
	} else if req.RepositoryID != nil && h.repositoryService != nil {
		// Use repository's default branch if not specified
		repo, err := h.repositoryService.GetByID(c.Request.Context(), *req.RepositoryID)
		if err == nil && repo != nil && repo.DefaultBranch != "" {
			config["branch"] = repo.DefaultBranch
		}
	}

	// 3. Resolve Ticket Identifier
	// Priority: ticket_identifier > ticket_id
	if req.TicketIdentifier != nil && *req.TicketIdentifier != "" {
		config["ticket_identifier"] = *req.TicketIdentifier
	} else if req.TicketID != nil && h.ticketService != nil {
		t, err := h.ticketService.GetTicket(c.Request.Context(), *req.TicketID)
		if err == nil && t != nil {
			config["ticket_identifier"] = t.Identifier
		}
	}

	// 4. Merge user-provided PluginConfig (can override above values)
	if req.PluginConfig != nil {
		for k, v := range req.PluginConfig {
			config[k] = v
		}
	}

	return config
}
