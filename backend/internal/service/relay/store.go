package relay

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/infra/cache"
)

// Store defines the interface for relay data persistence
type Store interface {
	// Relay operations
	SaveRelay(ctx context.Context, relay *RelayInfo) error
	GetRelay(ctx context.Context, relayID string) (*RelayInfo, error)
	GetAllRelays(ctx context.Context) ([]*RelayInfo, error)
	DeleteRelay(ctx context.Context, relayID string) error
	UpdateRelayHeartbeat(ctx context.Context, relayID string, heartbeat time.Time) error

	// Session operations
	SaveSession(ctx context.Context, session *ActiveSession) error
	GetSession(ctx context.Context, podKey string) (*ActiveSession, error)
	GetAllSessions(ctx context.Context) ([]*ActiveSession, error)
	GetSessionsByRelay(ctx context.Context, relayID string) ([]*ActiveSession, error)
	DeleteSession(ctx context.Context, podKey string) error
	UpdateSessionExpiry(ctx context.Context, podKey string, expiry time.Time) error
}

const (
	// Redis key prefixes
	relayKeyPrefix        = "relay:info:"
	relayHeartbeatPrefix  = "relay:heartbeat:"
	relayListKey          = "relay:list"
	sessionKeyPrefix      = "relay:session:"
	sessionListKey        = "relay:session:list"
	sessionByRelayPrefix  = "relay:session:by_relay:"

	// Default TTLs
	relayHeartbeatTTL = 60 * time.Second // Relay heartbeat expires after 60s
	sessionDefaultTTL = 24 * time.Hour   // Session expires after 24h
)

// MemoryStore implements Store interface using in-memory maps
// This is the current implementation (for backward compatibility)
type MemoryStore struct {
	// No state - the manager holds the data
}

// NewMemoryStore creates a new in-memory store
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// RedisStore implements Store interface using Redis
type RedisStore struct {
	cache  *cache.Cache
	prefix string // Optional key prefix for multi-tenant scenarios
}

// NewRedisStore creates a new Redis-backed store
func NewRedisStore(c *cache.Cache, prefix string) *RedisStore {
	return &RedisStore{
		cache:  c,
		prefix: prefix,
	}
}

// key returns a prefixed key
func (s *RedisStore) key(parts ...string) string {
	result := s.prefix
	for _, p := range parts {
		result += p
	}
	return result
}

// SaveRelay saves relay info to Redis
func (s *RedisStore) SaveRelay(ctx context.Context, relay *RelayInfo) error {
	data, err := json.Marshal(relay)
	if err != nil {
		return fmt.Errorf("failed to marshal relay: %w", err)
	}

	// Save relay data (no expiration, managed by heartbeat)
	key := s.key(relayKeyPrefix, relay.ID)
	if err := s.cache.Client().Set(ctx, key, data, 0).Err(); err != nil {
		return fmt.Errorf("failed to save relay: %w", err)
	}

	// Add to relay list set
	if err := s.cache.Client().SAdd(ctx, s.key(relayListKey), relay.ID).Err(); err != nil {
		return fmt.Errorf("failed to add relay to list: %w", err)
	}

	// Update heartbeat with TTL
	heartbeatKey := s.key(relayHeartbeatPrefix, relay.ID)
	if err := s.cache.Client().Set(ctx, heartbeatKey, time.Now().Unix(), relayHeartbeatTTL).Err(); err != nil {
		return fmt.Errorf("failed to set heartbeat: %w", err)
	}

	return nil
}

// GetRelay retrieves relay info from Redis
func (s *RedisStore) GetRelay(ctx context.Context, relayID string) (*RelayInfo, error) {
	key := s.key(relayKeyPrefix, relayID)
	data, err := s.cache.Client().Get(ctx, key).Bytes()
	if err != nil {
		if err.Error() == "redis: nil" {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get relay: %w", err)
	}

	var relay RelayInfo
	if err := json.Unmarshal(data, &relay); err != nil {
		return nil, fmt.Errorf("failed to unmarshal relay: %w", err)
	}

	// Check heartbeat to determine health
	heartbeatKey := s.key(relayHeartbeatPrefix, relayID)
	exists, _ := s.cache.Exists(ctx, heartbeatKey)
	relay.Healthy = exists

	return &relay, nil
}

// GetAllRelays retrieves all relay infos from Redis
func (s *RedisStore) GetAllRelays(ctx context.Context) ([]*RelayInfo, error) {
	// Get all relay IDs from the set
	relayIDs, err := s.cache.Client().SMembers(ctx, s.key(relayListKey)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get relay list: %w", err)
	}

	relays := make([]*RelayInfo, 0, len(relayIDs))
	for _, id := range relayIDs {
		relay, err := s.GetRelay(ctx, id)
		if err != nil {
			continue // Skip errors for individual relays
		}
		if relay != nil {
			relays = append(relays, relay)
		}
	}

	return relays, nil
}

// DeleteRelay removes relay from Redis
func (s *RedisStore) DeleteRelay(ctx context.Context, relayID string) error {
	// Delete relay data
	key := s.key(relayKeyPrefix, relayID)
	if err := s.cache.Client().Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete relay: %w", err)
	}

	// Remove from relay list
	if err := s.cache.Client().SRem(ctx, s.key(relayListKey), relayID).Err(); err != nil {
		return fmt.Errorf("failed to remove relay from list: %w", err)
	}

	// Delete heartbeat
	heartbeatKey := s.key(relayHeartbeatPrefix, relayID)
	s.cache.Client().Del(ctx, heartbeatKey)

	return nil
}

// UpdateRelayHeartbeat updates the heartbeat timestamp for a relay
func (s *RedisStore) UpdateRelayHeartbeat(ctx context.Context, relayID string, heartbeat time.Time) error {
	// Update the heartbeat key with TTL
	heartbeatKey := s.key(relayHeartbeatPrefix, relayID)
	if err := s.cache.Client().Set(ctx, heartbeatKey, heartbeat.Unix(), relayHeartbeatTTL).Err(); err != nil {
		return fmt.Errorf("failed to update heartbeat: %w", err)
	}

	// Also update the LastHeartbeat field in relay data
	relay, err := s.GetRelay(ctx, relayID)
	if err != nil || relay == nil {
		return nil // Relay not found, skip
	}

	relay.LastHeartbeat = heartbeat
	relay.Healthy = true

	data, _ := json.Marshal(relay)
	key := s.key(relayKeyPrefix, relayID)
	return s.cache.Client().Set(ctx, key, data, 0).Err()
}

// SaveSession saves session info to Redis
func (s *RedisStore) SaveSession(ctx context.Context, session *ActiveSession) error {
	data, err := json.Marshal(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Calculate TTL from expiry
	ttl := time.Until(session.ExpireAt)
	if ttl <= 0 {
		ttl = sessionDefaultTTL
	}

	// Save session data with TTL
	key := s.key(sessionKeyPrefix, session.PodKey)
	if err := s.cache.Client().Set(ctx, key, data, ttl).Err(); err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}

	// Add to session list set
	if err := s.cache.Client().SAdd(ctx, s.key(sessionListKey), session.PodKey).Err(); err != nil {
		return fmt.Errorf("failed to add session to list: %w", err)
	}

	// Add to relay-specific session set
	relaySessionKey := s.key(sessionByRelayPrefix, session.RelayID)
	if err := s.cache.Client().SAdd(ctx, relaySessionKey, session.PodKey).Err(); err != nil {
		return fmt.Errorf("failed to add session to relay set: %w", err)
	}

	return nil
}

// GetSession retrieves session info from Redis
func (s *RedisStore) GetSession(ctx context.Context, podKey string) (*ActiveSession, error) {
	key := s.key(sessionKeyPrefix, podKey)
	data, err := s.cache.Client().Get(ctx, key).Bytes()
	if err != nil {
		if err.Error() == "redis: nil" {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	var session ActiveSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}

// GetAllSessions retrieves all sessions from Redis
func (s *RedisStore) GetAllSessions(ctx context.Context) ([]*ActiveSession, error) {
	// Get all session pod keys from the set
	podKeys, err := s.cache.Client().SMembers(ctx, s.key(sessionListKey)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get session list: %w", err)
	}

	sessions := make([]*ActiveSession, 0, len(podKeys))
	for _, pk := range podKeys {
		session, err := s.GetSession(ctx, pk)
		if err != nil {
			continue
		}
		if session != nil {
			sessions = append(sessions, session)
		} else {
			// Session expired, remove from set
			s.cache.Client().SRem(ctx, s.key(sessionListKey), pk)
		}
	}

	return sessions, nil
}

// GetSessionsByRelay retrieves all sessions for a specific relay
func (s *RedisStore) GetSessionsByRelay(ctx context.Context, relayID string) ([]*ActiveSession, error) {
	relaySessionKey := s.key(sessionByRelayPrefix, relayID)
	podKeys, err := s.cache.Client().SMembers(ctx, relaySessionKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get relay sessions: %w", err)
	}

	sessions := make([]*ActiveSession, 0, len(podKeys))
	for _, pk := range podKeys {
		session, err := s.GetSession(ctx, pk)
		if err != nil {
			continue
		}
		if session != nil {
			sessions = append(sessions, session)
		} else {
			// Session expired, remove from set
			s.cache.Client().SRem(ctx, relaySessionKey, pk)
		}
	}

	return sessions, nil
}

// DeleteSession removes session from Redis
func (s *RedisStore) DeleteSession(ctx context.Context, podKey string) error {
	// Get session first to find relay ID
	session, _ := s.GetSession(ctx, podKey)

	// Delete session data
	key := s.key(sessionKeyPrefix, podKey)
	if err := s.cache.Client().Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	// Remove from session list
	s.cache.Client().SRem(ctx, s.key(sessionListKey), podKey)

	// Remove from relay-specific set if we know the relay
	if session != nil {
		relaySessionKey := s.key(sessionByRelayPrefix, session.RelayID)
		s.cache.Client().SRem(ctx, relaySessionKey, podKey)
	}

	return nil
}

// UpdateSessionExpiry updates the expiry time for a session
func (s *RedisStore) UpdateSessionExpiry(ctx context.Context, podKey string, expiry time.Time) error {
	session, err := s.GetSession(ctx, podKey)
	if err != nil || session == nil {
		return nil
	}

	session.ExpireAt = expiry
	return s.SaveSession(ctx, session)
}
