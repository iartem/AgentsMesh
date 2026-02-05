package v1

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/anthropics/agentsmesh/backend/internal/middleware"
	"github.com/anthropics/agentsmesh/backend/internal/service/runner"
	"github.com/gin-gonic/gin"
)

// ==================== Pre-generated Token Registration ====================

// GenerateGRPCToken creates a new pre-generated registration token.
// POST /api/v1/organizations/:slug/runners/grpc/tokens
// Requires JWT authentication (admin).
func (h *GRPCRunnerHandler) GenerateGRPCToken(c *gin.Context) {
	var req GenerateGRPCTokenRequest
	// Allow empty body - all fields are optional
	_ = c.ShouldBindJSON(&req)

	tenant := middleware.GetTenant(c)
	if tenant == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	// BaseURL is derived from PrimaryDomain
	serverURL := h.config.BaseURL()

	resp, err := h.runnerService.GenerateGRPCRegistrationToken(
		c.Request.Context(),
		tenant.OrganizationID,
		tenant.UserID,
		&runner.GenerateGRPCRegistrationTokenRequest{
			Name:      req.Name,
			Labels:    req.Labels,
			SingleUse: req.SingleUse,
			MaxUses:   req.MaxUses,
			ExpiresIn: req.ExpiresIn,
		},
		serverURL,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"token":      resp.Token,
		"expires_at": resp.ExpiresAt,
		"command":    resp.Command,
		"message":    "Save this token securely. It will only be shown once.",
	})
}

// ListGRPCTokens lists all gRPC registration tokens for an organization.
// GET /api/v1/organizations/:slug/runners/grpc/tokens
// Requires JWT authentication (admin).
func (h *GRPCRunnerHandler) ListGRPCTokens(c *gin.Context) {
	tenant := middleware.GetTenant(c)
	if tenant == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	tokens, err := h.runnerService.ListGRPCRegistrationTokens(c.Request.Context(), tenant.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list tokens"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"tokens": tokens})
}

// DeleteGRPCToken deletes a gRPC registration token.
// DELETE /api/v1/organizations/:slug/runners/grpc/tokens/:id
// Requires JWT authentication (admin).
func (h *GRPCRunnerHandler) DeleteGRPCToken(c *gin.Context) {
	tokenID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid token ID"})
		return
	}

	tenant := middleware.GetTenant(c)
	if tenant == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return
	}

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	if err := h.runnerService.DeleteGRPCRegistrationToken(c.Request.Context(), tokenID); err != nil {
		if errors.Is(err, runner.ErrGRPCTokenNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Token deleted"})
}

// RegisterWithToken registers a new runner using a pre-generated token.
// POST /api/v1/runners/grpc/register
// No authentication required - token serves as authentication.
func (h *GRPCRunnerHandler) RegisterWithToken(c *gin.Context) {
	var req RegisterWithTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.runnerService.RegisterWithToken(
		c.Request.Context(),
		&runner.RegisterWithTokenRequest{
			Token:  req.Token,
			NodeID: req.NodeID,
		},
		h.pkiService,
	)
	if err != nil {
		switch err {
		case runner.ErrInvalidToken:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		case runner.ErrTokenExpired:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token expired"})
		case runner.ErrTokenExhausted:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Token usage exhausted"})
		case runner.ErrRunnerAlreadyExists:
			c.JSON(http.StatusConflict, gin.H{"error": "Runner with this node_id already exists"})
		case runner.ErrRunnerQuotaExceeded:
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error": "Runner quota exceeded",
				"code":  "RUNNER_QUOTA_EXCEEDED",
			})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register runner"})
		}
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"runner_id":      resp.RunnerID,
		"certificate":    resp.Certificate,
		"private_key":    resp.PrivateKey,
		"ca_certificate": resp.CACertificate,
		"org_slug":       resp.OrgSlug,
		"grpc_endpoint":  h.config.GRPC.Endpoint,
	})
}
