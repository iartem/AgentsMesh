// Package tools provides MCP tools for agent collaboration.
package tools

import "context"

// BindingScope represents permission scopes for session bindings.
type BindingScope string

const (
	// ScopeTerminalRead allows reading terminal output.
	ScopeTerminalRead BindingScope = "terminal:read"
	// ScopeTerminalWrite allows writing to terminal.
	ScopeTerminalWrite BindingScope = "terminal:write"
)

// BindingStatus represents the status of a session binding.
type BindingStatus string

const (
	BindingStatusPending  BindingStatus = "pending"
	BindingStatusActive   BindingStatus = "active"
	BindingStatusRejected BindingStatus = "rejected"
	BindingStatusInactive BindingStatus = "inactive"
	BindingStatusExpired  BindingStatus = "expired"
)

// SessionStatus represents the status of a session.
type SessionStatus string

const (
	SessionStatusInitializing SessionStatus = "initializing"
	SessionStatusRunning      SessionStatus = "running"
	SessionStatusDisconnected SessionStatus = "disconnected"
	SessionStatusCompleted    SessionStatus = "completed"
	SessionStatusError        SessionStatus = "error"
	SessionStatusOrphaned     SessionStatus = "orphaned"
)

// TicketStatus represents ticket workflow states.
type TicketStatus string

const (
	TicketStatusBacklog    TicketStatus = "backlog"
	TicketStatusTodo       TicketStatus = "todo"
	TicketStatusInProgress TicketStatus = "in_progress"
	TicketStatusInReview   TicketStatus = "in_review"
	TicketStatusDone       TicketStatus = "done"
	TicketStatusCanceled   TicketStatus = "canceled"
)

// TicketType represents the type of ticket.
type TicketType string

const (
	TicketTypeTask        TicketType = "task"
	TicketTypeBug         TicketType = "bug"
	TicketTypeFeature     TicketType = "feature"
	TicketTypeImprovement TicketType = "improvement"
	TicketTypeEpic        TicketType = "epic"
)

// TicketPriority represents ticket priority levels.
type TicketPriority string

const (
	TicketPriorityUrgent TicketPriority = "urgent"
	TicketPriorityHigh   TicketPriority = "high"
	TicketPriorityMedium TicketPriority = "medium"
	TicketPriorityLow    TicketPriority = "low"
	TicketPriorityNone   TicketPriority = "none"
)

// ChannelMessageType represents the type of channel message.
type ChannelMessageType string

const (
	ChannelMessageTypeText   ChannelMessageType = "text"
	ChannelMessageTypeSystem ChannelMessageType = "system"
)

// Binding represents a session binding.
type Binding struct {
	ID               int            `json:"id"`
	InitiatorSession string         `json:"initiator_session"`
	TargetSession    string         `json:"target_session"`
	GrantedScopes    []BindingScope `json:"granted_scopes"`
	PendingScopes    []BindingScope `json:"pending_scopes"`
	Status           BindingStatus  `json:"status"`
	CreatedAt        string         `json:"created_at"`
	UpdatedAt        string         `json:"updated_at"`
}

// AvailableSession represents a session available for collaboration.
type AvailableSession struct {
	SessionKey     string        `json:"session_key"`
	UserID         int           `json:"user_id"`
	Username       string        `json:"username"`
	Status         SessionStatus `json:"status"`
	TicketID       *int          `json:"ticket_id,omitempty"`
	TicketTitle    string        `json:"ticket_title,omitempty"`
	ProjectID      *int          `json:"project_id,omitempty"`
	ProjectName    string        `json:"project_name,omitempty"`
	AgentType      string        `json:"agent_type,omitempty"`
	CreatedAt      string        `json:"created_at"`
}

// TerminalOutput represents terminal observation output.
type TerminalOutput struct {
	SessionKey   string `json:"session_key"`
	Output       string `json:"output"`
	Screen       string `json:"screen,omitempty"`
	CursorX      int    `json:"cursor_x"`
	CursorY      int    `json:"cursor_y"`
	TotalLines   int    `json:"total_lines"`
	HasMore      bool   `json:"has_more"`
}

// Channel represents a collaboration channel.
type Channel struct {
	ID             int     `json:"id"`
	Name           string  `json:"name"`
	Description    string  `json:"description,omitempty"`
	ProjectID      *int    `json:"project_id,omitempty"`
	TicketID       *int    `json:"ticket_id,omitempty"`
	Document       string  `json:"document,omitempty"`
	MemberCount    int     `json:"member_count"`
	IsArchived     bool    `json:"is_archived"`
	CreatedAt      string  `json:"created_at"`
	UpdatedAt      string  `json:"updated_at"`
}

// ChannelMessage represents a message in a channel.
type ChannelMessage struct {
	ID             int                `json:"id"`
	ChannelID      int                `json:"channel_id"`
	SenderSession  string             `json:"sender_session"`
	SenderUserID   *int               `json:"sender_user_id,omitempty"`
	Content        string             `json:"content"`
	MessageType    ChannelMessageType `json:"message_type"`
	Mentions       []string           `json:"mentions,omitempty"`
	ReplyTo        *int               `json:"reply_to,omitempty"`
	CreatedAt      string             `json:"created_at"`
}

// Ticket represents a ticket in the system.
type Ticket struct {
	ID             int            `json:"id"`
	Identifier     string         `json:"identifier"`
	Title          string         `json:"title"`
	Description    string         `json:"description,omitempty"`
	Content        string         `json:"content,omitempty"`
	Type           TicketType     `json:"type"`
	Status         TicketStatus   `json:"status"`
	Priority       TicketPriority `json:"priority"`
	ProductID      int            `json:"product_id"`
	ProductName    string         `json:"product_name,omitempty"`
	ReporterID     int            `json:"reporter_id"`
	ReporterName   string         `json:"reporter_name,omitempty"`
	ParentTicketID *int           `json:"parent_ticket_id,omitempty"`
	Estimate       *int           `json:"estimate,omitempty"`
	CreatedAt      string         `json:"created_at"`
	UpdatedAt      string         `json:"updated_at"`
}

// SessionCreateRequest represents a request to create a new session.
type SessionCreateRequest struct {
	RunnerID      int    `json:"runner_id,omitempty"`
	TicketID      *int   `json:"ticket_id,omitempty"`
	InitialPrompt string `json:"initial_prompt,omitempty"`
	Model         string `json:"model,omitempty"`
}

// SessionCreateResponse represents the response from creating a session.
type SessionCreateResponse struct {
	SessionKey    string `json:"session_key"`
	Status        string `json:"status"`
	TerminalURL   string `json:"terminal_url,omitempty"`
}

// TerminalClient defines the interface for terminal operations.
type TerminalClient interface {
	ObserveTerminal(ctx context.Context, sessionKey string, lines int, raw bool, includeScreen bool) (*TerminalOutput, error)
	SendTerminalText(ctx context.Context, sessionKey string, text string) error
	SendTerminalKey(ctx context.Context, sessionKey string, keys []string) error
}

// DiscoveryClient defines the interface for session discovery.
type DiscoveryClient interface {
	ListAvailableSessions(ctx context.Context) ([]AvailableSession, error)
}

// BindingClient defines the interface for session binding operations.
type BindingClient interface {
	RequestBinding(ctx context.Context, targetSession string, scopes []BindingScope) (*Binding, error)
	AcceptBinding(ctx context.Context, bindingID int) (*Binding, error)
	RejectBinding(ctx context.Context, bindingID int, reason string) (*Binding, error)
	UnbindSession(ctx context.Context, targetSession string) error
	GetBindings(ctx context.Context, status *BindingStatus) ([]Binding, error)
	GetBoundSessions(ctx context.Context) ([]AvailableSession, error)
}

// ChannelClient defines the interface for channel operations.
type ChannelClient interface {
	SearchChannels(ctx context.Context, name string, projectID, ticketID *int, isArchived *bool, offset, limit int) ([]Channel, error)
	CreateChannel(ctx context.Context, name, description string, projectID, ticketID *int) (*Channel, error)
	GetChannel(ctx context.Context, channelID int) (*Channel, error)
	SendMessage(ctx context.Context, channelID int, content string, msgType ChannelMessageType, mentions []string, replyTo *int) (*ChannelMessage, error)
	GetMessages(ctx context.Context, channelID int, beforeTime, afterTime *string, mentionedSession *string, limit int) ([]ChannelMessage, error)
	GetDocument(ctx context.Context, channelID int) (string, error)
	UpdateDocument(ctx context.Context, channelID int, document string) error
}

// TicketClient defines the interface for ticket operations.
type TicketClient interface {
	SearchTickets(ctx context.Context, productID *int, status *TicketStatus, ticketType *TicketType, priority *TicketPriority, assigneeID, parentID *int, query string, limit, page int) ([]Ticket, error)
	GetTicket(ctx context.Context, ticketID string) (*Ticket, error)
	CreateTicket(ctx context.Context, productID int, title, description string, ticketType TicketType, priority TicketPriority, parentTicketID *int) (*Ticket, error)
	UpdateTicket(ctx context.Context, ticketID string, title, description *string, status *TicketStatus, priority *TicketPriority, ticketType *TicketType) (*Ticket, error)
}

// SessionClient defines the interface for session creation.
type SessionClient interface {
	CreateSession(ctx context.Context, req *SessionCreateRequest) (*SessionCreateResponse, error)
}

// CollaborationClient combines all collaboration interfaces.
type CollaborationClient interface {
	TerminalClient
	DiscoveryClient
	BindingClient
	ChannelClient
	TicketClient
	SessionClient
}
