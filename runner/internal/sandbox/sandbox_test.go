package sandbox

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewSandbox(t *testing.T) {
	sessionKey := "test-session"
	rootPath := "/tmp/sandbox/test"

	sb := NewSandbox(sessionKey, rootPath)

	if sb.SessionKey != sessionKey {
		t.Errorf("SessionKey = %q, want %q", sb.SessionKey, sessionKey)
	}
	if sb.RootPath != rootPath {
		t.Errorf("RootPath = %q, want %q", sb.RootPath, rootPath)
	}
	if sb.EnvVars == nil {
		t.Error("EnvVars should not be nil")
	}
	if sb.LaunchArgs == nil {
		t.Error("LaunchArgs should not be nil")
	}
	if sb.Metadata == nil {
		t.Error("Metadata should not be nil")
	}
	if sb.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}
}

func TestSandboxAddPlugin(t *testing.T) {
	sb := NewSandbox("test", "/tmp")

	// Add mock plugins
	p1 := &mockPlugin{name: "plugin1"}
	p2 := &mockPlugin{name: "plugin2"}

	sb.AddPlugin(p1)
	sb.AddPlugin(p2)

	plugins := sb.GetPlugins()
	if len(plugins) != 2 {
		t.Errorf("len(plugins) = %d, want 2", len(plugins))
	}
	if plugins[0].Name() != "plugin1" {
		t.Errorf("plugins[0].Name() = %q, want %q", plugins[0].Name(), "plugin1")
	}
	if plugins[1].Name() != "plugin2" {
		t.Errorf("plugins[1].Name() = %q, want %q", plugins[1].Name(), "plugin2")
	}
}

func TestSandboxSaveAndLoad(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "sandbox-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create and save sandbox
	sb := NewSandbox("test-session", tmpDir)
	sb.WorkDir = "/work/dir"
	sb.EnvVars["KEY1"] = "value1"
	sb.LaunchArgs = []string{"--arg1", "--arg2"}
	sb.Metadata["custom"] = "data"

	if err := sb.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists
	metaPath := filepath.Join(tmpDir, "sandbox.json")
	if _, err := os.Stat(metaPath); os.IsNotExist(err) {
		t.Error("sandbox.json was not created")
	}

	// Load into new sandbox
	sb2 := &Sandbox{RootPath: tmpDir}
	if err := sb2.Load(); err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if sb2.SessionKey != "test-session" {
		t.Errorf("SessionKey = %q, want %q", sb2.SessionKey, "test-session")
	}
	if sb2.WorkDir != "/work/dir" {
		t.Errorf("WorkDir = %q, want %q", sb2.WorkDir, "/work/dir")
	}
	if sb2.EnvVars["KEY1"] != "value1" {
		t.Errorf("EnvVars[KEY1] = %q, want %q", sb2.EnvVars["KEY1"], "value1")
	}
	if len(sb2.LaunchArgs) != 2 {
		t.Errorf("len(LaunchArgs) = %d, want 2", len(sb2.LaunchArgs))
	}
}

func TestSandboxLoadError(t *testing.T) {
	sb := &Sandbox{RootPath: "/nonexistent/path"}
	err := sb.Load()
	if err == nil {
		t.Error("Load() should fail for nonexistent path")
	}
}

func TestSandboxGetLogsDir(t *testing.T) {
	sb := NewSandbox("test", "/tmp/sandbox")
	expected := "/tmp/sandbox/logs"
	if sb.GetLogsDir() != expected {
		t.Errorf("GetLogsDir() = %q, want %q", sb.GetLogsDir(), expected)
	}
}

func TestSandboxEnsureLogsDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sandbox-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sb := NewSandbox("test", tmpDir)
	if err := sb.EnsureLogsDir(); err != nil {
		t.Fatalf("EnsureLogsDir() failed: %v", err)
	}

	logsDir := sb.GetLogsDir()
	info, err := os.Stat(logsDir)
	if err != nil {
		t.Errorf("Logs directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Logs path is not a directory")
	}
}

func TestSandboxUpdatedAt(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sandbox-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	sb := NewSandbox("test", tmpDir)
	originalUpdatedAt := sb.UpdatedAt

	// Wait a bit and save
	time.Sleep(10 * time.Millisecond)
	if err := sb.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	if !sb.UpdatedAt.After(originalUpdatedAt) {
		t.Error("UpdatedAt should be updated after Save()")
	}
}

// Note: mockPlugin is defined in manager_test.go and shared across tests in this package
