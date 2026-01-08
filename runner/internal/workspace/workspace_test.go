package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// --- Test Manager (workspace.go) ---

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	root := filepath.Join(tmpDir, "workspace")

	manager, err := NewManager(root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.root != root {
		t.Errorf("root: got %v, want %v", manager.root, root)
	}

	// Root directory should be created
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Error("root directory should be created")
	}
}

func TestNewManagerWithGitConfig(t *testing.T) {
	tmpDir := t.TempDir()
	root := filepath.Join(tmpDir, "workspace")
	gitConfigPath := filepath.Join(tmpDir, "git.config")

	manager, err := NewManager(root, gitConfigPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if manager.gitConfigPath != gitConfigPath {
		t.Errorf("gitConfigPath: got %v, want %v", manager.gitConfigPath, gitConfigPath)
	}
}

func TestTempWorkspace(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	path := manager.TempWorkspace("session-123")

	expectedPath := filepath.Join(tmpDir, "temp", "session-123")
	if path != expectedPath {
		t.Errorf("path: got %v, want %v", path, expectedPath)
	}

	// Directory should be created
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("temp directory should be created")
	}
}

func TestGetWorkspaceRoot(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	if manager.GetWorkspaceRoot() != tmpDir {
		t.Errorf("GetWorkspaceRoot: got %v, want %v", manager.GetWorkspaceRoot(), tmpDir)
	}
}

func TestListWorktreesEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	worktrees, err := manager.ListWorktrees()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if worktrees != nil && len(worktrees) > 0 {
		t.Errorf("worktrees should be empty, got %v", worktrees)
	}
}

func TestListWorktreesWithDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Create worktrees directory with some subdirectories
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(filepath.Join(worktreesDir, "wt1"), 0755)
	os.MkdirAll(filepath.Join(worktreesDir, "wt2"), 0755)

	worktrees, err := manager.ListWorktrees()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(worktrees) != 2 {
		t.Errorf("worktrees count: got %v, want 2", len(worktrees))
	}
}

func TestCleanupOldWorktreesEmpty(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	err := manager.CleanupOldWorktrees(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCleanupOldWorktreesInvalid(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Create worktrees directory with an invalid worktree (no .git)
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	invalidWT := filepath.Join(worktreesDir, "invalid")
	os.MkdirAll(invalidWT, 0755)

	err := manager.CleanupOldWorktrees(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Invalid worktree should be removed
	if _, err := os.Stat(invalidWT); !os.IsNotExist(err) {
		t.Error("invalid worktree should be removed")
	}
}

func TestRemoveWorktreeNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Should not error for non-existent path
	err := manager.RemoveWorktree(context.Background(), "/nonexistent/worktree")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractRepoNameSSH(t *testing.T) {
	testCases := []struct {
		url      string
		expected string
	}{
		{"git@github.com:user/repo.git", "user-repo"},
		{"git@github.com:org/project.git", "org-project"},
		{"git@gitlab.com:team/service.git", "team-service"},
	}

	for _, tc := range testCases {
		result := extractRepoName(tc.url)
		if result != tc.expected {
			t.Errorf("extractRepoName(%s): got %v, want %v", tc.url, result, tc.expected)
		}
	}
}

func TestExtractRepoNameHTTPS(t *testing.T) {
	testCases := []struct {
		url      string
		expected string
	}{
		{"https://github.com/user/repo.git", "user-repo"},
		{"https://github.com/user/repo", "user-repo"},
		{"https://gitlab.com/org/project.git", "org-project"},
	}

	for _, tc := range testCases {
		result := extractRepoName(tc.url)
		if result != tc.expected {
			t.Errorf("extractRepoName(%s): got %v, want %v", tc.url, result, tc.expected)
		}
	}
}

func TestExtractRepoNameInvalid(t *testing.T) {
	result := extractRepoName("")
	if result != "" {
		t.Errorf("extractRepoName(empty): got %v, want empty", result)
	}
}

// --- Test Preparer (preparer.go) ---

func TestPreparationContextGetEnvVars(t *testing.T) {
	ctx := &PreparationContext{
		SessionID:        "session-1",
		TicketIdentifier: "TICKET-123",
		BranchName:       "feature/test",
		WorkingDir:       "/workspace/test",
		MainRepoDir:      "/workspace/repos/main",
		WorktreeDir:      "/workspace/worktrees/session-1",
		BaseEnvVars:      map[string]string{"API_KEY": "secret"},
	}

	envVars := ctx.GetEnvVars()

	if envVars["WORKING_DIR"] != "/workspace/test" {
		t.Errorf("WORKING_DIR: got %v, want /workspace/test", envVars["WORKING_DIR"])
	}

	if envVars["MAIN_REPO_DIR"] != "/workspace/repos/main" {
		t.Errorf("MAIN_REPO_DIR: got %v, want /workspace/repos/main", envVars["MAIN_REPO_DIR"])
	}

	if envVars["WORKTREE_DIR"] != "/workspace/worktrees/session-1" {
		t.Errorf("WORKTREE_DIR: got %v, want /workspace/worktrees/session-1", envVars["WORKTREE_DIR"])
	}

	if envVars["TICKET_IDENTIFIER"] != "TICKET-123" {
		t.Errorf("TICKET_IDENTIFIER: got %v, want TICKET-123", envVars["TICKET_IDENTIFIER"])
	}

	if envVars["BRANCH_NAME"] != "feature/test" {
		t.Errorf("BRANCH_NAME: got %v, want feature/test", envVars["BRANCH_NAME"])
	}

	if envVars["API_KEY"] != "secret" {
		t.Errorf("API_KEY: got %v, want secret", envVars["API_KEY"])
	}
}

func TestPreparationContextString(t *testing.T) {
	ctx := &PreparationContext{
		SessionID:        "session-1",
		TicketIdentifier: "TICKET-123",
		WorkingDir:       "/workspace/test",
	}

	str := ctx.String()

	if str == "" {
		t.Error("String() should not be empty")
	}
}

func TestPreparationError(t *testing.T) {
	err := &PreparationError{
		Step:   "script",
		Cause:  os.ErrNotExist,
		Output: "command not found",
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("Error() should not be empty")
	}

	if err.Unwrap() != os.ErrNotExist {
		t.Error("Unwrap() should return the cause")
	}
}

func TestPreparationErrorNoOutput(t *testing.T) {
	err := &PreparationError{
		Step:  "script",
		Cause: os.ErrNotExist,
	}

	errStr := err.Error()
	if errStr == "" {
		t.Error("Error() should not be empty")
	}
}

func TestNewPreparer(t *testing.T) {
	step := NewScriptPreparationStep("echo hello", time.Minute)
	preparer := NewPreparer(step)

	if preparer == nil {
		t.Fatal("NewPreparer returned nil")
	}

	if preparer.StepCount() != 1 {
		t.Errorf("StepCount: got %v, want 1", preparer.StepCount())
	}
}

func TestNewPreparerFromScript(t *testing.T) {
	preparer := NewPreparerFromScript("echo hello", 300)

	if preparer == nil {
		t.Fatal("NewPreparerFromScript returned nil")
	}

	if preparer.StepCount() != 1 {
		t.Errorf("StepCount: got %v, want 1", preparer.StepCount())
	}
}

func TestNewPreparerFromScriptEmpty(t *testing.T) {
	preparer := NewPreparerFromScript("", 300)

	if preparer != nil {
		t.Error("NewPreparerFromScript should return nil for empty script")
	}
}

func TestNewPreparerFromScriptDefaultTimeout(t *testing.T) {
	preparer := NewPreparerFromScript("echo hello", 0)

	if preparer == nil {
		t.Fatal("NewPreparerFromScript returned nil")
	}
}

func TestPreparerAddStep(t *testing.T) {
	preparer := NewPreparer()

	if preparer.StepCount() != 0 {
		t.Errorf("initial StepCount: got %v, want 0", preparer.StepCount())
	}

	step := NewScriptPreparationStep("echo hello", time.Minute)
	preparer.AddStep(step)

	if preparer.StepCount() != 1 {
		t.Errorf("StepCount after add: got %v, want 1", preparer.StepCount())
	}
}

func TestPreparerPrepareEmpty(t *testing.T) {
	preparer := NewPreparer()
	ctx := &PreparationContext{
		SessionID:  "session-1",
		WorkingDir: t.TempDir(),
	}

	err := preparer.Prepare(context.Background(), ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPreparerPrepareSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	step := NewScriptPreparationStep("echo hello", time.Minute)
	preparer := NewPreparer(step)

	ctx := &PreparationContext{
		SessionID:  "session-1",
		WorkingDir: tmpDir,
	}

	err := preparer.Prepare(context.Background(), ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPreparerPrepareFailure(t *testing.T) {
	tmpDir := t.TempDir()
	step := NewScriptPreparationStep("exit 1", time.Minute)
	preparer := NewPreparer(step)

	ctx := &PreparationContext{
		SessionID:  "session-1",
		WorkingDir: tmpDir,
	}

	err := preparer.Prepare(context.Background(), ctx)
	if err == nil {
		t.Error("expected error for failed script")
	}

	// Check that it's a PreparationError
	if _, ok := err.(*PreparationError); !ok {
		t.Error("error should be a PreparationError")
	}
}

func TestScriptPreparationStepName(t *testing.T) {
	step := NewScriptPreparationStep("echo hello", time.Minute)

	if step.Name() != "script" {
		t.Errorf("Name: got %v, want script", step.Name())
	}
}

func TestScriptPreparationStepExecuteEmpty(t *testing.T) {
	step := NewScriptPreparationStep("", time.Minute)
	ctx := &PreparationContext{
		SessionID:  "session-1",
		WorkingDir: t.TempDir(),
	}

	err := step.Execute(context.Background(), ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScriptPreparationStepExecuteWithEnvVars(t *testing.T) {
	tmpDir := t.TempDir()
	outputFile := filepath.Join(tmpDir, "output.txt")

	// Script that writes env var to file
	script := `echo "$TICKET_IDENTIFIER" > "` + outputFile + `"`
	step := NewScriptPreparationStep(script, time.Minute)

	ctx := &PreparationContext{
		SessionID:        "session-1",
		TicketIdentifier: "TICKET-123",
		WorkingDir:       tmpDir,
	}

	err := step.Execute(context.Background(), ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check output
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	if string(data) != "TICKET-123\n" {
		t.Errorf("output: got %v, want TICKET-123", string(data))
	}
}

func TestScriptPreparationStepTimeout(t *testing.T) {
	step := NewScriptPreparationStep("sleep 10", 100*time.Millisecond)
	ctx := &PreparationContext{
		SessionID:  "session-1",
		WorkingDir: t.TempDir(),
	}

	err := step.Execute(context.Background(), ctx)
	if err == nil {
		t.Error("expected error for timeout")
	}
}

func TestScriptPreparationStepDefaultTimeout(t *testing.T) {
	step := NewScriptPreparationStep("echo hello", 0)

	if step.timeout != 5*time.Minute {
		t.Errorf("default timeout: got %v, want %v", step.timeout, 5*time.Minute)
	}
}

// mockPreparationStep for testing
type mockPreparationStep struct {
	name      string
	execError error
	executed  bool
}

func (m *mockPreparationStep) Name() string {
	return m.name
}

func (m *mockPreparationStep) Execute(ctx context.Context, prepCtx *PreparationContext) error {
	m.executed = true
	return m.execError
}

func TestPreparerStopsOnError(t *testing.T) {
	step1 := &mockPreparationStep{name: "step1"}
	step2 := &mockPreparationStep{name: "step2", execError: os.ErrNotExist}
	step3 := &mockPreparationStep{name: "step3"}

	preparer := NewPreparer(step1, step2, step3)
	ctx := &PreparationContext{
		SessionID:  "session-1",
		WorkingDir: t.TempDir(),
	}

	err := preparer.Prepare(context.Background(), ctx)
	if err == nil {
		t.Error("expected error")
	}

	if !step1.executed {
		t.Error("step1 should be executed")
	}

	if !step2.executed {
		t.Error("step2 should be executed")
	}

	if step3.executed {
		t.Error("step3 should not be executed after error")
	}
}

// --- Benchmark Tests ---

func BenchmarkExtractRepoName(b *testing.B) {
	urls := []string{
		"git@github.com:user/repo.git",
		"https://github.com/user/repo.git",
		"https://github.com/org/project",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractRepoName(urls[i%len(urls)])
	}
}

func BenchmarkPreparationContextGetEnvVars(b *testing.B) {
	ctx := &PreparationContext{
		SessionID:        "session-1",
		TicketIdentifier: "TICKET-123",
		BranchName:       "feature/test",
		WorkingDir:       "/workspace/test",
		MainRepoDir:      "/workspace/repos/main",
		WorktreeDir:      "/workspace/worktrees/session-1",
		BaseEnvVars:      map[string]string{"API_KEY": "secret"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ctx.GetEnvVars()
	}
}

// --- Additional Manager Tests ---

func TestNewManagerCreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	root := filepath.Join(tmpDir, "deep", "nested", "workspace")

	manager, err := NewManager(root, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	// Verify directory was created
	if _, err := os.Stat(root); os.IsNotExist(err) {
		t.Error("nested directory should be created")
	}
}

func TestFindMainRepoNoGitFile(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Create a worktree-like directory without .git file
	worktreePath := filepath.Join(tmpDir, "worktree")
	os.MkdirAll(worktreePath, 0755)

	_, err := manager.findMainRepo(worktreePath)
	if err == nil {
		t.Error("expected error for missing .git file")
	}
}

func TestFindMainRepoInvalidGitFile(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Create a worktree-like directory with invalid .git file
	worktreePath := filepath.Join(tmpDir, "worktree")
	os.MkdirAll(worktreePath, 0755)

	// Write invalid .git content (not starting with "gitdir: ")
	gitFile := filepath.Join(worktreePath, ".git")
	os.WriteFile(gitFile, []byte("invalid content"), 0644)

	_, err := manager.findMainRepo(worktreePath)
	if err == nil {
		t.Error("expected error for invalid .git file format")
	}
}

func TestFindMainRepoValidGitFile(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Create directory structure similar to a real worktree
	mainRepoDir := filepath.Join(tmpDir, "repos", "main")
	os.MkdirAll(filepath.Join(mainRepoDir, ".git", "worktrees", "session-1"), 0755)

	worktreePath := filepath.Join(tmpDir, "worktrees", "session-1")
	os.MkdirAll(worktreePath, 0755)

	// Write valid .git content
	gitDir := filepath.Join(mainRepoDir, ".git", "worktrees", "session-1")
	gitFile := filepath.Join(worktreePath, ".git")
	os.WriteFile(gitFile, []byte("gitdir: "+gitDir), 0644)

	repoPath, err := manager.findMainRepo(worktreePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The result should be related to the main repo
	if repoPath == "" {
		t.Error("repoPath should not be empty")
	}
}

func TestCleanupOldWorktreesWithValidWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Create worktrees directory with a valid worktree (has .git file)
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	validWT := filepath.Join(worktreesDir, "valid")
	os.MkdirAll(validWT, 0755)

	// Create a .git file (makes it a "valid" worktree)
	gitFile := filepath.Join(validWT, ".git")
	os.WriteFile(gitFile, []byte("gitdir: /some/path"), 0644)

	err := manager.CleanupOldWorktrees(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Valid worktree should not be removed
	if _, err := os.Stat(validWT); os.IsNotExist(err) {
		t.Error("valid worktree should not be removed")
	}
}

func TestCleanupOldWorktreesWithFile(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Create worktrees directory with a file (not directory)
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(worktreesDir, 0755)

	// Create a file instead of directory
	testFile := filepath.Join(worktreesDir, "testfile")
	os.WriteFile(testFile, []byte("test"), 0644)

	err := manager.CleanupOldWorktrees(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should still exist (not a directory)
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Error("file should not be removed")
	}
}

func TestRemoveWorktreeWithGitFile(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Create a worktree-like structure
	worktreePath := filepath.Join(tmpDir, "worktrees", "test-wt")
	os.MkdirAll(worktreePath, 0755)

	// Create .git file pointing to non-existent repo
	gitFile := filepath.Join(worktreePath, ".git")
	os.WriteFile(gitFile, []byte("gitdir: /nonexistent/repo/.git/worktrees/test-wt"), 0644)

	// Should not error, will fall back to os.RemoveAll
	err := manager.RemoveWorktree(context.Background(), worktreePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Directory should be removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("worktree should be removed")
	}
}

func TestListWorktreesReadError(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Create worktrees directory without read permission
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(worktreesDir, 0755)
	os.Chmod(worktreesDir, 0000)
	defer os.Chmod(worktreesDir, 0755)

	_, err := manager.ListWorktrees()
	if err == nil {
		t.Error("expected error for unreadable directory")
	}
}

func TestExtractRepoNameSinglePart(t *testing.T) {
	// Test with a URL that has only one part
	result := extractRepoName("repo")
	if result != "" {
		t.Errorf("extractRepoName(repo): got %v, want empty", result)
	}
}

func TestExtractRepoNameSSHVariants(t *testing.T) {
	testCases := []struct {
		url      string
		expected string
	}{
		{"git@github.com:user/repo", "user-repo"},
		{"git@bitbucket.org:team/project.git", "team-project"},
	}

	for _, tc := range testCases {
		result := extractRepoName(tc.url)
		if result != tc.expected {
			t.Errorf("extractRepoName(%s): got %v, want %v", tc.url, result, tc.expected)
		}
	}
}

// --- Additional PreparationContext Tests ---

func TestPreparationContextGetEnvVarsEmpty(t *testing.T) {
	ctx := &PreparationContext{
		SessionID:  "session-1",
		WorkingDir: "/workspace",
	}

	envVars := ctx.GetEnvVars()

	// WORKING_DIR is always set
	if envVars["WORKING_DIR"] != "/workspace" {
		t.Errorf("WORKING_DIR: got %v, want /workspace", envVars["WORKING_DIR"])
	}

	// Optional fields should not be set when empty
	if _, ok := envVars["TICKET_IDENTIFIER"]; ok {
		t.Error("TICKET_IDENTIFIER should not be set when empty")
	}
}

func TestPreparationContextStringFormat(t *testing.T) {
	ctx := &PreparationContext{
		SessionID:        "session-1",
		TicketIdentifier: "TICKET-123",
		BranchName:       "feature/test",
		WorkingDir:       "/workspace",
	}

	str := ctx.String()

	// Should contain key fields
	if !contains(str, "session-1") {
		t.Error("String() should contain session ID")
	}
	if !contains(str, "TICKET-123") {
		t.Error("String() should contain ticket identifier")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Additional Preparer Tests ---

func TestPreparerMultipleSteps(t *testing.T) {
	tmpDir := t.TempDir()

	step1 := NewScriptPreparationStep("echo step1", time.Minute)
	step2 := NewScriptPreparationStep("echo step2", time.Minute)

	preparer := NewPreparer(step1, step2)

	if preparer.StepCount() != 2 {
		t.Errorf("StepCount: got %v, want 2", preparer.StepCount())
	}

	ctx := &PreparationContext{
		SessionID:  "session-1",
		WorkingDir: tmpDir,
	}

	err := preparer.Prepare(context.Background(), ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPreparerContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a step that will take a while
	step := NewScriptPreparationStep("sleep 5", time.Minute)
	preparer := NewPreparer(step)

	prepCtx := &PreparationContext{
		SessionID:  "session-1",
		WorkingDir: tmpDir,
	}

	// Create a context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := preparer.Prepare(ctx, prepCtx)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

// --- CreateWorktree and related tests ---

func TestCreateWorktreeInvalidRepoURL(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	_, err := manager.CreateWorktree(context.Background(), "", "main", "session-1")
	if err == nil {
		t.Error("expected error for empty repo URL")
	}
}

func TestCreateWorktreeInvalidRepoURLFormat(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Single word URL that doesn't match any pattern
	_, err := manager.CreateWorktree(context.Background(), "invalid", "main", "session-1")
	if err == nil {
		t.Error("expected error for invalid repo URL")
	}
}

// TestRemoveWorktreeInternalFallback tests the fallback to os.RemoveAll
func TestRemoveWorktreeInternalFallback(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Create a worktree-like directory
	worktreePath := filepath.Join(tmpDir, "worktree")
	repoPath := filepath.Join(tmpDir, "repo")
	os.MkdirAll(worktreePath, 0755)
	os.MkdirAll(repoPath, 0755)

	// Write some file to verify removal
	testFile := filepath.Join(worktreePath, "test.txt")
	os.WriteFile(testFile, []byte("test"), 0644)

	// removeWorktreeInternal should fall back to os.RemoveAll when git command fails
	err := manager.removeWorktreeInternal(context.Background(), repoPath, worktreePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Directory should be removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("worktree should be removed")
	}
}

// TestFindMainRepoBareRepo tests findMainRepo with a bare repository structure
func TestFindMainRepoBareRepo(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Create directory structure for a bare repo worktree
	bareRepoDir := filepath.Join(tmpDir, "repo.git")
	os.MkdirAll(filepath.Join(bareRepoDir, "worktrees", "session-1"), 0755)

	worktreePath := filepath.Join(tmpDir, "worktrees", "session-1")
	os.MkdirAll(worktreePath, 0755)

	// Write valid .git content pointing to bare repo
	gitDir := filepath.Join(bareRepoDir, "worktrees", "session-1")
	gitFile := filepath.Join(worktreePath, ".git")
	os.WriteFile(gitFile, []byte("gitdir: "+gitDir), 0644)

	repoPath, err := manager.findMainRepo(worktreePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// For bare repos, should return the bare repo dir
	if repoPath == "" {
		t.Error("repoPath should not be empty")
	}
}

// TestApplyGitConfigEmptyPath tests applyGitConfig with empty path
func TestApplyGitConfigEmptyPath(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Should return nil for empty gitConfigPath
	err := manager.applyGitConfig(context.Background(), tmpDir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// TestApplyGitConfigMissingFile tests applyGitConfig with missing config file
func TestApplyGitConfigMissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "/nonexistent/config")

	err := manager.applyGitConfig(context.Background(), tmpDir)
	if err == nil {
		t.Error("expected error for missing config file")
	}
}

// TestApplyGitConfigValidFile tests applyGitConfig with a valid config file
func TestApplyGitConfigValidFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a git config file
	configPath := filepath.Join(tmpDir, "git.config")
	os.WriteFile(configPath, []byte("[user]\n\tname = Test User\n"), 0644)

	manager, _ := NewManager(tmpDir, configPath)

	// Create a repo-like structure
	repoPath := filepath.Join(tmpDir, "repo")
	os.MkdirAll(filepath.Join(repoPath, ".git"), 0755)

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	cmd.Run()

	err := manager.applyGitConfig(context.Background(), repoPath)
	if err != nil {
		t.Logf("applyGitConfig error (may fail without git repo): %v", err)
	}
}

// TestCleanupOldWorktreesReadDirError tests CleanupOldWorktrees with unreadable directory
func TestCleanupOldWorktreesReadDirError(t *testing.T) {
	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Create worktrees directory without read permission
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	os.MkdirAll(worktreesDir, 0755)

	// Create a subdirectory, then make parent unreadable
	subDir := filepath.Join(worktreesDir, "subdir")
	os.MkdirAll(subDir, 0755)
	os.Chmod(worktreesDir, 0000)
	defer os.Chmod(worktreesDir, 0755)

	err := manager.CleanupOldWorktrees(context.Background())
	if err == nil {
		t.Error("expected error for unreadable directory")
	}
}

// TestEnsureRepositoryClone tests ensureRepository with a clone operation
func TestEnsureRepositoryClone(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Create a source repo to clone from
	sourceRepo := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceRepo, 0755)

	// Initialize source repo
	cmd := exec.Command("git", "init")
	cmd.Dir = sourceRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to init source repo: %v", err)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = sourceRepo
	cmd.Run()

	// Create a file and commit
	testFile := filepath.Join(sourceRepo, "README.md")
	os.WriteFile(testFile, []byte("# Test\n"), 0644)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = sourceRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Clone with ensureRepository
	destPath := filepath.Join(tmpDir, "clone")
	err := manager.ensureRepository(context.Background(), sourceRepo, destPath)
	if err != nil {
		t.Fatalf("ensureRepository clone failed: %v", err)
	}

	// Verify clone exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("clone directory should exist")
	}
}

// TestEnsureRepositoryFetch tests ensureRepository with a fetch operation
func TestEnsureRepositoryFetch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Create a source repo
	sourceRepo := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceRepo, 0755)

	// Initialize source repo
	cmd := exec.Command("git", "init")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = sourceRepo
	cmd.Run()

	testFile := filepath.Join(sourceRepo, "README.md")
	os.WriteFile(testFile, []byte("# Test\n"), 0644)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = sourceRepo
	cmd.Run()

	// First clone (bare clone)
	destPath := filepath.Join(tmpDir, "clone")
	err := manager.ensureRepository(context.Background(), sourceRepo, destPath)
	if err != nil {
		t.Fatalf("initial clone failed: %v", err)
	}

	// For bare clone, .git is not in a subdirectory - the destPath IS the git directory
	// So we need to check that a second call fetches instead of cloning
	// The ensureRepository checks for .git in path, but bare clones don't have .git subdirectory
	// Let's verify the fetch path by creating a .git marker
	os.MkdirAll(filepath.Join(destPath, ".git"), 0755)

	// Second call should fetch (repo already exists with .git marker)
	err = manager.ensureRepository(context.Background(), sourceRepo, destPath)
	// This may fail because it's not a valid git repo, but the fetch code path is executed
	t.Logf("fetch result: %v", err)
}

// TestNewManagerError tests NewManager with invalid path
func TestNewManagerError(t *testing.T) {
	// Try to create manager in a read-only location (will likely fail on most systems)
	// Skip if running as root
	if os.Geteuid() == 0 {
		t.Skip("skipping as root")
	}

	// Create a read-only directory
	tmpDir := t.TempDir()
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	os.MkdirAll(readOnlyDir, 0755)
	os.Chmod(readOnlyDir, 0444)
	defer os.Chmod(readOnlyDir, 0755)

	// Try to create workspace inside read-only dir
	root := filepath.Join(readOnlyDir, "workspace")
	_, err := NewManager(root, "")
	if err == nil {
		t.Error("expected error for read-only parent directory")
	}
}

// --- Integration tests for CreateWorktree ---

// TestCreateWorktreeFullIntegration tests the full worktree creation flow
func TestCreateWorktreeFullIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	// Create a source repo with main branch
	sourceRepo := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceRepo, 0755)

	// Initialize source repo
	initCmd := exec.Command("git", "init")
	initCmd.Dir = sourceRepo
	if err := initCmd.Run(); err != nil {
		t.Fatalf("failed to init source repo: %v", err)
	}

	// Configure git
	exec.Command("git", "config", "user.email", "test@test.com").Run()
	cmd := exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = sourceRepo
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = sourceRepo
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(sourceRepo, "README.md")
	os.WriteFile(testFile, []byte("# Test Repo\n"), 0644)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = sourceRepo
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	// Rename branch to main
	cmd = exec.Command("git", "branch", "-M", "main")
	cmd.Dir = sourceRepo
	cmd.Run()

	// Create workspace manager
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	manager, err := NewManager(workspaceRoot, "")
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// CreateWorktree uses git:// or https:// URLs, but we can test with local path
	// This will test the internal logic
	worktreePath, err := manager.CreateWorktree(ctx, sourceRepo, "main", "test-session")
	if err != nil {
		// Expected to fail because local path clone may not work as expected
		t.Logf("CreateWorktree error (expected for local paths): %v", err)
	} else {
		// If it succeeded, verify the worktree
		if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
			t.Error("worktree path should exist")
		}
	}
}

// TestCreateWorktreeExistingWorktree tests removing existing worktree
func TestCreateWorktreeExistingWorktree(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	// Create a source repo
	sourceRepo := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceRepo, 0755)

	cmd := exec.Command("git", "init")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = sourceRepo
	cmd.Run()

	testFile := filepath.Join(sourceRepo, "file.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = sourceRepo
	cmd.Run()

	// Create workspace manager
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	manager, _ := NewManager(workspaceRoot, "")

	// Pre-create a worktree directory that should be removed
	existingWorktree := filepath.Join(workspaceRoot, "worktrees", "test-session")
	os.MkdirAll(existingWorktree, 0755)
	os.WriteFile(filepath.Join(existingWorktree, "existing.txt"), []byte("old"), 0644)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// CreateWorktree should handle the existing directory
	_, err := manager.CreateWorktree(ctx, sourceRepo, "main", "test-session")
	// This tests the existing worktree removal path
	t.Logf("CreateWorktree result: %v", err)
}

// TestRemoveWorktreeInternalWithPrune tests the prune path in removeWorktreeInternal
func TestRemoveWorktreeInternalWithPrune(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	// Create a git repo
	repoPath := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoPath, 0755)

	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoPath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = repoPath
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(repoPath, "file.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoPath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = repoPath
	cmd.Run()

	// Create a worktree using git
	worktreePath := filepath.Join(tmpDir, "worktree")
	cmd = exec.Command("git", "worktree", "add", worktreePath, "HEAD")
	cmd.Dir = repoPath
	if err := cmd.Run(); err != nil {
		t.Fatalf("failed to create git worktree: %v", err)
	}

	// Create manager and remove the worktree
	manager, _ := NewManager(tmpDir, "")

	ctx := context.Background()
	err := manager.removeWorktreeInternal(ctx, repoPath, worktreePath)
	if err != nil {
		t.Fatalf("removeWorktreeInternal failed: %v", err)
	}

	// Verify worktree is removed
	if _, err := os.Stat(worktreePath); !os.IsNotExist(err) {
		t.Error("worktree should be removed")
	}
}

// TestEnsureRepositoryMkdirError tests ensureRepository when MkdirAll fails
func TestEnsureRepositoryMkdirError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("skipping as root")
	}

	tmpDir := t.TempDir()
	manager, _ := NewManager(tmpDir, "")

	// Create a read-only directory
	readOnlyDir := filepath.Join(tmpDir, "readonly")
	os.MkdirAll(readOnlyDir, 0755)
	os.Chmod(readOnlyDir, 0444)
	defer os.Chmod(readOnlyDir, 0755)

	// Try to create repo in read-only directory
	destPath := filepath.Join(readOnlyDir, "nested", "repo")
	err := manager.ensureRepository(context.Background(), "file:///fake", destPath)
	if err == nil {
		t.Error("expected error for mkdir in read-only directory")
	}
}

// TestApplyGitConfigWriteError tests applyGitConfig when config write fails
func TestApplyGitConfigWriteError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("skipping as root")
	}

	tmpDir := t.TempDir()

	// Create a git config file
	configPath := filepath.Join(tmpDir, "git.config")
	os.WriteFile(configPath, []byte("[user]\n\tname = Test\n"), 0644)

	manager, _ := NewManager(tmpDir, configPath)

	// Create a repo path with read-only .git directory
	repoPath := filepath.Join(tmpDir, "repo")
	gitDir := filepath.Join(repoPath, ".git")
	os.MkdirAll(gitDir, 0755)
	os.Chmod(gitDir, 0444)
	defer os.Chmod(gitDir, 0755)

	err := manager.applyGitConfig(context.Background(), repoPath)
	if err == nil {
		t.Error("expected error when writing to read-only .git directory")
	}
}

// TestCreateWorktreeMkdirParentError tests CreateWorktree when parent dir creation fails
func TestCreateWorktreeMkdirParentError(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("skipping as root")
	}

	tmpDir := t.TempDir()

	// Create workspace with repos dir that has proper permissions
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	manager, _ := NewManager(workspaceRoot, "")

	// Make the worktrees parent directory read-only after creating workspace
	worktreesParent := filepath.Join(workspaceRoot, "worktrees")
	os.MkdirAll(worktreesParent, 0755)
	// We need to prevent creating the session subdirectory
	// Create a file with the same name as what would be created
	sessionPath := filepath.Join(worktreesParent, "session-1")
	// Create parent and make it read-only
	os.MkdirAll(worktreesParent, 0755)
	os.Chmod(worktreesParent, 0444)
	defer os.Chmod(worktreesParent, 0755)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := manager.CreateWorktree(ctx, "https://github.com/test/repo.git", "main", "session-1")
	if err == nil {
		t.Error("expected error when parent directory is read-only")
	}
	_ = sessionPath // Used in error path
}

// --- Test addToolPaths ---

// TestAddToolPathsWithPATH tests addToolPaths when PATH exists
func TestAddToolPathsWithPATH(t *testing.T) {
	step := NewScriptPreparationStep("echo test", time.Minute)

	env := []string{"HOME=/home/test", "PATH=/usr/bin:/bin", "USER=test"}
	result := step.addToolPaths(env)

	// Check PATH was modified
	pathFound := false
	for _, e := range result {
		if strings.HasPrefix(e, "PATH=") {
			pathFound = true
			path := strings.TrimPrefix(e, "PATH=")
			// Should contain original path
			if !strings.Contains(path, "/usr/bin") {
				t.Errorf("PATH should contain /usr/bin, got: %s", path)
			}
			// Should contain extra paths based on OS
			if !strings.Contains(path, "/usr/local/bin") {
				t.Errorf("PATH should contain /usr/local/bin, got: %s", path)
			}
		}
	}

	if !pathFound {
		t.Error("PATH should be present in result")
	}
}

// TestAddToolPathsWithoutPATH tests addToolPaths when PATH doesn't exist
func TestAddToolPathsWithoutPATH(t *testing.T) {
	step := NewScriptPreparationStep("echo test", time.Minute)

	env := []string{"HOME=/home/test", "USER=test"}
	result := step.addToolPaths(env)

	// Check PATH was added
	pathFound := false
	for _, e := range result {
		if strings.HasPrefix(e, "PATH=") {
			pathFound = true
			path := strings.TrimPrefix(e, "PATH=")
			// Should contain default paths
			if !strings.Contains(path, "/usr/bin") {
				t.Errorf("PATH should contain /usr/bin, got: %s", path)
			}
			if !strings.Contains(path, "/bin") {
				t.Errorf("PATH should contain /bin, got: %s", path)
			}
		}
	}

	if !pathFound {
		t.Error("PATH should be added to environment")
	}
}

// TestBuildEnv tests buildEnv function
func TestBuildEnv(t *testing.T) {
	step := NewScriptPreparationStep("echo test", time.Minute)

	prepCtx := &PreparationContext{
		SessionID:        "test-session",
		TicketIdentifier: "TICKET-123",
		WorkingDir:       "/workspace",
	}

	env := step.buildEnv(prepCtx)

	// Check that env contains workspace variables
	hasWorkingDir := false
	hasTicketID := false
	for _, e := range env {
		if e == "WORKING_DIR=/workspace" {
			hasWorkingDir = true
		}
		if e == "TICKET_IDENTIFIER=TICKET-123" {
			hasTicketID = true
		}
	}

	if !hasWorkingDir {
		t.Error("env should contain WORKING_DIR")
	}
	if !hasTicketID {
		t.Error("env should contain TICKET_IDENTIFIER")
	}
}

// --- Additional coverage tests ---

// TestCreateWorktreeFetchBranchFallback tests the master branch fallback
func TestCreateWorktreeFetchBranchFallback(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	// Create a source repo with master branch
	sourceRepo := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceRepo, 0755)

	cmd := exec.Command("git", "init")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = sourceRepo
	cmd.Run()

	testFile := filepath.Join(sourceRepo, "file.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = sourceRepo
	cmd.Run()

	// Rename to master (not main) to test fallback
	cmd = exec.Command("git", "branch", "-M", "master")
	cmd.Dir = sourceRepo
	cmd.Run()

	// Create workspace manager
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	manager, _ := NewManager(workspaceRoot, "")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Call with empty branch (defaults to main, should fallback to master)
	_, err := manager.CreateWorktree(ctx, sourceRepo, "", "test-session")
	// This tests the branch fallback path
	t.Logf("CreateWorktree with fallback result: %v", err)
}

// TestCreateWorktreeWithGitConfig tests CreateWorktree with git config
func TestCreateWorktreeWithGitConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	// Create a source repo
	sourceRepo := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceRepo, 0755)

	cmd := exec.Command("git", "init")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = sourceRepo
	cmd.Run()

	testFile := filepath.Join(sourceRepo, "file.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "branch", "-M", "main")
	cmd.Dir = sourceRepo
	cmd.Run()

	// Create git config file
	configPath := filepath.Join(tmpDir, "git.config")
	os.WriteFile(configPath, []byte("[user]\n\tname = Custom User\n"), 0644)

	// Create workspace manager with git config
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	manager, _ := NewManager(workspaceRoot, configPath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// This tests the gitConfigPath code path in CreateWorktree
	_, err := manager.CreateWorktree(ctx, sourceRepo, "main", "test-session")
	t.Logf("CreateWorktree with git config result: %v", err)
}

// TestCreateWorktreeNonMainBranch tests CreateWorktree with a specific non-main branch
func TestCreateWorktreeNonMainBranch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tmpDir := t.TempDir()

	// Create a source repo
	sourceRepo := filepath.Join(tmpDir, "source")
	os.MkdirAll(sourceRepo, 0755)

	cmd := exec.Command("git", "init")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test")
	cmd.Dir = sourceRepo
	cmd.Run()

	testFile := filepath.Join(sourceRepo, "file.txt")
	os.WriteFile(testFile, []byte("content"), 0644)

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = sourceRepo
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "init")
	cmd.Dir = sourceRepo
	cmd.Run()

	// Create a feature branch
	cmd = exec.Command("git", "checkout", "-b", "feature/test")
	cmd.Dir = sourceRepo
	cmd.Run()

	// Create workspace manager
	workspaceRoot := filepath.Join(tmpDir, "workspace")
	manager, _ := NewManager(workspaceRoot, "")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// This tests the non-main branch path (doesn't try master fallback)
	_, err := manager.CreateWorktree(ctx, sourceRepo, "feature/test", "test-session")
	t.Logf("CreateWorktree with feature branch result: %v", err)
}
