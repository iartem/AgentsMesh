package v1

import (
	"net/http"
	"strconv"

	"github.com/anthropics/agentmesh/backend/internal/service/binding"
	"github.com/gin-gonic/gin"
)

// BindingHandler handles binding API endpoints
type BindingHandler struct {
	bindingSvc *binding.Service
}

// NewBindingHandler creates a new binding handler
func NewBindingHandler(bindingSvc *binding.Service) *BindingHandler {
	return &BindingHandler{
		bindingSvc: bindingSvc,
	}
}

// BindingRequest represents a request to create a binding
type BindingRequest struct {
	TargetSession string   `json:"target_session" binding:"required"`
	Scopes        []string `json:"scopes" binding:"required"`
	Policy        string   `json:"policy,omitempty"`
}

// AcceptRequest represents a request to accept a binding
type AcceptRequest struct {
	BindingID int64 `json:"binding_id" binding:"required"`
}

// RejectRequest represents a request to reject a binding
type RejectRequest struct {
	BindingID int64  `json:"binding_id" binding:"required"`
	Reason    string `json:"reason,omitempty"`
}

// ScopeRequest represents a request for additional scopes
type ScopeRequest struct {
	Scopes []string `json:"scopes" binding:"required"`
}

// UnbindRequest represents a request to unbind
type UnbindRequest struct {
	TargetSession string `json:"target_session" binding:"required"`
}

// getSessionKeyFromHeader extracts session key from X-Session-Key header
func getSessionKeyFromHeader(c *gin.Context) string {
	return c.GetHeader("X-Session-Key")
}

// RequestBinding handles POST /bindings
// @Summary Request binding to another session
// @Tags bindings
// @Accept json
// @Produce json
// @Param X-Session-Key header string true "Session Key"
// @Param request body BindingRequest true "Binding request"
// @Success 201 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Router /bindings [post]
func (h *BindingHandler) RequestBinding(c *gin.Context) {
	sessionKey := getSessionKeyFromHeader(c)
	if sessionKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Session-Key header required"})
		return
	}

	var req BindingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get org ID from context
	orgID, _ := c.Get("organization_id")
	orgIDInt64, ok := orgID.(int64)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "invalid organization context"})
		return
	}

	binding, err := h.bindingSvc.RequestBinding(c.Request.Context(), orgIDInt64, sessionKey, req.TargetSession, req.Scopes, req.Policy)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"binding": binding})
}

// AcceptBinding handles POST /bindings/accept
// @Summary Accept a pending binding request
// @Tags bindings
// @Accept json
// @Produce json
// @Param X-Session-Key header string true "Session Key"
// @Param request body AcceptRequest true "Accept request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Router /bindings/accept [post]
func (h *BindingHandler) AcceptBinding(c *gin.Context) {
	sessionKey := getSessionKeyFromHeader(c)
	if sessionKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Session-Key header required"})
		return
	}

	var req AcceptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	binding, err := h.bindingSvc.AcceptBinding(c.Request.Context(), req.BindingID, sessionKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"binding": binding})
}

// RejectBinding handles POST /bindings/reject
// @Summary Reject a pending binding request
// @Tags bindings
// @Accept json
// @Produce json
// @Param X-Session-Key header string true "Session Key"
// @Param request body RejectRequest true "Reject request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Router /bindings/reject [post]
func (h *BindingHandler) RejectBinding(c *gin.Context) {
	sessionKey := getSessionKeyFromHeader(c)
	if sessionKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Session-Key header required"})
		return
	}

	var req RejectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	binding, err := h.bindingSvc.RejectBinding(c.Request.Context(), req.BindingID, sessionKey, req.Reason)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"binding": binding})
}

// RequestScopes handles POST /bindings/:id/scopes
// @Summary Request additional scopes on an existing binding
// @Tags bindings
// @Accept json
// @Produce json
// @Param X-Session-Key header string true "Session Key"
// @Param id path int true "Binding ID"
// @Param request body ScopeRequest true "Scope request"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Router /bindings/{id}/scopes [post]
func (h *BindingHandler) RequestScopes(c *gin.Context) {
	sessionKey := getSessionKeyFromHeader(c)
	if sessionKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Session-Key header required"})
		return
	}

	bindingID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid binding ID"})
		return
	}

	var req ScopeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	binding, err := h.bindingSvc.RequestScopes(c.Request.Context(), bindingID, sessionKey, req.Scopes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"binding": binding})
}

// ApproveScopes handles POST /bindings/:id/scopes/approve
// @Summary Approve pending scope requests
// @Tags bindings
// @Accept json
// @Produce json
// @Param X-Session-Key header string true "Session Key"
// @Param id path int true "Binding ID"
// @Param request body ScopeRequest true "Scopes to approve"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} ErrorResponse
// @Router /bindings/{id}/scopes/approve [post]
func (h *BindingHandler) ApproveScopes(c *gin.Context) {
	sessionKey := getSessionKeyFromHeader(c)
	if sessionKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Session-Key header required"})
		return
	}

	bindingID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid binding ID"})
		return
	}

	var req ScopeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	binding, err := h.bindingSvc.ApproveScopes(c.Request.Context(), bindingID, sessionKey, req.Scopes)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"binding": binding})
}

// Unbind handles POST /bindings/unbind
// @Summary Remove binding between sessions
// @Tags bindings
// @Accept json
// @Produce json
// @Param X-Session-Key header string true "Session Key"
// @Param request body UnbindRequest true "Unbind request"
// @Success 204 "No Content"
// @Failure 404 {object} ErrorResponse
// @Router /bindings/unbind [post]
func (h *BindingHandler) Unbind(c *gin.Context) {
	sessionKey := getSessionKeyFromHeader(c)
	if sessionKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Session-Key header required"})
		return
	}

	var req UnbindRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	removed, err := h.bindingSvc.Unbind(c.Request.Context(), sessionKey, req.TargetSession)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !removed {
		c.JSON(http.StatusNotFound, gin.H{"error": "no active binding found"})
		return
	}

	c.Status(http.StatusNoContent)
}

// ListBindings handles GET /bindings
// @Summary List all bindings for this session
// @Tags bindings
// @Accept json
// @Produce json
// @Param X-Session-Key header string true "Session Key"
// @Param status query string false "Filter by status"
// @Success 200 {object} map[string]interface{}
// @Router /bindings [get]
func (h *BindingHandler) ListBindings(c *gin.Context) {
	sessionKey := getSessionKeyFromHeader(c)
	if sessionKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Session-Key header required"})
		return
	}

	var statusFilter *string
	if status := c.Query("status"); status != "" {
		statusFilter = &status
	}

	bindings, err := h.bindingSvc.GetBindingsForSession(c.Request.Context(), sessionKey, statusFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"bindings": bindings,
		"total":    len(bindings),
	})
}

// GetPendingBindings handles GET /bindings/pending
// @Summary Get pending binding requests for this session
// @Tags bindings
// @Accept json
// @Produce json
// @Param X-Session-Key header string true "Session Key"
// @Success 200 {object} map[string]interface{}
// @Router /bindings/pending [get]
func (h *BindingHandler) GetPendingBindings(c *gin.Context) {
	sessionKey := getSessionKeyFromHeader(c)
	if sessionKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Session-Key header required"})
		return
	}

	pending, err := h.bindingSvc.GetPendingRequests(c.Request.Context(), sessionKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pending": pending,
		"count":   len(pending),
	})
}

// GetBoundSessions handles GET /bindings/sessions
// @Summary Get list of sessions this session is bound to
// @Tags bindings
// @Accept json
// @Produce json
// @Param X-Session-Key header string true "Session Key"
// @Success 200 {object} map[string]interface{}
// @Router /bindings/sessions [get]
func (h *BindingHandler) GetBoundSessions(c *gin.Context) {
	sessionKey := getSessionKeyFromHeader(c)
	if sessionKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Session-Key header required"})
		return
	}

	sessions, err := h.bindingSvc.GetBoundSessions(c.Request.Context(), sessionKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sessions": sessions,
		"count":    len(sessions),
	})
}

// CheckBinding handles GET /bindings/check/:target_session
// @Summary Check if bound to a specific session
// @Tags bindings
// @Accept json
// @Produce json
// @Param X-Session-Key header string true "Session Key"
// @Param target_session path string true "Target Session Key"
// @Success 200 {object} map[string]interface{}
// @Router /bindings/check/{target_session} [get]
func (h *BindingHandler) CheckBinding(c *gin.Context) {
	sessionKey := getSessionKeyFromHeader(c)
	if sessionKey == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "X-Session-Key header required"})
		return
	}

	targetSession := c.Param("target_session")
	if targetSession == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "target_session is required"})
		return
	}

	isBound, err := h.bindingSvc.IsBound(c.Request.Context(), sessionKey, targetSession)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	response := gin.H{
		"is_bound": isBound,
		"binding":  nil,
	}

	if isBound {
		// Try to get the binding details
		binding, err := h.bindingSvc.GetActiveBinding(c.Request.Context(), sessionKey, targetSession)
		if err != nil {
			binding, _ = h.bindingSvc.GetActiveBinding(c.Request.Context(), targetSession, sessionKey)
		}
		if binding != nil {
			response["binding"] = binding
		}
	}

	c.JSON(http.StatusOK, response)
}
