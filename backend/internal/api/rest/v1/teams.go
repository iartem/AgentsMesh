package v1

import (
	"net/http"
	"strconv"

	"github.com/anthropics/agentmesh/backend/internal/middleware"
	"github.com/anthropics/agentmesh/backend/internal/service/organization"
	"github.com/anthropics/agentmesh/backend/internal/service/team"
	"github.com/gin-gonic/gin"
)

// TeamHandler handles team-related requests
type TeamHandler struct {
	teamService *team.Service
	orgService  *organization.Service
}

// NewTeamHandler creates a new team handler
func NewTeamHandler(teamService *team.Service, orgService *organization.Service) *TeamHandler {
	return &TeamHandler{
		teamService: teamService,
		orgService:  orgService,
	}
}

// CreateTeamRequest represents team creation request
type CreateTeamRequest struct {
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Description string `json:"description"`
}

// ListTeams lists teams in organization
// GET /api/v1/organizations/:slug/teams
func (h *TeamHandler) ListTeams(c *gin.Context) {
	tenant := middleware.GetTenant(c)

	teams, err := h.teamService.ListByOrganization(c.Request.Context(), tenant.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list teams"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"teams": teams})
}

// CreateTeam creates a new team
// POST /api/v1/organizations/:slug/teams
func (h *TeamHandler) CreateTeam(c *gin.Context) {
	var req CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	t, err := h.teamService.Create(c.Request.Context(), tenant.UserID, &team.CreateRequest{
		OrganizationID: tenant.OrganizationID,
		Name:           req.Name,
		Description:    req.Description,
	})

	if err != nil {
		if err == team.ErrTeamNameExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Team name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create team"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"team": t})
}

// GetTeam returns team by ID
// GET /api/v1/organizations/:slug/teams/:id
func (h *TeamHandler) GetTeam(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	t, err := h.teamService.GetByID(c.Request.Context(), teamID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"team": t})
}

// UpdateTeamRequest represents team update request
type UpdateTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// UpdateTeam updates a team
// PUT /api/v1/organizations/:slug/teams/:id
func (h *TeamHandler) UpdateTeam(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	var req UpdateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	t, err := h.teamService.GetByID(c.Request.Context(), teamID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return
	}

	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check if user is team lead or org admin
	isLead, _ := h.teamService.IsLead(c.Request.Context(), teamID, tenant.UserID)
	if !isLead && tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Team lead or admin permission required"})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}

	t, err = h.teamService.Update(c.Request.Context(), teamID, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update team"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"team": t})
}

// DeleteTeam deletes a team
// DELETE /api/v1/organizations/:slug/teams/:id
func (h *TeamHandler) DeleteTeam(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	t, err := h.teamService.GetByID(c.Request.Context(), teamID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return
	}

	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Only org admin can delete team
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	if err := h.teamService.Delete(c.Request.Context(), teamID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete team"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Team deleted"})
}

// ListTeamMembers lists team members
// GET /api/v1/organizations/:slug/teams/:id/members
func (h *TeamHandler) ListTeamMembers(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	t, err := h.teamService.GetByID(c.Request.Context(), teamID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return
	}

	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	members, err := h.teamService.ListMembers(c.Request.Context(), teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list members"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"members": members})
}

// AddTeamMemberRequest represents adding member to team
type AddTeamMemberRequest struct {
	UserID int64  `json:"user_id" binding:"required"`
	Role   string `json:"role" binding:"omitempty,oneof=lead member"`
}

// AddTeamMember adds a member to team
// POST /api/v1/organizations/:slug/teams/:id/members
func (h *TeamHandler) AddTeamMember(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	var req AddTeamMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	t, err := h.teamService.GetByID(c.Request.Context(), teamID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return
	}

	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check permission
	isLead, _ := h.teamService.IsLead(c.Request.Context(), teamID, tenant.UserID)
	if !isLead && tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Team lead or admin permission required"})
		return
	}

	role := req.Role
	if role == "" {
		role = "member"
	}

	if err := h.teamService.AddMember(c.Request.Context(), teamID, req.UserID, role); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add member"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "Member added"})
}

// RemoveTeamMember removes a member from team
// DELETE /api/v1/organizations/:slug/teams/:id/members/:user_id
func (h *TeamHandler) RemoveTeamMember(c *gin.Context) {
	teamID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid team ID"})
		return
	}

	targetUserID, err := strconv.ParseInt(c.Param("user_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	t, err := h.teamService.GetByID(c.Request.Context(), teamID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return
	}

	if t.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check permission
	isLead, _ := h.teamService.IsLead(c.Request.Context(), teamID, tenant.UserID)
	if !isLead && tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Team lead or admin permission required"})
		return
	}

	if err := h.teamService.RemoveMember(c.Request.Context(), teamID, targetUserID); err != nil {
		if err == team.ErrCannotRemoveLead {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Cannot remove the only team lead"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove member"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member removed"})
}
