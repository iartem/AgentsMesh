package runner

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var (
	ErrRunnerNotConnected = errors.New("runner not connected")
	ErrConnectionClosed   = errors.New("connection closed")
)

// RunnerConnection represents an active connection to a runner
type RunnerConnection struct {
	RunnerID int64
	Conn     *websocket.Conn
	Send     chan []byte
	LastPing time.Time
	mu       sync.Mutex
}

// RunnerMessage represents a message from/to a runner
type RunnerMessage struct {
	Type      string          `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	Data      json.RawMessage `json:"data,omitempty"`
	Timestamp int64           `json:"timestamp"`
}

// Runner message types
const (
	// From runner
	MsgTypeHeartbeat         = "heartbeat"
	MsgTypeSessionCreated    = "session_created"
	MsgTypeSessionTerminated = "session_terminated"
	MsgTypeTerminalOutput    = "terminal_output"
	MsgTypeAgentStatus       = "agent_status"
	MsgTypePtyResized        = "pty_resized"
	MsgTypeError             = "error"

	// To runner
	MsgTypeCreateSession    = "create_session"
	MsgTypeTerminateSession = "terminate_session"
	MsgTypeTerminalInput    = "terminal_input"
	MsgTypeTerminalResize   = "terminal_resize"
	MsgTypeSendPrompt       = "send_prompt"
)

// HeartbeatData represents heartbeat message data
type HeartbeatData struct {
	Sessions      []HeartbeatSession `json:"sessions"`
	RunnerVersion string             `json:"runner_version,omitempty"`
}

// SessionCreatedData represents session creation event data
type SessionCreatedData struct {
	SessionID    string `json:"session_id"`
	Pid          int    `json:"pid"`
	BranchName   string `json:"branch_name,omitempty"`
	WorktreePath string `json:"worktree_path,omitempty"`
	Cols         int    `json:"cols,omitempty"`
	Rows         int    `json:"rows,omitempty"`
}

// SessionTerminatedData represents session termination event data
type SessionTerminatedData struct {
	SessionID string `json:"session_id"`
	ExitCode  int    `json:"exit_code,omitempty"`
}

// TerminalOutputData represents terminal output data
type TerminalOutputData struct {
	SessionID string `json:"session_id"`
	Data      []byte `json:"data"`
}

// AgentStatusData represents agent status change data
type AgentStatusData struct {
	SessionID string `json:"session_id"`
	Status    string `json:"status"`
	Pid       int    `json:"pid,omitempty"`
}

// PtyResizedData represents PTY resize event data
type PtyResizedData struct {
	SessionID string `json:"session_id"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

// PreparationConfig contains workspace preparation configuration
type PreparationConfig struct {
	Script         string `json:"script,omitempty"`          // Shell script to execute
	TimeoutSeconds int    `json:"timeout_seconds,omitempty"` // Script execution timeout
}

// CreateSessionRequest represents a request to create a session
// Fields match Runner's client.CreateSessionRequest
type CreateSessionRequest struct {
	SessionID         string             `json:"session_id"`
	InitialCommand    string             `json:"initial_command,omitempty"`    // Command to run (e.g., "claude")
	InitialPrompt     string             `json:"initial_prompt,omitempty"`     // Prompt to send after command starts
	PermissionMode    string             `json:"permission_mode,omitempty"`    // Permission mode (plan/default)
	WorkingDir        string             `json:"working_dir,omitempty"`        // Working directory (deprecated, use PluginConfig)
	TicketIdentifier  string             `json:"ticket_identifier,omitempty"`  // For worktree creation (deprecated, use PluginConfig)
	WorktreeSuffix    string             `json:"worktree_suffix,omitempty"`    // Suffix for multiple worktrees per ticket
	EnvVars           map[string]string  `json:"env_vars,omitempty"`           // Environment variables (deprecated, use PluginConfig)
	PreparationConfig *PreparationConfig `json:"preparation_config,omitempty"` // Workspace preparation config (deprecated, use PluginConfig)

	// PluginConfig is the unified configuration passed to Runner's Sandbox plugins
	// Contains: repository_url, branch, ticket_identifier, git_token, init_script, init_timeout, env_vars
	PluginConfig map[string]interface{} `json:"plugin_config,omitempty"`
}

// TerminalInputRequest represents terminal input to send
type TerminalInputRequest struct {
	SessionID string `json:"session_id"`
	Data      []byte `json:"data"`
}

// TerminalResizeRequest represents terminal resize request
type TerminalResizeRequest struct {
	SessionID string `json:"session_id"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

// ConnectionManager manages runner WebSocket connections
type ConnectionManager struct {
	connections  map[int64]*RunnerConnection
	mu           sync.RWMutex
	logger       *slog.Logger
	pingInterval time.Duration
	pingTimeout  time.Duration

	// Event callbacks
	onHeartbeat         func(runnerID int64, data *HeartbeatData)
	onSessionCreated    func(runnerID int64, data *SessionCreatedData)
	onSessionTerminated func(runnerID int64, data *SessionTerminatedData)
	onTerminalOutput    func(runnerID int64, data *TerminalOutputData)
	onAgentStatus       func(runnerID int64, data *AgentStatusData)
	onPtyResized        func(runnerID int64, data *PtyResizedData)
	onDisconnect        func(runnerID int64)
}

// NewConnectionManager creates a new connection manager
func NewConnectionManager(logger *slog.Logger) *ConnectionManager {
	return &ConnectionManager{
		connections:  make(map[int64]*RunnerConnection),
		logger:       logger,
		pingInterval: 30 * time.Second,
		pingTimeout:  60 * time.Second,
	}
}

// SetHeartbeatCallback sets the heartbeat callback
func (cm *ConnectionManager) SetHeartbeatCallback(fn func(runnerID int64, data *HeartbeatData)) {
	cm.onHeartbeat = fn
}

// SetSessionCreatedCallback sets the session created callback
func (cm *ConnectionManager) SetSessionCreatedCallback(fn func(runnerID int64, data *SessionCreatedData)) {
	cm.onSessionCreated = fn
}

// SetSessionTerminatedCallback sets the session terminated callback
func (cm *ConnectionManager) SetSessionTerminatedCallback(fn func(runnerID int64, data *SessionTerminatedData)) {
	cm.onSessionTerminated = fn
}

// SetTerminalOutputCallback sets the terminal output callback
func (cm *ConnectionManager) SetTerminalOutputCallback(fn func(runnerID int64, data *TerminalOutputData)) {
	cm.onTerminalOutput = fn
}

// SetAgentStatusCallback sets the agent status callback
func (cm *ConnectionManager) SetAgentStatusCallback(fn func(runnerID int64, data *AgentStatusData)) {
	cm.onAgentStatus = fn
}

// SetPtyResizedCallback sets the PTY resized callback
func (cm *ConnectionManager) SetPtyResizedCallback(fn func(runnerID int64, data *PtyResizedData)) {
	cm.onPtyResized = fn
}

// SetDisconnectCallback sets the disconnect callback
func (cm *ConnectionManager) SetDisconnectCallback(fn func(runnerID int64)) {
	cm.onDisconnect = fn
}

// AddConnection adds a runner connection
func (cm *ConnectionManager) AddConnection(runnerID int64, conn *websocket.Conn) *RunnerConnection {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Close existing connection if any
	if existing, ok := cm.connections[runnerID]; ok {
		existing.Close()
	}

	rc := &RunnerConnection{
		RunnerID: runnerID,
		Conn:     conn,
		Send:     make(chan []byte, 256),
		LastPing: time.Now(),
	}

	cm.connections[runnerID] = rc
	cm.logger.Info("runner connected", "runner_id", runnerID)

	return rc
}

// RemoveConnection removes a runner connection
func (cm *ConnectionManager) RemoveConnection(runnerID int64) {
	cm.mu.Lock()
	conn, ok := cm.connections[runnerID]
	if ok {
		delete(cm.connections, runnerID)
	}
	cm.mu.Unlock()

	if ok {
		conn.Close()
		cm.logger.Info("runner disconnected", "runner_id", runnerID)

		if cm.onDisconnect != nil {
			cm.onDisconnect(runnerID)
		}
	}
}

// GetConnection returns a runner connection
func (cm *ConnectionManager) GetConnection(runnerID int64) *RunnerConnection {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.connections[runnerID]
}

// IsConnected checks if a runner is connected
func (cm *ConnectionManager) IsConnected(runnerID int64) bool {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	_, ok := cm.connections[runnerID]
	return ok
}

// UpdateHeartbeat updates the last ping time for a runner
func (cm *ConnectionManager) UpdateHeartbeat(runnerID int64) {
	cm.mu.RLock()
	conn, ok := cm.connections[runnerID]
	cm.mu.RUnlock()

	if ok {
		conn.mu.Lock()
		conn.LastPing = time.Now()
		conn.mu.Unlock()
	}
}

// HandleMessage handles an incoming message from a runner
func (cm *ConnectionManager) HandleMessage(runnerID int64, msgType int, data []byte) {
	if msgType != websocket.TextMessage && msgType != websocket.BinaryMessage {
		return
	}

	var msg RunnerMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		cm.logger.Warn("failed to parse runner message",
			"runner_id", runnerID,
			"error", err)
		return
	}

	switch msg.Type {
	case MsgTypeHeartbeat:
		var hbData HeartbeatData
		if err := json.Unmarshal(msg.Data, &hbData); err == nil {
			cm.UpdateHeartbeat(runnerID)
			if cm.onHeartbeat != nil {
				cm.onHeartbeat(runnerID, &hbData)
			}
		}

	case MsgTypeSessionCreated:
		var scData SessionCreatedData
		if err := json.Unmarshal(msg.Data, &scData); err == nil {
			if cm.onSessionCreated != nil {
				cm.onSessionCreated(runnerID, &scData)
			}
		}

	case MsgTypeSessionTerminated:
		var stData SessionTerminatedData
		if err := json.Unmarshal(msg.Data, &stData); err == nil {
			if cm.onSessionTerminated != nil {
				cm.onSessionTerminated(runnerID, &stData)
			}
		}

	case MsgTypeTerminalOutput:
		var toData TerminalOutputData
		if err := json.Unmarshal(msg.Data, &toData); err == nil {
			if cm.onTerminalOutput != nil {
				cm.onTerminalOutput(runnerID, &toData)
			}
		}

	case MsgTypeAgentStatus:
		var asData AgentStatusData
		if err := json.Unmarshal(msg.Data, &asData); err == nil {
			if cm.onAgentStatus != nil {
				cm.onAgentStatus(runnerID, &asData)
			}
		}

	case MsgTypePtyResized:
		var prData PtyResizedData
		if err := json.Unmarshal(msg.Data, &prData); err == nil {
			if cm.onPtyResized != nil {
				cm.onPtyResized(runnerID, &prData)
			}
		}

	default:
		cm.logger.Debug("unknown message type",
			"runner_id", runnerID,
			"type", msg.Type)
	}
}

// SendMessage sends a message to a runner
func (cm *ConnectionManager) SendMessage(ctx context.Context, runnerID int64, msg *RunnerMessage) error {
	cm.mu.RLock()
	conn, ok := cm.connections[runnerID]
	cm.mu.RUnlock()

	if !ok {
		return ErrRunnerNotConnected
	}

	return conn.SendMessage(msg)
}

// SendCreateSession sends a create session request to a runner
func (cm *ConnectionManager) SendCreateSession(ctx context.Context, runnerID int64, req *CreateSessionRequest) error {
	cm.logger.Info("sending create_session to runner",
		"runner_id", runnerID,
		"session_id", req.SessionID,
		"initial_command", req.InitialCommand,
		"permission_mode", req.PermissionMode)

	data, err := json.Marshal(req)
	if err != nil {
		cm.logger.Error("failed to marshal create_session request", "error", err)
		return err
	}

	err = cm.SendMessage(ctx, runnerID, &RunnerMessage{
		Type:      MsgTypeCreateSession,
		SessionID: req.SessionID,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	})

	if err != nil {
		cm.logger.Error("failed to send create_session to runner",
			"runner_id", runnerID,
			"session_id", req.SessionID,
			"error", err)
	} else {
		cm.logger.Info("create_session sent successfully",
			"runner_id", runnerID,
			"session_id", req.SessionID)
	}

	return err
}

// SendTerminateSession sends a terminate session request to a runner
func (cm *ConnectionManager) SendTerminateSession(ctx context.Context, runnerID int64, sessionID string) error {
	return cm.SendMessage(ctx, runnerID, &RunnerMessage{
		Type:      MsgTypeTerminateSession,
		SessionID: sessionID,
		Timestamp: time.Now().UnixMilli(),
	})
}

// SendTerminalInput sends terminal input to a runner
func (cm *ConnectionManager) SendTerminalInput(ctx context.Context, runnerID int64, sessionID string, data []byte) error {
	inputData, _ := json.Marshal(&TerminalInputRequest{
		SessionID: sessionID,
		Data:      data,
	})

	return cm.SendMessage(ctx, runnerID, &RunnerMessage{
		Type:      MsgTypeTerminalInput,
		SessionID: sessionID,
		Data:      inputData,
		Timestamp: time.Now().UnixMilli(),
	})
}

// SendTerminalResize sends terminal resize to a runner
func (cm *ConnectionManager) SendTerminalResize(ctx context.Context, runnerID int64, sessionID string, cols, rows int) error {
	resizeData, _ := json.Marshal(&TerminalResizeRequest{
		SessionID: sessionID,
		Cols:      cols,
		Rows:      rows,
	})

	return cm.SendMessage(ctx, runnerID, &RunnerMessage{
		Type:      MsgTypeTerminalResize,
		SessionID: sessionID,
		Data:      resizeData,
		Timestamp: time.Now().UnixMilli(),
	})
}

// SendPrompt sends a prompt to a session
func (cm *ConnectionManager) SendPrompt(ctx context.Context, runnerID int64, sessionID, prompt string) error {
	promptData, _ := json.Marshal(map[string]string{
		"session_id": sessionID,
		"prompt":     prompt,
	})

	return cm.SendMessage(ctx, runnerID, &RunnerMessage{
		Type:      MsgTypeSendPrompt,
		SessionID: sessionID,
		Data:      promptData,
		Timestamp: time.Now().UnixMilli(),
	})
}

// GetConnectedRunnerIDs returns IDs of all connected runners
func (cm *ConnectionManager) GetConnectedRunnerIDs() []int64 {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	ids := make([]int64, 0, len(cm.connections))
	for id := range cm.connections {
		ids = append(ids, id)
	}
	return ids
}

// Close closes the connection manager and all connections
func (cm *ConnectionManager) Close() {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	for _, conn := range cm.connections {
		conn.Close()
	}
	cm.connections = make(map[int64]*RunnerConnection)
}

// SendMessage sends a message on the connection
func (rc *RunnerConnection) SendMessage(msg *RunnerMessage) error {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.Conn == nil {
		return ErrConnectionClosed
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case rc.Send <- data:
		return nil
	default:
		return errors.New("send buffer full")
	}
}

// Close closes the connection
func (rc *RunnerConnection) Close() {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	if rc.Conn != nil {
		rc.Conn.Close()
		rc.Conn = nil
	}

	// Close send channel safely
	select {
	case <-rc.Send:
	default:
		close(rc.Send)
	}
}

// WritePump pumps messages from the send channel to the WebSocket
func (rc *RunnerConnection) WritePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		rc.Close()
	}()

	for {
		select {
		case message, ok := <-rc.Send:
			rc.mu.Lock()
			conn := rc.Conn
			rc.mu.Unlock()

			if conn == nil {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}

		case <-ticker.C:
			rc.mu.Lock()
			conn := rc.Conn
			rc.mu.Unlock()

			if conn == nil {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
