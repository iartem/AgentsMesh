package terminal

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager manages multiple PTY sessions.
type Manager struct {
	sessions map[string]*Session
	mu       sync.RWMutex

	// Track which sessions have been notified as closed
	// This prevents double callback invocation
	closedSessions map[string]bool

	// Default session configuration
	defaultShell string
	workingDir   string

	// Callbacks
	onSessionCreate func(*Session)
	onSessionClose  func(*Session)
}

// NewManager creates a new session manager.
func NewManager(defaultShell, workingDir string) *Manager {
	return &Manager{
		sessions:       make(map[string]*Session),
		closedSessions: make(map[string]bool),
		defaultShell:   defaultShell,
		workingDir:     workingDir,
	}
}

// SetCallbacks sets event callbacks.
func (m *Manager) SetCallbacks(onCreate, onClose func(*Session)) {
	m.onSessionCreate = onCreate
	m.onSessionClose = onClose
}

// CreateSession creates a new session with the given configuration.
func (m *Manager) CreateSession(cfg *SessionConfig) (*Session, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if session already exists
	if _, exists := m.sessions[cfg.ID]; exists {
		return nil, fmt.Errorf("session %s already exists", cfg.ID)
	}

	// Apply defaults
	if cfg.Command == "" {
		cfg.Command = m.defaultShell
	}
	if cfg.WorkingDir == "" {
		cfg.WorkingDir = m.workingDir
	}
	if cfg.BufferSize == 0 {
		cfg.BufferSize = 64 * 1024
	}

	session, err := NewSession(cfg)
	if err != nil {
		return nil, err
	}

	m.sessions[cfg.ID] = session

	// Start monitoring for process exit
	go m.monitorSession(session)

	if m.onSessionCreate != nil {
		m.onSessionCreate(session)
	}

	return session, nil
}

// GetSession returns a session by ID.
func (m *Manager) GetSession(id string) (*Session, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	session, exists := m.sessions[id]
	return session, exists
}

// GetOrCreateSession gets an existing session or creates a new one.
func (m *Manager) GetOrCreateSession(cfg *SessionConfig) (*Session, error) {
	m.mu.RLock()
	session, exists := m.sessions[cfg.ID]
	m.mu.RUnlock()

	if exists {
		return session, nil
	}

	return m.CreateSession(cfg)
}

// ListSessions returns information about all sessions.
func (m *Manager) ListSessions() []SessionInfoData {
	m.mu.RLock()
	defer m.mu.RUnlock()

	infos := make([]SessionInfoData, 0, len(m.sessions))
	for _, session := range m.sessions {
		infos = append(infos, session.Info())
	}
	return infos
}

// CloseSession closes a session by ID.
func (m *Manager) CloseSession(id string) error {
	m.mu.Lock()
	session, exists := m.sessions[id]
	if exists {
		delete(m.sessions, id)
	}
	// Check if already notified, mark as closed
	alreadyClosed := m.closedSessions[id]
	if exists {
		m.closedSessions[id] = true
	}
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("session %s not found", id)
	}

	// Only call callback if not already called
	if !alreadyClosed && m.onSessionClose != nil {
		m.onSessionClose(session)
	}

	// Clean up the tracking entry after callback
	m.mu.Lock()
	delete(m.closedSessions, id)
	m.mu.Unlock()

	return session.Close()
}

// CloseAllSessions closes all sessions.
func (m *Manager) CloseAllSessions() {
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	sessionIDs := make([]string, 0, len(m.sessions))
	for id, session := range m.sessions {
		sessions = append(sessions, session)
		sessionIDs = append(sessionIDs, id)
		m.closedSessions[id] = true
	}
	m.sessions = make(map[string]*Session)
	m.mu.Unlock()

	for _, session := range sessions {
		if m.onSessionClose != nil {
			m.onSessionClose(session)
		}
		session.Close()
	}

	// Clean up tracking entries
	m.mu.Lock()
	for _, id := range sessionIDs {
		delete(m.closedSessions, id)
	}
	m.mu.Unlock()
}

// Count returns the number of active sessions.
func (m *Manager) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.sessions)
}

// monitorSession monitors a session for process exit.
func (m *Manager) monitorSession(session *Session) {
	// Wait for process to exit
	if session.Cmd != nil && session.Cmd.Process != nil {
		session.Cmd.Wait()
	}

	log.Printf("[terminal] Session process exited: session_id=%s", session.ID)

	// Check if we should call the callback (avoid double invocation)
	m.mu.Lock()
	alreadyClosed := m.closedSessions[session.ID]
	if !alreadyClosed {
		m.closedSessions[session.ID] = true
	}
	m.mu.Unlock()

	// Only call callback if not already called
	if !alreadyClosed && m.onSessionClose != nil {
		m.onSessionClose(session)
	}

	// Clean up tracking entry
	m.mu.Lock()
	delete(m.closedSessions, session.ID)
	m.mu.Unlock()
}

// CleanupStaleSessions removes sessions that have been inactive for too long.
func (m *Manager) CleanupStaleSessions(maxIdleTime time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	toRemove := make([]string, 0)

	for id, session := range m.sessions {
		// Only cleanup sessions with no clients and not running
		if session.ClientCount() == 0 && !session.IsRunning() {
			if now.Sub(session.LastActivity) > maxIdleTime {
				toRemove = append(toRemove, id)
			}
		}
	}

	for _, id := range toRemove {
		session := m.sessions[id]
		delete(m.sessions, id)
		// Mark as closed and clean up the tracking entry
		delete(m.closedSessions, id)
		session.Close()

		log.Printf("[terminal] Cleaned up stale session: session_id=%s, last_activity=%v",
			id, session.LastActivity)
	}

	return len(toRemove)
}
