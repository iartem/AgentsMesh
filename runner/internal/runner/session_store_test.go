package runner

import (
	"sync"
	"testing"
)

func TestNewInMemorySessionStore(t *testing.T) {
	store := NewInMemorySessionStore()

	if store == nil {
		t.Fatal("NewInMemorySessionStore returned nil")
	}

	if store.sessions == nil {
		t.Error("sessions should be initialized")
	}
}

func TestInMemorySessionStoreGet(t *testing.T) {
	store := NewInMemorySessionStore()

	// Test non-existent
	_, ok := store.Get("nonexistent")
	if ok {
		t.Error("Get should return false for nonexistent session")
	}

	// Add a session
	session := &Session{ID: "session-1", SessionKey: "session-1"}
	store.Put("session-1", session)

	// Test existent
	retrieved, ok := store.Get("session-1")
	if !ok {
		t.Error("Get should return true for existing session")
	}

	if retrieved.ID != "session-1" {
		t.Errorf("ID: got %v, want session-1", retrieved.ID)
	}
}

func TestInMemorySessionStorePut(t *testing.T) {
	store := NewInMemorySessionStore()

	session := &Session{ID: "session-1", SessionKey: "session-1"}
	store.Put("session-1", session)

	if store.Count() != 1 {
		t.Errorf("Count after Put: got %v, want 1", store.Count())
	}
}

func TestInMemorySessionStoreDelete(t *testing.T) {
	store := NewInMemorySessionStore()

	session := &Session{ID: "session-1", SessionKey: "session-1"}
	store.Put("session-1", session)

	// Delete existing
	deleted := store.Delete("session-1")
	if deleted == nil {
		t.Error("Delete should return the deleted session")
	}

	if store.Count() != 0 {
		t.Errorf("Count after Delete: got %v, want 0", store.Count())
	}

	// Delete non-existing
	deleted = store.Delete("nonexistent")
	if deleted != nil {
		t.Error("Delete should return nil for nonexistent session")
	}
}

func TestInMemorySessionStoreCount(t *testing.T) {
	store := NewInMemorySessionStore()

	if store.Count() != 0 {
		t.Errorf("initial Count: got %v, want 0", store.Count())
	}

	store.Put("session-1", &Session{ID: "session-1"})
	store.Put("session-2", &Session{ID: "session-2"})

	if store.Count() != 2 {
		t.Errorf("Count after Put: got %v, want 2", store.Count())
	}
}

func TestInMemorySessionStoreAll(t *testing.T) {
	store := NewInMemorySessionStore()

	// Empty
	sessions := store.All()
	if len(sessions) != 0 {
		t.Errorf("All on empty: got %v, want 0", len(sessions))
	}

	// With sessions
	store.Put("session-1", &Session{ID: "session-1"})
	store.Put("session-2", &Session{ID: "session-2"})

	sessions = store.All()
	if len(sessions) != 2 {
		t.Errorf("All: got %v, want 2", len(sessions))
	}
}

func TestInMemorySessionStoreConcurrency(t *testing.T) {
	store := NewInMemorySessionStore()
	var wg sync.WaitGroup

	// Concurrent puts
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := "session-" + string(rune('0'+id%10))
			store.Put(key, &Session{ID: key})
		}(i)
	}

	// Concurrent gets
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			key := "session-" + string(rune('0'+id%10))
			store.Get(key)
		}(i)
	}

	wg.Wait()

	// Should not panic
	count := store.Count()
	if count > 10 {
		t.Errorf("Count should be at most 10, got %v", count)
	}
}

func TestInMemorySessionStoreUpdate(t *testing.T) {
	store := NewInMemorySessionStore()

	session1 := &Session{ID: "session-1", Status: SessionStatusInitializing}
	store.Put("session-1", session1)

	// Update the session
	session2 := &Session{ID: "session-1", Status: SessionStatusRunning}
	store.Put("session-1", session2)

	if store.Count() != 1 {
		t.Errorf("Count: got %v, want 1", store.Count())
	}

	retrieved, ok := store.Get("session-1")
	if !ok {
		t.Error("session should exist")
	}

	if retrieved.Status != SessionStatusRunning {
		t.Errorf("Status: got %v, want running", retrieved.Status)
	}
}

// Benchmarks

func BenchmarkInMemorySessionStoreGet(b *testing.B) {
	store := NewInMemorySessionStore()
	store.Put("session-1", &Session{ID: "session-1"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Get("session-1")
	}
}

func BenchmarkInMemorySessionStorePut(b *testing.B) {
	store := NewInMemorySessionStore()
	session := &Session{ID: "session-1"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Put("session-1", session)
	}
}

func BenchmarkInMemorySessionStoreCount(b *testing.B) {
	store := NewInMemorySessionStore()
	for i := 0; i < 10; i++ {
		store.Put("session-"+string(rune('0'+i)), &Session{ID: "session-" + string(rune('0'+i))})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.Count()
	}
}

func BenchmarkInMemorySessionStoreAll(b *testing.B) {
	store := NewInMemorySessionStore()
	for i := 0; i < 100; i++ {
		key := "session-" + string(rune('0'+i%10)) + string(rune('0'+i/10))
		store.Put(key, &Session{ID: key})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.All()
	}
}
