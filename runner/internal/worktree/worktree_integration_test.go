package worktree

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestRepo creates a temporary git repository for testing.
// Returns repoPath, cleanup function, and error.
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	tmpDir := t.TempDir()
	repoPath := filepath.Join(tmpDir, "repo")

	// Create directory
	if err := os.MkdirAll(repoPath, 0755); err != nil {
		t.Fatalf("failed to create repo dir: %v", err)
	}

	// Initialize git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\noutput: %s", err, output)
	}

	// Configure git user (required for commits)
	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = repoPath
	cmd.Run()

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = repoPath
	cmd.Run()

	// Create initial commit (required for worktrees)
	readmePath := filepath.Join(repoPath, "README.md")
	if err := os.WriteFile(readmePath, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("failed to create README: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = repoPath
	cmd.Run()

	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git commit failed: %v\noutput: %s", err, output)
	}

	// Rename default branch to main if needed
	cmd = exec.Command("git", "branch", "-M", "main")
	cmd.Dir = repoPath
	cmd.Run()

	cleanup := func() {
		// Cleanup worktrees first (if any)
		cmd := exec.Command("git", "worktree", "prune")
		cmd.Dir = repoPath
		cmd.Run()
	}

	return repoPath, cleanup
}

func TestCreateWorktreeNewBranch(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	worktreesDir := filepath.Join(filepath.Dir(repoPath), "worktrees")
	service := New(repoPath, worktreesDir, "main")

	// Create worktree with new branch
	path, branch, err := service.Create("TICKET-100", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify path
	expectedPath := filepath.Join(worktreesDir, "TICKET-100")
	if path != expectedPath {
		t.Errorf("path: got %v, want %v", path, expectedPath)
	}

	// Verify branch name
	if branch != "ticket/TICKET-100" {
		t.Errorf("branch: got %v, want ticket/TICKET-100", branch)
	}

	// Verify worktree actually exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("worktree directory should exist")
	}

	// Verify .git file exists in worktree
	gitFile := filepath.Join(path, ".git")
	if _, err := os.Stat(gitFile); os.IsNotExist(err) {
		t.Error(".git file should exist in worktree")
	}
}

func TestCreateWorktreeWithSuffix(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	worktreesDir := filepath.Join(filepath.Dir(repoPath), "worktrees")
	service := New(repoPath, worktreesDir, "main")

	// Create worktree with suffix
	path, branch, err := service.Create("TICKET-200", "v1")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	expectedPath := filepath.Join(worktreesDir, "TICKET-200-v1")
	if path != expectedPath {
		t.Errorf("path: got %v, want %v", path, expectedPath)
	}

	if branch != "ticket/TICKET-200-v1" {
		t.Errorf("branch: got %v, want ticket/TICKET-200-v1", branch)
	}
}

func TestCreateWorktreeExistingBranch(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	worktreesDir := filepath.Join(filepath.Dir(repoPath), "worktrees")
	service := New(repoPath, worktreesDir, "main")

	// First, create a branch manually
	cmd := exec.Command("git", "branch", "ticket/TICKET-300")
	cmd.Dir = repoPath
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git branch failed: %v\noutput: %s", err, output)
	}

	// Now create worktree for existing branch
	path, branch, err := service.Create("TICKET-300", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	expectedPath := filepath.Join(worktreesDir, "TICKET-300")
	if path != expectedPath {
		t.Errorf("path: got %v, want %v", path, expectedPath)
	}

	if branch != "ticket/TICKET-300" {
		t.Errorf("branch: got %v, want ticket/TICKET-300", branch)
	}

	// Verify worktree exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("worktree directory should exist")
	}
}

func TestRemoveWorktree(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	worktreesDir := filepath.Join(filepath.Dir(repoPath), "worktrees")
	service := New(repoPath, worktreesDir, "main")

	// Create a worktree first
	path, _, err := service.Create("TICKET-400", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Verify it exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("worktree should exist before removal")
	}

	// Remove the worktree
	err = service.Remove("TICKET-400")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Verify it no longer exists
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("worktree should not exist after removal")
	}
}

func TestListWorktrees(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	worktreesDir := filepath.Join(filepath.Dir(repoPath), "worktrees")
	service := New(repoPath, worktreesDir, "main")

	// Create multiple worktrees
	identifiers := []string{"TICKET-501", "TICKET-502", "TICKET-503"}
	for _, id := range identifiers {
		_, _, err := service.Create(id, "")
		if err != nil {
			t.Fatalf("Create %s failed: %v", id, err)
		}
	}

	// List worktrees
	worktrees, err := service.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Should have 3 worktrees
	if len(worktrees) != 3 {
		t.Errorf("worktrees count: got %d, want 3", len(worktrees))
	}

	// Verify all identifiers are present
	found := make(map[string]bool)
	for _, wt := range worktrees {
		found[wt.Identifier] = true
		// Verify branch format
		if !strings.HasPrefix(wt.Branch, "ticket/") {
			t.Errorf("branch should start with ticket/: %s", wt.Branch)
		}
	}

	for _, id := range identifiers {
		if !found[id] {
			t.Errorf("worktree %s not found in list", id)
		}
	}
}

func TestListWorktreesEmpty(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	worktreesDir := filepath.Join(filepath.Dir(repoPath), "worktrees")
	service := New(repoPath, worktreesDir, "main")

	// List worktrees (should be empty since main repo worktree is not in worktreesDir)
	worktrees, err := service.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(worktrees) != 0 {
		t.Errorf("worktrees count: got %d, want 0", len(worktrees))
	}
}

func TestBranchExists(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	worktreesDir := filepath.Join(filepath.Dir(repoPath), "worktrees")
	service := New(repoPath, worktreesDir, "main")

	// main branch should exist
	if !service.branchExists("main") {
		t.Error("main branch should exist")
	}

	// Non-existent branch should not exist
	if service.branchExists("nonexistent-branch") {
		t.Error("nonexistent-branch should not exist")
	}

	// Create a branch and check
	cmd := exec.Command("git", "branch", "test-branch")
	cmd.Dir = repoPath
	cmd.Run()

	if !service.branchExists("test-branch") {
		t.Error("test-branch should exist after creation")
	}
}

func TestCreateWorktreeInvalidBaseBranch(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	worktreesDir := filepath.Join(filepath.Dir(repoPath), "worktrees")
	// Use non-existent base branch
	service := New(repoPath, worktreesDir, "nonexistent-base")

	// This should fail because base branch doesn't exist
	_, _, err := service.Create("TICKET-600", "")
	if err == nil {
		t.Error("Create should fail with non-existent base branch")
	}
}

func TestCreateWorktreeMultipleSuffixes(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	worktreesDir := filepath.Join(filepath.Dir(repoPath), "worktrees")
	service := New(repoPath, worktreesDir, "main")

	// Create multiple worktrees for the same ticket with different suffixes
	suffixes := []string{"session1", "session2", "session3"}
	for _, suffix := range suffixes {
		path, branch, err := service.Create("TICKET-700", suffix)
		if err != nil {
			t.Fatalf("Create with suffix %s failed: %v", suffix, err)
		}

		expectedPath := filepath.Join(worktreesDir, "TICKET-700-"+suffix)
		if path != expectedPath {
			t.Errorf("path for suffix %s: got %v, want %v", suffix, path, expectedPath)
		}

		expectedBranch := "ticket/TICKET-700-" + suffix
		if branch != expectedBranch {
			t.Errorf("branch for suffix %s: got %v, want %v", suffix, branch, expectedBranch)
		}
	}

	// List should show all 3
	worktrees, err := service.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(worktrees) != 3 {
		t.Errorf("worktrees count: got %d, want 3", len(worktrees))
	}
}

func TestGetPathAfterCreate(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	worktreesDir := filepath.Join(filepath.Dir(repoPath), "worktrees")
	service := New(repoPath, worktreesDir, "main")

	// GetPath before create should return empty
	path := service.GetPath("TICKET-800")
	if path != "" {
		t.Errorf("GetPath before create: got %v, want empty", path)
	}

	// Create worktree
	createdPath, _, err := service.Create("TICKET-800", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// GetPath after create should return the path
	path = service.GetPath("TICKET-800")
	if path != createdPath {
		t.Errorf("GetPath after create: got %v, want %v", path, createdPath)
	}
}

func TestRemoveAndRecreate(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	worktreesDir := filepath.Join(filepath.Dir(repoPath), "worktrees")
	service := New(repoPath, worktreesDir, "main")

	// Create
	path1, _, err := service.Create("TICKET-900", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Remove
	err = service.Remove("TICKET-900")
	if err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Recreate - should attach to existing branch
	path2, branch, err := service.Create("TICKET-900", "")
	if err != nil {
		t.Fatalf("Recreate failed: %v", err)
	}

	if path1 != path2 {
		t.Errorf("recreated path differs: got %v, want %v", path2, path1)
	}

	if branch != "ticket/TICKET-900" {
		t.Errorf("branch: got %v, want ticket/TICKET-900", branch)
	}
}

// Test edge cases

func TestCreateWorktreeDirectoryExists(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	worktreesDir := filepath.Join(filepath.Dir(repoPath), "worktrees")
	service := New(repoPath, worktreesDir, "main")

	// Pre-create the worktrees directory
	os.MkdirAll(worktreesDir, 0755)

	// Create worktree should still work
	_, _, err := service.Create("TICKET-1000", "")
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
}

func TestListWorktreesWithMainRepo(t *testing.T) {
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Set worktreesDir to be parent of repo (so main repo is NOT in worktreesDir)
	worktreesDir := filepath.Join(filepath.Dir(repoPath), "worktrees")
	service := New(repoPath, worktreesDir, "main")

	// Create one worktree in the proper worktrees dir
	service.Create("TICKET-1100", "")

	worktrees, err := service.List()
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Should only list worktrees in worktreesDir, not the main repo
	if len(worktrees) != 1 {
		t.Errorf("worktrees count: got %d, want 1 (main repo should not be included)", len(worktrees))
	}
}
