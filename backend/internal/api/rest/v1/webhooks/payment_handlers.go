package webhooks

import (
	"encoding/json"
	"io"
	"net/http"

	billingdomain "github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment"
	"github.com/gin-gonic/gin"
)

// ===========================================
// Payment Webhook Handlers
// ===========================================

// handleStripeWebhook handles Stripe webhook events
func (r *WebhookRouter) handleStripeWebhook(c *gin.Context) {
	// Check if Stripe is configured
	if r.paymentFactory == nil || !r.paymentFactory.IsProviderAvailable(billingdomain.PaymentProviderStripe) {
		r.logger.Warn("Stripe webhook received but Stripe is not configured")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Stripe not configured"})
		return
	}

	// Read the request body
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		r.logger.Error("failed to read Stripe webhook body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	// Get the Stripe signature header
	signature := c.GetHeader("Stripe-Signature")
	if signature == "" {
		r.logger.Warn("missing Stripe-Signature header")
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing signature"})
		return
	}

	// Get the Stripe provider
	provider, err := r.paymentFactory.GetProvider(billingdomain.PaymentProviderStripe)
	if err != nil {
		r.logger.Error("failed to get Stripe provider", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "provider not available"})
		return
	}

	// Parse and validate the webhook
	event, err := provider.HandleWebhook(c.Request.Context(), payload, signature)
	if err != nil {
		r.logger.Error("failed to validate Stripe webhook", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid webhook signature"})
		return
	}

	r.logger.Info("received Stripe webhook",
		"event_id", event.EventID,
		"event_type", event.EventType,
	)

	// Process the event based on type
	var processErr error
	switch event.EventType {
	case billingdomain.WebhookEventCheckoutCompleted:
		event.Status = billingdomain.OrderStatusSucceeded
		processErr = r.billingSvc.HandlePaymentSucceeded(c, event)

	case billingdomain.WebhookEventInvoicePaid:
		event.Status = billingdomain.OrderStatusSucceeded
		processErr = r.billingSvc.HandlePaymentSucceeded(c, event)

	case billingdomain.WebhookEventInvoiceFailed:
		event.Status = billingdomain.OrderStatusFailed
		processErr = r.billingSvc.HandlePaymentFailed(c, event)

	case billingdomain.WebhookEventSubscriptionDeleted:
		event.Status = billingdomain.SubscriptionStatusCanceled
		processErr = r.billingSvc.HandleSubscriptionCanceled(c, event)

	case billingdomain.WebhookEventSubscriptionUpdated:
		processErr = r.billingSvc.HandleSubscriptionUpdated(c, event)

	default:
		r.logger.Debug("ignoring unhandled Stripe event type", "event_type", event.EventType)
	}

	if processErr != nil {
		r.logger.Error("failed to process Stripe webhook",
			"error", processErr,
			"event_type", event.EventType,
			"event_id", event.EventID,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process event"})
		return
	}

	// Acknowledge receipt
	c.JSON(http.StatusOK, gin.H{"received": true})
}

// handleAlipayWebhook handles Alipay webhook events (异步通知)
func (r *WebhookRouter) handleAlipayWebhook(c *gin.Context) {
	// Check if Alipay is configured
	if r.paymentFactory == nil || !r.paymentFactory.IsProviderAvailable(billingdomain.PaymentProviderAlipay) {
		r.logger.Warn("Alipay webhook received but Alipay is not configured")
		c.String(http.StatusServiceUnavailable, "fail")
		return
	}

	// Read the form data (Alipay uses application/x-www-form-urlencoded)
	if err := c.Request.ParseForm(); err != nil {
		r.logger.Error("failed to parse Alipay webhook form", "error", err)
		c.String(http.StatusBadRequest, "fail")
		return
	}

	// Convert form values to JSON for our provider
	formData := make(map[string]string)
	for key, values := range c.Request.Form {
		if len(values) > 0 {
			formData[key] = values[0]
		}
	}
	payload, _ := json.Marshal(formData)

	// Get the Alipay provider
	provider, err := r.paymentFactory.GetProvider(billingdomain.PaymentProviderAlipay)
	if err != nil {
		r.logger.Error("failed to get Alipay provider", "error", err)
		c.String(http.StatusInternalServerError, "fail")
		return
	}

	// Parse and validate the webhook (signature is verified inside)
	event, err := provider.HandleWebhook(c.Request.Context(), payload, "")
	if err != nil {
		r.logger.Error("failed to validate Alipay webhook", "error", err)
		c.String(http.StatusBadRequest, "fail")
		return
	}

	r.logger.Info("received Alipay webhook",
		"event_id", event.EventID,
		"event_type", event.EventType,
		"order_no", event.OrderNo,
		"status", event.Status,
	)

	// Process the event based on status
	var processErr error
	switch event.Status {
	case billingdomain.OrderStatusSucceeded:
		processErr = r.billingSvc.HandlePaymentSucceeded(c, event)

	case billingdomain.OrderStatusFailed:
		processErr = r.billingSvc.HandlePaymentFailed(c, event)

	case billingdomain.OrderStatusCanceled:
		// Trade was closed
		r.logger.Info("Alipay trade closed", "order_no", event.OrderNo)

	default:
		r.logger.Debug("ignoring Alipay event with pending status", "status", event.Status)
	}

	if processErr != nil {
		r.logger.Error("failed to process Alipay webhook",
			"error", processErr,
			"order_no", event.OrderNo,
			"status", event.Status,
		)
		c.String(http.StatusInternalServerError, "fail")
		return
	}

	// Alipay expects "success" string response (not JSON)
	c.String(http.StatusOK, "success")
}

// handleWeChatWebhook handles WeChat Pay webhook events (支付回调)
func (r *WebhookRouter) handleWeChatWebhook(c *gin.Context) {
	// Check if WeChat is configured
	if r.paymentFactory == nil || !r.paymentFactory.IsProviderAvailable(billingdomain.PaymentProviderWeChat) {
		r.logger.Warn("WeChat webhook received but WeChat is not configured")
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    "FAIL",
			"message": "WeChat not configured",
		})
		return
	}

	// Read the request body
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		r.logger.Error("failed to read WeChat webhook body", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "FAIL",
			"message": "failed to read request body",
		})
		return
	}

	// Get signature headers for verification
	// WeChat uses: Wechatpay-Signature, Wechatpay-Timestamp, Wechatpay-Nonce, Wechatpay-Serial
	signature := c.GetHeader("Wechatpay-Signature")
	timestamp := c.GetHeader("Wechatpay-Timestamp")
	nonce := c.GetHeader("Wechatpay-Nonce")

	// Build verification string for provider
	verifyStr := timestamp + "|" + nonce + "|" + signature

	// Get the WeChat provider
	provider, err := r.paymentFactory.GetProvider(billingdomain.PaymentProviderWeChat)
	if err != nil {
		r.logger.Error("failed to get WeChat provider", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "FAIL",
			"message": "provider not available",
		})
		return
	}

	// Parse and validate the webhook
	event, err := provider.HandleWebhook(c.Request.Context(), payload, verifyStr)
	if err != nil {
		r.logger.Error("failed to validate WeChat webhook", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    "FAIL",
			"message": "invalid webhook",
		})
		return
	}

	r.logger.Info("received WeChat webhook",
		"event_id", event.EventID,
		"event_type", event.EventType,
		"order_no", event.OrderNo,
		"status", event.Status,
	)

	// Process the event based on status
	var processErr error
	switch event.Status {
	case billingdomain.OrderStatusSucceeded:
		processErr = r.billingSvc.HandlePaymentSucceeded(c, event)

	case billingdomain.OrderStatusFailed:
		processErr = r.billingSvc.HandlePaymentFailed(c, event)

	case billingdomain.OrderStatusCanceled:
		// Order was closed
		r.logger.Info("WeChat order closed", "order_no", event.OrderNo)

	default:
		r.logger.Debug("ignoring WeChat event with pending status", "status", event.Status)
	}

	if processErr != nil {
		r.logger.Error("failed to process WeChat webhook",
			"error", processErr,
			"order_no", event.OrderNo,
			"status", event.Status,
		)
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    "FAIL",
			"message": "failed to process event",
		})
		return
	}

	// WeChat expects specific JSON response format
	c.JSON(http.StatusOK, gin.H{
		"code":    "SUCCESS",
		"message": "",
	})
}

// ===========================================
// Mock Payment Handlers (for testing)
// ===========================================

// MockCheckoutCompleteRequest represents a request to complete a mock checkout
type MockCheckoutCompleteRequest struct {
	SessionID string `json:"session_id" binding:"required"`
	OrderNo   string `json:"order_no"`
}

// handleMockCheckoutComplete handles mock checkout completion
func (r *WebhookRouter) handleMockCheckoutComplete(c *gin.Context) {
	// Check if mock is enabled
	if r.paymentFactory == nil || !r.paymentFactory.IsMockEnabled() {
		r.logger.Warn("mock checkout complete requested but mock is not enabled")
		c.JSON(http.StatusForbidden, gin.H{"error": "mock payment not enabled"})
		return
	}

	var req MockCheckoutCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get the mock provider
	mockProvider := r.paymentFactory.GetMockProvider()
	if mockProvider == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mock provider not available"})
		return
	}

	// Complete the session
	session, err := mockProvider.CompleteSession(req.SessionID)
	if err != nil {
		r.logger.Error("failed to complete mock session", "error", err, "session_id", req.SessionID)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	r.logger.Info("mock checkout completed",
		"session_id", req.SessionID,
		"order_no", req.OrderNo,
	)

	// Create a webhook event to process
	event := &payment.WebhookEvent{
		EventID:         "mock_evt_" + req.SessionID,
		EventType:       billingdomain.WebhookEventCheckoutCompleted,
		Provider:        "mock",
		OrderNo:         req.OrderNo,
		ExternalOrderNo: req.SessionID,
		CustomerID:      session.CustomerID,
		SubscriptionID:  session.SubscriptionID,
		Amount:          session.Request.ActualAmount,
		Currency:        session.Request.Currency,
		Status:          billingdomain.OrderStatusSucceeded,
	}

	// Process the payment success
	if err := r.billingSvc.HandlePaymentSucceeded(c, event); err != nil {
		r.logger.Error("failed to process mock payment",
			"error", err,
			"session_id", req.SessionID,
		)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process payment"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"session_id":      req.SessionID,
		"order_no":        req.OrderNo,
		"customer_id":     session.CustomerID,
		"subscription_id": session.SubscriptionID,
	})
}

// getMockSession retrieves mock session information
func (r *WebhookRouter) getMockSession(c *gin.Context) {
	// Check if mock is enabled
	if r.paymentFactory == nil || !r.paymentFactory.IsMockEnabled() {
		c.JSON(http.StatusForbidden, gin.H{"error": "mock payment not enabled"})
		return
	}

	sessionID := c.Param("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	// Get the mock provider
	mockProvider := r.paymentFactory.GetMockProvider()
	if mockProvider == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "mock provider not available"})
		return
	}

	// Get session info
	session, err := mockProvider.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id":      session.ID,
		"status":          session.Status,
		"created_at":      session.CreatedAt,
		"expires_at":      session.ExpiresAt,
		"completed_at":    session.CompletedAt,
		"customer_id":     session.CustomerID,
		"subscription_id": session.SubscriptionID,
		"order_type":      session.Request.OrderType,
		"amount":          session.Request.ActualAmount,
		"currency":        session.Request.Currency,
	})
}
