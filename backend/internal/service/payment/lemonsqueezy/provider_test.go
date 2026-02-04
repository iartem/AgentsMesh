package lemonsqueezy

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/anthropics/agentsmesh/backend/internal/config"
	"github.com/anthropics/agentsmesh/backend/internal/domain/billing"
	"github.com/anthropics/agentsmesh/backend/internal/service/payment/types"
)

func TestNewProvider(t *testing.T) {
	cfg := &config.LemonSqueezyConfig{
		APIKey:        "test_api_key",
		StoreID:       "12345",
		WebhookSecret: "test_secret",
	}

	provider := NewProvider(cfg)
	if provider.GetProviderName() != billing.PaymentProviderLemonSqueezy {
		t.Errorf("expected provider name %s, got %s", billing.PaymentProviderLemonSqueezy, provider.GetProviderName())
	}
}

func TestVerifySignature(t *testing.T) {
	cfg := &config.LemonSqueezyConfig{
		APIKey:        "test_api_key",
		StoreID:       "12345",
		WebhookSecret: "test_webhook_secret",
	}

	provider := NewProvider(cfg)

	tests := []struct {
		name      string
		payload   string
		signature string
		wantErr   bool
	}{
		{
			name:      "valid signature",
			payload:   `{"meta":{"event_name":"order_created"}}`,
			signature: generateHMAC(`{"meta":{"event_name":"order_created"}}`, "test_webhook_secret"),
			wantErr:   false,
		},
		{
			name:      "invalid signature",
			payload:   `{"meta":{"event_name":"order_created"}}`,
			signature: "invalid_signature",
			wantErr:   true,
		},
		{
			name:      "empty signature",
			payload:   `{"meta":{"event_name":"order_created"}}`,
			signature: "",
			wantErr:   true,
		},
		{
			name:      "tampered payload",
			payload:   `{"meta":{"event_name":"order_created","tampered":true}}`,
			signature: generateHMAC(`{"meta":{"event_name":"order_created"}}`, "test_webhook_secret"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.verifySignature([]byte(tt.payload), tt.signature)
			if (err != nil) != tt.wantErr {
				t.Errorf("verifySignature() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVerifySignature_NoSecret(t *testing.T) {
	cfg := &config.LemonSqueezyConfig{
		APIKey:        "test_api_key",
		StoreID:       "12345",
		WebhookSecret: "", // No secret configured
	}

	provider := NewProvider(cfg)

	// Should return error when no secret is configured (security requirement)
	err := provider.verifySignature([]byte(`{"test":"data"}`), "any_signature")
	if err == nil {
		t.Error("verifySignature() should return error when no secret is configured")
	}
	if err != ErrWebhookSecretNotConfigured {
		t.Errorf("verifySignature() should return ErrWebhookSecretNotConfigured, got %v", err)
	}
}

func TestParseOrderEvent(t *testing.T) {
	payload := WebhookPayload{
		Meta: WebhookMeta{
			EventName: billing.WebhookEventLSOrderCreated,
			CustomData: map[string]interface{}{
				"order_no": "ORD-123",
			},
		},
		Data: WebhookData{
			ID:   "order_12345",
			Type: "orders",
			Attributes: WebhookDataAttributes{
				Total:      2999,
				Currency:   "USD",
				CustomerID: 67890,
			},
		},
	}

	cfg := &config.LemonSqueezyConfig{}
	provider := NewProvider(cfg)

	result := &types.WebhookEvent{}
	provider.parseOrderEvent(&payload, result)

	if result.ExternalOrderNo != "order_12345" {
		t.Errorf("expected external order no 'order_12345', got %s", result.ExternalOrderNo)
	}
	if result.Amount != 29.99 {
		t.Errorf("expected amount 29.99, got %f", result.Amount)
	}
	if result.Currency != "USD" {
		t.Errorf("expected currency USD, got %s", result.Currency)
	}
	if result.CustomerID != "67890" {
		t.Errorf("expected customer ID '67890', got %s", result.CustomerID)
	}
	if result.Status != billing.OrderStatusSucceeded {
		t.Errorf("expected status succeeded, got %s", result.Status)
	}
}

func TestParseSubscriptionEvents(t *testing.T) {
	tests := []struct {
		name          string
		eventName     string
		parseFunc     func(*Provider, *WebhookPayload, *types.WebhookEvent)
		expectedState string
	}{
		{
			name:      "subscription_created",
			eventName: billing.WebhookEventLSSubscriptionCreated,
			parseFunc: func(p *Provider, payload *WebhookPayload, result *types.WebhookEvent) {
				p.parseSubscriptionCreatedEvent(payload, result)
			},
			expectedState: billing.SubscriptionStatusActive,
		},
		{
			name:      "subscription_cancelled",
			eventName: billing.WebhookEventLSSubscriptionCancelled,
			parseFunc: func(p *Provider, payload *WebhookPayload, result *types.WebhookEvent) {
				p.parseSubscriptionCancelledEvent(payload, result)
			},
			expectedState: billing.SubscriptionStatusCanceled,
		},
		{
			name:      "subscription_paused",
			eventName: billing.WebhookEventLSSubscriptionPaused,
			parseFunc: func(p *Provider, payload *WebhookPayload, result *types.WebhookEvent) {
				p.parseSubscriptionPausedEvent(payload, result)
			},
			expectedState: billing.SubscriptionStatusPaused,
		},
		{
			name:      "subscription_resumed",
			eventName: billing.WebhookEventLSSubscriptionResumed,
			parseFunc: func(p *Provider, payload *WebhookPayload, result *types.WebhookEvent) {
				p.parseSubscriptionResumedEvent(payload, result)
			},
			expectedState: billing.SubscriptionStatusActive,
		},
		{
			name:      "subscription_expired",
			eventName: billing.WebhookEventLSSubscriptionExpired,
			parseFunc: func(p *Provider, payload *WebhookPayload, result *types.WebhookEvent) {
				p.parseSubscriptionExpiredEvent(payload, result)
			},
			expectedState: billing.SubscriptionStatusExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := WebhookPayload{
				Meta: WebhookMeta{
					EventName: tt.eventName,
				},
				Data: WebhookData{
					ID:   "sub_12345",
					Type: "subscriptions",
					Attributes: WebhookDataAttributes{
						CustomerID: 67890,
					},
				},
			}

			cfg := &config.LemonSqueezyConfig{}
			provider := NewProvider(cfg)

			result := &types.WebhookEvent{}
			tt.parseFunc(provider, &payload, result)

			if result.SubscriptionID != "sub_12345" {
				t.Errorf("expected subscription ID 'sub_12345', got %s", result.SubscriptionID)
			}
			if result.CustomerID != "67890" {
				t.Errorf("expected customer ID '67890', got %s", result.CustomerID)
			}
			if result.Status != tt.expectedState {
				t.Errorf("expected status %s, got %s", tt.expectedState, result.Status)
			}
		})
	}
}

func TestParsePaymentEvents(t *testing.T) {
	tests := []struct {
		name           string
		eventName      string
		parseFunc      func(*Provider, *WebhookPayload, *types.WebhookEvent)
		expectedStatus string
	}{
		{
			name:      "payment_success",
			eventName: billing.WebhookEventLSPaymentSuccess,
			parseFunc: func(p *Provider, payload *WebhookPayload, result *types.WebhookEvent) {
				p.parsePaymentSuccessEvent(payload, result)
			},
			expectedStatus: billing.OrderStatusSucceeded,
		},
		{
			name:      "payment_failed",
			eventName: billing.WebhookEventLSPaymentFailed,
			parseFunc: func(p *Provider, payload *WebhookPayload, result *types.WebhookEvent) {
				p.parsePaymentFailedEvent(payload, result)
			},
			expectedStatus: billing.OrderStatusFailed,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := WebhookPayload{
				Meta: WebhookMeta{
					EventName: tt.eventName,
				},
				Data: WebhookData{
					ID:   "inv_12345",
					Type: "subscription-invoices",
					Attributes: WebhookDataAttributes{
						Total:          1999,
						Currency:       "USD",
						SubscriptionID: 54321,
						CustomerID:     67890,
					},
				},
			}

			cfg := &config.LemonSqueezyConfig{}
			provider := NewProvider(cfg)

			result := &types.WebhookEvent{}
			tt.parseFunc(provider, &payload, result)

			if result.SubscriptionID != "54321" {
				t.Errorf("expected subscription ID '54321', got %s", result.SubscriptionID)
			}
			if result.CustomerID != "67890" {
				t.Errorf("expected customer ID '67890', got %s", result.CustomerID)
			}
			if result.Status != tt.expectedStatus {
				t.Errorf("expected status %s, got %s", tt.expectedStatus, result.Status)
			}
		})
	}
}

func TestHandleWebhook_InvalidSignature(t *testing.T) {
	cfg := &config.LemonSqueezyConfig{
		APIKey:        "test_api_key",
		StoreID:       "12345",
		WebhookSecret: "test_secret",
	}

	provider := NewProvider(cfg)

	payload := []byte(`{"meta":{"event_name":"order_created"}}`)
	_, err := provider.HandleWebhook(nil, payload, "invalid_signature")
	if err == nil {
		t.Error("expected error for invalid signature")
	}
}

func TestHandleWebhook_ValidPayload(t *testing.T) {
	secret := "test_webhook_secret"
	cfg := &config.LemonSqueezyConfig{
		APIKey:        "test_api_key",
		StoreID:       "12345",
		WebhookSecret: secret,
	}

	provider := NewProvider(cfg)

	payload := WebhookPayload{
		Meta: WebhookMeta{
			EventName: billing.WebhookEventLSOrderCreated,
			CustomData: map[string]interface{}{
				"order_no": "ORD-123",
			},
		},
		Data: WebhookData{
			ID:   "order_12345",
			Type: "orders",
			Attributes: WebhookDataAttributes{
				Total:      2999,
				Currency:   "USD",
				CustomerID: 67890,
			},
		},
	}

	payloadBytes, _ := json.Marshal(payload)
	signature := generateHMAC(string(payloadBytes), secret)

	event, err := provider.HandleWebhook(nil, payloadBytes, signature)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if event.EventType != billing.WebhookEventLSOrderCreated {
		t.Errorf("expected event type %s, got %s", billing.WebhookEventLSOrderCreated, event.EventType)
	}
	if event.Provider != billing.PaymentProviderLemonSqueezy {
		t.Errorf("expected provider %s, got %s", billing.PaymentProviderLemonSqueezy, event.Provider)
	}
	if event.OrderNo != "ORD-123" {
		t.Errorf("expected order no 'ORD-123', got %s", event.OrderNo)
	}
}

func TestHandleWebhook_AllEventTypes(t *testing.T) {
	secret := "test_webhook_secret"
	cfg := &config.LemonSqueezyConfig{
		APIKey:        "test_api_key",
		StoreID:       "12345",
		WebhookSecret: secret,
	}

	provider := NewProvider(cfg)

	eventTypes := []string{
		billing.WebhookEventLSOrderCreated,
		billing.WebhookEventLSSubscriptionCreated,
		billing.WebhookEventLSSubscriptionUpdated,
		billing.WebhookEventLSSubscriptionCancelled,
		billing.WebhookEventLSSubscriptionPaused,
		billing.WebhookEventLSSubscriptionResumed,
		billing.WebhookEventLSSubscriptionExpired,
		billing.WebhookEventLSPaymentSuccess,
		billing.WebhookEventLSPaymentFailed,
	}

	for _, eventType := range eventTypes {
		t.Run(eventType, func(t *testing.T) {
			payload := WebhookPayload{
				Meta: WebhookMeta{
					EventName: eventType,
				},
				Data: WebhookData{
					ID:   "test_12345",
					Type: "test",
					Attributes: WebhookDataAttributes{
						Total:          1999,
						Currency:       "USD",
						CustomerID:     67890,
						SubscriptionID: 54321,
					},
				},
			}

			payloadBytes, _ := json.Marshal(payload)
			signature := generateHMAC(string(payloadBytes), secret)

			event, err := provider.HandleWebhook(nil, payloadBytes, signature)
			if err != nil {
				t.Fatalf("unexpected error for event %s: %v", eventType, err)
			}

			if event.EventType != eventType {
				t.Errorf("expected event type %s, got %s", eventType, event.EventType)
			}
			if event.RawPayload == nil {
				t.Error("expected raw payload to be stored")
			}
		})
	}
}

func TestStringToInt(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"123", 123},
		{"0", 0},
		{"invalid", 0},
		{"", 0},
	}

	for _, tt := range tests {
		got := stringToInt(tt.input)
		if got != tt.expected {
			t.Errorf("stringToInt(%q) = %d, want %d", tt.input, got, tt.expected)
		}
	}
}

// Helper function to generate HMAC signature
func generateHMAC(payload, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	return hex.EncodeToString(mac.Sum(nil))
}

func TestGetCheckoutStatus(t *testing.T) {
	cfg := &config.LemonSqueezyConfig{
		APIKey:  "test_api_key",
		StoreID: "12345",
	}

	provider := NewProvider(cfg)

	// GetCheckoutStatus always returns pending for LemonSqueezy
	status, err := provider.GetCheckoutStatus(nil, "any_session_id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != billing.OrderStatusPending {
		t.Errorf("expected status %s, got %s", billing.OrderStatusPending, status)
	}
}

func TestRefundPayment(t *testing.T) {
	cfg := &config.LemonSqueezyConfig{
		APIKey:  "test_api_key",
		StoreID: "12345",
	}

	provider := NewProvider(cfg)

	// RefundPayment always returns error for LemonSqueezy (must be done via dashboard)
	_, err := provider.RefundPayment(nil, &types.RefundRequest{})
	if err == nil {
		t.Error("expected error for RefundPayment")
	}
	if err.Error() != "refunds must be processed through the LemonSqueezy dashboard" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestCreateCustomer(t *testing.T) {
	cfg := &config.LemonSqueezyConfig{
		APIKey:  "test_api_key",
		StoreID: "12345",
	}

	provider := NewProvider(cfg)

	// CreateCustomer returns empty string for LemonSqueezy (customers are created during checkout)
	customerID, err := provider.CreateCustomer(nil, "test@example.com", "Test User", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if customerID != "" {
		t.Errorf("expected empty customer ID, got %s", customerID)
	}
}

func TestCreateCheckoutSession_MissingVariantID(t *testing.T) {
	cfg := &config.LemonSqueezyConfig{
		APIKey:  "test_api_key",
		StoreID: "12345",
	}

	provider := NewProvider(cfg)

	// CreateCheckoutSession should fail when variant_id is missing
	req := &types.CheckoutRequest{
		OrganizationID: 1,
		UserID:         1,
		OrderType:      "subscription",
		Metadata:       nil, // No metadata
	}

	_, err := provider.CreateCheckoutSession(nil, req)
	if err == nil {
		t.Error("expected error for missing variant_id")
	}
	if err.Error() != "variant_id is required in metadata for LemonSqueezy checkout" {
		t.Errorf("unexpected error message: %v", err)
	}

	// Also test with empty metadata
	req.Metadata = map[string]string{}
	_, err = provider.CreateCheckoutSession(nil, req)
	if err == nil {
		t.Error("expected error for empty variant_id")
	}
}

func TestGetCustomerPortalURL_MissingSubscriptionID(t *testing.T) {
	cfg := &config.LemonSqueezyConfig{
		APIKey:  "test_api_key",
		StoreID: "12345",
	}

	provider := NewProvider(cfg)

	// GetCustomerPortalURL should fail when subscription_id is missing
	req := &types.CustomerPortalRequest{
		CustomerID:     "",
		SubscriptionID: "",
	}

	_, err := provider.GetCustomerPortalURL(nil, req)
	if err == nil {
		t.Error("expected error for missing subscription_id")
	}
	if err.Error() != "subscription_id is required for LemonSqueezy customer portal" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestHandleWebhook_InvalidJSON(t *testing.T) {
	cfg := &config.LemonSqueezyConfig{
		APIKey:        "test_api_key",
		StoreID:       "12345",
		WebhookSecret: "test_secret",
	}

	provider := NewProvider(cfg)

	// Test with invalid JSON
	payload := []byte(`{invalid json}`)
	signature := generateHMAC(string(payload), "test_secret")

	_, err := provider.HandleWebhook(nil, payload, signature)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestHandleWebhook_EmptyDataID(t *testing.T) {
	secret := "test_webhook_secret"
	cfg := &config.LemonSqueezyConfig{
		APIKey:        "test_api_key",
		StoreID:       "12345",
		WebhookSecret: secret,
	}

	provider := NewProvider(cfg)

	// Test with empty Data.ID - should use fallback event ID
	payload := WebhookPayload{
		Meta: WebhookMeta{
			EventName: billing.WebhookEventLSOrderCreated,
		},
		Data: WebhookData{
			ID:   "", // Empty ID
			Type: "orders",
			Attributes: WebhookDataAttributes{
				Total:      1000,
				Currency:   "USD",
				CustomerID: 12345,
			},
		},
	}

	payloadBytes, _ := json.Marshal(payload)
	signature := generateHMAC(string(payloadBytes), secret)

	event, err := provider.HandleWebhook(nil, payloadBytes, signature)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Event ID should contain event name (fallback format)
	if event.EventID == "" {
		t.Error("expected non-empty event ID")
	}
}

func TestParseSubscriptionUpdatedEvent(t *testing.T) {
	payload := WebhookPayload{
		Meta: WebhookMeta{
			EventName: billing.WebhookEventLSSubscriptionUpdated,
		},
		Data: WebhookData{
			ID:   "sub_12345",
			Type: "subscriptions",
			Attributes: WebhookDataAttributes{
				CustomerID: 67890,
				Status:     "active",
			},
		},
	}

	cfg := &config.LemonSqueezyConfig{}
	provider := NewProvider(cfg)

	result := &types.WebhookEvent{}
	provider.parseSubscriptionUpdatedEvent(&payload, result)

	if result.SubscriptionID != "sub_12345" {
		t.Errorf("expected subscription ID 'sub_12345', got %s", result.SubscriptionID)
	}
	if result.CustomerID != "67890" {
		t.Errorf("expected customer ID '67890', got %s", result.CustomerID)
	}
	if result.Status != "active" {
		t.Errorf("expected status 'active', got %s", result.Status)
	}
}

func TestParseOrderEvent_ZeroCustomerID(t *testing.T) {
	payload := WebhookPayload{
		Meta: WebhookMeta{
			EventName: billing.WebhookEventLSOrderCreated,
		},
		Data: WebhookData{
			ID:   "order_12345",
			Type: "orders",
			Attributes: WebhookDataAttributes{
				Total:      2999,
				Currency:   "USD",
				CustomerID: 0, // Zero customer ID
			},
		},
	}

	cfg := &config.LemonSqueezyConfig{}
	provider := NewProvider(cfg)

	result := &types.WebhookEvent{}
	provider.parseOrderEvent(&payload, result)

	// CustomerID should remain empty when source is 0
	if result.CustomerID != "" {
		t.Errorf("expected empty customer ID for zero value, got %s", result.CustomerID)
	}
}

func TestParsePaymentEvents_ZeroIDs(t *testing.T) {
	cfg := &config.LemonSqueezyConfig{}
	provider := NewProvider(cfg)

	// Test payment success with zero subscription ID
	payload := WebhookPayload{
		Data: WebhookData{
			ID: "inv_12345",
			Attributes: WebhookDataAttributes{
				Total:          1000,
				Currency:       "USD",
				SubscriptionID: 0, // Zero
				CustomerID:     0, // Zero
			},
		},
	}

	result := &types.WebhookEvent{}
	provider.parsePaymentSuccessEvent(&payload, result)

	if result.SubscriptionID != "" {
		t.Errorf("expected empty subscription ID for zero value, got %s", result.SubscriptionID)
	}
	if result.CustomerID != "" {
		t.Errorf("expected empty customer ID for zero value, got %s", result.CustomerID)
	}

	// Test payment failed
	result2 := &types.WebhookEvent{}
	provider.parsePaymentFailedEvent(&payload, result2)

	if result2.SubscriptionID != "" {
		t.Errorf("expected empty subscription ID for zero value, got %s", result2.SubscriptionID)
	}
}
