package v1

import (
	"errors"
	"log"
	"net/http"

	"github.com/google/uuid"

	podDomain "github.com/anthropics/agentsmesh/backend/internal/domain/agentpod"
	"github.com/anthropics/agentsmesh/backend/internal/middleware"
	"github.com/anthropics/agentsmesh/backend/internal/service/agentpod"
	"github.com/gin-gonic/gin"
)

// CreatePodRequest represents pod creation request
type CreatePodRequest struct {
	RunnerID          int64   `json:"runner_id"`           // Required for new pods, optional when resuming (inherited from source)
	AgentTypeID       *int64  `json:"agent_type_id"`       // Required unless resuming (then inherited from source pod)
	CustomAgentTypeID *int64  `json:"custom_agent_type_id"`
	RepositoryID      *int64  `json:"repository_id"`
	RepositoryURL     *string `json:"repository_url"`    // Direct repository URL (takes precedence over repository_id)
	TicketID          *int64  `json:"ticket_id"`
	TicketIdentifier  *string `json:"ticket_identifier"` // Direct ticket identifier (takes precedence over ticket_id)
	InitialPrompt     string  `json:"initial_prompt"`
	BranchName        *string `json:"branch_name"`
	PermissionMode    *string `json:"permission_mode"` // "plan", "default", or "bypassPermissions"

	// CredentialProfileID specifies which credential profile to use
	// - nil or 0: RunnerHost mode (use Runner's local environment, no credentials injected)
	// - >0: Use specified credential profile ID
	CredentialProfileID *int64 `json:"credential_profile_id"`

	// ConfigOverrides allows users to override agent type default configuration
	ConfigOverrides map[string]interface{} `json:"config_overrides"`

	// Terminal size (from browser xterm.js)
	Cols int32 `json:"cols"` // Terminal columns (width)
	Rows int32 `json:"rows"` // Terminal rows (height)

	// Resume related fields
	SourcePodKey       string `json:"source_pod_key"`       // Pod key to resume from (enables resume mode)
	ResumeAgentSession *bool  `json:"resume_agent_session"` // Whether to restore agent session (default: true when resuming)
}

// CreatePod creates a new pod
// POST /api/v1/organizations/:slug/pods
// Supports Resume mode when source_pod_key is provided
func (h *PodHandler) CreatePod(c *gin.Context) {
	var req CreatePodRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tenant := middleware.GetTenant(c)
	ctx := c.Request.Context()

	// Handle Resume mode
	var sourcePod *podDomain.Pod
	var sessionID string
	isResumeMode := req.SourcePodKey != ""

	if isResumeMode {
		var err error
		sourcePod, sessionID, err = h.handleResumeMode(c, &req, tenant)
		if err != nil {
			return // Error response already sent
		}
	} else {
		sessionID, err := h.handleNormalMode(c, &req)
		if err != nil {
			return // Error response already sent
		}
		_ = sessionID // Used below
	}

	// Handle session ID for new pods
	if !isResumeMode {
		sessionID = generateSessionID()
	}

	// Add session_id to config overrides for NEW sessions only
	if req.ConfigOverrides == nil {
		req.ConfigOverrides = make(map[string]interface{})
	}
	if !isResumeMode {
		req.ConfigOverrides["session_id"] = sessionID
	}

	// Check concurrent pod quota before creation
	if err := h.checkQuota(c, tenant); err != nil {
		return // Error response already sent
	}

	// Create pod record in database
	pod, err := h.podService.CreatePod(ctx, &agentpod.CreatePodRequest{
		OrganizationID:    tenant.OrganizationID,
		RunnerID:          req.RunnerID,
		AgentTypeID:       req.AgentTypeID,
		CustomAgentTypeID: req.CustomAgentTypeID,
		RepositoryID:      req.RepositoryID,
		TicketID:          req.TicketID,
		CreatedByID:       tenant.UserID,
		InitialPrompt:     req.InitialPrompt,
		BranchName:        req.BranchName,
		SessionID:         sessionID,
		SourcePodKey:      req.SourcePodKey,
	})
	if err != nil {
		if errors.Is(err, ErrSandboxAlreadyResumed) {
			c.JSON(http.StatusConflict, gin.H{
				"error": "Sandbox has already been resumed by another active pod",
				"code":  "SANDBOX_ALREADY_RESUMED",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create pod"})
		return
	}

	// Send create_pod command to runner via PodCoordinator
	if h.podCoordinator != nil {
		if err := h.sendCreatePodCommand(c, &req, pod, sourcePod, isResumeMode); err != nil {
			return // Response already sent (either error or warning)
		}
	} else {
		log.Printf("[pods] PodCoordinator is nil, cannot send create_pod command")
	}

	c.JSON(http.StatusCreated, gin.H{"pod": pod})
}

// handleResumeMode handles the resume mode logic
func (h *PodHandler) handleResumeMode(c *gin.Context, req *CreatePodRequest, tenant *middleware.TenantContext) (*podDomain.Pod, string, error) {
	ctx := c.Request.Context()

	sourcePod, err := h.podService.GetPod(ctx, req.SourcePodKey)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Source pod not found for resume",
			"code":  "SOURCE_POD_NOT_FOUND",
		})
		return nil, "", err
	}

	// Verify source pod belongs to same organization
	if sourcePod.OrganizationID != tenant.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Source pod belongs to different organization",
			"code":  "SOURCE_POD_ACCESS_DENIED",
		})
		return nil, "", errors.New("access denied")
	}

	// Verify source pod is terminated
	if sourcePod.Status != podDomain.StatusTerminated &&
		sourcePod.Status != podDomain.StatusCompleted &&
		sourcePod.Status != podDomain.StatusOrphaned {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Can only resume from terminated, completed, or orphaned pods",
			"code":  "SOURCE_POD_NOT_TERMINATED",
		})
		return nil, "", errors.New("invalid source pod status")
	}

	// Check if source pod has already been resumed
	existingResumePod, err := h.podService.GetActivePodBySourcePodKey(ctx, req.SourcePodKey)
	if err == nil && existingResumePod != nil {
		c.JSON(http.StatusConflict, gin.H{
			"error": "Source pod has already been resumed by another active pod",
			"code":  "SOURCE_POD_ALREADY_RESUMED",
			"details": gin.H{
				"existing_pod_key": existingResumePod.PodKey,
			},
		})
		return nil, "", errors.New("already resumed")
	}

	// Inherit runner_id from source pod if not provided
	if req.RunnerID == 0 {
		req.RunnerID = sourcePod.RunnerID
	} else if sourcePod.RunnerID != req.RunnerID {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Resume requires same runner as source pod (Sandbox is local to runner)",
			"code":  "RESUME_RUNNER_MISMATCH",
		})
		return nil, "", errors.New("runner mismatch")
	}

	// Inherit configuration from source pod
	if req.AgentTypeID == nil {
		req.AgentTypeID = sourcePod.AgentTypeID
	}
	if req.CustomAgentTypeID == nil {
		req.CustomAgentTypeID = sourcePod.CustomAgentTypeID
	}
	if req.RepositoryID == nil {
		req.RepositoryID = sourcePod.RepositoryID
	}
	if req.TicketID == nil {
		req.TicketID = sourcePod.TicketID
	}
	if req.BranchName == nil {
		req.BranchName = sourcePod.BranchName
	}

	// Reuse session ID from source pod
	var sessionID string
	if sourcePod.SessionID != nil && *sourcePod.SessionID != "" {
		sessionID = *sourcePod.SessionID
	} else {
		sessionID = generateSessionID()
	}

	// Set resume configuration
	resumeAgentSession := req.ResumeAgentSession == nil || *req.ResumeAgentSession
	if resumeAgentSession {
		if req.ConfigOverrides == nil {
			req.ConfigOverrides = make(map[string]interface{})
		}
		req.ConfigOverrides["resume_enabled"] = true
		req.ConfigOverrides["resume_session"] = sessionID
	}

	return sourcePod, sessionID, nil
}

// handleNormalMode handles the normal (non-resume) mode logic
func (h *PodHandler) handleNormalMode(c *gin.Context, req *CreatePodRequest) (string, error) {
	if req.RunnerID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "runner_id is required for new pods",
			"code":  "MISSING_RUNNER_ID",
		})
		return "", errors.New("missing runner_id")
	}
	if req.AgentTypeID == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "agent_type_id is required for new pods",
			"code":  "MISSING_AGENT_TYPE_ID",
		})
		return "", errors.New("missing agent_type_id")
	}
	return generateSessionID(), nil
}

// checkQuota checks the concurrent pod quota
func (h *PodHandler) checkQuota(c *gin.Context, tenant *middleware.TenantContext) error {
	if h.billingService == nil {
		return nil
	}

	if err := h.billingService.CheckQuota(c.Request.Context(), tenant.OrganizationID, "concurrent_pods", 1); err != nil {
		if err == ErrQuotaExceeded {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error": "Concurrent pod quota exceeded. Please upgrade your plan or terminate existing pods.",
				"code":  "CONCURRENT_POD_QUOTA_EXCEEDED",
			})
			return err
		}
		if err == ErrSubscriptionFrozen {
			c.JSON(http.StatusPaymentRequired, gin.H{
				"error": "Your subscription has expired. Please renew to continue.",
				"code":  "SUBSCRIPTION_FROZEN",
			})
			return err
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to check quota"})
		return err
	}
	return nil
}

// sendCreatePodCommand sends the create_pod command to the runner
func (h *PodHandler) sendCreatePodCommand(c *gin.Context, req *CreatePodRequest, pod *podDomain.Pod, sourcePod *podDomain.Pod, isResumeMode bool) error {
	ctx := c.Request.Context()

	// Get permission mode
	permissionMode := "plan"
	if req.PermissionMode != nil {
		permissionMode = *req.PermissionMode
	} else if pod.PermissionMode != nil {
		permissionMode = *pod.PermissionMode
	}

	// For resume mode, set local_path to source pod's sandbox path
	if isResumeMode && sourcePod != nil && sourcePod.SandboxPath != nil {
		if req.ConfigOverrides == nil {
			req.ConfigOverrides = make(map[string]interface{})
		}
		req.ConfigOverrides["sandbox_local_path"] = *sourcePod.SandboxPath
	}

	// Build pod command
	podCmd, err := h.buildPodCommand(c, req, pod.PodKey, permissionMode)
	if err != nil {
		log.Printf("[pods] Failed to build pod command: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to build pod configuration: " + err.Error(),
			"code":  "POD_CONFIG_BUILD_FAILED",
		})
		return err
	}

	log.Printf("[pods] Sending create_pod to runner %d for pod %s (resume=%v)", req.RunnerID, pod.PodKey, isResumeMode)

	if err := h.podCoordinator.CreatePod(ctx, req.RunnerID, podCmd); err != nil {
		log.Printf("[pods] Failed to send create_pod: %v", err)
		c.JSON(http.StatusCreated, gin.H{
			"pod":     pod,
			"warning": "Pod created but runner communication failed: " + err.Error(),
		})
		return err
	}
	log.Printf("[pods] create_pod sent successfully for pod %s", pod.PodKey)
	return nil
}

// generateSessionID generates a new UUID v4 for agent session
func generateSessionID() string {
	return uuid.New().String()
}
