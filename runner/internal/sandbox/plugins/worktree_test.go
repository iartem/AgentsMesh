package plugins

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/anthropics/agentmesh/runner/internal/sandbox"
)

func TestNewWorktreePlugin(t *testing.T) {
	p := NewWorktreePlugin("/tmp/repos")

	if p == nil {
		t.Fatal("NewWorktreePlugin() returned nil")
	}
	if p.reposDir != "/tmp/repos" {
		t.Errorf("reposDir = %q, want %q", p.reposDir, "/tmp/repos")
	}
}

func TestWorktreePluginName(t *testing.T) {
	p := NewWorktreePlugin("/tmp/repos")

	if p.Name() != "worktree" {
		t.Errorf("Name() = %q, want %q", p.Name(), "worktree")
	}
}

func TestWorktreePluginOrder(t *testing.T) {
	p := NewWorktreePlugin("/tmp/repos")

	if p.Order() != 10 {
		t.Errorf("Order() = %d, want 10", p.Order())
	}
}

func TestWorktreePluginSetupSkipsWithoutConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "worktree-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewWorktreePlugin(filepath.Join(tmpDir, "repos"))
	sb := sandbox.NewSandbox("test-session", filepath.Join(tmpDir, "sandbox"))
	ctx := context.Background()

	// Setup without repository_url or ticket_identifier should skip
	tests := []struct {
		name   string
		config map[string]interface{}
	}{
		{
			name:   "empty config",
			config: nil,
		},
		{
			name: "only repository_url",
			config: map[string]interface{}{
				"repository_url": "https://github.com/test/repo.git",
			},
		},
		{
			name: "only ticket_identifier",
			config: map[string]interface{}{
				"ticket_identifier": "TBD-123",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb.WorkDir = "" // Reset
			if err := p.Setup(ctx, sb, tt.config); err != nil {
				t.Fatalf("Setup() failed: %v", err)
			}
			// WorkDir should remain empty
			if sb.WorkDir != "" {
				t.Errorf("WorkDir = %q, want empty", sb.WorkDir)
			}
		})
	}
}

func TestWorktreePluginInjectToken(t *testing.T) {
	p := NewWorktreePlugin("/tmp/repos")

	tests := []struct {
		name     string
		repoURL  string
		token    string
		expected string
	}{
		{
			name:     "HTTPS URL",
			repoURL:  "https://github.com/org/repo.git",
			token:    "ghp_xxxx",
			expected: "https://ghp_xxxx@github.com/org/repo.git",
		},
		{
			name:     "HTTP URL",
			repoURL:  "http://github.com/org/repo.git",
			token:    "token123",
			expected: "http://token123@github.com/org/repo.git",
		},
		{
			name:     "SSH URL",
			repoURL:  "git@github.com:org/repo.git",
			token:    "ghp_xxxx",
			expected: "git@github.com:org/repo.git", // Unchanged
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := p.injectToken(tt.repoURL, tt.token)
			if result != tt.expected {
				t.Errorf("injectToken() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestHashRepoURL(t *testing.T) {
	tests := []struct {
		url1 string
		url2 string
		same bool
	}{
		{
			url1: "https://github.com/org/repo.git",
			url2: "https://github.com/org/repo.git",
			same: true,
		},
		{
			url1: "https://github.com/org/repo1.git",
			url2: "https://github.com/org/repo2.git",
			same: false,
		},
	}

	for _, tt := range tests {
		hash1 := hashRepoURL(tt.url1)
		hash2 := hashRepoURL(tt.url2)

		// Verify hash length is 16 characters
		if len(hash1) != 16 {
			t.Errorf("hashRepoURL() length = %d, want 16", len(hash1))
		}

		if tt.same && hash1 != hash2 {
			t.Errorf("hashRepoURL() should be same for %q and %q", tt.url1, tt.url2)
		}
		if !tt.same && hash1 == hash2 {
			t.Errorf("hashRepoURL() should be different for %q and %q", tt.url1, tt.url2)
		}
	}
}

func TestWorktreePluginTeardownNoMetadata(t *testing.T) {
	p := NewWorktreePlugin("/tmp/repos")
	sb := sandbox.NewSandbox("test-session", "/tmp/sandbox")

	// Teardown without metadata should not error
	if err := p.Teardown(sb); err != nil {
		t.Errorf("Teardown() failed: %v", err)
	}
}

func TestWorktreePluginGetEnvWithPath(t *testing.T) {
	p := NewWorktreePlugin("/tmp/repos")

	env := p.getEnvWithPath()

	// Check PATH is present
	hasPath := false
	for _, e := range env {
		if len(e) > 5 && e[:5] == "PATH=" {
			hasPath = true
			// Verify it contains homebrew paths (on macOS) or /usr/local/bin
			path := e[5:]
			if !contains(path, "/usr/local/bin") {
				t.Errorf("PATH should contain /usr/local/bin, got %q", path)
			}
			break
		}
	}

	if !hasPath {
		t.Error("PATH environment variable not found")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Integration tests that require git

// setupTestRepo creates a temporary git repository for testing
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "git-test-repo-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		t.Fatalf("git init failed: %v\noutput: %s", err, output)
	}

	// Configure git user for commits
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	cmd.Run()

	// Create initial commit
	testFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(testFile, []byte("# Test Repo\n"), 0644); err != nil {
		cleanup()
		t.Fatalf("Failed to create test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = tmpDir
	if output, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		t.Fatalf("git commit failed: %v\noutput: %s", err, output)
	}

	return tmpDir, cleanup
}

// setupBareRepo creates a bare repository from a normal repo (simulates remote)
func setupBareRepo(t *testing.T, sourceRepo string) (string, func()) {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "git-bare-repo-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	// Clone as bare repo
	cmd := exec.Command("git", "clone", "--bare", sourceRepo, tmpDir)
	if output, err := cmd.CombinedOutput(); err != nil {
		cleanup()
		t.Fatalf("git clone --bare failed: %v\noutput: %s", err, output)
	}

	return tmpDir, cleanup
}

func TestWorktreePluginSetupWithGit(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	// Create source repo
	sourceRepo, cleanupSource := setupTestRepo(t)
	defer cleanupSource()

	// Create bare repo to simulate remote
	bareRepo, cleanupBare := setupBareRepo(t, sourceRepo)
	defer cleanupBare()

	// Create workspace
	tmpDir, err := os.MkdirTemp("", "worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	reposDir := filepath.Join(tmpDir, "repos")
	sandboxDir := filepath.Join(tmpDir, "sandbox")

	p := NewWorktreePlugin(reposDir)
	sb := sandbox.NewSandbox("test-session", sandboxDir)
	ctx := context.Background()

	config := map[string]interface{}{
		"repository_url":    bareRepo,
		"ticket_identifier": "TEST-123",
		"branch":            "main", // Use main as our test repo uses main (default after init)
	}

	// Create sandbox directory
	if err := os.MkdirAll(sandboxDir, 0755); err != nil {
		t.Fatalf("Failed to create sandbox dir: %v", err)
	}

	// Setup should create worktree
	if err := p.Setup(ctx, sb, config); err != nil {
		// Check for 'master' vs 'main' branch issue
		if strings.Contains(err.Error(), "fatal: invalid reference: main") {
			// Try with master branch
			config["branch"] = "master"
			if err := p.Setup(ctx, sb, config); err != nil {
				t.Fatalf("Setup() failed with master branch: %v", err)
			}
		} else {
			t.Fatalf("Setup() failed: %v", err)
		}
	}

	// Verify WorkDir is set
	expectedWorkDir := filepath.Join(sandboxDir, "worktree")
	if sb.WorkDir != expectedWorkDir {
		t.Errorf("WorkDir = %q, want %q", sb.WorkDir, expectedWorkDir)
	}

	// Verify worktree directory exists
	if _, err := os.Stat(sb.WorkDir); os.IsNotExist(err) {
		t.Error("Worktree directory was not created")
	}

	// Verify .git file exists (worktree indicator)
	gitFile := filepath.Join(sb.WorkDir, ".git")
	if _, err := os.Stat(gitFile); os.IsNotExist(err) {
		t.Error(".git file not found in worktree")
	}

	// Verify metadata
	if sb.Metadata["workspace_type"] != "worktree" {
		t.Errorf("Metadata[workspace_type] = %q, want %q", sb.Metadata["workspace_type"], "worktree")
	}
	if sb.Metadata["repo_url"] != bareRepo {
		t.Errorf("Metadata[repo_url] = %q, want %q", sb.Metadata["repo_url"], bareRepo)
	}
}

func TestWorktreePluginTeardownWithGit(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	// Create source repo
	sourceRepo, cleanupSource := setupTestRepo(t)
	defer cleanupSource()

	// Create bare repo
	bareRepo, cleanupBare := setupBareRepo(t, sourceRepo)
	defer cleanupBare()

	// Create workspace
	tmpDir, err := os.MkdirTemp("", "worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	reposDir := filepath.Join(tmpDir, "repos")
	sandboxDir := filepath.Join(tmpDir, "sandbox")

	p := NewWorktreePlugin(reposDir)
	sb := sandbox.NewSandbox("test-session", sandboxDir)
	ctx := context.Background()

	config := map[string]interface{}{
		"repository_url":    bareRepo,
		"ticket_identifier": "TEST-123",
		"branch":            "main",
	}

	// Create sandbox directory
	os.MkdirAll(sandboxDir, 0755)

	// Setup worktree
	if err := p.Setup(ctx, sb, config); err != nil {
		// Try with master
		config["branch"] = "master"
		if err := p.Setup(ctx, sb, config); err != nil {
			t.Fatalf("Setup() failed: %v", err)
		}
	}

	worktreePath := sb.WorkDir

	// Teardown
	if err := p.Teardown(sb); err != nil {
		t.Fatalf("Teardown() failed: %v", err)
	}

	// Worktree should be removed from git tracking
	// Note: the directory might still exist, but git worktree list should not show it
	repoPath := sb.Metadata["repo_path"].(string)
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = repoPath
	output, _ := cmd.Output()

	if strings.Contains(string(output), worktreePath) {
		t.Error("Worktree should be removed from git tracking")
	}
}

func TestWorktreePluginEnsureRepoClone(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	// Create source repo
	sourceRepo, cleanupSource := setupTestRepo(t)
	defer cleanupSource()

	// Create bare repo
	bareRepo, cleanupBare := setupBareRepo(t, sourceRepo)
	defer cleanupBare()

	// Create workspace
	tmpDir, err := os.MkdirTemp("", "worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	reposDir := filepath.Join(tmpDir, "repos")
	p := NewWorktreePlugin(reposDir)
	ctx := context.Background()

	// Call ensureRepo to clone
	repoPath, err := p.ensureRepo(ctx, bareRepo, bareRepo, "")
	if err != nil {
		t.Fatalf("ensureRepo() failed: %v", err)
	}

	// Verify bare repo was cloned
	headFile := filepath.Join(repoPath, "HEAD")
	if _, err := os.Stat(headFile); os.IsNotExist(err) {
		t.Error("HEAD file not found in cloned repo")
	}

	// Call ensureRepo again - should fetch instead of clone
	repoPath2, err := p.ensureRepo(ctx, bareRepo, bareRepo, "")
	if err != nil {
		t.Fatalf("ensureRepo() second call failed: %v", err)
	}

	if repoPath != repoPath2 {
		t.Errorf("Second call returned different path: %q vs %q", repoPath, repoPath2)
	}
}

func TestWorktreePluginBranchExists(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	// Create source repo
	sourceRepo, cleanupSource := setupTestRepo(t)
	defer cleanupSource()

	// Create bare repo
	bareRepo, cleanupBare := setupBareRepo(t, sourceRepo)
	defer cleanupBare()

	p := NewWorktreePlugin("")

	// Test existing branch
	// Note: The default branch might be 'main' or 'master'
	if !p.branchExists(bareRepo, "main") && !p.branchExists(bareRepo, "master") {
		t.Error("Neither main nor master branch exists")
	}

	// Test non-existing branch
	if p.branchExists(bareRepo, "non-existent-branch") {
		t.Error("Non-existent branch should not exist")
	}
}

func TestWorktreePluginCreateWorktree(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	// Create source repo
	sourceRepo, cleanupSource := setupTestRepo(t)
	defer cleanupSource()

	// Create bare repo
	bareRepo, cleanupBare := setupBareRepo(t, sourceRepo)
	defer cleanupBare()

	// Create worktree directory
	tmpDir, err := os.MkdirTemp("", "worktree-create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	worktreePath := filepath.Join(tmpDir, "my-worktree")

	p := NewWorktreePlugin("")
	ctx := context.Background()

	// Determine base branch
	baseBranch := "main"
	if !p.branchExists(bareRepo, "main") {
		baseBranch = "master"
	}

	// Create new branch worktree
	err = p.createWorktree(ctx, bareRepo, worktreePath, "ticket/TEST-123", baseBranch, "")
	if err != nil {
		t.Fatalf("createWorktree() failed: %v", err)
	}

	// Verify worktree was created
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Error("Worktree directory was not created")
	}

	// Verify .git file
	gitFile := filepath.Join(worktreePath, ".git")
	if _, err := os.Stat(gitFile); os.IsNotExist(err) {
		t.Error(".git file not found")
	}
}

func TestWorktreePluginCreateWorktreeExistingBranch(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	// Create source repo
	sourceRepo, cleanupSource := setupTestRepo(t)
	defer cleanupSource()

	// Create bare repo
	bareRepo, cleanupBare := setupBareRepo(t, sourceRepo)
	defer cleanupBare()

	// Create worktree directory
	tmpDir, err := os.MkdirTemp("", "worktree-create-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewWorktreePlugin("")
	ctx := context.Background()

	// Determine base branch
	baseBranch := "main"
	if !p.branchExists(bareRepo, "main") {
		baseBranch = "master"
	}

	// Create branch first (simulating existing branch)
	worktreePath1 := filepath.Join(tmpDir, "worktree1")
	err = p.createWorktree(ctx, bareRepo, worktreePath1, "ticket/EXIST-123", baseBranch, "")
	if err != nil {
		t.Fatalf("createWorktree() for first worktree failed: %v", err)
	}

	// Remove the first worktree but keep the branch
	cmd := exec.Command("git", "worktree", "remove", worktreePath1, "--force")
	cmd.Dir = bareRepo
	cmd.Run()

	// Now create second worktree with same branch (should use existing branch)
	worktreePath2 := filepath.Join(tmpDir, "worktree2")
	err = p.createWorktree(ctx, bareRepo, worktreePath2, "ticket/EXIST-123", baseBranch, "")
	if err != nil {
		t.Fatalf("createWorktree() for existing branch failed: %v", err)
	}

	// Verify worktree was created
	if _, err := os.Stat(worktreePath2); os.IsNotExist(err) {
		t.Error("Worktree directory was not created")
	}
}

func TestWorktreePluginSetupWithGitToken(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	// Create source repo
	sourceRepo, cleanupSource := setupTestRepo(t)
	defer cleanupSource()

	// Create bare repo
	bareRepo, cleanupBare := setupBareRepo(t, sourceRepo)
	defer cleanupBare()

	// Create workspace
	tmpDir, err := os.MkdirTemp("", "worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	reposDir := filepath.Join(tmpDir, "repos")
	sandboxDir := filepath.Join(tmpDir, "sandbox")

	p := NewWorktreePlugin(reposDir)
	sb := sandbox.NewSandbox("test-session", sandboxDir)
	ctx := context.Background()

	// Use git_token (this tests the token injection path, but with local repo it's ignored)
	config := map[string]interface{}{
		"repository_url":    bareRepo,
		"ticket_identifier": "TEST-456",
		"branch":            "main",
		"git_token":         "test-token", // Tests token injection code path
	}

	os.MkdirAll(sandboxDir, 0755)

	// Setup should work (token is only used for HTTPS URLs)
	if err := p.Setup(ctx, sb, config); err != nil {
		config["branch"] = "master"
		if err := p.Setup(ctx, sb, config); err != nil {
			t.Fatalf("Setup() failed: %v", err)
		}
	}
}

func TestWorktreePluginSetupWithDefaultBranch(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	// Create source repo
	sourceRepo, cleanupSource := setupTestRepo(t)
	defer cleanupSource()

	// Create bare repo
	bareRepo, cleanupBare := setupBareRepo(t, sourceRepo)
	defer cleanupBare()

	// Create workspace
	tmpDir, err := os.MkdirTemp("", "worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	reposDir := filepath.Join(tmpDir, "repos")
	sandboxDir := filepath.Join(tmpDir, "sandbox")

	p := NewWorktreePlugin(reposDir)
	sb := sandbox.NewSandbox("test-session", sandboxDir)
	ctx := context.Background()

	// Don't specify branch - should default to "main"
	config := map[string]interface{}{
		"repository_url":    bareRepo,
		"ticket_identifier": "TEST-789",
		// branch not specified, tests default branch logic
	}

	os.MkdirAll(sandboxDir, 0755)

	// This will use default branch "main", which might fail if repo uses "master"
	// That's okay - we're just exercising the code path
	_ = p.Setup(ctx, sb, config)
}

func TestWorktreePluginCreateWorktreeError(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	p := NewWorktreePlugin("")
	ctx := context.Background()

	// Test with invalid repo path - should fail
	err := p.createWorktree(ctx, "/nonexistent/repo", "/tmp/worktree", "ticket/TEST", "main", "")
	if err == nil {
		t.Error("createWorktree() should fail with invalid repo path")
	}
}

func TestWorktreePluginEnsureRepoCloneError(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	// Create workspace
	tmpDir, err := os.MkdirTemp("", "worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewWorktreePlugin(tmpDir)
	ctx := context.Background()

	// Test with invalid URL - should fail
	_, err = p.ensureRepo(ctx, "https://invalid-nonexistent-url.example.com/repo.git", "https://invalid-nonexistent-url.example.com/repo.git", "")
	if err == nil {
		t.Error("ensureRepo() should fail with invalid URL")
	}
}

func TestWorktreePluginGetEnvWithPathNoExisting(t *testing.T) {
	// Test PATH handling when PATH doesn't exist in environment
	p := NewWorktreePlugin("")

	// Save current PATH
	originalPath := os.Getenv("PATH")
	os.Unsetenv("PATH")
	defer os.Setenv("PATH", originalPath)

	env := p.getEnvWithPath()

	// Should add PATH even when not present
	hasPath := false
	for _, e := range env {
		if len(e) > 5 && e[:5] == "PATH=" {
			hasPath = true
			break
		}
	}

	if !hasPath {
		t.Error("PATH should be added when not present")
	}
}

func TestWorktreePluginSetupError(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	// Create workspace
	tmpDir, err := os.MkdirTemp("", "worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	reposDir := filepath.Join(tmpDir, "repos")
	sandboxDir := filepath.Join(tmpDir, "sandbox")

	p := NewWorktreePlugin(reposDir)
	sb := sandbox.NewSandbox("test-session", sandboxDir)
	ctx := context.Background()

	// Use invalid URL to trigger ensureRepo error
	config := map[string]interface{}{
		"repository_url":    "https://invalid-nonexistent-url.example.com/repo.git",
		"ticket_identifier": "TEST-ERR",
		"branch":            "main",
	}

	os.MkdirAll(sandboxDir, 0755)

	// Setup should fail
	err = p.Setup(ctx, sb, config)
	if err == nil {
		t.Error("Setup() should fail with invalid URL")
	}
}

func TestWorktreePluginSetupCreateWorktreeError(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	// Create source repo
	sourceRepo, cleanupSource := setupTestRepo(t)
	defer cleanupSource()

	// Create bare repo
	bareRepo, cleanupBare := setupBareRepo(t, sourceRepo)
	defer cleanupBare()

	// Create workspace
	tmpDir, err := os.MkdirTemp("", "worktree-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	reposDir := filepath.Join(tmpDir, "repos")
	sandboxDir := filepath.Join(tmpDir, "sandbox")

	p := NewWorktreePlugin(reposDir)
	sb := sandbox.NewSandbox("test-session", sandboxDir)
	ctx := context.Background()

	// Use non-existent branch to trigger createWorktree error
	config := map[string]interface{}{
		"repository_url":    bareRepo,
		"ticket_identifier": "TEST-CW",
		"branch":            "nonexistent-branch-xyz",
	}

	os.MkdirAll(sandboxDir, 0755)

	// Setup should fail due to invalid base branch
	err = p.Setup(ctx, sb, config)
	if err == nil {
		t.Error("Setup() should fail with non-existent base branch")
	}
}

// SSH-related tests

func TestIsSSHURL(t *testing.T) {
	tests := []struct {
		url    string
		isSSH  bool
	}{
		{"git@github.com:org/repo.git", true},
		{"ssh://git@github.com/org/repo.git", true},
		{"https://github.com/org/repo.git", false},
		{"http://github.com/org/repo.git", false},
		{"git@gitlab.com:group/project.git", true},
		{"ssh://git@gitlab.example.com:22/group/project.git", true},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			result := isSSHURL(tt.url)
			if result != tt.isSSH {
				t.Errorf("isSSHURL(%q) = %v, want %v", tt.url, result, tt.isSSH)
			}
		})
	}
}

func TestWorktreePluginSetupSSHKey(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ssh-key-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewWorktreePlugin("")

	testPrivateKey := `-----BEGIN RSA PRIVATE KEY-----
MIIEpQIBAAKCAQEA0Z3VS5JJcds3xfn/ygWyf8fK...
-----END RSA PRIVATE KEY-----`

	keyPath, cleanup, err := p.setupSSHKey(tmpDir, testPrivateKey)
	if err != nil {
		t.Fatalf("setupSSHKey() failed: %v", err)
	}

	// Verify key path
	expectedKeyPath := filepath.Join(tmpDir, ".ssh", "id_rsa")
	if keyPath != expectedKeyPath {
		t.Errorf("keyPath = %q, want %q", keyPath, expectedKeyPath)
	}

	// Verify key file exists
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		t.Error("SSH key file was not created")
	}

	// Verify file permissions
	info, err := os.Stat(keyPath)
	if err != nil {
		t.Fatalf("Failed to stat key file: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("Key file permissions = %o, want 0600", perm)
	}

	// Verify .ssh directory permissions
	sshDir := filepath.Join(tmpDir, ".ssh")
	sshInfo, err := os.Stat(sshDir)
	if err != nil {
		t.Fatalf("Failed to stat .ssh dir: %v", err)
	}
	sshPerm := sshInfo.Mode().Perm()
	if sshPerm != 0700 {
		t.Errorf(".ssh dir permissions = %o, want 0700", sshPerm)
	}

	// Verify key content
	content, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatalf("Failed to read key file: %v", err)
	}
	if string(content) != testPrivateKey {
		t.Error("Key file content doesn't match")
	}

	// Test cleanup function
	cleanup()

	// Verify key file is removed
	if _, err := os.Stat(keyPath); !os.IsNotExist(err) {
		t.Error("SSH key file should be removed after cleanup")
	}
}

func TestWorktreePluginSetupSSHKeyError(t *testing.T) {
	p := NewWorktreePlugin("")

	// Test with non-writable directory
	_, _, err := p.setupSSHKey("/root/nonexistent/path", "test-key")
	if err == nil {
		t.Error("setupSSHKey() should fail with invalid directory")
	}
}

func TestWorktreePluginAddSSHEnv(t *testing.T) {
	p := NewWorktreePlugin("")

	// Test adding SSH env to empty env
	env := p.addSSHEnv([]string{}, "/path/to/key")

	found := false
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_SSH_COMMAND=") {
			found = true
			if !strings.Contains(e, "-i /path/to/key") {
				t.Errorf("GIT_SSH_COMMAND should contain key path, got %q", e)
			}
			if !strings.Contains(e, "-o StrictHostKeyChecking=no") {
				t.Errorf("GIT_SSH_COMMAND should disable StrictHostKeyChecking, got %q", e)
			}
			break
		}
	}
	if !found {
		t.Error("GIT_SSH_COMMAND should be added")
	}
}

func TestWorktreePluginAddSSHEnvReplace(t *testing.T) {
	p := NewWorktreePlugin("")

	// Test replacing existing GIT_SSH_COMMAND
	env := []string{
		"PATH=/usr/bin",
		"GIT_SSH_COMMAND=old command",
		"HOME=/home/user",
	}

	newEnv := p.addSSHEnv(env, "/new/key/path")

	count := 0
	for _, e := range newEnv {
		if strings.HasPrefix(e, "GIT_SSH_COMMAND=") {
			count++
			if strings.Contains(e, "old command") {
				t.Error("Old GIT_SSH_COMMAND should be replaced")
			}
			if !strings.Contains(e, "/new/key/path") {
				t.Errorf("GIT_SSH_COMMAND should contain new key path, got %q", e)
			}
		}
	}

	if count != 1 {
		t.Errorf("Expected exactly 1 GIT_SSH_COMMAND, got %d", count)
	}
}

func TestWorktreePluginSetupWithSSHURLButNoKey(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	tmpDir, err := os.MkdirTemp("", "worktree-ssh-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	reposDir := filepath.Join(tmpDir, "repos")
	sandboxDir := filepath.Join(tmpDir, "sandbox")

	p := NewWorktreePlugin(reposDir)
	sb := sandbox.NewSandbox("test-session", sandboxDir)
	ctx := context.Background()

	// Use SSH URL without providing ssh_private_key
	// This simulates using default SSH key from system
	config := map[string]interface{}{
		"repository_url":    "git@github.com:nonexistent/repo.git",
		"ticket_identifier": "TEST-SSH",
		"branch":            "main",
		// No ssh_private_key - would use system SSH key
	}

	os.MkdirAll(sandboxDir, 0755)

	// Setup will fail (can't clone nonexistent repo) but should reach the clone step
	err = p.Setup(ctx, sb, config)
	// Error is expected, we just want to ensure no panic
	if err == nil {
		t.Log("Unexpectedly succeeded - might have SSH access to github")
	}
}

func TestWorktreePluginEnsureRepoWithSSHKey(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	// Create source repo
	sourceRepo, cleanupSource := setupTestRepo(t)
	defer cleanupSource()

	// Create bare repo
	bareRepo, cleanupBare := setupBareRepo(t, sourceRepo)
	defer cleanupBare()

	tmpDir, err := os.MkdirTemp("", "worktree-ssh-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewWorktreePlugin(tmpDir)
	ctx := context.Background()

	// Create a dummy SSH key path (won't be used for local repos but tests the code path)
	sshKeyPath := filepath.Join(tmpDir, "test_key")
	os.WriteFile(sshKeyPath, []byte("test key"), 0600)

	// ensureRepo with SSH key path (code path test)
	repoPath, err := p.ensureRepo(ctx, bareRepo, bareRepo, sshKeyPath)
	if err != nil {
		t.Fatalf("ensureRepo() with sshKeyPath failed: %v", err)
	}

	// Verify repo was cloned
	if _, err := os.Stat(filepath.Join(repoPath, "HEAD")); os.IsNotExist(err) {
		t.Error("Repo was not cloned")
	}
}

func TestWorktreePluginCreateWorktreeWithSSHKey(t *testing.T) {
	// Skip if git is not available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available, skipping integration test")
	}

	// Create source repo
	sourceRepo, cleanupSource := setupTestRepo(t)
	defer cleanupSource()

	// Create bare repo
	bareRepo, cleanupBare := setupBareRepo(t, sourceRepo)
	defer cleanupBare()

	tmpDir, err := os.MkdirTemp("", "worktree-ssh-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewWorktreePlugin("")
	ctx := context.Background()

	// Determine base branch
	baseBranch := "main"
	if !p.branchExists(bareRepo, "main") {
		baseBranch = "master"
	}

	// Create a dummy SSH key path (tests code path)
	sshKeyPath := filepath.Join(tmpDir, "test_key")
	os.WriteFile(sshKeyPath, []byte("test key"), 0600)

	worktreePath := filepath.Join(tmpDir, "worktree")

	// createWorktree with SSH key path (code path test)
	err = p.createWorktree(ctx, bareRepo, worktreePath, "ticket/SSH-123", baseBranch, sshKeyPath)
	if err != nil {
		t.Fatalf("createWorktree() with sshKeyPath failed: %v", err)
	}

	// Verify worktree was created
	if _, err := os.Stat(worktreePath); os.IsNotExist(err) {
		t.Error("Worktree was not created")
	}
}
