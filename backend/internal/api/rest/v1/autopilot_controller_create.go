package v1

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/anthropics/agentsmesh/backend/internal/domain/agentpod"
	runnerv1 "github.com/anthropics/agentsmesh/proto/gen/go/runner/v1"
)

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
		OrganizationID:         orgID,
		AutopilotControllerKey: autopilotPodKey,
		PodKey:                 req.PodKey,
		PodID:                  targetPod.ID,
		RunnerID:               targetPod.RunnerID,
		InitialPrompt:          req.InitialPrompt,
		Phase:                  agentpod.AutopilotPhaseInitializing,
		MaxIterations:          maxIterations,
		IterationTimeoutSec:    iterationTimeout,
		NoProgressThreshold:    noProgressThreshold,
		SameErrorThreshold:     sameErrorThreshold,
		ApprovalTimeoutMin:     approvalTimeout,
		CircuitBreakerState:    agentpod.CircuitBreakerClosed,
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
