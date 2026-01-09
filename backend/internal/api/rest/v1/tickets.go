package v1

import (
	"net/http"
	"strconv"

	"github.com/anthropics/agentmesh/backend/internal/middleware"
	"github.com/anthropics/agentmesh/backend/internal/service/ticket"
	"github.com/gin-gonic/gin"
)

// TicketHandler handles ticket-related requests
type TicketHandler struct {
	ticketService *ticket.Service
}

// NewTicketHandler creates a new ticket handler
func NewTicketHandler(ticketService *ticket.Service) *TicketHandler {
	return &TicketHandler{
		ticketService: ticketService,
	}
}

// ListTicketsRequest represents ticket list request
type ListTicketsRequest struct {
	TeamID       *int64   `form:"team_id"`
	RepositoryID *int64   `form:"repository_id"`
	Status       string   `form:"status"`
	Type         string   `form:"type"`
	AssigneeID   *int64   `form:"assignee_id"`
	Labels       []string `form:"labels"`
	Limit        int      `form:"limit"`
	Offset       int      `form:"offset"`
}

// ListTickets lists tickets
// GET /api/v1/organizations/:slug/tickets
func (h *TicketHandler) ListTickets(c *gin.Context) {
	var req ListTicketsRequest
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
	tickets, total, err := h.ticketService.ListTickets(c.Request.Context(), &ticket.ListTicketsFilter{
		OrganizationID: tenant.OrganizationID,
		TeamID:         req.TeamID, // Kept for backward compatibility, may be nil
		RepositoryID:   req.RepositoryID,
		Status:         req.Status,
		Type:           req.Type,
		AssigneeID:     req.AssigneeID,
		UserRole:       tenant.UserRole,
		Limit:          limit,
		Offset:         req.Offset,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tickets"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"tickets": tickets,
		"total":   total,
		"limit":   limit,
		"offset":  req.Offset,
	})
}

// CreateTicketRequest represents ticket creation request
type CreateTicketRequest struct {
	RepositoryID   *int64   `json:"repository_id"`
	TeamID         *int64   `json:"team_id"`
	Type           string   `json:"type" binding:"required,oneof=task bug feature epic"`
	Title          string   `json:"title" binding:"required,min=1,max=500"`
	Description    string   `json:"description"`
	Content        string   `json:"content"`
	Status         string   `json:"status"`
	Priority       string   `json:"priority"`
	AssigneeIDs    []int64  `json:"assignee_ids"`
	Labels         []string `json:"labels"`
	ParentTicketID *int64   `json:"parent_ticket_id"`
	DueDate        *string  `json:"due_date"`
}

// CreateTicket creates a new ticket
// POST /api/v1/organizations/:slug/tickets
func (h *TicketHandler) CreateTicket(c *gin.Context) {
	var req CreateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	var description, content *string
	if req.Description != "" {
		description = &req.Description
	}
	if req.Content != "" {
		content = &req.Content
	}

	t, err := h.ticketService.CreateTicket(c.Request.Context(), &ticket.CreateTicketRequest{
		OrganizationID: tenant.OrganizationID,
		TeamID:         req.TeamID,
		RepositoryID:   req.RepositoryID,
		ReporterID:     tenant.UserID,
		Type:           req.Type,
		Title:          req.Title,
		Description:    description,
		Content:        content,
		Status:         req.Status,
		Priority:       req.Priority,
		AssigneeIDs:    req.AssigneeIDs,
		Labels:         req.Labels,
		ParentTicketID: req.ParentTicketID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create ticket"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"ticket": t})
}

// GetTicket returns ticket by identifier
// GET /api/v1/organizations/:slug/tickets/:identifier
func (h *TicketHandler) GetTicket(c *gin.Context) {
	identifier := c.Param("identifier")

	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ticket": t})
}

// UpdateTicketRequest represents ticket update request
type UpdateTicketRequest struct {
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Content     string   `json:"content"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Type        string   `json:"type"`
	AssigneeIDs []int64  `json:"assignee_ids"`
	Labels      []string `json:"labels"`
	DueDate     *string  `json:"due_date"`
}

// UpdateTicket updates a ticket
// PUT /api/v1/organizations/:slug/tickets/:identifier
func (h *TicketHandler) UpdateTicket(c *gin.Context) {
	identifier := c.Param("identifier")

	var req UpdateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	updates := make(map[string]interface{})
	if req.Title != "" {
		updates["title"] = req.Title
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Content != "" {
		updates["content"] = req.Content
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}
	if req.Priority != "" {
		updates["priority"] = req.Priority
	}
	if req.Type != "" {
		updates["type"] = req.Type
	}

	t, err = h.ticketService.UpdateTicket(c.Request.Context(), t.ID, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update ticket"})
		return
	}

	// Update assignees if provided
	if req.AssigneeIDs != nil {
		if err := h.ticketService.UpdateAssignees(c.Request.Context(), t.ID, req.AssigneeIDs); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update assignees"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"ticket": t})
}

// DeleteTicket deletes a ticket
// DELETE /api/v1/organizations/:slug/tickets/:identifier
func (h *TicketHandler) DeleteTicket(c *gin.Context) {
	identifier := c.Param("identifier")

	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.ticketService.DeleteTicket(c.Request.Context(), t.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete ticket"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Ticket deleted"})
}

// ListLabels lists labels
// GET /api/v1/organizations/:slug/labels
func (h *TicketHandler) ListLabels(c *gin.Context) {
	tenant := middleware.GetTenant(c)

	repositoryID := c.Query("repository_id")
	var repoID *int64
	if repositoryID != "" {
		id, err := strconv.ParseInt(repositoryID, 10, 64)
		if err == nil {
			repoID = &id
		}
	}

	labels, err := h.ticketService.ListLabels(c.Request.Context(), tenant.OrganizationID, repoID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list labels"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"labels": labels})
}

// CreateLabelRequest represents label creation request
type CreateLabelRequest struct {
	Name         string `json:"name" binding:"required,min=1,max=100"`
	Color        string `json:"color" binding:"required,hexcolor"`
	RepositoryID *int64 `json:"repository_id"`
}

// CreateLabel creates a new label
// POST /api/v1/organizations/:slug/labels
func (h *TicketHandler) CreateLabel(c *gin.Context) {
	var req CreateLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	label, err := h.ticketService.CreateLabel(c.Request.Context(), tenant.OrganizationID, req.RepositoryID, req.Name, req.Color)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create label"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"label": label})
}

// UpdateTicketStatusRequest represents status update request
type UpdateTicketStatusRequest struct {
	Status string `json:"status" binding:"required"`
}

// UpdateTicketStatus updates ticket status (convenience endpoint)
// PATCH /api/v1/organizations/:slug/tickets/:identifier/status
func (h *TicketHandler) UpdateTicketStatus(c *gin.Context) {
	identifier := c.Param("identifier")

	var req UpdateTicketStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.ticketService.UpdateStatus(c.Request.Context(), t.ID, req.Status); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Status updated"})
}

// UpdateLabelRequest represents label update request
type UpdateLabelRequest struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// UpdateLabel updates a label
// PUT /api/v1/organizations/:slug/labels/:id
func (h *TicketHandler) UpdateLabel(c *gin.Context) {
	labelID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid label ID"})
		return
	}

	var req UpdateLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Color != "" {
		updates["color"] = req.Color
	}

	label, err := h.ticketService.UpdateLabel(c.Request.Context(), tenant.OrganizationID, labelID, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update label"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"label": label})
}

// DeleteLabel deletes a label
// DELETE /api/v1/organizations/:slug/labels/:id
func (h *TicketHandler) DeleteLabel(c *gin.Context) {
	labelID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid label ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	if err := h.ticketService.DeleteLabel(c.Request.Context(), tenant.OrganizationID, labelID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete label"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Label deleted"})
}

// AddAssigneeRequest represents assignee addition request
type AddAssigneeRequest struct {
	UserID int64 `json:"user_id" binding:"required"`
}

// AddAssignee adds an assignee to a ticket
// POST /api/v1/organizations/:slug/tickets/:identifier/assignees
func (h *TicketHandler) AddAssignee(c *gin.Context) {
	identifier := c.Param("identifier")

	var req AddAssigneeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.ticketService.AddAssignee(c.Request.Context(), t.ID, req.UserID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add assignee"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Assignee added"})
}

// RemoveAssignee removes an assignee from a ticket
// DELETE /api/v1/organizations/:slug/tickets/:identifier/assignees/:user_id
func (h *TicketHandler) RemoveAssignee(c *gin.Context) {
	identifier := c.Param("identifier")
	userID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.ticketService.RemoveAssignee(c.Request.Context(), t.ID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove assignee"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Assignee removed"})
}

// AddLabelRequest represents label addition request
type AddLabelRequest struct {
	LabelID int64 `json:"label_id" binding:"required"`
}

// AddLabel adds a label to a ticket
// POST /api/v1/organizations/:slug/tickets/:identifier/labels
func (h *TicketHandler) AddLabel(c *gin.Context) {
	identifier := c.Param("identifier")

	var req AddLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.ticketService.AddLabel(c.Request.Context(), t.ID, req.LabelID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add label"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Label added"})
}

// RemoveLabel removes a label from a ticket
// DELETE /api/v1/organizations/:slug/tickets/:identifier/labels/:label_id
func (h *TicketHandler) RemoveLabel(c *gin.Context) {
	identifier := c.Param("identifier")
	labelID, err := strconv.ParseInt(c.Param("label_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid label ID"})
		return
	}

	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.ticketService.RemoveLabel(c.Request.Context(), t.ID, labelID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove label"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Label removed"})
}

// ListMergeRequests lists merge requests for a ticket
// GET /api/v1/organizations/:slug/tickets/:identifier/merge-requests
func (h *TicketHandler) ListMergeRequests(c *gin.Context) {
	identifier := c.Param("identifier")

	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	mergeRequests, err := h.ticketService.ListMergeRequests(c.Request.Context(), t.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list merge requests"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"merge_requests": mergeRequests})
}

// GetActiveTickets returns active (non-completed) tickets
// GET /api/v1/organizations/:slug/tickets/active
func (h *TicketHandler) GetActiveTickets(c *gin.Context) {
	tenant := middleware.GetTenant(c)

	limit := 20
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	var repoID *int64
	if repoIDStr := c.Query("repository_id"); repoIDStr != "" {
		if id, err := strconv.ParseInt(repoIDStr, 10, 64); err == nil {
			repoID = &id
		}
	}

	tickets, err := h.ticketService.GetActiveTickets(c.Request.Context(), tenant.OrganizationID, repoID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get active tickets"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tickets": tickets})
}

// GetBoard returns the kanban board view
// GET /api/v1/organizations/:slug/tickets/board
func (h *TicketHandler) GetBoard(c *gin.Context) {
	tenant := middleware.GetTenant(c)

	var repoID, teamID *int64
	if repoIDStr := c.Query("repository_id"); repoIDStr != "" {
		if id, err := strconv.ParseInt(repoIDStr, 10, 64); err == nil {
			repoID = &id
		}
	}
	if teamIDStr := c.Query("team_id"); teamIDStr != "" {
		if id, err := strconv.ParseInt(teamIDStr, 10, 64); err == nil {
			teamID = &id
		}
	}

	// TeamID is deprecated - all resources are visible to organization members
	board, err := h.ticketService.GetBoard(c.Request.Context(), &ticket.ListTicketsFilter{
		OrganizationID: tenant.OrganizationID,
		RepositoryID:   repoID,
		TeamID:         teamID, // Kept for backward compatibility, may be nil
		UserRole:       tenant.UserRole,
		Limit:          50,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get board"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"board": board})
}

// GetSubTickets returns sub-tickets of a parent ticket
// GET /api/v1/organizations/:slug/tickets/:identifier/sub-tickets
func (h *TicketHandler) GetSubTickets(c *gin.Context) {
	identifier := c.Param("identifier")

	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	subTickets, err := h.ticketService.GetChildTickets(c.Request.Context(), t.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get sub-tickets"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"sub_tickets": subTickets})
}

// ========== Relations ==========

// ListRelations lists relations for a ticket
// GET /api/v1/organizations/:slug/tickets/:identifier/relations
func (h *TicketHandler) ListRelations(c *gin.Context) {
	identifier := c.Param("identifier")

	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	relations, err := h.ticketService.ListRelations(c.Request.Context(), t.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list relations"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"relations": relations})
}

// CreateRelationRequest represents relation creation request
type CreateRelationRequest struct {
	TargetIdentifier string `json:"target_identifier" binding:"required"`
	RelationType     string `json:"relation_type" binding:"required,oneof=blocks blocked_by relates_to duplicates"`
}

// CreateRelation creates a relation between tickets
// POST /api/v1/organizations/:slug/tickets/:identifier/relations
func (h *TicketHandler) CreateRelation(c *gin.Context) {
	identifier := c.Param("identifier")

	var req CreateRelationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	// Get source ticket
	sourceTicket, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Source ticket not found"})
		return
	}

	if sourceTicket.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get target ticket
	targetTicket, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), req.TargetIdentifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Target ticket not found"})
		return
	}

	relation, err := h.ticketService.CreateRelation(
		c.Request.Context(),
		tenant.OrganizationID,
		sourceTicket.ID,
		targetTicket.ID,
		req.RelationType,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create relation: " + err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"relation": relation})
}

// DeleteRelation deletes a relation
// DELETE /api/v1/organizations/:slug/tickets/:identifier/relations/:relation_id
func (h *TicketHandler) DeleteRelation(c *gin.Context) {
	identifier := c.Param("identifier")
	relationID, err := strconv.ParseInt(c.Param("relation_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid relation ID"})
		return
	}

	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.ticketService.DeleteRelation(c.Request.Context(), relationID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete relation"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Relation deleted"})
}

// ========== Commits ==========

// ListCommits lists commits for a ticket
// GET /api/v1/organizations/:slug/tickets/:identifier/commits
func (h *TicketHandler) ListCommits(c *gin.Context) {
	identifier := c.Param("identifier")

	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	commits, err := h.ticketService.ListCommits(c.Request.Context(), t.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list commits"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"commits": commits})
}

// LinkCommitRequest represents commit link request
type LinkCommitRequest struct {
	CommitSHA     string `json:"commit_sha" binding:"required"`
	CommitMessage string `json:"commit_message"`
	CommitURL     string `json:"commit_url"`
	AuthorName    string `json:"author_name"`
	AuthorEmail   string `json:"author_email"`
}

// LinkCommit links a commit to a ticket
// POST /api/v1/organizations/:slug/tickets/:identifier/commits
func (h *TicketHandler) LinkCommit(c *gin.Context) {
	identifier := c.Param("identifier")

	var req LinkCommitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if t.RepositoryID == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Ticket has no repository"})
		return
	}

	var commitURL, authorName, authorEmail *string
	if req.CommitURL != "" {
		commitURL = &req.CommitURL
	}
	if req.AuthorName != "" {
		authorName = &req.AuthorName
	}
	if req.AuthorEmail != "" {
		authorEmail = &req.AuthorEmail
	}

	commit, err := h.ticketService.LinkCommit(
		c.Request.Context(),
		tenant.OrganizationID,
		t.ID,
		*t.RepositoryID,
		nil, // sessionID
		req.CommitSHA,
		req.CommitMessage,
		commitURL,
		authorName,
		authorEmail,
		nil, // committedAt
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to link commit"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"commit": commit})
}

// UnlinkCommit unlinks a commit from a ticket
// DELETE /api/v1/organizations/:slug/tickets/:identifier/commits/:commit_id
func (h *TicketHandler) UnlinkCommit(c *gin.Context) {
	identifier := c.Param("identifier")
	commitID, err := strconv.ParseInt(c.Param("commit_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid commit ID"})
		return
	}

	t, err := h.ticketService.GetTicketByIdentifier(c.Request.Context(), identifier)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Ticket not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.ticketService.UnlinkCommit(c.Request.Context(), commitID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to unlink commit"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Commit unlinked"})
}
