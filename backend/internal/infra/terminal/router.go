package terminal

import (
	"sync"

	"github.com/gorilla/websocket"
)

const (
	// DefaultScrollbackSize is the default scrollback buffer size (100KB)
	DefaultScrollbackSize = 100 * 1024
)

// Router routes terminal data between frontend clients and runners
// Architecture:
// - Frontend connects via WebSocket to /ws/terminal/{session_key}
// - Backend maintains a mapping of session_key -> frontend WebSocket
// - Runner sends terminal_output via control connection
// - Backend routes output to the appropriate frontend WebSocket
// - Frontend sends input, Backend routes to runner via control connection
type Router struct {
	mu sync.RWMutex

	// Maps session_key -> list of frontend WebSockets
	terminalClients map[string][]*websocket.Conn

	// Maps session_key -> runner_id for routing
	sessionRunnerMap map[string]int64

	// Scrollback buffers for reconnection support
	scrollbackBuffers map[string]*ScrollbackBuffer

	// Virtual terminals for agent observation
	virtualTerminals map[string]*VirtualTerminal

	// Connection manager for sending data to runners
	runnerConnections RunnerConnectionManager

	scrollbackSize int
}

// RunnerConnectionManager interface for sending data to runners
type RunnerConnectionManager interface {
	SendTerminalInput(runnerID int64, sessionKey string, data []byte) error
	SendTerminalResize(runnerID int64, sessionKey string, cols, rows int) error
}

// NewRouter creates a new terminal router
func NewRouter(connManager RunnerConnectionManager) *Router {
	return &Router{
		terminalClients:   make(map[string][]*websocket.Conn),
		sessionRunnerMap:  make(map[string]int64),
		scrollbackBuffers: make(map[string]*ScrollbackBuffer),
		virtualTerminals:  make(map[string]*VirtualTerminal),
		runnerConnections: connManager,
		scrollbackSize:    DefaultScrollbackSize,
	}
}

// RegisterSession registers a session's runner mapping
func (r *Router) RegisterSession(sessionKey string, runnerID int64, cols, rows int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.sessionRunnerMap[sessionKey] = runnerID

	// Initialize scrollback buffer
	if _, exists := r.scrollbackBuffers[sessionKey]; !exists {
		r.scrollbackBuffers[sessionKey] = NewScrollbackBuffer(r.scrollbackSize)
	}

	// Initialize or resize virtual terminal
	if vt, exists := r.virtualTerminals[sessionKey]; !exists {
		r.virtualTerminals[sessionKey] = NewVirtualTerminal(cols, rows, 10000)
	} else {
		vt.Resize(cols, rows)
	}
}

// UnregisterSession unregisters a session
func (r *Router) UnregisterSession(sessionKey string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.sessionRunnerMap, sessionKey)
	delete(r.scrollbackBuffers, sessionKey)
	delete(r.virtualTerminals, sessionKey)

	// Close any connected clients
	if clients, exists := r.terminalClients[sessionKey]; exists {
		for _, ws := range clients {
			ws.Close()
		}
		delete(r.terminalClients, sessionKey)
	}
}

// ResizeVirtualTerminal resizes the virtual terminal for a session
func (r *Router) ResizeVirtualTerminal(sessionKey string, cols, rows int) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if vt, exists := r.virtualTerminals[sessionKey]; exists {
		vt.Resize(cols, rows)
	}
}

// ConnectClient registers a frontend client for a session
func (r *Router) ConnectClient(sessionKey string, ws *websocket.Conn) error {
	r.mu.Lock()
	if _, exists := r.terminalClients[sessionKey]; !exists {
		r.terminalClients[sessionKey] = make([]*websocket.Conn, 0)
	}
	r.terminalClients[sessionKey] = append(r.terminalClients[sessionKey], ws)

	// Get scrollback data before releasing lock
	var scrollback []byte
	if buffer, exists := r.scrollbackBuffers[sessionKey]; exists {
		scrollback = buffer.GetAll()
	}
	r.mu.Unlock()

	// Send scrollback buffer to newly connected client
	if len(scrollback) > 0 {
		if err := ws.WriteMessage(websocket.BinaryMessage, scrollback); err != nil {
			return err
		}
	}

	return nil
}

// DisconnectClient unregisters a frontend client
func (r *Router) DisconnectClient(sessionKey string, ws *websocket.Conn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if clients, exists := r.terminalClients[sessionKey]; exists {
		for i, client := range clients {
			if client == ws {
				r.terminalClients[sessionKey] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
		if len(r.terminalClients[sessionKey]) == 0 {
			delete(r.terminalClients, sessionKey)
		}
	}
}

// RouteOutput routes terminal output from runner to frontend clients
func (r *Router) RouteOutput(sessionKey string, data []byte) {
	r.mu.Lock()
	// Store in scrollback buffer
	if buffer, exists := r.scrollbackBuffers[sessionKey]; exists {
		buffer.Write(data)
	}

	// Feed to virtual terminal
	if vt, exists := r.virtualTerminals[sessionKey]; exists {
		vt.Feed(data)
	}

	// Get clients list
	clients := make([]*websocket.Conn, len(r.terminalClients[sessionKey]))
	copy(clients, r.terminalClients[sessionKey])
	r.mu.Unlock()

	// Send to all connected clients (broadcast)
	var deadClients []*websocket.Conn
	for _, ws := range clients {
		if err := ws.WriteMessage(websocket.BinaryMessage, data); err != nil {
			deadClients = append(deadClients, ws)
		}
	}

	// Clean up dead clients
	if len(deadClients) > 0 {
		r.mu.Lock()
		for _, dead := range deadClients {
			r.removeClientLocked(sessionKey, dead)
		}
		r.mu.Unlock()
	}
}

// RouteInput routes terminal input from frontend to runner
func (r *Router) RouteInput(sessionKey string, data []byte) error {
	r.mu.RLock()
	runnerID, exists := r.sessionRunnerMap[sessionKey]
	r.mu.RUnlock()

	if !exists {
		return nil
	}

	if r.runnerConnections != nil {
		return r.runnerConnections.SendTerminalInput(runnerID, sessionKey, data)
	}
	return nil
}

// RouteResize routes terminal resize from frontend to runner
func (r *Router) RouteResize(sessionKey string, cols, rows int) error {
	r.mu.Lock()
	runnerID, exists := r.sessionRunnerMap[sessionKey]
	if exists {
		// Also resize virtual terminal
		if vt, vtExists := r.virtualTerminals[sessionKey]; vtExists {
			vt.Resize(cols, rows)
		}
	}
	r.mu.Unlock()

	if !exists {
		return nil
	}

	if r.runnerConnections != nil {
		return r.runnerConnections.SendTerminalResize(runnerID, sessionKey, cols, rows)
	}
	return nil
}

// GetRecentOutput gets recent terminal output for agent observation
func (r *Router) GetRecentOutput(sessionKey string, lines int, raw bool) []byte {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if raw {
		// Return raw scrollback data
		if buffer, exists := r.scrollbackBuffers[sessionKey]; exists {
			return buffer.GetLastNLines(lines)
		}
		return nil
	}

	// Return processed output from virtual terminal
	if vt, exists := r.virtualTerminals[sessionKey]; exists {
		return []byte(vt.GetOutput(lines))
	}
	return nil
}

// GetScreenSnapshot gets the current screen snapshot for a session
func (r *Router) GetScreenSnapshot(sessionKey string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if vt, exists := r.virtualTerminals[sessionKey]; exists {
		return vt.GetDisplay()
	}
	return ""
}

// GetClientCount returns the number of clients connected to a session
func (r *Router) GetClientCount(sessionKey string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.terminalClients[sessionKey])
}

// IsSessionRegistered checks if a session is registered
func (r *Router) IsSessionRegistered(sessionKey string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.sessionRunnerMap[sessionKey]
	return exists
}

// GetRunnerID returns the runner ID for a session
func (r *Router) GetRunnerID(sessionKey string) (int64, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	runnerID, exists := r.sessionRunnerMap[sessionKey]
	return runnerID, exists
}

// removeClientLocked removes a client without locking (caller must hold lock)
func (r *Router) removeClientLocked(sessionKey string, ws *websocket.Conn) {
	if clients, exists := r.terminalClients[sessionKey]; exists {
		for i, client := range clients {
			if client == ws {
				r.terminalClients[sessionKey] = append(clients[:i], clients[i+1:]...)
				break
			}
		}
		if len(r.terminalClients[sessionKey]) == 0 {
			delete(r.terminalClients, sessionKey)
		}
	}
}
