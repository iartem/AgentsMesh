package relay

import (
	"testing"
	"time"
)

func TestSessionLifecycle(t *testing.T) {
	m := NewManager()
	relay := &RelayInfo{ID: "relay-1", URL: "wss://relay.example.com"}
	if err := m.Register(relay); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Create session
	session, err := m.CreateSession("pod-1", "session-123", relay)
	if err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if session == nil {
		t.Fatal("CreateSession returned nil")
	}
	if session.PodKey != "pod-1" || session.SessionID != "session-123" {
		t.Error("session fields incorrect")
	}

	// Get session
	got := m.GetSession("pod-1")
	if got == nil {
		t.Fatal("GetSession returned nil")
	}
	if got.SessionID != "session-123" {
		t.Error("wrong session")
	}

	// Refresh session
	oldExpire := got.ExpireAt
	time.Sleep(10 * time.Millisecond)
	m.RefreshSession("pod-1")
	got = m.GetSession("pod-1")
	if !got.ExpireAt.After(oldExpire) {
		t.Error("RefreshSession should extend expiry")
	}

	// Remove session
	m.RemoveSession("pod-1")
	if m.GetSession("pod-1") != nil {
		t.Error("session should be removed")
	}
}

func TestMigrateSession(t *testing.T) {
	m := NewManager()
	relay1 := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	relay2 := &RelayInfo{ID: "relay-2", URL: "wss://r2.com"}
	if err := m.Register(relay1); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := m.Register(relay2); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Create session on relay-1
	if _, err := m.CreateSession("pod-1", "session-1", relay1); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Migrate to relay-2
	newSession, oldRelayID, err := m.MigrateSession("pod-1", relay2)
	if err != nil {
		t.Fatalf("MigrateSession: %v", err)
	}
	if oldRelayID != "relay-1" {
		t.Errorf("old relay: got %q, want %q", oldRelayID, "relay-1")
	}
	if newSession.RelayID != "relay-2" {
		t.Errorf("new relay: got %q, want %q", newSession.RelayID, "relay-2")
	}
	if newSession.SessionID != "session-1" {
		t.Error("session ID should be preserved")
	}

	// Verify session is updated
	session := m.GetSession("pod-1")
	if session.RelayID != "relay-2" {
		t.Error("session not updated in manager")
	}
}

func TestMigrateSessionNotFound(t *testing.T) {
	m := NewManager()
	relay := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	if err := m.Register(relay); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	_, _, err := m.MigrateSession("unknown-pod", relay)
	if err == nil {
		t.Error("expected error for unknown session")
	}
}

func TestGetSessionsByRelay(t *testing.T) {
	m := NewManager()
	relay1 := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	relay2 := &RelayInfo{ID: "relay-2", URL: "wss://r2.com"}
	if err := m.Register(relay1); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := m.Register(relay2); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if _, err := m.CreateSession("pod-1", "s1", relay1); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if _, err := m.CreateSession("pod-2", "s2", relay1); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if _, err := m.CreateSession("pod-3", "s3", relay2); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	sessions := m.GetSessionsByRelay("relay-1")
	if len(sessions) != 2 {
		t.Errorf("sessions on relay-1: got %d, want 2", len(sessions))
	}

	sessions = m.GetSessionsByRelay("relay-2")
	if len(sessions) != 1 {
		t.Errorf("sessions on relay-2: got %d, want 1", len(sessions))
	}

	sessions = m.GetSessionsByRelay("unknown")
	if len(sessions) != 0 {
		t.Errorf("sessions on unknown: got %d, want 0", len(sessions))
	}
}

func TestGetAllSessions(t *testing.T) {
	m := NewManager()
	relay := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	if err := m.Register(relay); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if len(m.GetAllSessions()) != 0 {
		t.Error("should return empty for no sessions")
	}

	if _, err := m.CreateSession("pod-1", "s1", relay); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if _, err := m.CreateSession("pod-2", "s2", relay); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	sessions := m.GetAllSessions()
	if len(sessions) != 2 {
		t.Errorf("all sessions: got %d, want 2", len(sessions))
	}
}
