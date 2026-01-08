package billing

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// Features represents plan features as JSONB
type Features map[string]interface{}

// Scan implements sql.Scanner for Features
func (f *Features) Scan(value interface{}) error {
	if value == nil {
		*f = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, f)
}

// Value implements driver.Valuer for Features
func (f Features) Value() (driver.Value, error) {
	if f == nil {
		return nil, nil
	}
	return json.Marshal(f)
}

// Plan names
const (
	PlanFree       = "free"
	PlanPro        = "pro"
	PlanEnterprise = "enterprise"
)

// SubscriptionPlan represents a subscription plan
type SubscriptionPlan struct {
	ID          int64  `gorm:"primaryKey" json:"id"`
	Name        string `gorm:"size:50;not null;uniqueIndex" json:"name"`
	DisplayName string `gorm:"size:100;not null" json:"display_name"`

	PricePerSeatMonthly   float64 `gorm:"type:decimal(10,2);not null;default:0" json:"price_per_seat_monthly"`
	IncludedSessionMinutes int     `gorm:"not null;default:0" json:"included_session_minutes"`
	PricePerExtraMinute   float64 `gorm:"type:decimal(10,4);not null;default:0" json:"price_per_extra_minute"`

	MaxUsers        int `gorm:"not null" json:"max_users"`
	MaxRunners      int `gorm:"not null" json:"max_runners"`
	MaxRepositories int `gorm:"not null" json:"max_repositories"`

	Features Features `gorm:"type:jsonb;not null;default:'{}'" json:"features"`

	IsActive bool `gorm:"not null;default:true" json:"is_active"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
}

func (SubscriptionPlan) TableName() string {
	return "subscription_plans"
}

// Subscription status constants
const (
	SubscriptionStatusActive   = "active"
	SubscriptionStatusPastDue  = "past_due"
	SubscriptionStatusCanceled = "canceled"
	SubscriptionStatusTrialing = "trialing"
)

// Billing cycle constants
const (
	BillingCycleMonthly = "monthly"
	BillingCycleYearly  = "yearly"
)

// CustomQuotas represents custom quota overrides
type CustomQuotas map[string]interface{}

// Scan implements sql.Scanner for CustomQuotas
func (cq *CustomQuotas) Scan(value interface{}) error {
	if value == nil {
		*cq = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, cq)
}

// Value implements driver.Valuer for CustomQuotas
func (cq CustomQuotas) Value() (driver.Value, error) {
	if cq == nil {
		return nil, nil
	}
	return json.Marshal(cq)
}

// Subscription represents an organization's subscription
type Subscription struct {
	ID             int64 `gorm:"primaryKey" json:"id"`
	OrganizationID int64 `gorm:"not null;uniqueIndex" json:"organization_id"`
	PlanID         int64 `gorm:"not null" json:"plan_id"`

	Status       string `gorm:"size:50;not null;default:'active'" json:"status"`
	BillingCycle string `gorm:"size:20;not null;default:'monthly'" json:"billing_cycle"`

	CurrentPeriodStart time.Time `gorm:"not null" json:"current_period_start"`
	CurrentPeriodEnd   time.Time `gorm:"not null" json:"current_period_end"`

	StripeCustomerID     *string `gorm:"size:255" json:"stripe_customer_id,omitempty"`
	StripeSubscriptionID *string `gorm:"size:255" json:"stripe_subscription_id,omitempty"`

	CustomQuotas CustomQuotas `gorm:"type:jsonb" json:"custom_quotas,omitempty"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`

	// Associations
	Plan *SubscriptionPlan `gorm:"foreignKey:PlanID" json:"plan,omitempty"`
}

func (Subscription) TableName() string {
	return "subscriptions"
}

// Usage type constants
const (
	UsageTypeSessionMinutes = "session_minutes"
	UsageTypeStorageGB      = "storage_gb"
	UsageTypeAPIRequests    = "api_requests"
)

// UsageMetadata represents optional usage metadata
type UsageMetadata map[string]interface{}

// Scan implements sql.Scanner for UsageMetadata
func (um *UsageMetadata) Scan(value interface{}) error {
	if value == nil {
		*um = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, um)
}

// Value implements driver.Valuer for UsageMetadata
func (um UsageMetadata) Value() (driver.Value, error) {
	if um == nil {
		return nil, nil
	}
	return json.Marshal(um)
}

// UsageRecord represents a usage record for billing
type UsageRecord struct {
	ID             int64 `gorm:"primaryKey" json:"id"`
	OrganizationID int64 `gorm:"not null;index" json:"organization_id"`

	UsageType string  `gorm:"size:50;not null;index" json:"usage_type"`
	Quantity  float64 `gorm:"type:decimal(10,2);not null" json:"quantity"`

	PeriodStart time.Time `gorm:"not null" json:"period_start"`
	PeriodEnd   time.Time `gorm:"not null" json:"period_end"`

	Metadata UsageMetadata `gorm:"type:jsonb" json:"metadata,omitempty"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
}

func (UsageRecord) TableName() string {
	return "usage_records"
}
