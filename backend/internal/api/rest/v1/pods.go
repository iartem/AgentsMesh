package v1

import (
	"context"

	agentDomain "github.com/anthropics/agentsmesh/backend/internal/domain/agent"
	"github.com/anthropics/agentsmesh/backend/internal/service/agent"
	"github.com/anthropics/agentsmesh/backend/internal/service/agentpod"
	"github.com/anthropics/agentsmesh/backend/internal/service/runner"
)

// PodHandler handles pod-related requests
// Uses interfaces for service dependencies to enable dependency inversion and easier testing
type PodHandler struct {
	podService        PodServiceForHandler             // Pod CRUD operations
	runnerService     *runner.Service                  // Runner management (not abstracted - rarely mocked)
	agentService      AgentServiceForHandler           // Agent type and credentials
	billingService    BillingServiceForHandler         // Quota checking
	repositoryService RepositoryServiceForHandler      // Repository lookup
	ticketService     TicketServiceForHandler          // Ticket lookup
	userService       UserServiceForPod                // User credential retrieval (权限跟人走)
	runnerConnMgr     *runner.RunnerConnectionManager  // Runner gRPC connections (not abstracted)
	podCoordinator    *runner.PodCoordinator           // Pod coordination (not abstracted)
	terminalRouter    interface{}                      // *runner.TerminalRouter, optional
	configBuilder     *agent.ConfigBuilder             // New protocol: builds pod config from agent type templates
}

// PodHandlerOption is a functional option for configuring PodHandler
type PodHandlerOption func(*PodHandler)

// WithRunnerConnectionManager sets the runner connection manager
func WithRunnerConnectionManager(cm *runner.RunnerConnectionManager) PodHandlerOption {
	return func(h *PodHandler) {
		h.runnerConnMgr = cm
	}
}

// WithPodCoordinator sets the pod coordinator
func WithPodCoordinator(pc *runner.PodCoordinator) PodHandlerOption {
	return func(h *PodHandler) {
		h.podCoordinator = pc
	}
}

// WithTerminalRouter sets the terminal router
func WithTerminalRouter(tr interface{}) PodHandlerOption {
	return func(h *PodHandler) {
		h.terminalRouter = tr
	}
}

// WithRepositoryService sets the repository service
func WithRepositoryService(rs RepositoryServiceForHandler) PodHandlerOption {
	return func(h *PodHandler) {
		h.repositoryService = rs
	}
}

// WithTicketService sets the ticket service
func WithTicketService(ts TicketServiceForHandler) PodHandlerOption {
	return func(h *PodHandler) {
		h.ticketService = ts
	}
}

// WithUserService sets the user service for credential retrieval (权限跟人走)
func WithUserService(us UserServiceForPod) PodHandlerOption {
	return func(h *PodHandler) {
		h.userService = us
	}
}

// WithBillingService sets the billing service for quota checking
func WithBillingService(bs BillingServiceForHandler) PodHandlerOption {
	return func(h *PodHandler) {
		h.billingService = bs
	}
}

// WithPodService sets the pod service (for testing with mock implementations)
func WithPodService(ps PodServiceForHandler) PodHandlerOption {
	return func(h *PodHandler) {
		h.podService = ps
	}
}

// WithAgentService sets the agent service (for testing with mock implementations)
func WithAgentService(as AgentServiceForHandler) PodHandlerOption {
	return func(h *PodHandler) {
		h.agentService = as
	}
}

// compositeAgentProvider implements agent.AgentConfigProvider by combining three sub-services
// This allows PodHandler to work with the split service architecture
type compositeAgentProvider struct {
	agentTypeSvc  *agent.AgentTypeService
	credentialSvc *agent.CredentialProfileService
	userConfigSvc *agent.UserConfigService
}

func (p *compositeAgentProvider) GetAgentType(ctx context.Context, id int64) (*agentDomain.AgentType, error) {
	return p.agentTypeSvc.GetAgentType(ctx, id)
}

func (p *compositeAgentProvider) GetUserEffectiveConfig(ctx context.Context, userID, agentTypeID int64, overrides agentDomain.ConfigValues) agentDomain.ConfigValues {
	return p.userConfigSvc.GetUserEffectiveConfig(ctx, userID, agentTypeID, overrides)
}

func (p *compositeAgentProvider) GetEffectiveCredentialsForPod(ctx context.Context, userID, agentTypeID int64, profileID *int64) (agentDomain.EncryptedCredentials, bool, error) {
	return p.credentialSvc.GetEffectiveCredentialsForPod(ctx, userID, agentTypeID, profileID)
}

// NewPodHandler creates a new pod handler with required dependencies and optional configurations
func NewPodHandler(
	podService *agentpod.PodService,
	runnerService *runner.Service,
	agentTypeSvc *agent.AgentTypeService,
	credentialSvc *agent.CredentialProfileService,
	userConfigSvc *agent.UserConfigService,
	opts ...PodHandlerOption,
) *PodHandler {
	// Create composite provider for ConfigBuilder
	provider := &compositeAgentProvider{
		agentTypeSvc:  agentTypeSvc,
		credentialSvc: credentialSvc,
		userConfigSvc: userConfigSvc,
	}

	h := &PodHandler{
		podService:    podService,
		runnerService: runnerService,
		agentService:  provider,
		configBuilder: agent.NewConfigBuilder(provider),
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}
