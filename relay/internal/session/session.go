package session

import (
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/anthropics/agentsmesh/relay/internal/protocol"
)

// BrowserConn represents a browser WebSocket connection
type BrowserConn struct {
	ID   string
	Conn *websocket.Conn
}

// SessionConfig holds configuration for a terminal session
type SessionConfig struct {
	KeepAliveDuration        time.Duration // How long to keep session alive after all browsers disconnect
	RunnerReconnectTimeout   time.Duration // How long to wait for runner to reconnect
	BrowserReconnectTimeout  time.Duration // How long to wait for browser to reconnect (future use)
	OutputBufferSize         int           // Max bytes for output buffer
	OutputBufferCount        int           // Max messages for output buffer
}

// DefaultSessionConfig returns default session configuration
func DefaultSessionConfig() SessionConfig {
	return SessionConfig{
		KeepAliveDuration:       30 * time.Second,
		RunnerReconnectTimeout:  30 * time.Second,
		BrowserReconnectTimeout: 30 * time.Second,
		OutputBufferSize:        256 * 1024, // 256KB
		OutputBufferCount:       200,
	}
}

// TerminalSession manages a terminal session between Runner and multiple Browsers
type TerminalSession struct {
	ID     string
	PodKey string

	// Configuration
	config SessionConfig

	// Runner connection (1)
	runnerConn *websocket.Conn
	runnerMu   sync.RWMutex

	// Browser connections (N)
	browsers   map[string]*BrowserConn // browserID -> conn
	browsersMu sync.RWMutex

	// Disconnect handling
	lastBrowserDisconnect time.Time
	keepAliveTimer        *time.Timer

	// Input control
	controllerID string // Current controller browser ID
	controllerMu sync.RWMutex

	// Output buffer for new observers (ring buffer of recent Output messages)
	// This allows new browsers to receive recent terminal output missed before connecting
	outputBuffer      [][]byte
	outputBufferBytes int // Total bytes in buffer (for size limiting)
	outputBufferMu    sync.RWMutex

	// Runner reconnection support
	runnerDisconnected   bool        // Runner currently disconnected
	runnerReconnectTimer *time.Timer // Timer for runner reconnect timeout

	// Callbacks
	onAllBrowsersGone func(podKey string)
	onSessionClosed   func(sessionID string)

	// Session state
	closed   bool
	closedMu sync.RWMutex

	logger *slog.Logger
}

// NewTerminalSession creates a new terminal session with default configuration
func NewTerminalSession(id, podKey string, keepAliveDuration time.Duration, onAllBrowsersGone func(string), onSessionClosed func(string)) *TerminalSession {
	cfg := DefaultSessionConfig()
	cfg.KeepAliveDuration = keepAliveDuration
	return NewTerminalSessionWithConfig(id, podKey, cfg, onAllBrowsersGone, onSessionClosed)
}

// NewTerminalSessionWithConfig creates a new terminal session with custom configuration
func NewTerminalSessionWithConfig(id, podKey string, cfg SessionConfig, onAllBrowsersGone func(string), onSessionClosed func(string)) *TerminalSession {
	return &TerminalSession{
		ID:                id,
		PodKey:            podKey,
		config:            cfg,
		browsers:          make(map[string]*BrowserConn),
		onAllBrowsersGone: onAllBrowsersGone,
		onSessionClosed:   onSessionClosed,
		outputBuffer:      make([][]byte, 0, cfg.OutputBufferCount),
		logger:            slog.With("session_id", id, "pod_key", podKey),
	}
}

// SetRunnerConn sets the runner connection
func (s *TerminalSession) SetRunnerConn(conn *websocket.Conn) {
	s.runnerMu.Lock()
	wasDisconnected := s.runnerDisconnected
	s.runnerConn = conn
	s.runnerDisconnected = false

	// Cancel reconnect timer if exists
	if s.runnerReconnectTimer != nil {
		s.runnerReconnectTimer.Stop()
		s.runnerReconnectTimer = nil
	}
	s.runnerMu.Unlock()

	if wasDisconnected {
		s.logger.Info("Runner reconnected")
		// Notify all browsers that runner has reconnected
		s.BroadcastToAllBrowsers(protocol.EncodeRunnerReconnected())
	} else {
		s.logger.Info("Runner connected")
	}

	// Start forwarding from runner to browsers
	go s.forwardRunnerToBrowsers()
}

// GetRunnerConn returns the runner connection (for checking if connected)
func (s *TerminalSession) GetRunnerConn() *websocket.Conn {
	s.runnerMu.RLock()
	defer s.runnerMu.RUnlock()
	return s.runnerConn
}

// IsRunnerDisconnected returns true if the runner is currently disconnected
func (s *TerminalSession) IsRunnerDisconnected() bool {
	s.runnerMu.RLock()
	defer s.runnerMu.RUnlock()
	return s.runnerDisconnected
}

// AddBrowser adds a browser observer
func (s *TerminalSession) AddBrowser(browserID string, conn *websocket.Conn) {
	s.browsersMu.Lock()
	s.browsers[browserID] = &BrowserConn{ID: browserID, Conn: conn}

	// Cancel keep-alive timer if exists
	if s.keepAliveTimer != nil {
		s.keepAliveTimer.Stop()
		s.keepAliveTimer = nil
	}
	s.browsersMu.Unlock()

	s.logger.Info("Browser connected", "browser_id", browserID, "total_browsers", len(s.browsers))

	// Send buffered Output messages to new browser
	// This allows new observers to see recent terminal output they missed
	bufferedOutput := s.getBufferedOutput()
	if len(bufferedOutput) > 0 {
		s.logger.Debug("Sending buffered output to new browser",
			"browser_id", browserID, "count", len(bufferedOutput))
		for _, data := range bufferedOutput {
			if err := conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				s.logger.Warn("Failed to send buffered output to new browser",
					"browser_id", browserID, "error", err)
				break // Stop sending if connection has issues
			}
		}
	}

	// Notify new browser if runner is currently disconnected
	if s.IsRunnerDisconnected() {
		if err := conn.WriteMessage(websocket.BinaryMessage, protocol.EncodeRunnerDisconnected()); err != nil {
			s.logger.Warn("Failed to send runner disconnected status to new browser",
				"browser_id", browserID, "error", err)
		}
	}

	// Start forwarding from this browser to runner
	go s.forwardBrowserToRunner(browserID)
}

// RemoveBrowser removes a browser observer
func (s *TerminalSession) RemoveBrowser(browserID string) {
	s.browsersMu.Lock()
	if browser, ok := s.browsers[browserID]; ok {
		browser.Conn.Close()
		delete(s.browsers, browserID)
	}
	count := len(s.browsers)
	s.browsersMu.Unlock()

	s.logger.Info("Browser disconnected", "browser_id", browserID, "remaining_browsers", count)

	// Release control if this browser had it
	s.controllerMu.Lock()
	if s.controllerID == browserID {
		s.controllerID = ""
	}
	s.controllerMu.Unlock()

	if count == 0 {
		// Last browser left, start keep-alive timer
		s.lastBrowserDisconnect = time.Now()
		s.browsersMu.Lock()
		s.keepAliveTimer = time.AfterFunc(s.config.KeepAliveDuration, func() {
			// Check if still no browsers after timeout
			s.browsersMu.RLock()
			stillEmpty := len(s.browsers) == 0
			s.browsersMu.RUnlock()

			if stillEmpty {
				s.logger.Info("Keep-alive timeout, notifying backend")
				if s.onAllBrowsersGone != nil {
					s.onAllBrowsersGone(s.PodKey)
				}
			}
		})
		s.browsersMu.Unlock()
	}
}

// BrowserCount returns the number of connected browsers
func (s *TerminalSession) BrowserCount() int {
	s.browsersMu.RLock()
	defer s.browsersMu.RUnlock()
	return len(s.browsers)
}

// BroadcastToAllBrowsers sends data to all connected browsers
func (s *TerminalSession) BroadcastToAllBrowsers(data []byte) {
	s.browsersMu.RLock()
	defer s.browsersMu.RUnlock()

	browserCount := len(s.browsers)
	s.logger.Debug("Broadcasting to browsers", "data_len", len(data), "browser_count", browserCount)

	for _, browser := range s.browsers {
		if err := browser.Conn.WriteMessage(websocket.BinaryMessage, data); err != nil {
			s.logger.Warn("Failed to send to browser", "browser_id", browser.ID, "error", err)
		} else {
			s.logger.Debug("Sent to browser", "browser_id", browser.ID, "data_len", len(data))
		}
	}
}

// bufferOutput adds an Output message to the ring buffer for new observers
func (s *TerminalSession) bufferOutput(data []byte) {
	s.outputBufferMu.Lock()
	defer s.outputBufferMu.Unlock()

	dataLen := len(data)

	// Evict old messages if buffer is full (by count or size)
	for len(s.outputBuffer) >= s.config.OutputBufferCount || (s.outputBufferBytes+dataLen > s.config.OutputBufferSize && len(s.outputBuffer) > 0) {
		// Remove oldest message
		oldMsg := s.outputBuffer[0]
		s.outputBuffer = s.outputBuffer[1:]
		s.outputBufferBytes -= len(oldMsg)
	}

	// Only buffer if this single message fits
	if dataLen <= s.config.OutputBufferSize {
		// Make a copy to avoid data races
		dataCopy := make([]byte, dataLen)
		copy(dataCopy, data)
		s.outputBuffer = append(s.outputBuffer, dataCopy)
		s.outputBufferBytes += dataLen
	}
}

// getBufferedOutput returns a copy of all buffered Output messages
func (s *TerminalSession) getBufferedOutput() [][]byte {
	s.outputBufferMu.RLock()
	defer s.outputBufferMu.RUnlock()

	result := make([][]byte, len(s.outputBuffer))
	for i, data := range s.outputBuffer {
		dataCopy := make([]byte, len(data))
		copy(dataCopy, data)
		result[i] = dataCopy
	}
	return result
}

// forwardRunnerToBrowsers forwards data from runner to all browsers
func (s *TerminalSession) forwardRunnerToBrowsers() {
	s.logger.Debug("Starting forwardRunnerToBrowsers loop")
	for {
		s.runnerMu.RLock()
		conn := s.runnerConn
		s.runnerMu.RUnlock()

		if conn == nil {
			s.logger.Debug("Runner conn is nil, exiting forward loop")
			break
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			s.logger.Info("Runner disconnected", "error", err)
			s.handleRunnerDisconnect()
			break
		}

		s.logger.Debug("Received data from runner", "data_len", len(data))

		// Buffer Output messages for new observers
		msg, _ := protocol.DecodeMessage(data)
		if msg != nil && msg.Type == protocol.MsgTypeOutput {
			s.bufferOutput(data)
		}

		s.BroadcastToAllBrowsers(data)
	}
}

// forwardBrowserToRunner forwards input from a browser to runner
func (s *TerminalSession) forwardBrowserToRunner(browserID string) {
	s.browsersMu.RLock()
	browser, ok := s.browsers[browserID]
	s.browsersMu.RUnlock()

	if !ok {
		return
	}

	for {
		_, data, err := browser.Conn.ReadMessage()
		if err != nil {
			s.RemoveBrowser(browserID)
			break
		}

		// Parse message type
		msg, err := protocol.DecodeMessage(data)
		if err != nil {
			continue
		}

		// Handle control requests
		if msg.Type == protocol.MsgTypeControl {
			s.handleControlRequest(browserID, msg.Payload)
			continue
		}

		// For input messages, check control permission
		if msg.Type == protocol.MsgTypeInput {
			if !s.CanInput(browserID) {
				s.logger.Debug("Input rejected, no control", "browser_id", browserID)
				continue
			}
		}

		// Handle ping/pong locally
		if msg.Type == protocol.MsgTypePing {
			if err := browser.Conn.WriteMessage(websocket.BinaryMessage, protocol.EncodePong()); err != nil {
				s.logger.Warn("Failed to send pong", "browser_id", browserID)
			}
			continue
		}

		// Forward to runner
		s.runnerMu.RLock()
		if s.runnerConn != nil {
			if err := s.runnerConn.WriteMessage(websocket.BinaryMessage, data); err != nil {
				s.logger.Warn("Failed to forward to runner", "error", err)
			}
		}
		s.runnerMu.RUnlock()
	}
}

// handleRunnerDisconnect handles runner disconnect
// Instead of immediately closing the session, wait for runner to reconnect
func (s *TerminalSession) handleRunnerDisconnect() {
	s.runnerMu.Lock()
	if s.runnerConn != nil {
		s.runnerConn.Close()
		s.runnerConn = nil
	}
	s.runnerDisconnected = true

	// Notify all browsers that runner has disconnected
	s.runnerMu.Unlock()
	s.BroadcastToAllBrowsers(protocol.EncodeRunnerDisconnected())
	s.runnerMu.Lock()

	s.logger.Info("Runner disconnected, waiting for reconnection",
		"timeout", s.config.RunnerReconnectTimeout)

	// Start reconnect timer
	s.runnerReconnectTimer = time.AfterFunc(s.config.RunnerReconnectTimeout, func() {
		s.runnerMu.Lock()
		stillDisconnected := s.runnerDisconnected
		s.runnerMu.Unlock()

		if stillDisconnected {
			s.logger.Info("Runner reconnect timeout, closing session")
			s.Close()
		}
	})
	s.runnerMu.Unlock()
}

// handleControlRequest handles input control requests
func (s *TerminalSession) handleControlRequest(browserID string, payload []byte) {
	req, err := protocol.DecodeControlRequest(payload)
	if err != nil {
		return
	}

	var response *protocol.ControlRequest

	switch req.Action {
	case "request":
		if s.RequestControl(browserID) {
			response = &protocol.ControlRequest{Action: "granted", Controller: browserID}
		} else {
			s.controllerMu.RLock()
			response = &protocol.ControlRequest{Action: "denied", Controller: s.controllerID}
			s.controllerMu.RUnlock()
		}

	case "release":
		s.ReleaseControl(browserID)
		response = &protocol.ControlRequest{Action: "released", Controller: ""}

	case "query":
		s.controllerMu.RLock()
		response = &protocol.ControlRequest{Action: "status", Controller: s.controllerID}
		s.controllerMu.RUnlock()
	}

	if response != nil {
		data, _ := protocol.EncodeControlRequest(response)
		s.browsersMu.RLock()
		if browser, ok := s.browsers[browserID]; ok {
			browser.Conn.WriteMessage(websocket.BinaryMessage, data)
		}
		s.browsersMu.RUnlock()
	}
}

// CanInput checks if a browser can send input
func (s *TerminalSession) CanInput(browserID string) bool {
	s.controllerMu.RLock()
	defer s.controllerMu.RUnlock()

	// No controller or this browser is controller
	return s.controllerID == "" || s.controllerID == browserID
}

// RequestControl requests input control
func (s *TerminalSession) RequestControl(browserID string) bool {
	s.controllerMu.Lock()
	defer s.controllerMu.Unlock()

	if s.controllerID == "" {
		s.controllerID = browserID
		s.logger.Info("Control granted", "browser_id", browserID)
		return true
	}
	return false
}

// ReleaseControl releases input control
func (s *TerminalSession) ReleaseControl(browserID string) {
	s.controllerMu.Lock()
	defer s.controllerMu.Unlock()

	if s.controllerID == browserID {
		s.controllerID = ""
		s.logger.Info("Control released", "browser_id", browserID)
	}
}

// Close closes the session and all connections
func (s *TerminalSession) Close() {
	s.closedMu.Lock()
	if s.closed {
		s.closedMu.Unlock()
		return
	}
	s.closed = true
	s.closedMu.Unlock()

	s.logger.Info("Closing session")

	// Stop keep-alive timer
	s.browsersMu.Lock()
	if s.keepAliveTimer != nil {
		s.keepAliveTimer.Stop()
	}
	s.browsersMu.Unlock()

	// Stop runner reconnect timer and close runner connection
	s.runnerMu.Lock()
	if s.runnerReconnectTimer != nil {
		s.runnerReconnectTimer.Stop()
		s.runnerReconnectTimer = nil
	}
	if s.runnerConn != nil {
		s.runnerConn.Close()
		s.runnerConn = nil
	}
	s.runnerMu.Unlock()

	// Close all browser connections
	s.browsersMu.Lock()
	for _, browser := range s.browsers {
		browser.Conn.Close()
	}
	s.browsers = make(map[string]*BrowserConn)
	s.browsersMu.Unlock()

	// Notify session closed
	if s.onSessionClosed != nil {
		s.onSessionClosed(s.ID)
	}
}

// IsClosed checks if the session is closed
func (s *TerminalSession) IsClosed() bool {
	s.closedMu.RLock()
	defer s.closedMu.RUnlock()
	return s.closed
}
