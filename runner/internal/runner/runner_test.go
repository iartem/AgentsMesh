package runner

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/config"
)

// --- Test Constants ---

func TestSessionStatusConstants(t *testing.T) {
	if SessionStatusInitializing != "initializing" {
		t.Errorf("SessionStatusInitializing: got %v, want initializing", SessionStatusInitializing)
	}
	if SessionStatusRunning != "running" {
		t.Errorf("SessionStatusRunning: got %v, want running", SessionStatusRunning)
	}
	if SessionStatusStopped != "stopped" {
		t.Errorf("SessionStatusStopped: got %v, want stopped", SessionStatusStopped)
	}
	if SessionStatusFailed != "failed" {
		t.Errorf("SessionStatusFailed: got %v, want failed", SessionStatusFailed)
	}
}

// --- Test Session Struct ---

func TestSessionStruct(t *testing.T) {
	now := time.Now()
	session := Session{
		ID:               "session-1",
		SessionKey:       "key-123",
		AgentType:        "claude-code",
		RepositoryURL:    "https://github.com/test/repo.git",
		Branch:           "main",
		WorktreePath:     "/workspace/worktrees/session-1",
		InitialPrompt:    "Hello",
		Terminal:         nil,
		StartedAt:        now,
		Status:           SessionStatusRunning,
		TicketIdentifier: "TICKET-123",
	}

	if session.ID != "session-1" {
		t.Errorf("ID: got %v, want session-1", session.ID)
	}
	if session.SessionKey != "key-123" {
		t.Errorf("SessionKey: got %v, want key-123", session.SessionKey)
	}
	if session.AgentType != "claude-code" {
		t.Errorf("AgentType: got %v, want claude-code", session.AgentType)
	}
	if session.Status != SessionStatusRunning {
		t.Errorf("Status: got %v, want running", session.Status)
	}
	if session.TicketIdentifier != "TICKET-123" {
		t.Errorf("TicketIdentifier: got %v, want TICKET-123", session.TicketIdentifier)
	}
}

func TestSessionAllFields(t *testing.T) {
	now := time.Now()
	forwarder := &PTYForwarder{sessionKey: "test"}

	session := &Session{
		ID:               "id-1",
		SessionKey:       "key-1",
		AgentType:        "claude-code",
		RepositoryURL:    "https://github.com/test/repo.git",
		Branch:           "feature/test",
		WorktreePath:     "/workspace/worktrees/test",
		InitialPrompt:    "Hello, Claude!",
		Terminal:         nil,
		StartedAt:        now,
		Status:           SessionStatusRunning,
		TicketIdentifier: "TICKET-123",
		OnOutput:         func([]byte) {},
		OnExit:           func(int) {},
		Forwarder:        forwarder,
	}

	if session.OnOutput == nil {
		t.Error("OnOutput should not be nil")
	}
	if session.OnExit == nil {
		t.Error("OnExit should not be nil")
	}
	if session.Forwarder == nil {
		t.Error("Forwarder should not be nil")
	}
}

func TestSessionWithCallbacks(t *testing.T) {
	outputCalled := false
	exitCalled := false

	session := &Session{
		ID:     "session-1",
		Status: SessionStatusRunning,
		OnOutput: func(data []byte) {
			outputCalled = true
		},
		OnExit: func(exitCode int) {
			exitCalled = true
		},
	}

	if session.OnOutput != nil {
		session.OnOutput([]byte("test"))
	}
	if session.OnExit != nil {
		session.OnExit(0)
	}

	if !outputCalled {
		t.Error("OnOutput should be called")
	}
	if !exitCalled {
		t.Error("OnExit should be called")
	}
}

// --- Test Payload Structs ---

func TestSessionStartPayload(t *testing.T) {
	payload := SessionStartPayload{
		SessionKey:       "session-1",
		AgentType:        "claude-code",
		LaunchCommand:    "claude",
		LaunchArgs:       []string{"--headless"},
		EnvVars:          map[string]string{"API_KEY": "secret"},
		RepositoryURL:    "https://github.com/test/repo.git",
		Branch:           "main",
		InitialPrompt:    "Hello",
		Rows:             24,
		Cols:             80,
		TicketIdentifier: "TICKET-123",
		PrepScript:       "npm install",
		PrepTimeout:      300,
	}

	if payload.SessionKey != "session-1" {
		t.Errorf("SessionKey: got %v, want session-1", payload.SessionKey)
	}
	if payload.AgentType != "claude-code" {
		t.Errorf("AgentType: got %v, want claude-code", payload.AgentType)
	}
	if len(payload.LaunchArgs) != 1 {
		t.Errorf("LaunchArgs length: got %v, want 1", len(payload.LaunchArgs))
	}
	if payload.EnvVars["API_KEY"] != "secret" {
		t.Errorf("EnvVars[API_KEY]: got %v, want secret", payload.EnvVars["API_KEY"])
	}
}

func TestSessionStartPayloadJSON(t *testing.T) {
	jsonStr := `{
		"session_key": "session-1",
		"agent_type": "claude-code",
		"launch_command": "claude",
		"launch_args": ["--headless"],
		"env_vars": {"API_KEY": "secret"},
		"rows": 24,
		"cols": 80,
		"ticket_identifier": "TICKET-123"
	}`

	var payload SessionStartPayload
	if err := json.Unmarshal([]byte(jsonStr), &payload); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if payload.SessionKey != "session-1" {
		t.Errorf("SessionKey: got %v, want session-1", payload.SessionKey)
	}
	if payload.TicketIdentifier != "TICKET-123" {
		t.Errorf("TicketIdentifier: got %v, want TICKET-123", payload.TicketIdentifier)
	}
}

func TestSessionStopPayload(t *testing.T) {
	payload := SessionStopPayload{SessionKey: "session-1"}

	if payload.SessionKey != "session-1" {
		t.Errorf("SessionKey: got %v, want session-1", payload.SessionKey)
	}
}

func TestSessionStopPayloadJSON(t *testing.T) {
	jsonStr := `{"session_key": "session-1"}`

	var payload SessionStopPayload
	if err := json.Unmarshal([]byte(jsonStr), &payload); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if payload.SessionKey != "session-1" {
		t.Errorf("SessionKey: got %v, want session-1", payload.SessionKey)
	}
}

func TestTerminalInputPayload(t *testing.T) {
	payload := TerminalInputPayload{
		SessionKey: "session-1",
		Data:       []byte("hello"),
	}

	if payload.SessionKey != "session-1" {
		t.Errorf("SessionKey: got %v, want session-1", payload.SessionKey)
	}
	if string(payload.Data) != "hello" {
		t.Errorf("Data: got %v, want hello", string(payload.Data))
	}
}

func TestTerminalResizePayload(t *testing.T) {
	payload := TerminalResizePayload{
		SessionKey: "session-1",
		Rows:       40,
		Cols:       120,
	}

	if payload.SessionKey != "session-1" {
		t.Errorf("SessionKey: got %v, want session-1", payload.SessionKey)
	}
	if payload.Rows != 40 {
		t.Errorf("Rows: got %v, want 40", payload.Rows)
	}
	if payload.Cols != 120 {
		t.Errorf("Cols: got %v, want 120", payload.Cols)
	}
}

func TestSessionListPayloadJSON(t *testing.T) {
	jsonStr := `{"request_id": "req-123"}`

	var payload SessionListPayload
	if err := json.Unmarshal([]byte(jsonStr), &payload); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if payload.RequestID != "req-123" {
		t.Errorf("RequestID: got %v, want req-123", payload.RequestID)
	}
}

// --- Test Runner Struct ---

func TestRunnerStruct(t *testing.T) {
	r := &Runner{
		sessions: make(map[string]*Session),
		stopChan: make(chan struct{}),
	}

	if r.sessions == nil {
		t.Error("sessions should be initialized")
	}
	if r.stopChan == nil {
		t.Error("stopChan should be initialized")
	}
}

func TestRunnerStructFields(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			NodeID:                "node-1",
			MaxConcurrentSessions: 5,
		},
		sessions: make(map[string]*Session),
		stopChan: make(chan struct{}),
	}

	if r.cfg == nil {
		t.Error("cfg should not be nil")
	}
	if r.cfg.NodeID != "node-1" {
		t.Errorf("cfg.NodeID = %v, want node-1", r.cfg.NodeID)
	}
	if r.sessions == nil {
		t.Error("sessions should not be nil")
	}
}

// --- Test Runner Methods ---

func TestRunnerStopAllSessionsEmpty(t *testing.T) {
	store := NewInMemorySessionStore()
	r := &Runner{
		sessions:     make(map[string]*Session),
		sessionStore: store,
	}

	// Should not panic with empty sessions
	r.stopAllSessions()
}

func TestRunnerStopAllSessionsWithNilTerminal(t *testing.T) {
	store := NewInMemorySessionStore()
	store.Put("session-1", &Session{ID: "session-1", SessionKey: "session-1", Terminal: nil})
	r := &Runner{
		sessions:     make(map[string]*Session),
		sessionStore: store,
	}

	// Should not panic
	r.stopAllSessions()

	if store.Count() != 0 {
		t.Errorf("sessions should be empty after stopAllSessions")
	}
}

// --- Test buildWebSocketURL ---

func TestBuildWebSocketURLHTTP(t *testing.T) {
	result := buildWebSocketURL("http://localhost:8080")
	expected := "ws://localhost:8080/api/v1/runners/ws"
	if result != expected {
		t.Errorf("buildWebSocketURL(http): got %v, want %v", result, expected)
	}
}

func TestBuildWebSocketURLHTTPS(t *testing.T) {
	result := buildWebSocketURL("https://api.example.com")
	expected := "wss://api.example.com/api/v1/runners/ws"
	if result != expected {
		t.Errorf("buildWebSocketURL(https): got %v, want %v", result, expected)
	}
}

// --- Test ExtendedSession ---

func TestExtendedSessionStruct(t *testing.T) {
	session := &Session{ID: "session-1", Status: SessionStatusRunning}

	extended := ExtendedSession{
		Session:          session,
		OnOutput:         func([]byte) {},
		OnExit:           func(int) {},
		TicketIdentifier: "TICKET-123",
		ManagedSession:   nil,
	}

	if extended.ID != "session-1" {
		t.Errorf("ID: got %v, want session-1", extended.ID)
	}
	if extended.TicketIdentifier != "TICKET-123" {
		t.Errorf("TicketIdentifier: got %v, want TICKET-123", extended.TicketIdentifier)
	}
}

// --- Benchmarks ---

func BenchmarkSessionStartPayloadUnmarshal(b *testing.B) {
	jsonStr := []byte(`{
		"session_key": "session-1",
		"agent_type": "claude-code",
		"launch_command": "claude",
		"launch_args": ["--headless"],
		"rows": 24,
		"cols": 80
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var payload SessionStartPayload
		json.Unmarshal(jsonStr, &payload)
	}
}

func BenchmarkBuildWebSocketURL(b *testing.B) {
	urls := []string{
		"http://localhost:8080",
		"https://api.example.com",
		"http://192.168.1.1:3000",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildWebSocketURL(urls[i%len(urls)])
	}
}
