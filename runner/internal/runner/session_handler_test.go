package runner

import (
	"testing"

	"github.com/anthropics/agentmesh/runner/internal/terminal"
)

// Basic unit tests for SessionHandler creation and helper methods

func TestNewSessionHandler(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	if handler == nil {
		t.Fatal("NewSessionHandler returned nil")
	}

	if handler.runner != runner {
		t.Error("runner should be set")
	}

	if handler.termManager != termManager {
		t.Error("termManager should be set")
	}

	if handler.eventSender != eventSender {
		t.Error("eventSender should be set")
	}

	if handler.sessionStore != store {
		t.Error("sessionStore should be set")
	}
}

func TestSessionHandlerHandleSessionExit(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	// Add a session
	store.Put("session-1", &Session{
		ID:         "session-1",
		SessionKey: "session-1",
		Status:     SessionStatusRunning,
	})

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	handler.handleSessionExit("session-1", 0)

	// Session should be removed
	_, ok := store.Get("session-1")
	if ok {
		t.Error("session should be removed after exit")
	}

	// Check that exit status was sent
	eventSender.mu.Lock()
	defer eventSender.mu.Unlock()

	if len(eventSender.statuses) != 1 {
		t.Errorf("statuses count: got %v, want 1", len(eventSender.statuses))
	}

	if eventSender.statuses[0].status != "exited" {
		t.Errorf("status: got %v, want exited", eventSender.statuses[0].status)
	}
}

func TestSessionHandlerHandleSessionExitNonExistent(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	// Handle exit for non-existent session
	handler.handleSessionExit("nonexistent", 0)

	eventSender.mu.Lock()
	defer eventSender.mu.Unlock()

	// Should still send exited status
	if len(eventSender.statuses) != 1 {
		t.Errorf("statuses count = %d, want 1", len(eventSender.statuses))
	}
	if eventSender.statuses[0].status != "exited" {
		t.Errorf("status = %v, want exited", eventSender.statuses[0].status)
	}
}

func TestSessionHandlerSendSessionError(t *testing.T) {
	runner := &Runner{}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	handler.sendSessionError("session-1", "test error")

	eventSender.mu.Lock()
	defer eventSender.mu.Unlock()

	if len(eventSender.statuses) != 1 {
		t.Errorf("statuses count: got %v, want 1", len(eventSender.statuses))
	}

	if eventSender.statuses[0].status != "error" {
		t.Errorf("status: got %v, want error", eventSender.statuses[0].status)
	}

	if eventSender.statuses[0].data["error"] != "test error" {
		t.Errorf("error message: got %v, want test error", eventSender.statuses[0].data["error"])
	}
}
