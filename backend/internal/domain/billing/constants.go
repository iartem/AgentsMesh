package billing

// Plan names
const (
	PlanBased      = "based" // Entry-level paid plan
	PlanPro        = "pro"
	PlanEnterprise = "enterprise"
	PlanOnPremise  = "onpremise"
)

// Currency constants
const (
	CurrencyUSD = "USD"
	CurrencyCNY = "CNY"
)

// Default trial period in days
const DefaultTrialDays = 30

// Subscription status constants
const (
	SubscriptionStatusActive   = "active"
	SubscriptionStatusPastDue  = "past_due"
	SubscriptionStatusCanceled = "canceled"
	SubscriptionStatusTrialing = "trialing"
	SubscriptionStatusFrozen   = "frozen"
	SubscriptionStatusPaused   = "paused"
	SubscriptionStatusExpired  = "expired"
)

// Payment provider constants
const (
	PaymentProviderStripe       = "stripe"
	PaymentProviderLemonSqueezy = "lemonsqueezy"
	PaymentProviderAlipay       = "alipay"
	PaymentProviderWeChat       = "wechat"
	PaymentProviderLicense      = "license"
)

// Payment method constants
const (
	PaymentMethodCard            = "card"
	PaymentMethodAlipayQR        = "alipay_qr"
	PaymentMethodAlipayAgreement = "alipay_agreement"
	PaymentMethodWeChatNative    = "wechat_native"
	PaymentMethodWeChatContract  = "wechat_contract"
)

// Billing cycle constants
const (
	BillingCycleMonthly = "monthly"
	BillingCycleYearly  = "yearly"
)

// Usage type constants
const (
	UsageTypePodMinutes  = "pod_minutes"
	UsageTypeStorageGB   = "storage_gb"
	UsageTypeAPIRequests = "api_requests"
)

// Order type constants
const (
	OrderTypeSubscription = "subscription"
	OrderTypeSeatPurchase = "seat_purchase"
	OrderTypePlanUpgrade  = "plan_upgrade"
	OrderTypeRenewal      = "renewal"
)

// Order status constants
const (
	OrderStatusPending    = "pending"
	OrderStatusProcessing = "processing"
	OrderStatusSucceeded  = "succeeded"
	OrderStatusFailed     = "failed"
	OrderStatusCanceled   = "canceled"
	OrderStatusRefunded   = "refunded"
)

// Transaction type constants
const (
	TransactionTypePayment    = "payment"
	TransactionTypeRefund     = "refund"
	TransactionTypeChargeback = "chargeback"
)

// Transaction status constants
const (
	TransactionStatusPending   = "pending"
	TransactionStatusSucceeded = "succeeded"
	TransactionStatusFailed    = "failed"
)

// Invoice status constants
const (
	InvoiceStatusDraft  = "draft"
	InvoiceStatusIssued = "issued"
	InvoiceStatusPaid   = "paid"
	InvoiceStatusVoid   = "void"
)

// Webhook event type constants (Stripe-compatible)
const (
	// Checkout events
	WebhookEventCheckoutCompleted = "checkout.session.completed"

	// Invoice events
	WebhookEventInvoicePaid   = "invoice.paid"
	WebhookEventInvoiceFailed = "invoice.payment_failed"

	// Subscription events
	WebhookEventSubscriptionDeleted = "customer.subscription.deleted"
	WebhookEventSubscriptionUpdated = "customer.subscription.updated"
)

// LemonSqueezy webhook event constants are defined in lemonsqueezy.go
