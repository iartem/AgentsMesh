package invitation

import (
	"time"
)

// Invitation represents a pending organization member invitation
type Invitation struct {
	ID             int64      `gorm:"primaryKey" json:"id"`
	OrganizationID int64      `gorm:"not null;index" json:"organization_id"`
	Email          string     `gorm:"size:255;not null;index" json:"email"`
	Role           string     `gorm:"size:20;not null;default:member" json:"role"`
	Token          string     `gorm:"size:255;not null;uniqueIndex" json:"-"`
	InvitedBy      int64      `gorm:"not null" json:"invited_by"`
	ExpiresAt      time.Time  `gorm:"not null" json:"expires_at"`
	AcceptedAt     *time.Time `json:"accepted_at,omitempty"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName specifies the table name for GORM
func (Invitation) TableName() string {
	return "invitations"
}

// IsExpired checks if the invitation has expired
func (i *Invitation) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

// IsAccepted checks if the invitation has been accepted
func (i *Invitation) IsAccepted() bool {
	return i.AcceptedAt != nil
}

// IsPending checks if the invitation is still pending
func (i *Invitation) IsPending() bool {
	return !i.IsAccepted() && !i.IsExpired()
}

// Repository defines the interface for invitation data access
type Repository interface {
	Create(invitation *Invitation) error
	GetByToken(token string) (*Invitation, error)
	GetByID(id int64) (*Invitation, error)
	GetByOrgAndEmail(orgID int64, email string) (*Invitation, error)
	ListByOrganization(orgID int64) ([]*Invitation, error)
	ListPendingByEmail(email string) ([]*Invitation, error)
	Update(invitation *Invitation) error
	Delete(id int64) error
	DeleteExpired() error
}
