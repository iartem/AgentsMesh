package runner

import (
	"testing"

	"github.com/anthropics/agentmesh/runner/internal/client"
	"github.com/anthropics/agentmesh/runner/internal/config"
)

// Basic unit tests for RunnerMessageHandler creation and interface

func TestNewRunnerMessageHandler(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()
	cfg := &config.Config{WorkspaceRoot: t.TempDir()}
	runner := &Runner{cfg: cfg}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	if handler == nil {
		t.Fatal("handler should not be nil")
	}
	if handler.runner != runner {
		t.Error("handler.runner mismatch")
	}
	if handler.sessionStore != store {
		t.Error("handler.sessionStore mismatch")
	}
	if handler.conn != mockConn {
		t.Error("handler.conn mismatch")
	}
}

func TestRunnerMessageHandlerImplementsInterface(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()
	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, mockConn)

	// Verify it implements client.MessageHandler
	var _ client.MessageHandler = handler
}

func TestRunnerMessageHandlerWithNilRunner(t *testing.T) {
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	// Should not panic with nil runner
	handler := NewRunnerMessageHandler(nil, store, mockConn)

	if handler == nil {
		t.Fatal("handler should not be nil even with nil runner")
	}
	if handler.runner != nil {
		t.Error("handler.runner should be nil")
	}
}

func TestRunnerMessageHandlerWithNilStore(t *testing.T) {
	mockConn := client.NewMockConnection()
	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, nil, mockConn)

	if handler == nil {
		t.Fatal("handler should not be nil even with nil store")
	}
	if handler.sessionStore != nil {
		t.Error("handler.sessionStore should be nil")
	}
}

func TestRunnerMessageHandlerWithNilConnection(t *testing.T) {
	store := NewInMemorySessionStore()
	runner := &Runner{cfg: &config.Config{}}

	handler := NewRunnerMessageHandler(runner, store, nil)

	if handler == nil {
		t.Fatal("handler should not be nil even with nil connection")
	}
	if handler.conn != nil {
		t.Error("handler.conn should be nil")
	}
}

// --- Tests for buildPluginConfig ---

func TestBuildPluginConfigEmpty(t *testing.T) {
	store := NewInMemorySessionStore()
	runner := &Runner{cfg: &config.Config{}}
	handler := NewRunnerMessageHandler(runner, store, nil)

	req := &client.CreateSessionRequest{
		SessionID: "test-session",
	}

	config := handler.buildPluginConfig(req)

	if config == nil {
		t.Fatal("config should not be nil")
	}
	if len(config) != 0 {
		t.Errorf("config should be empty, got %v", config)
	}
}

func TestBuildPluginConfigWithLegacyFields(t *testing.T) {
	store := NewInMemorySessionStore()
	runner := &Runner{cfg: &config.Config{}}
	handler := NewRunnerMessageHandler(runner, store, nil)

	req := &client.CreateSessionRequest{
		SessionID:        "test-session",
		TicketIdentifier: "TICKET-123",
		WorkingDir:       "/workspace/project",
		EnvVars: map[string]string{
			"ENV_VAR1": "value1",
			"ENV_VAR2": "value2",
		},
		PreparationConfig: &client.PreparationConfig{
			Script:         "npm install",
			TimeoutSeconds: 120,
		},
	}

	config := handler.buildPluginConfig(req)

	// Check ticket_identifier
	if config["ticket_identifier"] != "TICKET-123" {
		t.Errorf("ticket_identifier = %v, want TICKET-123", config["ticket_identifier"])
	}

	// Check working_dir
	if config["working_dir"] != "/workspace/project" {
		t.Errorf("working_dir = %v, want /workspace/project", config["working_dir"])
	}

	// Check env_vars
	envVars, ok := config["env_vars"].(map[string]interface{})
	if !ok {
		t.Fatal("env_vars should be map[string]interface{}")
	}
	if envVars["ENV_VAR1"] != "value1" {
		t.Errorf("ENV_VAR1 = %v, want value1", envVars["ENV_VAR1"])
	}
	if envVars["ENV_VAR2"] != "value2" {
		t.Errorf("ENV_VAR2 = %v, want value2", envVars["ENV_VAR2"])
	}

	// Check init_script and init_timeout
	if config["init_script"] != "npm install" {
		t.Errorf("init_script = %v, want npm install", config["init_script"])
	}
	if config["init_timeout"] != 120 {
		t.Errorf("init_timeout = %v, want 120", config["init_timeout"])
	}
}

func TestBuildPluginConfigWithPluginConfig(t *testing.T) {
	store := NewInMemorySessionStore()
	runner := &Runner{cfg: &config.Config{}}
	handler := NewRunnerMessageHandler(runner, store, nil)

	req := &client.CreateSessionRequest{
		SessionID: "test-session",
		PluginConfig: map[string]interface{}{
			"repository_url": "https://github.com/org/repo.git",
			"branch":         "develop",
			"git_token":      "ghp_xxx",
		},
	}

	config := handler.buildPluginConfig(req)

	if config["repository_url"] != "https://github.com/org/repo.git" {
		t.Errorf("repository_url = %v, want https://github.com/org/repo.git", config["repository_url"])
	}
	if config["branch"] != "develop" {
		t.Errorf("branch = %v, want develop", config["branch"])
	}
	if config["git_token"] != "ghp_xxx" {
		t.Errorf("git_token = %v, want ghp_xxx", config["git_token"])
	}
}

func TestBuildPluginConfigMergeOverride(t *testing.T) {
	store := NewInMemorySessionStore()
	runner := &Runner{cfg: &config.Config{}}
	handler := NewRunnerMessageHandler(runner, store, nil)

	// PluginConfig should override legacy fields
	req := &client.CreateSessionRequest{
		SessionID:        "test-session",
		TicketIdentifier: "TICKET-123",  // Legacy field
		PluginConfig: map[string]interface{}{
			"ticket_identifier": "TICKET-456",  // Override via PluginConfig
			"extra_field":       "extra_value",
		},
	}

	config := handler.buildPluginConfig(req)

	// PluginConfig should override legacy field
	if config["ticket_identifier"] != "TICKET-456" {
		t.Errorf("ticket_identifier = %v, want TICKET-456 (from PluginConfig)", config["ticket_identifier"])
	}

	// Extra field from PluginConfig should be present
	if config["extra_field"] != "extra_value" {
		t.Errorf("extra_field = %v, want extra_value", config["extra_field"])
	}
}

func TestBuildPluginConfigPartialPreparation(t *testing.T) {
	store := NewInMemorySessionStore()
	runner := &Runner{cfg: &config.Config{}}
	handler := NewRunnerMessageHandler(runner, store, nil)

	// Test with only script, no timeout
	req := &client.CreateSessionRequest{
		SessionID: "test-session",
		PreparationConfig: &client.PreparationConfig{
			Script: "echo hello",
			// TimeoutSeconds is 0
		},
	}

	config := handler.buildPluginConfig(req)

	if config["init_script"] != "echo hello" {
		t.Errorf("init_script = %v, want echo hello", config["init_script"])
	}

	// init_timeout should not be present when TimeoutSeconds is 0
	if _, exists := config["init_timeout"]; exists {
		t.Error("init_timeout should not be set when TimeoutSeconds is 0")
	}
}
