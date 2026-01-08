package team

import (
	"time"
)

// Team represents a team within an organization
type Team struct {
	ID             int64   `gorm:"primaryKey" json:"id"`
	OrganizationID int64   `gorm:"not null;index" json:"organization_id"`
	Name           string  `gorm:"size:100;not null" json:"name"`
	Description    *string `gorm:"type:text" json:"description,omitempty"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`

	// Associations
	Members []TeamMember `gorm:"foreignKey:TeamID" json:"members,omitempty"`
}

func (Team) TableName() string {
	return "teams"
}

// TeamMember represents a user's membership in a team
type TeamMember struct {
	ID     int64 `gorm:"primaryKey" json:"id"`
	TeamID int64 `gorm:"not null;index" json:"team_id"`
	UserID int64 `gorm:"not null;index" json:"user_id"`

	Role string `gorm:"size:50;not null;default:'member'" json:"role"` // lead, member

	// Associations - avoid cycle with pointers
	Team *Team `gorm:"foreignKey:TeamID" json:"team,omitempty"`
}

func (TeamMember) TableName() string {
	return "team_members"
}

// TeamWithMembers represents a team with member count
type TeamWithMembers struct {
	Team
	MemberCount int `json:"member_count"`
}
