package organization

import (
	"time"
)

// Organization role constants
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

// Team role constants
const (
	TeamRoleLead   = "lead"
	TeamRoleMember = "member"
)

// Organization represents a tenant in the multi-tenant system
type Organization struct {
	ID   int64  `gorm:"primaryKey" json:"id"`
	Name string `gorm:"size:100;not null" json:"name"`
	Slug string `gorm:"size:100;not null;uniqueIndex" json:"slug"`

	LogoURL *string `gorm:"type:text" json:"logo_url,omitempty"`

	// Subscription info
	SubscriptionPlan   string `gorm:"size:50;not null;default:'free'" json:"subscription_plan"`
	SubscriptionStatus string `gorm:"size:20;not null;default:'active'" json:"subscription_status"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`

	// Associations
	Teams   []Team   `gorm:"foreignKey:OrganizationID" json:"teams,omitempty"`
	Members []Member `gorm:"foreignKey:OrganizationID" json:"members,omitempty"`
}

func (Organization) TableName() string {
	return "organizations"
}

// GetID returns the organization ID (implements OrganizationGetter interface)
func (o *Organization) GetID() int64 {
	return o.ID
}

// GetSlug returns the organization slug (implements OrganizationGetter interface)
func (o *Organization) GetSlug() string {
	return o.Slug
}

// GetName returns the organization name (implements OrganizationGetter interface)
func (o *Organization) GetName() string {
	return o.Name
}

// Team represents a team within an organization
type Team struct {
	ID             int64  `gorm:"primaryKey" json:"id"`
	OrganizationID int64  `gorm:"not null;index" json:"organization_id"`
	Name           string `gorm:"size:100;not null" json:"name"`
	Description    string `gorm:"type:text" json:"description,omitempty"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`

	// Associations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Members      []TeamMember  `gorm:"foreignKey:TeamID" json:"members,omitempty"`
}

func (Team) TableName() string {
	return "teams"
}

// Member represents an organization membership
type Member struct {
	ID             int64  `gorm:"primaryKey" json:"id"`
	OrganizationID int64  `gorm:"not null;index" json:"organization_id"`
	UserID         int64  `gorm:"not null;index" json:"user_id"`
	Role           string `gorm:"size:50;not null;default:'member'" json:"role"` // owner, admin, member

	JoinedAt time.Time `gorm:"not null;default:now()" json:"joined_at"`

	// Associations
	Organization *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

func (Member) TableName() string {
	return "organization_members"
}

// TeamMember represents a team membership
type TeamMember struct {
	ID     int64  `gorm:"primaryKey" json:"id"`
	TeamID int64  `gorm:"not null;index" json:"team_id"`
	UserID int64  `gorm:"not null;index" json:"user_id"`
	Role   string `gorm:"size:50;not null;default:'member'" json:"role"` // lead, member

	// Associations
	Team *Team `gorm:"foreignKey:TeamID" json:"team,omitempty"`
}

func (TeamMember) TableName() string {
	return "team_members"
}
