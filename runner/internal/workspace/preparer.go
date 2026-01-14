// Package workspace provides workspace preparation utilities.
// Preparer executes initialization steps before agent starts.
package workspace

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// PreparationStep represents a single step in the workspace preparation process.
// Implements the Strategy pattern for extensible preparation steps.
type PreparationStep interface {
	// Name returns the step name for logging and error reporting.
	Name() string

	// Execute runs the preparation step.
	// Returns an error if the step fails, which should abort pod creation.
	Execute(ctx context.Context, prepCtx *PreparationContext) error
}

// PreparationContext contains all the context needed for workspace preparation.
type PreparationContext struct {
	PodID            string            // Pod identifier
	TicketIdentifier string            // Ticket identifier (e.g., "TBD-123")
	BranchName       string            // Git branch name
	WorkingDir       string            // Actual working directory (may be worktree path)
	MainRepoDir      string            // Path to the main git repository
	WorktreeDir      string            // Path to the worktree (empty if not using worktree)
	BaseEnvVars      map[string]string // Base environment variables (e.g., AI provider credentials)
}

// GetEnvVars returns all environment variables for script execution.
// Includes base variables plus workspace-specific variables.
func (c *PreparationContext) GetEnvVars() map[string]string {
	result := make(map[string]string)

	// Copy base environment variables
	for k, v := range c.BaseEnvVars {
		result[k] = v
	}

	// Add workspace-specific variables
	result["WORKING_DIR"] = c.WorkingDir
	if c.MainRepoDir != "" {
		result["MAIN_REPO_DIR"] = c.MainRepoDir
	}
	if c.WorktreeDir != "" {
		result["WORKTREE_DIR"] = c.WorktreeDir
	}
	if c.TicketIdentifier != "" {
		result["TICKET_IDENTIFIER"] = c.TicketIdentifier
	}
	if c.BranchName != "" {
		result["BRANCH_NAME"] = c.BranchName
	}

	return result
}

// String returns a string representation for logging.
func (c *PreparationContext) String() string {
	return fmt.Sprintf(
		"PreparationContext{PodID: %s, Ticket: %s, WorkingDir: %s}",
		c.PodID, c.TicketIdentifier, c.WorkingDir,
	)
}

// PreparationError represents an error during workspace preparation.
type PreparationError struct {
	Step   string // Name of the step that failed
	Cause  error  // Underlying error
	Output string // Command output if applicable
}

// Error implements the error interface.
func (e *PreparationError) Error() string {
	if e.Output != "" {
		return fmt.Sprintf("preparation step '%s' failed: %v\nOutput: %s", e.Step, e.Cause, e.Output)
	}
	return fmt.Sprintf("preparation step '%s' failed: %v", e.Step, e.Cause)
}

// Unwrap returns the underlying error.
func (e *PreparationError) Unwrap() error {
	return e.Cause
}

// Preparer orchestrates the execution of preparation steps.
type Preparer struct {
	steps []PreparationStep
}

// NewPreparer creates a new Preparer with the given steps.
func NewPreparer(steps ...PreparationStep) *Preparer {
	return &Preparer{
		steps: steps,
	}
}

// NewPreparerFromScript creates a Preparer from a script and timeout.
// Returns nil if script is empty.
func NewPreparerFromScript(script string, timeoutSeconds int) *Preparer {
	if script == "" {
		return nil
	}

	timeout := time.Duration(timeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	step := NewScriptPreparationStep(script, timeout)
	return NewPreparer(step)
}

// Prepare executes all preparation steps in order.
// Stops and returns error on first failure.
func (p *Preparer) Prepare(ctx context.Context, prepCtx *PreparationContext) error {
	if len(p.steps) == 0 {
		return nil
	}

	log.Printf("[workspace] Starting workspace preparation: pod_id=%s, step_count=%d",
		prepCtx.PodID, len(p.steps))

	for i, step := range p.steps {
		log.Printf("[workspace] Executing preparation step: pod_id=%s, step=%s, step_num=%d, total=%d",
			prepCtx.PodID, step.Name(), i+1, len(p.steps))

		if err := step.Execute(ctx, prepCtx); err != nil {
			log.Printf("[workspace] Preparation step failed: pod_id=%s, step=%s, error=%v",
				prepCtx.PodID, step.Name(), err)
			return err
		}
	}

	log.Printf("[workspace] Workspace preparation completed: pod_id=%s", prepCtx.PodID)
	return nil
}

// AddStep adds a preparation step to the preparer.
func (p *Preparer) AddStep(step PreparationStep) {
	p.steps = append(p.steps, step)
}

// StepCount returns the number of steps in the preparer.
func (p *Preparer) StepCount() int {
	return len(p.steps)
}

// ScriptPreparationStep executes a shell script as a preparation step.
type ScriptPreparationStep struct {
	script  string
	timeout time.Duration
}

// NewScriptPreparationStep creates a new ScriptPreparationStep.
func NewScriptPreparationStep(script string, timeout time.Duration) *ScriptPreparationStep {
	if timeout <= 0 {
		timeout = 5 * time.Minute // Default timeout
	}
	return &ScriptPreparationStep{
		script:  script,
		timeout: timeout,
	}
}

// Name returns the step name.
func (s *ScriptPreparationStep) Name() string {
	return "script"
}

// Execute runs the script with the preparation context.
func (s *ScriptPreparationStep) Execute(ctx context.Context, prepCtx *PreparationContext) error {
	if s.script == "" {
		return nil
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()

	log.Printf("[workspace] Executing preparation script: pod_id=%s, working_dir=%s, timeout=%s",
		prepCtx.PodID, prepCtx.WorkingDir, s.timeout.String())

	// Create command
	cmd := exec.CommandContext(ctx, "/bin/sh", "-c", s.script)
	cmd.Dir = prepCtx.WorkingDir
	cmd.Env = s.buildEnv(prepCtx)

	// Execute and capture output
	output, err := cmd.CombinedOutput()
	outputStr := string(output)

	if err != nil {
		log.Printf("[workspace] Preparation script failed: pod_id=%s, error=%v, output=%s",
			prepCtx.PodID, err, outputStr)

		return &PreparationError{
			Step:   s.Name(),
			Cause:  err,
			Output: outputStr,
		}
	}

	log.Printf("[workspace] Preparation script completed: pod_id=%s, output_len=%d",
		prepCtx.PodID, len(outputStr))

	if outputStr != "" {
		log.Printf("[workspace] Script output: %s", outputStr)
	}

	return nil
}

// buildEnv builds the environment variables for script execution.
func (s *ScriptPreparationStep) buildEnv(prepCtx *PreparationContext) []string {
	// Start with current environment
	env := os.Environ()

	// Add extra paths for common tools
	env = s.addToolPaths(env)

	// Add preparation context variables
	for k, v := range prepCtx.GetEnvVars() {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}

	return env
}

// addToolPaths adds common tool paths to the environment.
func (s *ScriptPreparationStep) addToolPaths(env []string) []string {
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
