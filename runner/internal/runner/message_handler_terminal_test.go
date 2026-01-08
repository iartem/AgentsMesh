package runner

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/client"
	"github.com/anthropics/agentmesh/runner/internal/config"
)

// Tests for terminal operations: OnTerminalInput, OnTerminalResize

// --- OnTerminalInput Tests ---

func TestOnTerminalInputSessionNotFound(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	req := client.TerminalInputRequest{
		SessionID: "nonexistent",
		Data:      base64.StdEncoding.EncodeToString([]byte("hello")),
	}

	err := handler.OnTerminalInput(req)
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
	if !contains(err.Error(), "session not found") {
		t.Errorf("error = %v, want containing 'session not found'", err)
	}
}

func TestOnTerminalInputInvalidBase64(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	// Add session with nil terminal (will fail on Write, not decode)
	store.Put("input-session", &Session{
		ID:       "input-session",
		Terminal: nil,
	})

	req := client.TerminalInputRequest{
		SessionID: "input-session",
		Data:      "not-valid-base64!!!", // Invalid base64
	}

	err := handler.OnTerminalInput(req)
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestOnTerminalInputSuccess(t *testing.T) {
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

	// First create a session
	createReq := client.CreateSessionRequest{
		SessionID:      "input-success-session",
		InitialCommand: "cat",
		WorkingDir:     tempDir,
	}

	err := handler.OnCreateSession(createReq)
	if err != nil {
		t.Skipf("Could not create session: %v", err)
	}

	// Wait for terminal to be ready
	time.Sleep(100 * time.Millisecond)

	// Send input
	inputReq := client.TerminalInputRequest{
		SessionID: "input-success-session",
		Data:      base64.StdEncoding.EncodeToString([]byte("hello\n")),
	}

	err = handler.OnTerminalInput(inputReq)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Clean up
	session, ok := store.Get("input-success-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}
}

// --- OnTerminalResize Tests ---

func TestOnTerminalResizeSessionNotFound(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	req := client.TerminalResizeRequest{
		SessionID: "nonexistent",
		Rows:      30,
		Cols:      100,
	}

	err := handler.OnTerminalResize(req)
	if err == nil {
		t.Error("expected error for nonexistent session")
	}
	if !contains(err.Error(), "session not found") {
		t.Errorf("error = %v, want containing 'session not found'", err)
	}
}

func TestOnTerminalResizeSuccess(t *testing.T) {
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

	// First create a session
	createReq := client.CreateSessionRequest{
		SessionID:      "resize-session",
		InitialCommand: "cat",
		WorkingDir:     tempDir,
	}

	err := handler.OnCreateSession(createReq)
	if err != nil {
		t.Skipf("Could not create session: %v", err)
	}

	// Wait for terminal to be ready
	time.Sleep(100 * time.Millisecond)

	// Now resize the terminal
	resizeReq := client.TerminalResizeRequest{
		SessionID: "resize-session",
		Rows:      40,
		Cols:      120,
	}

	err = handler.OnTerminalResize(resizeReq)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify pty_resized event was sent
	events := mockConn.GetEvents()
	hasResized := false
	for _, e := range events {
		if e.Type == client.MsgTypePtyResized {
			hasResized = true
			break
		}
	}
	if !hasResized {
		t.Error("should have sent pty_resized event")
	}

	// Clean up
	session, ok := store.Get("resize-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}
}
