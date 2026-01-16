package billing

import "time"

// License represents an OnPremise license
type License struct {
	ID int64 `gorm:"primaryKey" json:"id"`

	// License identification
	LicenseKey string `gorm:"size:255;not null;uniqueIndex" json:"license_key"`

	// License information
	OrganizationName string `gorm:"size:255;not null" json:"organization_name"`
	ContactEmail     string `gorm:"size:255;not null" json:"contact_email"`

	// License scope
	PlanName          string   `gorm:"size:50;not null" json:"plan_name"`
	MaxUsers          int      `gorm:"not null;default:-1" json:"max_users"`
	MaxRunners        int      `gorm:"not null;default:-1" json:"max_runners"`
	MaxRepositories   int      `gorm:"not null;default:-1" json:"max_repositories"`
	MaxConcurrentPods int      `gorm:"not null;default:-1" json:"max_concurrent_pods"`
	Features          Features `gorm:"type:jsonb;default:'{}'" json:"features,omitempty"`

	// Validity
	IssuedAt  time.Time  `gorm:"not null;default:now()" json:"issued_at"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	// Signature verification
	Signature            string  `gorm:"type:text;not null" json:"signature"`
	PublicKeyFingerprint *string `gorm:"size:64" json:"public_key_fingerprint,omitempty"`

	// Status
	IsActive         bool       `gorm:"not null;default:true" json:"is_active"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	RevocationReason *string    `gorm:"type:text" json:"revocation_reason,omitempty"`

	// Activation tracking
	ActivatedAt    *time.Time `json:"activated_at,omitempty"`
	ActivatedOrgID *int64     `json:"activated_org_id,omitempty"`
	LastVerifiedAt *time.Time `json:"last_verified_at,omitempty"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (License) TableName() string {
	return "licenses"
}

// IsValid returns true if the license is valid and not expired
func (l *License) IsValid() bool {
	if !l.IsActive {
		return false
	}
	if l.RevokedAt != nil {
		return false
	}
	if l.ExpiresAt != nil && time.Now().After(*l.ExpiresAt) {
		return false
	}
	return true
}

// IsActivated returns true if the license has been activated
func (l *License) IsActivated() bool {
	return l.ActivatedAt != nil && l.ActivatedOrgID != nil
}

// DaysUntilExpiry returns the number of days until expiration, or -1 if no expiry
func (l *License) DaysUntilExpiry() int {
	if l.ExpiresAt == nil {
		return -1
	}
	duration := time.Until(*l.ExpiresAt)
	return int(duration.Hours() / 24)
}

// LicenseStatus represents the current license status (used for API responses)
type LicenseStatus struct {
	IsActive         bool       `json:"is_active"`
	LicenseKey       string     `json:"license_key,omitempty"`
	OrganizationName string     `json:"organization_name,omitempty"`
	Plan             string     `json:"plan,omitempty"`
	ExpiresAt        *time.Time `json:"expires_at,omitempty"`
	MaxUsers         int        `json:"max_users,omitempty"`
	MaxRunners       int        `json:"max_runners,omitempty"`
	MaxRepositories  int        `json:"max_repositories,omitempty"`
	MaxPodMinutes    int        `json:"max_pod_minutes,omitempty"` // -1 for unlimited
	Features         []string   `json:"features,omitempty"`
	Message          string     `json:"message,omitempty"`
}
