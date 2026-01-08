package session

import (
	"time"

	"github.com/anthropics/agentmesh/backend/internal/domain/agent"
	"github.com/anthropics/agentmesh/backend/internal/domain/runner"
)

// Session status constants
const (
	StatusInitializing = "initializing"
	StatusRunning      = "running"
	StatusPaused       = "paused"
	StatusDisconnected = "disconnected" // User closed browser
	StatusOrphaned     = "orphaned"     // Lost due to runner restart
	StatusCompleted    = "completed"
	StatusTerminated   = "terminated"
	StatusError        = "error"

	// Legacy aliases
	SessionStatusInitializing = StatusInitializing
	SessionStatusRunning      = StatusRunning
	SessionStatusPaused       = StatusPaused
	SessionStatusDisconnected = StatusDisconnected
	SessionStatusOrphaned     = StatusOrphaned
	SessionStatusTerminated   = StatusTerminated
	SessionStatusError        = StatusError
)

// Agent status constants
const (
	AgentStatusUnknown  = "unknown"
	AgentStatusIdle     = "idle"
	AgentStatusWorking  = "working"
	AgentStatusWaiting  = "waiting"
	AgentStatusFinished = "finished"
)

// Think level magic words for Claude
const (
	ThinkLevelNone       = ""
	ThinkLevelUltrathink = "ultrathink"
	ThinkLevelMegathink  = "megathink"
)

// Permission mode for Claude
const (
	PermissionModePlan    = "plan"
	PermissionModeDefault = "default"
	PermissionModeBypass  = "bypassPermissions"
)

// Session represents an AI coding session
type Session struct {
	ID             int64  `gorm:"primaryKey" json:"id"`
	OrganizationID int64  `gorm:"not null;index" json:"organization_id"`
	TeamID         *int64 `gorm:"index" json:"team_id,omitempty"`

	SessionKey string `gorm:"size:100;not null;uniqueIndex" json:"session_key"`
	RunnerID   int64  `gorm:"not null;index" json:"runner_id"`

	AgentTypeID       *int64 `json:"agent_type_id,omitempty"`
	CustomAgentTypeID *int64 `json:"custom_agent_type_id,omitempty"`

	RepositoryID *int64 `json:"repository_id,omitempty"`
	TicketID     *int64 `json:"ticket_id,omitempty"`
	CreatedByID  int64  `gorm:"not null" json:"created_by_id"`

	PtyPID      *int   `gorm:"column:pty_pid" json:"pty_pid,omitempty"`
	Status      string `gorm:"size:50;not null;default:'initializing';index" json:"status"`
	AgentStatus string `gorm:"size:50;not null;default:'unknown'" json:"agent_status"`
	AgentPID    *int   `gorm:"column:agent_pid" json:"agent_pid,omitempty"` // Claude/Agent process PID

	StartedAt    *time.Time `json:"started_at,omitempty"`
	FinishedAt   *time.Time `json:"finished_at,omitempty"`
	LastActivity *time.Time `json:"last_activity,omitempty"`

	// Initial prompt and configuration
	InitialPrompt string  `gorm:"type:text" json:"initial_prompt,omitempty"`
	BranchName    *string `gorm:"size:255" json:"branch_name,omitempty"`
	WorktreePath  *string `gorm:"size:500" json:"worktree_path,omitempty"`

	// Agent configuration used for this session
	Model          *string `gorm:"size:50" json:"model,omitempty"`           // opus/sonnet/haiku
	PermissionMode *string `gorm:"size:50" json:"permission_mode,omitempty"` // plan/default/bypassPermissions
	ThinkLevel     *string `gorm:"size:50" json:"think_level,omitempty"`     // ultrathink/megathink

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`

	// Associations
	Runner          *runner.Runner         `gorm:"foreignKey:RunnerID" json:"runner,omitempty"`
	AgentType       *agent.AgentType       `gorm:"foreignKey:AgentTypeID" json:"agent_type,omitempty"`
	CustomAgentType *agent.CustomAgentType `gorm:"foreignKey:CustomAgentTypeID" json:"custom_agent_type,omitempty"`
}

func (Session) TableName() string {
	return "sessions"
}

// IsActive returns true if session is active
func (s *Session) IsActive() bool {
	return s.Status == SessionStatusRunning ||
		s.Status == SessionStatusInitializing ||
		s.Status == SessionStatusPaused ||
		s.Status == SessionStatusDisconnected
}

// IsTerminal returns true if session is in a terminal state
func (s *Session) IsTerminal() bool {
	return s.Status == SessionStatusTerminated ||
		s.Status == SessionStatusOrphaned ||
		s.Status == SessionStatusError
}

// CanReconnect returns true if session can be reconnected
func (s *Session) CanReconnect() bool {
	return s.Status == SessionStatusDisconnected
}

// PreparationConfig holds the preparation script configuration
type PreparationConfig struct {
	Script  string `json:"script,omitempty"`
	Timeout int    `json:"timeout,omitempty"` // in seconds
}

// CreateSessionCommand represents a command to create a session on a runner
type CreateSessionCommand struct {
	SessionKey        string             `json:"session_id"` // Use session_id for compatibility with runner
	InitialCommand    string             `json:"initial_command,omitempty"`
	InitialPrompt     string             `json:"initial_prompt,omitempty"`
	PermissionMode    string             `json:"permission_mode,omitempty"`
	TicketIdentifier  string             `json:"ticket_identifier,omitempty"`
	WorktreeSuffix    string             `json:"worktree_suffix,omitempty"`
	EnvVars           map[string]string  `json:"env_vars,omitempty"`
	PreparationConfig *PreparationConfig `json:"preparation_config,omitempty"`
}

// TerminateSessionCommand represents a command to terminate a session
type TerminateSessionCommand struct {
	SessionKey string `json:"session_id"`
}
