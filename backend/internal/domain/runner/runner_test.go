package runner

import (
	"testing"
	"time"
)

// --- Test HostInfo ---

func TestHostInfoScanNil(t *testing.T) {
	var hi HostInfo
	err := hi.Scan(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if hi != nil {
		t.Error("expected nil HostInfo")
	}
}

func TestHostInfoScanValid(t *testing.T) {
	var hi HostInfo
	err := hi.Scan([]byte(`{"os": "darwin", "arch": "arm64"}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if hi["os"] != "darwin" {
		t.Errorf("expected os 'darwin', got %v", hi["os"])
	}
	if hi["arch"] != "arm64" {
		t.Errorf("expected arch 'arm64', got %v", hi["arch"])
	}
}

func TestHostInfoScanInvalidType(t *testing.T) {
	var hi HostInfo
	err := hi.Scan("not bytes")
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestHostInfoScanInvalidJSON(t *testing.T) {
	var hi HostInfo
	err := hi.Scan([]byte(`invalid json`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestHostInfoValueNil(t *testing.T) {
	var hi HostInfo
	val, err := hi.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != nil {
		t.Error("expected nil value")
	}
}

func TestHostInfoValueValid(t *testing.T) {
	hi := HostInfo{"os": "linux", "version": "5.4"}
	val, err := hi.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val == nil {
		t.Error("expected non-nil value")
	}
}

// --- Test RegistrationToken ---

func TestRegistrationTokenTableName(t *testing.T) {
	token := RegistrationToken{}
	if token.TableName() != "runner_registration_tokens" {
		t.Errorf("expected TableName 'runner_registration_tokens', got %s", token.TableName())
	}
}

func TestRegistrationTokenStruct(t *testing.T) {
	maxUses := 10
	expiresAt := time.Now().Add(24 * time.Hour)

	token := RegistrationToken{
		ID:             1,
		OrganizationID: 100,
		TokenHash:      "hash123",
		Description:    "Test token",
		CreatedByID:    50,
		IsActive:       true,
		MaxUses:        &maxUses,
		UsedCount:      5,
		ExpiresAt:      &expiresAt,
		CreatedAt:      time.Now(),
	}

	if token.ID != 1 {
		t.Errorf("expected ID 1, got %d", token.ID)
	}
	if token.OrganizationID != 100 {
		t.Errorf("expected OrganizationID 100, got %d", token.OrganizationID)
	}
	if !token.IsActive {
		t.Error("expected IsActive true")
	}
	if *token.MaxUses != 10 {
		t.Errorf("expected MaxUses 10, got %d", *token.MaxUses)
	}
}

// --- Test Runner Status Constants ---

func TestRunnerStatusConstants(t *testing.T) {
	if RunnerStatusOnline != "online" {
		t.Errorf("expected 'online', got %s", RunnerStatusOnline)
	}
	if RunnerStatusOffline != "offline" {
		t.Errorf("expected 'offline', got %s", RunnerStatusOffline)
	}
	if RunnerStatusBusy != "busy" {
		t.Errorf("expected 'busy', got %s", RunnerStatusBusy)
	}
}

// --- Test Runner ---

func TestRunnerTableName(t *testing.T) {
	runner := Runner{}
	if runner.TableName() != "runners" {
		t.Errorf("expected TableName 'runners', got %s", runner.TableName())
	}
}

func TestRunnerIsOnline(t *testing.T) {
	tests := []struct {
		name     string
		status   string
		expected bool
	}{
		{"online status", RunnerStatusOnline, true},
		{"offline status", RunnerStatusOffline, false},
		{"busy status", RunnerStatusBusy, false},
		{"empty status", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{Status: tt.status}
			if r.IsOnline() != tt.expected {
				t.Errorf("expected IsOnline() = %v, got %v", tt.expected, r.IsOnline())
			}
		})
	}
}

func TestRunnerCanAcceptSession(t *testing.T) {
	tests := []struct {
		name              string
		isEnabled         bool
		status            string
		currentSessions   int
		maxSessions       int
		expected          bool
	}{
		{
			name:            "can accept - all conditions met",
			isEnabled:       true,
			status:          RunnerStatusOnline,
			currentSessions: 2,
			maxSessions:     5,
			expected:        true,
		},
		{
			name:            "cannot accept - disabled",
			isEnabled:       false,
			status:          RunnerStatusOnline,
			currentSessions: 2,
			maxSessions:     5,
			expected:        false,
		},
		{
			name:            "cannot accept - offline",
			isEnabled:       true,
			status:          RunnerStatusOffline,
			currentSessions: 2,
			maxSessions:     5,
			expected:        false,
		},
		{
			name:            "cannot accept - at capacity",
			isEnabled:       true,
			status:          RunnerStatusOnline,
			currentSessions: 5,
			maxSessions:     5,
			expected:        false,
		},
		{
			name:            "cannot accept - over capacity",
			isEnabled:       true,
			status:          RunnerStatusOnline,
			currentSessions: 6,
			maxSessions:     5,
			expected:        false,
		},
		{
			name:            "can accept - one slot left",
			isEnabled:       true,
			status:          RunnerStatusOnline,
			currentSessions: 4,
			maxSessions:     5,
			expected:        true,
		},
		{
			name:            "can accept - zero sessions",
			isEnabled:       true,
			status:          RunnerStatusOnline,
			currentSessions: 0,
			maxSessions:     5,
			expected:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Runner{
				IsEnabled:             tt.isEnabled,
				Status:                tt.status,
				CurrentSessions:       tt.currentSessions,
				MaxConcurrentSessions: tt.maxSessions,
			}
			if r.CanAcceptSession() != tt.expected {
				t.Errorf("expected CanAcceptSession() = %v, got %v", tt.expected, r.CanAcceptSession())
			}
		})
	}
}

func TestRunnerStruct(t *testing.T) {
	now := time.Now()
	version := "1.0.0"

	r := Runner{
		ID:                    1,
		OrganizationID:        100,
		NodeID:                "node-001",
		Description:           "Test runner",
		AuthTokenHash:         "hash",
		Status:                RunnerStatusOnline,
		LastHeartbeat:         &now,
		CurrentSessions:       3,
		MaxConcurrentSessions: 10,
		RunnerVersion:         &version,
		IsEnabled:             true,
		HostInfo:              HostInfo{"os": "linux"},
		CreatedAt:             now,
		UpdatedAt:             now,
	}

	if r.ID != 1 {
		t.Errorf("expected ID 1, got %d", r.ID)
	}
	if r.NodeID != "node-001" {
		t.Errorf("expected NodeID 'node-001', got %s", r.NodeID)
	}
	if *r.RunnerVersion != "1.0.0" {
		t.Errorf("expected RunnerVersion '1.0.0', got %s", *r.RunnerVersion)
	}
	if r.HostInfo["os"] != "linux" {
		t.Errorf("expected HostInfo os 'linux', got %v", r.HostInfo["os"])
	}
}

// --- Benchmark Tests ---

func BenchmarkRunnerIsOnline(b *testing.B) {
	r := &Runner{Status: RunnerStatusOnline}
	for i := 0; i < b.N; i++ {
		r.IsOnline()
	}
}

func BenchmarkRunnerCanAcceptSession(b *testing.B) {
	r := &Runner{
		IsEnabled:             true,
		Status:                RunnerStatusOnline,
		CurrentSessions:       2,
		MaxConcurrentSessions: 5,
	}
	for i := 0; i < b.N; i++ {
		r.CanAcceptSession()
	}
}
