package relay

import (
	"testing"
	"time"
)

func TestForceUnregister(t *testing.T) {
	m := NewManager()
	relay := &RelayInfo{ID: "relay-1", URL: "wss://relay.example.com"}
	if err := m.Register(relay); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Create sessions on this relay
	if _, err := m.CreateSession("pod-1", "s1", relay); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if _, err := m.CreateSession("pod-2", "s2", relay); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Force unregister
	affected := m.ForceUnregister("relay-1")

	if len(affected) != 2 {
		t.Errorf("affected sessions: got %d, want 2", len(affected))
	}

	// Verify relay is removed
	if m.GetRelayByID("relay-1") != nil {
		t.Error("relay should be removed")
	}

	// Verify sessions are removed
	if m.GetSession("pod-1") != nil || m.GetSession("pod-2") != nil {
		t.Error("sessions should be removed")
	}
}

func TestForceUnregisterNoSessions(t *testing.T) {
	m := NewManager()
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://relay.example.com"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	affected := m.ForceUnregister("relay-1")

	if len(affected) != 0 {
		t.Errorf("should have no affected sessions, got %d", len(affected))
	}
}

func TestForceUnregisterNotFound(t *testing.T) {
	m := NewManager()

	affected := m.ForceUnregister("unknown")

	if len(affected) != 0 {
		t.Errorf("should return empty for unknown relay, got %d", len(affected))
	}
}

func TestGracefulUnregister(t *testing.T) {
	m := NewManager()
	relay := &RelayInfo{ID: "relay-1", URL: "wss://relay.example.com"}
	if err := m.Register(relay); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Create sessions
	if _, err := m.CreateSession("pod-1", "s1", relay); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if _, err := m.CreateSession("pod-2", "s2", relay); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Graceful unregister
	affected := m.GracefulUnregister("relay-1", "shutdown")

	if len(affected) != 2 {
		t.Errorf("affected sessions: got %d, want 2", len(affected))
	}

	// Verify relay is removed
	if m.GetRelayByID("relay-1") != nil {
		t.Error("relay should be removed")
	}

	// Verify sessions are removed
	if m.GetSession("pod-1") != nil || m.GetSession("pod-2") != nil {
		t.Error("sessions should be removed")
	}
}

func TestGracefulUnregisterNotFound(t *testing.T) {
	m := NewManager()

	affected := m.GracefulUnregister("unknown", "shutdown")

	if affected != nil {
		t.Error("should return nil for unknown relay")
	}
}

func TestManagerStop(t *testing.T) {
	m := NewManager()

	// Should not be stopped initially
	if m.IsStopped() {
		t.Error("manager should not be stopped initially")
	}

	// Stop the manager
	m.Stop()

	// Should be stopped now
	if !m.IsStopped() {
		t.Error("manager should be stopped after Stop()")
	}

	// Stop should be idempotent (no panic on double stop)
	m.Stop()
	if !m.IsStopped() {
		t.Error("manager should remain stopped")
	}
}

func TestManagerStopWithHealthCheck(t *testing.T) {
	m := NewManagerWithOptions(
		WithHealthCheckInterval(50 * time.Millisecond),
	)

	// Register a relay to trigger health check activity
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://r1.com"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Wait for at least one health check cycle
	time.Sleep(100 * time.Millisecond)

	// Stop should complete without blocking
	done := make(chan struct{})
	go func() {
		m.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Good, stop completed
	case <-time.After(500 * time.Millisecond):
		t.Error("Stop() should not block")
	}

	if !m.IsStopped() {
		t.Error("manager should be stopped")
	}
}

func TestOnRelayUnhealthyCallback(t *testing.T) {
	callbackCalled := false
	var callbackRelayID string
	var callbackSessions []*ActiveSession
	callbackDone := make(chan struct{})

	m := NewManagerWithOptions(
		WithOnRelayUnhealthy(func(relayID string, sessions []*ActiveSession) {
			callbackCalled = true
			callbackRelayID = relayID
			callbackSessions = sessions
			close(callbackDone)
		}),
	)

	// Register relay and create session
	relay := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	if err := m.Register(relay); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if _, err := m.CreateSession("pod-1", "session-1", relay); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Call markRelayUnhealthy (relay is currently healthy)
	// This will mark it unhealthy and trigger the callback
	m.markRelayUnhealthy("relay-1")

	// Wait for async callback to complete
	select {
	case <-callbackDone:
		// Callback completed
	case <-time.After(500 * time.Millisecond):
		t.Fatal("callback did not complete in time")
	}

	if !callbackCalled {
		t.Error("onRelayUnhealthy callback should be called")
	}
	if callbackRelayID != "relay-1" {
		t.Errorf("callback relayID: got %q, want %q", callbackRelayID, "relay-1")
	}
	if len(callbackSessions) != 1 {
		t.Errorf("callback sessions: got %d, want 1", len(callbackSessions))
	}
}

func TestOnRelayUnhealthyCallbackNoSessions(t *testing.T) {
	callbackCalled := false

	m := NewManagerWithOptions(
		WithOnRelayUnhealthy(func(relayID string, sessions []*ActiveSession) {
			callbackCalled = true
		}),
	)

	// Register relay without sessions
	relay := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	if err := m.Register(relay); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Mark unhealthy
	m.mu.Lock()
	m.relays["relay-1"].Healthy = false
	m.mu.Unlock()

	m.markRelayUnhealthy("relay-1")

	// Callback should not be called for relay with no sessions
	if callbackCalled {
		t.Error("callback should not be called for relay with no sessions")
	}
}
