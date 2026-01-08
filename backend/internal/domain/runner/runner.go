package runner

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"time"
)

// HostInfo represents runner host information
type HostInfo map[string]interface{}

// Scan implements sql.Scanner for HostInfo
func (hi *HostInfo) Scan(value interface{}) error {
	if value == nil {
		*hi = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("type assertion to []byte failed")
	}
	return json.Unmarshal(bytes, hi)
}

// Value implements driver.Valuer for HostInfo
func (hi HostInfo) Value() (driver.Value, error) {
	if hi == nil {
		return nil, nil
	}
	return json.Marshal(hi)
}

// RegistrationToken represents a token used to register runners
type RegistrationToken struct {
	ID             int64  `gorm:"primaryKey" json:"id"`
	OrganizationID int64  `gorm:"not null;index" json:"organization_id"`
	TokenHash      string `gorm:"size:255;not null;uniqueIndex" json:"-"`
	Description    string `gorm:"type:text" json:"description,omitempty"`
	CreatedByID    int64  `gorm:"not null" json:"created_by_id"`

	IsActive  bool       `gorm:"not null;default:true" json:"is_active"`
	MaxUses   *int       `json:"max_uses,omitempty"`
	UsedCount int        `gorm:"not null;default:0" json:"used_count"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
}

func (RegistrationToken) TableName() string {
	return "runner_registration_tokens"
}

// Runner status constants
const (
	RunnerStatusOnline  = "online"
	RunnerStatusOffline = "offline"
	RunnerStatusBusy    = "busy"
)

// Runner represents a self-hosted runner
type Runner struct {
	ID             int64  `gorm:"primaryKey" json:"id"`
	OrganizationID int64  `gorm:"not null;index" json:"organization_id"`
	NodeID         string `gorm:"size:100;not null" json:"node_id"`
	Description    string `gorm:"type:text" json:"description,omitempty"`
	AuthTokenHash  string `gorm:"size:255;not null" json:"-"`

	Status                string     `gorm:"size:50;not null;default:'offline';index" json:"status"`
	LastHeartbeat         *time.Time `json:"last_heartbeat,omitempty"`
	CurrentSessions       int        `gorm:"not null;default:0" json:"current_sessions"`
	MaxConcurrentSessions int        `gorm:"not null;default:5" json:"max_concurrent_sessions"`
	RunnerVersion         *string    `gorm:"size:50" json:"runner_version,omitempty"`
	IsEnabled             bool       `gorm:"not null;default:true" json:"is_enabled"`

	HostInfo HostInfo `gorm:"type:jsonb" json:"host_info,omitempty"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`
}

func (Runner) TableName() string {
	return "runners"
}

// IsOnline returns true if runner is online
func (r *Runner) IsOnline() bool {
	return r.Status == RunnerStatusOnline
}

// CanAcceptSession returns true if runner can accept new sessions
func (r *Runner) CanAcceptSession() bool {
	return r.IsEnabled && r.IsOnline() && r.CurrentSessions < r.MaxConcurrentSessions
}
