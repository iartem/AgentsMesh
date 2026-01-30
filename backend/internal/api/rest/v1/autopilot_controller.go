package v1

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/anthropics/agentsmesh/backend/internal/domain/agentpod"
	"github.com/anthropics/agentsmesh/backend/internal/middleware"
	runnerv1 "github.com/anthropics/agentsmesh/proto/gen/go/runner/v1"
)

// AutopilotControllerCommandSender defines the interface for sending AutopilotController commands to runners
type AutopilotControllerCommandSender interface {
	SendCreateAutopilot(runnerID int64, cmd *runnerv1.CreateAutopilotCommand) error
	SendAutopilotControl(runnerID int64, cmd *runnerv1.AutopilotControlCommand) error
}

// AutopilotControllerServiceInterface defines the interface for AutopilotController service operations
type AutopilotControllerServiceInterface interface {
	GetAutopilotController(orgID int64, autopilotPodKey string) (*agentpod.AutopilotController, error)
	ListAutopilotControllers(orgID int64) ([]*agentpod.AutopilotController, error)
	CreateAutopilotController(pod *agentpod.AutopilotController) error
	UpdateAutopilotController(pod *agentpod.AutopilotController) error
	GetIterations(autopilotPodID int64) ([]*agentpod.AutopilotIteration, error)
}

// AutopilotControllerHandler handles AutopilotController-related HTTP requests
type AutopilotControllerHandler struct {
	service       AutopilotControllerServiceInterface
	podService    PodServiceForHandler
	commandSender AutopilotControllerCommandSender
}

// AutopilotControllerHandlerOption is a functional option for AutopilotControllerHandler
type AutopilotControllerHandlerOption func(*AutopilotControllerHandler)

// WithAutopilotControllerService sets the AutopilotController service
func WithAutopilotControllerService(svc AutopilotControllerServiceInterface) AutopilotControllerHandlerOption {
	return func(h *AutopilotControllerHandler) {
		h.service = svc
	}
}

// WithAutopilotCommandSender sets the command sender
func WithAutopilotCommandSender(sender AutopilotControllerCommandSender) AutopilotControllerHandlerOption {
	return func(h *AutopilotControllerHandler) {
		h.commandSender = sender
	}
}

// WithPodServiceForAutopilot sets the pod service for worker pod lookup
func WithPodServiceForAutopilot(svc PodServiceForHandler) AutopilotControllerHandlerOption {
	return func(h *AutopilotControllerHandler) {
		h.podService = svc
	}
}

// NewAutopilotControllerHandler creates a new AutopilotControllerHandler
func NewAutopilotControllerHandler(opts ...AutopilotControllerHandlerOption) *AutopilotControllerHandler {
	h := &AutopilotControllerHandler{}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

// CreateAutopilotControllerRequest represents the request to create a AutopilotController
type CreateAutopilotControllerRequest struct {
	PodKey string `json:"pod_key" binding:"required"`

	// Task
	InitialPrompt string `json:"initial_prompt,omitempty"`

	// Configuration (all optional with defaults)
	MaxIterations         int32  `json:"max_iterations,omitempty"`
	IterationTimeoutSec   int32  `json:"iteration_timeout_sec,omitempty"`
	NoProgressThreshold   int32  `json:"no_progress_threshold,omitempty"`
	SameErrorThreshold    int32  `json:"same_error_threshold,omitempty"`
	ApprovalTimeoutMin    int32  `json:"approval_timeout_min,omitempty"`
	ControlAgentType      string `json:"control_agent_type,omitempty"`
	ControlPromptTemplate string `json:"control_prompt_template,omitempty"`
	MCPConfigJSON         string `json:"mcp_config_json,omitempty"`
}

// AutopilotControllerResponse represents the response for AutopilotController operations
type AutopilotControllerResponse struct {
	ID               int64  `json:"id"`
	AutopilotControllerKey      string `json:"autopilot_controller_key"`
	PodKey     string `json:"pod_key"`
	Phase            string `json:"phase"`
	CurrentIteration int32  `json:"current_iteration"`
	MaxIterations    int32  `json:"max_iterations"`
	CircuitBreaker   struct {
		State  string `json:"state"`
		Reason string `json:"reason,omitempty"`
	} `json:"circuit_breaker"`
	UserTakeover    bool       `json:"user_takeover"`
	InitialPrompt   string     `json:"initial_prompt,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	LastIterationAt *time.Time `json:"last_iteration_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

// toAutopilotControllerResponse converts domain model to API response
func toAutopilotControllerResponse(rp *agentpod.AutopilotController) *AutopilotControllerResponse {
	resp := &AutopilotControllerResponse{
		ID:               rp.ID,
		AutopilotControllerKey:      rp.AutopilotControllerKey,
		PodKey:     rp.PodKey,
		Phase:            rp.Phase,
		CurrentIteration: rp.CurrentIteration,
		MaxIterations:    rp.MaxIterations,
		UserTakeover:     rp.UserTakeover,
		InitialPrompt:    rp.InitialPrompt,
		StartedAt:        rp.StartedAt,
		LastIterationAt:  rp.LastIterationAt,
		CreatedAt:        rp.CreatedAt,
	}
	resp.CircuitBreaker.State = rp.CircuitBreakerState
	if rp.CircuitBreakerReason != nil {
		resp.CircuitBreaker.Reason = *rp.CircuitBreakerReason
	}
	return resp
}

// getOrgID extracts organization ID from context
func getOrgID(c *gin.Context) int64 {
	tenant := middleware.GetTenant(c)
	if tenant == nil {
		return 0
	}
	return tenant.OrganizationID
}

// CreateAutopilotController handles POST /autopilot-controllers
func (h *AutopilotControllerHandler) CreateAutopilotController(c *gin.Context) {
	orgID := getOrgID(c)
	if orgID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "organization context required"})
		return
	}

	var req CreateAutopilotControllerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the target pod to verify it exists and get runner info
	if h.podService == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "pod service not configured"})
		return
	}

	targetPod, err := h.podService.GetPod(c.Request.Context(), req.PodKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "target pod not found"})
		return
	}

	// Verify pod belongs to this organization
	if targetPod.OrganizationID != orgID {
		c.JSON(http.StatusForbidden, gin.H{"error": "target pod does not belong to this organization"})
		return
	}

	// Generate AutopilotController key with timestamp suffix to allow multiple AutopilotControllers
	// for the same Pod (e.g., sequential automation tasks)
	autopilotPodKey := fmt.Sprintf("autopilot-%s-%d", req.PodKey, time.Now().UnixNano())

	// Apply defaults
	maxIterations := req.MaxIterations
	if maxIterations == 0 {
		maxIterations = 10
	}
	iterationTimeout := req.IterationTimeoutSec
	if iterationTimeout == 0 {
		iterationTimeout = 300
	}
	noProgressThreshold := req.NoProgressThreshold
	if noProgressThreshold == 0 {
		noProgressThreshold = 3
	}
	sameErrorThreshold := req.SameErrorThreshold
	if sameErrorThreshold == 0 {
		sameErrorThreshold = 5
	}
	approvalTimeout := req.ApprovalTimeoutMin
	if approvalTimeout == 0 {
		approvalTimeout = 30
	}

	// Create domain model
	autopilotPod := &agentpod.AutopilotController{
		OrganizationID:      orgID,
		AutopilotControllerKey:         autopilotPodKey,
		PodKey:        req.PodKey,
		PodID:         targetPod.ID,
		RunnerID:            targetPod.RunnerID,
		InitialPrompt:       req.InitialPrompt,
		Phase:               agentpod.AutopilotPhaseInitializing,
		MaxIterations:       maxIterations,
		IterationTimeoutSec: iterationTimeout,
		NoProgressThreshold: noProgressThreshold,
		SameErrorThreshold:  sameErrorThreshold,
		ApprovalTimeoutMin:  approvalTimeout,
		CircuitBreakerState: agentpod.CircuitBreakerClosed,
	}

	if req.ControlAgentType != "" {
		autopilotPod.ControlAgentType = &req.ControlAgentType
	}
	if req.ControlPromptTemplate != "" {
		autopilotPod.ControlPromptTemplate = &req.ControlPromptTemplate
	}
	if req.MCPConfigJSON != "" {
		autopilotPod.MCPConfigJSON = &req.MCPConfigJSON
	}

	// Save to database
	if h.service != nil {
		if err := h.service.CreateAutopilotController(autopilotPod); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create autopilot pod"})
			return
		}
	}

	// Send command to runner
	if h.commandSender != nil {
		cmd := &runnerv1.CreateAutopilotCommand{
			AutopilotKey: autopilotPodKey,
			PodKey:       req.PodKey,
			Config: &runnerv1.AutopilotConfig{
				InitialPrompt:           req.InitialPrompt,
				MaxIterations:           maxIterations,
				IterationTimeoutSeconds: iterationTimeout,
				NoProgressThreshold:     noProgressThreshold,
				SameErrorThreshold:      sameErrorThreshold,
				ApprovalTimeoutMinutes:  approvalTimeout,
				ControlAgentType:        req.ControlAgentType,
				ControlPromptTemplate:   req.ControlPromptTemplate,
				McpConfigJson:           req.MCPConfigJSON,
			},
		}
		if err := h.commandSender.SendCreateAutopilot(targetPod.RunnerID, cmd); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send create command to runner"})
			return
		}
	}

	c.JSON(http.StatusCreated, toAutopilotControllerResponse(autopilotPod))
}

// GetAutopilotController handles GET /autopilot-controllers/:key
func (h *AutopilotControllerHandler) GetAutopilotController(c *gin.Context) {
	orgID := getOrgID(c)
	if orgID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "organization context required"})
		return
	}

	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "autopilot pod key required"})
		return
	}

	if h.service == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "service not configured"})
		return
	}

	autopilotPod, err := h.service.GetAutopilotController(orgID, key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "autopilot pod not found"})
		return
	}

	c.JSON(http.StatusOK, toAutopilotControllerResponse(autopilotPod))
}

// ListAutopilotControllers handles GET /autopilot-controllers
func (h *AutopilotControllerHandler) ListAutopilotControllers(c *gin.Context) {
	orgID := getOrgID(c)
	if orgID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "organization context required"})
		return
	}

	if h.service == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "service not configured"})
		return
	}

	pods, err := h.service.ListAutopilotControllers(orgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list autopilot pods"})
		return
	}

	resp := make([]*AutopilotControllerResponse, len(pods))
	for i, p := range pods {
		resp[i] = toAutopilotControllerResponse(p)
	}

	c.JSON(http.StatusOK, resp)
}

// AutopilotControlRequest represents control action request
type AutopilotControlRequest struct {
	Action               string `json:"action" binding:"required,oneof=pause resume stop approve takeover handback"`
	ContinueExecution    *bool  `json:"continue_execution,omitempty"`    // For approve action
	AdditionalIterations int32  `json:"additional_iterations,omitempty"` // For approve action
}

// sendAutopilotControl sends a control command to the runner
func (h *AutopilotControllerHandler) sendAutopilotControl(c *gin.Context, autopilotPod *agentpod.AutopilotController, action string, req *AutopilotControlRequest) {
	if h.commandSender == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "command sender not configured"})
		return
	}

	cmd := &runnerv1.AutopilotControlCommand{
		AutopilotKey: autopilotPod.AutopilotControllerKey,
	}

	switch action {
	case "pause":
		cmd.Action = &runnerv1.AutopilotControlCommand_Pause{Pause: &runnerv1.AutopilotPauseAction{}}
	case "resume":
		cmd.Action = &runnerv1.AutopilotControlCommand_Resume{Resume: &runnerv1.AutopilotResumeAction{}}
	case "stop":
		cmd.Action = &runnerv1.AutopilotControlCommand_Stop{Stop: &runnerv1.AutopilotStopAction{}}
	case "approve":
		continueExec := true
		if req != nil && req.ContinueExecution != nil {
			continueExec = *req.ContinueExecution
		}
		additionalIter := int32(0)
		if req != nil {
			additionalIter = req.AdditionalIterations
		}
		cmd.Action = &runnerv1.AutopilotControlCommand_Approve{
			Approve: &runnerv1.AutopilotApproveAction{
				ContinueExecution:    continueExec,
				AdditionalIterations: additionalIter,
			},
		}
	case "takeover":
		cmd.Action = &runnerv1.AutopilotControlCommand_Takeover{Takeover: &runnerv1.AutopilotTakeoverAction{}}
	case "handback":
		cmd.Action = &runnerv1.AutopilotControlCommand_Handback{Handback: &runnerv1.AutopilotHandbackAction{}}
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid action"})
		return
	}

	if err := h.commandSender.SendAutopilotControl(autopilotPod.RunnerID, cmd); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to send control command"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok", "action": action})
}

// PauseAutopilotController handles POST /autopilot-controllers/:key/pause
func (h *AutopilotControllerHandler) PauseAutopilotController(c *gin.Context) {
	autopilotPod := h.getAutopilotControllerFromContext(c)
	if autopilotPod == nil {
		return
	}
	h.sendAutopilotControl(c, autopilotPod, "pause", nil)
}

// ResumeAutopilotController handles POST /autopilot-controllers/:key/resume
func (h *AutopilotControllerHandler) ResumeAutopilotController(c *gin.Context) {
	autopilotPod := h.getAutopilotControllerFromContext(c)
	if autopilotPod == nil {
		return
	}
	h.sendAutopilotControl(c, autopilotPod, "resume", nil)
}

// StopAutopilotController handles POST /autopilot-controllers/:key/stop
func (h *AutopilotControllerHandler) StopAutopilotController(c *gin.Context) {
	autopilotPod := h.getAutopilotControllerFromContext(c)
	if autopilotPod == nil {
		return
	}
	h.sendAutopilotControl(c, autopilotPod, "stop", nil)
}

// ApproveAutopilotController handles POST /autopilot-controllers/:key/approve
func (h *AutopilotControllerHandler) ApproveAutopilotController(c *gin.Context) {
	autopilotPod := h.getAutopilotControllerFromContext(c)
	if autopilotPod == nil {
		return
	}

	var req AutopilotControlRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// Allow empty body for simple approval
		req.Action = "approve"
	}

	h.sendAutopilotControl(c, autopilotPod, "approve", &req)
}

// TakeoverAutopilotController handles POST /autopilot-controllers/:key/takeover
func (h *AutopilotControllerHandler) TakeoverAutopilotController(c *gin.Context) {
	autopilotPod := h.getAutopilotControllerFromContext(c)
	if autopilotPod == nil {
		return
	}
	h.sendAutopilotControl(c, autopilotPod, "takeover", nil)
}

// HandbackAutopilotController handles POST /autopilot-controllers/:key/handback
func (h *AutopilotControllerHandler) HandbackAutopilotController(c *gin.Context) {
	autopilotPod := h.getAutopilotControllerFromContext(c)
	if autopilotPod == nil {
		return
	}
	h.sendAutopilotControl(c, autopilotPod, "handback", nil)
}

// GetIterations handles GET /autopilot-controllers/:key/iterations
func (h *AutopilotControllerHandler) GetIterations(c *gin.Context) {
	autopilotPod := h.getAutopilotControllerFromContext(c)
	if autopilotPod == nil {
		return
	}

	if h.service == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "service not configured"})
		return
	}

	iterations, err := h.service.GetIterations(autopilotPod.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get iterations"})
		return
	}

	c.JSON(http.StatusOK, iterations)
}

// getAutopilotControllerFromContext is a helper to get AutopilotController from request context
func (h *AutopilotControllerHandler) getAutopilotControllerFromContext(c *gin.Context) *agentpod.AutopilotController {
	orgID := getOrgID(c)
	if orgID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "organization context required"})
		return nil
	}

	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "autopilot pod key required"})
		return nil
	}

	if h.service == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "service not configured"})
		return nil
	}

	autopilotPod, err := h.service.GetAutopilotController(orgID, key)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "autopilot pod not found"})
		return nil
	}

	return autopilotPod
}

// RegisterAutopilotControllerRoutes registers AutopilotController routes
func RegisterAutopilotControllerRoutes(rg *gin.RouterGroup, handler *AutopilotControllerHandler) {
	autopilotPods := rg.Group("/autopilot-controllers")
	{
		autopilotPods.GET("", handler.ListAutopilotControllers)
		autopilotPods.POST("", handler.CreateAutopilotController)
		autopilotPods.GET("/:key", handler.GetAutopilotController)
		autopilotPods.POST("/:key/pause", handler.PauseAutopilotController)
		autopilotPods.POST("/:key/resume", handler.ResumeAutopilotController)
		autopilotPods.POST("/:key/stop", handler.StopAutopilotController)
		autopilotPods.POST("/:key/approve", handler.ApproveAutopilotController)
		autopilotPods.POST("/:key/takeover", handler.TakeoverAutopilotController)
		autopilotPods.POST("/:key/handback", handler.HandbackAutopilotController)
		autopilotPods.GET("/:key/iterations", handler.GetIterations)
	}
}
