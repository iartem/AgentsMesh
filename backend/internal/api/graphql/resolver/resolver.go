package resolver

import (
	"github.com/anthropics/agentmesh/backend/internal/service/agent"
	"github.com/anthropics/agentmesh/backend/internal/service/channel"
	"github.com/anthropics/agentmesh/backend/internal/service/gitprovider"
	"github.com/anthropics/agentmesh/backend/internal/service/organization"
	"github.com/anthropics/agentmesh/backend/internal/service/repository"
	"github.com/anthropics/agentmesh/backend/internal/service/runner"
	"github.com/anthropics/agentmesh/backend/internal/service/session"
	"github.com/anthropics/agentmesh/backend/internal/service/ticket"
	"github.com/anthropics/agentmesh/backend/internal/service/user"
)

// Resolver is the root resolver
type Resolver struct {
	userService         *user.Service
	organizationService *organization.Service
	runnerService       *runner.Service
	sessionService      *session.Service
	agentService        *agent.Service
	repositoryService   *repository.Service
	ticketService       *ticket.Service
	channelService      *channel.Service
	gitProviderService  *gitprovider.Service
}

// NewResolver creates a new resolver with all dependencies
func NewResolver(
	userSvc *user.Service,
	orgSvc *organization.Service,
	runnerSvc *runner.Service,
	sessionSvc *session.Service,
	agentSvc *agent.Service,
	repoSvc *repository.Service,
	ticketSvc *ticket.Service,
	channelSvc *channel.Service,
	gitProviderSvc *gitprovider.Service,
) *Resolver {
	return &Resolver{
		userService:         userSvc,
		organizationService: orgSvc,
		runnerService:       runnerSvc,
		sessionService:      sessionSvc,
		agentService:        agentSvc,
		repositoryService:   repoSvc,
		ticketService:       ticketSvc,
		channelService:      channelSvc,
		gitProviderService:  gitProviderSvc,
	}
}
