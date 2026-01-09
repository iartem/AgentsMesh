package plugins

import (
	"context"
	"os"
	"path/filepath"

	"github.com/anthropics/agentmesh/runner/internal/sandbox"
)

// TempDirPlugin creates a temporary working directory when no WorkDir is set.
// This is used for sessions without a git repository (e.g., operations tasks).
type TempDirPlugin struct{}

// NewTempDirPlugin creates a new TempDirPlugin.
func NewTempDirPlugin() *TempDirPlugin {
	return &TempDirPlugin{}
}

func (p *TempDirPlugin) Name() string {
	return "tempdir"
}

func (p *TempDirPlugin) Order() int {
	return 20 // After WorktreePlugin (10)
}

func (p *TempDirPlugin) Setup(ctx context.Context, sb *sandbox.Sandbox, config map[string]interface{}) error {
	// Skip if WorkDir is already set (e.g., by WorktreePlugin)
	if sb.WorkDir != "" {
		return nil
	}

	// Create workspace directory inside sandbox
	workspacePath := filepath.Join(sb.RootPath, "workspace")
	if err := os.MkdirAll(workspacePath, 0755); err != nil {
		return err
	}

	sb.WorkDir = workspacePath
	sb.Metadata["workspace_type"] = "tempdir"

	return nil
}

func (p *TempDirPlugin) Teardown(sb *sandbox.Sandbox) error {
	// No special cleanup needed - directory will be removed with sandbox
	return nil
}
