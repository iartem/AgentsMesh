package plugins

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/agentmesh/runner/internal/sandbox"
)

func TestNewTempDirPlugin(t *testing.T) {
	p := NewTempDirPlugin()

	if p == nil {
		t.Fatal("NewTempDirPlugin() returned nil")
	}
}

func TestTempDirPluginName(t *testing.T) {
	p := NewTempDirPlugin()

	if p.Name() != "tempdir" {
		t.Errorf("Name() = %q, want %q", p.Name(), "tempdir")
	}
}

func TestTempDirPluginOrder(t *testing.T) {
	p := NewTempDirPlugin()

	if p.Order() != 20 {
		t.Errorf("Order() = %d, want 20", p.Order())
	}
}

func TestTempDirPluginSetup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tempdir-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewTempDirPlugin()
	sb := sandbox.NewSandbox("test-session", tmpDir)
	ctx := context.Background()

	// Setup should create workspace directory
	if err := p.Setup(ctx, sb, nil); err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	// Verify WorkDir is set
	expectedWorkDir := filepath.Join(tmpDir, "workspace")
	if sb.WorkDir != expectedWorkDir {
		t.Errorf("WorkDir = %q, want %q", sb.WorkDir, expectedWorkDir)
	}

	// Verify directory was created
	if _, err := os.Stat(sb.WorkDir); os.IsNotExist(err) {
		t.Error("Workspace directory was not created")
	}

	// Verify metadata
	if sb.Metadata["workspace_type"] != "tempdir" {
		t.Errorf("Metadata[workspace_type] = %q, want %q", sb.Metadata["workspace_type"], "tempdir")
	}
}

func TestTempDirPluginSetupSkipsIfWorkDirSet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tempdir-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewTempDirPlugin()
	sb := sandbox.NewSandbox("test-session", tmpDir)
	ctx := context.Background()

	// Pre-set WorkDir (simulating WorktreePlugin)
	existingWorkDir := "/some/existing/path"
	sb.WorkDir = existingWorkDir

	// Setup should skip
	if err := p.Setup(ctx, sb, nil); err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	// WorkDir should remain unchanged
	if sb.WorkDir != existingWorkDir {
		t.Errorf("WorkDir = %q, want %q", sb.WorkDir, existingWorkDir)
	}
}

func TestTempDirPluginTeardown(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "tempdir-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewTempDirPlugin()
	sb := sandbox.NewSandbox("test-session", tmpDir)

	// Teardown should not error
	if err := p.Teardown(sb); err != nil {
		t.Errorf("Teardown() failed: %v", err)
	}
}

func TestTempDirPluginSetupMkdirError(t *testing.T) {
	// Test error case when directory creation fails
	p := NewTempDirPlugin()
	// Use a path that can't be created (read-only location)
	sb := sandbox.NewSandbox("test-session", "/nonexistent/path/that/cannot/be/created")
	ctx := context.Background()

	// Setup should fail
	err := p.Setup(ctx, sb, nil)
	if err == nil {
		t.Error("Setup() should fail when directory cannot be created")
	}
}
