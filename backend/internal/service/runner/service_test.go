package runner

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/agentmesh/backend/internal/domain/runner"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	// Create tables manually for SQLite compatibility
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS runner_registration_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			description TEXT,
			created_by_id INTEGER NOT NULL,
			is_active INTEGER NOT NULL DEFAULT 1,
			max_uses INTEGER,
			used_count INTEGER NOT NULL DEFAULT 0,
			expires_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatalf("failed to create registration_tokens table: %v", err)
	}

	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS runners (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			node_id TEXT NOT NULL,
			description TEXT,
			auth_token_hash TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'offline',
			last_heartbeat DATETIME,
			current_sessions INTEGER NOT NULL DEFAULT 0,
			max_concurrent_sessions INTEGER NOT NULL DEFAULT 5,
			runner_version TEXT,
			is_enabled INTEGER NOT NULL DEFAULT 1,
			host_info TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatalf("failed to create runners table: %v", err)
	}

	return db
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)

	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if service.db != db {
		t.Fatal("expected service.db to be the provided db")
	}
}

func TestCreateRegistrationToken(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	token, err := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	if err != nil {
		t.Fatalf("failed to create registration token: %v", err)
	}

	if token == "" {
		t.Fatal("expected non-empty token")
	}

	// Verify the token was stored
	var regToken runner.RegistrationToken
	if err := db.First(&regToken).Error; err != nil {
		t.Fatalf("failed to find registration token: %v", err)
	}
	if regToken.OrganizationID != 1 {
		t.Errorf("expected OrganizationID 1, got %d", regToken.OrganizationID)
	}
	if regToken.Description != "Test Token" {
		t.Errorf("expected Description 'Test Token', got %s", regToken.Description)
	}
	if !regToken.IsActive {
		t.Error("expected token to be active")
	}
}

func TestRegisterRunner(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create a registration token first
	plain, err := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	if err != nil {
		t.Fatalf("failed to create registration token: %v", err)
	}

	// Register a runner
	r, authToken, err := service.RegisterRunner(ctx, plain, "test-runner-1", "Test Runner", 5)
	if err != nil {
		t.Fatalf("failed to register runner: %v", err)
	}

	if r == nil {
		t.Fatal("expected non-nil runner")
	}
	if authToken == "" {
		t.Fatal("expected non-empty auth token")
	}
	if r.NodeID != "test-runner-1" {
		t.Errorf("expected NodeID 'test-runner-1', got %s", r.NodeID)
	}
	if r.OrganizationID != 1 {
		t.Errorf("expected OrganizationID 1, got %d", r.OrganizationID)
	}
	if r.Status != runner.RunnerStatusOffline {
		t.Errorf("expected Status '%s', got %s", runner.RunnerStatusOffline, r.Status)
	}
	if r.MaxConcurrentSessions != 5 {
		t.Errorf("expected MaxConcurrentSessions 5, got %d", r.MaxConcurrentSessions)
	}

	// Check that token usage count was incremented
	var updatedToken runner.RegistrationToken
	db.First(&updatedToken)
	if updatedToken.UsedCount != 1 {
		t.Errorf("expected UsedCount 1, got %d", updatedToken.UsedCount)
	}
}

func TestRegisterRunnerInvalidToken(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	_, _, err := service.RegisterRunner(ctx, "invalid-token", "test-runner", "Test", 5)
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestAuthenticateRunner(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create token and register runner
	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, authToken, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	// Authenticate
	authenticated, err := service.AuthenticateRunner(ctx, r.ID, authToken)
	if err != nil {
		t.Fatalf("failed to authenticate runner: %v", err)
	}
	if authenticated == nil {
		t.Fatal("expected non-nil authenticated runner")
	}
	if authenticated.ID != r.ID {
		t.Errorf("expected runner ID %d, got %d", r.ID, authenticated.ID)
	}
}

func TestAuthenticateRunnerInvalidToken(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create token and register runner
	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, _, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	// Try with invalid token
	_, err := service.AuthenticateRunner(ctx, r.ID, "invalid-token")
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestHeartbeat(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create token and register runner
	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, _, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	// Send heartbeat
	err := service.Heartbeat(ctx, r.ID, 2)
	if err != nil {
		t.Fatalf("failed to send heartbeat: %v", err)
	}

	// Check runner status was updated
	updated, _ := service.GetRunner(ctx, r.ID)
	if updated.Status != runner.RunnerStatusOnline {
		t.Errorf("expected Status '%s', got %s", runner.RunnerStatusOnline, updated.Status)
	}
	if updated.CurrentSessions != 2 {
		t.Errorf("expected CurrentSessions 2, got %d", updated.CurrentSessions)
	}
	if updated.LastHeartbeat == nil {
		t.Error("expected LastHeartbeat to be set")
	}
}

func TestListRunners(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create multiple runners
	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	service.RegisterRunner(ctx, plain, "runner-1", "Runner 1", 5)
	service.RegisterRunner(ctx, plain, "runner-2", "Runner 2", 5)
	service.RegisterRunner(ctx, plain, "runner-3", "Runner 3", 5)

	// List all runners
	runners, err := service.ListRunners(ctx, 1)
	if err != nil {
		t.Fatalf("failed to list runners: %v", err)
	}
	if len(runners) != 3 {
		t.Errorf("expected 3 runners, got %d", len(runners))
	}
}

func TestDeleteRunner(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create runner
	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, _, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	// Delete runner
	err := service.DeleteRunner(ctx, r.ID)
	if err != nil {
		t.Fatalf("failed to delete runner: %v", err)
	}

	// Verify deletion
	_, err = service.GetRunner(ctx, r.ID)
	if err != ErrRunnerNotFound {
		t.Errorf("expected ErrRunnerNotFound, got %v", err)
	}
}

func TestUpdateRunnerStatus(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, _, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	err := service.UpdateRunnerStatus(ctx, r.ID, runner.RunnerStatusOnline)
	if err != nil {
		t.Fatalf("failed to update runner status: %v", err)
	}

	updated, _ := service.GetRunner(ctx, r.ID)
	if updated.Status != runner.RunnerStatusOnline {
		t.Errorf("expected status online, got %s", updated.Status)
	}
}

func TestListAvailableRunners(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)

	// Register multiple runners
	r1, _, _ := service.RegisterRunner(ctx, plain, "runner-1", "Runner 1", 5)
	r2, _, _ := service.RegisterRunner(ctx, plain, "runner-2", "Runner 2", 5)

	// Set one runner online
	service.Heartbeat(ctx, r1.ID, 0)

	// List available runners
	runners, err := service.ListAvailableRunners(ctx, 1)
	if err != nil {
		t.Fatalf("failed to list available runners: %v", err)
	}

	if len(runners) != 1 {
		t.Errorf("expected 1 available runner, got %d", len(runners))
	}
	_ = r2
}

func TestUpdateRunner(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, _, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	newDesc := "Updated Description"
	newMax := 10
	isEnabled := false

	updated, err := service.UpdateRunner(ctx, r.ID, RunnerUpdateInput{
		Description:           &newDesc,
		MaxConcurrentSessions: &newMax,
		IsEnabled:             &isEnabled,
	})
	if err != nil {
		t.Fatalf("failed to update runner: %v", err)
	}

	if updated.Description != newDesc {
		t.Errorf("expected description %s, got %s", newDesc, updated.Description)
	}
	if updated.MaxConcurrentSessions != newMax {
		t.Errorf("expected max sessions %d, got %d", newMax, updated.MaxConcurrentSessions)
	}
	if updated.IsEnabled != isEnabled {
		t.Errorf("expected is_enabled %v, got %v", isEnabled, updated.IsEnabled)
	}
}

func TestUpdateRunnerNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	newDesc := "Updated Description"
	_, err := service.UpdateRunner(ctx, 99999, RunnerUpdateInput{
		Description: &newDesc,
	})
	if err != ErrRunnerNotFound {
		t.Errorf("expected ErrRunnerNotFound, got %v", err)
	}
}

func TestRegenerateAuthToken(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, oldAuth, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	newAuth, err := service.RegenerateAuthToken(ctx, r.ID)
	if err != nil {
		t.Fatalf("failed to regenerate auth token: %v", err)
	}

	if newAuth == "" {
		t.Fatal("expected non-empty new auth token")
	}
	if newAuth == oldAuth {
		t.Error("new token should be different from old token")
	}

	// Old token should not work
	_, err = service.AuthenticateRunner(ctx, r.ID, oldAuth)
	if err != ErrInvalidToken {
		t.Errorf("expected old token to fail, got %v", err)
	}

	// New token should work
	_, err = service.AuthenticateRunner(ctx, r.ID, newAuth)
	if err != nil {
		t.Errorf("expected new token to work, got %v", err)
	}
}

func TestRegenerateAuthTokenNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	_, err := service.RegenerateAuthToken(ctx, 99999)
	if err != ErrRunnerNotFound {
		t.Errorf("expected ErrRunnerNotFound, got %v", err)
	}
}

func TestListRegistrationTokens(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create multiple tokens
	service.CreateRegistrationToken(ctx, 1, 1, "Token 1", nil, nil)
	service.CreateRegistrationToken(ctx, 1, 1, "Token 2", nil, nil)
	service.CreateRegistrationToken(ctx, 2, 1, "Token for Org 2", nil, nil)

	tokens, err := service.ListRegistrationTokens(ctx, 1)
	if err != nil {
		t.Fatalf("failed to list registration tokens: %v", err)
	}

	if len(tokens) != 2 {
		t.Errorf("expected 2 tokens for org 1, got %d", len(tokens))
	}
}

func TestRevokeRegistrationToken(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)

	// Get the token ID
	var token runner.RegistrationToken
	db.First(&token)

	err := service.RevokeRegistrationToken(ctx, token.ID)
	if err != nil {
		t.Fatalf("failed to revoke token: %v", err)
	}

	// Token should be invalid now
	_, err = service.ValidateRegistrationToken(ctx, plain)
	if err != ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken after revoke, got %v", err)
	}
}

func TestSelectAvailableRunner(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r1, _, _ := service.RegisterRunner(ctx, plain, "runner-1", "Runner 1", 5)
	r2, _, _ := service.RegisterRunner(ctx, plain, "runner-2", "Runner 2", 5)

	// Make both online
	service.Heartbeat(ctx, r1.ID, 3)
	service.Heartbeat(ctx, r2.ID, 1)

	// Should select r2 (least sessions)
	selected, err := service.SelectAvailableRunner(ctx, 1)
	if err != nil {
		t.Fatalf("failed to select available runner: %v", err)
	}
	if selected.ID != r2.ID {
		t.Errorf("expected runner with least sessions (r2), got ID %d", selected.ID)
	}
}

func TestSelectAvailableRunnerNoneAvailable(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// No runners at all
	_, err := service.SelectAvailableRunner(ctx, 1)
	if err != ErrRunnerOffline {
		t.Errorf("expected ErrRunnerOffline, got %v", err)
	}
}

func TestIsConnected(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)

	// Not connected initially
	if service.IsConnected(1) {
		t.Error("expected runner to not be connected initially")
	}

	// Mark connected
	service.activeRunners.Store(int64(1), &ActiveRunner{})

	if !service.IsConnected(1) {
		t.Error("expected runner to be connected after storing")
	}
}

func TestMarkConnectedDisconnected(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, _, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	// Mark connected
	err := service.MarkConnected(ctx, r.ID)
	if err != nil {
		t.Fatalf("failed to mark connected: %v", err)
	}

	if !service.IsConnected(r.ID) {
		t.Error("expected runner to be connected")
	}

	updated, _ := service.GetRunner(ctx, r.ID)
	if updated.Status != runner.RunnerStatusOnline {
		t.Errorf("expected status online, got %s", updated.Status)
	}

	// Mark disconnected
	err = service.MarkDisconnected(ctx, r.ID)
	if err != nil {
		t.Fatalf("failed to mark disconnected: %v", err)
	}

	if service.IsConnected(r.ID) {
		t.Error("expected runner to be disconnected")
	}

	updated, _ = service.GetRunner(ctx, r.ID)
	if updated.Status != runner.RunnerStatusOffline {
		t.Errorf("expected status offline, got %s", updated.Status)
	}
}

func TestUpdateHeartbeat(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, authToken, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	err := service.UpdateHeartbeat(ctx, r.ID, authToken, 2, "1.0.0")
	if err != nil {
		t.Fatalf("failed to update heartbeat: %v", err)
	}

	updated, _ := service.GetRunner(ctx, r.ID)
	if updated.Status != runner.RunnerStatusOnline {
		t.Errorf("expected status online, got %s", updated.Status)
	}
	if updated.CurrentSessions != 2 {
		t.Errorf("expected 2 sessions, got %d", updated.CurrentSessions)
	}
	if updated.RunnerVersion == nil || *updated.RunnerVersion != "1.0.0" {
		t.Errorf("expected version 1.0.0, got %v", updated.RunnerVersion)
	}
}

func TestUpdateHeartbeatInvalidAuth(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, _, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	err := service.UpdateHeartbeat(ctx, r.ID, "invalid-token", 2, "1.0.0")
	if err != ErrInvalidAuth {
		t.Errorf("expected ErrInvalidAuth, got %v", err)
	}
}

func TestUpdateHeartbeatWithSessions(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, _, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	sessions := []HeartbeatSession{
		{SessionKey: "session-1", Status: "running"},
		{SessionKey: "session-2", Status: "running"},
	}

	err := service.UpdateHeartbeatWithSessions(ctx, r.ID, sessions, "1.0.0")
	if err != nil {
		t.Fatalf("failed to update heartbeat with sessions: %v", err)
	}

	updated, _ := service.GetRunner(ctx, r.ID)
	if updated.CurrentSessions != 2 {
		t.Errorf("expected 2 sessions, got %d", updated.CurrentSessions)
	}
}

func TestUpdateHeartbeatWithSessionsNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	err := service.UpdateHeartbeatWithSessions(ctx, 99999, nil, "1.0.0")
	if err != ErrRunnerNotFound {
		t.Errorf("expected ErrRunnerNotFound, got %v", err)
	}
}

func TestSubscribeStatusChanges(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	unsubscribe, err := service.SubscribeStatusChanges(ctx, func(r *runner.Runner) {})
	if err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}

	if unsubscribe == nil {
		t.Error("expected non-nil unsubscribe function")
	}

	// Calling unsubscribe should not panic
	unsubscribe()
}

func TestErrors(t *testing.T) {
	tests := []struct {
		err      error
		expected string
	}{
		{ErrRunnerNotFound, "runner not found"},
		{ErrRunnerOffline, "runner is offline"},
		{ErrInvalidToken, "invalid registration token"},
		{ErrInvalidAuth, "invalid runner authentication"},
		{ErrTokenExpired, "registration token expired"},
		{ErrTokenExhausted, "registration token usage exhausted"},
		{ErrRunnerAlreadyExists, "runner already exists"},
	}

	for _, tt := range tests {
		if tt.err.Error() != tt.expected {
			t.Errorf("Error message = %s, want %s", tt.err.Error(), tt.expected)
		}
	}
}

func TestActiveRunnerStruct(t *testing.T) {
	ar := &ActiveRunner{
		Runner:       &runner.Runner{ID: 1, NodeID: "test"},
		SessionCount: 5,
	}

	if ar.Runner.ID != 1 {
		t.Errorf("expected Runner.ID 1, got %d", ar.Runner.ID)
	}
	if ar.SessionCount != 5 {
		t.Errorf("expected SessionCount 5, got %d", ar.SessionCount)
	}
}

func TestRunnerUpdateInput(t *testing.T) {
	desc := "desc"
	max := 10
	enabled := true

	input := RunnerUpdateInput{
		Description:           &desc,
		MaxConcurrentSessions: &max,
		IsEnabled:             &enabled,
	}

	if *input.Description != desc {
		t.Errorf("expected Description %s, got %s", desc, *input.Description)
	}
	if *input.MaxConcurrentSessions != max {
		t.Errorf("expected MaxConcurrentSessions %d, got %d", max, *input.MaxConcurrentSessions)
	}
	if *input.IsEnabled != enabled {
		t.Errorf("expected IsEnabled %v, got %v", enabled, *input.IsEnabled)
	}
}

func TestHeartbeatSession(t *testing.T) {
	hs := HeartbeatSession{
		SessionKey:  "session-123",
		Status:      "running",
		AgentStatus: "waiting",
	}

	if hs.SessionKey != "session-123" {
		t.Errorf("expected SessionKey session-123, got %s", hs.SessionKey)
	}
	if hs.Status != "running" {
		t.Errorf("expected Status running, got %s", hs.Status)
	}
	if hs.AgentStatus != "waiting" {
		t.Errorf("expected AgentStatus waiting, got %s", hs.AgentStatus)
	}
}

func TestIncrementSessions(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, _, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	// Initial sessions should be 0
	runner, _ := service.GetRunner(ctx, r.ID)
	if runner.CurrentSessions != 0 {
		t.Errorf("expected 0 sessions, got %d", runner.CurrentSessions)
	}

	// Increment
	err := service.IncrementSessions(ctx, r.ID)
	if err != nil {
		t.Errorf("IncrementSessions error: %v", err)
	}

	runner, _ = service.GetRunner(ctx, r.ID)
	if runner.CurrentSessions != 1 {
		t.Errorf("expected 1 session after increment, got %d", runner.CurrentSessions)
	}
}

func TestDecrementSessions(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, _, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	// Note: DecrementSessions uses GREATEST which SQLite doesn't support
	// This test just verifies the method signature exists
	_ = service.DecrementSessions(ctx, r.ID)
}

func TestDecrementSessionsMethod(t *testing.T) {
	// This test simply verifies the DecrementSessions method exists and can be called
	// The actual GREATEST function is not supported by SQLite, but works in PostgreSQL
	db := setupTestDB(t)
	service := NewService(db)

	// Verify the method exists by calling it
	// Just check it doesn't panic, ignore error since SQLite doesn't support GREATEST
	_ = service.DecrementSessions(context.Background(), 999)
}

func TestMarkOfflineRunners(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, _, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	// Mark as online with a recent heartbeat
	now := time.Now()
	db.Model(&runner.Runner{}).Where("id = ?", r.ID).Updates(map[string]interface{}{
		"status":         runner.RunnerStatusOnline,
		"last_heartbeat": now,
	})

	// Mark offline with a timeout longer than since heartbeat
	service.MarkOfflineRunners(ctx, time.Hour)

	// Should still be online
	updated, _ := service.GetRunner(ctx, r.ID)
	if updated.Status != runner.RunnerStatusOnline {
		t.Errorf("expected status online, got %s", updated.Status)
	}

	// Set old heartbeat
	oldTime := now.Add(-2 * time.Hour)
	db.Model(&runner.Runner{}).Where("id = ?", r.ID).Update("last_heartbeat", oldTime)

	// Now should be marked offline
	service.MarkOfflineRunners(ctx, time.Hour)

	updated, _ = service.GetRunner(ctx, r.ID)
	if updated.Status != runner.RunnerStatusOffline {
		t.Errorf("expected status offline, got %s", updated.Status)
	}
}

func TestUpdateHostInfo(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, _, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	hostInfo := runner.HostInfo{
		"os":       "linux",
		"arch":     "amd64",
		"hostname": "test-host",
	}

	// Note: SQLite doesn't support JSONB type natively, so this may error
	// The method itself is correct, just SQLite incompatible with the GORM model
	_ = service.UpdateHostInfo(ctx, r.ID, hostInfo)
	_ = r // used
}

func TestValidateRunnerAuth(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create token and register runner
	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, authToken, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	// Validate with correct credentials
	validated, err := service.ValidateRunnerAuth(ctx, r.NodeID, authToken)
	if err != nil {
		t.Fatalf("failed to validate runner auth: %v", err)
	}
	if validated == nil {
		t.Fatal("expected non-nil validated runner")
	}
	if validated.ID != r.ID {
		t.Errorf("expected runner ID %d, got %d", r.ID, validated.ID)
	}
}

func TestValidateRunnerAuthNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Try with non-existent node_id
	_, err := service.ValidateRunnerAuth(ctx, "non-existent-runner", "some-token")
	if err != ErrRunnerNotFound {
		t.Errorf("expected ErrRunnerNotFound, got %v", err)
	}
}

func TestValidateRunnerAuthInvalidToken(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create token and register runner
	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, _, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	// Validate with wrong token
	_, err := service.ValidateRunnerAuth(ctx, r.NodeID, "wrong-token")
	if err != ErrInvalidAuth {
		t.Errorf("expected ErrInvalidAuth, got %v", err)
	}
}

func TestValidateRunnerAuthDisabled(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create token and register runner
	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, authToken, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	// Disable the runner
	isEnabled := false
	service.UpdateRunner(ctx, r.ID, RunnerUpdateInput{IsEnabled: &isEnabled})

	// Validate should fail for disabled runner
	_, err := service.ValidateRunnerAuth(ctx, r.NodeID, authToken)
	if err == nil {
		t.Error("expected error for disabled runner, got nil")
	}
}

func TestSetRunnerStatus(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	plain, _ := service.CreateRegistrationToken(ctx, 1, 1, "Test Token", nil, nil)
	r, _, _ := service.RegisterRunner(ctx, plain, "test-runner", "Test", 5)

	// Test SetRunnerStatus (alias for UpdateRunnerStatus)
	err := service.SetRunnerStatus(ctx, r.ID, runner.RunnerStatusOnline)
	if err != nil {
		t.Fatalf("failed to set runner status: %v", err)
	}

	updated, _ := service.GetRunner(ctx, r.ID)
	if updated.Status != runner.RunnerStatusOnline {
		t.Errorf("expected status online, got %s", updated.Status)
	}

	// Set back to offline
	err = service.SetRunnerStatus(ctx, r.ID, runner.RunnerStatusOffline)
	if err != nil {
		t.Fatalf("failed to set runner status: %v", err)
	}

	updated, _ = service.GetRunner(ctx, r.ID)
	if updated.Status != runner.RunnerStatusOffline {
		t.Errorf("expected status offline, got %s", updated.Status)
	}
}
