package runner

import (
	"testing"

	"github.com/anthropics/agentmesh/runner/internal/client"
	"github.com/anthropics/agentmesh/runner/internal/config"
)

// Basic unit tests for RunnerMessageHandler creation and interface

func TestNewRunnerMessageHandler(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()
	cfg := &config.Config{WorkspaceRoot: t.TempDir()}
	runner := &Runner{cfg: cfg}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	if handler == nil {
		t.Fatal("handler should not be nil")
	}
	if handler.runner != runner {
		t.Error("handler.runner mismatch")
	}
	if handler.sessionStore != store {
		t.Error("handler.sessionStore mismatch")
	}
	if handler.conn != mockConn {
		t.Error("handler.conn mismatch")
	}
}

func TestRunnerMessageHandlerImplementsInterface(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()
	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	// Verify it implements client.MessageHandler
	var _ client.MessageHandler = handler
}

func TestRunnerMessageHandlerWithNilRunner(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	// Should not panic with nil runner
	handler := NewRunnerMessageHandler(nil, store, mockConn)

	if handler == nil {
		t.Fatal("handler should not be nil even with nil runner")
	}
	if handler.runner != nil {
		t.Error("handler.runner should be nil")
	}
}

func TestRunnerMessageHandlerWithNilStore(t *testing.T) {
	mockConn := client.NewMockConnection()
	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, nil, mockConn)

	if handler == nil {
		t.Fatal("handler should not be nil even with nil store")
	}
	if handler.sessionStore != nil {
		t.Error("handler.sessionStore should be nil")
	}
}

func TestRunnerMessageHandlerWithNilConnection(t *testing.T) {
	store := NewInMemorySessionStore()
	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, nil)

	if handler == nil {
		t.Fatal("handler should not be nil even with nil connection")
	}
	if handler.conn != nil {
		t.Error("handler.conn should be nil")
	}
}
