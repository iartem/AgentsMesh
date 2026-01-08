package runner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/client"
	"github.com/anthropics/agentmesh/runner/internal/terminal"
)

// RunnerMessageHandler implements client.MessageHandler interface.
// It bridges the client protocol layer with the runner session management.
type RunnerMessageHandler struct {
	runner       *Runner
	sessionStore SessionStore
	conn         client.Connection
}

// NewRunnerMessageHandler creates a new message handler.
func NewRunnerMessageHandler(runner *Runner, store SessionStore, conn client.Connection) *RunnerMessageHandler {
	return &RunnerMessageHandler{
		runner:       runner,
		sessionStore: store,
		conn:         conn,
	}
}

// OnCreateSession handles create session requests from server.
// Implements client.MessageHandler interface.
func (h *RunnerMessageHandler) OnCreateSession(req client.CreateSessionRequest) error {
	log.Printf("[message_handler] Creating session: session_id=%s, command=%s, permission_mode=%s",
		req.SessionID, req.InitialCommand, req.PermissionMode)

	ctx := context.Background()

	// Check capacity
	if h.runner.cfg.MaxConcurrentSessions > 0 && h.sessionStore.Count() >= h.runner.cfg.MaxConcurrentSessions {
		h.sendSessionError(req.SessionID, "max concurrent sessions reached")
		return fmt.Errorf("max concurrent sessions reached")
	}

	// Determine working directory
	workDir := req.WorkingDir
	var worktreePath, branchName string

	// Create worktree if ticket identifier is specified
	if req.TicketIdentifier != "" && h.runner.worktreeService != nil {
		suffix := req.WorktreeSuffix
		if suffix == "" {
			suffix = "default"
		}
		path, branch, err := h.runner.worktreeService.Create(req.TicketIdentifier, suffix)
		if err != nil {
			h.sendSessionError(req.SessionID, fmt.Sprintf("failed to create worktree: %v", err))
			return fmt.Errorf("failed to create worktree: %w", err)
		}
		workDir = path
		worktreePath = path
		branchName = branch
		log.Printf("[message_handler] Created worktree: path=%s, branch=%s", path, branch)
	}

	// Run preparation script if specified
	if req.PreparationConfig != nil && req.PreparationConfig.Script != "" {
		timeout := req.PreparationConfig.TimeoutSeconds
		if timeout <= 0 {
			timeout = 300 // Default 5 minutes
		}
		log.Printf("[message_handler] Running preparation script (timeout=%ds)", timeout)
		if err := h.runPreparationScript(ctx, workDir, req.PreparationConfig.Script, timeout); err != nil {
			h.sendSessionError(req.SessionID, fmt.Sprintf("preparation script failed: %v", err))
			return fmt.Errorf("preparation script failed: %w", err)
		}
	}

	// Merge environment variables
	envVars := h.mergeEnvVars(req.EnvVars)

	// Create terminal options
	termOpts := terminal.Options{
		Command:  req.InitialCommand,
		WorkDir:  workDir,
		Env:      envVars,
		Rows:     24,
		Cols:     80,
		OnOutput: h.createOutputHandler(req.SessionID),
		OnExit:   h.createExitHandler(req.SessionID),
	}

	// Create terminal
	term, err := terminal.New(termOpts)
	if err != nil {
		h.sendSessionError(req.SessionID, fmt.Sprintf("failed to create terminal: %v", err))
		return fmt.Errorf("failed to create terminal: %w", err)
	}

	// Create session
	session := &Session{
		ID:               req.SessionID,
		SessionKey:       req.SessionID,
		WorktreePath:     worktreePath,
		InitialPrompt:    req.InitialPrompt,
		Terminal:         term,
		StartedAt:        time.Now(),
		Status:           SessionStatusInitializing,
		TicketIdentifier: req.TicketIdentifier,
	}

	// Start terminal
	if err := term.Start(); err != nil {
		h.sendSessionError(req.SessionID, fmt.Sprintf("failed to start terminal: %v", err))
		return fmt.Errorf("failed to start terminal: %w", err)
	}

	// Store session
	h.sessionStore.Put(req.SessionID, session)
	session.Status = SessionStatusRunning

	// Send Shift+Tab if plan mode is requested
	if req.PermissionMode == "plan" {
		time.AfterFunc(1*time.Second, func() {
			// Shift+Tab escape sequence
			if err := term.Write([]byte("\x1b[Z")); err != nil {
				log.Printf("[message_handler] Failed to send Shift+Tab: %v", err)
			}
		})
	}

	// Send initial prompt if specified
	if req.InitialPrompt != "" {
		delay := 1500 * time.Millisecond
		if req.PermissionMode == "plan" {
			delay = 2500 * time.Millisecond // Give time for plan mode to activate
		}
		time.AfterFunc(delay, func() {
			if err := term.Write([]byte(req.InitialPrompt + "\n")); err != nil {
				log.Printf("[message_handler] Failed to send initial prompt: %v", err)
			}
		})
	}

	// Notify server that session is created
	// Use default PTY size since Terminal doesn't track current size
	h.sendSessionCreated(req.SessionID, term.PID(), worktreePath, branchName, 80, 24)

	log.Printf("[message_handler] Session created: session_id=%s, pid=%d", req.SessionID, term.PID())
	return nil
}

// OnTerminateSession handles terminate session requests from server.
// Implements client.MessageHandler interface.
func (h *RunnerMessageHandler) OnTerminateSession(req client.TerminateSessionRequest) error {
	log.Printf("[message_handler] Terminating session: session_id=%s", req.SessionID)

	session := h.sessionStore.Delete(req.SessionID)
	if session == nil {
		return fmt.Errorf("session not found: %s", req.SessionID)
	}

	// Stop terminal
	if session.Terminal != nil {
		session.Terminal.Stop()
	}

	// Clean up worktree if applicable
	if session.WorktreePath != "" && h.runner.worktreeService != nil {
		// Note: We might want to keep worktrees for ticket-based sessions
		log.Printf("[message_handler] Worktree preserved: %s", session.WorktreePath)
	}

	// Notify server
	h.sendSessionTerminated(req.SessionID)

	log.Printf("[message_handler] Session terminated: session_id=%s", req.SessionID)
	return nil
}

// OnListSessions returns current sessions.
// Implements client.MessageHandler interface.
func (h *RunnerMessageHandler) OnListSessions() []client.SessionInfo {
	sessions := h.sessionStore.All()
	result := make([]client.SessionInfo, 0, len(sessions))

	for _, s := range sessions {
		info := client.SessionInfo{
			SessionID:    s.SessionKey,
			Status:       s.Status,
			ClaudeStatus: "", // TODO: Get from Claude monitor
		}
		if s.Terminal != nil {
			info.Pid = s.Terminal.PID()
		}
		result = append(result, info)
	}

	return result
}

// OnTerminalInput handles terminal input from server.
// Implements client.MessageHandler interface.
func (h *RunnerMessageHandler) OnTerminalInput(req client.TerminalInputRequest) error {
	session, ok := h.sessionStore.Get(req.SessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", req.SessionID)
	}

	// Decode base64 data
	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		return fmt.Errorf("failed to decode terminal input: %w", err)
	}

	return session.Terminal.Write(data)
}

// OnTerminalResize handles terminal resize requests from server.
// Implements client.MessageHandler interface.
func (h *RunnerMessageHandler) OnTerminalResize(req client.TerminalResizeRequest) error {
	session, ok := h.sessionStore.Get(req.SessionID)
	if !ok {
		return fmt.Errorf("session not found: %s", req.SessionID)
	}

	if err := session.Terminal.Resize(int(req.Rows), int(req.Cols)); err != nil {
		return err
	}

	// Notify server of resize
	h.sendPtyResized(req.SessionID, req.Cols, req.Rows)
	return nil
}

// Helper methods

func (h *RunnerMessageHandler) createOutputHandler(sessionID string) func([]byte) {
	return func(data []byte) {
		h.sendTerminalOutput(sessionID, data)
	}
}

func (h *RunnerMessageHandler) createExitHandler(sessionID string) func(int) {
	return func(exitCode int) {
		log.Printf("[message_handler] Session exited: session_id=%s, exit_code=%d", sessionID, exitCode)

		session := h.sessionStore.Delete(sessionID)
		if session != nil {
			session.Status = SessionStatusStopped
		}

		h.sendSessionTerminated(sessionID)
	}
}

func (h *RunnerMessageHandler) mergeEnvVars(envVars map[string]string) map[string]string {
	result := make(map[string]string)

	// Add config env vars first
	for k, v := range h.runner.cfg.AgentEnvVars {
		result[k] = v
	}

	// Override with session-specific env vars
	for k, v := range envVars {
		result[k] = v
	}

	return result
}

func (h *RunnerMessageHandler) runPreparationScript(ctx context.Context, workDir, script string, timeout int) error {
	// TODO: Implement preparation script execution
	log.Printf("[message_handler] Preparation script execution not implemented")
	return nil
}

// Event sending methods

func (h *RunnerMessageHandler) sendSessionCreated(sessionID string, pid int, worktreePath, branchName string, cols, rows uint16) {
	if h.conn == nil {
		return
	}
	event := client.SessionCreatedEvent{
		SessionID:    sessionID,
		Pid:          pid,
		WorktreePath: worktreePath,
		BranchName:   branchName,
		PtyCols:      cols,
		PtyRows:      rows,
	}
	if err := h.conn.SendEvent(client.MsgTypeSessionCreated, event); err != nil {
		log.Printf("[message_handler] Failed to send session created event: %v", err)
	}
}

func (h *RunnerMessageHandler) sendSessionTerminated(sessionID string) {
	if h.conn == nil {
		return
	}
	event := client.SessionTerminatedEvent{
		SessionID: sessionID,
	}
	if err := h.conn.SendEvent(client.MsgTypeSessionTerminated, event); err != nil {
		log.Printf("[message_handler] Failed to send session terminated event: %v", err)
	}
}

func (h *RunnerMessageHandler) sendTerminalOutput(sessionID string, data []byte) {
	if h.conn == nil {
		return
	}
	event := client.TerminalOutputEvent{
		SessionID: sessionID,
		Data:      base64.StdEncoding.EncodeToString(data),
	}
	// Use backpressure for terminal output to ensure no data loss
	msg := client.ProtocolMessage{
		Type: client.MsgTypeTerminalOutput,
	}
	msgData, _ := json.Marshal(event)
	msg.Data = msgData
	h.conn.SendWithBackpressure(msg)
}

func (h *RunnerMessageHandler) sendPtyResized(sessionID string, cols, rows uint16) {
	if h.conn == nil {
		return
	}
	event := client.PtyResizedEvent{
		SessionID: sessionID,
		Cols:      cols,
		Rows:      rows,
	}
	if err := h.conn.SendEvent(client.MsgTypePtyResized, event); err != nil {
		log.Printf("[message_handler] Failed to send pty resized event: %v", err)
	}
}

func (h *RunnerMessageHandler) sendSessionError(sessionID, errorMsg string) {
	if h.conn == nil {
		return
	}
	event := map[string]interface{}{
		"session_id": sessionID,
		"error":      errorMsg,
	}
	if err := h.conn.SendEvent(client.MessageType("error"), event); err != nil {
		log.Printf("[message_handler] Failed to send error event: %v", err)
	}
}

// Ensure RunnerMessageHandler implements client.MessageHandler
var _ client.MessageHandler = (*RunnerMessageHandler)(nil)
