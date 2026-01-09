package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/client"
	"github.com/anthropics/agentmesh/runner/internal/config"
)

// TestNewRunnerWithMockConnection tests Runner creation with new Connection interface
func TestNewRunnerWithMockConnection(t *testing.T) {
	cfg := &config.Config{
		ServerURL:             "http://localhost:8080",
		NodeID:                "test-runner",
		AuthToken:             "test-token",
		WorkspaceRoot:         t.TempDir(),
		MaxConcurrentSessions: 5,
	}

	r, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	if r.conn == nil {
		t.Error("Runner connection should not be nil")
	}

	if r.sessionStore == nil {
		t.Error("Runner sessionStore should not be nil")
	}

	if r.messageHandler == nil {
		t.Error("Runner messageHandler should not be nil")
	}
}

// TestRunnerWithConnection tests WithConnection method
func TestRunnerWithConnection(t *testing.T) {
	cfg := &config.Config{
		ServerURL:             "http://localhost:8080",
		NodeID:                "test-runner",
		AuthToken:             "test-token",
		WorkspaceRoot:         t.TempDir(),
		MaxConcurrentSessions: 5,
	}

	r, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	mockConn := client.NewMockConnection()
	r.WithConnection(mockConn)

	if r.conn != mockConn {
		t.Error("Connection should be replaced with mock")
	}

	// Verify handler is set on mock connection
	// This is verified by simulating a message
}

// TestRunnerMessageHandlerOnListSessions tests the MessageHandler interface
func TestRunnerMessageHandlerOnListSessions(t *testing.T) {
	cfg := &config.Config{
		ServerURL:             "http://localhost:8080",
		NodeID:                "test-runner",
		AuthToken:             "test-token",
		WorkspaceRoot:         t.TempDir(),
		MaxConcurrentSessions: 5,
	}

	r, err := New(cfg)
	if err != nil {
		t.Fatalf("Failed to create runner: %v", err)
	}

	mockConn := client.NewMockConnection()
	r.WithConnection(mockConn)

	// Get sessions (should be empty initially)
	sessions := r.messageHandler.OnListSessions()
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions, got %d", len(sessions))
	}
}

// TestBuildWebSocketURL tests URL conversion
func TestBuildWebSocketURL(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"http://localhost:8080", "ws://localhost:8080/api/v1/runners/ws"},
		{"https://api.example.com", "wss://api.example.com/api/v1/runners/ws"},
		{"http://localhost:8080/", "ws://localhost:8080//api/v1/runners/ws"},
	}

	for _, tt := range tests {
		result := buildWebSocketURL(tt.input)
		if result != tt.expected {
			t.Errorf("buildWebSocketURL(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestMockConnectionInterface tests that MockConnection implements Connection
func TestMockConnectionInterface(t *testing.T) {
	var _ client.Connection = client.NewMockConnection()
}

// TestRunnerMessageHandlerInterface tests that RunnerMessageHandler implements MessageHandler
func TestRunnerMessageHandlerInterface(t *testing.T) {
	cfg := &config.Config{
		WorkspaceRoot: t.TempDir(),
	}
	r := &Runner{
		cfg:          cfg,
		sessionStore: NewInMemorySessionStore(),
	}
	mockConn := client.NewMockConnection()
	handler := NewRunnerMessageHandler(r, r.sessionStore, mockConn)

	var _ client.MessageHandler = handler
}

// --- Test Runner.Run ---

func TestRunnerRunMissingTokens(t *testing.T) {
	cfg := &config.Config{
		WorkspaceRoot: t.TempDir(),
		AuthToken:     "",
		RegistrationToken: "",
	}

	r := &Runner{
		cfg:          cfg,
		sessionStore: NewInMemorySessionStore(),
		stopChan:     make(chan struct{}),
	}

	mockConn := client.NewMockConnection()
	r.conn = mockConn

	ctx := context.Background()
	err := r.Run(ctx)

	if err == nil {
		t.Error("expected error for missing tokens")
	}
	if !contains(err.Error(), "no auth_token or registration_token") {
		t.Errorf("error = %v, want containing 'no auth_token or registration_token'", err)
	}
}

func TestRunnerRunWithAuthToken(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		WorkspaceRoot: tempDir,
		NodeID:        "test-node",
		AuthToken:     "test-token",
	}

	r := &Runner{
		cfg:          cfg,
		sessionStore: NewInMemorySessionStore(),
		stopChan:     make(chan struct{}),
	}

	mockConn := client.NewMockConnection()
	r.conn = mockConn

	// Run with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := r.Run(ctx)
	// Should exit cleanly on context cancellation
	if err != nil {
		t.Logf("Run returned: %v", err)
	}

	// Verify connection was started
	if !mockConn.IsStarted() {
		t.Error("connection should be started")
	}
}

func TestRunnerRunWithRegistrationTokenError(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		WorkspaceRoot:     tempDir,
		NodeID:            "test-node",
		AuthToken:         "",
		RegistrationToken: "test-reg-token",
		ServerURL:         "http://localhost:9999", // Non-existent server
	}

	r := &Runner{
		cfg:          cfg,
		sessionStore: NewInMemorySessionStore(),
		stopChan:     make(chan struct{}),
	}

	mockConn := client.NewMockConnection()
	r.conn = mockConn

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := r.Run(ctx)
	// Should fail due to registration error
	if err == nil {
		t.Error("expected error for registration failure")
	}
	if !contains(err.Error(), "registration failed") {
		t.Errorf("error = %v, want containing 'registration failed'", err)
	}
}

func TestRunnerRunStopAllSessions(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		WorkspaceRoot: tempDir,
		NodeID:        "test-node",
		AuthToken:     "test-token",
	}

	store := NewInMemorySessionStore()
	store.Put("session-1", &Session{ID: "session-1", SessionKey: "session-1"})

	r := &Runner{
		cfg:          cfg,
		sessionStore: store,
		stopChan:     make(chan struct{}),
	}

	mockConn := client.NewMockConnection()
	r.conn = mockConn

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	r.Run(ctx)

	// Verify sessions were cleaned up
	if store.Count() != 0 {
		t.Errorf("session count = %d, want 0", store.Count())
	}
}

// --- Test initEnhancedComponents ---

// Note: TestInitEnhancedComponentsWithWorktree removed - worktreeService has been
// replaced by sandbox plugins (WorktreePlugin). See sandbox/plugins/worktree_test.go.

func TestInitEnhancedComponentsWithMCPConfig(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		WorkspaceRoot: tempDir,
		MCPConfigPath: "/nonexistent/mcp.json", // Non-existent file - should log warning but not fail
	}

	r := &Runner{
		cfg:      cfg,
		sessions: make(map[string]*Session),
	}

	// Should not panic
	r.initEnhancedComponents(cfg)

	// MCP manager should still be initialized
	if r.mcpManager == nil {
		t.Error("mcpManager should be initialized")
	}
}

func TestInitEnhancedComponentsDefaultShell(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		WorkspaceRoot: tempDir,
		DefaultShell:  "", // Empty - should default to /bin/sh
	}

	r := &Runner{
		cfg:      cfg,
		sessions: make(map[string]*Session),
	}

	r.initEnhancedComponents(cfg)

	if r.termManager == nil {
		t.Error("termManager should be initialized")
	}
}

func TestInitEnhancedComponentsCustomShell(t *testing.T) {
	tempDir := t.TempDir()
	cfg := &config.Config{
		WorkspaceRoot: tempDir,
		DefaultShell:  "/bin/bash",
	}

	r := &Runner{
		cfg:      cfg,
		sessions: make(map[string]*Session),
	}

	r.initEnhancedComponents(cfg)

	if r.termManager == nil {
		t.Error("termManager should be initialized")
	}
}

// --- Test stopAllSessions ---

func TestStopAllSessionsWithTerminals(t *testing.T) {
	store := NewInMemorySessionStore()
	store.Put("session-1", &Session{
		ID:         "session-1",
		SessionKey: "session-1",
		Terminal:   nil,
	})
	store.Put("session-2", &Session{
		ID:         "session-2",
		SessionKey: "session-2",
		Terminal:   nil,
	})

	r := &Runner{
		cfg:          &config.Config{},
		sessionStore: store,
	}

	r.stopAllSessions()

	if store.Count() != 0 {
		t.Errorf("session count = %d, want 0", store.Count())
	}
}

// --- Test buildWebSocketURL edge cases ---

func TestBuildWebSocketURLPlainURL(t *testing.T) {
	result := buildWebSocketURL("localhost:8080")
	expected := "localhost:8080/api/v1/runners/ws"
	if result != expected {
		t.Errorf("buildWebSocketURL(plain) = %s, want %s", result, expected)
	}
}

// --- Test MockConnection helpers ---

func TestMockConnectionSimulateCreateSession(t *testing.T) {
	mockConn := client.NewMockConnection()
	store := NewInMemorySessionStore()

	tempDir := t.TempDir()
	r := &Runner{
		cfg: &config.Config{
			WorkspaceRoot:         tempDir,
			MaxConcurrentSessions: 10,
		},
		sessionStore: store,
	}

	handler := NewRunnerMessageHandler(r, store, mockConn)
	mockConn.SetHandler(handler)

	req := client.CreateSessionRequest{
		SessionID:      "mock-session",
		InitialCommand: "echo",
		WorkingDir:     tempDir,
	}

	err := mockConn.SimulateCreateSession(req)
	if err != nil {
		t.Logf("SimulateCreateSession: %v", err)
	}

	// Clean up
	session, ok := store.Get("mock-session")
	if ok && session.Terminal != nil {
		session.Terminal.Stop()
	}
}

func TestMockConnectionSimulateTerminateSession(t *testing.T) {
	mockConn := client.NewMockConnection()
	store := NewInMemorySessionStore()

	r := &Runner{
		cfg: &config.Config{},
	}

	store.Put("terminate-mock", &Session{
		ID:         "terminate-mock",
		SessionKey: "terminate-mock",
		Terminal:   nil,
	})

	handler := NewRunnerMessageHandler(r, store, mockConn)
	mockConn.SetHandler(handler)

	req := client.TerminateSessionRequest{
		SessionID: "terminate-mock",
	}

	err := mockConn.SimulateTerminateSession(req)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	_, exists := store.Get("terminate-mock")
	if exists {
		t.Error("session should be removed")
	}
}

func TestMockConnectionGetSessions(t *testing.T) {
	mockConn := client.NewMockConnection()
	store := NewInMemorySessionStore()

	r := &Runner{cfg: &config.Config{}}

	store.Put("list-session", &Session{
		ID:         "list-session",
		SessionKey: "list-session",
		Status:     SessionStatusRunning,
	})

	handler := NewRunnerMessageHandler(r, store, mockConn)
	mockConn.SetHandler(handler)

	sessions := mockConn.GetSessions()
	if len(sessions) != 1 {
		t.Errorf("sessions count = %d, want 1", len(sessions))
	}
}

func TestMockConnectionReset(t *testing.T) {
	mockConn := client.NewMockConnection()

	// Send some events
	mockConn.SendEvent(client.MsgTypeSessionCreated, map[string]string{"test": "data"})
	mockConn.Start()

	// Verify state
	if len(mockConn.GetEvents()) == 0 {
		t.Error("should have events before reset")
	}

	// Reset
	mockConn.Reset()

	// Verify state is cleared
	if len(mockConn.GetEvents()) != 0 {
		t.Errorf("events count after reset = %d, want 0", len(mockConn.GetEvents()))
	}
	if mockConn.IsStarted() {
		t.Error("should not be started after reset")
	}
}

func TestMockConnectionConnectError(t *testing.T) {
	mockConn := client.NewMockConnection()
	mockConn.ConnectErr = errors.New("connection refused")

	err := mockConn.Connect()
	if err == nil {
		t.Error("expected error for ConnectErr")
	}
	if !contains(err.Error(), "connection refused") {
		t.Errorf("error = %v, want containing 'connection refused'", err)
	}
}

func TestMockConnectionSendWithBackpressureWhenStopped(t *testing.T) {
	mockConn := client.NewMockConnection()
	mockConn.Stop()

	msg := client.ProtocolMessage{Type: "test"}
	ok := mockConn.SendWithBackpressure(msg)
	if ok {
		t.Error("SendWithBackpressure should return false when stopped")
	}
}

func TestMockConnectionQueueLength(t *testing.T) {
	mockConn := client.NewMockConnection()

	if mockConn.QueueLength() != 0 {
		t.Errorf("initial queue length = %d, want 0", mockConn.QueueLength())
	}

	mockConn.Send(client.ProtocolMessage{Type: "test1"})
	mockConn.Send(client.ProtocolMessage{Type: "test2"})

	if mockConn.QueueLength() != 2 {
		t.Errorf("queue length = %d, want 2", mockConn.QueueLength())
	}
}

func TestMockConnectionQueueCapacity(t *testing.T) {
	mockConn := client.NewMockConnection()

	if mockConn.QueueCapacity() != 100 {
		t.Errorf("queue capacity = %d, want 100", mockConn.QueueCapacity())
	}
}
