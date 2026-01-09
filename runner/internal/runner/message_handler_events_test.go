package runner

import (
	"testing"

	"github.com/anthropics/agentmesh/runner/internal/client"
	"github.com/anthropics/agentmesh/runner/internal/config"
)

// Tests for event sending methods and helper functions

// --- Test event sending methods ---

func TestSendSessionCreated(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	handler.sendSessionCreated("session-1", 12345, "/worktree/path", "feature/test", 80, 24)

	events := mockConn.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Type != client.MsgTypeSessionCreated {
		t.Errorf("event type = %s, want session_created", events[0].Type)
	}

	event, ok := events[0].Data.(client.SessionCreatedEvent)
	if !ok {
		t.Fatalf("event data should be SessionCreatedEvent")
	}
	if event.SessionID != "session-1" {
		t.Errorf("session_id = %s, want session-1", event.SessionID)
	}
	if event.Pid != 12345 {
		t.Errorf("pid = %d, want 12345", event.Pid)
	}
}

func TestSendSessionTerminated(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	handler.sendSessionTerminated("session-1")

	events := mockConn.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Type != client.MsgTypeSessionTerminated {
		t.Errorf("event type = %s, want session_terminated", events[0].Type)
	}
}

func TestSendTerminalOutput(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	handler.sendTerminalOutput("session-1", []byte("hello world"))

	// Terminal output uses SendWithBackpressure, so check SentMessages
	msgs := mockConn.GetSentMessages()
	hasOutput := false
	for _, m := range msgs {
		if m.Type == client.MsgTypeTerminalOutput {
			hasOutput = true
			break
		}
	}
	if !hasOutput {
		t.Error("should have sent terminal_output message")
	}
}

func TestSendPtyResized(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	handler.sendPtyResized("session-1", 100, 30)

	events := mockConn.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}

	if events[0].Type != client.MsgTypePtyResized {
		t.Errorf("event type = %s, want pty_resized", events[0].Type)
	}
}

func TestSendSessionError(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	handler.sendSessionError("session-1", "something went wrong")

	events := mockConn.GetEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

// --- Test send methods with nil connection ---

func TestSendMethodsWithNilConnection(t *testing.T) {
	store := NewInMemorySessionStore()

	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, nil)

	// These should not panic with nil connection
	handler.sendSessionCreated("session-1", 123, "", "", 80, 24)
	handler.sendSessionTerminated("session-1")
	handler.sendTerminalOutput("session-1", []byte("data"))
	handler.sendPtyResized("session-1", 80, 24)
	handler.sendSessionError("session-1", "error")
}

// --- Test createOutputHandler ---

func TestCreateOutputHandler(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	outputHandler := handler.createOutputHandler("session-1")

	// Call the handler
	outputHandler([]byte("test output"))

	// Verify output was sent
	msgs := mockConn.GetSentMessages()
	hasOutput := false
	for _, m := range msgs {
		if m.Type == client.MsgTypeTerminalOutput {
			hasOutput = true
			break
		}
	}
	if !hasOutput {
		t.Error("output handler should send terminal output")
	}
}

// --- Test createExitHandler ---

func TestCreateExitHandler(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	// Add a session
	store.Put("exit-session", &Session{
		ID:     "exit-session",
		Status: SessionStatusRunning,
	})

	exitHandler := handler.createExitHandler("exit-session")

	// Call the handler
	exitHandler(0)

	// Verify session was removed
	_, exists := store.Get("exit-session")
	if exists {
		t.Error("session should be removed after exit")
	}

	// Verify terminated event was sent
	events := mockConn.GetEvents()
	hasTerminated := false
	for _, e := range events {
		if e.Type == client.MsgTypeSessionTerminated {
			hasTerminated = true
			break
		}
	}
	if !hasTerminated {
		t.Error("exit handler should send session_terminated")
	}
}

// Note: runPreparationScript and mergeEnvVars have been moved to SessionBuilder.
// Tests for these functions are in session_builder_test.go.

// --- Benchmark tests ---

func BenchmarkOnListSessions(b *testing.B) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	// Add some sessions
	for i := 0; i < 100; i++ {
		store.Put(string(rune('a'+i%26))+string(rune(i)), &Session{
			ID:     string(rune('a' + i%26)),
			Status: SessionStatusRunning,
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		handler.OnListSessions()
	}
}

// Note: BenchmarkMergeEnvVars moved to session_builder_test.go since the method is now on SessionBuilder.
