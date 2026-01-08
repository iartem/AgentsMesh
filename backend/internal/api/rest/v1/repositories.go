package v1

import (
	"net/http"
	"strconv"

	"github.com/anthropics/agentmesh/backend/internal/middleware"
	"github.com/anthropics/agentmesh/backend/internal/service/repository"
	"github.com/gin-gonic/gin"
)

// RepositoryHandler handles repository-related requests
type RepositoryHandler struct {
	repositoryService *repository.Service
}

// NewRepositoryHandler creates a new repository handler
func NewRepositoryHandler(repositoryService *repository.Service) *RepositoryHandler {
	return &RepositoryHandler{
		repositoryService: repositoryService,
	}
}

// ListRepositories lists configured repositories
// GET /api/v1/organizations/:slug/repositories
func (h *RepositoryHandler) ListRepositories(c *gin.Context) {
	tenant := middleware.GetTenant(c)

	var teamID *int64
	if tid := c.Query("team_id"); tid != "" {
		if id, err := strconv.ParseInt(tid, 10, 64); err == nil {
			teamID = &id
		}
	}

	repos, err := h.repositoryService.ListByOrganization(c.Request.Context(), tenant.OrganizationID, teamID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list repositories"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"repositories": repos})
}

// CreateRepositoryRequest represents repository creation request
type CreateRepositoryRequest struct {
	GitProviderID int64  `json:"git_provider_id" binding:"required"`
	ExternalID    string `json:"external_id" binding:"required"`
	Name          string `json:"name" binding:"required"`
	FullPath      string `json:"full_path" binding:"required"`
	DefaultBranch string `json:"default_branch"`
	TeamID        *int64 `json:"team_id"`
	TicketPrefix  string `json:"ticket_prefix"`
	AccessToken   string `json:"access_token"` // For git provider operations
}

// CreateRepository creates a new repository configuration
// POST /api/v1/organizations/:slug/repositories
func (h *RepositoryHandler) CreateRepository(c *gin.Context) {
	var req CreateRepositoryRequest
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

	defaultBranch := req.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	var ticketPrefix *string
	if req.TicketPrefix != "" {
		ticketPrefix = &req.TicketPrefix
	}

	repo, err := h.repositoryService.Create(c.Request.Context(), &repository.CreateRequest{
		OrganizationID: tenant.OrganizationID,
		GitProviderID:  req.GitProviderID,
		ExternalID:     req.ExternalID,
		Name:           req.Name,
		FullPath:       req.FullPath,
		DefaultBranch:  defaultBranch,
		TeamID:         req.TeamID,
		TicketPrefix:   ticketPrefix,
	})
	if err != nil {
		if err == repository.ErrRepositoryExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Repository already configured"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create repository"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"repository": repo})
}

// GetRepository returns repository by ID
// GET /api/v1/organizations/:slug/repositories/:id
func (h *RepositoryHandler) GetRepository(c *gin.Context) {
	repoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid repository ID"})
		return
	}

	repo, err := h.repositoryService.GetByID(c.Request.Context(), repoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if repo.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"repository": repo})
}

// UpdateRepositoryRequest represents repository update request
type UpdateRepositoryRequest struct {
	Name          string `json:"name"`
	DefaultBranch string `json:"default_branch"`
	TeamID        *int64 `json:"team_id"`
	TicketPrefix  string `json:"ticket_prefix"`
	IsActive      *bool  `json:"is_active"`
}

// UpdateRepository updates a repository configuration
// PUT /api/v1/organizations/:slug/repositories/:id
func (h *RepositoryHandler) UpdateRepository(c *gin.Context) {
	repoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid repository ID"})
		return
	}

	var req UpdateRepositoryRequest
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

	repo, err := h.repositoryService.GetByID(c.Request.Context(), repoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found"})
		return
	}

	if repo.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.DefaultBranch != "" {
		updates["default_branch"] = req.DefaultBranch
	}
	if req.TeamID != nil {
		updates["team_id"] = req.TeamID
	}
	if req.TicketPrefix != "" {
		updates["ticket_prefix"] = req.TicketPrefix
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	repo, err = h.repositoryService.Update(c.Request.Context(), repoID, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update repository"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"repository": repo})
}

// DeleteRepository deletes a repository configuration
// DELETE /api/v1/organizations/:slug/repositories/:id
func (h *RepositoryHandler) DeleteRepository(c *gin.Context) {
	repoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid repository ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	repo, err := h.repositoryService.GetByID(c.Request.Context(), repoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found"})
		return
	}

	if repo.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.repositoryService.Delete(c.Request.Context(), repoID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete repository"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Repository deleted"})
}

// SyncBranchesRequest represents sync branches request
type SyncBranchesRequest struct {
	AccessToken string `json:"access_token" binding:"required"`
}

// SyncBranches syncs branches from git provider
// POST /api/v1/organizations/:slug/repositories/:id/sync-branches
func (h *RepositoryHandler) SyncBranches(c *gin.Context) {
	repoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid repository ID"})
		return
	}

	var req SyncBranchesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	repo, err := h.repositoryService.GetByID(c.Request.Context(), repoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found"})
		return
	}

	if repo.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	branches, err := h.repositoryService.ListBranches(c.Request.Context(), repoID, req.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync branches"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"branches": branches})
}

// ListBranches lists repository branches
// GET /api/v1/organizations/:slug/repositories/:id/branches
func (h *RepositoryHandler) ListBranches(c *gin.Context) {
	repoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid repository ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	repo, err := h.repositoryService.GetByID(c.Request.Context(), repoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found"})
		return
	}

	if repo.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get access token from query or header
	accessToken := c.Query("access_token")
	if accessToken == "" {
		accessToken = c.GetHeader("X-Git-Access-Token")
	}
	if accessToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Access token required"})
		return
	}

	branches, err := h.repositoryService.ListBranches(c.Request.Context(), repoID, accessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list branches"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"branches": branches})
}

// SetupWebhook sets up webhook for repository
// POST /api/v1/organizations/:slug/repositories/:id/webhook
func (h *RepositoryHandler) SetupWebhook(c *gin.Context) {
	// TODO: Implement webhook setup
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Webhook setup not implemented yet",
	})
}
