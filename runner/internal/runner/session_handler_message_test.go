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

// Tests for HandleMessage routing and JSON parsing

func TestSessionHandlerHandleMessageUnknownType(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	msg := &client.Message{
		Type:    "unknown_type",
		Payload: []byte("{}"),
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error for unknown type: %v", err)
	}
}

// --- SessionStop Tests ---

func TestSessionHandlerHandleSessionStopNotFound(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionStopPayload{SessionKey: "nonexistent"}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionStop,
		Payload: payloadBytes,
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

func TestSessionHandlerHandleSessionStopWithWorktree(t *testing.T) {
	runner := &Runner{
		cfg:       &config.Config{},
		workspace: nil, // No workspace manager
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	// Add session with worktree
	store.Put("session-1", &Session{
		ID:           "session-1",
		SessionKey:   "session-1",
		WorktreePath: "/workspace/worktrees/session-1",
		Terminal:     nil,
	})

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionStopPayload{SessionKey: "session-1"}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionStop,
		Payload: payloadBytes,
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Session should be removed
	if store.Count() != 0 {
		t.Errorf("session count = %d, want 0", store.Count())
	}

	// Check status sent
	eventSender.mu.Lock()
	defer eventSender.mu.Unlock()

	if len(eventSender.statuses) != 1 {
		t.Errorf("statuses count = %d, want 1", len(eventSender.statuses))
	}
	if eventSender.statuses[0].status != "stopped" {
		t.Errorf("status = %v, want stopped", eventSender.statuses[0].status)
	}
}

// --- TerminalInput Tests ---

func TestSessionHandlerHandleTerminalInputNotFound(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := TerminalInputPayload{SessionKey: "nonexistent", Data: []byte("test")}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeTerminalInput,
		Payload: payloadBytes,
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

// --- TerminalResize Tests ---

func TestSessionHandlerHandleTerminalResizeNotFound(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := TerminalResizePayload{SessionKey: "nonexistent", Rows: 40, Cols: 120}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeTerminalResize,
		Payload: payloadBytes,
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
}

// --- SessionList Tests ---

func TestSessionHandlerHandleSessionList(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	// Add some sessions
	store.Put("session-1", &Session{
		ID:         "session-1",
		SessionKey: "session-1",
		AgentType:  "claude-code",
		Status:     SessionStatusRunning,
		StartedAt:  time.Now(),
	})

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionListPayload{RequestID: "req-123"}
	payloadBytes, _ := json.Marshal(payload)

	msg := &client.Message{
		Type:    client.MessageTypeSessionList,
		Payload: payloadBytes,
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Check that status was sent
	eventSender.mu.Lock()
	defer eventSender.mu.Unlock()

	if len(eventSender.statuses) != 1 {
		t.Errorf("statuses count: got %v, want 1", len(eventSender.statuses))
	}

	if eventSender.statuses[0].status != "session_list" {
		t.Errorf("status: got %v, want session_list", eventSender.statuses[0].status)
	}
}

func TestSessionHandlerHandleSessionListWithMultipleSessions(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
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
		WorktreePath:  "/workspace/worktrees/session-2",
		RepositoryURL: "https://github.com/test/repo.git",
	})

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionListPayload{RequestID: "req-456"}
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
	defer eventSender.mu.Unlock()

	if len(eventSender.statuses) != 1 {
		t.Errorf("statuses count: got %v, want 1", len(eventSender.statuses))
	}
}

func TestSessionHandlerHandleSessionListEmpty(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	payload := SessionListPayload{RequestID: "req-empty"}
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
	defer eventSender.mu.Unlock()

	if len(eventSender.statuses) != 1 {
		t.Errorf("statuses count = %d, want 1", len(eventSender.statuses))
	}
}

// --- Invalid JSON Tests ---

func TestSessionHandlerHandleSessionStartInvalidJSON(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	msg := &client.Message{
		Type:    client.MessageTypeSessionStart,
		Payload: []byte("invalid json"),
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSessionHandlerHandleSessionStopInvalidJSON(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	msg := &client.Message{
		Type:    client.MessageTypeSessionStop,
		Payload: []byte("invalid json"),
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSessionHandlerHandleTerminalInputInvalidJSON(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	msg := &client.Message{
		Type:    client.MessageTypeTerminalInput,
		Payload: []byte("invalid json"),
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSessionHandlerHandleTerminalResizeInvalidJSON(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	msg := &client.Message{
		Type:    client.MessageTypeTerminalResize,
		Payload: []byte("invalid json"),
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSessionHandlerHandleSessionListInvalidJSON(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	msg := &client.Message{
		Type:    client.MessageTypeSessionList,
		Payload: []byte("invalid json"),
	}

	err := handler.HandleMessage(context.Background(), msg)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}
