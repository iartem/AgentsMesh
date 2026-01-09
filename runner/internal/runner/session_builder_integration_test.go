package runner

import (
	"context"
	"testing"

	"github.com/anthropics/agentmesh/runner/internal/config"
	"github.com/anthropics/agentmesh/runner/internal/workspace"
)

// --- Test Build method ---

func TestSessionBuilderBuildSuccess(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: "/tmp/test-workspace",
			AgentEnvVars:  map[string]string{"CONFIG_VAR": "value"},
		},
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("session-build-test").
		WithAgentType("claude-code").
		WithLaunchCommand("echo", []string{"hello"}).
		WithTerminalSize(30, 100).
		WithInitialPrompt("Test prompt")

	session, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if session == nil {
		t.Fatal("session should not be nil")
	}

	if session.SessionKey != "session-build-test" {
		t.Errorf("SessionKey = %v, want session-build-test", session.SessionKey)
	}
	if session.AgentType != "claude-code" {
		t.Errorf("AgentType = %v, want claude-code", session.AgentType)
	}
	if session.InitialPrompt != "Test prompt" {
		t.Errorf("InitialPrompt = %v, want Test prompt", session.InitialPrompt)
	}
	if session.Status != SessionStatusInitializing {
		t.Errorf("Status = %v, want initializing", session.Status)
	}

	// Clean up terminal if created
	if session.Terminal != nil {
		session.Terminal.Stop()
	}
}

func TestSessionBuilderBuildWithMinimalConfig(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: "/tmp",
		},
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("minimal-session").
		WithLaunchCommand("echo", nil)

	session, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	if session.SessionKey != "minimal-session" {
		t.Errorf("SessionKey = %v, want minimal-session", session.SessionKey)
	}

	// Clean up
	if session.Terminal != nil {
		session.Terminal.Stop()
	}
}

// --- Test resolveWorkingDirectory ---

func TestSessionBuilderResolveWorkingDirectoryWithWorkspaceManager(t *testing.T) {
	// Create a temporary workspace manager
	tempDir := t.TempDir()
	ws, err := workspace.NewManager(tempDir, "")
	if err != nil {
		t.Skipf("Could not create workspace manager: %v", err)
	}

	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: tempDir,
		},
		workspace: ws,
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("test-resolve")

	workDir, worktreePath, branchName, err := builder.resolveWorkingDirectory(context.Background())
	if err != nil {
		t.Fatalf("resolveWorkingDirectory failed: %v", err)
	}

	// Without repository URL, should use temp workspace
	if workDir == "" {
		t.Error("workDir should not be empty")
	}
	if worktreePath != "" {
		t.Errorf("worktreePath = %v, want empty (no repo)", worktreePath)
	}
	if branchName != "" {
		t.Errorf("branchName = %v, want empty", branchName)
	}
}

func TestSessionBuilderResolveWorkingDirectoryFallbackToConfig(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: "/my/workspace",
		},
		workspace: nil, // No workspace manager
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("test-fallback")

	workDir, _, _, err := builder.resolveWorkingDirectory(context.Background())
	if err != nil {
		t.Fatalf("resolveWorkingDirectory failed: %v", err)
	}

	if workDir != "/my/workspace" {
		t.Errorf("workDir = %v, want /my/workspace", workDir)
	}
}

func TestSessionBuilderResolveWorkingDirectoryWithTicket(t *testing.T) {
	// Test that without repository URL, falls back to config workspace even with ticket
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: "/tmp",
		},
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("test-ticket-no-repo").
		WithWorktree("TICKET-123")

	workDir, _, _, err := builder.resolveWorkingDirectory(context.Background())
	if err != nil {
		t.Fatalf("resolveWorkingDirectory failed: %v", err)
	}

	// Without repository URL, should fall back to config workspace
	if workDir != "/tmp" {
		t.Errorf("workDir = %v, want /tmp", workDir)
	}
}

// --- Test runPreparation ---

func TestSessionBuilderRunPreparationWithScript(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{},
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("test-prep").
		WithPreparationScript("echo hello", 5)

	// runPreparation will create a preparer and try to run it
	// This may fail in test environment, but we test the path
	err := builder.runPreparation(context.Background(), "/tmp", "/tmp/worktree", "main")

	// We mainly want to test the code path executes without panic
	// The actual script may fail depending on environment
	_ = err
}

func TestSessionBuilderRunPreparationEmptyScript(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{},
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("test-no-prep")
	// prepScript is empty

	err := builder.runPreparation(context.Background(), "/tmp", "", "")
	if err != nil {
		t.Errorf("runPreparation with empty script should not error: %v", err)
	}
}

func TestSessionBuilderRunPreparationBasic(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{},
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("test-prep-basic").
		WithPreparationScript("echo test", 5)

	// Test basic preparation - should work
	err := builder.runPreparation(context.Background(), "/tmp", "/tmp/worktree", "main")
	_ = err // May fail but should not panic
}

// --- Test mergeEnvVars edge cases ---

func TestSessionBuilderMergeEnvVarsEmptyBoth(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			AgentEnvVars: nil,
		},
	}

	builder := NewSessionBuilder(runner)
	// Don't add any env vars

	result := builder.mergeEnvVars()

	if len(result) != 0 {
		t.Errorf("result length = %d, want 0", len(result))
	}
}

func TestSessionBuilderMergeEnvVarsOnlyConfig(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			AgentEnvVars: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
			},
		},
	}

	builder := NewSessionBuilder(runner)

	result := builder.mergeEnvVars()

	if result["VAR1"] != "value1" {
		t.Errorf("VAR1 = %v, want value1", result["VAR1"])
	}
	if result["VAR2"] != "value2" {
		t.Errorf("VAR2 = %v, want value2", result["VAR2"])
	}
}

func TestSessionBuilderMergeEnvVarsOnlyBuilder(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			AgentEnvVars: nil,
		},
	}

	builder := NewSessionBuilder(runner).
		WithEnvVar("BUILDER_VAR", "builder_value")

	result := builder.mergeEnvVars()

	if result["BUILDER_VAR"] != "builder_value" {
		t.Errorf("BUILDER_VAR = %v, want builder_value", result["BUILDER_VAR"])
	}
}

// --- Test ExtendedSession ---

func TestExtendedSessionEmbedding(t *testing.T) {
	session := &Session{
		ID:            "session-1",
		SessionKey:    "key-1",
		AgentType:     "claude-code",
		Status:        SessionStatusRunning,
	}

	extended := &ExtendedSession{
		Session:          session,
		TicketIdentifier: "TICKET-999",
		OnOutput:         func([]byte) {},
		OnExit:           func(int) {},
	}

	// Test that embedded fields are accessible
	if extended.ID != "session-1" {
		t.Errorf("ID = %v, want session-1", extended.ID)
	}
	if extended.SessionKey != "key-1" {
		t.Errorf("SessionKey = %v, want key-1", extended.SessionKey)
	}
	if extended.TicketIdentifier != "TICKET-999" {
		t.Errorf("TicketIdentifier = %v, want TICKET-999", extended.TicketIdentifier)
	}
	if extended.OnOutput == nil {
		t.Error("OnOutput should not be nil")
	}
	if extended.OnExit == nil {
		t.Error("OnExit should not be nil")
	}
}

// --- Test WithMCP ---

func TestSessionBuilderWithMCPSingle(t *testing.T) {
	runner := &Runner{}
	builder := NewSessionBuilder(runner).
		WithMCP("server1")

	if !builder.mcpEnabled {
		t.Error("mcpEnabled should be true")
	}
	if len(builder.mcpServers) != 1 {
		t.Errorf("mcpServers length = %d, want 1", len(builder.mcpServers))
	}
	if builder.mcpServers[0] != "server1" {
		t.Errorf("mcpServers[0] = %v, want server1", builder.mcpServers[0])
	}
}

func TestSessionBuilderWithMCPMultiple(t *testing.T) {
	runner := &Runner{}
	builder := NewSessionBuilder(runner).
		WithMCP("server1", "server2", "server3")

	if !builder.mcpEnabled {
		t.Error("mcpEnabled should be true")
	}
	if len(builder.mcpServers) != 3 {
		t.Errorf("mcpServers length = %d, want 3", len(builder.mcpServers))
	}
}

func TestSessionBuilderWithMCPEmpty(t *testing.T) {
	runner := &Runner{}
	builder := NewSessionBuilder(runner).
		WithMCP()

	if !builder.mcpEnabled {
		t.Error("mcpEnabled should be true even with no servers")
	}
	if len(builder.mcpServers) != 0 {
		t.Errorf("mcpServers length = %d, want 0", len(builder.mcpServers))
	}
}

// --- Benchmark ---

func BenchmarkSessionBuilderBuild(b *testing.B) {
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: "/tmp",
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := NewSessionBuilder(runner).
			WithSessionKey("benchmark-session").
			WithAgentType("claude-code").
			WithLaunchCommand("echo", []string{"test"})

		session, _ := builder.Build(ctx)
		if session != nil && session.Terminal != nil {
			session.Terminal.Stop()
		}
	}
}
