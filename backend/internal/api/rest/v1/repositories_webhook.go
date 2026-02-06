package v1

import (
	"net/http"
	"strconv"

	"github.com/anthropics/agentsmesh/backend/internal/middleware"
	"github.com/anthropics/agentsmesh/backend/internal/service/repository"
	"github.com/gin-gonic/gin"
)

// RegisterRepositoryWebhook registers a webhook for a repository
// POST /api/v1/organizations/:slug/repositories/:id/webhook
func (h *RepositoryHandler) RegisterRepositoryWebhook(c *gin.Context) {
	repoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid repository ID"})
		return
	}

	tenant := middleware.GetTenant(c)
	userID := middleware.GetUserID(c)

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	// Get repository
	repo, err := h.repositoryService.GetByID(c.Request.Context(), repoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found"})
		return
	}

	if repo.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get webhook service
	webhookService := h.repositoryService.GetWebhookService()
	if webhookService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Webhook service not available"})
		return
	}

	// Register webhook
	result, err := webhookService.RegisterWebhookForRepository(c.Request.Context(), repo, tenant.OrganizationSlug, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": result})
}

// DeleteRepositoryWebhook deletes a webhook from a repository
// DELETE /api/v1/organizations/:slug/repositories/:id/webhook
func (h *RepositoryHandler) DeleteRepositoryWebhook(c *gin.Context) {
	repoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid repository ID"})
		return
	}

	tenant := middleware.GetTenant(c)
	userID := middleware.GetUserID(c)

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	// Get repository
	repo, err := h.repositoryService.GetByID(c.Request.Context(), repoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found"})
		return
	}

	if repo.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get webhook service
	webhookService := h.repositoryService.GetWebhookService()
	if webhookService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Webhook service not available"})
		return
	}

	// Delete webhook
	if err := webhookService.DeleteWebhookForRepository(c.Request.Context(), repo, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Webhook deleted"})
}

// GetRepositoryWebhookStatus returns the webhook status for a repository
// GET /api/v1/organizations/:slug/repositories/:id/webhook/status
func (h *RepositoryHandler) GetRepositoryWebhookStatus(c *gin.Context) {
	repoID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid repository ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	// Get repository
	repo, err := h.repositoryService.GetByID(c.Request.Context(), repoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found"})
		return
	}

	if repo.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get webhook service
	webhookService := h.repositoryService.GetWebhookService()
	if webhookService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Webhook service not available"})
		return
	}

	status := webhookService.GetWebhookStatus(c.Request.Context(), repo)
	c.JSON(http.StatusOK, gin.H{"webhook_status": status})
}

// GetRepositoryWebhookSecret returns the webhook secret for manual configuration
// GET /api/v1/organizations/:slug/repositories/:id/webhook/secret
// Only returns secret if webhook needs manual setup
func (h *RepositoryHandler) GetRepositoryWebhookSecret(c *gin.Context) {
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

	// Get repository
	repo, err := h.repositoryService.GetByID(c.Request.Context(), repoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found"})
		return
	}

	if repo.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get webhook service
	webhookService := h.repositoryService.GetWebhookService()
	if webhookService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Webhook service not available"})
		return
	}

	secret, err := webhookService.GetWebhookSecret(c.Request.Context(), repo)
	if err != nil {
		if err == repository.ErrWebhookNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not configured"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Return the secret along with webhook URL for manual configuration
	c.JSON(http.StatusOK, gin.H{
		"webhook_url":    repo.WebhookConfig.URL,
		"webhook_secret": secret,
		"events":         repo.WebhookConfig.Events,
	})
}

// MarkRepositoryWebhookConfigured marks a webhook as manually configured
// POST /api/v1/organizations/:slug/repositories/:id/webhook/configured
func (h *RepositoryHandler) MarkRepositoryWebhookConfigured(c *gin.Context) {
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

	// Get repository
	repo, err := h.repositoryService.GetByID(c.Request.Context(), repoID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Repository not found"})
		return
	}

	if repo.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Get webhook service
	webhookService := h.repositoryService.GetWebhookService()
	if webhookService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Webhook service not available"})
		return
	}

	if err := webhookService.MarkWebhookAsConfigured(c.Request.Context(), repo); err != nil {
		if err == repository.ErrWebhookNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Webhook not configured"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Webhook marked as configured"})
}
