// Package client provides communication with AgentMesh server.
package client

import (
	"encoding/json"
)

// MessageType defines the type of control message.
type MessageType string

const (
	// Client -> Server
	MsgTypeHeartbeat         MessageType = "heartbeat"
	MsgTypeSessionCreated    MessageType = "session_created"
	MsgTypeSessionTerminated MessageType = "session_terminated"
	MsgTypeStatusChange      MessageType = "status_change"
	MsgTypeSessionList       MessageType = "session_list"
	MsgTypeTerminalOutput    MessageType = "terminal_output" // PTY output from runner
	MsgTypePtyResized        MessageType = "pty_resized"     // PTY size changed

	// Server -> Client
	MsgTypeCreateSession    MessageType = "create_session"
	MsgTypeTerminateSession MessageType = "terminate_session"
	MsgTypeListSessions     MessageType = "list_sessions"
	MsgTypeTerminalInput    MessageType = "terminal_input"  // User input to PTY
	MsgTypeTerminalResize   MessageType = "terminal_resize" // Terminal resize
)

// ProtocolMessage is the base message structure for the new protocol.
// Matches backend's RunnerMessage struct for compatibility.
type ProtocolMessage struct {
	Type      MessageType     `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	Timestamp int64           `json:"timestamp"`           // Unix milliseconds to match backend
	Data      json.RawMessage `json:"data,omitempty"`
}

// HeartbeatData contains heartbeat information.
type HeartbeatData struct {
	NodeID   string        `json:"node_id"`
	Sessions []SessionInfo `json:"sessions"`
}

// SessionInfo contains session information for protocol messages.
type SessionInfo struct {
	SessionID    string `json:"session_id"`
	Status       string `json:"status"`
	ClaudeStatus string `json:"claude_status"`
	Pid          int    `json:"pid"`
	ClientCount  int    `json:"client_count"`
}

// PreparationConfig contains workspace preparation configuration.
type PreparationConfig struct {
	Script         string `json:"script,omitempty"`          // Shell script to execute
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"` // Script execution timeout (default: 300)
}

// CreateSessionRequest contains session creation request data.
type CreateSessionRequest struct {
	SessionID         string             `json:"session_id"`
	InitialCommand    string             `json:"initial_command,omitempty"`
	InitialPrompt     string             `json:"initial_prompt,omitempty"`     // Prompt to send after command starts (for interactive mode)
	PermissionMode    string             `json:"permission_mode,omitempty"`    // Permission mode (plan/default/etc). If "plan", will send Shift+Tab to enter Plan Mode
	WorkingDir        string             `json:"working_dir,omitempty"`
	TicketIdentifier  string             `json:"ticket_identifier,omitempty"`  // For worktree creation
	WorktreeSuffix    string             `json:"worktree_suffix,omitempty"`    // Suffix for worktree path to support multiple instances per ticket
	EnvVars           map[string]string  `json:"env_vars,omitempty"`           // Extra environment variables (e.g., AI provider credentials)
	PreparationConfig *PreparationConfig `json:"preparation_config,omitempty"` // Workspace preparation config
}

// TerminateSessionRequest contains session termination request data.
type TerminateSessionRequest struct {
	SessionID string `json:"session_id"`
}

// SessionCreatedEvent is sent when a session is created.
type SessionCreatedEvent struct {
	SessionID    string `json:"session_id"`
	Pid          int    `json:"pid"`
	WorktreePath string `json:"worktree_path,omitempty"` // Worktree path if created
	BranchName   string `json:"branch_name,omitempty"`   // Branch name if worktree created
	PtyCols      uint16 `json:"pty_cols"`                // PTY width in columns
	PtyRows      uint16 `json:"pty_rows"`                // PTY height in rows
}

// SessionTerminatedEvent is sent when a session is terminated.
type SessionTerminatedEvent struct {
	SessionID string `json:"session_id"`
}

// StatusChangeEvent is sent when claude status changes.
type StatusChangeEvent struct {
	SessionID    string `json:"session_id"`
	ClaudeStatus string `json:"claude_status"`
	ClaudePid    int    `json:"claude_pid,omitempty"`
}

// TerminalOutputEvent is sent when there's PTY output.
type TerminalOutputEvent struct {
	SessionID string `json:"session_id"`
	Data      string `json:"data"` // Base64 encoded binary data
}

// TerminalInputRequest is sent to write to PTY.
type TerminalInputRequest struct {
	SessionID string `json:"session_id"`
	Data      string `json:"data"` // Base64 encoded binary data
}

// TerminalResizeRequest is sent to resize PTY.
type TerminalResizeRequest struct {
	SessionID string `json:"session_id"`
	Cols      uint16 `json:"cols"`
	Rows      uint16 `json:"rows"`
}

// PtyResizedEvent is sent when PTY size changes.
type PtyResizedEvent struct {
	SessionID string `json:"session_id"`
	Cols      uint16 `json:"cols"`
	Rows      uint16 `json:"rows"`
}

// MessageHandler handles incoming messages from server.
type MessageHandler interface {
	OnCreateSession(req CreateSessionRequest) error
	OnTerminateSession(req TerminateSessionRequest) error
	OnListSessions() []SessionInfo
	OnTerminalInput(req TerminalInputRequest) error
	OnTerminalResize(req TerminalResizeRequest) error
}
