package session

import (
	"testing"
	"time"
)

// --- Test Status Constants ---

func TestSessionStatusConstants(t *testing.T) {
	tests := []struct {
		constant string
		expected string
	}{
		{StatusInitializing, "initializing"},
		{StatusRunning, "running"},
		{StatusPaused, "paused"},
		{StatusDisconnected, "disconnected"},
		{StatusOrphaned, "orphaned"},
		{StatusCompleted, "completed"},
		{StatusTerminated, "terminated"},
		{StatusError, "error"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
		}
	}
}

func TestLegacyAliases(t *testing.T) {
	// Test that legacy aliases match the new constants
	if SessionStatusInitializing != StatusInitializing {
		t.Error("SessionStatusInitializing should equal StatusInitializing")
	}
	if SessionStatusRunning != StatusRunning {
		t.Error("SessionStatusRunning should equal StatusRunning")
	}
	if SessionStatusPaused != StatusPaused {
		t.Error("SessionStatusPaused should equal StatusPaused")
	}
	if SessionStatusDisconnected != StatusDisconnected {
		t.Error("SessionStatusDisconnected should equal StatusDisconnected")
	}
	if SessionStatusOrphaned != StatusOrphaned {
		t.Error("SessionStatusOrphaned should equal StatusOrphaned")
	}
	if SessionStatusTerminated != StatusTerminated {
		t.Error("SessionStatusTerminated should equal StatusTerminated")
	}
	if SessionStatusError != StatusError {
		t.Error("SessionStatusError should equal StatusError")
	}
}

func TestAgentStatusConstants(t *testing.T) {
	tests := []struct {
		constant string
		expected string
	}{
		{AgentStatusUnknown, "unknown"},
		{AgentStatusIdle, "idle"},
		{AgentStatusWorking, "working"},
		{AgentStatusWaiting, "waiting"},
		{AgentStatusFinished, "finished"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
		}
	}
}

func TestThinkLevelConstants(t *testing.T) {
	if ThinkLevelNone != "" {
		t.Errorf("expected empty string, got '%s'", ThinkLevelNone)
	}
	if ThinkLevelUltrathink != "ultrathink" {
		t.Errorf("expected 'ultrathink', got '%s'", ThinkLevelUltrathink)
	}
	if ThinkLevelMegathink != "megathink" {
		t.Errorf("expected 'megathink', got '%s'", ThinkLevelMegathink)
	}
}

func TestPermissionModeConstants(t *testing.T) {
	tests := []struct {
		constant string
		expected string
	}{
		{PermissionModePlan, "plan"},
		{PermissionModeDefault, "default"},
		{PermissionModeBypass, "bypassPermissions"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
		}
	}
}

// --- Test Session ---

func TestSessionTableName(t *testing.T) {
	s := Session{}
	if s.TableName() != "sessions" {
		t.Errorf("expected 'sessions', got %s", s.TableName())
	}
}

func TestSessionIsActive(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{"running is active", StatusRunning, true},
		{"initializing is active", StatusInitializing, true},
		{"paused is active", StatusPaused, true},
		{"disconnected is active", StatusDisconnected, true},
		{"completed not active", StatusCompleted, false},
		{"terminated not active", StatusTerminated, false},
		{"orphaned not active", StatusOrphaned, false},
		{"error not active", StatusError, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{Status: tt.status}
			if s.IsActive() != tt.expected {
				t.Errorf("expected IsActive() = %v, got %v", tt.expected, s.IsActive())
			}
		})
	}
}

func TestSessionIsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{"terminated is terminal", StatusTerminated, true},
		{"orphaned is terminal", StatusOrphaned, true},
		{"error is terminal", StatusError, true},
		{"running not terminal", StatusRunning, false},
		{"paused not terminal", StatusPaused, false},
		{"completed not terminal", StatusCompleted, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{Status: tt.status}
			if s.IsTerminal() != tt.expected {
				t.Errorf("expected IsTerminal() = %v, got %v", tt.expected, s.IsTerminal())
			}
		})
	}
}

func TestSessionCanReconnect(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{"disconnected can reconnect", StatusDisconnected, true},
		{"running cannot reconnect", StatusRunning, false},
		{"terminated cannot reconnect", StatusTerminated, false},
		{"completed cannot reconnect", StatusCompleted, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &Session{Status: tt.status}
			if s.CanReconnect() != tt.expected {
				t.Errorf("expected CanReconnect() = %v, got %v", tt.expected, s.CanReconnect())
			}
		})
	}
}

func TestSessionStruct(t *testing.T) {
	now := time.Now()
	teamID := int64(10)
	model := "opus"
	permMode := "default"
	branch := "feature/test"

	s := Session{
		ID:             1,
		OrganizationID: 100,
		TeamID:         &teamID,
		SessionKey:     "sess-123",
		RunnerID:       5,
		CreatedByID:    50,
		Status:         StatusRunning,
		AgentStatus:    AgentStatusWorking,
		InitialPrompt:  "Test prompt",
		BranchName:     &branch,
		Model:          &model,
		PermissionMode: &permMode,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if s.ID != 1 {
		t.Errorf("expected ID 1, got %d", s.ID)
	}
	if s.SessionKey != "sess-123" {
		t.Errorf("expected SessionKey 'sess-123', got %s", s.SessionKey)
	}
	if *s.Model != "opus" {
		t.Errorf("expected Model 'opus', got %s", *s.Model)
	}
}

// --- Test PreparationConfig ---

func TestPreparationConfigStruct(t *testing.T) {
	config := PreparationConfig{
		Script:  "npm install",
		Timeout: 300,
	}

	if config.Script != "npm install" {
		t.Errorf("expected Script 'npm install', got %s", config.Script)
	}
	if config.Timeout != 300 {
		t.Errorf("expected Timeout 300, got %d", config.Timeout)
	}
}

// --- Test CreateSessionCommand ---

func TestCreateSessionCommandStruct(t *testing.T) {
	cmd := CreateSessionCommand{
		SessionKey:       "sess-456",
		InitialCommand:   "bash",
		InitialPrompt:    "Start working",
		PermissionMode:   "bypassPermissions",
		TicketIdentifier: "TICKET-123",
		WorktreeSuffix:   "v1",
		EnvVars:          map[string]string{"FOO": "bar"},
		PreparationConfig: &PreparationConfig{
			Script:  "npm ci",
			Timeout: 120,
		},
	}

	if cmd.SessionKey != "sess-456" {
		t.Errorf("expected SessionKey 'sess-456', got %s", cmd.SessionKey)
	}
	if cmd.TicketIdentifier != "TICKET-123" {
		t.Errorf("expected TicketIdentifier 'TICKET-123', got %s", cmd.TicketIdentifier)
	}
	if cmd.EnvVars["FOO"] != "bar" {
		t.Error("expected EnvVars['FOO'] = 'bar'")
	}
	if cmd.PreparationConfig.Script != "npm ci" {
		t.Errorf("expected PreparationConfig.Script 'npm ci', got %s", cmd.PreparationConfig.Script)
	}
}

// --- Test TerminateSessionCommand ---

func TestTerminateSessionCommandStruct(t *testing.T) {
	cmd := TerminateSessionCommand{
		SessionKey: "sess-789",
	}

	if cmd.SessionKey != "sess-789" {
		t.Errorf("expected SessionKey 'sess-789', got %s", cmd.SessionKey)
	}
}

// --- Benchmark Tests ---

func BenchmarkSessionIsActive(b *testing.B) {
	s := &Session{Status: StatusRunning}
	for i := 0; i < b.N; i++ {
		s.IsActive()
	}
}

func BenchmarkSessionIsTerminal(b *testing.B) {
	s := &Session{Status: StatusTerminated}
	for i := 0; i < b.N; i++ {
		s.IsTerminal()
	}
}

func BenchmarkSessionCanReconnect(b *testing.B) {
	s := &Session{Status: StatusDisconnected}
	for i := 0; i < b.N; i++ {
		s.CanReconnect()
	}
}
