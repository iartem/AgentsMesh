package relay

import (
	"context"
	"fmt"
	"time"
)

// ActiveSession tracks active terminal sessions
type ActiveSession struct {
	PodKey    string    `json:"pod_key"`
	SessionID string    `json:"session_id"`
	RelayURL  string    `json:"relay_url"`
	RelayID   string    `json:"relay_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpireAt  time.Time `json:"expire_at"` // Session expires if runner disconnects
}

// CreateSession creates a new active session
// Returns error if persistence fails (when store is configured)
func (m *Manager) CreateSession(podKey, sessionID string, relay *RelayInfo) (*ActiveSession, error) {
	session := &ActiveSession{
		PodKey:    podKey,
		SessionID: sessionID,
		RelayURL:  relay.URL,
		RelayID:   relay.ID,
		CreatedAt: time.Now(),
		ExpireAt:  time.Now().Add(m.sessionExpiry),
	}

	// Persist to store first (if configured)
	if m.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.store.SaveSession(ctx, session); err != nil {
			m.logger.Error("Failed to persist session to store", "pod_key", podKey, "error", err)
			return nil, fmt.Errorf("failed to persist session: %w", err)
		}
	}

	// Then update memory
	m.mu.Lock()
	m.activeSessions[podKey] = session
	m.mu.Unlock()

	m.logger.Info("Session created",
		"pod_key", podKey,
		"session_id", sessionID,
		"relay_id", relay.ID)

	return session, nil
}

// GetSession returns an active session by pod key
// Returns a copy to prevent data races
func (m *Manager) GetSession(podKey string) *ActiveSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if session, ok := m.activeSessions[podKey]; ok {
		sessionCopy := *session
		return &sessionCopy
	}
	return nil
}

// RemoveSession removes an active session
func (m *Manager) RemoveSession(podKey string) {
	m.mu.Lock()
	session, ok := m.activeSessions[podKey]
	if ok {
		delete(m.activeSessions, podKey)
	}
	m.mu.Unlock()

	if ok {
		// Remove from store
		if m.store != nil {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := m.store.DeleteSession(ctx, podKey); err != nil {
				m.logger.Warn("Failed to delete session from store", "pod_key", podKey, "error", err)
			}
		}
		m.logger.Info("Session removed", "pod_key", podKey, "session_id", session.SessionID)
	}
}

// RefreshSession extends the session expiry
func (m *Manager) RefreshSession(podKey string) {
	m.mu.Lock()
	session, ok := m.activeSessions[podKey]
	if ok {
		session.ExpireAt = time.Now().Add(m.sessionExpiry)
	}
	m.mu.Unlock()

	// Update in store
	if ok && m.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.store.UpdateSessionExpiry(ctx, podKey, session.ExpireAt); err != nil {
			m.logger.Warn("Failed to update session expiry in store", "pod_key", podKey, "error", err)
		}
	}
}

// MigrateSession migrates a session from one relay to another
// Returns the new session and the old relay ID
func (m *Manager) MigrateSession(podKey string, newRelay *RelayInfo) (*ActiveSession, string, error) {
	m.mu.Lock()

	oldSession, exists := m.activeSessions[podKey]
	if !exists {
		m.mu.Unlock()
		return nil, "", fmt.Errorf("session not found: %s", podKey)
	}

	oldRelayID := oldSession.RelayID

	// Create new session on new relay
	newSession := &ActiveSession{
		PodKey:    podKey,
		SessionID: oldSession.SessionID, // Keep same session ID
		RelayURL:  newRelay.URL,
		RelayID:   newRelay.ID,
		CreatedAt: oldSession.CreatedAt, // Keep original creation time
		ExpireAt:  time.Now().Add(m.sessionExpiry),
	}

	m.activeSessions[podKey] = newSession
	m.mu.Unlock()

	// Update in store
	if m.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.store.SaveSession(ctx, newSession); err != nil {
			m.logger.Warn("Failed to update session in store", "pod_key", podKey, "error", err)
		}
	}

	m.logger.Info("Session migrated",
		"pod_key", podKey,
		"from_relay", oldRelayID,
		"to_relay", newRelay.ID)

	return newSession, oldRelayID, nil
}

// GetSessionsByRelay returns all sessions on a specific relay
func (m *Manager) GetSessionsByRelay(relayID string) []*ActiveSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*ActiveSession, 0)
	for _, session := range m.activeSessions {
		if session.RelayID == relayID {
			sessionCopy := *session
			sessions = append(sessions, &sessionCopy)
		}
	}
	return sessions
}

// GetAllSessions returns all active sessions
func (m *Manager) GetAllSessions() []*ActiveSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sessions := make([]*ActiveSession, 0, len(m.activeSessions))
	for _, session := range m.activeSessions {
		sessionCopy := *session
		sessions = append(sessions, &sessionCopy)
	}
	return sessions
}
