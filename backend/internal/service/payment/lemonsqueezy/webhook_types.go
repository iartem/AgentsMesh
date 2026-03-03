package lemonsqueezy

import "time"

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
	SubscriptionID        int                        `json:"subscription_id,omitempty"`
	VariantID             int                        `json:"variant_id,omitempty"`
	ProductID             int                        `json:"product_id,omitempty"`
	OrderID               int                        `json:"order_id,omitempty"`
	Cancelled             bool                       `json:"cancelled,omitempty"`
	BillingAnchor         int                        `json:"billing_anchor,omitempty"` // Day of month for billing
	FirstSubscriptionItem *WebhookSubscriptionItem   `json:"first_subscription_item,omitempty"`
	RenewsAt              *time.Time                 `json:"renews_at,omitempty"`
	EndsAt                *time.Time                 `json:"ends_at,omitempty"`
	CreatedAt             *time.Time                 `json:"created_at,omitempty"`
	UpdatedAt             *time.Time                 `json:"updated_at,omitempty"`
	PausedAt              *time.Time                 `json:"pause_starts_at,omitempty"`  // When subscription pause started
	ResumedAt             *time.Time                 `json:"pause_resumes_at,omitempty"` // When subscription will resume
}

// WebhookSubscriptionItem represents a subscription item in webhook payload
type WebhookSubscriptionItem struct {
	ID        int `json:"id"`
	Quantity  int `json:"quantity"`
	PriceID   int `json:"price_id"`
}
