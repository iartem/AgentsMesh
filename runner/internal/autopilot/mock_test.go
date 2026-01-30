package autopilot

import (
	"sync"
	"sync/atomic"

	runnerv1 "github.com/anthropics/agentsmesh/proto/gen/go/runner/v1"
)

// MockPodController is a mock implementation of PodController for testing
type MockPodController struct {
	sendTextCalls   []string
	workDir         string
	podKey          string
	agentStatus     string
	sendTextError   error         // If set, SendTerminalText will return this error
	stateDetector   StateDetector // Mock state detector
}

func (m *MockPodController) SendTerminalText(text string) error {
	m.sendTextCalls = append(m.sendTextCalls, text)
	return m.sendTextError
}

func (m *MockPodController) GetWorkDir() string {
	return m.workDir
}

func (m *MockPodController) GetPodKey() string {
	return m.podKey
}

func (m *MockPodController) GetAgentStatus() string {
	return m.agentStatus
}

func (m *MockPodController) GetStateDetector() StateDetector {
	return m.stateDetector
}

// MockEventReporter is a mock implementation of EventReporter for testing
type MockEventReporter struct {
	mu               sync.RWMutex
	statusEvents     []*runnerv1.AutopilotStatusEvent
	iterationEvents  []*runnerv1.AutopilotIterationEvent
	createdEvents    []*runnerv1.AutopilotCreatedEvent
	terminatedEvents []*runnerv1.AutopilotTerminatedEvent
	thinkingEvents   []*runnerv1.AutopilotThinkingEvent
}

func (m *MockEventReporter) ReportAutopilotStatus(event *runnerv1.AutopilotStatusEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusEvents = append(m.statusEvents, event)
}

func (m *MockEventReporter) ReportAutopilotIteration(event *runnerv1.AutopilotIterationEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.iterationEvents = append(m.iterationEvents, event)
}

func (m *MockEventReporter) ReportAutopilotCreated(event *runnerv1.AutopilotCreatedEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createdEvents = append(m.createdEvents, event)
}

func (m *MockEventReporter) ReportAutopilotTerminated(event *runnerv1.AutopilotTerminatedEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.terminatedEvents = append(m.terminatedEvents, event)
}

func (m *MockEventReporter) ReportAutopilotThinking(event *runnerv1.AutopilotThinkingEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.thinkingEvents = append(m.thinkingEvents, event)
}

// GetIterationEvents returns a copy of iteration events for safe access
func (m *MockEventReporter) GetIterationEvents() []*runnerv1.AutopilotIterationEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*runnerv1.AutopilotIterationEvent, len(m.iterationEvents))
	copy(result, m.iterationEvents)
	return result
}

// GetStatusEvents returns a copy of status events for safe access
func (m *MockEventReporter) GetStatusEvents() []*runnerv1.AutopilotStatusEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*runnerv1.AutopilotStatusEvent, len(m.statusEvents))
	copy(result, m.statusEvents)
	return result
}

// GetThinkingEvents returns a copy of thinking events for safe access
func (m *MockEventReporter) GetThinkingEvents() []*runnerv1.AutopilotThinkingEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*runnerv1.AutopilotThinkingEvent, len(m.thinkingEvents))
	copy(result, m.thinkingEvents)
	return result
}

// GetTerminatedEvents returns a copy of terminated events for safe access
func (m *MockEventReporter) GetTerminatedEvents() []*runnerv1.AutopilotTerminatedEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*runnerv1.AutopilotTerminatedEvent, len(m.terminatedEvents))
	copy(result, m.terminatedEvents)
	return result
}

// GetCreatedEvents returns a copy of created events for safe access
func (m *MockEventReporter) GetCreatedEvents() []*runnerv1.AutopilotCreatedEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*runnerv1.AutopilotCreatedEvent, len(m.createdEvents))
	copy(result, m.createdEvents)
	return result
}

// MockStateDetector is a mock implementation of StateDetector for testing
type MockStateDetector struct {
	state           AgentState
	stateMu         sync.RWMutex
	callback        StateChangeCallback
	callbackMu      sync.RWMutex
	detectCallCount atomic.Int32 // Track number of DetectState calls (atomic for race safety)
}

func NewMockStateDetector() *MockStateDetector {
	return &MockStateDetector{
		state: StateNotRunning,
	}
}

func (m *MockStateDetector) DetectState() AgentState {
	m.detectCallCount.Add(1)
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.state
}

func (m *MockStateDetector) GetDetectCallCount() int {
	return int(m.detectCallCount.Load())
}

func (m *MockStateDetector) GetState() AgentState {
	m.stateMu.RLock()
	defer m.stateMu.RUnlock()
	return m.state
}

func (m *MockStateDetector) SetCallback(cb StateChangeCallback) {
	m.callbackMu.Lock()
	defer m.callbackMu.Unlock()
	m.callback = cb
}

func (m *MockStateDetector) Reset() {
	m.stateMu.Lock()
	defer m.stateMu.Unlock()
	m.state = StateNotRunning
}

func (m *MockStateDetector) SetState(state AgentState) {
	m.stateMu.Lock()
	prevState := m.state
	m.state = state
	m.stateMu.Unlock()

	m.callbackMu.RLock()
	cb := m.callback
	m.callbackMu.RUnlock()

	if cb != nil && prevState != state {
		cb(state, prevState)
	}
}

func (m *MockStateDetector) OnOutput(bytes int) {
	// No-op for mock
}

func (m *MockStateDetector) OnScreenUpdate(lines []string) {
	// No-op for mock
}
