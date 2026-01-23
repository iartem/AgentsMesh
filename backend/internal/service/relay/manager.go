package relay

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// RelayInfo holds information about a relay server
type RelayInfo struct {
	ID                 string    `json:"id"`
	URL                string    `json:"url"`          // Public WebSocket URL for browsers (wss://relay.example.com)
	InternalURL        string    `json:"internal_url"` // Internal WebSocket URL for runners (ws://relay:8090 in Docker)
	Region             string    `json:"region"`       // Geographic region
	Capacity           int       `json:"capacity"`     // Maximum connections
	CurrentConnections int       `json:"connections"`  // Current active connections
	CPUUsage           float64   `json:"cpu_usage"`    // CPU usage percentage
	MemoryUsage        float64   `json:"memory_usage"` // Memory usage percentage
	LastHeartbeat      time.Time `json:"last_heartbeat"`
	Healthy            bool      `json:"healthy"`

	// Metrics for enhanced load balancing
	AvgLatencyMs int `json:"avg_latency_ms"` // Average heartbeat latency in milliseconds
}

// LoadBalancingConfig holds configuration for relay selection algorithm
type LoadBalancingConfig struct {
	ConnectionWeight float64 // Weight for connection count factor (default: 0.4)
	CPUWeight        float64 // Weight for CPU usage factor (default: 0.25)
	MemoryWeight     float64 // Weight for memory usage factor (default: 0.15)
	LatencyWeight    float64 // Weight for latency factor (default: 0.1)
	RegionBonus      float64 // Bonus score for same region (default: 50)
}

// DefaultLoadBalancingConfig returns default load balancing configuration
func DefaultLoadBalancingConfig() LoadBalancingConfig {
	return LoadBalancingConfig{
		ConnectionWeight: 0.4,
		CPUWeight:        0.25,
		MemoryWeight:     0.15,
		LatencyWeight:    0.1,
		RegionBonus:      50,
	}
}

// GetRunnerURL returns the URL that runners should use to connect.
// Returns InternalURL if set, otherwise falls back to URL.
func (r *RelayInfo) GetRunnerURL() string {
	if r.InternalURL != "" {
		return r.InternalURL
	}
	return r.URL
}

// ActiveSession tracks active terminal sessions
type ActiveSession struct {
	PodKey    string    `json:"pod_key"`
	SessionID string    `json:"session_id"`
	RelayURL  string    `json:"relay_url"`
	RelayID   string    `json:"relay_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpireAt  time.Time `json:"expire_at"` // Session expires if runner disconnects
}

// Manager manages relay servers
type Manager struct {
	relays         map[string]*RelayInfo     // relayID -> info (in-memory cache)
	activeSessions map[string]*ActiveSession // podKey -> session (in-memory cache)
	mu             sync.RWMutex

	healthCheckInterval time.Duration
	sessionExpiry       time.Duration
	loadBalancingConfig LoadBalancingConfig

	// Optional persistent store (Redis)
	store Store

	// Lifecycle management
	stopCh  chan struct{}
	stopped bool

	// Auto-migration callback (optional)
	onRelayUnhealthy func(relayID string, sessions []*ActiveSession)

	logger *slog.Logger
}

// ManagerOption is a functional option for Manager
type ManagerOption func(*Manager)

// WithStore sets a persistent store for the manager
func WithStore(store Store) ManagerOption {
	return func(m *Manager) {
		m.store = store
	}
}

// WithLoadBalancingConfig sets the load balancing configuration
func WithLoadBalancingConfig(cfg LoadBalancingConfig) ManagerOption {
	return func(m *Manager) {
		m.loadBalancingConfig = cfg
	}
}

// WithOnRelayUnhealthy sets a callback for when a relay becomes unhealthy
// The callback receives the relay ID and affected sessions for migration
func WithOnRelayUnhealthy(fn func(relayID string, sessions []*ActiveSession)) ManagerOption {
	return func(m *Manager) {
		m.onRelayUnhealthy = fn
	}
}

// WithHealthCheckInterval sets the interval for health checks
func WithHealthCheckInterval(interval time.Duration) ManagerOption {
	return func(m *Manager) {
		m.healthCheckInterval = interval
	}
}

// NewManager creates a new relay manager
func NewManager() *Manager {
	return NewManagerWithOptions()
}

// NewManagerWithConfig creates a new relay manager with custom load balancing config
// Deprecated: Use NewManagerWithOptions instead
func NewManagerWithConfig(lbConfig LoadBalancingConfig) *Manager {
	return NewManagerWithOptions(WithLoadBalancingConfig(lbConfig))
}

// NewManagerWithOptions creates a new relay manager with options
func NewManagerWithOptions(opts ...ManagerOption) *Manager {
	m := &Manager{
		relays:              make(map[string]*RelayInfo),
		activeSessions:      make(map[string]*ActiveSession),
		healthCheckInterval: 30 * time.Second,
		sessionExpiry:       24 * time.Hour,
		loadBalancingConfig: DefaultLoadBalancingConfig(),
		stopCh:              make(chan struct{}),
		logger:              slog.With("component", "relay_manager"),
	}

	// Apply options
	for _, opt := range opts {
		opt(m)
	}

	// Load from persistent store if available
	if m.store != nil {
		m.loadFromStore()
	}

	// Start background health check
	go m.healthCheckLoop()

	return m
}

// Stop gracefully stops the manager and its background goroutines
func (m *Manager) Stop() {
	m.mu.Lock()
	if m.stopped {
		m.mu.Unlock()
		return
	}
	m.stopped = true
	m.mu.Unlock()

	close(m.stopCh)
	m.logger.Info("Relay manager stopped")
}

// IsStopped returns true if the manager has been stopped
func (m *Manager) IsStopped() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stopped
}

// loadFromStore loads relays and sessions from the persistent store
func (m *Manager) loadFromStore() {
	if m.store == nil {
		return
	}

	ctx := context.Background()

	// Load relays
	relays, err := m.store.GetAllRelays(ctx)
	if err != nil {
		m.logger.Warn("Failed to load relays from store", "error", err)
	} else {
		m.mu.Lock()
		for _, r := range relays {
			m.relays[r.ID] = r
		}
		m.mu.Unlock()
		m.logger.Info("Loaded relays from store", "count", len(relays))
	}

	// Load sessions
	sessions, err := m.store.GetAllSessions(ctx)
	if err != nil {
		m.logger.Warn("Failed to load sessions from store", "error", err)
	} else {
		m.mu.Lock()
		for _, s := range sessions {
			m.activeSessions[s.PodKey] = s
		}
		m.mu.Unlock()
		m.logger.Info("Loaded sessions from store", "count", len(sessions))
	}
}

// Register registers a new relay or updates existing one
// Returns error if persistence fails (when store is configured)
func (m *Manager) Register(info *RelayInfo) error {
	info.LastHeartbeat = time.Now()
	info.Healthy = true

	// Persist to store first (if configured)
	if m.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.store.SaveRelay(ctx, info); err != nil {
			m.logger.Error("Failed to persist relay to store", "relay_id", info.ID, "error", err)
			return fmt.Errorf("failed to persist relay: %w", err)
		}
	}

	// Then update memory
	m.mu.Lock()
	m.relays[info.ID] = info
	m.mu.Unlock()

	m.logger.Info("Relay registered",
		"relay_id", info.ID,
		"url", info.URL,
		"region", info.Region,
		"capacity", info.Capacity)

	return nil
}

// Heartbeat updates relay health status
func (m *Manager) Heartbeat(relayID string, connections int, cpuUsage, memoryUsage float64) error {
	return m.HeartbeatWithLatency(relayID, connections, cpuUsage, memoryUsage, 0)
}

// HeartbeatWithLatency updates relay health status including latency metric
func (m *Manager) HeartbeatWithLatency(relayID string, connections int, cpuUsage, memoryUsage float64, latencyMs int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	relay, ok := m.relays[relayID]
	if !ok {
		return fmt.Errorf("relay not found: %s", relayID)
	}

	relay.CurrentConnections = connections
	relay.CPUUsage = cpuUsage
	relay.MemoryUsage = memoryUsage
	relay.LastHeartbeat = time.Now()
	relay.Healthy = true

	// Update latency with exponential moving average for smoothing
	if latencyMs > 0 {
		if relay.AvgLatencyMs == 0 {
			relay.AvgLatencyMs = latencyMs
		} else {
			// EMA with alpha = 0.3 for moderate smoothing
			relay.AvgLatencyMs = int(float64(relay.AvgLatencyMs)*0.7 + float64(latencyMs)*0.3)
		}
	}

	return nil
}

// Unregister removes a relay
func (m *Manager) Unregister(relayID string) {
	m.mu.Lock()
	delete(m.relays, relayID)
	m.mu.Unlock()

	// Remove from store
	if m.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.store.DeleteRelay(ctx, relayID); err != nil {
			m.logger.Warn("Failed to delete relay from store", "relay_id", relayID, "error", err)
		}
	}

	m.logger.Info("Relay unregistered", "relay_id", relayID)
}

// ForceUnregister removes a relay and returns affected sessions for migration
func (m *Manager) ForceUnregister(relayID string) []*ActiveSession {
	m.mu.Lock()

	// Find all sessions using this relay
	affectedSessions := make([]*ActiveSession, 0)
	affectedPodKeys := make([]string, 0)
	for podKey, session := range m.activeSessions {
		if session.RelayID == relayID {
			sessionCopy := *session
			affectedSessions = append(affectedSessions, &sessionCopy)
			affectedPodKeys = append(affectedPodKeys, podKey)
			delete(m.activeSessions, podKey)
		}
	}

	// Remove the relay
	delete(m.relays, relayID)
	m.mu.Unlock()

	// Remove from store
	if m.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Delete relay
		if err := m.store.DeleteRelay(ctx, relayID); err != nil {
			m.logger.Warn("Failed to delete relay from store", "relay_id", relayID, "error", err)
		}

		// Delete affected sessions
		for _, podKey := range affectedPodKeys {
			if err := m.store.DeleteSession(ctx, podKey); err != nil {
				m.logger.Warn("Failed to delete session from store", "pod_key", podKey, "error", err)
			}
		}
	}

	m.logger.Info("Relay force unregistered",
		"relay_id", relayID,
		"affected_sessions", len(affectedSessions))

	return affectedSessions
}

// GracefulUnregister marks a relay as offline (graceful shutdown from relay itself)
// It doesn't immediately remove the relay record, allowing for session migration
// Returns affected sessions that need to be migrated
func (m *Manager) GracefulUnregister(relayID string, reason string) []*ActiveSession {
	m.mu.Lock()

	relay, ok := m.relays[relayID]
	if !ok {
		m.mu.Unlock()
		return nil
	}

	// Mark as unhealthy so no new sessions are assigned
	relay.Healthy = false

	// Find all sessions using this relay
	affectedSessions := make([]*ActiveSession, 0)
	affectedPodKeys := make([]string, 0)
	for podKey, session := range m.activeSessions {
		if session.RelayID == relayID {
			sessionCopy := *session
			affectedSessions = append(affectedSessions, &sessionCopy)
			affectedPodKeys = append(affectedPodKeys, podKey)
			// Don't delete sessions immediately - let migration happen first
			delete(m.activeSessions, podKey)
		}
	}

	// Remove the relay after collecting sessions
	delete(m.relays, relayID)
	m.mu.Unlock()

	// Remove from store
	if m.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Delete relay
		if err := m.store.DeleteRelay(ctx, relayID); err != nil {
			m.logger.Warn("Failed to delete relay from store", "relay_id", relayID, "error", err)
		}

		// Delete affected sessions
		for _, podKey := range affectedPodKeys {
			if err := m.store.DeleteSession(ctx, podKey); err != nil {
				m.logger.Warn("Failed to delete session from store", "pod_key", podKey, "error", err)
			}
		}
	}

	m.logger.Info("Relay gracefully unregistered",
		"relay_id", relayID,
		"reason", reason,
		"affected_sessions", len(affectedSessions))

	return affectedSessions
}

// GetRelayByID returns a relay by ID
func (m *Manager) GetRelayByID(relayID string) *RelayInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if relay, ok := m.relays[relayID]; ok {
		relayCopy := *relay
		return &relayCopy
	}
	return nil
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

// SelectRelay selects the best relay for a given runner region using weighted scoring
// Factors considered: connection load, CPU, memory, latency, and region affinity
func (m *Manager) SelectRelay(runnerRegion string) *RelayInfo {
	return m.SelectRelayWithOptions(SelectRelayOptions{Region: runnerRegion})
}

// SelectRelayOptions holds options for relay selection
type SelectRelayOptions struct {
	Region          string   // Preferred region
	ExcludeRelayIDs []string // Relays to exclude from selection
}

// SelectRelayWithOptions selects the best relay with additional options
func (m *Manager) SelectRelayWithOptions(opts SelectRelayOptions) *RelayInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Build exclusion set for O(1) lookup
	excluded := make(map[string]bool)
	for _, id := range opts.ExcludeRelayIDs {
		excluded[id] = true
	}

	var best *RelayInfo
	bestScore := float64(-1)
	cfg := m.loadBalancingConfig

	for _, relay := range m.relays {
		// Skip unhealthy, excluded, or at-capacity relays
		if !relay.Healthy || excluded[relay.ID] {
			continue
		}
		if relay.Capacity > 0 && relay.CurrentConnections >= relay.Capacity {
			continue
		}

		// Calculate weighted score (higher is better)
		score := 100.0

		// Connection load factor: prefer relays with more available capacity
		if relay.Capacity > 0 {
			availableRatio := 1 - float64(relay.CurrentConnections)/float64(relay.Capacity)
			score += availableRatio * cfg.ConnectionWeight * 100
		}

		// CPU factor: prefer relays with lower CPU usage
		if relay.CPUUsage >= 0 {
			cpuAvailable := (100 - relay.CPUUsage) / 100
			score += cpuAvailable * cfg.CPUWeight * 100
		}

		// Memory factor: prefer relays with lower memory usage
		if relay.MemoryUsage >= 0 {
			memAvailable := (100 - relay.MemoryUsage) / 100
			score += memAvailable * cfg.MemoryWeight * 100
		}

		// Latency factor: prefer relays with lower latency
		// Normalize latency: 0ms = 1.0, 500ms = 0.0 (capped)
		if relay.AvgLatencyMs >= 0 {
			latencyFactor := 1 - float64(min(relay.AvgLatencyMs, 500))/500
			score += latencyFactor * cfg.LatencyWeight * 100
		}

		// Region affinity bonus
		if opts.Region != "" && relay.Region == opts.Region {
			score += cfg.RegionBonus
		}

		if score > bestScore {
			bestScore = score
			best = relay
		}
	}

	if best != nil {
		m.logger.Debug("Selected relay",
			"relay_id", best.ID,
			"region", best.Region,
			"score", bestScore,
			"connections", best.CurrentConnections,
			"capacity", best.Capacity,
			"cpu", best.CPUUsage,
			"memory", best.MemoryUsage,
			"latency_ms", best.AvgLatencyMs)

		// Return copy to prevent data races
		bestCopy := *best
		return &bestCopy
	}

	return nil
}

// SelectRelayForPod selects relay for a pod, checking for existing active session
// Returns copies to prevent data races
func (m *Manager) SelectRelayForPod(podKey, runnerRegion string) (*RelayInfo, *ActiveSession) {
	m.mu.RLock()
	// Check for existing active session
	if session, ok := m.activeSessions[podKey]; ok {
		if time.Now().Before(session.ExpireAt) {
			if relay, exists := m.relays[session.RelayID]; exists && relay.Healthy {
				// Return copies to prevent data races
				relayCopy := *relay
				sessionCopy := *session
				m.mu.RUnlock()
				return &relayCopy, &sessionCopy
			}
		}
	}
	m.mu.RUnlock()

	// No existing session, select new relay
	relay := m.SelectRelay(runnerRegion)
	return relay, nil
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

// GetRelays returns all registered relays
func (m *Manager) GetRelays() []*RelayInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	relays := make([]*RelayInfo, 0, len(m.relays))
	for _, relay := range m.relays {
		// Copy to avoid data race
		relayCopy := *relay
		relays = append(relays, &relayCopy)
	}
	return relays
}

// GetHealthyRelayCount returns count of healthy relays
func (m *Manager) GetHealthyRelayCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, relay := range m.relays {
		if relay.Healthy {
			count++
		}
	}
	return count
}

// HasHealthyRelays checks if there are any healthy relays
func (m *Manager) HasHealthyRelays() bool {
	return m.GetHealthyRelayCount() > 0
}

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

// Stats returns relay statistics
type Stats struct {
	TotalRelays      int `json:"total_relays"`
	HealthyRelays    int `json:"healthy_relays"`
	TotalConnections int `json:"total_connections"`
	ActiveSessions   int `json:"active_sessions"`
}

// GetStats returns current statistics
func (m *Manager) GetStats() Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := Stats{
		TotalRelays:    len(m.relays),
		ActiveSessions: len(m.activeSessions),
	}

	for _, relay := range m.relays {
		if relay.Healthy {
			stats.HealthyRelays++
		}
		stats.TotalConnections += relay.CurrentConnections
	}

	return stats
}
