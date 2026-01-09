package runner

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/sandbox"
	"github.com/anthropics/agentmesh/runner/internal/terminal"
	"github.com/anthropics/agentmesh/runner/internal/workspace"
)

// SessionBuilder builds sessions using the Builder pattern.
// It provides a fluent API for configuring and creating sessions.
type SessionBuilder struct {
	runner *Runner

	// Session configuration
	sessionKey       string
	agentType        string
	launchCommand    string
	launchArgs       []string
	envVars          map[string]string
	rows             int
	cols             int
	initialPrompt    string
	repositoryURL    string
	branch           string
	ticketIdentifier string
	useWorktree      bool
	prepScript       string
	prepTimeout      int

	// MCP configuration (legacy - now handled by sandbox plugin)
	mcpEnabled bool
	mcpServers []string

	// Sandbox mode - use new plugin-based sandbox system
	useSandbox   bool
	pluginConfig map[string]interface{}
}

// NewSessionBuilder creates a new session builder.
func NewSessionBuilder(runner *Runner) *SessionBuilder {
	return &SessionBuilder{
		runner:  runner,
		envVars: make(map[string]string),
		rows:    24,
		cols:    80,
	}
}

// WithSessionKey sets the session key.
func (b *SessionBuilder) WithSessionKey(key string) *SessionBuilder {
	b.sessionKey = key
	return b
}

// WithAgentType sets the agent type.
func (b *SessionBuilder) WithAgentType(agentType string) *SessionBuilder {
	b.agentType = agentType
	return b
}

// WithLaunchCommand sets the command to launch.
func (b *SessionBuilder) WithLaunchCommand(command string, args []string) *SessionBuilder {
	b.launchCommand = command
	b.launchArgs = args
	return b
}

// WithEnvVars sets environment variables.
func (b *SessionBuilder) WithEnvVars(envVars map[string]string) *SessionBuilder {
	for k, v := range envVars {
		b.envVars[k] = v
	}
	return b
}

// WithEnvVar adds a single environment variable.
func (b *SessionBuilder) WithEnvVar(key, value string) *SessionBuilder {
	b.envVars[key] = value
	return b
}

// WithTerminalSize sets terminal dimensions.
func (b *SessionBuilder) WithTerminalSize(rows, cols int) *SessionBuilder {
	if rows > 0 {
		b.rows = rows
	}
	if cols > 0 {
		b.cols = cols
	}
	return b
}

// WithInitialPrompt sets the initial prompt to send.
func (b *SessionBuilder) WithInitialPrompt(prompt string) *SessionBuilder {
	b.initialPrompt = prompt
	return b
}

// WithRepository configures repository URL and branch.
func (b *SessionBuilder) WithRepository(url, branch string) *SessionBuilder {
	b.repositoryURL = url
	b.branch = branch
	return b
}

// WithWorktree enables worktree mode for the given ticket.
func (b *SessionBuilder) WithWorktree(ticketIdentifier string) *SessionBuilder {
	b.ticketIdentifier = ticketIdentifier
	b.useWorktree = true
	return b
}

// WithPreparationScript sets a script to run before session starts.
func (b *SessionBuilder) WithPreparationScript(script string, timeoutSeconds int) *SessionBuilder {
	b.prepScript = script
	b.prepTimeout = timeoutSeconds
	return b
}

// WithMCP enables MCP with specified servers (legacy - use WithSandbox instead).
func (b *SessionBuilder) WithMCP(serverNames ...string) *SessionBuilder {
	b.mcpEnabled = true
	b.mcpServers = serverNames
	return b
}

// WithSandbox enables sandbox mode with plugin configuration.
// This is the recommended way to configure session environments.
func (b *SessionBuilder) WithSandbox(pluginConfig map[string]interface{}) *SessionBuilder {
	b.useSandbox = true
	b.pluginConfig = pluginConfig
	return b
}

// WithPluginConfig adds or updates plugin configuration.
func (b *SessionBuilder) WithPluginConfig(key string, value interface{}) *SessionBuilder {
	if b.pluginConfig == nil {
		b.pluginConfig = make(map[string]interface{})
	}
	b.pluginConfig[key] = value
	return b
}

// Build creates the session.
func (b *SessionBuilder) Build(ctx context.Context) (*Session, error) {
	if b.sessionKey == "" {
		return nil, fmt.Errorf("session key is required")
	}

	log.Printf("[session_builder] Building session: session_key=%s, agent=%s, use_sandbox=%v",
		b.sessionKey, b.agentType, b.useSandbox)

	var workingDir, worktreePath, branchName string
	var sb *sandbox.Sandbox
	var launchArgs []string
	var err error

	// Use sandbox system if enabled and sandbox manager is available
	if b.useSandbox && b.runner.sandboxManager != nil {
		sb, err = b.buildWithSandbox(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create sandbox: %w", err)
		}
		workingDir = sb.WorkDir
		if wt, ok := sb.Metadata["worktree_path"].(string); ok {
			worktreePath = wt
		}
		if bn, ok := sb.Metadata["branch_name"].(string); ok {
			branchName = bn
		}
		launchArgs = append(b.launchArgs, sb.LaunchArgs...)
	} else {
		// Legacy mode: use old working directory resolution
		workingDir, worktreePath, branchName, err = b.resolveWorkingDirectory(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve working directory: %w", err)
		}

		// Run preparation script if specified (legacy mode)
		if b.prepScript != "" {
			if err := b.runPreparation(ctx, workingDir, worktreePath, branchName); err != nil {
				return nil, fmt.Errorf("preparation failed: %w", err)
			}
		}
		launchArgs = b.launchArgs
	}

	// Merge environment variables
	envVars := b.mergeEnvVars()

	// Add sandbox env vars if available
	if sb != nil {
		for k, v := range sb.EnvVars {
			envVars[k] = v
		}
	}

	// Create terminal
	term, err := terminal.New(terminal.Options{
		Command:  b.launchCommand,
		Args:     launchArgs,
		WorkDir:  workingDir,
		Env:      envVars,
		Rows:     b.rows,
		Cols:     b.cols,
		OnOutput: nil, // Will be set by caller
		OnExit:   nil, // Will be set by caller
	})
	if err != nil {
		// Cleanup sandbox on failure
		if sb != nil && b.runner.sandboxManager != nil {
			b.runner.sandboxManager.Cleanup(b.sessionKey)
		}
		return nil, fmt.Errorf("failed to create terminal: %w", err)
	}

	// Create session
	session := &Session{
		ID:               b.sessionKey,
		SessionKey:       b.sessionKey,
		AgentType:        b.agentType,
		RepositoryURL:    b.repositoryURL,
		Branch:           branchName,
		WorktreePath:     worktreePath,
		InitialPrompt:    b.initialPrompt,
		Terminal:         term,
		StartedAt:        time.Now(),
		Status:           SessionStatusInitializing,
		TicketIdentifier: b.ticketIdentifier,
	}

	log.Printf("[session_builder] Session built: session_key=%s, working_dir=%s, sandbox=%v",
		b.sessionKey, workingDir, sb != nil)

	return session, nil
}

// buildWithSandbox creates a sandbox using the plugin system.
func (b *SessionBuilder) buildWithSandbox(ctx context.Context) (*sandbox.Sandbox, error) {
	// Build plugin config from builder fields
	config := make(map[string]interface{})

	// Copy explicit builder fields
	if b.repositoryURL != "" {
		config["repository_url"] = b.repositoryURL
	}
	if b.branch != "" {
		config["branch"] = b.branch
	}
	if b.ticketIdentifier != "" {
		config["ticket_identifier"] = b.ticketIdentifier
	}
	if b.prepScript != "" {
		config["init_script"] = b.prepScript
	}
	if b.prepTimeout > 0 {
		config["init_timeout"] = b.prepTimeout
	}
	if len(b.envVars) > 0 {
		envMap := make(map[string]interface{})
		for k, v := range b.envVars {
			envMap[k] = v
		}
		config["env_vars"] = envMap
	}

	// Merge explicit plugin config (can override above values)
	for k, v := range b.pluginConfig {
		config[k] = v
	}

	// Create sandbox
	return b.runner.sandboxManager.Create(ctx, b.sessionKey, config)
}

// resolveWorkingDirectory determines the working directory for the session.
// Returns (workingDir, worktreePath, branchName, error).
// Note: This method is only used in non-sandbox mode. For sandbox mode,
// the working directory is set by the SandboxManager plugins.
func (b *SessionBuilder) resolveWorkingDirectory(ctx context.Context) (string, string, string, error) {
	// Priority 1: Use workspace manager with repository URL
	if b.repositoryURL != "" && b.runner.workspace != nil {
		worktreePath, err := b.runner.workspace.CreateWorktree(ctx, b.repositoryURL, b.branch, b.sessionKey)
		if err != nil {
			return "", "", "", fmt.Errorf("failed to create repository worktree: %w", err)
		}
		return worktreePath, worktreePath, b.branch, nil
	}

	// Priority 2: Use temporary workspace
	if b.runner.workspace != nil {
		tempPath := b.runner.workspace.TempWorkspace(b.sessionKey)
		return tempPath, "", "", nil
	}

	// Priority 3: Use workspace root from config
	return b.runner.cfg.WorkspaceRoot, "", "", nil
}

// runPreparation runs the preparation script.
func (b *SessionBuilder) runPreparation(ctx context.Context, workingDir, worktreePath, branchName string) error {
	preparer := workspace.NewPreparerFromScript(b.prepScript, b.prepTimeout)
	if preparer == nil {
		return nil
	}

	prepCtx := &workspace.PreparationContext{
		SessionID:        b.sessionKey,
		TicketIdentifier: b.ticketIdentifier,
		BranchName:       branchName,
		WorkingDir:       workingDir,
		WorktreeDir:      worktreePath,
		BaseEnvVars:      b.envVars,
	}

	log.Printf("[session_builder] Running preparation script: session_key=%s", b.sessionKey)

	if err := preparer.Prepare(ctx, prepCtx); err != nil {
		return fmt.Errorf("preparation script failed: %w", err)
	}

	log.Printf("[session_builder] Preparation completed: session_key=%s", b.sessionKey)
	return nil
}

// mergeEnvVars merges all environment variable sources.
func (b *SessionBuilder) mergeEnvVars() map[string]string {
	result := make(map[string]string)

	// Add config env vars first (lowest priority)
	if b.runner.cfg != nil {
		for k, v := range b.runner.cfg.AgentEnvVars {
			result[k] = v
		}
	}

	// Add builder env vars (highest priority)
	for k, v := range b.envVars {
		result[k] = v
	}

	return result
}

// ExtendedSession adds additional fields to Session for enhanced functionality.
type ExtendedSession struct {
	*Session

	// Output/exit callbacks
	OnOutput func([]byte)
	OnExit   func(int)

	// Additional metadata
	TicketIdentifier string
	ManagedSession   *terminal.Session // Reference to managed terminal session
}

// UpdateSession modifies Session to include additional fields.
// This is done by embedding in the runner package.
func init() {
	// The Session struct in runner.go will be extended
}
