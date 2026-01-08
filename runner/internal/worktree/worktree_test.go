package worktree

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNew(t *testing.T) {
	service := New("/repo", "/worktrees", "main")

	if service == nil {
		t.Fatal("New returned nil")
	}

	if service.repoPath != "/repo" {
		t.Errorf("repoPath: got %v, want /repo", service.repoPath)
	}

	if service.worktreesDir != "/worktrees" {
		t.Errorf("worktreesDir: got %v, want /worktrees", service.worktreesDir)
	}

	if service.baseBranch != "main" {
		t.Errorf("baseBranch: got %v, want main", service.baseBranch)
	}
}

func TestNewDefaultBranch(t *testing.T) {
	service := New("/repo", "/worktrees", "")

	if service == nil {
		t.Fatal("New returned nil")
	}

	if service.baseBranch != "main" {
		t.Errorf("baseBranch: got %v, want main (default)", service.baseBranch)
	}
}

func TestNewEmptyRepoPath(t *testing.T) {
	service := New("", "/worktrees", "main")

	if service != nil {
		t.Error("New should return nil for empty repoPath")
	}
}

func TestNewEmptyWorktreesDir(t *testing.T) {
	service := New("/repo", "", "main")

	if service != nil {
		t.Error("New should return nil for empty worktreesDir")
	}
}

func TestGetPath(t *testing.T) {
	tmpDir := t.TempDir()
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	service := New(tmpDir, worktreesDir, "main")

	// Non-existent worktree should return empty string
	path := service.GetPath("nonexistent")
	if path != "" {
		t.Errorf("GetPath for nonexistent: got %v, want empty", path)
	}
}

func TestGetPathExists(t *testing.T) {
	tmpDir := t.TempDir()
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	service := New(tmpDir, worktreesDir, "main")

	// Create a fake worktree directory with .git file
	worktreePath := filepath.Join(worktreesDir, "TICKET-123")
	os.MkdirAll(worktreePath, 0755)
	os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /some/path"), 0644)

	path := service.GetPath("TICKET-123")
	if path != worktreePath {
		t.Errorf("GetPath: got %v, want %v", path, worktreePath)
	}
}

func TestGetRepositoryPath(t *testing.T) {
	service := New("/repo", "/worktrees", "main")

	if service.GetRepositoryPath() != "/repo" {
		t.Errorf("GetRepositoryPath: got %v, want /repo", service.GetRepositoryPath())
	}
}

func TestCreateEmptyIdentifier(t *testing.T) {
	tmpDir := t.TempDir()
	service := New(tmpDir, filepath.Join(tmpDir, "worktrees"), "main")

	_, _, err := service.Create("", "")
	if err == nil {
		t.Error("Create should error for empty identifier")
	}
}

func TestWorktreeExistsReturnsExisting(t *testing.T) {
	tmpDir := t.TempDir()
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	service := New(tmpDir, worktreesDir, "main")

	// Create a fake existing worktree
	worktreePath := filepath.Join(worktreesDir, "TICKET-123")
	os.MkdirAll(worktreePath, 0755)
	os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /some/path"), 0644)

	path, branch, err := service.Create("TICKET-123", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path != worktreePath {
		t.Errorf("path: got %v, want %v", path, worktreePath)
	}

	if branch != "ticket/TICKET-123" {
		t.Errorf("branch: got %v, want ticket/TICKET-123", branch)
	}
}

func TestWorktreeExistsWithSuffix(t *testing.T) {
	tmpDir := t.TempDir()
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	service := New(tmpDir, worktreesDir, "main")

	// Create a fake existing worktree with suffix
	worktreePath := filepath.Join(worktreesDir, "TICKET-123-v1")
	os.MkdirAll(worktreePath, 0755)
	os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /some/path"), 0644)

	path, branch, err := service.Create("TICKET-123", "v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if path != worktreePath {
		t.Errorf("path: got %v, want %v", path, worktreePath)
	}

	if branch != "ticket/TICKET-123-v1" {
		t.Errorf("branch: got %v, want ticket/TICKET-123-v1", branch)
	}
}

func TestRemoveNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	service := New(tmpDir, filepath.Join(tmpDir, "worktrees"), "main")

	// Should not error for non-existent worktree
	err := service.Remove("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestWorktreeInfoStruct(t *testing.T) {
	info := WorktreeInfo{
		Path:       "/workspace/worktrees/TICKET-123",
		Branch:     "ticket/TICKET-123",
		Identifier: "TICKET-123",
	}

	if info.Path != "/workspace/worktrees/TICKET-123" {
		t.Errorf("Path: got %v, want /workspace/worktrees/TICKET-123", info.Path)
	}

	if info.Branch != "ticket/TICKET-123" {
		t.Errorf("Branch: got %v, want ticket/TICKET-123", info.Branch)
	}

	if info.Identifier != "TICKET-123" {
		t.Errorf("Identifier: got %v, want TICKET-123", info.Identifier)
	}
}

func TestWorktreeExists(t *testing.T) {
	tmpDir := t.TempDir()
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	service := New(tmpDir, worktreesDir, "main")

	// Non-existent path
	if service.worktreeExists("/nonexistent") {
		t.Error("worktreeExists should return false for non-existent path")
	}

	// Create directory without .git file
	noGitDir := filepath.Join(worktreesDir, "no-git")
	os.MkdirAll(noGitDir, 0755)

	if service.worktreeExists(noGitDir) {
		t.Error("worktreeExists should return false for directory without .git")
	}

	// Create directory with .git file
	withGitDir := filepath.Join(worktreesDir, "with-git")
	os.MkdirAll(withGitDir, 0755)
	os.WriteFile(filepath.Join(withGitDir, ".git"), []byte("gitdir: /some/path"), 0644)

	if !service.worktreeExists(withGitDir) {
		t.Error("worktreeExists should return true for directory with .git")
	}
}

func TestGetEnvWithPath(t *testing.T) {
	service := New("/repo", "/worktrees", "main")

	env := service.getEnvWithPath()

	// Should have PATH in environment
	pathFound := false
	for _, e := range env {
		if len(e) > 5 && e[:5] == "PATH=" {
			pathFound = true
			break
		}
	}

	if !pathFound {
		t.Error("PATH should be in environment")
	}
}

// --- Benchmark Tests ---

func BenchmarkWorktreeExists(b *testing.B) {
	tmpDir := b.TempDir()
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	service := New(tmpDir, worktreesDir, "main")

	// Create a directory with .git
	testDir := filepath.Join(worktreesDir, "test")
	os.MkdirAll(testDir, 0755)
	os.WriteFile(filepath.Join(testDir, ".git"), []byte("gitdir: /some/path"), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.worktreeExists(testDir)
	}
}

// --- Additional Tests for Coverage ---

func TestNewWithAllEmptyParams(t *testing.T) {
	service := New("", "", "")

	if service != nil {
		t.Error("New should return nil for all empty params")
	}
}

func TestServiceStruct(t *testing.T) {
	service := &Service{
		repoPath:     "/repo",
		worktreesDir: "/worktrees",
		baseBranch:   "develop",
	}

	if service.repoPath != "/repo" {
		t.Errorf("repoPath: got %v, want /repo", service.repoPath)
	}

	if service.worktreesDir != "/worktrees" {
		t.Errorf("worktreesDir: got %v, want /worktrees", service.worktreesDir)
	}

	if service.baseBranch != "develop" {
		t.Errorf("baseBranch: got %v, want develop", service.baseBranch)
	}
}

func TestGetPathMultiple(t *testing.T) {
	tmpDir := t.TempDir()
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	service := New(tmpDir, worktreesDir, "main")

	// Create multiple fake worktrees
	identifiers := []string{"TICKET-001", "TICKET-002", "TICKET-003"}
	for _, id := range identifiers {
		worktreePath := filepath.Join(worktreesDir, id)
		os.MkdirAll(worktreePath, 0755)
		os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /some/path"), 0644)
	}

	// Test each one
	for _, id := range identifiers {
		path := service.GetPath(id)
		expected := filepath.Join(worktreesDir, id)
		if path != expected {
			t.Errorf("GetPath(%s): got %v, want %v", id, path, expected)
		}
	}
}

func TestCreateWithDifferentSuffixes(t *testing.T) {
	tmpDir := t.TempDir()
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	service := New(tmpDir, worktreesDir, "main")

	suffixes := []string{"v1", "v2", "session-abc123"}
	for _, suffix := range suffixes {
		// Create fake worktree with suffix
		worktreeName := "TICKET-123-" + suffix
		worktreePath := filepath.Join(worktreesDir, worktreeName)
		os.MkdirAll(worktreePath, 0755)
		os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /some/path"), 0644)

		path, branch, err := service.Create("TICKET-123", suffix)
		if err != nil {
			t.Errorf("Create with suffix %s: unexpected error: %v", suffix, err)
			continue
		}

		if path != worktreePath {
			t.Errorf("Create with suffix %s: path got %v, want %v", suffix, path, worktreePath)
		}

		expectedBranch := "ticket/TICKET-123-" + suffix
		if branch != expectedBranch {
			t.Errorf("Create with suffix %s: branch got %v, want %v", suffix, branch, expectedBranch)
		}
	}
}

func TestWorktreeExistsWithSubdirectories(t *testing.T) {
	tmpDir := t.TempDir()
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	service := New(tmpDir, worktreesDir, "main")

	// Create nested directory structure with .git at proper level
	nestedPath := filepath.Join(worktreesDir, "project", "subdir")
	os.MkdirAll(nestedPath, 0755)

	// .git should be at project level, not subdir
	projectPath := filepath.Join(worktreesDir, "project")
	os.WriteFile(filepath.Join(projectPath, ".git"), []byte("gitdir: /some/path"), 0644)

	if !service.worktreeExists(projectPath) {
		t.Error("worktreeExists should return true for project with .git")
	}

	if service.worktreeExists(nestedPath) {
		t.Error("worktreeExists should return false for nested dir without .git")
	}
}

func TestGetEnvWithPathPreservesExistingVars(t *testing.T) {
	service := New("/repo", "/worktrees", "main")

	env := service.getEnvWithPath()

	// Check that common environment variables are preserved
	// We can't guarantee which vars exist, but env should not be empty
	if len(env) == 0 {
		t.Error("env should not be empty")
	}

	// Check PATH is present and not empty
	pathFound := false
	for _, e := range env {
		if len(e) > 5 && e[:5] == "PATH=" {
			pathFound = true
			pathValue := e[5:]
			if pathValue == "" {
				t.Error("PATH value should not be empty")
			}
			break
		}
	}

	if !pathFound {
		t.Error("PATH should be in environment")
	}
}

func TestWorktreeInfoMultipleInstances(t *testing.T) {
	infos := []WorktreeInfo{
		{Path: "/path/1", Branch: "branch/1", Identifier: "ID-1"},
		{Path: "/path/2", Branch: "branch/2", Identifier: "ID-2"},
		{Path: "/path/3", Branch: "branch/3", Identifier: "ID-3"},
	}

	for i, info := range infos {
		if info.Path != "/path/"+string(rune('1'+i)) {
			t.Errorf("info[%d].Path: got %v", i, info.Path)
		}
	}
}

func TestRemoveWithExistingButNotRealWorktree(t *testing.T) {
	tmpDir := t.TempDir()
	worktreesDir := filepath.Join(tmpDir, "worktrees")

	// Create a directory but not inside a real git repo
	os.MkdirAll(worktreesDir, 0755)

	service := New(tmpDir, worktreesDir, "main")

	// This should not error since the worktree doesn't exist (no .git file)
	err := service.Remove("fake-ticket")
	if err != nil {
		t.Errorf("Remove for non-existent should not error: %v", err)
	}
}

func TestGetPathWithSpecialCharacters(t *testing.T) {
	tmpDir := t.TempDir()
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	service := New(tmpDir, worktreesDir, "main")

	// Create worktree with special identifier
	specialID := "PROJ-123_feature"
	worktreePath := filepath.Join(worktreesDir, specialID)
	os.MkdirAll(worktreePath, 0755)
	os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /some/path"), 0644)

	path := service.GetPath(specialID)
	if path != worktreePath {
		t.Errorf("GetPath: got %v, want %v", path, worktreePath)
	}
}

// Benchmark for GetPath
func BenchmarkGetPath(b *testing.B) {
	tmpDir := b.TempDir()
	worktreesDir := filepath.Join(tmpDir, "worktrees")
	service := New(tmpDir, worktreesDir, "main")

	// Create a fake worktree
	worktreePath := filepath.Join(worktreesDir, "TICKET-123")
	os.MkdirAll(worktreePath, 0755)
	os.WriteFile(filepath.Join(worktreePath, ".git"), []byte("gitdir: /some/path"), 0644)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.GetPath("TICKET-123")
	}
}

// Benchmark for getEnvWithPath
func BenchmarkGetEnvWithPath(b *testing.B) {
	service := New("/repo", "/worktrees", "main")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.getEnvWithPath()
	}
}
