package runner

import (
	"context"
	"testing"

	"github.com/anthropics/agentmesh/runner/internal/config"
)

// --- Tests for SessionBuilder ---

func TestNewSessionBuilder(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{},
	}

	builder := NewSessionBuilder(runner)

	if builder == nil {
		t.Fatal("NewSessionBuilder returned nil")
	}

	if builder.runner != runner {
		t.Error("runner should be set")
	}

	if builder.envVars == nil {
		t.Error("envVars should be initialized")
	}

	if builder.rows != 24 {
		t.Errorf("rows default = %d, want 24", builder.rows)
	}

	if builder.cols != 80 {
		t.Errorf("cols default = %d, want 80", builder.cols)
	}
}

func TestSessionBuilderWithSessionKey(t *testing.T) {
	runner := &Runner{cfg: &config.Config{}}
	builder := NewSessionBuilder(runner).WithSessionKey("test-key")

	if builder.sessionKey != "test-key" {
		t.Errorf("sessionKey = %v, want test-key", builder.sessionKey)
	}
}

func TestSessionBuilderWithAgentType(t *testing.T) {
	runner := &Runner{cfg: &config.Config{}}
	builder := NewSessionBuilder(runner).WithAgentType("claude-code")

	if builder.agentType != "claude-code" {
		t.Errorf("agentType = %v, want claude-code", builder.agentType)
	}
}

func TestSessionBuilderWithLaunchCommand(t *testing.T) {
	runner := &Runner{cfg: &config.Config{}}
	builder := NewSessionBuilder(runner).WithLaunchCommand("claude", []string{"--headless"})

	if builder.launchCommand != "claude" {
		t.Errorf("launchCommand = %v, want claude", builder.launchCommand)
	}

	if len(builder.launchArgs) != 1 || builder.launchArgs[0] != "--headless" {
		t.Errorf("launchArgs = %v, want [--headless]", builder.launchArgs)
	}
}

func TestSessionBuilderWithEnvVars(t *testing.T) {
	runner := &Runner{cfg: &config.Config{}}
	builder := NewSessionBuilder(runner).WithEnvVars(map[string]string{
		"VAR1": "value1",
		"VAR2": "value2",
	})

	if builder.envVars["VAR1"] != "value1" {
		t.Errorf("VAR1 = %v, want value1", builder.envVars["VAR1"])
	}

	if builder.envVars["VAR2"] != "value2" {
		t.Errorf("VAR2 = %v, want value2", builder.envVars["VAR2"])
	}
}

func TestSessionBuilderWithEnvVar(t *testing.T) {
	runner := &Runner{cfg: &config.Config{}}
	builder := NewSessionBuilder(runner).
		WithEnvVar("KEY1", "VALUE1").
		WithEnvVar("KEY2", "VALUE2")

	if builder.envVars["KEY1"] != "VALUE1" {
		t.Errorf("KEY1 = %v, want VALUE1", builder.envVars["KEY1"])
	}

	if builder.envVars["KEY2"] != "VALUE2" {
		t.Errorf("KEY2 = %v, want VALUE2", builder.envVars["KEY2"])
	}
}

func TestSessionBuilderWithTerminalSize(t *testing.T) {
	runner := &Runner{cfg: &config.Config{}}
	builder := NewSessionBuilder(runner).WithTerminalSize(48, 160)

	if builder.rows != 48 {
		t.Errorf("rows = %d, want 48", builder.rows)
	}

	if builder.cols != 160 {
		t.Errorf("cols = %d, want 160", builder.cols)
	}
}

func TestSessionBuilderWithTerminalSizeZeroValues(t *testing.T) {
	runner := &Runner{cfg: &config.Config{}}
	builder := NewSessionBuilder(runner).WithTerminalSize(0, 0)

	// Should keep defaults
	if builder.rows != 24 {
		t.Errorf("rows = %d, want 24 (default)", builder.rows)
	}

	if builder.cols != 80 {
		t.Errorf("cols = %d, want 80 (default)", builder.cols)
	}
}

func TestSessionBuilderWithInitialPrompt(t *testing.T) {
	runner := &Runner{cfg: &config.Config{}}
	builder := NewSessionBuilder(runner).WithInitialPrompt("Hello, Claude!")

	if builder.initialPrompt != "Hello, Claude!" {
		t.Errorf("initialPrompt = %v, want Hello, Claude!", builder.initialPrompt)
	}
}

func TestSessionBuilderWithRepository(t *testing.T) {
	runner := &Runner{cfg: &config.Config{}}
	builder := NewSessionBuilder(runner).WithRepository(
		"https://github.com/test/repo.git",
		"feature/test",
	)

	if builder.repositoryURL != "https://github.com/test/repo.git" {
		t.Errorf("repositoryURL = %v, want https://github.com/test/repo.git", builder.repositoryURL)
	}

	if builder.branch != "feature/test" {
		t.Errorf("branch = %v, want feature/test", builder.branch)
	}
}

func TestSessionBuilderWithWorktree(t *testing.T) {
	runner := &Runner{cfg: &config.Config{}}
	builder := NewSessionBuilder(runner).WithWorktree("TICKET-123")

	if builder.ticketIdentifier != "TICKET-123" {
		t.Errorf("ticketIdentifier = %v, want TICKET-123", builder.ticketIdentifier)
	}

	if !builder.useWorktree {
		t.Error("useWorktree should be true")
	}
}

func TestSessionBuilderWithPreparationScript(t *testing.T) {
	runner := &Runner{cfg: &config.Config{}}
	builder := NewSessionBuilder(runner).WithPreparationScript("npm install", 300)

	if builder.prepScript != "npm install" {
		t.Errorf("prepScript = %v, want npm install", builder.prepScript)
	}

	if builder.prepTimeout != 300 {
		t.Errorf("prepTimeout = %d, want 300", builder.prepTimeout)
	}
}

func TestSessionBuilderWithMCP(t *testing.T) {
	runner := &Runner{cfg: &config.Config{}}
	builder := NewSessionBuilder(runner).WithMCP("server1", "server2")

	if !builder.mcpEnabled {
		t.Error("mcpEnabled should be true")
	}

	if len(builder.mcpServers) != 2 {
		t.Errorf("mcpServers length = %d, want 2", len(builder.mcpServers))
	}

	if builder.mcpServers[0] != "server1" {
		t.Errorf("mcpServers[0] = %v, want server1", builder.mcpServers[0])
	}
}

func TestSessionBuilderChaining(t *testing.T) {
	runner := &Runner{cfg: &config.Config{}}
	builder := NewSessionBuilder(runner).
		WithSessionKey("session-1").
		WithAgentType("claude-code").
		WithLaunchCommand("claude", []string{"--headless"}).
		WithEnvVar("API_KEY", "secret").
		WithTerminalSize(48, 160).
		WithInitialPrompt("Hello!").
		WithRepository("https://github.com/test/repo.git", "main").
		WithWorktree("TICKET-123").
		WithPreparationScript("npm install", 300).
		WithMCP("server1")

	if builder.sessionKey != "session-1" {
		t.Errorf("sessionKey = %v, want session-1", builder.sessionKey)
	}

	if builder.agentType != "claude-code" {
		t.Errorf("agentType = %v, want claude-code", builder.agentType)
	}

	if builder.launchCommand != "claude" {
		t.Errorf("launchCommand = %v, want claude", builder.launchCommand)
	}

	if builder.rows != 48 {
		t.Errorf("rows = %d, want 48", builder.rows)
	}

	if builder.initialPrompt != "Hello!" {
		t.Errorf("initialPrompt = %v, want Hello!", builder.initialPrompt)
	}

	if builder.ticketIdentifier != "TICKET-123" {
		t.Errorf("ticketIdentifier = %v, want TICKET-123", builder.ticketIdentifier)
	}

	if builder.prepScript != "npm install" {
		t.Errorf("prepScript = %v, want npm install", builder.prepScript)
	}

	if !builder.mcpEnabled {
		t.Error("mcpEnabled should be true")
	}
}

func TestSessionBuilderBuildEmptySessionKey(t *testing.T) {
	runner := &Runner{cfg: &config.Config{}}
	builder := NewSessionBuilder(runner)

	ctx := context.Background()
	_, err := builder.Build(ctx)

	if err == nil {
		t.Error("expected error for empty session key")
	}

	if !contains(err.Error(), "session key is required") {
		t.Errorf("error = %v, want containing 'session key is required'", err)
	}
}

// Note: TestSessionBuilderMergeEnvVars, TestSessionBuilderMergeEnvVarsNilConfig,
// TestSessionBuilderResolveWorkingDirectoryFallback, and TestExtendedSessionStruct
// are defined in session_builder_test.go
