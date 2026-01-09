package runner

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/client"
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
// Uses the Sandbox plugin system for environment configuration.
func (h *RunnerMessageHandler) OnCreateSession(req client.CreateSessionRequest) error {
	log.Printf("[message_handler] Creating session: session_id=%s, command=%s, permission_mode=%s, plugin_config=%v",
		req.SessionID, req.InitialCommand, req.PermissionMode, req.PluginConfig)

	ctx := context.Background()

	// Check capacity
	if h.runner.cfg.MaxConcurrentSessions > 0 && h.sessionStore.Count() >= h.runner.cfg.MaxConcurrentSessions {
		h.sendSessionError(req.SessionID, "max concurrent sessions reached")
		return fmt.Errorf("max concurrent sessions reached")
	}

	// Build PluginConfig from both legacy fields and new PluginConfig
	pluginConfig := h.buildPluginConfig(&req)

	// Use SessionBuilder with Sandbox mode
	builder := NewSessionBuilder(h.runner).
		WithSessionKey(req.SessionID).
		WithLaunchCommand(req.InitialCommand, nil).
		WithInitialPrompt(req.InitialPrompt).
		WithSandbox(pluginConfig)

	// Build session
	session, err := builder.Build(ctx)
	if err != nil {
		h.sendSessionError(req.SessionID, fmt.Sprintf("failed to build session: %v", err))
		return fmt.Errorf("failed to build session: %w", err)
	}

	// Set output/exit handlers
	session.Terminal.SetOutputHandler(h.createOutputHandler(req.SessionID))
	session.Terminal.SetExitHandler(h.createExitHandler(req.SessionID))

	// Start terminal
	if err := session.Terminal.Start(); err != nil {
		// Cleanup sandbox on failure
		if h.runner.sandboxManager != nil {
			h.runner.sandboxManager.Cleanup(req.SessionID)
		}
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
			if err := session.Terminal.Write([]byte("\x1b[Z")); err != nil {
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
			if err := session.Terminal.Write([]byte(req.InitialPrompt + "\n")); err != nil {
				log.Printf("[message_handler] Failed to send initial prompt: %v", err)
			}
		})
	}

	// Notify server that session is created
	h.sendSessionCreated(req.SessionID, session.Terminal.PID(), session.WorktreePath, session.Branch, 80, 24)

	log.Printf("[message_handler] Session created: session_id=%s, pid=%d, worktree=%s, branch=%s",
		req.SessionID, session.Terminal.PID(), session.WorktreePath, session.Branch)
	return nil
}

// buildPluginConfig merges legacy fields with PluginConfig for backward compatibility.
func (h *RunnerMessageHandler) buildPluginConfig(req *client.CreateSessionRequest) map[string]interface{} {
	config := make(map[string]interface{})

	// Copy legacy fields to PluginConfig for backward compatibility
	if req.TicketIdentifier != "" {
		config["ticket_identifier"] = req.TicketIdentifier
	}
	if req.WorkingDir != "" {
		config["working_dir"] = req.WorkingDir
	}
	if len(req.EnvVars) > 0 {
		envMap := make(map[string]interface{})
		for k, v := range req.EnvVars {
			envMap[k] = v
		}
		config["env_vars"] = envMap
	}
	if req.PreparationConfig != nil {
		if req.PreparationConfig.Script != "" {
			config["init_script"] = req.PreparationConfig.Script
		}
		if req.PreparationConfig.TimeoutSeconds > 0 {
			config["init_timeout"] = req.PreparationConfig.TimeoutSeconds
		}
	}

	// Merge PluginConfig (overrides legacy fields)
	for k, v := range req.PluginConfig {
		config[k] = v
	}

	return config
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

	// Clean up sandbox
	if h.runner.sandboxManager != nil {
		if err := h.runner.sandboxManager.Cleanup(req.SessionID); err != nil {
			log.Printf("[message_handler] Warning: failed to cleanup sandbox: %v", err)
		}
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
