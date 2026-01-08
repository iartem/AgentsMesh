package v1

import (
	"net/http"
	"strconv"

	"github.com/anthropics/agentmesh/backend/internal/middleware"
	"github.com/anthropics/agentmesh/backend/internal/service/devpod"
	"github.com/gin-gonic/gin"
)

// DevPodHandler handles DevPod settings and AI provider endpoints
type DevPodHandler struct {
	settingsService   *devpod.SettingsService
	aiProviderService *devpod.AIProviderService
}

// NewDevPodHandler creates a new DevPod handler
func NewDevPodHandler(settingsService *devpod.SettingsService, aiProviderService *devpod.AIProviderService) *DevPodHandler {
	return &DevPodHandler{
		settingsService:   settingsService,
		aiProviderService: aiProviderService,
	}
}

// GetSettings returns the DevPod settings for the current user
// GET /api/v1/users/me/devpod/settings
func (h *DevPodHandler) GetSettings(c *gin.Context) {
	userID := middleware.GetUserID(c)

	settings, err := h.settingsService.GetUserSettings(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

// UpdateSettingsRequest represents settings update request
type UpdateSettingsRequest struct {
	PreparationScript  *string `json:"preparation_script"`
	PreparationTimeout *int    `json:"preparation_timeout" binding:"omitempty,min=30,max=3600"`
	DefaultAgentTypeID *int64  `json:"default_agent_type_id"`
	DefaultModel       *string `json:"default_model"`
	DefaultPermMode    *string `json:"default_perm_mode" binding:"omitempty,oneof=default accept-edits full-auto"`
	TerminalFontSize   *int    `json:"terminal_font_size" binding:"omitempty,min=8,max=32"`
	TerminalTheme      *string `json:"terminal_theme"`
}

// UpdateSettings updates the DevPod settings for the current user
// PUT /api/v1/users/me/devpod/settings
func (h *DevPodHandler) UpdateSettings(c *gin.Context) {
	var req UpdateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)

	updates := &devpod.UserSettingsUpdate{
		PreparationScript:  req.PreparationScript,
		PreparationTimeout: req.PreparationTimeout,
		DefaultAgentTypeID: req.DefaultAgentTypeID,
		DefaultModel:       req.DefaultModel,
		DefaultPermMode:    req.DefaultPermMode,
		TerminalFontSize:   req.TerminalFontSize,
		TerminalTheme:      req.TerminalTheme,
	}

	settings, err := h.settingsService.UpdateUserSettings(c.Request.Context(), userID, updates)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update settings"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"settings": settings})
}

// ListProviders returns all AI providers for the current user
// GET /api/v1/users/me/devpod/providers
func (h *DevPodHandler) ListProviders(c *gin.Context) {
	userID := middleware.GetUserID(c)

	providers, err := h.aiProviderService.GetUserProviders(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list providers"})
		return
	}

	// Don't return encrypted credentials
	for _, p := range providers {
		p.EncryptedCredentials = ""
	}

	c.JSON(http.StatusOK, gin.H{"providers": providers})
}

// CreateProviderRequest represents AI provider creation request
type CreateProviderRequest struct {
	ProviderType string            `json:"provider_type" binding:"required,oneof=claude gemini codex openai"`
	Name         string            `json:"name" binding:"required,min=1,max=100"`
	Credentials  map[string]string `json:"credentials" binding:"required"`
	IsDefault    bool              `json:"is_default"`
}

// CreateProvider creates a new AI provider for the current user
// POST /api/v1/users/me/devpod/providers
func (h *DevPodHandler) CreateProvider(c *gin.Context) {
	var req CreateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Validate credentials
	if err := h.aiProviderService.ValidateCredentials(req.ProviderType, req.Credentials); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := middleware.GetUserID(c)

	provider, err := h.aiProviderService.CreateUserProvider(
		c.Request.Context(),
		userID,
		req.ProviderType,
		req.Name,
		req.Credentials,
		req.IsDefault,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create provider"})
		return
	}

	// Don't return encrypted credentials
	provider.EncryptedCredentials = ""

	c.JSON(http.StatusCreated, gin.H{"provider": provider})
}

// UpdateProviderRequest represents AI provider update request
type UpdateProviderRequest struct {
	Name        string            `json:"name" binding:"omitempty,min=1,max=100"`
	Credentials map[string]string `json:"credentials"`
	IsDefault   *bool             `json:"is_default"`
	IsEnabled   *bool             `json:"is_enabled"`
}

// UpdateProvider updates an AI provider
// PUT /api/v1/users/me/devpod/providers/:id
func (h *DevPodHandler) UpdateProvider(c *gin.Context) {
	idStr := c.Param("id")
	providerID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	var req UpdateProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Set defaults for booleans
	isDefault := false
	if req.IsDefault != nil {
		isDefault = *req.IsDefault
	}
	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}

	provider, err := h.aiProviderService.UpdateUserProvider(
		c.Request.Context(),
		providerID,
		req.Name,
		req.Credentials,
		isDefault,
		isEnabled,
	)
	if err != nil {
		if err == devpod.ErrProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update provider"})
		return
	}

	// Don't return encrypted credentials
	provider.EncryptedCredentials = ""

	c.JSON(http.StatusOK, gin.H{"provider": provider})
}

// DeleteProvider deletes an AI provider
// DELETE /api/v1/users/me/devpod/providers/:id
func (h *DevPodHandler) DeleteProvider(c *gin.Context) {
	idStr := c.Param("id")
	providerID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	if err := h.aiProviderService.DeleteUserProvider(c.Request.Context(), providerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete provider"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Provider deleted"})
}

// SetDefaultProvider sets a provider as the default for its type
// POST /api/v1/users/me/devpod/providers/:id/default
func (h *DevPodHandler) SetDefaultProvider(c *gin.Context) {
	idStr := c.Param("id")
	providerID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid provider ID"})
		return
	}

	if err := h.aiProviderService.SetDefaultProvider(c.Request.Context(), providerID); err != nil {
		if err == devpod.ErrProviderNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "Provider not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to set default provider"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Default provider set"})
}
