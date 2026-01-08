package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/client"
	"github.com/anthropics/agentmesh/runner/internal/terminal"
)

// SessionHandler handles session-related commands from the server.
// Implements the Strategy pattern for different command handlers.
type SessionHandler struct {
	runner       *Runner
	termManager  *terminal.Manager
	eventSender  EventSender
	sessionStore SessionStore
}

// EventSender sends events to the server.
type EventSender interface {
	SendSessionStatus(sessionKey, status string, data map[string]interface{})
	SendTerminalOutput(sessionKey string, data []byte) error
}

// SessionStore manages session state.
type SessionStore interface {
	Get(sessionKey string) (*Session, bool)
	Put(sessionKey string, session *Session)
	Delete(sessionKey string) *Session
	Count() int
	All() []*Session
}

// NewSessionHandler creates a new session handler.
func NewSessionHandler(runner *Runner, termManager *terminal.Manager, eventSender EventSender, store SessionStore) *SessionHandler {
	return &SessionHandler{
		runner:       runner,
		termManager:  termManager,
		eventSender:  eventSender,
		sessionStore: store,
	}
}

// HandleMessage routes a message to the appropriate handler.
func (h *SessionHandler) HandleMessage(ctx context.Context, msg *client.Message) error {
	switch msg.Type {
	case client.MessageTypeSessionStart:
		return h.handleSessionStart(ctx, msg)
	case client.MessageTypeSessionStop:
		return h.handleSessionStop(ctx, msg)
	case client.MessageTypeTerminalInput:
		return h.handleTerminalInput(ctx, msg)
	case client.MessageTypeTerminalResize:
		return h.handleTerminalResize(ctx, msg)
	case client.MessageTypeSessionList:
		return h.handleSessionList(ctx, msg)
	default:
		log.Printf("[session_handler] Unknown message type: %s", msg.Type)
		return nil
	}
}

// handleSessionStart handles session start requests.
func (h *SessionHandler) handleSessionStart(ctx context.Context, msg *client.Message) error {
	var payload SessionStartPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("invalid session start payload: %w", err)
	}

	log.Printf("[session_handler] Starting session: session_key=%s, agent=%s",
		payload.SessionKey, payload.AgentType)

	// Check capacity
	if h.runner.cfg.MaxConcurrentSessions > 0 && h.sessionStore.Count() >= h.runner.cfg.MaxConcurrentSessions {
		h.sendSessionError(payload.SessionKey, "max concurrent sessions reached")
		return fmt.Errorf("max concurrent sessions reached")
	}

	// Build the session using SessionBuilder
	builder := NewSessionBuilder(h.runner).
		WithSessionKey(payload.SessionKey).
		WithAgentType(payload.AgentType).
		WithLaunchCommand(payload.LaunchCommand, payload.LaunchArgs).
		WithEnvVars(payload.EnvVars).
		WithTerminalSize(payload.Rows, payload.Cols).
		WithInitialPrompt(payload.InitialPrompt)

	// Configure repository if specified
	if payload.RepositoryURL != "" {
		builder.WithRepository(payload.RepositoryURL, payload.Branch)
	}

	// Configure worktree if ticket identifier is specified
	if payload.TicketIdentifier != "" {
		builder.WithWorktree(payload.TicketIdentifier)
	}

	// Build and start the session
	session, err := builder.Build(ctx)
	if err != nil {
		h.sendSessionError(payload.SessionKey, fmt.Sprintf("failed to build session: %v", err))
		return fmt.Errorf("failed to build session: %w", err)
	}

	// Set up output handler
	session.OnOutput = func(data []byte) {
		if err := h.eventSender.SendTerminalOutput(payload.SessionKey, data); err != nil {
			log.Printf("[session_handler] Failed to send terminal output: %v", err)
		}
	}

	// Set up exit handler
	session.OnExit = func(exitCode int) {
		h.handleSessionExit(payload.SessionKey, exitCode)
	}

	// Start the terminal
	if err := session.Terminal.Start(); err != nil {
		h.sendSessionError(payload.SessionKey, fmt.Sprintf("failed to start terminal: %v", err))
		return fmt.Errorf("failed to start terminal: %w", err)
	}

	// Store the session
	h.sessionStore.Put(payload.SessionKey, session)

	// Send initial prompt if specified
	if payload.InitialPrompt != "" {
		time.AfterFunc(500*time.Millisecond, func() {
			if err := session.Terminal.Write([]byte(payload.InitialPrompt + "\n")); err != nil {
				log.Printf("[session_handler] Failed to send initial prompt: %v", err)
			}
		})
	}

	// Notify server
	h.eventSender.SendSessionStatus(payload.SessionKey, "started", map[string]interface{}{
		"pid": session.Terminal.PID(),
	})

	log.Printf("[session_handler] Session started: session_key=%s, pid=%d",
		payload.SessionKey, session.Terminal.PID())

	return nil
}

// handleSessionStop handles session stop requests.
func (h *SessionHandler) handleSessionStop(ctx context.Context, msg *client.Message) error {
	var payload SessionStopPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("invalid session stop payload: %w", err)
	}

	log.Printf("[session_handler] Stopping session: session_key=%s", payload.SessionKey)

	session := h.sessionStore.Delete(payload.SessionKey)
	if session == nil {
		return fmt.Errorf("session not found: %s", payload.SessionKey)
	}

	// Stop terminal
	if session.Terminal != nil {
		session.Terminal.Stop()
	}

	// Clean up worktree if applicable
	if session.WorktreePath != "" && h.runner.workspace != nil {
		if err := h.runner.workspace.RemoveWorktree(ctx, session.WorktreePath); err != nil {
			log.Printf("[session_handler] Warning: failed to remove worktree: %v", err)
		}
	}

	// Notify server
	h.eventSender.SendSessionStatus(payload.SessionKey, "stopped", nil)

	log.Printf("[session_handler] Session stopped: session_key=%s", payload.SessionKey)
	return nil
}

// handleTerminalInput handles terminal input.
func (h *SessionHandler) handleTerminalInput(ctx context.Context, msg *client.Message) error {
	var payload TerminalInputPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("invalid terminal input payload: %w", err)
	}

	session, ok := h.sessionStore.Get(payload.SessionKey)
	if !ok {
		return fmt.Errorf("session not found: %s", payload.SessionKey)
	}

	return session.Terminal.Write(payload.Data)
}

// handleTerminalResize handles terminal resize requests.
func (h *SessionHandler) handleTerminalResize(ctx context.Context, msg *client.Message) error {
	var payload TerminalResizePayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("invalid terminal resize payload: %w", err)
	}

	session, ok := h.sessionStore.Get(payload.SessionKey)
	if !ok {
		return fmt.Errorf("session not found: %s", payload.SessionKey)
	}

	return session.Terminal.Resize(payload.Rows, payload.Cols)
}

// SessionListPayload represents the payload for session list request.
type SessionListPayload struct {
	RequestID string `json:"request_id"`
}

// handleSessionList handles session list requests.
func (h *SessionHandler) handleSessionList(ctx context.Context, msg *client.Message) error {
	var payload SessionListPayload
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("invalid session list payload: %w", err)
	}

	sessions := h.sessionStore.All()
	sessionInfos := make([]map[string]interface{}, 0, len(sessions))

	for _, s := range sessions {
		info := map[string]interface{}{
			"session_key":    s.SessionKey,
			"agent_type":     s.AgentType,
			"status":         s.Status,
			"started_at":     s.StartedAt.Format(time.RFC3339),
			"worktree_path":  s.WorktreePath,
			"repository_url": s.RepositoryURL,
		}
		if s.Terminal != nil {
			info["pid"] = s.Terminal.PID()
		}
		sessionInfos = append(sessionInfos, info)
	}

	h.eventSender.SendSessionStatus("", "session_list", map[string]interface{}{
		"request_id": payload.RequestID,
		"sessions":   sessionInfos,
	})

	return nil
}

// handleSessionExit handles terminal exit events.
func (h *SessionHandler) handleSessionExit(sessionKey string, exitCode int) {
	log.Printf("[session_handler] Session exited: session_key=%s, exit_code=%d",
		sessionKey, exitCode)

	session := h.sessionStore.Delete(sessionKey)
	if session != nil {
		session.Status = SessionStatusStopped
	}

	h.eventSender.SendSessionStatus(sessionKey, "exited", map[string]interface{}{
		"exit_code": exitCode,
	})
}

// sendSessionError sends a session error notification.
func (h *SessionHandler) sendSessionError(sessionKey, errorMsg string) {
	h.eventSender.SendSessionStatus(sessionKey, "error", map[string]interface{}{
		"error": errorMsg,
	})
}

// InMemorySessionStore is a simple in-memory session store.
type InMemorySessionStore struct {
	sessions map[string]*Session
	mu       sync.RWMutex
}

// NewInMemorySessionStore creates a new in-memory session store.
func NewInMemorySessionStore() *InMemorySessionStore {
	return &InMemorySessionStore{
		sessions: make(map[string]*Session),
	}
}

// Get retrieves a session by key.
func (s *InMemorySessionStore) Get(sessionKey string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[sessionKey]
	return session, ok
}

// Put stores a session.
func (s *InMemorySessionStore) Put(sessionKey string, session *Session) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[sessionKey] = session
}

// Delete removes and returns a session.
func (s *InMemorySessionStore) Delete(sessionKey string) *Session {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[sessionKey]
	if ok {
		delete(s.sessions, sessionKey)
	}
	return session
}

// Count returns the number of sessions.
func (s *InMemorySessionStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.sessions)
}

// All returns all sessions.
func (s *InMemorySessionStore) All() []*Session {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sessions := make([]*Session, 0, len(s.sessions))
	for _, session := range s.sessions {
		sessions = append(sessions, session)
	}
	return sessions
}

