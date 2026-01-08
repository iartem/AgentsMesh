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

// --- Additional tests for SessionHandler ---

func TestSessionHandlerHandleAllMessageTypes(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
		},
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	tests := []struct {
		name    string
		msgType string
		payload interface{}
		wantErr bool
	}{
		{
			name:    "heartbeat - unknown type",
			msgType: client.MessageTypeHeartbeat,
			payload: map[string]string{},
			wantErr: false,
		},
		{
			name:    "error - unknown type",
			msgType: client.MessageTypeError,
			payload: map[string]string{"error": "test"},
			wantErr: false,
		},
		{
			name:    "custom unknown type",
			msgType: "custom.unknown",
			payload: map[string]string{},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payloadBytes, _ := json.Marshal(tt.payload)
			msg := &client.Message{
				Type:    tt.msgType,
				Payload: payloadBytes,
			}

			err := handler.HandleMessage(context.Background(), msg)
			if tt.wantErr && err == nil {
				t.Error("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSessionHandlerHandleSessionListWithPID(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{},
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	// Add session (Terminal is nil in this test)
	store.Put("session-1", &Session{
		ID:            "session-1",
		SessionKey:    "session-1",
		AgentType:     "claude-code",
		Status:        SessionStatusRunning,
		StartedAt:     time.Now(),
		WorktreePath:  "/test/path",
		RepositoryURL: "https://github.com/test/repo.git",
		Terminal:      nil, // nil terminal
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

	// Verify status was sent
	eventSender.mu.Lock()
	defer eventSender.mu.Unlock()

	if len(eventSender.statuses) != 1 {
		t.Errorf("statuses count = %d, want 1", len(eventSender.statuses))
	}

	if eventSender.statuses[0].status != "session_list" {
		t.Errorf("status = %v, want session_list", eventSender.statuses[0].status)
	}
}

func TestSessionHandlerHandleSessionExitUpdatesStatus(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{},
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	// Add session
	session := &Session{
		ID:     "session-1",
		Status: SessionStatusRunning,
	}
	store.Put("session-1", session)

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	// Handle exit
	handler.handleSessionExit("session-1", 42)

	// Session should be deleted from store
	_, ok := store.Get("session-1")
	if ok {
		t.Error("session should be removed from store")
	}

	// Event should be sent
	eventSender.mu.Lock()
	defer eventSender.mu.Unlock()

	if len(eventSender.statuses) != 1 {
		t.Errorf("statuses count = %d, want 1", len(eventSender.statuses))
	}

	if eventSender.statuses[0].status != "exited" {
		t.Errorf("status = %v, want exited", eventSender.statuses[0].status)
	}

	exitCode := eventSender.statuses[0].data["exit_code"]
	if exitCode != 42 {
		t.Errorf("exit_code = %v, want 42", exitCode)
	}
}

func TestSessionHandlerSendSessionErrorFormat(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{},
	}
	termManager := terminal.NewManager("/bin/bash", "/tmp")
	eventSender := newMockEventSender()
	store := NewInMemorySessionStore()

	handler := NewSessionHandler(runner, termManager, eventSender, store)

	// Send error
	handler.sendSessionError("session-1", "test error message")

	// Verify
	eventSender.mu.Lock()
	defer eventSender.mu.Unlock()

	if len(eventSender.statuses) != 1 {
		t.Errorf("statuses count = %d, want 1", len(eventSender.statuses))
	}

	status := eventSender.statuses[0]
	if status.sessionKey != "session-1" {
		t.Errorf("sessionKey = %v, want session-1", status.sessionKey)
	}

	if status.status != "error" {
		t.Errorf("status = %v, want error", status.status)
	}

	if status.data["error"] != "test error message" {
		t.Errorf("error = %v, want 'test error message'", status.data["error"])
	}
}

// --- InMemorySessionStore tests ---

func TestInMemorySessionStoreOperations(t *testing.T) {
	store := NewInMemorySessionStore()

	// Test Put and Get
	session := &Session{ID: "session-1", Status: SessionStatusRunning}
	store.Put("session-1", session)

	got, ok := store.Get("session-1")
	if !ok {
		t.Error("session should exist")
	}
	if got.ID != "session-1" {
		t.Errorf("ID = %v, want session-1", got.ID)
	}

	// Test Count
	if store.Count() != 1 {
		t.Errorf("Count = %d, want 1", store.Count())
	}

	// Test All
	all := store.All()
	if len(all) != 1 {
		t.Errorf("All() length = %d, want 1", len(all))
	}

	// Test Delete
	deleted := store.Delete("session-1")
	if deleted == nil {
		t.Error("deleted should not be nil")
	}
	if deleted.ID != "session-1" {
		t.Errorf("deleted ID = %v, want session-1", deleted.ID)
	}

	// Test Get after Delete
	_, ok = store.Get("session-1")
	if ok {
		t.Error("session should not exist after delete")
	}

	// Test Count after Delete
	if store.Count() != 0 {
		t.Errorf("Count after delete = %d, want 0", store.Count())
	}
}

func TestInMemorySessionStoreDeleteNonExistent(t *testing.T) {
	store := NewInMemorySessionStore()

	deleted := store.Delete("nonexistent")
	if deleted != nil {
		t.Error("delete of nonexistent should return nil")
	}
}

func TestInMemorySessionStoreAllEmpty(t *testing.T) {
	store := NewInMemorySessionStore()

	all := store.All()
	if len(all) != 0 {
		t.Errorf("All() on empty store = %d, want 0", len(all))
	}
}

func TestInMemorySessionStoreMultipleSessions(t *testing.T) {
	store := NewInMemorySessionStore()

	// Add multiple sessions
	for i := 1; i <= 5; i++ {
		store.Put("session-"+string(rune('0'+i)), &Session{
			ID:     "session-" + string(rune('0'+i)),
			Status: SessionStatusRunning,
		})
	}

	if store.Count() != 5 {
		t.Errorf("Count = %d, want 5", store.Count())
	}

	all := store.All()
	if len(all) != 5 {
		t.Errorf("All() length = %d, want 5", len(all))
	}
}

func TestInMemorySessionStoreOverwrite(t *testing.T) {
	store := NewInMemorySessionStore()

	// Add session
	store.Put("session-1", &Session{ID: "session-1", Status: SessionStatusRunning})

	// Overwrite
	store.Put("session-1", &Session{ID: "session-1", Status: SessionStatusStopped})

	got, ok := store.Get("session-1")
	if !ok {
		t.Error("session should exist")
	}
	if got.Status != SessionStatusStopped {
		t.Errorf("Status = %v, want stopped", got.Status)
	}

	// Count should still be 1
	if store.Count() != 1 {
		t.Errorf("Count after overwrite = %d, want 1", store.Count())
	}
}
