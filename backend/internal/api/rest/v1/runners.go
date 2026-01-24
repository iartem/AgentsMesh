package v1

import (
	"net/http"
	"strconv"

	"github.com/anthropics/agentsmesh/backend/internal/middleware"
	"github.com/anthropics/agentsmesh/backend/internal/service/agentpod"
	runner "github.com/anthropics/agentsmesh/backend/internal/service/runner"
	"github.com/gin-gonic/gin"
)

// RunnerHandler handles runner-related requests
type RunnerHandler struct {
	runnerService       *runner.Service
	podService          *agentpod.PodService
	sandboxQueryService *runner.SandboxQueryService
	sandboxQuerySender  runner.SandboxQuerySender
}

// NewRunnerHandler creates a new runner handler
func NewRunnerHandler(runnerService *runner.Service, opts ...RunnerHandlerOption) *RunnerHandler {
	h := &RunnerHandler{
		runnerService: runnerService,
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// RunnerHandlerOption is a functional option for configuring RunnerHandler
type RunnerHandlerOption func(*RunnerHandler)

// WithPodServiceForRunner sets the pod service for runner handler
func WithPodServiceForRunner(ps *agentpod.PodService) RunnerHandlerOption {
	return func(h *RunnerHandler) {
		h.podService = ps
	}
}

// WithSandboxQueryService sets the sandbox query service for runner handler
func WithSandboxQueryService(sqs *runner.SandboxQueryService) RunnerHandlerOption {
	return func(h *RunnerHandler) {
		h.sandboxQueryService = sqs
	}
}

// WithSandboxQuerySender sets the sandbox query sender for runner handler
func WithSandboxQuerySender(sqs runner.SandboxQuerySender) RunnerHandlerOption {
	return func(h *RunnerHandler) {
		h.sandboxQuerySender = sqs
	}
}

// ListRunners lists runners in organization
// GET /api/v1/organizations/:slug/runners
func (h *RunnerHandler) ListRunners(c *gin.Context) {
	tenant := middleware.GetTenant(c)

	runners, err := h.runnerService.ListRunners(c.Request.Context(), tenant.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list runners"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"runners": runners})
}

// GetRunner returns runner by ID
// GET /api/v1/organizations/:slug/runners/:id
func (h *RunnerHandler) GetRunner(c *gin.Context) {
	runnerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid runner ID"})
		return
	}

	r, err := h.runnerService.GetRunner(c.Request.Context(), runnerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Runner not found"})
		return
	}

	tenant := middleware.GetTenant(c)
	if r.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"runner": r})
}

// DeleteRunner deletes a runner
// DELETE /api/v1/organizations/:slug/runners/:id
func (h *RunnerHandler) DeleteRunner(c *gin.Context) {
	runnerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid runner ID"})
		return
	}

	tenant := middleware.GetTenant(c)

	// Check admin permission
	if tenant.UserRole != "owner" && tenant.UserRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "Admin permission required"})
		return
	}

	r, err := h.runnerService.GetRunner(c.Request.Context(), runnerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Runner not found"})
		return
	}

	if r.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	if err := h.runnerService.DeleteRunner(c.Request.Context(), runnerID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete runner"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Runner deleted"})
}

// UpdateRunnerRequest represents runner update request
type UpdateRunnerRequest struct {
	Description       *string `json:"description"`
	MaxConcurrentPods *int    `json:"max_concurrent_pods"`
	IsEnabled         *bool   `json:"is_enabled"`
}

// UpdateRunner updates a runner
// PUT /api/v1/organizations/:slug/runners/:id
func (h *RunnerHandler) UpdateRunner(c *gin.Context) {
	runnerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid runner ID"})
		return
	}

	var req UpdateRunnerRequest
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

	// Verify runner belongs to organization
	r, err := h.runnerService.GetRunner(c.Request.Context(), runnerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Runner not found"})
		return
	}

	if r.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Update runner
	updated, err := h.runnerService.UpdateRunner(c.Request.Context(), runnerID, runner.RunnerUpdateInput{
		Description:       req.Description,
		MaxConcurrentPods: req.MaxConcurrentPods,
		IsEnabled:         req.IsEnabled,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update runner"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"runner": updated})
}


// ListAvailableRunners lists available runners for pods
// GET /api/v1/organizations/:slug/runners/available
func (h *RunnerHandler) ListAvailableRunners(c *gin.Context) {
	tenant := middleware.GetTenant(c)

	runners, err := h.runnerService.ListAvailableRunners(c.Request.Context(), tenant.OrganizationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list runners"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"runners": runners})
}

// ListRunnerPodsRequest represents request for listing runner pods
type ListRunnerPodsRequest struct {
	Status string `form:"status"`
	Limit  int    `form:"limit"`
	Offset int    `form:"offset"`
}

// ListRunnerPods lists pods for a specific runner
// GET /api/v1/organizations/:slug/runners/:id/pods
func (h *RunnerHandler) ListRunnerPods(c *gin.Context) {
	if h.podService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Pod service not configured"})
		return
	}

	runnerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid runner ID"})
		return
	}

	var req ListRunnerPodsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	// Verify runner belongs to organization
	r, err := h.runnerService.GetRunner(c.Request.Context(), runnerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Runner not found"})
		return
	}

	if r.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Default limit
	limit := req.Limit
	if limit == 0 {
		limit = 50
	}

	pods, total, err := h.podService.ListPodsByRunner(c.Request.Context(), runnerID, req.Status, limit, req.Offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list pods"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"pods":   pods,
		"total":  total,
		"limit":  limit,
		"offset": req.Offset,
	})
}

// QuerySandboxesRequest represents request for querying sandbox status
type QuerySandboxesRequest struct {
	PodKeys []string `json:"pod_keys" binding:"required,min=1,max=100"`
}

// QuerySandboxes queries sandbox status for specified pod keys on a runner
// POST /api/v1/organizations/:slug/runners/:id/sandboxes/query
func (h *RunnerHandler) QuerySandboxes(c *gin.Context) {
	if h.sandboxQueryService == nil || h.sandboxQuerySender == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Sandbox query service not configured"})
		return
	}

	runnerID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid runner ID"})
		return
	}

	var req QuerySandboxesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)

	// Verify runner belongs to organization
	r, err := h.runnerService.GetRunner(c.Request.Context(), runnerID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Runner not found"})
		return
	}

	if r.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "Access denied"})
		return
	}

	// Check if runner is connected
	if !h.sandboxQuerySender.IsConnected(runnerID) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Runner is not connected"})
		return
	}

	// Query sandboxes
	result, err := h.sandboxQueryService.QuerySandboxes(
		c.Request.Context(),
		runnerID,
		req.PodKeys,
		h.sandboxQuerySender.SendQuerySandboxes,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if result.Error != "" {
		c.JSON(http.StatusOK, gin.H{
			"error":     result.Error,
			"sandboxes": result.Sandboxes,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"sandboxes": result.Sandboxes,
	})
}
