package promocode

import (
	"time"
)

// PromoCodeType represents the type of promo code
type PromoCodeType string

const (
	PromoTypeMedia    PromoCodeType = "media"    // Media promotion code
	PromoTypePartner  PromoCodeType = "partner"  // Partner code
	PromoTypeCampaign PromoCodeType = "campaign" // Marketing campaign code
	PromoTypeInternal PromoCodeType = "internal" // Internal use code
	PromoTypeReferral PromoCodeType = "referral" // Referral code
)

// PromoCode represents a promotional code
type PromoCode struct {
	ID          int64  `gorm:"primaryKey" json:"id"`
	Code        string `gorm:"size:50;not null;uniqueIndex" json:"code"`
	Name        string `gorm:"size:100;not null" json:"name"`
	Description string `gorm:"type:text" json:"description,omitempty"`

	Type           PromoCodeType `gorm:"size:50;not null" json:"type"`
	PlanName       string        `gorm:"size:50;not null" json:"plan_name"`
	DurationMonths int           `gorm:"not null" json:"duration_months"`

	MaxUses      *int `gorm:"" json:"max_uses,omitempty"`
	UsedCount    int  `gorm:"not null;default:0" json:"used_count"`
	MaxUsesPerOrg int `gorm:"not null;default:1" json:"max_uses_per_org"`

	StartsAt  time.Time  `gorm:"not null;default:now()" json:"starts_at"`
	ExpiresAt *time.Time `gorm:"" json:"expires_at,omitempty"`

	IsActive bool `gorm:"not null;default:true" json:"is_active"`

	CreatedByID *int64    `gorm:"" json:"created_by_id,omitempty"`
	CreatedAt   time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt   time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

// TableName returns the table name for PromoCode
func (PromoCode) TableName() string {
	return "promo_codes"
}

// IsValid checks if the promo code is valid for use
func (p *PromoCode) IsValid() bool {
	now := time.Now()

	// Check if active
	if !p.IsActive {
		return false
	}

	// Check if started
	if now.Before(p.StartsAt) {
		return false
	}

	// Check if expired
	if p.ExpiresAt != nil && now.After(*p.ExpiresAt) {
		return false
	}

	// Check if max uses reached
	if p.MaxUses != nil && p.UsedCount >= *p.MaxUses {
		return false
	}

	return true
}

// RemainingUses returns the remaining number of uses (-1 means unlimited)
func (p *PromoCode) RemainingUses() int {
	if p.MaxUses == nil {
		return -1
	}
	remaining := *p.MaxUses - p.UsedCount
	if remaining < 0 {
		return 0
	}
	return remaining
}

// Redemption represents a promo code redemption record
type Redemption struct {
	ID             int64 `gorm:"primaryKey" json:"id"`
	PromoCodeID    int64 `gorm:"not null;index" json:"promo_code_id"`
	OrganizationID int64 `gorm:"not null;index" json:"organization_id"`
	UserID         int64 `gorm:"not null;index" json:"user_id"`

	PlanName       string `gorm:"size:50;not null" json:"plan_name"`
	DurationMonths int    `gorm:"not null" json:"duration_months"`

	PreviousPlanName  *string    `gorm:"size:50" json:"previous_plan_name,omitempty"`
	PreviousPeriodEnd *time.Time `gorm:"" json:"previous_period_end,omitempty"`
	NewPeriodEnd      time.Time  `gorm:"not null" json:"new_period_end"`

	IPAddress *string `gorm:"type:inet" json:"ip_address,omitempty"`
	UserAgent *string `gorm:"type:text" json:"user_agent,omitempty"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`

	// Associations
	PromoCode *PromoCode `gorm:"foreignKey:PromoCodeID" json:"promo_code,omitempty"`
}

// TableName returns the table name for Redemption
func (Redemption) TableName() string {
	return "promo_code_redemptions"
}

// ListFilter represents filter options for listing promo codes
type ListFilter struct {
	Type     *PromoCodeType
	PlanName *string
	IsActive *bool
	Search   *string // Search in code or name
	Page     int
	PageSize int
}
