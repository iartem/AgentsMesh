package v1

import (
	"net/http"
	"strconv"

	"github.com/anthropics/agentmesh/backend/internal/middleware"
	"github.com/anthropics/agentmesh/backend/internal/service/gitprovider"
	"github.com/gin-gonic/gin"
)

// GitProviderHandler handles git provider-related requests
type GitProviderHandler struct {
	gitProviderService *gitprovider.Service
}

// NewGitProviderHandler creates a new git provider handler
func NewGitProviderHandler(gitProviderService *gitprovider.Service) *GitProviderHandler {
	return &GitProviderHandler{
		gitProviderService: gitProviderService,
	}
}

// ListGitProviders lists configured git providers
// GET /api/v1/organizations/:slug/git-providers
func (h *GitProviderHandler) ListGitProviders(c *gin.Context) {
	tenant := middleware.GetTenant(c)

	providers, err := h.gitProviderService.ListByOrganization(c.Request.Context(), tenant.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list git providers"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"git_providers": providers})
}

// CreateGitProviderRequest represents git provider creation request
type CreateGitProviderRequest struct {
	ProviderType string `json:"provider_type" binding:"required,oneof=gitlab github gitee"`
	Name         string `json:"name" binding:"required,min=2,max=100"`
	BaseURL      string `json:"base_url" binding:"required,url"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	BotToken     string `json:"bot_token"`
	IsDefault    bool   `json:"is_default"`
}

// CreateGitProvider creates a new git provider configuration
// POST /api/v1/organizations/:slug/git-providers
func (h *GitProviderHandler) CreateGitProvider(c *gin.Context) {
	var req CreateGitProviderRequest
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

	var clientID, clientSecret, botToken *string
	if req.ClientID != "" {
		clientID = &req.ClientID
	}
	if req.ClientSecret != "" {
		clientSecret = &req.ClientSecret
	}
	if req.BotToken != "" {
		botToken = &req.BotToken
	}

	provider, err := h.gitProviderService.Create(c.Request.Context(), &gitprovider.CreateRequest{
		OrganizationID:      tenant.OrganizationID,
		ProviderType:        req.ProviderType,
		Name:                req.Name,
		BaseURL:             req.BaseURL,
		ClientID:            clientID,
		ClientSecretEncrypt: clientSecret,
		BotTokenEncrypt:     botToken,
		IsDefault:           req.IsDefault,
	})
	if err != nil {
		if err == gitprovider.ErrProviderNameExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Git provider name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create git provider"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"git_provider": provider})
}

// GetGitProvider returns git provider by ID
// GET /api/v1/organizations/:slug/git-providers/:id
func (h *GitProviderHandler) GetGitProvider(c *gin.Context) {
	providerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	provider, err := h.gitProviderService.GetByID(c.Request.Context(), providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Git provider not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if provider.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"git_provider": provider})
}

// UpdateGitProviderRequest represents git provider update request
type UpdateGitProviderRequest struct {
	Name         string `json:"name"`
	BaseURL      string `json:"base_url"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	BotToken     string `json:"bot_token"`
	IsDefault    *bool  `json:"is_default"`
	IsActive     *bool  `json:"is_active"`
}

// UpdateGitProvider updates a git provider configuration
// PUT /api/v1/organizations/:slug/git-providers/:id
func (h *GitProviderHandler) UpdateGitProvider(c *gin.Context) {
	providerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	var req UpdateGitProviderRequest
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

	provider, err := h.gitProviderService.GetByID(c.Request.Context(), providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Git provider not found"})
		return
	}

	if provider.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.BaseURL != "" {
		updates["base_url"] = req.BaseURL
	}
	if req.ClientID != "" {
		updates["client_id"] = req.ClientID
	}
	if req.ClientSecret != "" {
		updates["client_secret_encrypted"] = req.ClientSecret // Will be encrypted by service
	}
	if req.BotToken != "" {
		updates["bot_token_encrypted"] = req.BotToken // Will be encrypted by service
	}
	if req.IsDefault != nil {
		updates["is_default"] = *req.IsDefault
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	provider, err = h.gitProviderService.Update(c.Request.Context(), providerID, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update git provider"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"git_provider": provider})
}

// DeleteGitProvider deletes a git provider configuration
// DELETE /api/v1/organizations/:slug/git-providers/:id
func (h *GitProviderHandler) DeleteGitProvider(c *gin.Context) {
	providerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	provider, err := h.gitProviderService.GetByID(c.Request.Context(), providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Git provider not found"})
		return
	}

	if provider.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.gitProviderService.Delete(c.Request.Context(), providerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete git provider"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Git provider deleted"})
}

// TestConnection tests git provider connection
// POST /api/v1/organizations/:slug/git-providers/:id/test
func (h *GitProviderHandler) TestConnection(c *gin.Context) {
	providerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	provider, err := h.gitProviderService.GetByID(c.Request.Context(), providerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Git provider not found"})
		return
	}

	if provider.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get the access token for testing (from request body or use bot token)
	var reqBody struct {
		AccessToken string `json:"access_token"`
	}
	c.ShouldBindJSON(&reqBody)

	accessToken := reqBody.AccessToken
	if accessToken == "" && provider.BotTokenEncrypted != nil {
		accessToken = *provider.BotTokenEncrypted // TODO: decrypt
	}

	if accessToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Access token required for testing"})
		return
	}

	err = h.gitProviderService.TestConnection(c.Request.Context(), provider.ProviderType, provider.BaseURL, accessToken)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
	})
}

// SyncProjects syncs projects from git provider
// POST /api/v1/organizations/:slug/git-providers/:id/sync
func (h *GitProviderHandler) SyncProjects(c *gin.Context) {
	// This endpoint is a placeholder - actual sync requires user access token
	// and should use the repository service to import projects
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": "Project sync not implemented. Use the repositories API to import projects.",
	})
}
