package runner

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/client"
	"github.com/anthropics/agentmesh/runner/internal/config"
	"github.com/anthropics/agentmesh/runner/internal/terminal"
	"github.com/anthropics/agentmesh/runner/internal/workspace"
)

// errorEventSenderStatus represents a captured status event
type errorEventSenderStatus struct {
	sessionKey string
	status     string
	data       map[string]interface{}
}

// errorEventSender is a mock that returns errors for SendTerminalOutput
type errorEventSender struct {
	statuses    []errorEventSenderStatus
	outputError error
	mu          sync.Mutex
}

func newErrorEventSender(err error) *errorEventSender {
	return &errorEventSender{outputError: err}
}

func (m *errorEventSender) SendSessionStatus(sessionKey, status string, data map[string]interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statuses = append(m.statuses, errorEventSenderStatus{sessionKey, status, data})
}

func (m *errorEventSender) SendTerminalOutput(sessionKey string, data []byte) error {
	return m.outputError
}

// --- Test handleSessionStart with initial prompt ---

func TestSessionHandlerStartWithInitialPrompt(t *testing.T) {
	tempDir := t.TempDir()
	ws, err := workspace.NewManager(tempDir, "")
	if err != nil {
		t.Skipf("Could not create workspace manager: %v", err)
	}

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         tempDir,
		},
		workspace: ws,
	}
	termManager := terminal.NewManager("/bin/sh", tempDir)
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionStartPayload{
		SessionKey:    "prompt-session",
		AgentType:     "claude-code",
		LaunchCommand: "cat",
		Rows:          24,
		Cols:          80,
		InitialPrompt: "Hello, world!",
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionStart,
		Payload: payloadBytes,
	}

	err = handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Wait for initial prompt to be sent (500ms + buffer)
	time.Sleep(700 * time.Millisecond)

	// Clean up
	session, ok := store.Get("prompt-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}
}

// --- Test OnOutput error handling ---

func TestSessionHandlerOnOutputErrorPath(t *testing.T) {
	tempDir := t.TempDir()
	ws, err := workspace.NewManager(tempDir, "")
	if err != nil {
		t.Skipf("Could not create workspace manager: %v", err)
	}

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         tempDir,
		},
		workspace: ws,
	}
	termManager := terminal.NewManager("/bin/sh", tempDir)
	eventSender := newErrorEventSender(errors.New("send output failed"))
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionStartPayload{
		SessionKey:    "error-output-session",
		AgentType:     "claude-code",
		LaunchCommand: "echo",
		LaunchArgs:    []string{"test output"},
		Rows:          24,
		Cols:          80,
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionStart,
		Payload: payloadBytes,
	}

	err = handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	// Clean up
	session, ok := store.Get("error-output-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}
}

// --- Test handleSessionStop with worktree cleanup error ---

func TestSessionHandlerStopWithWorktreeCleanupError(t *testing.T) {
	tempDir := t.TempDir()
	ws, err := workspace.NewManager(tempDir, "")
	if err != nil {
		t.Skipf("Could not create workspace manager: %v", err)
	}

	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: tempDir,
		},
		workspace: ws,
	}
	termManager := terminal.NewManager("/bin/sh", tempDir)
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	// Add session with non-existent worktree path
	store.Put("cleanup-error-session", &Session{
		ID:           "cleanup-error-session",
		SessionKey:   "cleanup-error-session",
		WorktreePath: "/nonexistent/worktree/path",
		Terminal:     nil,
	})

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionStopPayload{SessionKey: "cleanup-error-session"}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionStop,
		Payload: payloadBytes,
	}

	err = handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if store.Count() != 0 {
		t.Errorf("session count = %d, want 0", store.Count())
	}
}

// --- Test handleSessionStart capacity reached ---

func TestSessionHandlerStartMaxCapacity(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 1,
			WorkspaceRoot:         "/tmp",
		},
	}
	termManager := terminal.NewManager("/bin/sh", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	// Add existing session to reach capacity
	store.Put("existing-session", &Session{ID: "existing-session"})

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionStartPayload{
		SessionKey:    "overflow-session",
		AgentType:     "claude-code",
		LaunchCommand: "echo",
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionStart,
		Payload: payloadBytes,
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Error("expected error for max capacity")
	}

	// Verify error status was sent
	eventSender.mu.Lock()
	hasErrorStatus := false
	for _, s := range eventSender.statuses {
		if s.status == "error" {
			hasErrorStatus = true
		}
	}
	eventSender.mu.Unlock()

	if !hasErrorStatus {
		t.Error("should have sent error status")
	}
}

// --- Test handleSessionList ---

func TestSessionHandlerListEmpty(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/sh", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionListPayload{RequestID: "empty-list-req"}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionList,
		Payload: payloadBytes,
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	eventSender.mu.Lock()
	statusCount := len(eventSender.statuses)
	eventSender.mu.Unlock()

	if statusCount != 1 {
		t.Errorf("status count = %d, want 1", statusCount)
	}
}

func TestSessionHandlerListMultipleSessions(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/sh", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	// Add multiple sessions
	store.Put("session-1", &Session{
		ID:         "session-1",
		SessionKey: "session-1",
		AgentType:  "claude-code",
		Status:     SessionStatusRunning,
		StartedAt:  time.Now(),
	})
	store.Put("session-2", &Session{
		ID:            "session-2",
		SessionKey:    "session-2",
		AgentType:     "aider",
		Status:        SessionStatusRunning,
		StartedAt:     time.Now(),
		WorktreePath:  "/some/path",
		RepositoryURL: "https://github.com/test/repo",
	})

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionListPayload{RequestID: "multi-list-req"}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionList,
		Payload: payloadBytes,
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test handleSessionExit ---

func TestSessionHandlerExitRemovesSession(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/sh", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	// Add session
	store.Put("exit-session", &Session{
		ID:     "exit-session",
		Status: SessionStatusRunning,
	})

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	// Trigger exit handler
	handler.handleSessionExit("exit-session", 0)

	// Verify session was removed
	_, exists := store.Get("exit-session")
	if exists {
		t.Error("session should be removed after exit")
	}

	// Verify exit status was sent
	eventSender.mu.Lock()
	hasExitStatus := false
	for _, s := range eventSender.statuses {
		if s.status == "exited" {
			hasExitStatus = true
		}
	}
	eventSender.mu.Unlock()

	if !hasExitStatus {
		t.Error("should have sent exited status")
	}
}

func TestSessionHandlerExitNonExistentSession(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/sh", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	// Should not panic for nonexistent session
	handler.handleSessionExit("nonexistent-session", 1)
}

// --- Test concurrent session operations ---

func TestSessionHandlerConcurrentSessions(t *testing.T) {
	tempDir := t.TempDir()
	ws, err := workspace.NewManager(tempDir, "")
	if err != nil {
		t.Skipf("Could not create workspace manager: %v", err)
	}

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 100,
			WorkspaceRoot:         tempDir,
		},
		workspace: ws,
	}
	termManager := terminal.NewManager("/bin/sh", tempDir)
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	var wg sync.WaitGroup
	numSessions := 5

	// Start multiple sessions concurrently
	for i := 0; i < numSessions; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			payload := SessionStartPayload{
				SessionKey:    "concurrent-" + string(rune('A'+idx)),
				AgentType:     "claude-code",
				LaunchCommand: "sleep",
				LaunchArgs:    []string{"0.1"},
				Rows:          24,
				Cols:          80,
			}
			payloadBytes, _ := json.Marshal(payload)

			msg := &client.Message{
				Type:    client.MessageTypeSessionStart,
				Payload: payloadBytes,
			}

			handler.HandleMessage(context.Background(), msg)
		}(i)
	}

	wg.Wait()
	time.Sleep(50 * time.Millisecond)

	t.Logf("Created %d concurrent sessions", store.Count())

	// Clean up all sessions
	sessions := store.All()
	for _, session := range sessions {
		if session.Terminal != nil {
			session.Terminal.Stop()
		}
	}

	time.Sleep(200 * time.Millisecond)
}
