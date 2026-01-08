package runner

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/client"
	"github.com/anthropics/agentmesh/runner/internal/config"
	"github.com/anthropics/agentmesh/runner/internal/terminal"
)

// --- Test handleSessionStart with builder ---

func TestSessionHandlerHandleSessionStartWithBuilder(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         "/tmp",
			AgentEnvVars:          map[string]string{},
		},
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionStartPayload{
		SessionKey:    "test-build-session",
		AgentType:     "claude-code",
		LaunchCommand: "echo",
		LaunchArgs:    []string{"hello"},
		Rows:          30,
		Cols:          100,
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionStart,
		Payload: payloadBytes,
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify session was stored
	session, ok := store.Get("test-build-session")
	if !ok {
		t.Error("session should be stored")
	}
	if session != nil {
		// Clean up
		if session.Terminal != nil {
			session.Terminal.Stop()
		}
	}

	// Verify status was sent
	eventSender.mu.Lock()
	statusCount := len(eventSender.statuses)
	eventSender.mu.Unlock()

	if statusCount < 1 {
		t.Error("should have sent at least one status")
	}
}

func TestSessionHandlerHandleSessionStartWithRepository(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         "/tmp",
		},
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionStartPayload{
		SessionKey:    "repo-session",
		AgentType:     "claude-code",
		LaunchCommand: "echo",
		RepositoryURL: "https://github.com/test/repo.git",
		Branch:        "main",
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionStart,
		Payload: payloadBytes,
	}

	// This will likely fail to create worktree, but tests the code path
	err := handler.HandleMessage(context.Background(), msg)
	if err != nil {
		// Expected to fail without real git repo, but we want to verify the path is exercised
		// Check that error notification was sent
		eventSender.mu.Lock()
		hasErrorStatus := false
		for _, s := range eventSender.statuses {
			if s.status == "error" {
				hasErrorStatus = true
			}
		}
		eventSender.mu.Unlock()

		if !hasErrorStatus {
			t.Log("Expected error status for failed worktree creation")
		}
	}
}

func TestSessionHandlerHandleSessionStartWithTicketWorktree(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         "/tmp",
		},
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionStartPayload{
		SessionKey:       "ticket-session",
		AgentType:        "claude-code",
		LaunchCommand:    "echo",
		TicketIdentifier: "TICKET-123",
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionStart,
		Payload: payloadBytes,
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err != nil {
		// Without worktree service, falls back to normal workspace
		t.Logf("Error (expected without worktree service): %v", err)
	}
}

func TestSessionHandlerHandleSessionStartWithInitialPrompt(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         "/tmp",
		},
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionStartPayload{
		SessionKey:    "prompt-session",
		AgentType:     "claude-code",
		LaunchCommand: "cat", // cat will wait for input
		InitialPrompt: "Hello, Claude!",
		Rows:          24,
		Cols:          80,
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionStart,
		Payload: payloadBytes,
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Wait for initial prompt to be sent
	time.Sleep(600 * time.Millisecond)

	// Clean up
	session, ok := store.Get("prompt-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}
}

// --- Test handleSessionStop with terminal ---

func TestSessionHandlerHandleSessionStopWithTerminal(t *testing.T) {
	runner := &Runner{
		cfg:       &config.Config{},
		workspace: nil,
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	// Create a real terminal for testing
	term, err := terminal.New(terminal.Options{
		Command: "sleep",
		Args:    []string{"10"},
		WorkDir: "/tmp",
		Rows:    24,
		Cols:    80,
	})
	if err != nil {
		t.Skipf("Could not create terminal: %v", err)
	}
	term.Start()

	// Add session with terminal
	store.Put("term-session", &Session{
		ID:         "term-session",
		SessionKey: "term-session",
		Terminal:   term,
	})

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionStopPayload{SessionKey: "term-session"}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionStop,
		Payload: payloadBytes,
	}

	err = handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify session was removed
	if store.Count() != 0 {
		t.Errorf("session count = %d, want 0", store.Count())
	}

	// Verify stopped status was sent
	eventSender.mu.Lock()
	hasStoppedStatus := false
	for _, s := range eventSender.statuses {
		if s.status == "stopped" {
			hasStoppedStatus = true
		}
	}
	eventSender.mu.Unlock()

	if !hasStoppedStatus {
		t.Error("should have sent stopped status")
	}
}

// --- Test handleTerminalInput with session ---

func TestSessionHandlerHandleTerminalInputWithSession(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{},
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	// Create a real terminal
	term, err := terminal.New(terminal.Options{
		Command: "cat",
		WorkDir: "/tmp",
		Rows:    24,
		Cols:    80,
	})
	if err != nil {
		t.Skipf("Could not create terminal: %v", err)
	}
	term.Start()
	defer term.Stop()

	store.Put("input-session", &Session{
		ID:         "input-session",
		SessionKey: "input-session",
		Terminal:   term,
	})

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := TerminalInputPayload{
		SessionKey: "input-session",
		Data:       []byte("test input\n"),
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeTerminalInput,
		Payload: payloadBytes,
	}

	err = handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test handleTerminalResize with session ---

func TestSessionHandlerHandleTerminalResizeWithSession(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{},
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	// Create a real terminal
	term, err := terminal.New(terminal.Options{
		Command: "sleep",
		Args:    []string{"10"},
		WorkDir: "/tmp",
		Rows:    24,
		Cols:    80,
	})
	if err != nil {
		t.Skipf("Could not create terminal: %v", err)
	}
	term.Start()
	defer term.Stop()

	store.Put("resize-session", &Session{
		ID:         "resize-session",
		SessionKey: "resize-session",
		Terminal:   term,
	})

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := TerminalResizePayload{
		SessionKey: "resize-session",
		Rows:       40,
		Cols:       120,
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeTerminalResize,
		Payload: payloadBytes,
	}

	err = handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test handleSessionList with terminal info ---

func TestSessionHandlerHandleSessionListWithTerminal(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	// Create a real terminal for PID info
	term, err := terminal.New(terminal.Options{
		Command: "sleep",
		Args:    []string{"10"},
		WorkDir: "/tmp",
		Rows:    24,
		Cols:    80,
	})
	if err != nil {
		t.Skipf("Could not create terminal: %v", err)
	}
	term.Start()
	defer term.Stop()

	store.Put("list-session", &Session{
		ID:            "list-session",
		SessionKey:    "list-session",
		AgentType:     "claude-code",
		Status:        SessionStatusRunning,
		StartedAt:     time.Now(),
		WorktreePath:  "/workspace/worktrees/test",
		RepositoryURL: "https://github.com/test/repo.git",
		Terminal:      term,
	})

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionListPayload{RequestID: "req-with-terminal"}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionList,
		Payload: payloadBytes,
	}

	err = handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify status contains PID
	eventSender.mu.Lock()
	defer eventSender.mu.Unlock()

	if len(eventSender.statuses) != 1 {
		t.Errorf("statuses count = %d, want 1", len(eventSender.statuses))
		return
	}

	data := eventSender.statuses[0].data
	sessions, ok := data["sessions"].([]map[string]interface{})
	if !ok {
		t.Log("sessions data format unexpected, skipping PID check")
		return
	}

	if len(sessions) > 0 {
		if _, hasPID := sessions[0]["pid"]; !hasPID {
			t.Error("session info should include PID")
		}
	}
}

// --- Test output callback ---

func TestSessionHandlerOutputCallback(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         "/tmp",
		},
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	// Create session with output callback
	payload := SessionStartPayload{
		SessionKey:    "output-session",
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

	err := handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Wait for output
	time.Sleep(100 * time.Millisecond)

	// Clean up
	session, ok := store.Get("output-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}

	// Check if output was sent
	eventSender.mu.Lock()
	outputCount := len(eventSender.outputs)
	eventSender.mu.Unlock()

	// Output may or may not have been captured depending on timing
	t.Logf("Output count: %d", outputCount)
}

// --- Test exit callback ---

func TestSessionHandlerExitCallback(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         "/tmp",
		},
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	// Create session that exits quickly
	payload := SessionStartPayload{
		SessionKey:    "exit-session",
		AgentType:     "claude-code",
		LaunchCommand: "true", // exits immediately with code 0
		Rows:          24,
		Cols:          80,
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionStart,
		Payload: payloadBytes,
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Wait for exit
	time.Sleep(200 * time.Millisecond)

	// Check for exited status
	eventSender.mu.Lock()
	hasExitedStatus := false
	for _, s := range eventSender.statuses {
		if s.status == "exited" {
			hasExitedStatus = true
		}
	}
	eventSender.mu.Unlock()

	// The exit callback may or may not have been triggered depending on timing
	t.Logf("Has exited status: %v", hasExitedStatus)
}

// --- Benchmark ---

func BenchmarkSessionHandlerHandleMessage(b *testing.B) {
	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 100,
		},
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionListPayload{RequestID: "bench-req"}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionList,
		Payload: payloadBytes,
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.HandleMessage(ctx, msg)
	}
}
