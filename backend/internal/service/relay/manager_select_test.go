package relay

import (
	"testing"
)

func TestSelectRelay(t *testing.T) {
	m := NewManager()
	if err := m.Register(&RelayInfo{ID: "relay-us", URL: "wss://us.relay.com", Region: "us-east", Capacity: 100, Healthy: true}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := m.Register(&RelayInfo{ID: "relay-eu", URL: "wss://eu.relay.com", Region: "eu-west", Capacity: 100, Healthy: true}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

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
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://r1.com", Region: "us", Capacity: 100, CurrentConnections: 90}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := m.Register(&RelayInfo{ID: "relay-2", URL: "wss://r2.com", Region: "us", Capacity: 100, CurrentConnections: 10}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

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
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://r1.com"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := m.Register(&RelayInfo{ID: "relay-2", URL: "wss://r2.com"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

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
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://r1.com"}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Mark as unhealthy directly
	m.mu.Lock()
	m.relays["relay-1"].Healthy = false
	m.mu.Unlock()

	relay := m.SelectRelay("us")
	if relay != nil {
		t.Error("should return nil when no healthy relays")
	}
}

func TestSelectRelayForPod(t *testing.T) {
	m := NewManager()
	relay := &RelayInfo{ID: "relay-1", URL: "wss://relay.example.com", Healthy: true}
	if err := m.Register(relay); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

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

func TestSelectRelayWithOptions(t *testing.T) {
	m := NewManager()
	if err := m.Register(&RelayInfo{ID: "relay-1", URL: "wss://r1.com", Region: "us-east", Capacity: 100, Healthy: true}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := m.Register(&RelayInfo{ID: "relay-2", URL: "wss://r2.com", Region: "us-east", Capacity: 100, Healthy: true}); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

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
