package v1

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/anthropics/agentsmesh/backend/internal/middleware"
	"github.com/anthropics/agentsmesh/backend/internal/service/license"
	"github.com/gin-gonic/gin"
)

// LicenseHandler handles license-related HTTP requests
type LicenseHandler struct {
	licenseService *license.Service
}

// NewLicenseHandler creates a new license handler
func NewLicenseHandler(licenseService *license.Service) *LicenseHandler {
	return &LicenseHandler{licenseService: licenseService}
}

// GetLicenseStatus returns the current license status
// @Summary Get license status
// @Description Returns the current license status for OnPremise deployments
// @Tags license
// @Produce json
// @Success 200 {object} billing.LicenseStatus
// @Router /api/v1/license/status [get]
func (h *LicenseHandler) GetLicenseStatus(c *gin.Context) {
	if h.licenseService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "license service not configured",
			"message": "This endpoint is only available for OnPremise deployments",
		})
		return
	}

	status := h.licenseService.GetLicenseStatus()
	c.JSON(http.StatusOK, status)
}

// ActivateLicenseRequest represents the license activation request
type ActivateLicenseRequest struct {
	LicenseData string `json:"license_data"` // Base64 encoded or raw JSON license data
}

// ActivateLicense activates a new license
// @Summary Activate license
// @Description Activates a new license for OnPremise deployments
// @Tags license
// @Accept json
// @Produce json
// @Param request body ActivateLicenseRequest true "License activation request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 403 {object} map[string]string
// @Router /api/v1/license/activate [post]
func (h *LicenseHandler) ActivateLicense(c *gin.Context) {
	if h.licenseService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "license service not configured",
			"message": "This endpoint is only available for OnPremise deployments",
		})
		return
	}

	// Check if user is system admin (for multi-tenant, only admins can activate licenses)
	tenant, exists := c.Get("tenant")
	if exists {
		tc := tenant.(*middleware.TenantContext)
		if tc.UserRole != "owner" {
			c.JSON(http.StatusForbidden, gin.H{"error": "only organization owners can activate licenses"})
			return
		}
	}

	// Try to parse JSON request first
	var req ActivateLicenseRequest
	if err := c.ShouldBindJSON(&req); err == nil && req.LicenseData != "" {
		if err := h.licenseService.ActivateLicense(c.Request.Context(), []byte(req.LicenseData)); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	} else {
		// Fall back to reading raw body (for file upload)
		c.Request.Body = io.NopCloser(c.Request.Body)
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
			return
		}

		if len(body) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "license data is required"})
			return
		}

		if err := h.licenseService.ActivateLicense(c.Request.Context(), body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	// Return updated status
	status := h.licenseService.GetLicenseStatus()
	c.JSON(http.StatusOK, gin.H{
		"message": "license activated successfully",
		"status":  status,
	})
}

// UploadLicense handles license file upload
// @Summary Upload license file
// @Description Uploads and activates a license file for OnPremise deployments
// @Tags license
// @Accept multipart/form-data
// @Produce json
// @Param file formData file true "License file"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /api/v1/license/upload [post]
func (h *LicenseHandler) UploadLicense(c *gin.Context) {
	if h.licenseService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "license service not configured",
			"message": "This endpoint is only available for OnPremise deployments",
		})
		return
	}

	// Check permissions
	tenant, exists := c.Get("tenant")
	if exists {
		tc := tenant.(*middleware.TenantContext)
		if tc.UserRole != "owner" {
			c.JSON(http.StatusForbidden, gin.H{"error": "only organization owners can activate licenses"})
			return
		}
	}

	// Get file from form
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "license file is required"})
		return
	}

	// Open and read file
	f, err := file.Open()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to open license file"})
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read license file"})
		return
	}

	// Activate license
	if err := h.licenseService.ActivateLicense(c.Request.Context(), data); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status := h.licenseService.GetLicenseStatus()
	c.JSON(http.StatusOK, gin.H{
		"message": "license activated successfully",
		"status":  status,
	})
}

// RefreshLicense reloads the license from file
// @Summary Refresh license
// @Description Reloads the license from the configured file path
// @Tags license
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /api/v1/license/refresh [post]
func (h *LicenseHandler) RefreshLicense(c *gin.Context) {
	if h.licenseService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "license service not configured",
			"message": "This endpoint is only available for OnPremise deployments",
		})
		return
	}

	if err := h.licenseService.RefreshLicense(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	status := h.licenseService.GetLicenseStatus()
	c.JSON(http.StatusOK, gin.H{
		"message": "license refreshed successfully",
		"status":  status,
	})
}

// CheckFeature checks if a specific feature is enabled
// @Summary Check feature
// @Description Checks if a specific feature is enabled in the current license
// @Tags license
// @Produce json
// @Param feature query string true "Feature name"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/license/feature [get]
func (h *LicenseHandler) CheckFeature(c *gin.Context) {
	if h.licenseService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "license service not configured",
			"message": "This endpoint is only available for OnPremise deployments",
		})
		return
	}

	feature := c.Query("feature")
	if feature == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "feature parameter is required"})
		return
	}

	enabled := h.licenseService.HasFeature(feature)
	c.JSON(http.StatusOK, gin.H{
		"feature": feature,
		"enabled": enabled,
	})
}

// GetLicenseLimits returns the current license limits
// @Summary Get license limits
// @Description Returns the resource limits defined in the current license
// @Tags license
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/license/limits [get]
func (h *LicenseHandler) GetLicenseLimits(c *gin.Context) {
	if h.licenseService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "license service not configured",
			"message": "This endpoint is only available for OnPremise deployments",
		})
		return
	}

	licenseData := h.licenseService.GetCurrentLicense()
	if licenseData == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active license"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"limits": licenseData.Limits,
		"plan":   licenseData.Plan,
	})
}

// ValidateLicenseRequest represents a license validation request
type ValidateLicenseRequest struct {
	LicenseData string `json:"license_data" binding:"required"`
}

// ValidateLicense validates license data without activating
// @Summary Validate license
// @Description Validates license data and returns parsed information without activating
// @Tags license
// @Accept json
// @Produce json
// @Param request body ValidateLicenseRequest true "License validation request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /api/v1/license/validate [post]
func (h *LicenseHandler) ValidateLicense(c *gin.Context) {
	if h.licenseService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"error":   "license service not configured",
			"message": "This endpoint is only available for OnPremise deployments",
		})
		return
	}

	var req ValidateLicenseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Parse and verify without activating
	licenseData, err := h.licenseService.ParseAndVerify([]byte(req.LicenseData))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"valid": false,
			"error": err.Error(),
		})
		return
	}

	// Return parsed license info (without signature)
	c.JSON(http.StatusOK, gin.H{
		"valid":             true,
		"license_key":       licenseData.LicenseKey,
		"organization_name": licenseData.OrganizationName,
		"contact_email":     licenseData.ContactEmail,
		"plan":              licenseData.Plan,
		"limits":            licenseData.Limits,
		"features":          licenseData.Features,
		"issued_at":         licenseData.IssuedAt,
		"expires_at":        licenseData.ExpiresAt,
	})
}

// RegisterLicenseHandlers registers license routes
func RegisterLicenseHandlers(rg *gin.RouterGroup, licenseService *license.Service) {
	handler := NewLicenseHandler(licenseService)

	// Public endpoints (no auth required for status)
	rg.GET("/status", handler.GetLicenseStatus)
	rg.GET("/limits", handler.GetLicenseLimits)
	rg.GET("/feature", handler.CheckFeature)

	// Protected endpoints
	rg.POST("/activate", handler.ActivateLicense)
	rg.POST("/upload", handler.UploadLicense)
	rg.POST("/refresh", handler.RefreshLicense)
	rg.POST("/validate", handler.ValidateLicense)
}

// LicenseStatusResponse represents the license status response for API documentation
type LicenseStatusResponse struct {
	IsActive         bool     `json:"is_active"`
	LicenseKey       string   `json:"license_key,omitempty"`
	OrganizationName string   `json:"organization_name,omitempty"`
	Plan             string   `json:"plan,omitempty"`
	ExpiresAt        string   `json:"expires_at,omitempty"`
	MaxUsers         int      `json:"max_users,omitempty"`
	MaxRunners       int      `json:"max_runners,omitempty"`
	MaxRepositories  int      `json:"max_repositories,omitempty"`
	MaxPodMinutes    int      `json:"max_pod_minutes,omitempty"`
	Features         []string `json:"features,omitempty"`
	Message          string   `json:"message"`
}

// UnmarshalJSON implements custom JSON unmarshaling for license activation
func (r *ActivateLicenseRequest) UnmarshalJSON(data []byte) error {
	// Try to unmarshal as JSON object first
	type Alias ActivateLicenseRequest
	aux := &struct {
		*Alias
	}{
		Alias: (*Alias)(r),
	}

	if err := json.Unmarshal(data, aux); err == nil {
		return nil
	}

	// If that fails, treat the entire body as license data
	r.LicenseData = string(data)
	return nil
}
