package devmesh

import "time"

// DevMeshNode represents a session node in the DevMesh topology
type DevMeshNode struct {
	SessionKey  string       `json:"session_key"`
	Status      string       `json:"status"`
	AgentStatus string       `json:"agent_status"`
	Model       *string      `json:"model,omitempty"`
	TicketID    *int64       `json:"ticket_id,omitempty"`
	RepositoryID *int64      `json:"repository_id,omitempty"`
	CreatedByID int64        `json:"created_by_id"`
	RunnerID    int64        `json:"runner_id"`
	StartedAt   *time.Time   `json:"started_at,omitempty"`
	Position    *NodePosition `json:"position,omitempty"`
}

// NodePosition represents the visual position of a node in the topology graph
type NodePosition struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// DevMeshEdge represents a binding/connection between two session nodes
type DevMeshEdge struct {
	ID            int64    `json:"id"`
	Source        string   `json:"source"`         // Initiator session key
	Target        string   `json:"target"`         // Target session key
	GrantedScopes []string `json:"granted_scopes"`
	PendingScopes []string `json:"pending_scopes,omitempty"`
	Status        string   `json:"status"`
}

// ChannelInfo represents channel information for DevMesh visualization
type ChannelInfo struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Description  *string  `json:"description,omitempty"`
	SessionKeys  []string `json:"session_keys"`
	MessageCount int      `json:"message_count"`
	IsArchived   bool     `json:"is_archived"`
}

// DevMeshTopology represents the complete topology of active sessions and their connections
type DevMeshTopology struct {
	Nodes    []DevMeshNode `json:"nodes"`
	Edges    []DevMeshEdge `json:"edges"`
	Channels []ChannelInfo `json:"channels"`
}

// ChannelSession represents the association between a channel and a session
type ChannelSession struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	ChannelID  int64     `gorm:"not null;index" json:"channel_id"`
	SessionKey string    `gorm:"size:100;not null;index" json:"session_key"`
	JoinedAt   time.Time `gorm:"not null;default:now()" json:"joined_at"`
}

func (ChannelSession) TableName() string {
	return "channel_sessions"
}

// ChannelAccess represents access tracking for channels
type ChannelAccess struct {
	ID         int64     `gorm:"primaryKey" json:"id"`
	ChannelID  int64     `gorm:"not null;index" json:"channel_id"`
	SessionKey *string   `gorm:"size:100;index" json:"session_key,omitempty"`
	UserID     *int64    `json:"user_id,omitempty"`
	LastAccess time.Time `gorm:"not null;default:now()" json:"last_access"`
}

func (ChannelAccess) TableName() string {
	return "channel_access"
}

// CreateSessionForTicketRequest represents the request to create a session for a ticket
type CreateSessionForTicketRequest struct {
	OrganizationID int64  `json:"organization_id"`
	TeamID         *int64 `json:"team_id,omitempty"`
	TicketID       int64  `json:"ticket_id"`
	RunnerID       int64  `json:"runner_id"`
	CreatedByID    int64  `json:"created_by_id"`
	InitialPrompt  string `json:"initial_prompt,omitempty"`
	Model          string `json:"model,omitempty"`
	PermissionMode string `json:"permission_mode,omitempty"`
	ThinkLevel     string `json:"think_level,omitempty"`
}

// TicketSessionInfo represents session information for a ticket
type TicketSessionInfo struct {
	TicketID   int64           `json:"ticket_id"`
	Sessions   []DevMeshNode   `json:"sessions"`
}

// BatchTicketSessionsResponse represents the response for batch ticket sessions query
type BatchTicketSessionsResponse struct {
	TicketSessions map[int64][]DevMeshNode `json:"ticket_sessions"`
}
