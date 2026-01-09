package runner

import (
	"context"
	"testing"

	"github.com/anthropics/agentmesh/runner/internal/config"
	"github.com/anthropics/agentmesh/runner/internal/workspace"
)

// Note: Worktree functionality is now handled by sandbox plugins (WorktreePlugin)
// Tests for worktree are in internal/sandbox/plugins/worktree_test.go

func TestSessionBuilderBuildWithEmptySessionKey(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: "/tmp",
		},
	}

	builder := NewSessionBuilder(runner)
	// Don't set session key

	_, err := builder.Build(context.Background())
	if err == nil {
		t.Error("expected error for empty session key")
	}
	if !contains(err.Error(), "session key is required") {
		t.Errorf("error = %v, want containing 'session key is required'", err)
	}
}

func TestSessionBuilderBuildWithAllOptions(t *testing.T) {
	tempDir := t.TempDir()
	ws, err := workspace.NewManager(tempDir, "")
	if err != nil {
		t.Skipf("Could not create workspace manager: %v", err)
	}

	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: tempDir,
			AgentEnvVars: map[string]string{
				"CONFIG_VAR": "config_value",
			},
		},
		workspace: ws,
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("all-options-session").
		WithAgentType("claude-code").
		WithLaunchCommand("echo", []string{"hello", "world"}).
		WithEnvVars(map[string]string{"VAR1": "value1"}).
		WithEnvVar("VAR2", "value2").
		WithTerminalSize(30, 100).
		WithInitialPrompt("Hello!")

	session, err := builder.Build(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if session.SessionKey != "all-options-session" {
		t.Errorf("session key = %s, want all-options-session", session.SessionKey)
	}
	if session.AgentType != "claude-code" {
		t.Errorf("agent type = %s, want claude-code", session.AgentType)
	}
	if session.InitialPrompt != "Hello!" {
		t.Errorf("initial prompt = %s, want Hello!", session.InitialPrompt)
	}
	if session.Terminal == nil {
		t.Error("terminal should not be nil")
	} else {
		session.Terminal.Stop()
	}
}

func TestSessionBuilderMergeEnvVarsWithNilConfig(t *testing.T) {
	runner := &Runner{
		cfg: nil,
	}

	builder := NewSessionBuilder(runner).
		WithEnvVar("SESSION_VAR", "session_value")

	result := builder.mergeEnvVars()

	if len(result) != 1 {
		t.Errorf("result length = %d, want 1", len(result))
	}
	if result["SESSION_VAR"] != "session_value" {
		t.Errorf("SESSION_VAR = %s, want session_value", result["SESSION_VAR"])
	}
}

func TestSessionBuilderMergeEnvVarsOverride(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			AgentEnvVars: map[string]string{
				"SHARED_VAR": "config_value",
				"CONFIG_VAR": "config_only",
			},
		},
	}

	builder := NewSessionBuilder(runner).
		WithEnvVar("SHARED_VAR", "session_value").
		WithEnvVar("SESSION_VAR", "session_only")

	result := builder.mergeEnvVars()

	// Session should override config
	if result["SHARED_VAR"] != "session_value" {
		t.Errorf("SHARED_VAR = %s, want session_value", result["SHARED_VAR"])
	}
	if result["CONFIG_VAR"] != "config_only" {
		t.Errorf("CONFIG_VAR = %s, want config_only", result["CONFIG_VAR"])
	}
	if result["SESSION_VAR"] != "session_only" {
		t.Errorf("SESSION_VAR = %s, want session_only", result["SESSION_VAR"])
	}
}

func TestSessionBuilderTerminalSizeDefaults(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: "/tmp",
		},
	}

	builder := NewSessionBuilder(runner).
		WithTerminalSize(0, 0) // Zero values should use defaults

	if builder.rows != 24 {
		t.Errorf("rows = %d, want 24 (default)", builder.rows)
	}
	if builder.cols != 80 {
		t.Errorf("cols = %d, want 80 (default)", builder.cols)
	}
}

func TestSessionBuilderResolveFallbackToConfigRoot(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: "/fallback/path",
		},
		workspace: nil,
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("fallback-session")

	workDir, worktreePath, branchName, err := builder.resolveWorkingDirectory(context.Background())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if workDir != "/fallback/path" {
		t.Errorf("workDir = %s, want /fallback/path", workDir)
	}
	if worktreePath != "" {
		t.Errorf("worktreePath should be empty, got %s", worktreePath)
	}
	if branchName != "" {
		t.Errorf("branchName should be empty, got %s", branchName)
	}
}

func TestSessionBuilderRunPreparationWithEmptyScript(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: "/tmp",
		},
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("empty-prep-session").
		WithPreparationScript("", 0)

	err := builder.runPreparation(context.Background(), "/tmp", "", "")
	if err != nil {
		t.Errorf("unexpected error for empty prep script: %v", err)
	}
}

func TestSessionBuilderRunPreparationWithEchoScript(t *testing.T) {
	tempDir := t.TempDir()

	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: tempDir,
		},
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("prep-script-session").
		WithPreparationScript("echo 'test preparation'", 10)

	err := builder.runPreparation(context.Background(), tempDir, "", "")
	if err != nil {
		t.Logf("Preparation script result: %v", err)
	}
}

func TestSessionBuilderFluentChaining(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: "/tmp",
		},
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("chain-session").
		WithAgentType("test-agent").
		WithLaunchCommand("test", []string{"arg1"}).
		WithEnvVars(map[string]string{"VAR1": "val1"}).
		WithEnvVar("VAR2", "val2").
		WithTerminalSize(40, 120).
		WithInitialPrompt("prompt").
		WithRepository("https://example.com/repo", "develop").
		WithWorktree("TICKET-789").
		WithPreparationScript("echo test", 30).
		WithMCP("server1")

	if builder.sessionKey != "chain-session" {
		t.Error("sessionKey not set")
	}
	if builder.agentType != "test-agent" {
		t.Error("agentType not set")
	}
	if builder.launchCommand != "test" {
		t.Error("launchCommand not set")
	}
	if len(builder.launchArgs) != 1 || builder.launchArgs[0] != "arg1" {
		t.Error("launchArgs not set correctly")
	}
	if builder.envVars["VAR1"] != "val1" || builder.envVars["VAR2"] != "val2" {
		t.Error("envVars not set correctly")
	}
	if builder.rows != 40 || builder.cols != 120 {
		t.Error("terminal size not set correctly")
	}
	if builder.initialPrompt != "prompt" {
		t.Error("initialPrompt not set")
	}
	if builder.repositoryURL != "https://example.com/repo" || builder.branch != "develop" {
		t.Error("repository not set correctly")
	}
	if !builder.useWorktree || builder.ticketIdentifier != "TICKET-789" {
		t.Error("worktree not set correctly")
	}
	if builder.prepScript != "echo test" || builder.prepTimeout != 30 {
		t.Error("preparation script not set correctly")
	}
	if !builder.mcpEnabled || len(builder.mcpServers) != 1 {
		t.Error("MCP not set correctly")
	}
}
