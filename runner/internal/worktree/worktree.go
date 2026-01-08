// Package worktree provides git worktree management for DevPod sessions.
package worktree

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Service manages git worktrees for ticket-based development sessions.
type Service struct {
	repoPath     string // Path to the main git repository
	worktreesDir string // Directory where worktrees are created
	baseBranch   string // Base branch for new worktrees (e.g., "main")
}

// WorktreeInfo contains information about a worktree.
type WorktreeInfo struct {
	Path       string // Full path to the worktree
	Branch     string // Branch name
	Identifier string // Ticket identifier (e.g., "TBD-123")
}

// New creates a new WorktreeService.
// Returns nil if repoPath or worktreesDir is empty.
func New(repoPath, worktreesDir, baseBranch string) *Service {
	if repoPath == "" || worktreesDir == "" {
		return nil
	}

	if baseBranch == "" {
		baseBranch = "main"
	}

	return &Service{
		repoPath:     repoPath,
		worktreesDir: worktreesDir,
		baseBranch:   baseBranch,
	}
}

// Create creates or attaches a worktree for the given ticket identifier.
// Returns (worktreePath, branchName, error).
//
// Branch naming: ticket/{identifier} or ticket/{identifier}-{suffix}
// Worktree path: {worktreesDir}/{identifier} or {worktreesDir}/{identifier}-{suffix}
//
// If suffix is provided, it's appended to both branch and path names to support
// multiple worktrees for the same ticket (e.g., parallel agent sessions).
//
// If the branch already exists, the worktree is attached to it.
// If the branch doesn't exist, it's created from the base branch.
func (s *Service) Create(ticketIdentifier, suffix string) (string, string, error) {
	if ticketIdentifier == "" {
		return "", "", fmt.Errorf("ticket identifier is required")
	}

	// Build names with optional suffix for multi-instance support
	var branchName, worktreeName string
	if suffix != "" {
		worktreeName = fmt.Sprintf("%s-%s", ticketIdentifier, suffix)
		branchName = fmt.Sprintf("ticket/%s-%s", ticketIdentifier, suffix)
	} else {
		worktreeName = ticketIdentifier
		branchName = fmt.Sprintf("ticket/%s", ticketIdentifier)
	}
	worktreePath := filepath.Join(s.worktreesDir, worktreeName)

	// Ensure worktrees directory exists
	if err := os.MkdirAll(s.worktreesDir, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create worktrees directory: %w", err)
	}

	// Check if worktree already exists
	if s.worktreeExists(worktreePath) {
		log.Printf("[worktree] Worktree already exists, reusing: identifier=%s, path=%s, branch=%s",
			ticketIdentifier, worktreePath, branchName)
		return worktreePath, branchName, nil
	}

	// Check if branch exists
	branchExists := s.branchExists(branchName)

	var cmd *exec.Cmd
	if branchExists {
		// Branch exists, attach worktree to it
		log.Printf("[worktree] Attaching worktree to existing branch: identifier=%s, branch=%s",
			ticketIdentifier, branchName)
		cmd = exec.Command("git", "worktree", "add", worktreePath, branchName)
	} else {
		// Create new branch from base
		log.Printf("[worktree] Creating worktree with new branch: identifier=%s, branch=%s, base=%s",
			ticketIdentifier, branchName, s.baseBranch)
		cmd = exec.Command("git", "worktree", "add", "-b", branchName, worktreePath, s.baseBranch)
	}

	cmd.Dir = s.repoPath
	cmd.Env = s.getEnvWithPath()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", "", fmt.Errorf("git worktree add failed: %w\noutput: %s", err, string(output))
	}

	log.Printf("[worktree] Worktree created successfully: identifier=%s, path=%s, branch=%s",
		ticketIdentifier, worktreePath, branchName)

	return worktreePath, branchName, nil
}

// Remove removes a worktree by ticket identifier.
func (s *Service) Remove(ticketIdentifier string) error {
	worktreePath := filepath.Join(s.worktreesDir, ticketIdentifier)

	if !s.worktreeExists(worktreePath) {
		return nil // Already removed
	}

	cmd := exec.Command("git", "worktree", "remove", worktreePath, "--force")
	cmd.Dir = s.repoPath
	cmd.Env = s.getEnvWithPath()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree remove failed: %w\noutput: %s", err, string(output))
	}

	log.Printf("[worktree] Worktree removed: identifier=%s", ticketIdentifier)
	return nil
}

// List returns all worktrees in the worktrees directory.
func (s *Service) List() ([]WorktreeInfo, error) {
	cmd := exec.Command("git", "worktree", "list", "--porcelain")
	cmd.Dir = s.repoPath
	cmd.Env = s.getEnvWithPath()
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git worktree list failed: %w", err)
	}

	// Resolve symlinks in worktreesDir for proper comparison
	// (e.g., on macOS /var/folders is a symlink to /private/var/folders)
	resolvedWorktreesDir, err := filepath.EvalSymlinks(s.worktreesDir)
	if err != nil {
		// If symlink resolution fails, use original path
		resolvedWorktreesDir = s.worktreesDir
	}

	var worktrees []WorktreeInfo
	lines := strings.Split(string(output), "\n")

	var current WorktreeInfo
	for _, line := range lines {
		if strings.HasPrefix(line, "worktree ") {
			path := strings.TrimPrefix(line, "worktree ")
			// Only include worktrees in our worktrees directory
			// Compare using resolved paths to handle symlinks
			if strings.HasPrefix(path, resolvedWorktreesDir) || strings.HasPrefix(path, s.worktreesDir) {
				current.Path = path
				current.Identifier = filepath.Base(path)
			}
		} else if strings.HasPrefix(line, "branch refs/heads/") {
			current.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		} else if line == "" && current.Path != "" {
			worktrees = append(worktrees, current)
			current = WorktreeInfo{}
		}
	}

	return worktrees, nil
}

// GetPath returns the worktree path for a ticket identifier, or empty if not found.
func (s *Service) GetPath(ticketIdentifier string) string {
	worktreePath := filepath.Join(s.worktreesDir, ticketIdentifier)
	if s.worktreeExists(worktreePath) {
		return worktreePath
	}
	return ""
}

// GetRepositoryPath returns the main repository path.
func (s *Service) GetRepositoryPath() string {
	return s.repoPath
}

// branchExists checks if a branch exists in the repository.
func (s *Service) branchExists(branchName string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branchName)
	cmd.Dir = s.repoPath
	cmd.Env = s.getEnvWithPath()
	err := cmd.Run()
	return err == nil
}

// getEnvWithPath returns environment variables with PATH including common tool locations.
// This ensures git-lfs and other tools are available when running as a launchd service.
func (s *Service) getEnvWithPath() []string {
	env := os.Environ()

	// Add common paths for homebrew and other tools
	var extraPaths string
	if runtime.GOOS == "darwin" {
		// macOS: add homebrew paths for both Apple Silicon and Intel
		extraPaths = "/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin"
	} else {
		// Linux: add common paths
		extraPaths = "/usr/local/bin"
	}

	// Find and update PATH, or add it if not present
	pathFound := false
	for i, e := range env {
		if strings.HasPrefix(e, "PATH=") {
			currentPath := strings.TrimPrefix(e, "PATH=")
			env[i] = "PATH=" + extraPaths + ":" + currentPath
			pathFound = true
			break
		}
	}
	if !pathFound {
		env = append(env, "PATH="+extraPaths+":/usr/bin:/bin:/usr/sbin:/sbin")
	}

	return env
}

// worktreeExists checks if a worktree exists at the given path.
func (s *Service) worktreeExists(worktreePath string) bool {
	// Check if directory exists and contains .git file
	gitFile := filepath.Join(worktreePath, ".git")
	_, err := os.Stat(gitFile)
	return err == nil
}
