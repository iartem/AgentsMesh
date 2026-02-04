package lemonsqueezy

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	lemonsqueezy "github.com/NdoleStudio/lemonsqueezy-go"

	"github.com/anthropics/agentsmesh/backend/internal/config"
	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment/types"
)

// Provider implements payment.SubscriptionProvider for LemonSqueezy
type Provider struct {
	client        *lemonsqueezy.Client
	storeID       string
	webhookSecret string
}

// NewProvider creates a new LemonSqueezy provider
func NewProvider(cfg *config.LemonSqueezyConfig) *Provider {
	client := lemonsqueezy.New(lemonsqueezy.WithAPIKey(cfg.APIKey))
	return &Provider{
		client:        client,
		storeID:       cfg.StoreID,
		webhookSecret: cfg.WebhookSecret,
	}
}

// GetProviderName returns the provider name
func (p *Provider) GetProviderName() string {
	return billing.PaymentProviderLemonSqueezy
}

// CreateCheckoutSession creates a LemonSqueezy Checkout session
func (p *Provider) CreateCheckoutSession(ctx context.Context, req *types.CheckoutRequest) (*types.CheckoutResponse, error) {
	// LemonSqueezy requires a variant ID - this should be passed via metadata
	variantID := ""
	if req.Metadata != nil {
		variantID = req.Metadata["variant_id"]
	}
	if variantID == "" {
		return nil, fmt.Errorf("variant_id is required in metadata for LemonSqueezy checkout")
	}

	// Build custom data for checkout
	customData := map[string]any{
		"organization_id": strconv.FormatInt(req.OrganizationID, 10),
		"user_id":         strconv.FormatInt(req.UserID, 10),
		"order_type":      req.OrderType,
		"billing_cycle":   req.BillingCycle,
		"seats":           strconv.Itoa(req.Seats),
	}
	if req.Metadata != nil {
		if orderNo, ok := req.Metadata["order_no"]; ok {
			customData["order_no"] = orderNo
		}
	}

	// Helper function for bool pointers
	boolPtr := func(b bool) *bool { return &b }

	// Prepare checkout attributes
	checkoutAttrs := &lemonsqueezy.CheckoutCreateAttributes{
		CheckoutData: lemonsqueezy.CheckoutCreateData{
			Email:  req.UserEmail,
			Custom: customData,
			VariantQuantities: []lemonsqueezy.CheckoutCreateDataQuantity{
				{
					VariantID: stringToInt(variantID),
					Quantity:  req.Seats,
				},
			},
		},
		CheckoutOptions: lemonsqueezy.CheckoutCreateOptions{
			Embed:               boolPtr(false),
			Media:               boolPtr(true),
			Logo:                boolPtr(true),
			Desc:                boolPtr(true),
			Discount:            boolPtr(true),
			Dark:                boolPtr(false),
			SubscriptionPreview: boolPtr(true),
		},
		ProductOptions: lemonsqueezy.CheckoutCreateProductOptions{
			EnabledVariants: []int{stringToInt(variantID)},
			RedirectURL:     req.SuccessURL,
		},
	}

	// Set expiration time (30 minutes from now)
	expiresAt := time.Now().Add(30 * time.Minute).Format(time.RFC3339)
	checkoutAttrs.ExpiresAt = &expiresAt

	// Create checkout
	storeID := stringToInt(p.storeID)
	variantIDInt := stringToInt(variantID)

	checkout, _, err := p.client.Checkouts.Create(ctx, storeID, variantIDInt, checkoutAttrs)
	if err != nil {
		return nil, fmt.Errorf("failed to create LemonSqueezy checkout: %w", err)
	}

	// Get expiration time
	expiresAtTime := time.Now().Add(30 * time.Minute)
	if checkout.Data.Attributes.ExpiresAt != nil {
		expiresAtTime = *checkout.Data.Attributes.ExpiresAt
	}

	return &types.CheckoutResponse{
		SessionID:       checkout.Data.ID,
		SessionURL:      checkout.Data.Attributes.URL,
		OrderNo:         req.IdempotencyKey,
		ExternalOrderNo: checkout.Data.ID,
		ExpiresAt:       expiresAtTime,
	}, nil
}

// GetCheckoutStatus checks the status of a checkout session
// LemonSqueezy doesn't have a direct checkout status API, so we return pending
func (p *Provider) GetCheckoutStatus(ctx context.Context, sessionID string) (string, error) {
	// LemonSqueezy checkouts are one-time use and don't have a status check API
	// The status is determined via webhooks
	return billing.OrderStatusPending, nil
}

// HandleWebhook parses and validates a LemonSqueezy webhook
func (p *Provider) HandleWebhook(ctx context.Context, payload []byte, signature string) (*types.WebhookEvent, error) {
	// Verify signature using HMAC-SHA256
	if err := p.verifySignature(payload, signature); err != nil {
		return nil, fmt.Errorf("webhook signature verification failed: %w", err)
	}

	// Parse the webhook payload
	var webhookPayload WebhookPayload
	if err := json.Unmarshal(payload, &webhookPayload); err != nil {
		return nil, fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	// Use Data.ID as the unique event identifier for idempotency
	// This ensures each webhook event has a truly unique ID
	eventID := webhookPayload.Data.ID
	if eventID == "" {
		// Fallback to event name + timestamp if ID is missing (shouldn't happen)
		eventID = webhookPayload.Meta.EventName + "_" + time.Now().Format("20060102150405.000000")
	}

	result := &types.WebhookEvent{
		EventID:   eventID,
		EventType: webhookPayload.Meta.EventName,
		Provider:  billing.PaymentProviderLemonSqueezy,
	}

	// Extract custom data for order_no
	if webhookPayload.Meta.CustomData != nil {
		if orderNo, ok := webhookPayload.Meta.CustomData["order_no"].(string); ok {
			result.OrderNo = orderNo
		}
	}

	// Parse event data based on type
	switch webhookPayload.Meta.EventName {
	case billing.WebhookEventLSOrderCreated:
		p.parseOrderEvent(&webhookPayload, result)

	case billing.WebhookEventLSSubscriptionCreated:
		p.parseSubscriptionCreatedEvent(&webhookPayload, result)

	case billing.WebhookEventLSSubscriptionUpdated:
		p.parseSubscriptionUpdatedEvent(&webhookPayload, result)

	case billing.WebhookEventLSSubscriptionCancelled:
		p.parseSubscriptionCancelledEvent(&webhookPayload, result)

	case billing.WebhookEventLSSubscriptionPaused:
		p.parseSubscriptionPausedEvent(&webhookPayload, result)

	case billing.WebhookEventLSSubscriptionResumed:
		p.parseSubscriptionResumedEvent(&webhookPayload, result)

	case billing.WebhookEventLSSubscriptionExpired:
		p.parseSubscriptionExpiredEvent(&webhookPayload, result)

	case billing.WebhookEventLSPaymentSuccess:
		p.parsePaymentSuccessEvent(&webhookPayload, result)

	case billing.WebhookEventLSPaymentFailed:
		p.parsePaymentFailedEvent(&webhookPayload, result)
	}

	// Store raw payload
	result.RawPayload = make(map[string]interface{})
	_ = json.Unmarshal(payload, &result.RawPayload)

	return result, nil
}

// RefundPayment initiates a refund
// Note: LemonSqueezy refunds are primarily handled through the dashboard
func (p *Provider) RefundPayment(ctx context.Context, req *types.RefundRequest) (*types.RefundResponse, error) {
	// LemonSqueezy doesn't have a public refund API
	// Refunds should be processed through the LemonSqueezy dashboard
	return nil, fmt.Errorf("refunds must be processed through the LemonSqueezy dashboard")
}

// CancelSubscription cancels a LemonSqueezy subscription
func (p *Provider) CancelSubscription(ctx context.Context, subscriptionID string, immediate bool) error {
	if immediate {
		// Cancel immediately
		_, _, err := p.client.Subscriptions.Cancel(ctx, subscriptionID)
		if err != nil {
			return fmt.Errorf("failed to cancel subscription: %w", err)
		}
	} else {
		// Cancel at period end (update subscription with cancelled=true)
		_, _, err := p.client.Subscriptions.Update(ctx, &lemonsqueezy.SubscriptionUpdateParams{
			ID: subscriptionID,
			Attributes: lemonsqueezy.SubscriptionUpdateParamsAttributes{
				Cancelled: true,
			},
		})
		if err != nil {
			return fmt.Errorf("failed to set cancel at period end: %w", err)
		}
	}
	return nil
}

// CreateCustomer creates a customer in LemonSqueezy
// Note: LemonSqueezy automatically creates customers during checkout
func (p *Provider) CreateCustomer(ctx context.Context, email string, name string, metadata map[string]string) (string, error) {
	// LemonSqueezy doesn't have a separate customer creation API
	// Customers are created automatically during the checkout process
	return "", nil
}

// GetCustomerPortalURL returns a URL for the customer to manage their billing
func (p *Provider) GetCustomerPortalURL(ctx context.Context, req *types.CustomerPortalRequest) (*types.CustomerPortalResponse, error) {
	// LemonSqueezy uses customer portal URLs from subscription data
	// The SubscriptionID is required to get the portal URL
	subscriptionID := req.SubscriptionID
	if subscriptionID == "" {
		// Fallback: some callers may pass subscription ID as CustomerID for backwards compatibility
		subscriptionID = req.CustomerID
	}
	if subscriptionID == "" {
		return nil, fmt.Errorf("subscription_id is required for LemonSqueezy customer portal")
	}

	// Get customer portal URL from the subscription
	sub, _, err := p.client.Subscriptions.Get(ctx, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	// Use the customer portal URL or update payment method URL
	portalURL := sub.Data.Attributes.Urls.CustomerPortal
	if portalURL == "" {
		portalURL = sub.Data.Attributes.Urls.UpdatePaymentMethod
	}

	return &types.CustomerPortalResponse{
		URL: portalURL,
	}, nil
}

// UpdateSubscriptionSeats updates the seat count for a subscription
// Note: LemonSqueezy uses subscription items for quantity management
func (p *Provider) UpdateSubscriptionSeats(ctx context.Context, subscriptionID string, seats int) error {
	// Get subscription to find the first subscription item
	sub, _, err := p.client.Subscriptions.Get(ctx, subscriptionID)
	if err != nil {
		return fmt.Errorf("failed to get subscription: %w", err)
	}

	// Get the first subscription item ID
	if sub.Data.Attributes.FirstSubscriptionItem == nil {
		return fmt.Errorf("subscription has no subscription items")
	}

	itemID := strconv.Itoa(sub.Data.Attributes.FirstSubscriptionItem.ID)

	// Update the subscription item quantity
	_, _, err = p.client.SubscriptionItems.Update(ctx, &lemonsqueezy.SubscriptionItemUpdateParams{
		ID: itemID,
		Attributes: lemonsqueezy.SubscriptionItemUpdateParamsAttributes{
			Quantity: seats,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to update subscription seats: %w", err)
	}

	return nil
}

// GetSubscription retrieves subscription details
func (p *Provider) GetSubscription(ctx context.Context, subscriptionID string) (*types.SubscriptionDetails, error) {
	sub, _, err := p.client.Subscriptions.Get(ctx, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get subscription: %w", err)
	}

	renewsAt := sub.Data.Attributes.RenewsAt
	result := &types.SubscriptionDetails{
		ID:                subscriptionID,
		CustomerID:        strconv.Itoa(sub.Data.Attributes.CustomerID),
		Status:            sub.Data.Attributes.Status,
		CurrentPeriodEnd:  renewsAt,
		CancelAtPeriodEnd: sub.Data.Attributes.Cancelled,
	}

	// Calculate CurrentPeriodStart based on billing interval
	// LemonSqueezy doesn't provide period_start directly, so we calculate it from RenewsAt
	if !renewsAt.IsZero() {
		billingInterval := sub.Data.Attributes.BillingAnchor
		// Default to monthly if billing anchor not specified
		if billingInterval == 0 {
			// Check variant interval from subscription item
			if sub.Data.Attributes.FirstSubscriptionItem != nil {
				// Assume monthly if we can't determine
				result.CurrentPeriodStart = renewsAt.AddDate(0, -1, 0)
			}
		} else if billingInterval == 12 {
			// Yearly
			result.CurrentPeriodStart = renewsAt.AddDate(-1, 0, 0)
		} else {
			// Monthly (most common)
			result.CurrentPeriodStart = renewsAt.AddDate(0, -1, 0)
		}
	} else {
		// Fallback to created_at if renews_at is zero value
		result.CurrentPeriodStart = sub.Data.Attributes.CreatedAt
	}

	// Get seats from first subscription item
	if sub.Data.Attributes.FirstSubscriptionItem != nil {
		result.Seats = sub.Data.Attributes.FirstSubscriptionItem.Quantity
	}

	return result, nil
}

// ===========================================
// Internal Helper Methods
// ===========================================

// ErrWebhookSecretNotConfigured indicates webhook secret is not configured
var ErrWebhookSecretNotConfigured = fmt.Errorf("webhook secret is not configured - this is required for security")

// verifySignature verifies the webhook signature using HMAC-SHA256
// SECURITY: Webhook secret MUST be configured for signature verification
func (p *Provider) verifySignature(payload []byte, signature string) error {
	if p.webhookSecret == "" {
		// SECURITY: Never skip signature verification
		// This prevents processing webhooks without proper authentication
		return ErrWebhookSecretNotConfigured
	}

	if signature == "" {
		return fmt.Errorf("missing signature header")
	}

	mac := hmac.New(sha256.New, []byte(p.webhookSecret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedMAC)) {
		return fmt.Errorf("invalid webhook signature")
	}

	return nil
}

// parseOrderEvent parses an order_created event
func (p *Provider) parseOrderEvent(payload *WebhookPayload, result *types.WebhookEvent) {
	result.ExternalOrderNo = payload.Data.ID
	result.Amount = float64(payload.Data.Attributes.Total) / 100.0 // Convert from cents to dollars
	result.Currency = payload.Data.Attributes.Currency
	result.Status = billing.OrderStatusSucceeded

	if payload.Data.Attributes.CustomerID != 0 {
		result.CustomerID = strconv.Itoa(payload.Data.Attributes.CustomerID)
	}
}

// parseSubscriptionCreatedEvent parses a subscription_created event
func (p *Provider) parseSubscriptionCreatedEvent(payload *WebhookPayload, result *types.WebhookEvent) {
	result.SubscriptionID = payload.Data.ID
	result.Status = billing.SubscriptionStatusActive

	if payload.Data.Attributes.CustomerID != 0 {
		result.CustomerID = strconv.Itoa(payload.Data.Attributes.CustomerID)
	}
}

// parseSubscriptionUpdatedEvent parses a subscription_updated event
func (p *Provider) parseSubscriptionUpdatedEvent(payload *WebhookPayload, result *types.WebhookEvent) {
	result.SubscriptionID = payload.Data.ID
	result.Status = payload.Data.Attributes.Status

	if payload.Data.Attributes.CustomerID != 0 {
		result.CustomerID = strconv.Itoa(payload.Data.Attributes.CustomerID)
	}
}

// parseSubscriptionCancelledEvent parses a subscription_cancelled event
func (p *Provider) parseSubscriptionCancelledEvent(payload *WebhookPayload, result *types.WebhookEvent) {
	result.SubscriptionID = payload.Data.ID
	result.Status = billing.SubscriptionStatusCanceled

	if payload.Data.Attributes.CustomerID != 0 {
		result.CustomerID = strconv.Itoa(payload.Data.Attributes.CustomerID)
	}
}

// parsePaymentSuccessEvent parses a subscription_payment_success event
func (p *Provider) parsePaymentSuccessEvent(payload *WebhookPayload, result *types.WebhookEvent) {
	result.ExternalOrderNo = payload.Data.ID
	result.Amount = float64(payload.Data.Attributes.Total) / 100.0 // Convert from cents to dollars
	result.Currency = payload.Data.Attributes.Currency
	result.Status = billing.OrderStatusSucceeded

	if payload.Data.Attributes.SubscriptionID != 0 {
		result.SubscriptionID = strconv.Itoa(payload.Data.Attributes.SubscriptionID)
	}
	if payload.Data.Attributes.CustomerID != 0 {
		result.CustomerID = strconv.Itoa(payload.Data.Attributes.CustomerID)
	}
}

// parsePaymentFailedEvent parses a subscription_payment_failed event
func (p *Provider) parsePaymentFailedEvent(payload *WebhookPayload, result *types.WebhookEvent) {
	result.ExternalOrderNo = payload.Data.ID
	result.Status = billing.OrderStatusFailed

	if payload.Data.Attributes.SubscriptionID != 0 {
		result.SubscriptionID = strconv.Itoa(payload.Data.Attributes.SubscriptionID)
	}
	if payload.Data.Attributes.CustomerID != 0 {
		result.CustomerID = strconv.Itoa(payload.Data.Attributes.CustomerID)
	}
}

// parseSubscriptionPausedEvent parses a subscription_paused event
func (p *Provider) parseSubscriptionPausedEvent(payload *WebhookPayload, result *types.WebhookEvent) {
	result.SubscriptionID = payload.Data.ID
	result.Status = billing.SubscriptionStatusPaused

	if payload.Data.Attributes.CustomerID != 0 {
		result.CustomerID = strconv.Itoa(payload.Data.Attributes.CustomerID)
	}
}

// parseSubscriptionResumedEvent parses a subscription_resumed event
func (p *Provider) parseSubscriptionResumedEvent(payload *WebhookPayload, result *types.WebhookEvent) {
	result.SubscriptionID = payload.Data.ID
	result.Status = billing.SubscriptionStatusActive

	if payload.Data.Attributes.CustomerID != 0 {
		result.CustomerID = strconv.Itoa(payload.Data.Attributes.CustomerID)
	}
}

// parseSubscriptionExpiredEvent parses a subscription_expired event
func (p *Provider) parseSubscriptionExpiredEvent(payload *WebhookPayload, result *types.WebhookEvent) {
	result.SubscriptionID = payload.Data.ID
	result.Status = billing.SubscriptionStatusExpired

	if payload.Data.Attributes.CustomerID != 0 {
		result.CustomerID = strconv.Itoa(payload.Data.Attributes.CustomerID)
	}
}

// stringToInt converts a string to int, returns 0 on error
func stringToInt(s string) int {
	i, _ := strconv.Atoi(s)
	return i
}

// ===========================================
// Webhook Payload Types
// ===========================================

// WebhookPayload represents the LemonSqueezy webhook payload structure
type WebhookPayload struct {
	Meta WebhookMeta `json:"meta"`
	Data WebhookData `json:"data"`
}

// WebhookMeta contains webhook metadata
type WebhookMeta struct {
	EventName  string                 `json:"event_name"`
	TestMode   bool                   `json:"test_mode"`
	CustomData map[string]interface{} `json:"custom_data,omitempty"`
}

// WebhookData contains the webhook event data
type WebhookData struct {
	ID         string                `json:"id"`
	Type       string                `json:"type"`
	Attributes WebhookDataAttributes `json:"attributes"`
}

// WebhookDataAttributes contains the event-specific attributes
type WebhookDataAttributes struct {
	// Common fields
	StoreID    int    `json:"store_id"`
	CustomerID int    `json:"customer_id"`
	Status     string `json:"status"`

	// Order/Payment fields (Total is in cents)
	Total        int64  `json:"total"`
	Currency     string `json:"currency"`
	CurrencyRate string `json:"currency_rate"`

	// Subscription fields
	SubscriptionID int        `json:"subscription_id,omitempty"`
	VariantID      int        `json:"variant_id,omitempty"`
	ProductID      int        `json:"product_id,omitempty"`
	OrderID        int        `json:"order_id,omitempty"`
	Cancelled      bool       `json:"cancelled,omitempty"`
	BillingAnchor  int        `json:"billing_anchor,omitempty"` // Day of month for billing
	RenewsAt       *time.Time `json:"renews_at,omitempty"`
	EndsAt         *time.Time `json:"ends_at,omitempty"`
	CreatedAt      *time.Time `json:"created_at,omitempty"`
	UpdatedAt      *time.Time `json:"updated_at,omitempty"`
	PausedAt       *time.Time `json:"pause_starts_at,omitempty"` // When subscription pause started
	ResumedAt      *time.Time `json:"pause_resumes_at,omitempty"` // When subscription will resume
}
