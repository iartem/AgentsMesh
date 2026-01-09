package plugins

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/anthropics/agentmesh/runner/internal/sandbox"
)

// WorktreePlugin creates a git worktree inside the sandbox for ticket-based development.
type WorktreePlugin struct {
	reposDir string // Directory for repository cache
}

// NewWorktreePlugin creates a new WorktreePlugin.
func NewWorktreePlugin(reposDir string) *WorktreePlugin {
	return &WorktreePlugin{
		reposDir: reposDir,
	}
}

func (p *WorktreePlugin) Name() string {
	return "worktree"
}

func (p *WorktreePlugin) Order() int {
	return 10 // First plugin to set WorkDir
}

func (p *WorktreePlugin) Setup(ctx context.Context, sb *sandbox.Sandbox, config map[string]interface{}) error {
	repoURL := sandbox.GetStringConfig(config, "repository_url")
	ticketID := sandbox.GetStringConfig(config, "ticket_identifier")
	baseBranch := sandbox.GetStringConfig(config, "branch")
	gitToken := sandbox.GetStringConfig(config, "git_token")
	sshPrivateKey := sandbox.GetStringConfig(config, "ssh_private_key")

	// Skip if no repository URL or ticket identifier
	if repoURL == "" || ticketID == "" {
		log.Printf("[worktree] Skipping: no repository_url or ticket_identifier")
		return nil
	}

	if baseBranch == "" {
		baseBranch = "main"
	}

	// Handle SSH key setup (for SSH URLs)
	var sshKeyPath string
	var cleanupSSHKey func()
	if sshPrivateKey != "" && isSSHURL(repoURL) {
		var err error
		sshKeyPath, cleanupSSHKey, err = p.setupSSHKey(sb.RootPath, sshPrivateKey)
		if err != nil {
			return fmt.Errorf("failed to setup SSH key: %w", err)
		}
		// Cleanup will be deferred after git operations are complete
		log.Printf("[worktree] SSH key configured for authentication")
	}

	// If git token is provided and URL is HTTPS, inject it into the URL
	cloneURL := repoURL
	if gitToken != "" && !isSSHURL(repoURL) {
		cloneURL = p.injectToken(repoURL, gitToken)
	}

	// Ensure repository is cloned/updated in cache
	repoPath, err := p.ensureRepo(ctx, cloneURL, repoURL, sshKeyPath)
	if err != nil {
		if cleanupSSHKey != nil {
			cleanupSSHKey()
		}
		return fmt.Errorf("failed to ensure repo: %w", err)
	}

	// Worktree path inside sandbox
	worktreePath := filepath.Join(sb.RootPath, "worktree")

	// Branch naming: ticket/{ticket_id}-{session_suffix}
	sessionSuffix := sb.SessionKey
	if len(sessionSuffix) > 8 {
		sessionSuffix = sessionSuffix[:8]
	}
	branchName := fmt.Sprintf("ticket/%s-%s", ticketID, sessionSuffix)

	// Create worktree
	if err := p.createWorktree(ctx, repoPath, worktreePath, branchName, baseBranch, sshKeyPath); err != nil {
		if cleanupSSHKey != nil {
			cleanupSSHKey()
		}
		return fmt.Errorf("failed to create worktree: %w", err)
	}

	// Cleanup SSH key after git operations (for security)
	if cleanupSSHKey != nil {
		cleanupSSHKey()
	}

	// Set sandbox attributes
	sb.WorkDir = worktreePath
	sb.Metadata["repo_url"] = repoURL
	sb.Metadata["repo_path"] = repoPath
	sb.Metadata["worktree_path"] = worktreePath
	sb.Metadata["branch_name"] = branchName
	sb.Metadata["workspace_type"] = "worktree"

	log.Printf("[worktree] Created worktree: session=%s, branch=%s, path=%s",
		sb.SessionKey, branchName, worktreePath)

	return nil
}

func (p *WorktreePlugin) Teardown(sb *sandbox.Sandbox) error {
	repoPath, _ := sb.Metadata["repo_path"].(string)
	worktreePath, _ := sb.Metadata["worktree_path"].(string)

	if repoPath == "" || worktreePath == "" {
		return nil
	}

	// Remove worktree from git
	cmd := exec.Command("git", "worktree", "remove", worktreePath, "--force")
	cmd.Dir = repoPath
	cmd.Env = p.getEnvWithPath()

	if output, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[worktree] Warning: git worktree remove failed: %v, output: %s", err, output)
		// Continue - directory cleanup will happen with sandbox removal
	}

	return nil
}

// injectToken injects authentication token into the repository URL.
func (p *WorktreePlugin) injectToken(repoURL, token string) string {
	// Handle HTTPS URLs: https://github.com/org/repo.git -> https://token@github.com/org/repo.git
	if strings.HasPrefix(repoURL, "https://") {
		return strings.Replace(repoURL, "https://", fmt.Sprintf("https://%s@", token), 1)
	}
	// Handle HTTP URLs (not recommended but supported)
	if strings.HasPrefix(repoURL, "http://") {
		return strings.Replace(repoURL, "http://", fmt.Sprintf("http://%s@", token), 1)
	}
	// SSH URLs don't need token injection
	return repoURL
}

// ensureRepo ensures the repository is cloned/updated in the cache directory.
func (p *WorktreePlugin) ensureRepo(ctx context.Context, cloneURL, originalURL, sshKeyPath string) (string, error) {
	// Use hash of original URL (without token) as directory name
	repoHash := hashRepoURL(originalURL)
	repoPath := filepath.Join(p.reposDir, repoHash)

	// Get environment with optional SSH configuration
	env := p.getEnvWithPath()
	if sshKeyPath != "" {
		env = p.addSSHEnv(env, sshKeyPath)
	}

	// Check if repository already exists
	if _, err := os.Stat(filepath.Join(repoPath, "HEAD")); err == nil {
		// Repository exists, fetch updates
		log.Printf("[worktree] Fetching updates for repo %s", originalURL)
		cmd := exec.CommandContext(ctx, "git", "fetch", "--all", "--prune")
		cmd.Dir = repoPath
		cmd.Env = env
		// Ignore fetch errors - use existing version
		cmd.Run()
	} else {
		// Clone as bare repository (saves space, only used for worktrees)
		log.Printf("[worktree] Cloning repo %s to %s", originalURL, repoPath)
		if err := os.MkdirAll(p.reposDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create repos directory: %w", err)
		}

		cmd := exec.CommandContext(ctx, "git", "clone", "--bare", cloneURL, repoPath)
		cmd.Env = env
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("git clone failed: %w\noutput: %s", err, string(output))
		}
	}

	return repoPath, nil
}

// createWorktree creates a git worktree at the specified path.
func (p *WorktreePlugin) createWorktree(ctx context.Context, repoPath, worktreePath, branchName, baseBranch, sshKeyPath string) error {
	var cmd *exec.Cmd

	// Get environment with optional SSH configuration
	env := p.getEnvWithPath()
	if sshKeyPath != "" {
		env = p.addSSHEnv(env, sshKeyPath)
	}

	if p.branchExists(repoPath, branchName) {
		// Branch exists, attach worktree to it
		log.Printf("[worktree] Attaching worktree to existing branch: %s", branchName)
		cmd = exec.CommandContext(ctx, "git", "worktree", "add", worktreePath, branchName)
	} else {
		// Create new branch from base
		log.Printf("[worktree] Creating worktree with new branch: %s (base: %s)", branchName, baseBranch)
		cmd = exec.CommandContext(ctx, "git", "worktree", "add", "-b", branchName, worktreePath, baseBranch)
	}

	cmd.Dir = repoPath
	cmd.Env = env

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git worktree add failed: %w\noutput: %s", err, string(output))
	}

	return nil
}

// branchExists checks if a branch exists in the repository.
func (p *WorktreePlugin) branchExists(repoPath, branchName string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branchName)
	cmd.Dir = repoPath
	cmd.Env = p.getEnvWithPath()
	return cmd.Run() == nil
}

// getEnvWithPath returns environment variables with PATH including common tool locations.
func (p *WorktreePlugin) getEnvWithPath() []string {
	env := os.Environ()

	var extraPaths string
	if runtime.GOOS == "darwin" {
		// macOS: add homebrew paths for both Apple Silicon and Intel
		extraPaths = "/opt/homebrew/bin:/opt/homebrew/sbin:/usr/local/bin"
	} else {
		// Linux: add common paths
		extraPaths = "/usr/local/bin"
	}

	// Find and update PATH
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

// hashRepoURL generates a short hash for the repository URL (used as directory name).
func hashRepoURL(url string) string {
	h := sha256.Sum256([]byte(url))
	return hex.EncodeToString(h[:8]) // 16 characters
}

// isSSHURL checks if a URL is an SSH URL (git@... or ssh://...).
func isSSHURL(url string) bool {
	return strings.HasPrefix(url, "git@") || strings.HasPrefix(url, "ssh://")
}

// setupSSHKey writes the SSH private key to a temporary file and returns the path and cleanup function.
func (p *WorktreePlugin) setupSSHKey(sandboxRoot, privateKey string) (string, func(), error) {
	// Create .ssh directory in sandbox
	sshDir := filepath.Join(sandboxRoot, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return "", nil, fmt.Errorf("failed to create .ssh directory: %w", err)
	}

	// Write private key to file
	keyPath := filepath.Join(sshDir, "id_rsa")
	if err := os.WriteFile(keyPath, []byte(privateKey), 0600); err != nil {
		return "", nil, fmt.Errorf("failed to write SSH key: %w", err)
	}

	// Cleanup function to remove the key file
	cleanup := func() {
		if err := os.Remove(keyPath); err != nil {
			log.Printf("[worktree] Warning: failed to remove SSH key file: %v", err)
		}
	}

	return keyPath, cleanup, nil
}

// addSSHEnv adds GIT_SSH_COMMAND environment variable for SSH authentication.
func (p *WorktreePlugin) addSSHEnv(env []string, sshKeyPath string) []string {
	// GIT_SSH_COMMAND tells git to use specific SSH options
	sshCmd := fmt.Sprintf("ssh -i %s -o StrictHostKeyChecking=no -o UserKnownHostsFile=/dev/null", sshKeyPath)

	// Check if GIT_SSH_COMMAND already exists and replace it
	found := false
	for i, e := range env {
		if strings.HasPrefix(e, "GIT_SSH_COMMAND=") {
			env[i] = "GIT_SSH_COMMAND=" + sshCmd
			found = true
			break
		}
	}
	if !found {
		env = append(env, "GIT_SSH_COMMAND="+sshCmd)
	}

	return env
}
