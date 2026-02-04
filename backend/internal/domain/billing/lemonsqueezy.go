package billing

// =============================================================================
// LemonSqueezy Payment Provider Constants
// =============================================================================
//
// LemonSqueezy is the primary payment provider for US/Global deployments.
// Documentation: https://docs.lemonsqueezy.com/api
// =============================================================================

// LemonSqueezy webhook event type constants
// See: https://docs.lemonsqueezy.com/help/webhooks
const (
	// Order events
	WebhookEventLSOrderCreated = "order_created"

	// Subscription lifecycle events
	WebhookEventLSSubscriptionCreated   = "subscription_created"
	WebhookEventLSSubscriptionUpdated   = "subscription_updated"
	WebhookEventLSSubscriptionCancelled = "subscription_cancelled"
	WebhookEventLSSubscriptionPaused    = "subscription_paused"
	WebhookEventLSSubscriptionResumed   = "subscription_resumed"
	WebhookEventLSSubscriptionExpired   = "subscription_expired"

	// Subscription payment events
	WebhookEventLSPaymentSuccess = "subscription_payment_success"
	WebhookEventLSPaymentFailed  = "subscription_payment_failed"
)

// LemonSqueezy subscription status values
// These are the status values returned by the LemonSqueezy API
const (
	LSSubscriptionStatusOnTrial   = "on_trial"
	LSSubscriptionStatusActive    = "active"
	LSSubscriptionStatusPaused    = "paused"
	LSSubscriptionStatusPastDue   = "past_due"
	LSSubscriptionStatusUnpaid    = "unpaid"
	LSSubscriptionStatusCancelled = "cancelled"
	LSSubscriptionStatusExpired   = "expired"
)

// MapLSStatusToInternal maps LemonSqueezy subscription status to internal status
func MapLSStatusToInternal(lsStatus string) string {
	switch lsStatus {
	case LSSubscriptionStatusOnTrial:
		return SubscriptionStatusTrialing
	case LSSubscriptionStatusActive:
		return SubscriptionStatusActive
	case LSSubscriptionStatusPaused:
		return SubscriptionStatusPaused
	case LSSubscriptionStatusPastDue:
		return SubscriptionStatusPastDue
	case LSSubscriptionStatusUnpaid:
		return SubscriptionStatusFrozen
	case LSSubscriptionStatusCancelled:
		return SubscriptionStatusCanceled
	case LSSubscriptionStatusExpired:
		return SubscriptionStatusExpired
	default:
		return lsStatus
	}
}
