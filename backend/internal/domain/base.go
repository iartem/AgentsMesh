package domain

import (
	"time"
)

// BaseModel contains common fields for all models
type BaseModel struct {
	ID        int64     `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

// TenantModel adds organization_id for multi-tenant models
type TenantModel struct {
	BaseModel
	OrganizationID int64 `gorm:"not null;index" json:"organization_id"`
}

// TeamScopedModel adds team_id for team-level isolation
type TeamScopedModel struct {
	TenantModel
	TeamID *int64 `gorm:"index" json:"team_id,omitempty"`
}
