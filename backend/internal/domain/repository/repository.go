package repository

import (
	"time"
)

// Repository represents a Git repository configuration
type Repository struct {
	ID             int64   `gorm:"primaryKey" json:"id"`
	OrganizationID int64   `gorm:"not null;index" json:"organization_id"`
	TeamID         *int64  `gorm:"index" json:"team_id,omitempty"`
	GitProviderID  int64   `gorm:"not null;index" json:"git_provider_id"`

	ExternalID    string  `gorm:"size:255;not null" json:"external_id"` // Provider's project ID
	Name          string  `gorm:"size:255;not null" json:"name"`
	FullPath      string  `gorm:"size:500;not null" json:"full_path"`
	DefaultBranch string  `gorm:"size:100;default:'main'" json:"default_branch"`
	TicketPrefix  *string `gorm:"size:10" json:"ticket_prefix,omitempty"` // Ticket prefix like 'AM'

	IsActive bool `gorm:"not null;default:true" json:"is_active"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (Repository) TableName() string {
	return "repositories"
}

// Branch represents a Git branch
type Branch struct {
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	Commit    string `json:"commit,omitempty"`
}

// WebhookConfig represents webhook configuration for a repository
type WebhookConfig struct {
	ID        string   `json:"id"`
	URL       string   `json:"url"`
	Events    []string `json:"events"`
	IsActive  bool     `json:"is_active"`
	CreatedAt string   `json:"created_at,omitempty"`
}
