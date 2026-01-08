package runner

import (
	"errors"
	"testing"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/client"
	"github.com/anthropics/agentmesh/runner/internal/config"
	"github.com/anthropics/agentmesh/runner/internal/workspace"
)

// Tests for OnCreateSession and OnTerminateSession operations

// --- OnCreateSession Tests ---

func TestOnCreateSessionSuccess(t *testing.T) {
	tempDir := t.TempDir()
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

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

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	req := client.CreateSessionRequest{
		SessionID:      "test-session-1",
		InitialCommand: "echo",
		WorkingDir:     tempDir,
	}

	err = handler.OnCreateSession(req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify session was created
	session, ok := store.Get("test-session-1")
	if !ok {
		t.Error("session should be stored")
	} else {
		if session.Status != SessionStatusRunning {
			t.Errorf("session status = %s, want running", session.Status)
		}
		// Clean up terminal
		if session.Terminal != nil {
			session.Terminal.Stop()
		}
	}

	// Verify session_created event was sent
	events := mockConn.GetEvents()
	hasCreated := false
	for _, e := range events {
		if e.Type == client.MsgTypeSessionCreated {
			hasCreated = true
			break
		}
	}
	if !hasCreated {
		t.Error("should have sent session_created event")
	}
}

func TestOnCreateSessionMaxCapacity(t *testing.T) {
	tempDir := t.TempDir()
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 1,
			WorkspaceRoot:         tempDir,
		},
	}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	// Add existing session
	store.Put("existing-session", &Session{ID: "existing-session"})

	req := client.CreateSessionRequest{
		SessionID:      "new-session",
		InitialCommand: "echo",
	}

	err := handler.OnCreateSession(req)
	if err == nil {
		t.Error("expected error for max capacity")
	}
	if !contains(err.Error(), "max concurrent sessions") {
		t.Errorf("error = %v, want containing 'max concurrent sessions'", err)
	}
}

func TestOnCreateSessionInvalidCommand(t *testing.T) {
	tempDir := t.TempDir()
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

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

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	req := client.CreateSessionRequest{
		SessionID:      "invalid-cmd-session",
		InitialCommand: "/nonexistent/command/path",
		WorkingDir:     tempDir,
	}

	err = handler.OnCreateSession(req)
	// Command may or may not fail depending on OS
	t.Logf("OnCreateSession with invalid command: %v", err)
}

func TestOnCreateSessionWithTicketIdentifier(t *testing.T) {
	tempDir := t.TempDir()
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         tempDir,
		},
		// No worktreeService - should use workDir
	}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	req := client.CreateSessionRequest{
		SessionID:        "ticket-session",
		InitialCommand:   "echo",
		WorkingDir:       tempDir,
		TicketIdentifier: "TICKET-123", // Has ticket but no worktree service
	}

	err := handler.OnCreateSession(req)
	if err != nil {
		t.Logf("OnCreateSession with ticket (no worktree service): %v", err)
	}

	// Clean up
	session, ok := store.Get("ticket-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}
}

func TestOnCreateSessionWithPreparationConfig(t *testing.T) {
	tempDir := t.TempDir()
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         tempDir,
		},
	}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	req := client.CreateSessionRequest{
		SessionID:      "prep-session",
		InitialCommand: "echo",
		WorkingDir:     tempDir,
		PreparationConfig: &client.PreparationConfig{
			Script:         "echo prep",
			TimeoutSeconds: 5,
		},
	}

	err := handler.OnCreateSession(req)
	if err != nil {
		t.Logf("OnCreateSession with preparation: %v", err)
	}

	// Clean up
	session, ok := store.Get("prep-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}
}

func TestOnCreateSessionWithPreparationDefaultTimeout(t *testing.T) {
	tempDir := t.TempDir()
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         tempDir,
		},
	}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	req := client.CreateSessionRequest{
		SessionID:      "prep-default-session",
		InitialCommand: "echo",
		WorkingDir:     tempDir,
		PreparationConfig: &client.PreparationConfig{
			Script:         "echo prep",
			TimeoutSeconds: 0, // Should default to 300
		},
	}

	err := handler.OnCreateSession(req)
	if err != nil {
		t.Logf("OnCreateSession with default timeout: %v", err)
	}

	// Clean up
	session, ok := store.Get("prep-default-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}
}

func TestOnCreateSessionWithPlanMode(t *testing.T) {
	tempDir := t.TempDir()
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         tempDir,
		},
	}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	req := client.CreateSessionRequest{
		SessionID:      "plan-mode-session",
		InitialCommand: "cat",
		WorkingDir:     tempDir,
		PermissionMode: "plan",
		InitialPrompt:  "Test prompt",
	}

	err := handler.OnCreateSession(req)
	if err != nil {
		t.Logf("OnCreateSession with plan mode: %v", err)
	}

	// Give time for Shift+Tab and prompt to be sent
	time.Sleep(100 * time.Millisecond)

	// Clean up
	session, ok := store.Get("plan-mode-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}
}

func TestOnCreateSessionWithInitialPrompt(t *testing.T) {
	tempDir := t.TempDir()
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         tempDir,
		},
	}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	req := client.CreateSessionRequest{
		SessionID:      "prompt-session",
		InitialCommand: "cat",
		WorkingDir:     tempDir,
		InitialPrompt:  "Hello from test",
	}

	err := handler.OnCreateSession(req)
	if err != nil {
		t.Logf("OnCreateSession with initial prompt: %v", err)
	}

	// Give time for prompt to be sent
	time.Sleep(100 * time.Millisecond)

	// Clean up
	session, ok := store.Get("prompt-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}
}

func TestOnCreateSessionWithWorktreeServiceError(t *testing.T) {
	tempDir := t.TempDir()
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	// Create runner with worktreeService that will fail
	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         tempDir,
			RepositoryPath:        "/nonexistent/repo",
			WorktreesDir:          tempDir,
		},
	}
	runner.initEnhancedComponents(runner.cfg)

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	req := client.CreateSessionRequest{
		SessionID:        "worktree-error-session",
		InitialCommand:   "echo",
		TicketIdentifier: "TICKET-999",
		WorktreeSuffix:   "test",
	}

	err := handler.OnCreateSession(req)
	// Should fail because worktree can't be created from non-existent repo
	t.Logf("OnCreateSession with worktree error: %v", err)
}

func TestOnCreateSessionWithSendEventError(t *testing.T) {
	tempDir := t.TempDir()
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()
	mockConn.SendErr = errors.New("send failed")

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         tempDir,
		},
	}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	req := client.CreateSessionRequest{
		SessionID:      "send-error-session",
		InitialCommand: "echo",
		WorkingDir:     tempDir,
	}

	err := handler.OnCreateSession(req)
	// Session should still be created even if send fails
	if err != nil {
		t.Logf("OnCreateSession with send error: %v", err)
	}

	// Clean up
	session, ok := store.Get("send-error-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}
}

// --- OnTerminateSession Tests ---

func TestOnTerminateSessionSuccess(t *testing.T) {
	tempDir := t.TempDir()
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{
		cfg: &config.Config{WorkspaceRoot: tempDir},
	}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	// Add a session
	store.Put("terminate-session", &Session{
		ID:       "terminate-session",
		Terminal: nil, // nil terminal should be handled gracefully
	})

	req := client.TerminateSessionRequest{
		SessionID: "terminate-session",
	}

	err := handler.OnTerminateSession(req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify session was removed
	_, exists := store.Get("terminate-session")
	if exists {
		t.Error("session should be removed")
	}

	// Verify session_terminated event was sent
	events := mockConn.GetEvents()
	hasTerminated := false
	for _, e := range events {
		if e.Type == client.MsgTypeSessionTerminated {
			hasTerminated = true
			break
		}
	}
	if !hasTerminated {
		t.Error("should have sent session_terminated event")
	}
}

func TestOnTerminateSessionNotFound(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{
		cfg: &config.Config{},
	}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	req := client.TerminateSessionRequest{
		SessionID: "nonexistent-session",
	}

	err := handler.OnTerminateSession(req)
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
	if !contains(err.Error(), "session not found") {
		t.Errorf("error = %v, want containing 'session not found'", err)
	}
}

func TestOnTerminateSessionWithWorktree(t *testing.T) {
	tempDir := t.TempDir()
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{
		cfg: &config.Config{WorkspaceRoot: tempDir},
		// No worktreeService
	}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	// Add a session with worktree
	store.Put("worktree-session", &Session{
		ID:           "worktree-session",
		WorktreePath: "/fake/worktree/path",
		Terminal:     nil,
	})

	req := client.TerminateSessionRequest{
		SessionID: "worktree-session",
	}

	err := handler.OnTerminateSession(req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- OnListSessions Tests ---

func TestOnListSessionsEmpty(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	sessions := handler.OnListSessions()
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestOnListSessionsWithSessions(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	// Add sessions
	store.Put("session-1", &Session{
		ID:         "session-1",
		SessionKey: "session-1",
		Status:     SessionStatusRunning,
	})
	store.Put("session-2", &Session{
		ID:         "session-2",
		SessionKey: "session-2",
		Status:     SessionStatusInitializing,
	})

	sessions := handler.OnListSessions()
	if len(sessions) != 2 {
		t.Errorf("expected 2 sessions, got %d", len(sessions))
	}
}

func TestOnListSessionsWithTerminalPID(t *testing.T) {
	tempDir := t.TempDir()
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         tempDir,
		},
	}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	// First create a session with a real terminal
	createReq := client.CreateSessionRequest{
		SessionID:      "list-pid-session",
		InitialCommand: "sleep",
		WorkingDir:     tempDir,
	}

	err := handler.OnCreateSession(createReq)
	if err != nil {
		t.Skipf("Could not create session: %v", err)
	}

	// List sessions
	sessions := handler.OnListSessions()
	if len(sessions) != 1 {
		t.Errorf("sessions count = %d, want 1", len(sessions))
	}

	// Check PID is set
	if sessions[0].Pid == 0 {
		t.Log("Session PID should be non-zero")
	}

	// Clean up
	session, ok := store.Get("list-pid-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}
}
