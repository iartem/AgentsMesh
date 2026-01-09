package plugins

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/sandbox"
)

// InitScriptPlugin executes initialization scripts in the working directory.
type InitScriptPlugin struct {
	defaultTimeout time.Duration
}

// NewInitScriptPlugin creates a new InitScriptPlugin.
func NewInitScriptPlugin() *InitScriptPlugin {
	return &InitScriptPlugin{
		defaultTimeout: 5 * time.Minute, // Default timeout for init scripts
	}
}

func (p *InitScriptPlugin) Name() string {
	return "initscript"
}

func (p *InitScriptPlugin) Order() int {
	return 30 // After WorktreePlugin (10) and TempDirPlugin (20)
}

func (p *InitScriptPlugin) Setup(ctx context.Context, sb *sandbox.Sandbox, config map[string]interface{}) error {
	initScript := sandbox.GetStringConfig(config, "init_script")
	if initScript == "" {
		return nil
	}

	// Ensure WorkDir is set
	if sb.WorkDir == "" {
		return fmt.Errorf("init_script requires WorkDir to be set")
	}

	// Get timeout from config or use default
	timeout := p.defaultTimeout
	if t := sandbox.GetIntConfig(config, "init_timeout"); t > 0 {
		timeout = time.Duration(t) * time.Second
	}

	// Create logs directory
	if err := sb.EnsureLogsDir(); err != nil {
		log.Printf("[initscript] Warning: failed to create logs directory: %v", err)
	}

	// Open log file
	logPath := filepath.Join(sb.GetLogsDir(), "init.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		log.Printf("[initscript] Warning: failed to create log file: %v", err)
		logFile = nil
	}
	if logFile != nil {
		defer logFile.Close()
	}

	// Create context with timeout
	scriptCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute script
	log.Printf("[initscript] Running init script in %s (timeout: %v)", sb.WorkDir, timeout)

	cmd := exec.CommandContext(scriptCtx, "sh", "-c", initScript)
	cmd.Dir = sb.WorkDir
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("SANDBOX_ROOT=%s", sb.RootPath),
		fmt.Sprintf("SESSION_KEY=%s", sb.SessionKey),
	)

	// Capture output
	if logFile != nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	if err := cmd.Run(); err != nil {
		// Log the error
		errMsg := fmt.Sprintf("init script failed: %v", err)
		if logFile != nil {
			logFile.WriteString("\n" + errMsg + "\n")
		}

		// Check if it was a timeout
		if scriptCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("init script timed out after %v", timeout)
		}

		return fmt.Errorf("init script failed: %w", err)
	}

	// Record metadata
	sb.Metadata["init_script_ran"] = true
	sb.Metadata["init_script_log"] = logPath

	log.Printf("[initscript] Init script completed successfully")
	return nil
}

func (p *InitScriptPlugin) Teardown(sb *sandbox.Sandbox) error {
	// No cleanup needed
	return nil
}
