package relay

import (
	"context"
	"time"
)

// healthCheckLoop periodically checks relay health
func (m *Manager) healthCheckLoop() {
	ticker := time.NewTicker(m.healthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			m.logger.Info("Health check loop stopped")
			return
		case <-ticker.C:
			m.doHealthCheck()
		}
	}
}

// doHealthCheck performs a single health check iteration
func (m *Manager) doHealthCheck() {
	now := time.Now()
	healthyTimeout := 30 * time.Second

	// Step 1: Read-lock to collect items that need attention
	m.mu.RLock()
	var unhealthyRelays []string
	var expiredSessions []string

	for id, relay := range m.relays {
		if now.Sub(relay.LastHeartbeat) > healthyTimeout && relay.Healthy {
			unhealthyRelays = append(unhealthyRelays, id)
		}
	}

	for podKey, session := range m.activeSessions {
		if now.After(session.ExpireAt) {
			expiredSessions = append(expiredSessions, podKey)
		}
	}
	m.mu.RUnlock()

	// Step 2: Process unhealthy relays (needs write lock)
	for _, relayID := range unhealthyRelays {
		m.markRelayUnhealthy(relayID)
	}

	// Step 3: Process expired sessions (needs write lock)
	if len(expiredSessions) > 0 {
		m.mu.Lock()
		for _, podKey := range expiredSessions {
			if session, ok := m.activeSessions[podKey]; ok {
				delete(m.activeSessions, podKey)
				m.logger.Info("Session expired", "pod_key", podKey, "session_id", session.SessionID)

				// Remove from store
				if m.store != nil {
					go func(pk string) {
						ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
						defer cancel()
						if err := m.store.DeleteSession(ctx, pk); err != nil {
							m.logger.Warn("Failed to delete expired session from store", "pod_key", pk, "error", err)
						}
					}(podKey)
				}
			}
		}
		m.mu.Unlock()
	}
}

// markRelayUnhealthy marks a relay as unhealthy and triggers auto-migration if configured
func (m *Manager) markRelayUnhealthy(relayID string) {
	m.mu.Lock()
	relay, ok := m.relays[relayID]
	if !ok || !relay.Healthy {
		m.mu.Unlock()
		return
	}

	relay.Healthy = false
	m.logger.Warn("Relay marked unhealthy", "relay_id", relayID, "last_heartbeat", relay.LastHeartbeat)

	// Collect affected sessions if auto-migration is configured
	var affectedSessions []*ActiveSession
	if m.onRelayUnhealthy != nil {
		for _, session := range m.activeSessions {
			if session.RelayID == relayID {
				sessionCopy := *session
				affectedSessions = append(affectedSessions, &sessionCopy)
			}
		}
	}
	m.mu.Unlock()

	// Trigger auto-migration callback asynchronously
	if m.onRelayUnhealthy != nil && len(affectedSessions) > 0 {
		go m.onRelayUnhealthy(relayID, affectedSessions)
	}
}
