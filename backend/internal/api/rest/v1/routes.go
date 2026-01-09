package v1

import (
	"github.com/anthropics/agentmesh/backend/internal/config"
	"github.com/anthropics/agentmesh/backend/internal/infra/email"
	"github.com/anthropics/agentmesh/backend/internal/infra/websocket"
	"github.com/anthropics/agentmesh/backend/internal/service/agent"
	"github.com/anthropics/agentmesh/backend/internal/service/auth"
	"github.com/anthropics/agentmesh/backend/internal/service/billing"
	"github.com/anthropics/agentmesh/backend/internal/service/binding"
	"github.com/anthropics/agentmesh/backend/internal/service/channel"
	"github.com/anthropics/agentmesh/backend/internal/service/devmesh"
	"github.com/anthropics/agentmesh/backend/internal/service/devpod"
	"github.com/anthropics/agentmesh/backend/internal/service/gitprovider"
	"github.com/anthropics/agentmesh/backend/internal/service/invitation"
	"github.com/anthropics/agentmesh/backend/internal/service/organization"
	"github.com/anthropics/agentmesh/backend/internal/service/repository"
	"github.com/anthropics/agentmesh/backend/internal/service/runner"
	"github.com/anthropics/agentmesh/backend/internal/service/session"
	"github.com/anthropics/agentmesh/backend/internal/service/sshkey"
	"github.com/anthropics/agentmesh/backend/internal/service/ticket"
	"github.com/anthropics/agentmesh/backend/internal/service/user"
	"github.com/gin-gonic/gin"
)

// MessageService is a type alias for agent.MessageService
type MessageService = agent.MessageService

// Services holds all service dependencies for API handlers
type Services struct {
	Auth               *auth.Service
	User               *user.Service
	Org                *organization.Service
	Agent              *agent.Service
	GitProvider        *gitprovider.Service
	Repository         *repository.Service
	Runner             *runner.Service
	RunnerConnMgr      *runner.ConnectionManager   // Runner WebSocket connection manager
	SessionCoordinator *runner.SessionCoordinator  // Session lifecycle coordinator
	TerminalRouter     *runner.TerminalRouter      // Terminal data router
	Session            *session.Service
	Channel            *channel.Service
	Binding            *binding.Service
	Ticket             *ticket.Service
	DevMesh            *devmesh.Service
	DevPodSettings     *devpod.SettingsService    // DevPod user settings
	DevPodAIProvider   *devpod.AIProviderService  // DevPod AI provider management
	Billing            *billing.Service
	Message            *MessageService      // Agent-to-agent messaging
	Hub                *websocket.Hub       // WebSocket hub for real-time communication
	SSHKey             *sshkey.Service      // SSH key management
	Email              email.Service        // Email service
	Invitation         *invitation.Service  // Organization invitations
}

// RegisterAllRoutes registers all API v1 routes with proper handlers
func RegisterAllRoutes(rg *gin.RouterGroup, cfg *config.Config, svc *Services) {
	// Auth routes (public)
	RegisterAuthRoutes(rg.Group("/auth"), cfg, svc.Auth, svc.User, svc.Email)

	// User routes (authenticated, but not org-scoped)
	RegisterUserRoutes(rg.Group("/users"), svc.User, svc.Org, svc.Agent, svc.DevPodSettings, svc.DevPodAIProvider)

	// Organization routes (authenticated, some require org context)
	RegisterOrganizationRoutes(rg.Group("/organizations"), svc.Org)
}

// RegisterOrgScopedRoutes registers organization-scoped routes (require tenant context)
func RegisterOrgScopedRoutes(rg *gin.RouterGroup, svc *Services) {
	// Agents
	agentHandler := NewAgentHandler(svc.Agent)
	agents := rg.Group("/agents")
	{
		agents.GET("/types", agentHandler.ListAgentTypes)
		agents.GET("/config", agentHandler.GetOrganizationAgentConfig)
		agents.POST("/config", agentHandler.EnableAgent)
		agents.DELETE("/config/:agent_type_id", agentHandler.DisableAgent)
		agents.PUT("/config/:agent_type_id/credentials", agentHandler.SetOrganizationCredentials)
		agents.POST("/custom", agentHandler.CreateCustomAgent)
		agents.PUT("/custom/:id", agentHandler.UpdateCustomAgent)
		agents.DELETE("/custom/:id", agentHandler.DeleteCustomAgent)
	}

	// Git Providers
	gitProviderHandler := NewGitProviderHandler(svc.GitProvider)
	gitProviders := rg.Group("/git-providers")
	{
		gitProviders.GET("", gitProviderHandler.ListGitProviders)
		gitProviders.POST("", gitProviderHandler.CreateGitProvider)
		gitProviders.GET("/:id", gitProviderHandler.GetGitProvider)
		gitProviders.PUT("/:id", gitProviderHandler.UpdateGitProvider)
		gitProviders.DELETE("/:id", gitProviderHandler.DeleteGitProvider)
		gitProviders.POST("/:id/test", gitProviderHandler.TestConnection)
		gitProviders.POST("/:id/sync", gitProviderHandler.SyncProjects)
	}

	// SSH Keys
	if svc.SSHKey != nil {
		sshKeyHandler := NewSSHKeyHandler(svc.SSHKey)
		sshKeys := rg.Group("/ssh-keys")
		{
			sshKeys.GET("", sshKeyHandler.ListSSHKeys)
			sshKeys.POST("", sshKeyHandler.CreateSSHKey)
			sshKeys.GET("/:id", sshKeyHandler.GetSSHKey)
			sshKeys.PUT("/:id", sshKeyHandler.UpdateSSHKey)
			sshKeys.DELETE("/:id", sshKeyHandler.DeleteSSHKey)
		}
	}

	// Repositories
	repositoryHandler := NewRepositoryHandler(svc.Repository)
	repositories := rg.Group("/repositories")
	{
		repositories.GET("", repositoryHandler.ListRepositories)
		repositories.POST("", repositoryHandler.CreateRepository)
		repositories.GET("/:id", repositoryHandler.GetRepository)
		repositories.PUT("/:id", repositoryHandler.UpdateRepository)
		repositories.DELETE("/:id", repositoryHandler.DeleteRepository)
		repositories.GET("/:id/branches", repositoryHandler.ListBranches)
		repositories.POST("/:id/sync-branches", repositoryHandler.SyncBranches)
		repositories.POST("/:id/webhook", repositoryHandler.SetupWebhook)
	}

	// Runners
	runnerHandler := NewRunnerHandler(svc.Runner)
	runners := rg.Group("/runners")
	{
		runners.GET("", runnerHandler.ListRunners)
		runners.GET("/available", runnerHandler.ListAvailableRunners)
		runners.GET("/tokens", runnerHandler.ListRegistrationTokens)
		runners.POST("/tokens", runnerHandler.CreateRegistrationToken)
		runners.DELETE("/tokens/:id", runnerHandler.RevokeRegistrationToken)
		runners.GET("/:id", runnerHandler.GetRunner)
		runners.PUT("/:id", runnerHandler.UpdateRunner)
		runners.DELETE("/:id", runnerHandler.DeleteRunner)
		runners.POST("/:id/regenerate-token", runnerHandler.RegenerateAuthToken)
	}

	// Sessions
	sessionHandler := NewSessionHandler(svc.Session, svc.Runner, svc.Agent)
	// Inject session coordinator and terminal router
	if svc.SessionCoordinator != nil {
		sessionHandler.SetSessionCoordinator(svc.SessionCoordinator)
	}
	if svc.TerminalRouter != nil {
		sessionHandler.SetTerminalRouter(svc.TerminalRouter)
	}
	sessions := rg.Group("/sessions")
	{
		sessions.GET("", sessionHandler.ListSessions)
		sessions.POST("", sessionHandler.CreateSession)
		sessions.GET("/:key", sessionHandler.GetSession)
		sessions.POST("/:key/terminate", sessionHandler.TerminateSession)
		sessions.GET("/:key/connect", sessionHandler.GetConnectionInfo)
		sessions.POST("/:key/send-prompt", sessionHandler.SendPrompt)
		// Terminal control endpoints
		sessions.GET("/:key/terminal/observe", sessionHandler.ObserveTerminal)
		sessions.POST("/:key/terminal/input", sessionHandler.SendTerminalInput)
		sessions.POST("/:key/terminal/resize", sessionHandler.ResizeTerminal)
	}

	// Channels
	channelHandler := NewChannelHandler(svc.Channel)
	channels := rg.Group("/channels")
	{
		channels.GET("", channelHandler.ListChannels)
		channels.POST("", channelHandler.CreateChannel)
		channels.GET("/:id", channelHandler.GetChannel)
		channels.PUT("/:id", channelHandler.UpdateChannel)
		channels.POST("/:id/archive", channelHandler.ArchiveChannel)
		channels.POST("/:id/unarchive", channelHandler.UnarchiveChannel)
		channels.GET("/:id/messages", channelHandler.ListMessages)
		channels.POST("/:id/messages", channelHandler.SendMessage)
		channels.GET("/:id/sessions", channelHandler.ListChannelSessions)
		channels.POST("/:id/sessions", channelHandler.JoinSession)
		channels.DELETE("/:id/sessions/:session_key", channelHandler.LeaveSession)
	}

	// Tickets
	ticketHandler := NewTicketHandler(svc.Ticket)
	devmeshHandler := NewDevMeshHandler(svc.DevMesh, svc.Ticket)
	tickets := rg.Group("/tickets")
	{
		tickets.GET("", ticketHandler.ListTickets)
		tickets.POST("", ticketHandler.CreateTicket)
		tickets.GET("/active", ticketHandler.GetActiveTickets)         // New: active tickets
		tickets.GET("/board", ticketHandler.GetBoard)                  // New: kanban board
		tickets.POST("/batch-sessions", devmeshHandler.BatchGetTicketSessions) // New: batch sessions
		tickets.GET("/:identifier", ticketHandler.GetTicket)
		tickets.PUT("/:identifier", ticketHandler.UpdateTicket)
		tickets.DELETE("/:identifier", ticketHandler.DeleteTicket)
		tickets.PATCH("/:identifier/status", ticketHandler.UpdateTicketStatus)
		tickets.POST("/:identifier/assignees", ticketHandler.AddAssignee)
		tickets.DELETE("/:identifier/assignees/:user_id", ticketHandler.RemoveAssignee)
		tickets.POST("/:identifier/labels", ticketHandler.AddLabel)
		tickets.DELETE("/:identifier/labels/:label_id", ticketHandler.RemoveLabel)
		tickets.GET("/:identifier/merge-requests", ticketHandler.ListMergeRequests)
		tickets.GET("/:identifier/sub-tickets", ticketHandler.GetSubTickets)   // New: sub-tickets
		tickets.GET("/:identifier/relations", ticketHandler.ListRelations)     // New: relations
		tickets.POST("/:identifier/relations", ticketHandler.CreateRelation)   // New: create relation
		tickets.DELETE("/:identifier/relations/:relation_id", ticketHandler.DeleteRelation) // New: delete relation
		tickets.GET("/:identifier/commits", ticketHandler.ListCommits)         // New: commits
		tickets.POST("/:identifier/commits", ticketHandler.LinkCommit)         // New: link commit
		tickets.DELETE("/:identifier/commits/:commit_id", ticketHandler.UnlinkCommit) // New: unlink commit
		tickets.GET("/:identifier/sessions", devmeshHandler.GetTicketSessions) // New: ticket sessions
		tickets.POST("/:identifier/sessions", devmeshHandler.CreateSessionForTicket) // New: create session for ticket
	}

	// Labels (organization-level)
	labels := rg.Group("/labels")
	{
		labels.GET("", ticketHandler.ListLabels)
		labels.POST("", ticketHandler.CreateLabel)
		labels.PUT("/:id", ticketHandler.UpdateLabel)
		labels.DELETE("/:id", ticketHandler.DeleteLabel)
	}

	// Billing
	RegisterBillingHandlers(rg.Group("/billing"), svc.Billing)

	// DevMesh (topology visualization)
	devmesh := rg.Group("/devmesh")
	{
		devmesh.GET("/topology", devmeshHandler.GetTopology)
	}

	// Bindings (session collaboration)
	bindingHandler := NewBindingHandler(svc.Binding)
	bindings := rg.Group("/bindings")
	{
		bindings.POST("", bindingHandler.RequestBinding)
		bindings.GET("", bindingHandler.ListBindings)
		bindings.POST("/accept", bindingHandler.AcceptBinding)
		bindings.POST("/reject", bindingHandler.RejectBinding)
		bindings.POST("/unbind", bindingHandler.Unbind)
		bindings.GET("/pending", bindingHandler.GetPendingBindings)
		bindings.GET("/sessions", bindingHandler.GetBoundSessions)
		bindings.GET("/check/:target_session", bindingHandler.CheckBinding)
		bindings.POST("/:id/scopes", bindingHandler.RequestScopes)
		bindings.POST("/:id/scopes/approve", bindingHandler.ApproveScopes)
	}

	// Messages (agent-to-agent communication)
	if svc.Message != nil {
		messageHandler := NewMessageHandler(svc.Message)
		messages := rg.Group("/messages")
		{
			messages.POST("", messageHandler.SendMessage)
			messages.GET("", messageHandler.GetMessages)
			messages.GET("/unread-count", messageHandler.GetUnreadCount)
			messages.GET("/sent", messageHandler.GetSentMessages)
			messages.POST("/mark-read", messageHandler.MarkRead)
			messages.POST("/mark-all-read", messageHandler.MarkAllRead)
			messages.GET("/conversation/:correlation_id", messageHandler.GetConversation)
			messages.GET("/dlq", messageHandler.GetDeadLetters)
			messages.POST("/dlq/:id/replay", messageHandler.ReplayDeadLetter)
			messages.GET("/:id", messageHandler.GetMessage)
		}
	}

	// Invitations (organization-scoped)
	if svc.Invitation != nil {
		invitationHandler := NewInvitationHandler(svc.Invitation, svc.Org, svc.User)
		invitationHandler.RegisterOrgRoutes(rg)
	}
}


// RegisterUserRoutes registers user routes
func RegisterUserRoutes(rg *gin.RouterGroup, userSvc *user.Service, orgSvc *organization.Service, agentSvc *agent.Service, devpodSettingsSvc *devpod.SettingsService, devpodAIProviderSvc *devpod.AIProviderService) {
	userHandler := NewUserHandler(userSvc, orgSvc)
	agentHandler := NewAgentHandler(agentSvc)

	// Profile routes
	rg.GET("/me", userHandler.GetCurrentUser)
	rg.PUT("/me", userHandler.UpdateCurrentUser)
	rg.POST("/me/password", userHandler.ChangePassword)
	rg.GET("/me/organizations", userHandler.ListUserOrganizations)
	rg.GET("/me/identities", userHandler.ListIdentities)
	rg.DELETE("/me/identities/:provider", userHandler.DeleteIdentity)

	// User agent credentials (not org-scoped)
	rg.GET("/me/agents/credentials", agentHandler.GetUserCredentials)
	rg.PUT("/me/agents/credentials/:agent_type_id", agentHandler.SetUserCredentials)
	rg.DELETE("/me/agents/credentials/:agent_type_id", agentHandler.DeleteUserCredentials)

	// DevPod settings routes
	if devpodSettingsSvc != nil && devpodAIProviderSvc != nil {
		devpodHandler := NewDevPodHandler(devpodSettingsSvc, devpodAIProviderSvc)
		devpodGroup := rg.Group("/me/devpod")
		{
			// Settings
			devpodGroup.GET("/settings", devpodHandler.GetSettings)
			devpodGroup.PUT("/settings", devpodHandler.UpdateSettings)

			// AI Providers
			providers := devpodGroup.Group("/providers")
			{
				providers.GET("", devpodHandler.ListProviders)
				providers.POST("", devpodHandler.CreateProvider)
				providers.PUT("/:id", devpodHandler.UpdateProvider)
				providers.DELETE("/:id", devpodHandler.DeleteProvider)
				providers.POST("/:id/default", devpodHandler.SetDefaultProvider)
			}
		}
	}

	// User search
	rg.GET("/search", userHandler.SearchUsers)
}

// RegisterOrganizationRoutes registers organization routes
func RegisterOrganizationRoutes(rg *gin.RouterGroup, orgSvc *organization.Service) {
	handler := NewOrganizationHandler(orgSvc)

	// Organization CRUD
	rg.GET("", handler.ListOrganizations)
	rg.POST("", handler.CreateOrganization)
	rg.GET("/:slug", handler.GetOrganization)
	rg.PUT("/:slug", handler.UpdateOrganization)
	rg.DELETE("/:slug", handler.DeleteOrganization)

	// Member management
	rg.GET("/:slug/members", handler.ListMembers)
	rg.POST("/:slug/members", handler.InviteMember)
	rg.PUT("/:slug/members/:user_id", handler.UpdateMemberRole)
	rg.DELETE("/:slug/members/:user_id", handler.RemoveMember)
}
