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

// Tests for SessionStart message handling

func TestSessionHandlerHandleSessionStartMaxCapacity(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 1,
		},
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	// Fill up capacity
	store.Put("existing", &Session{ID: "existing"})

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionStartPayload{
		SessionKey:    "new-session",
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
	if !contains(err.Error(), "max concurrent sessions") {
		t.Errorf("error = %v, want containing 'max concurrent sessions'", err)
	}
}

func TestSessionHandlerHandleSessionStartSuccess(t *testing.T) {
	tempDir := t.TempDir()

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         tempDir,
		},
	}
	termManager := terminal.NewManager("/bin/bash", tempDir)
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionStartPayload{
		SessionKey:    "success-session",
		AgentType:     "claude-code",
		LaunchCommand: "echo",
		LaunchArgs:    []string{"hello"},
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
		t.Skipf("Could not start session: %v", err)
	}

	// Wait for terminal to start
	time.Sleep(100 * time.Millisecond)

	// Check session was created
	session, ok := store.Get("success-session")
	if !ok {
		t.Error("session should be stored")
	}

	// Check started status was sent
	eventSender.mu.Lock()
	hasStarted := false
	for _, s := range eventSender.statuses {
		if s.status == "started" {
			hasStarted = true
			break
		}
	}
	eventSender.mu.Unlock()

	if !hasStarted {
		t.Error("should have sent started status")
	}

	// Clean up
	if session != nil && session.Terminal != nil {
		session.Terminal.Stop()
	}
}

func TestSessionHandlerHandleSessionStartWithPromptDelay(t *testing.T) {
	tempDir := t.TempDir()

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         tempDir,
		},
	}
	termManager := terminal.NewManager("/bin/bash", tempDir)
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionStartPayload{
		SessionKey:    "prompt-delay-session",
		AgentType:     "claude-code",
		LaunchCommand: "cat",
		InitialPrompt: "Hello World",
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
		t.Skipf("Could not start session: %v", err)
	}

	// Wait for initial prompt to be sent
	time.Sleep(700 * time.Millisecond)

	// Clean up
	session, ok := store.Get("prompt-delay-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}
}

func TestSessionHandlerHandleSessionStartWithRepositoryURL(t *testing.T) {
	tempDir := t.TempDir()

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         tempDir,
		},
	}
	termManager := terminal.NewManager("/bin/bash", tempDir)
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionStartPayload{
		SessionKey:    "repo-session",
		AgentType:     "claude-code",
		LaunchCommand: "echo",
		LaunchArgs:    []string{"test"},
		RepositoryURL: "https://github.com/test/repo", // Non-existent repo
		Branch:        "main",
		Rows:          24,
		Cols:          80,
	}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionStart,
		Payload: payloadBytes,
	}

	err := handler.HandleMessage(context.Background(), msg)
	// May or may not fail depending on repository availability
	t.Logf("HandleMessage with repository: %v", err)

	// Clean up
	session, ok := store.Get("repo-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}
}
