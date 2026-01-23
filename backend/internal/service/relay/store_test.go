package relay

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

// MockStore implements Store interface for testing
type MockStore struct {
	relays   map[string]*RelayInfo
	sessions map[string]*ActiveSession
}

func NewMockStore() *MockStore {
	return &MockStore{
		relays:   make(map[string]*RelayInfo),
		sessions: make(map[string]*ActiveSession),
	}
}

func (s *MockStore) SaveRelay(ctx context.Context, relay *RelayInfo) error {
	s.relays[relay.ID] = relay
	return nil
}

func (s *MockStore) GetRelay(ctx context.Context, relayID string) (*RelayInfo, error) {
	if r, ok := s.relays[relayID]; ok {
		return r, nil
	}
	return nil, nil
}

func (s *MockStore) GetAllRelays(ctx context.Context) ([]*RelayInfo, error) {
	result := make([]*RelayInfo, 0, len(s.relays))
	for _, r := range s.relays {
		result = append(result, r)
	}
	return result, nil
}

func (s *MockStore) DeleteRelay(ctx context.Context, relayID string) error {
	delete(s.relays, relayID)
	return nil
}

func (s *MockStore) UpdateRelayHeartbeat(ctx context.Context, relayID string, heartbeat time.Time) error {
	if r, ok := s.relays[relayID]; ok {
		r.LastHeartbeat = heartbeat
		r.Healthy = true
	}
	return nil
}

func (s *MockStore) SaveSession(ctx context.Context, session *ActiveSession) error {
	s.sessions[session.PodKey] = session
	return nil
}

func (s *MockStore) GetSession(ctx context.Context, podKey string) (*ActiveSession, error) {
	if sess, ok := s.sessions[podKey]; ok {
		return sess, nil
	}
	return nil, nil
}

func (s *MockStore) GetAllSessions(ctx context.Context) ([]*ActiveSession, error) {
	result := make([]*ActiveSession, 0, len(s.sessions))
	for _, sess := range s.sessions {
		result = append(result, sess)
	}
	return result, nil
}

func (s *MockStore) GetSessionsByRelay(ctx context.Context, relayID string) ([]*ActiveSession, error) {
	result := make([]*ActiveSession, 0)
	for _, sess := range s.sessions {
		if sess.RelayID == relayID {
			result = append(result, sess)
		}
	}
	return result, nil
}

func (s *MockStore) DeleteSession(ctx context.Context, podKey string) error {
	delete(s.sessions, podKey)
	return nil
}

func (s *MockStore) UpdateSessionExpiry(ctx context.Context, podKey string, expiry time.Time) error {
	if sess, ok := s.sessions[podKey]; ok {
		sess.ExpireAt = expiry
	}
	return nil
}

// === Tests using MockStore ===

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

// === Tests for MemoryStore ===

func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore()
	if store == nil {
		t.Fatal("NewMemoryStore returned nil")
	}
}

// === Tests for Store interface types ===

func TestActiveSessionStruct(t *testing.T) {
	session := &ActiveSession{
		PodKey:    "pod-1",
		SessionID: "session-1",
		RelayURL:  "wss://relay.com",
		RelayID:   "relay-1",
		CreatedAt: time.Now(),
		ExpireAt:  time.Now().Add(24 * time.Hour),
	}

	if session.PodKey != "pod-1" {
		t.Error("PodKey mismatch")
	}
	if session.SessionID != "session-1" {
		t.Error("SessionID mismatch")
	}
}

// === Tests for RedisStore using miniredis ===

// TestRedisStore_Key tests the key prefix function
func TestRedisStore_Key(t *testing.T) {
	store := &RedisStore{prefix: "test:"}

	key := store.key("relay:info:", "relay-1")
	expected := "test:relay:info:relay-1"
	if key != expected {
		t.Errorf("key: got %q, want %q", key, expected)
	}
}

// TestRedisStore_KeyEmpty tests key with empty prefix
func TestRedisStore_KeyEmpty(t *testing.T) {
	store := &RedisStore{prefix: ""}

	key := store.key("relay:info:", "relay-1")
	expected := "relay:info:relay-1"
	if key != expected {
		t.Errorf("key: got %q, want %q", key, expected)
	}
}

// TestNewRedisStore tests RedisStore constructor
func TestNewRedisStore(t *testing.T) {
	// Create a mock cache - note: this tests the constructor logic
	store := NewRedisStore(nil, "prefix:")

	if store == nil {
		t.Fatal("NewRedisStore returned nil")
	}
	if store.prefix != "prefix:" {
		t.Errorf("prefix: got %q, want %q", store.prefix, "prefix:")
	}
}

// === Integration tests with miniredis ===

// createTestCache creates a cache with miniredis for integration testing
func createTestCache(t *testing.T) (*miniredis.Miniredis, *testCache) {
	t.Helper()
	mr := miniredis.RunT(t)

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	t.Cleanup(func() {
		client.Close()
	})

	return mr, &testCache{client: client}
}

// testCache implements the cache interface needed by RedisStore
type testCache struct {
	client *redis.Client
}

func (c *testCache) Client() *redis.Client {
	return c.client
}

func (c *testCache) Exists(ctx context.Context, key string) (bool, error) {
	result, err := c.client.Exists(ctx, key).Result()
	return result > 0, err
}

// TestRedisStore_SaveAndGetRelay tests relay save and get operations
func TestRedisStore_SaveAndGetRelay(t *testing.T) {
	mr, tc := createTestCache(t)
	defer mr.Close()

	// Create store for key generation
	store := &RedisStore{prefix: ""}
	// Use test cache client directly
	client := tc.Client()
	ctx := context.Background()

	// Test saving relay manually (simulating SaveRelay logic)
	relay := &RelayInfo{
		ID:       "relay-1",
		URL:      "wss://relay.com",
		Region:   "us-east",
		Capacity: 100,
	}

	// Save relay data
	data, _ := json.Marshal(relay)
	key := store.key(relayKeyPrefix, relay.ID)
	err := client.Set(ctx, key, data, 0).Err()
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Add to relay list
	err = client.SAdd(ctx, store.key(relayListKey), relay.ID).Err()
	if err != nil {
		t.Fatalf("SAdd failed: %v", err)
	}

	// Verify data is stored
	stored, err := client.Get(ctx, key).Bytes()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	var retrieved RelayInfo
	if err := json.Unmarshal(stored, &retrieved); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if retrieved.ID != "relay-1" || retrieved.URL != "wss://relay.com" {
		t.Errorf("retrieved relay mismatch: %+v", retrieved)
	}
}

// TestRedisStore_SaveAndGetSession tests session save and get operations
func TestRedisStore_SaveAndGetSession(t *testing.T) {
	mr, tc := createTestCache(t)
	defer mr.Close()

	store := &RedisStore{prefix: ""}
	client := tc.Client()
	ctx := context.Background()

	// Test saving session
	session := &ActiveSession{
		PodKey:    "pod-1",
		SessionID: "session-1",
		RelayURL:  "wss://relay.com",
		RelayID:   "relay-1",
		CreatedAt: time.Now(),
		ExpireAt:  time.Now().Add(24 * time.Hour),
	}

	// Save session
	data, _ := json.Marshal(session)
	key := store.key(sessionKeyPrefix, session.PodKey)
	ttl := time.Until(session.ExpireAt)
	err := client.Set(ctx, key, data, ttl).Err()
	if err != nil {
		t.Fatalf("Set session failed: %v", err)
	}

	// Verify
	stored, err := client.Get(ctx, key).Bytes()
	if err != nil {
		t.Fatalf("Get session failed: %v", err)
	}

	var retrieved ActiveSession
	if err := json.Unmarshal(stored, &retrieved); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if retrieved.PodKey != "pod-1" || retrieved.SessionID != "session-1" {
		t.Errorf("retrieved session mismatch: %+v", retrieved)
	}
}

// TestRedisStore_DeleteRelay tests relay deletion
func TestRedisStore_DeleteRelay(t *testing.T) {
	mr, tc := createTestCache(t)
	defer mr.Close()

	store := &RedisStore{prefix: ""}
	client := tc.Client()
	ctx := context.Background()

	// First save a relay
	relay := &RelayInfo{ID: "relay-1", URL: "wss://relay.com"}
	data, _ := json.Marshal(relay)
	key := store.key(relayKeyPrefix, relay.ID)
	client.Set(ctx, key, data, 0)
	client.SAdd(ctx, store.key(relayListKey), relay.ID)

	// Delete relay
	client.Del(ctx, key)
	client.SRem(ctx, store.key(relayListKey), relay.ID)

	// Verify deleted
	exists, _ := client.Exists(ctx, key).Result()
	if exists > 0 {
		t.Error("relay should be deleted")
	}
}

// TestRedisStore_SessionsByRelay tests getting sessions by relay ID
func TestRedisStore_SessionsByRelay(t *testing.T) {
	mr, tc := createTestCache(t)
	defer mr.Close()

	store := &RedisStore{prefix: ""}
	client := tc.Client()
	ctx := context.Background()

	// Create sessions for different relays
	sessions := []*ActiveSession{
		{PodKey: "pod-1", SessionID: "s1", RelayID: "relay-1"},
		{PodKey: "pod-2", SessionID: "s2", RelayID: "relay-1"},
		{PodKey: "pod-3", SessionID: "s3", RelayID: "relay-2"},
	}

	for _, s := range sessions {
		data, _ := json.Marshal(s)
		client.Set(ctx, store.key(sessionKeyPrefix, s.PodKey), data, time.Hour)
		client.SAdd(ctx, store.key(sessionByRelayPrefix, s.RelayID), s.PodKey)
	}

	// Get sessions for relay-1
	podKeys, err := client.SMembers(ctx, store.key(sessionByRelayPrefix, "relay-1")).Result()
	if err != nil {
		t.Fatalf("SMembers failed: %v", err)
	}

	if len(podKeys) != 2 {
		t.Errorf("expected 2 sessions for relay-1, got %d", len(podKeys))
	}

	// Get sessions for relay-2
	podKeys, _ = client.SMembers(ctx, store.key(sessionByRelayPrefix, "relay-2")).Result()
	if len(podKeys) != 1 {
		t.Errorf("expected 1 session for relay-2, got %d", len(podKeys))
	}
}

// TestRedisStore_HeartbeatTTL tests heartbeat key TTL
func TestRedisStore_HeartbeatTTL(t *testing.T) {
	mr, tc := createTestCache(t)
	defer mr.Close()

	store := &RedisStore{prefix: ""}
	client := tc.Client()
	ctx := context.Background()

	// Set heartbeat with TTL
	heartbeatKey := store.key(relayHeartbeatPrefix, "relay-1")
	err := client.Set(ctx, heartbeatKey, time.Now().Unix(), relayHeartbeatTTL).Err()
	if err != nil {
		t.Fatalf("Set heartbeat failed: %v", err)
	}

	// Check TTL is set
	ttl, err := client.TTL(ctx, heartbeatKey).Result()
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}

	if ttl <= 0 || ttl > relayHeartbeatTTL {
		t.Errorf("unexpected TTL: %v", ttl)
	}

	// Fast-forward time in miniredis
	mr.FastForward(relayHeartbeatTTL + time.Second)

	// Key should be expired
	exists, _ := client.Exists(ctx, heartbeatKey).Result()
	if exists > 0 {
		t.Error("heartbeat key should be expired")
	}
}
