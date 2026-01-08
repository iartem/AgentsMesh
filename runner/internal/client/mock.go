package client

import (
	"encoding/json"
	"sync"
)

// MockConnection is a mock implementation of Connection for testing.
type MockConnection struct {
	mu sync.Mutex

	// Handler
	handler MessageHandler

	// Captured calls for verification
	SentMessages []ProtocolMessage
	Events       []EventCall

	// Configurable responses
	ConnectErr error
	SendErr    error

	// State
	started bool
	stopped bool
}

// EventCall represents a captured SendEvent call
type EventCall struct {
	Type MessageType
	Data interface{}
}

// NewMockConnection creates a new mock connection for testing.
func NewMockConnection() *MockConnection {
	return &MockConnection{}
}

// SetHandler implements Connection.
func (m *MockConnection) SetHandler(handler MessageHandler) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handler = handler
}

// Connect implements Connection.
func (m *MockConnection) Connect() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.ConnectErr != nil {
		return m.ConnectErr
	}
	return nil
}

// Start implements Connection.
func (m *MockConnection) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.started = true
}

// Stop implements Connection.
func (m *MockConnection) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.stopped = true
}

// Send implements Connection.
func (m *MockConnection) Send(msg ProtocolMessage) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SendErr == nil {
		m.SentMessages = append(m.SentMessages, msg)
	}
}

// SendWithBackpressure implements Connection.
func (m *MockConnection) SendWithBackpressure(msg ProtocolMessage) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.stopped {
		return false
	}
	m.SentMessages = append(m.SentMessages, msg)
	return true
}

// SendEvent implements Connection.
func (m *MockConnection) SendEvent(msgType MessageType, data interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.SendErr != nil {
		return m.SendErr
	}
	m.Events = append(m.Events, EventCall{Type: msgType, Data: data})

	// Also add to SentMessages for compatibility
	dataBytes, _ := json.Marshal(data)
	m.SentMessages = append(m.SentMessages, ProtocolMessage{
		Type: msgType,
		Data: dataBytes,
	})
	return nil
}

// QueueLength implements Connection.
func (m *MockConnection) QueueLength() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.SentMessages)
}

// QueueCapacity implements Connection.
func (m *MockConnection) QueueCapacity() int {
	return 100
}

// SetAuthToken implements Connection.
func (m *MockConnection) SetAuthToken(token string) {
	// No-op for mock
}

// --- Test helper methods ---

// SimulateCreateSession simulates server sending a create_session message.
func (m *MockConnection) SimulateCreateSession(req CreateSessionRequest) error {
	m.mu.Lock()
	handler := m.handler
	m.mu.Unlock()
	if handler != nil {
		return handler.OnCreateSession(req)
	}
	return nil
}

// SimulateTerminateSession simulates server sending a terminate_session message.
func (m *MockConnection) SimulateTerminateSession(req TerminateSessionRequest) error {
	m.mu.Lock()
	handler := m.handler
	m.mu.Unlock()
	if handler != nil {
		return handler.OnTerminateSession(req)
	}
	return nil
}

// SimulateTerminalInput simulates server sending a terminal_input message.
func (m *MockConnection) SimulateTerminalInput(req TerminalInputRequest) error {
	m.mu.Lock()
	handler := m.handler
	m.mu.Unlock()
	if handler != nil {
		return handler.OnTerminalInput(req)
	}
	return nil
}

// SimulateTerminalResize simulates server sending a terminal_resize message.
func (m *MockConnection) SimulateTerminalResize(req TerminalResizeRequest) error {
	m.mu.Lock()
	handler := m.handler
	m.mu.Unlock()
	if handler != nil {
		return handler.OnTerminalResize(req)
	}
	return nil
}

// GetSessions returns sessions from handler (if available).
func (m *MockConnection) GetSessions() []SessionInfo {
	m.mu.Lock()
	handler := m.handler
	m.mu.Unlock()
	if handler != nil {
		return handler.OnListSessions()
	}
	return nil
}

// GetEvents returns captured events (thread-safe).
func (m *MockConnection) GetEvents() []EventCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]EventCall, len(m.Events))
	copy(result, m.Events)
	return result
}

// GetSentMessages returns captured messages (thread-safe).
func (m *MockConnection) GetSentMessages() []ProtocolMessage {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]ProtocolMessage, len(m.SentMessages))
	copy(result, m.SentMessages)
	return result
}

// IsStarted returns whether Start was called.
func (m *MockConnection) IsStarted() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.started
}

// IsStopped returns whether Stop was called.
func (m *MockConnection) IsStopped() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopped
}

// Reset clears all captured calls.
func (m *MockConnection) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.SentMessages = nil
	m.Events = nil
	m.started = false
	m.stopped = false
}

// Ensure MockConnection implements Connection interface
var _ Connection = (*MockConnection)(nil)
