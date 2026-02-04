package webhooks

import (
	"log/slog"

	"github.com/anthropics/agentsmesh/backend/internal/config"
	"github.com/anthropics/agentsmesh/backend/internal/service/billing"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// WebhookRouter handles webhook endpoint routing
type WebhookRouter struct {
	db             *gorm.DB
	cfg            *config.Config
	logger         *slog.Logger
	registry       *HandlerRegistry
	billingSvc     *billing.Service
	paymentFactory *payment.Factory
}

// NewWebhookRouter creates a new webhook router
func NewWebhookRouter(db *gorm.DB, cfg *config.Config, logger *slog.Logger) *WebhookRouter {
	registry := NewHandlerRegistry(logger)
	SetupDefaultHandlers(registry, logger)

	// Initialize billing service and payment factory for payment webhooks
	// Full config is passed for URL derivation (AlipayNotifyURL, etc.)
	billingSvc := billing.NewServiceWithConfig(db, cfg)
	paymentFactory := billingSvc.GetPaymentFactory()

	return &WebhookRouter{
		db:             db,
		cfg:            cfg,
		logger:         logger,
		registry:       registry,
		billingSvc:     billingSvc,
		paymentFactory: paymentFactory,
	}
}

// NewWebhookRouterWithBillingSvc creates a new webhook router with an external billing service
// This allows sharing the same payment factory instance across the application
func NewWebhookRouterWithBillingSvc(db *gorm.DB, cfg *config.Config, logger *slog.Logger, billingSvc *billing.Service) *WebhookRouter {
	registry := NewHandlerRegistry(logger)
	SetupDefaultHandlers(registry, logger)

	return &WebhookRouter{
		db:             db,
		cfg:            cfg,
		logger:         logger,
		registry:       registry,
		billingSvc:     billingSvc,
		paymentFactory: billingSvc.GetPaymentFactory(),
	}
}

// RegisterRoutes registers webhook routes to the router group
func (r *WebhookRouter) RegisterRoutes(rg *gin.RouterGroup) {
	// GitLab webhook endpoint
	rg.POST("/gitlab", r.handleGitLabWebhook)

	// GitHub webhook endpoint
	rg.POST("/github", r.handleGitHubWebhook)

	// Gitee webhook endpoint
	rg.POST("/gitee", r.handleGiteeWebhook)

	// Payment webhook endpoints
	rg.POST("/stripe", r.handleStripeWebhook)
	rg.POST("/lemonsqueezy", r.handleLemonSqueezyWebhook)
	rg.POST("/alipay", r.handleAlipayWebhook)
	rg.POST("/wechat", r.handleWeChatWebhook)

	// Mock payment endpoints (for testing)
	rg.POST("/mock/complete", r.handleMockCheckoutComplete)
	rg.GET("/mock/session/:session_id", r.getMockSession)
}
