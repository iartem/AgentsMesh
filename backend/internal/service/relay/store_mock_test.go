package relay

import (
	"context"
	"testing"
	"time"
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

// === Tests for Store interface types ===

func TestNewMemoryStore(t *testing.T) {
	store := NewMemoryStore()
	if store == nil {
		t.Fatal("NewMemoryStore returned nil")
	}
}

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
