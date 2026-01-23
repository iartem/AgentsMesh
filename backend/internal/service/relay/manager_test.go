package relay

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("NewManager returned nil")
	}
	if m.relays == nil || m.activeSessions == nil {
		t.Error("maps not initialized")
	}
}

func TestRegister(t *testing.T) {
	m := NewManager()
	info := &RelayInfo{
		ID:       "relay-1",
		URL:      "wss://relay.example.com",
		Region:   "us-east",
		Capacity: 1000,
	}

	if err := m.Register(info); err != nil { t.Fatalf("Register failed: %v", err) }

	relays := m.GetRelays()
	if len(relays) != 1 {
		t.Fatalf("expected 1 relay, got %d", len(relays))
	}
	if relays[0].ID != "relay-1" {
		t.Errorf("id: got %q, want %q", relays[0].ID, "relay-1")
	}
	if !relays[0].Healthy {
		t.Error("newly registered relay should be healthy")
	}
}

func TestHeartbeat(t *testing.T) {
	m := NewManager()
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://relay.example.com"}); err != nil { t.Fatalf("Register failed: %v", err) }

	err := m.Heartbeat("relay-1", 50, 25.5, 60.0)
	if err != nil {
		t.Fatalf("Heartbeat: %v", err)
	}

	relays := m.GetRelays()
	if relays[0].CurrentConnections != 50 {
		t.Errorf("connections: got %d, want 50", relays[0].CurrentConnections)
	}
	if relays[0].CPUUsage != 25.5 {
		t.Errorf("cpu: got %f, want 25.5", relays[0].CPUUsage)
	}
}

func TestHeartbeatNotFound(t *testing.T) {
	m := NewManager()
	err := m.Heartbeat("unknown", 0, 0, 0)
	if err == nil {
		t.Error("expected error for unknown relay")
	}
}

func TestUnregister(t *testing.T) {
	m := NewManager()
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://relay.example.com"}); err != nil { t.Fatalf("Register failed: %v", err) }

	m.Unregister("relay-1")

	if len(m.GetRelays()) != 0 {
		t.Error("relay should be removed")
	}
}

func TestSelectRelay(t *testing.T) {
	m := NewManager()
	if err := m.Register(&RelayInfo{ID: "relay-us", URL: "wss://us.relay.com", Region: "us-east", Capacity: 100, Healthy: true}); err != nil { t.Fatalf("Register failed: %v", err) }
	if err := m.Register(&RelayInfo{ID: "relay-eu", URL: "wss://eu.relay.com", Region: "eu-west", Capacity: 100, Healthy: true}); err != nil { t.Fatalf("Register failed: %v", err) }

	// Same region should be preferred
	relay := m.SelectRelay("us-east")
	if relay == nil {
		t.Fatal("SelectRelay returned nil")
	}
	if relay.ID != "relay-us" {
		t.Errorf("should prefer same region: got %q", relay.ID)
	}

	// Different region
	relay = m.SelectRelay("eu-west")
	if relay.ID != "relay-eu" {
		t.Errorf("should prefer same region: got %q", relay.ID)
	}
}

func TestSelectRelayPreferLowLoad(t *testing.T) {
	m := NewManager()
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://r1.com", Region: "us", Capacity: 100, CurrentConnections: 90}); err != nil { t.Fatalf("Register failed: %v", err) }
	if err := m.Register(&RelayInfo{ID: "relay-2", URL: "wss://r2.com", Region: "us", Capacity: 100, CurrentConnections: 10}); err != nil { t.Fatalf("Register failed: %v", err) }

	relay := m.SelectRelay("us")
	if relay == nil {
		t.Fatal("SelectRelay returned nil")
	}
	if relay.ID != "relay-2" {
		t.Errorf("should prefer lower load: got %q", relay.ID)
	}
}

func TestSelectRelaySkipsUnhealthy(t *testing.T) {
	m := NewManager()
	// Register both relays (both become healthy automatically)
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://r1.com"}); err != nil { t.Fatalf("Register failed: %v", err) }
	if err := m.Register(&RelayInfo{ID: "relay-2", URL: "wss://r2.com"}); err != nil { t.Fatalf("Register failed: %v", err) }

	// Mark relay-1 as unhealthy directly
	m.mu.Lock()
	m.relays["relay-1"].Healthy = false
	m.mu.Unlock()

	relay := m.SelectRelay("us")
	if relay == nil {
		t.Fatal("SelectRelay returned nil")
	}
	if relay.ID != "relay-2" {
		t.Errorf("should skip unhealthy: got %q", relay.ID)
	}
}

func TestSelectRelayNoHealthy(t *testing.T) {
	m := NewManager()
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://r1.com"}); err != nil { t.Fatalf("Register failed: %v", err) }

	// Mark as unhealthy directly
	m.mu.Lock()
	m.relays["relay-1"].Healthy = false
	m.mu.Unlock()

	relay := m.SelectRelay("us")
	if relay != nil {
		t.Error("should return nil when no healthy relays")
	}
}

func TestSessionLifecycle(t *testing.T) {
	m := NewManager()
	relay := &RelayInfo{ID: "relay-1", URL: "wss://relay.example.com"}
	if err := m.Register(relay); err != nil { t.Fatalf("Register failed: %v", err) }

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

func TestSelectRelayForPod(t *testing.T) {
	m := NewManager()
	relay := &RelayInfo{ID: "relay-1", URL: "wss://relay.example.com", Healthy: true}
	if err := m.Register(relay); err != nil { t.Fatalf("Register failed: %v", err) }

	// First call - no existing session
	r1, s1 := m.SelectRelayForPod("pod-1", "us")
	if r1 == nil {
		t.Fatal("first SelectRelayForPod returned nil relay")
	}
	if s1 != nil {
		t.Error("first call should not have existing session")
	}

	// Create session
	if _, err := m.CreateSession("pod-1", "session-1", relay); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Second call - should return existing session
	r2, s2 := m.SelectRelayForPod("pod-1", "us")
	if r2 == nil || s2 == nil {
		t.Fatal("second SelectRelayForPod returned nil")
	}
	if s2.SessionID != "session-1" {
		t.Error("should return existing session")
	}
}

func TestGetHealthyRelayCount(t *testing.T) {
	m := NewManager()
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://r1.com"}); err != nil { t.Fatalf("Register failed: %v", err) }
	if err := m.Register(&RelayInfo{ID: "relay-2", URL: "wss://r2.com"}); err != nil { t.Fatalf("Register failed: %v", err) }
	if err := m.Register(&RelayInfo{ID: "relay-3", URL: "wss://r3.com"}); err != nil { t.Fatalf("Register failed: %v", err) }

	// Mark relay-3 as unhealthy directly
	m.mu.Lock()
	m.relays["relay-3"].Healthy = false
	m.mu.Unlock()

	if m.GetHealthyRelayCount() != 2 {
		t.Errorf("healthy count: got %d, want 2", m.GetHealthyRelayCount())
	}
}

func TestHasHealthyRelays(t *testing.T) {
	m := NewManager()
	if m.HasHealthyRelays() {
		t.Error("should be false with no relays")
	}

	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://r1.com", Healthy: true}); err != nil { t.Fatalf("Register failed: %v", err) }
	if !m.HasHealthyRelays() {
		t.Error("should be true with healthy relay")
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager()
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://r1.com", CurrentConnections: 10}); err != nil { t.Fatalf("Register failed: %v", err) }
	if err := m.Register(&RelayInfo{ID: "relay-2", URL: "wss://r2.com", CurrentConnections: 5}); err != nil { t.Fatalf("Register failed: %v", err) }

	// Mark relay-2 as unhealthy directly
	m.mu.Lock()
	m.relays["relay-2"].Healthy = false
	m.mu.Unlock()

	if _, err := m.CreateSession("pod-1", "s1", &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	stats := m.GetStats()
	if stats.TotalRelays != 2 {
		t.Errorf("TotalRelays: got %d, want 2", stats.TotalRelays)
	}
	if stats.HealthyRelays != 1 {
		t.Errorf("HealthyRelays: got %d, want 1", stats.HealthyRelays)
	}
	if stats.TotalConnections != 15 {
		t.Errorf("TotalConnections: got %d, want 15", stats.TotalConnections)
	}
	if stats.ActiveSessions != 1 {
		t.Errorf("ActiveSessions: got %d, want 1", stats.ActiveSessions)
	}
}

func TestRelayInfoGetRunnerURL(t *testing.T) {
	// With internal URL
	r1 := &RelayInfo{URL: "wss://public.com", InternalURL: "ws://internal:8090"}
	if r1.GetRunnerURL() != "ws://internal:8090" {
		t.Errorf("should return internal URL: %s", r1.GetRunnerURL())
	}

	// Without internal URL
	r2 := &RelayInfo{URL: "wss://public.com"}
	if r2.GetRunnerURL() != "wss://public.com" {
		t.Errorf("should fallback to URL: %s", r2.GetRunnerURL())
	}
}

func TestForceUnregister(t *testing.T) {
	m := NewManager()
	relay := &RelayInfo{ID: "relay-1", URL: "wss://relay.example.com"}
	if err := m.Register(relay); err != nil { t.Fatalf("Register failed: %v", err) }

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
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://relay.example.com"}); err != nil { t.Fatalf("Register failed: %v", err) }

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

func TestGetRelayByID(t *testing.T) {
	m := NewManager()
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://relay.example.com", Region: "us-east"}); err != nil { t.Fatalf("Register failed: %v", err) }

	// Found
	relay := m.GetRelayByID("relay-1")
	if relay == nil {
		t.Fatal("GetRelayByID returned nil")
	}
	if relay.ID != "relay-1" || relay.Region != "us-east" {
		t.Error("relay data mismatch")
	}

	// Not found
	if m.GetRelayByID("unknown") != nil {
		t.Error("should return nil for unknown relay")
	}
}

func TestMigrateSession(t *testing.T) {
	m := NewManager()
	relay1 := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	relay2 := &RelayInfo{ID: "relay-2", URL: "wss://r2.com"}
	if err := m.Register(relay1); err != nil { t.Fatalf("Register failed: %v", err) }
	if err := m.Register(relay2); err != nil { t.Fatalf("Register failed: %v", err) }

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
	if err := m.Register(relay); err != nil { t.Fatalf("Register failed: %v", err) }

	_, _, err := m.MigrateSession("unknown-pod", relay)
	if err == nil {
		t.Error("expected error for unknown session")
	}
}

func TestGetSessionsByRelay(t *testing.T) {
	m := NewManager()
	relay1 := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	relay2 := &RelayInfo{ID: "relay-2", URL: "wss://r2.com"}
	if err := m.Register(relay1); err != nil { t.Fatalf("Register failed: %v", err) }
	if err := m.Register(relay2); err != nil { t.Fatalf("Register failed: %v", err) }

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
	if err := m.Register(relay); err != nil { t.Fatalf("Register failed: %v", err) }

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

// === New tests for enhanced features ===

func TestHeartbeatWithLatency(t *testing.T) {
	m := NewManager()
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://relay.example.com"}); err != nil { t.Fatalf("Register failed: %v", err) }

	// First heartbeat with latency
	err := m.HeartbeatWithLatency("relay-1", 50, 25.5, 60.0, 100)
	if err != nil {
		t.Fatalf("HeartbeatWithLatency: %v", err)
	}

	relays := m.GetRelays()
	if relays[0].AvgLatencyMs != 100 {
		t.Errorf("initial latency: got %d, want 100", relays[0].AvgLatencyMs)
	}

	// Second heartbeat should apply EMA smoothing
	err = m.HeartbeatWithLatency("relay-1", 50, 25.5, 60.0, 200)
	if err != nil {
		t.Fatalf("HeartbeatWithLatency: %v", err)
	}

	relays = m.GetRelays()
	// EMA: 100 * 0.7 + 200 * 0.3 = 70 + 60 = 130
	if relays[0].AvgLatencyMs != 130 {
		t.Errorf("smoothed latency: got %d, want 130", relays[0].AvgLatencyMs)
	}
}

func TestHeartbeatWithLatencyNotFound(t *testing.T) {
	m := NewManager()
	err := m.HeartbeatWithLatency("unknown", 0, 0, 0, 100)
	if err == nil {
		t.Error("expected error for unknown relay")
	}
}

func TestGracefulUnregister(t *testing.T) {
	m := NewManager()
	relay := &RelayInfo{ID: "relay-1", URL: "wss://relay.example.com"}
	if err := m.Register(relay); err != nil { t.Fatalf("Register failed: %v", err) }

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

func TestSelectRelayWithOptions(t *testing.T) {
	m := NewManager()
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://r1.com", Region: "us-east", Capacity: 100, Healthy: true}); err != nil { t.Fatalf("Register failed: %v", err) }
	if err := m.Register(&RelayInfo{ID: "relay-2", URL: "wss://r2.com", Region: "us-east", Capacity: 100, Healthy: true}); err != nil { t.Fatalf("Register failed: %v", err) }

	// Select without exclusion
	relay := m.SelectRelayWithOptions(SelectRelayOptions{Region: "us-east"})
	if relay == nil {
		t.Fatal("SelectRelayWithOptions returned nil")
	}

	// Select with exclusion
	relay = m.SelectRelayWithOptions(SelectRelayOptions{
		Region:          "us-east",
		ExcludeRelayIDs: []string{"relay-1"},
	})
	if relay == nil {
		t.Fatal("SelectRelayWithOptions returned nil")
	}
	if relay.ID != "relay-2" {
		t.Errorf("should exclude relay-1: got %q", relay.ID)
	}

	// Exclude all
	relay = m.SelectRelayWithOptions(SelectRelayOptions{
		ExcludeRelayIDs: []string{"relay-1", "relay-2"},
	})
	if relay != nil {
		t.Error("should return nil when all excluded")
	}
}

func TestSelectRelayWeightedScoring(t *testing.T) {
	m := NewManager()

	// Register relays with different characteristics
	if err := m.Register(&RelayInfo{
		ID:                 "relay-high-load",
		URL:                "wss://r1.com",
		Region:             "us-east",
		Capacity:           100,
		CurrentConnections: 90,
		CPUUsage:           80,
		MemoryUsage:        70,
	}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := m.Register(&RelayInfo{
		ID:                 "relay-low-load",
		URL:                "wss://r2.com",
		Region:             "us-east",
		Capacity:           100,
		CurrentConnections: 10,
		CPUUsage:           20,
		MemoryUsage:        30,
	}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	relay := m.SelectRelay("us-east")
	if relay == nil {
		t.Fatal("SelectRelay returned nil")
	}
	if relay.ID != "relay-low-load" {
		t.Errorf("should prefer low load: got %q", relay.ID)
	}
}

func TestSelectRelayLatencyFactor(t *testing.T) {
	m := NewManager()

	// Register relays with different latencies (same other metrics)
	if err := m.Register(&RelayInfo{
		ID:           "relay-high-latency",
		URL:          "wss://r1.com",
		Region:       "us-east",
		Capacity:     100,
		AvgLatencyMs: 300,
	}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := m.Register(&RelayInfo{
		ID:           "relay-low-latency",
		URL:          "wss://r2.com",
		Region:       "us-east",
		Capacity:     100,
		AvgLatencyMs: 50,
	}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	relay := m.SelectRelay("us-east")
	if relay == nil {
		t.Fatal("SelectRelay returned nil")
	}
	if relay.ID != "relay-low-latency" {
		t.Errorf("should prefer low latency: got %q", relay.ID)
	}
}

func TestSelectRelayAtCapacity(t *testing.T) {
	m := NewManager()

	if err := m.Register(&RelayInfo{
		ID:                 "relay-full",
		URL:                "wss://r1.com",
		Region:             "us-east",
		Capacity:           100,
		CurrentConnections: 100, // At capacity
	}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := m.Register(&RelayInfo{
		ID:                 "relay-available",
		URL:                "wss://r2.com",
		Region:             "us-east",
		Capacity:           100,
		CurrentConnections: 50,
	}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	relay := m.SelectRelay("us-east")
	if relay == nil {
		t.Fatal("SelectRelay returned nil")
	}
	if relay.ID != "relay-available" {
		t.Errorf("should skip at-capacity relay: got %q", relay.ID)
	}
}

func TestLoadBalancingConfig(t *testing.T) {
	cfg := DefaultLoadBalancingConfig()

	if cfg.ConnectionWeight != 0.4 {
		t.Errorf("ConnectionWeight: got %f, want 0.4", cfg.ConnectionWeight)
	}
	if cfg.CPUWeight != 0.25 {
		t.Errorf("CPUWeight: got %f, want 0.25", cfg.CPUWeight)
	}
	if cfg.MemoryWeight != 0.15 {
		t.Errorf("MemoryWeight: got %f, want 0.15", cfg.MemoryWeight)
	}
	if cfg.LatencyWeight != 0.1 {
		t.Errorf("LatencyWeight: got %f, want 0.1", cfg.LatencyWeight)
	}
	if cfg.RegionBonus != 50 {
		t.Errorf("RegionBonus: got %f, want 50", cfg.RegionBonus)
	}
}

func TestNewManagerWithConfig(t *testing.T) {
	cfg := LoadBalancingConfig{
		ConnectionWeight: 0.5,
		CPUWeight:        0.3,
		MemoryWeight:     0.1,
		LatencyWeight:    0.05,
		RegionBonus:      100,
	}

	m := NewManagerWithConfig(cfg)
	if m == nil {
		t.Fatal("NewManagerWithConfig returned nil")
	}
	if m.loadBalancingConfig.ConnectionWeight != 0.5 {
		t.Errorf("config not applied: ConnectionWeight %f", m.loadBalancingConfig.ConnectionWeight)
	}
}

func TestNewManagerWithOptions(t *testing.T) {
	cfg := LoadBalancingConfig{
		ConnectionWeight: 0.6,
		RegionBonus:      200,
	}

	m := NewManagerWithOptions(WithLoadBalancingConfig(cfg))
	if m == nil {
		t.Fatal("NewManagerWithOptions returned nil")
	}
	if m.loadBalancingConfig.ConnectionWeight != 0.6 {
		t.Errorf("ConnectionWeight: got %f, want 0.6", m.loadBalancingConfig.ConnectionWeight)
	}
	if m.loadBalancingConfig.RegionBonus != 200 {
		t.Errorf("RegionBonus: got %f, want 200", m.loadBalancingConfig.RegionBonus)
	}
}

// === Tests for new lifecycle management features ===

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

func TestRegisterPersistenceFailure(t *testing.T) {
	// Create a mock store that always fails
	failStore := &FailingMockStore{}
	m := NewManagerWithOptions(WithStore(failStore))

	relay := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	err := m.Register(relay)

	if err == nil {
		t.Error("Register should return error when persistence fails")
	}

	// Verify relay was NOT added to memory (persistence-first pattern)
	if m.GetRelayByID("relay-1") != nil {
		t.Error("relay should not be in memory when persistence fails")
	}
}

func TestCreateSessionPersistenceFailure(t *testing.T) {
	// Create a mock store that fails on session save
	failStore := &SessionFailingMockStore{}
	m := NewManagerWithOptions(WithStore(failStore))

	relay := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	// Use manual registration to bypass store for relay
	m.mu.Lock()
	m.relays["relay-1"] = relay
	m.mu.Unlock()

	_, err := m.CreateSession("pod-1", "session-1", relay)

	if err == nil {
		t.Error("CreateSession should return error when persistence fails")
	}

	// Verify session was NOT added to memory (persistence-first pattern)
	if m.GetSession("pod-1") != nil {
		t.Error("session should not be in memory when persistence fails")
	}
}

// FailingMockStore is a mock store that always fails on SaveRelay
type FailingMockStore struct {
	MockStore
}

func (s *FailingMockStore) SaveRelay(ctx context.Context, relay *RelayInfo) error {
	return fmt.Errorf("simulated persistence failure")
}

// SessionFailingMockStore is a mock store that fails on SaveSession
type SessionFailingMockStore struct {
	MockStore
}

func (s *SessionFailingMockStore) SaveRelay(ctx context.Context, relay *RelayInfo) error {
	return nil // Relay save succeeds
}

func (s *SessionFailingMockStore) SaveSession(ctx context.Context, session *ActiveSession) error {
	return fmt.Errorf("simulated session persistence failure")
}
