package rest

import (
	"log/slog"

	"github.com/anthropics/agentmesh/backend/internal/api/rest/v1"
	"github.com/anthropics/agentmesh/backend/internal/api/rest/v1/webhooks"
	"github.com/anthropics/agentmesh/backend/internal/api/rest/ws"
	"github.com/anthropics/agentmesh/backend/internal/config"
	"github.com/anthropics/agentmesh/backend/internal/infra/email"
	"github.com/anthropics/agentmesh/backend/internal/middleware"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// NewRouter creates and configures the Gin router
func NewRouter(cfg *config.Config, svc *v1.Services, db *gorm.DB, logger *slog.Logger) *gin.Engine {
	if !cfg.Server.Debug {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.Default()

	// CORS configuration
	corsConfig := cors.Config{
		AllowOrigins:     cfg.Server.CORSAllowedOrigins,
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Organization-Slug"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}
	if len(corsConfig.AllowOrigins) == 0 {
		corsConfig.AllowOrigins = []string{"*"}
	}
	r.Use(cors.New(corsConfig))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":  "ok",
			"service": "agentmesh-api",
		})
	})

	r.GET("/health/ready", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ready",
		})
	})

	// Initialize email service
	emailSvc := email.NewService(email.Config{
		Provider:    cfg.Email.Provider,
		ResendKey:   cfg.Email.ResendKey,
		FromAddress: cfg.Email.FromAddress,
		BaseURL:     cfg.Email.BaseURL,
	})

	// API v1
	apiV1 := r.Group("/api/v1")
	{
		// Public routes (no auth required)
		v1.RegisterAuthRoutes(apiV1.Group("/auth"), cfg, svc.Auth, svc.User, emailSvc)

		// Runner registration (uses token-based auth, not JWT)
		RegisterRunnerAuthRoutes(apiV1.Group("/runners"), svc, logger)

		// Webhook endpoints (no auth required, use token verification)
		webhookRouter := webhooks.NewWebhookRouter(db, cfg, logger)
		webhookRouter.RegisterRoutes(apiV1.Group("/webhooks"))

		// Public invitation routes (token-based access)
		if svc.Invitation != nil {
			invitationHandler := v1.NewInvitationHandler(svc.Invitation, svc.Org, svc.User)
			invitationHandler.RegisterRoutes(apiV1, middleware.AuthMiddleware(cfg.JWT.Secret))
		}

		// Protected routes (auth required)
		protected := apiV1.Group("")
		protected.Use(middleware.AuthMiddleware(cfg.JWT.Secret))
		{
			// User-level routes (no tenant context required)
			v1.RegisterUserRoutes(protected.Group("/users"), svc.User, svc.Org, svc.Agent, svc.DevPodSettings, svc.DevPodAIProvider)

			// Organization routes (authenticated, some require org context)
			v1.RegisterOrganizationRoutes(protected.Group("/organizations"), svc.Org)

			// Organization-scoped routes (require tenant context)
			orgScoped := protected.Group("/organizations/:slug")
			orgScoped.Use(middleware.TenantMiddleware(svc.Org))
			{
				v1.RegisterOrgScopedRoutes(orgScoped, svc)
			}

			// Alias route /api/v1/org/* that uses X-Organization-Slug header
			// This is for backward compatibility with frontend that uses /api/v1/org/
			orgAlias := protected.Group("/org")
			orgAlias.Use(middleware.TenantMiddleware(svc.Org))
			{
				v1.RegisterOrgScopedRoutes(orgAlias, svc)
			}
		}
	}

	// WebSocket endpoints
	wsGroup := r.Group("/ws")
	wsGroup.Use(middleware.AuthMiddleware(cfg.JWT.Secret))
	wsGroup.Use(middleware.TenantMiddleware(svc.Org))
	{
		terminalHandler := ws.NewTerminalHandler(svc.Hub, svc.Session)
		if svc.TerminalRouter != nil {
			terminalHandler.SetTerminalRouter(svc.TerminalRouter)
		}
		wsGroup.GET("/terminal/:session_key", terminalHandler.HandleTerminal)

		eventHandler := ws.NewEventsHandler(svc.Hub)
		wsGroup.GET("/events", eventHandler.HandleEvents)
	}

	return r
}

// RegisterRunnerAuthRoutes registers runner-specific authentication routes
func RegisterRunnerAuthRoutes(rg *gin.RouterGroup, svc *v1.Services, logger *slog.Logger) {
	runnerHandler := v1.NewRunnerHandler(svc.Runner)
	rg.POST("/register", runnerHandler.RegisterRunner)
	rg.POST("/heartbeat", runnerHandler.Heartbeat)

	// Runner WebSocket endpoint (no JWT auth, uses runner auth token)
	if svc.RunnerConnMgr != nil {
		runnerWsHandler := ws.NewRunnerHandler(svc.Runner, svc.RunnerConnMgr, logger)
		rg.GET("/ws", runnerWsHandler.HandleRunnerWS)
	}
}
