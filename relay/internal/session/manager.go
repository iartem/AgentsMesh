package session

import (
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ManagerConfig holds configuration for the session manager
type ManagerConfig struct {
	KeepAliveDuration        time.Duration // How long to keep session alive after all browsers disconnect
	MaxBrowsersPerPod        int           // Maximum browsers per pod
	RunnerReconnectTimeout   time.Duration // How long to wait for runner to reconnect
	BrowserReconnectTimeout  time.Duration // How long to wait for browser to reconnect
	PendingConnectionTimeout time.Duration // How long to wait for counterpart connection
	OutputBufferSize         int           // Max bytes for output buffer
	OutputBufferCount        int           // Max messages for output buffer
}

// DefaultManagerConfig returns default manager configuration
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		KeepAliveDuration:        30 * time.Second,
		MaxBrowsersPerPod:        10,
		RunnerReconnectTimeout:   30 * time.Second,
		BrowserReconnectTimeout:  30 * time.Second,
		PendingConnectionTimeout: 60 * time.Second,
		OutputBufferSize:         256 * 1024, // 256KB
		OutputBufferCount:        200,
	}
}

// Manager manages terminal sessions
type Manager struct {
	mu       sync.RWMutex
	sessions map[string]*TerminalSession // sessionID -> session

	// Pending connections waiting for counterpart
	pendingRunners  map[string]*pendingRunner  // sessionID -> pending runner
	pendingBrowsers map[string]*pendingBrowser // sessionID -> pending browser

	// Configuration
	config ManagerConfig

	// Callbacks
	onAllBrowsersGone func(podKey string)

	logger *slog.Logger
}

type pendingRunner struct {
	conn      *websocket.Conn
	podKey    string
	sessionID string
	createdAt time.Time
}

type pendingBrowser struct {
	conn      *websocket.Conn
	browserID string
	podKey    string
	sessionID string
	createdAt time.Time
}

// NewManager creates a new session manager with default configuration
func NewManager(keepAliveDuration time.Duration, maxBrowsersPerPod int, onAllBrowsersGone func(string)) *Manager {
	cfg := DefaultManagerConfig()
	cfg.KeepAliveDuration = keepAliveDuration
	cfg.MaxBrowsersPerPod = maxBrowsersPerPod
	return NewManagerWithConfig(cfg, onAllBrowsersGone)
}

// NewManagerWithConfig creates a new session manager with custom configuration
func NewManagerWithConfig(cfg ManagerConfig, onAllBrowsersGone func(string)) *Manager {
	m := &Manager{
		sessions:          make(map[string]*TerminalSession),
		pendingRunners:    make(map[string]*pendingRunner),
		pendingBrowsers:   make(map[string]*pendingBrowser),
		config:            cfg,
		onAllBrowsersGone: onAllBrowsersGone,
		logger:            slog.With("component", "session_manager"),
	}

	// Start cleanup goroutine for pending connections
	go m.cleanupPendingConnections()

	return m
}

// HandleRunnerConnect handles a runner WebSocket connection
func (m *Manager) HandleRunnerConnect(sessionID, podKey string, conn *websocket.Conn) error {
	m.mu.Lock()

	// Check if session already exists
	if session, ok := m.sessions[sessionID]; ok {
		m.mu.Unlock()
		// Session exists, just update runner connection
		session.SetRunnerConn(conn)
		return nil
	}

	// Check if there's a pending browser waiting
	if pending, ok := m.pendingBrowsers[sessionID]; ok {
		delete(m.pendingBrowsers, sessionID)
		m.mu.Unlock()

		// Create new session and connect both
		session := NewTerminalSessionWithConfig(sessionID, podKey, m.buildSessionConfig(), m.onAllBrowsersGone, m.onSessionClosed)
		session.SetRunnerConn(conn)
		session.AddBrowser(pending.browserID, pending.conn)

		m.mu.Lock()
		m.sessions[sessionID] = session
		m.mu.Unlock()

		m.logger.Info("Session created (runner connected to waiting browser)",
			"session_id", sessionID, "pod_key", podKey)
		return nil
	}

	// No browser waiting, add to pending runners
	m.pendingRunners[sessionID] = &pendingRunner{
		conn:      conn,
		podKey:    podKey,
		sessionID: sessionID,
		createdAt: time.Now(),
	}
	m.mu.Unlock()

	m.logger.Info("Runner waiting for browser", "session_id", sessionID, "pod_key", podKey)
	return nil
}

// HandleBrowserConnect handles a browser WebSocket connection
func (m *Manager) HandleBrowserConnect(sessionID, podKey, browserID string, conn *websocket.Conn) error {
	m.mu.Lock()

	// Check if session already exists
	if session, ok := m.sessions[sessionID]; ok {
		// Check browser limit
		if session.BrowserCount() >= m.config.MaxBrowsersPerPod {
			m.mu.Unlock()
			return &MaxBrowsersError{Max: m.config.MaxBrowsersPerPod}
		}
		m.mu.Unlock()

		// Add browser to existing session
		session.AddBrowser(browserID, conn)
		return nil
	}

	// Check if there's a pending runner waiting
	if pending, ok := m.pendingRunners[sessionID]; ok {
		delete(m.pendingRunners, sessionID)
		m.mu.Unlock()

		// Create new session and connect both
		session := NewTerminalSessionWithConfig(sessionID, podKey, m.buildSessionConfig(), m.onAllBrowsersGone, m.onSessionClosed)
		session.SetRunnerConn(pending.conn)
		session.AddBrowser(browserID, conn)

		m.mu.Lock()
		m.sessions[sessionID] = session
		m.mu.Unlock()

		m.logger.Info("Session created (browser connected to waiting runner)",
			"session_id", sessionID, "pod_key", podKey)
		return nil
	}

	// No runner waiting, add to pending browsers
	m.pendingBrowsers[sessionID] = &pendingBrowser{
		conn:      conn,
		browserID: browserID,
		podKey:    podKey,
		sessionID: sessionID,
		createdAt: time.Now(),
	}
	m.mu.Unlock()

	m.logger.Info("Browser waiting for runner", "session_id", sessionID, "pod_key", podKey)
	return nil
}

// GetSession returns a session by ID
func (m *Manager) GetSession(sessionID string) *TerminalSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sessions[sessionID]
}

// GetSessionByPodKey returns a session by pod key
func (m *Manager) GetSessionByPodKey(podKey string) *TerminalSession {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, session := range m.sessions {
		if session.PodKey == podKey {
			return session
		}
	}
	return nil
}

// CloseSession closes and removes a session
func (m *Manager) CloseSession(sessionID string) {
	m.mu.Lock()
	session, ok := m.sessions[sessionID]
	if ok {
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()

	if session != nil {
		session.Close()
	}
}

// CloseSessionByPodKey closes a session by pod key
func (m *Manager) CloseSessionByPodKey(podKey string) {
	m.mu.Lock()
	var sessionToClose *TerminalSession
	var sessionID string

	for id, session := range m.sessions {
		if session.PodKey == podKey {
			sessionToClose = session
			sessionID = id
			break
		}
	}

	if sessionID != "" {
		delete(m.sessions, sessionID)
	}
	m.mu.Unlock()

	if sessionToClose != nil {
		sessionToClose.Close()
	}
}

// onSessionClosed is called when a session closes
func (m *Manager) onSessionClosed(sessionID string) {
	m.mu.Lock()
	delete(m.sessions, sessionID)
	m.mu.Unlock()
	m.logger.Info("Session removed", "session_id", sessionID)
}

// cleanupPendingConnections periodically cleans up stale pending connections
func (m *Manager) cleanupPendingConnections() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()

		now := time.Now()
		timeout := m.config.PendingConnectionTimeout

		// Clean up stale pending runners
		for sessionID, pending := range m.pendingRunners {
			if now.Sub(pending.createdAt) > timeout {
				pending.conn.Close()
				delete(m.pendingRunners, sessionID)
				m.logger.Info("Cleaned up stale pending runner", "session_id", sessionID)
			}
		}

		// Clean up stale pending browsers
		for sessionID, pending := range m.pendingBrowsers {
			if now.Sub(pending.createdAt) > timeout {
				pending.conn.Close()
				delete(m.pendingBrowsers, sessionID)
				m.logger.Info("Cleaned up stale pending browser", "session_id", sessionID)
			}
		}

		m.mu.Unlock()
	}
}

// buildSessionConfig creates a SessionConfig from ManagerConfig
func (m *Manager) buildSessionConfig() SessionConfig {
	return SessionConfig{
		KeepAliveDuration:       m.config.KeepAliveDuration,
		RunnerReconnectTimeout:  m.config.RunnerReconnectTimeout,
		BrowserReconnectTimeout: m.config.BrowserReconnectTimeout,
		OutputBufferSize:        m.config.OutputBufferSize,
		OutputBufferCount:       m.config.OutputBufferCount,
	}
}

// Stats returns session statistics
func (m *Manager) Stats() SessionStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalBrowsers := 0
	for _, session := range m.sessions {
		totalBrowsers += session.BrowserCount()
	}

	return SessionStats{
		ActiveSessions:  len(m.sessions),
		TotalBrowsers:   totalBrowsers,
		PendingRunners:  len(m.pendingRunners),
		PendingBrowsers: len(m.pendingBrowsers),
	}
}

// SessionStats holds session statistics
type SessionStats struct {
	ActiveSessions  int `json:"active_sessions"`
	TotalBrowsers   int `json:"total_browsers"`
	PendingRunners  int `json:"pending_runners"`
	PendingBrowsers int `json:"pending_browsers"`
}

// MaxBrowsersError indicates max browsers limit reached
type MaxBrowsersError struct {
	Max int
}

func (e *MaxBrowsersError) Error() string {
	return "maximum browsers per pod reached"
}
