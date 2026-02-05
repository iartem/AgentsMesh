package internal

import (
	"log/slog"
	"net/http"

	"github.com/anthropics/agentsmesh/backend/internal/infra/acme"
	"github.com/anthropics/agentsmesh/backend/internal/service/agentpod"
	"github.com/anthropics/agentsmesh/backend/internal/service/relay"
	"github.com/anthropics/agentsmesh/backend/internal/service/runner"
	"github.com/gin-gonic/gin"
)

// RelayHandler handles internal relay API endpoints
type RelayHandler struct {
	relayManager  *relay.Manager
	dnsService    *relay.DNSService
	acmeManager   *acme.Manager
	commandSender runner.RunnerCommandSender
	podService    *agentpod.PodService
	logger        *slog.Logger
}

// NewRelayHandler creates a new relay handler
func NewRelayHandler(relayManager *relay.Manager, dnsService *relay.DNSService, acmeManager *acme.Manager, commandSender runner.RunnerCommandSender, podService *agentpod.PodService) *RelayHandler {
	return &RelayHandler{
		relayManager:  relayManager,
		dnsService:    dnsService,
		acmeManager:   acmeManager,
		commandSender: commandSender,
		podService:    podService,
		logger:        slog.With("component", "relay_handler"),
	}
}

// RelayRouterDeps holds dependencies for relay routes
type RelayRouterDeps struct {
	RelayManager   *relay.Manager
	DNSService     *relay.DNSService
	ACMEManager    *acme.Manager
	CommandSender  runner.RunnerCommandSender
	PodService     *agentpod.PodService
	InternalSecret string
}

// RegisterRelayRoutes registers relay API routes
func RegisterRelayRoutes(router *gin.RouterGroup, deps *RelayRouterDeps) {
	handler := NewRelayHandler(deps.RelayManager, deps.DNSService, deps.ACMEManager, deps.CommandSender, deps.PodService)

	// Internal API authentication middleware
	router.Use(InternalAPIAuth(deps.InternalSecret))

	router.POST("/register", handler.Register)
	router.POST("/heartbeat", handler.Heartbeat)
	router.POST("/unregister", handler.Unregister)
	router.POST("/session-closed", handler.SessionClosed)
	router.GET("/stats", handler.Stats)
	router.GET("", handler.List)
	router.GET("/:relay_id", handler.Get)
	router.DELETE("/:relay_id", handler.ForceUnregister)

	// Session management
	router.GET("/sessions", handler.ListSessions)
	router.POST("/sessions/migrate", handler.MigrateSession)
	router.POST("/sessions/bulk-migrate", handler.BulkMigrateSessions)
}

// InternalAPIAuth is middleware for internal API authentication
func InternalAPIAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("X-Internal-Secret")
		if auth != secret {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			c.Abort()
			return
		}
		c.Next()
	}
}
