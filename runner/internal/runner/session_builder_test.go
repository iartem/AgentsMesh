package runner

import (
	"context"
	"testing"

	"github.com/anthropics/agentmesh/runner/internal/config"
	"github.com/anthropics/agentmesh/runner/internal/workspace"
)

func TestSessionBuilderStruct(t *testing.T) {
	builder := &SessionBuilder{
		sessionKey:       "session-1",
		agentType:        "claude-code",
		launchCommand:    "claude",
		launchArgs:       []string{"--headless"},
		envVars:          map[string]string{"KEY": "VALUE"},
		rows:             24,
		cols:             80,
		initialPrompt:    "Hello",
		repositoryURL:    "https://github.com/test/repo.git",
		branch:           "main",
		ticketIdentifier: "TICKET-123",
		useWorktree:      true,
		prepScript:       "npm install",
		prepTimeout:      300,
		mcpEnabled:       true,
		mcpServers:       []string{"server1"},
	}

	if builder.sessionKey != "session-1" {
		t.Errorf("sessionKey: got %v, want session-1", builder.sessionKey)
	}

	if builder.rows != 24 {
		t.Errorf("rows: got %v, want 24", builder.rows)
	}
}

func TestSessionBuilderFluentAPI(t *testing.T) {
	runner := &Runner{}
	builder := NewSessionBuilder(runner)

	// Test fluent API chain
	result := builder.
		WithSessionKey("session-1").
		WithAgentType("claude-code").
		WithLaunchCommand("claude", []string{"--headless"}).
		WithEnvVars(map[string]string{"KEY1": "VALUE1"}).
		WithEnvVar("KEY2", "VALUE2").
		WithTerminalSize(40, 120).
		WithInitialPrompt("Hello").
		WithRepository("https://github.com/test/repo.git", "main").
		WithWorktree("TICKET-123").
		WithPreparationScript("npm install", 300).
		WithMCP("server1", "server2")

	// Verify it returns the same builder
	if result != builder {
		t.Error("fluent API should return the same builder")
	}

	// Verify values
	if builder.sessionKey != "session-1" {
		t.Errorf("sessionKey: got %v, want session-1", builder.sessionKey)
	}
	if builder.agentType != "claude-code" {
		t.Errorf("agentType: got %v, want claude-code", builder.agentType)
	}
	if builder.launchCommand != "claude" {
		t.Errorf("launchCommand: got %v, want claude", builder.launchCommand)
	}
	if len(builder.launchArgs) != 1 {
		t.Errorf("launchArgs length: got %v, want 1", len(builder.launchArgs))
	}
	if builder.envVars["KEY1"] != "VALUE1" {
		t.Errorf("envVars[KEY1]: got %v, want VALUE1", builder.envVars["KEY1"])
	}
	if builder.envVars["KEY2"] != "VALUE2" {
		t.Errorf("envVars[KEY2]: got %v, want VALUE2", builder.envVars["KEY2"])
	}
	if builder.rows != 40 {
		t.Errorf("rows: got %v, want 40", builder.rows)
	}
	if builder.cols != 120 {
		t.Errorf("cols: got %v, want 120", builder.cols)
	}
	if builder.initialPrompt != "Hello" {
		t.Errorf("initialPrompt: got %v, want Hello", builder.initialPrompt)
	}
	if builder.repositoryURL != "https://github.com/test/repo.git" {
		t.Errorf("repositoryURL: got %v, want https://github.com/test/repo.git", builder.repositoryURL)
	}
	if builder.branch != "main" {
		t.Errorf("branch: got %v, want main", builder.branch)
	}
	if builder.ticketIdentifier != "TICKET-123" {
		t.Errorf("ticketIdentifier: got %v, want TICKET-123", builder.ticketIdentifier)
	}
	if !builder.useWorktree {
		t.Error("useWorktree should be true")
	}
	if builder.prepScript != "npm install" {
		t.Errorf("prepScript: got %v, want npm install", builder.prepScript)
	}
	if builder.prepTimeout != 300 {
		t.Errorf("prepTimeout: got %v, want 300", builder.prepTimeout)
	}
	if !builder.mcpEnabled {
		t.Error("mcpEnabled should be true")
	}
	if len(builder.mcpServers) != 2 {
		t.Errorf("mcpServers length: got %v, want 2", len(builder.mcpServers))
	}
}

func TestSessionBuilderDefaultValues(t *testing.T) {
	runner := &Runner{}
	builder := NewSessionBuilder(runner)

	if builder.rows != 24 {
		t.Errorf("default rows: got %v, want 24", builder.rows)
	}

	if builder.cols != 80 {
		t.Errorf("default cols: got %v, want 80", builder.cols)
	}

	if builder.envVars == nil {
		t.Error("envVars should be initialized")
	}
}

func TestSessionBuilderTerminalSizeValidation(t *testing.T) {
	runner := &Runner{}
	builder := NewSessionBuilder(runner)

	// Test with invalid values (should use defaults)
	builder.WithTerminalSize(0, 0)

	if builder.rows != 24 {
		t.Errorf("rows with zero: got %v, want 24 (default)", builder.rows)
	}

	if builder.cols != 80 {
		t.Errorf("cols with zero: got %v, want 80 (default)", builder.cols)
	}

	// Test with negative values (should use defaults)
	builder.WithTerminalSize(-1, -1)

	if builder.rows != 24 {
		t.Errorf("rows with negative: got %v, want 24 (default)", builder.rows)
	}
}

func TestSessionBuilderBuildWithoutSessionKey(t *testing.T) {
	runner := &Runner{}
	builder := NewSessionBuilder(runner)

	_, err := builder.Build(context.Background())
	if err == nil {
		t.Error("expected error for missing session key")
	}
}

func TestSessionBuilderMergeEnvVars(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			AgentEnvVars: map[string]string{
				"CONFIG_VAR": "config_value",
				"SHARED_VAR": "config_shared",
			},
		},
	}

	builder := NewSessionBuilder(runner)
	builder.WithEnvVars(map[string]string{
		"BUILDER_VAR": "builder_value",
		"SHARED_VAR":  "builder_shared",
	})

	result := builder.mergeEnvVars()

	if result["CONFIG_VAR"] != "config_value" {
		t.Errorf("CONFIG_VAR: got %v, want config_value", result["CONFIG_VAR"])
	}

	if result["BUILDER_VAR"] != "builder_value" {
		t.Errorf("BUILDER_VAR: got %v, want builder_value", result["BUILDER_VAR"])
	}

	if result["SHARED_VAR"] != "builder_shared" {
		t.Errorf("SHARED_VAR: got %v, want builder_shared (builder should override config)", result["SHARED_VAR"])
	}
}

func TestSessionBuilderMergeEnvVarsNilConfig(t *testing.T) {
	runner := &Runner{
		cfg: nil,
	}

	builder := NewSessionBuilder(runner)
	builder.WithEnvVars(map[string]string{
		"BUILDER_VAR": "builder_value",
	})

	result := builder.mergeEnvVars()

	if result["BUILDER_VAR"] != "builder_value" {
		t.Errorf("BUILDER_VAR: got %v, want builder_value", result["BUILDER_VAR"])
	}
}

func TestSessionBuilderResolveWorkingDirectoryFallback(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: "/default/workspace",
		},
	}

	builder := NewSessionBuilder(runner).WithSessionKey("test")

	workDir, worktreePath, branchName, err := builder.resolveWorkingDirectory(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if workDir != "/default/workspace" {
		t.Errorf("workDir = %v, want /default/workspace", workDir)
	}
	if worktreePath != "" {
		t.Errorf("worktreePath = %v, want empty", worktreePath)
	}
	if branchName != "" {
		t.Errorf("branchName = %v, want empty", branchName)
	}
}

func TestSessionBuilderResolveWorkingDirectoryEmptyCfg(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: "",
		},
	}

	builder := NewSessionBuilder(runner).WithSessionKey("test")

	workDir, _, _, err := builder.resolveWorkingDirectory(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if workDir != "" {
		t.Errorf("workDir = %v, want empty", workDir)
	}
}

func TestSessionBuilderRunPreparationNilPreparer(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{},
	}

	builder := NewSessionBuilder(runner).WithSessionKey("test")
	// prepScript is empty, so preparer will be nil

	err := builder.runPreparation(context.Background(), "/workdir", "/worktree", "main")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestSessionBuilderWithAllOptions(t *testing.T) {
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: "/workspace",
			AgentEnvVars: map[string]string{
				"CONFIG_VAR": "config_value",
			},
		},
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("session-key").
		WithAgentType("claude-code").
		WithLaunchCommand("claude", []string{"--headless"}).
		WithEnvVars(map[string]string{"ENV1": "value1"}).
		WithEnvVar("ENV2", "value2").
		WithTerminalSize(40, 120).
		WithInitialPrompt("Hello").
		WithRepository("https://github.com/test/repo.git", "main").
		WithWorktree("TICKET-123").
		WithPreparationScript("npm install", 300).
		WithMCP("server1", "server2")

	if builder.sessionKey != "session-key" {
		t.Errorf("sessionKey = %v, want session-key", builder.sessionKey)
	}
	if builder.agentType != "claude-code" {
		t.Errorf("agentType = %v, want claude-code", builder.agentType)
	}
	if len(builder.launchArgs) != 1 || builder.launchArgs[0] != "--headless" {
		t.Errorf("launchArgs = %v, want [--headless]", builder.launchArgs)
	}
	if builder.envVars["ENV1"] != "value1" {
		t.Errorf("envVars[ENV1] = %v, want value1", builder.envVars["ENV1"])
	}
	if builder.envVars["ENV2"] != "value2" {
		t.Errorf("envVars[ENV2] = %v, want value2", builder.envVars["ENV2"])
	}
	if builder.rows != 40 || builder.cols != 120 {
		t.Errorf("terminal size = %dx%d, want 40x120", builder.rows, builder.cols)
	}
	if builder.initialPrompt != "Hello" {
		t.Errorf("initialPrompt = %v, want Hello", builder.initialPrompt)
	}
	if builder.repositoryURL != "https://github.com/test/repo.git" {
		t.Errorf("repositoryURL = %v, want https://github.com/test/repo.git", builder.repositoryURL)
	}
	if builder.branch != "main" {
		t.Errorf("branch = %v, want main", builder.branch)
	}
	if builder.ticketIdentifier != "TICKET-123" || !builder.useWorktree {
		t.Error("worktree config not set correctly")
	}
	if builder.prepScript != "npm install" || builder.prepTimeout != 300 {
		t.Error("prep config not set correctly")
	}
	if !builder.mcpEnabled || len(builder.mcpServers) != 2 {
		t.Error("mcp config not set correctly")
	}
}

// Benchmarks

func BenchmarkNewSessionBuilder(b *testing.B) {
	runner := &Runner{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewSessionBuilder(runner)
	}
}

func BenchmarkSessionBuilderFluentAPI(b *testing.B) {
	runner := &Runner{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewSessionBuilder(runner).
			WithSessionKey("session-1").
			WithAgentType("claude-code").
			WithLaunchCommand("claude", []string{"--headless"}).
			WithEnvVars(map[string]string{"KEY": "VALUE"}).
			WithTerminalSize(40, 120)
	}
}

func BenchmarkSessionBuilderMergeEnvVars(b *testing.B) {
	runner := &Runner{
		cfg: &config.Config{
			AgentEnvVars: map[string]string{
				"VAR1": "value1",
				"VAR2": "value2",
			},
		},
	}

	builder := NewSessionBuilder(runner).
		WithEnvVars(map[string]string{
			"SESSION1": "session_value1",
		})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder.mergeEnvVars()
	}
}

// --- Additional tests for Build coverage ---

func TestSessionBuilderBuildSuccessWithOptions(t *testing.T) {
	tempDir := t.TempDir()
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: tempDir,
		},
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("build-session").
		WithAgentType("claude-code").
		WithLaunchCommand("echo", []string{"hello"})

	session, err := builder.Build(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if session.SessionKey != "build-session" {
		t.Errorf("SessionKey = %s, want build-session", session.SessionKey)
	}
	if session.Status != SessionStatusInitializing {
		t.Errorf("Status = %s, want initializing", session.Status)
	}

	// Clean up
	if session.Terminal != nil {
		session.Terminal.Stop()
	}
}

func TestSessionBuilderBuildWithPrepScript(t *testing.T) {
	tempDir := t.TempDir()
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: tempDir,
		},
	}

	builder := NewSessionBuilder(runner).
		WithSessionKey("prep-build-session").
		WithAgentType("test").
		WithLaunchCommand("echo", nil).
		WithPreparationScript("echo prep", 5)

	session, err := builder.Build(context.Background())
	if err != nil {
		t.Logf("Build with prep script: %v", err)
	}

	// Clean up
	if session != nil && session.Terminal != nil {
		session.Terminal.Stop()
	}
}

func TestSessionBuilderBuildTerminalError(t *testing.T) {
	tempDir := t.TempDir()
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: tempDir,
		},
	}

	// Use a command that doesn't exist
	builder := NewSessionBuilder(runner).
		WithSessionKey("error-session").
		WithAgentType("test").
		WithLaunchCommand("/nonexistent/command/path/that/doesnt/exist/12345", nil)

	session, err := builder.Build(context.Background())
	// May or may not fail depending on terminal implementation
	t.Logf("Build with invalid command: session=%v, err=%v", session != nil, err)

	// Clean up if session was created
	if session != nil && session.Terminal != nil {
		session.Terminal.Stop()
	}
}

// --- Additional tests for resolveWorkingDirectory coverage ---

func TestSessionBuilderResolveWithWorkspaceManager(t *testing.T) {
	tempDir := t.TempDir()
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot: tempDir,
		},
	}

	// No workspace manager, but with config
	builder := NewSessionBuilder(runner).
		WithSessionKey("workspace-test")

	workDir, worktreePath, branchName, err := builder.resolveWorkingDirectory(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should fall back to workspace root
	if workDir != tempDir {
		t.Errorf("workDir = %s, want %s", workDir, tempDir)
	}
	if worktreePath != "" {
		t.Errorf("worktreePath should be empty, got %s", worktreePath)
	}
	if branchName != "" {
		t.Errorf("branchName should be empty, got %s", branchName)
	}
}

// --- Test resolveWorkingDirectory Priority 3 ---

func TestSessionBuilderResolveWithTempWorkspace(t *testing.T) {
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
		WithSessionKey("temp-workspace-test")
	// No repository URL, no worktree - should use temp workspace (Priority 3)

	workDir, worktreePath, branchName, err := builder.resolveWorkingDirectory(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should use TempWorkspace
	if workDir == "" {
		t.Error("workDir should not be empty")
	}
	if worktreePath != "" {
		t.Errorf("worktreePath should be empty, got %s", worktreePath)
	}
	if branchName != "" {
		t.Errorf("branchName should be empty, got %s", branchName)
	}
}

// --- Test runPreparation with worktreeService ---

func TestSessionBuilderRunPreparationWithMainRepoDir(t *testing.T) {
	tempDir := t.TempDir()

	// Create mock worktree service
	runner := &Runner{
		cfg: &config.Config{
			WorkspaceRoot:  tempDir,
			RepositoryPath: tempDir,
			WorktreesDir:   tempDir,
		},
	}
	runner.initEnhancedComponents(runner.cfg)

	builder := NewSessionBuilder(runner).
		WithSessionKey("prep-worktree-session").
		WithPreparationScript("echo test", 5).
		WithEnvVars(map[string]string{"TEST": "value"})

	err := builder.runPreparation(context.Background(), tempDir, tempDir, "main")
	// May or may not fail depending on script
	t.Logf("runPreparation: %v", err)
}

// --- Test ExtendedSession ---

func TestExtendedSessionStructWithCallbacks(t *testing.T) {
	session := &Session{
		ID:     "ext-session",
		Status: SessionStatusRunning,
	}

	extended := ExtendedSession{
		Session:          session,
		OnOutput:         func([]byte) {},
		OnExit:           func(int) {},
		TicketIdentifier: "TICKET-456",
		ManagedSession:   nil,
	}

	if extended.ID != "ext-session" {
		t.Errorf("ID = %s, want ext-session", extended.ID)
	}
	if extended.TicketIdentifier != "TICKET-456" {
		t.Errorf("TicketIdentifier = %s, want TICKET-456", extended.TicketIdentifier)
	}
	if extended.OnOutput == nil {
		t.Error("OnOutput should not be nil")
	}
	if extended.OnExit == nil {
		t.Error("OnExit should not be nil")
	}
}
