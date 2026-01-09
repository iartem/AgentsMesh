package v1

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/anthropics/agentmesh/backend/internal/middleware"
	"github.com/anthropics/agentmesh/backend/internal/service/sshkey"
	"github.com/gin-gonic/gin"
)

// SSHKeyHandler handles SSH key-related requests
type SSHKeyHandler struct {
	sshKeyService sshkey.Interface
}

// NewSSHKeyHandler creates a new SSH key handler
func NewSSHKeyHandler(sshKeyService sshkey.Interface) *SSHKeyHandler {
	return &SSHKeyHandler{
		sshKeyService: sshKeyService,
	}
}

// ListSSHKeys lists SSH keys for an organization
// GET /api/v1/organizations/:slug/ssh-keys
func (h *SSHKeyHandler) ListSSHKeys(c *gin.Context) {
	tenant := middleware.GetTenant(c)

	keys, err := h.sshKeyService.ListByOrganization(c.Request.Context(), tenant.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list SSH keys"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ssh_keys": keys})
}

// CreateSSHKeyRequest represents SSH key creation request
type CreateSSHKeyRequest struct {
	Name       string  `json:"name" binding:"required,min=2,max=100"`
	PrivateKey *string `json:"private_key"` // Optional: if nil, generate a new key pair
}

// CreateSSHKey creates a new SSH key
// POST /api/v1/organizations/:slug/ssh-keys
func (h *SSHKeyHandler) CreateSSHKey(c *gin.Context) {
	var req CreateSSHKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	key, err := h.sshKeyService.Create(c.Request.Context(), &sshkey.CreateRequest{
		OrganizationID: tenant.OrganizationID,
		Name:           req.Name,
		PrivateKey:     req.PrivateKey,
	})
	if err != nil {
		if errors.Is(err, sshkey.ErrSSHKeyNameExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "SSH key name already exists"})
			return
		}
		if errors.Is(err, sshkey.ErrInvalidPrivateKey) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create SSH key"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"ssh_key": key})
}

// GetSSHKey returns an SSH key by ID
// GET /api/v1/organizations/:slug/ssh-keys/:id
func (h *SSHKeyHandler) GetSSHKey(c *gin.Context) {
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SSH key ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	key, err := h.sshKeyService.GetByIDAndOrg(c.Request.Context(), keyID, tenant.OrganizationID)
	if err != nil {
		if errors.Is(err, sshkey.ErrSSHKeyNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "SSH key not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get SSH key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ssh_key": key})
}

// UpdateSSHKeyRequest represents SSH key update request
type UpdateSSHKeyRequest struct {
	Name string `json:"name" binding:"required,min=2,max=100"`
}

// UpdateSSHKey updates an SSH key
// PUT /api/v1/organizations/:slug/ssh-keys/:id
func (h *SSHKeyHandler) UpdateSSHKey(c *gin.Context) {
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SSH key ID"})
		return
	}

	var req UpdateSSHKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	// Verify the key belongs to this organization
	_, err = h.sshKeyService.GetByIDAndOrg(c.Request.Context(), keyID, tenant.OrganizationID)
	if err != nil {
		if errors.Is(err, sshkey.ErrSSHKeyNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "SSH key not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get SSH key"})
		return
	}

	key, err := h.sshKeyService.Update(c.Request.Context(), keyID, req.Name)
	if err != nil {
		if errors.Is(err, sshkey.ErrSSHKeyNameExists) {
			c.JSON(http.StatusConflict, gin.H{"error": "SSH key name already exists"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update SSH key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ssh_key": key})
}

// DeleteSSHKey deletes an SSH key
// DELETE /api/v1/organizations/:slug/ssh-keys/:id
func (h *SSHKeyHandler) DeleteSSHKey(c *gin.Context) {
	keyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid SSH key ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	// Verify the key belongs to this organization
	_, err = h.sshKeyService.GetByIDAndOrg(c.Request.Context(), keyID, tenant.OrganizationID)
	if err != nil {
		if errors.Is(err, sshkey.ErrSSHKeyNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "SSH key not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get SSH key"})
		return
	}

	if err := h.sshKeyService.Delete(c.Request.Context(), keyID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete SSH key"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "SSH key deleted"})
}
