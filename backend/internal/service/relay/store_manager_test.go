package relay

import (
	"context"
	"testing"
	"time"
)

func TestManagerWithStore(t *testing.T) {
	store := NewMockStore()
	m := NewManagerWithOptions(WithStore(store))

	if m == nil {
		t.Fatal("NewManagerWithOptions returned nil")
	}
	if m.store != store {
		t.Error("store not set")
	}
}

func TestRegisterPersistsToStore(t *testing.T) {
	store := NewMockStore()
	m := NewManagerWithOptions(WithStore(store))

	relay := &RelayInfo{ID: "relay-1", URL: "wss://r1.com", Region: "us-east"}
	if err := m.Register(relay); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Verify persisted to store
	ctx := context.Background()
	stored, err := store.GetRelay(ctx, "relay-1")
	if err != nil {
		t.Fatalf("GetRelay: %v", err)
	}
	if stored == nil {
		t.Fatal("relay not persisted to store")
	}
	if stored.ID != "relay-1" || stored.Region != "us-east" {
		t.Error("relay data mismatch")
	}
}

func TestCreateSessionPersistsToStore(t *testing.T) {
	store := NewMockStore()
	m := NewManagerWithOptions(WithStore(store))

	relay := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	if err := m.Register(relay); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	if _, err := m.CreateSession("pod-1", "session-1", relay); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Verify persisted to store
	ctx := context.Background()
	stored, err := store.GetSession(ctx, "pod-1")
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if stored == nil {
		t.Fatal("session not persisted to store")
	}
	if stored.SessionID != "session-1" || stored.RelayID != "relay-1" {
		t.Error("session data mismatch")
	}
}

func TestRemoveSessionDeletesFromStore(t *testing.T) {
	store := NewMockStore()
	m := NewManagerWithOptions(WithStore(store))

	relay := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	if err := m.Register(relay); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if _, err := m.CreateSession("pod-1", "session-1", relay); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Remove session
	m.RemoveSession("pod-1")

	// Verify removed from store
	ctx := context.Background()
	stored, _ := store.GetSession(ctx, "pod-1")
	if stored != nil {
		t.Error("session should be deleted from store")
	}
}

func TestUnregisterDeletesFromStore(t *testing.T) {
	store := NewMockStore()
	m := NewManagerWithOptions(WithStore(store))

	relay := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	if err := m.Register(relay); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Unregister
	m.Unregister("relay-1")

	// Verify removed from store
	ctx := context.Background()
	stored, _ := store.GetRelay(ctx, "relay-1")
	if stored != nil {
		t.Error("relay should be deleted from store")
	}
}

func TestRefreshSessionUpdatesStore(t *testing.T) {
	store := NewMockStore()
	m := NewManagerWithOptions(WithStore(store))

	relay := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	if err := m.Register(relay); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if _, err := m.CreateSession("pod-1", "session-1", relay); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Get original expiry
	ctx := context.Background()
	original, _ := store.GetSession(ctx, "pod-1")
	originalExpiry := original.ExpireAt

	// Wait a bit and refresh
	time.Sleep(10 * time.Millisecond)
	m.RefreshSession("pod-1")

	// Verify updated in store
	updated, _ := store.GetSession(ctx, "pod-1")
	if !updated.ExpireAt.After(originalExpiry) {
		t.Error("expiry should be extended in store")
	}
}

func TestMigrateSessionUpdatesStore(t *testing.T) {
	store := NewMockStore()
	m := NewManagerWithOptions(WithStore(store))

	relay1 := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	relay2 := &RelayInfo{ID: "relay-2", URL: "wss://r2.com"}
	if err := m.Register(relay1); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if err := m.Register(relay2); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if _, err := m.CreateSession("pod-1", "session-1", relay1); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Migrate
	m.MigrateSession("pod-1", relay2)

	// Verify updated in store
	ctx := context.Background()
	stored, _ := store.GetSession(ctx, "pod-1")
	if stored == nil {
		t.Fatal("session should exist in store")
	}
	if stored.RelayID != "relay-2" {
		t.Errorf("session relay should be updated: got %q", stored.RelayID)
	}
}

func TestForceUnregisterCleansUpStore(t *testing.T) {
	store := NewMockStore()
	m := NewManagerWithOptions(WithStore(store))

	relay := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	if err := m.Register(relay); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if _, err := m.CreateSession("pod-1", "s1", relay); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if _, err := m.CreateSession("pod-2", "s2", relay); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Force unregister
	m.ForceUnregister("relay-1")

	// Verify all cleaned up in store
	ctx := context.Background()
	if r, _ := store.GetRelay(ctx, "relay-1"); r != nil {
		t.Error("relay should be deleted from store")
	}
	if s, _ := store.GetSession(ctx, "pod-1"); s != nil {
		t.Error("session pod-1 should be deleted from store")
	}
	if s, _ := store.GetSession(ctx, "pod-2"); s != nil {
		t.Error("session pod-2 should be deleted from store")
	}
}

func TestGracefulUnregisterCleansUpStore(t *testing.T) {
	store := NewMockStore()
	m := NewManagerWithOptions(WithStore(store))

	relay := &RelayInfo{ID: "relay-1", URL: "wss://r1.com"}
	if err := m.Register(relay); err != nil {
		t.Fatalf("Register failed: %v", err)
	}
	if _, err := m.CreateSession("pod-1", "s1", relay); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Graceful unregister
	m.GracefulUnregister("relay-1", "shutdown")

	// Verify all cleaned up in store
	ctx := context.Background()
	if r, _ := store.GetRelay(ctx, "relay-1"); r != nil {
		t.Error("relay should be deleted from store")
	}
	if s, _ := store.GetSession(ctx, "pod-1"); s != nil {
		t.Error("session should be deleted from store")
	}
}
