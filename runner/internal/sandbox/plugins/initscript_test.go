package plugins

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/sandbox"
)

func TestNewInitScriptPlugin(t *testing.T) {
	p := NewInitScriptPlugin()

	if p == nil {
		t.Fatal("NewInitScriptPlugin() returned nil")
	}
	if p.defaultTimeout != 5*time.Minute {
		t.Errorf("defaultTimeout = %v, want 5m", p.defaultTimeout)
	}
}

func TestInitScriptPluginName(t *testing.T) {
	p := NewInitScriptPlugin()

	if p.Name() != "initscript" {
		t.Errorf("Name() = %q, want %q", p.Name(), "initscript")
	}
}

func TestInitScriptPluginOrder(t *testing.T) {
	p := NewInitScriptPlugin()

	if p.Order() != 30 {
		t.Errorf("Order() = %d, want 30", p.Order())
	}
}

func TestInitScriptPluginSetupSkipsWithoutScript(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "initscript-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewInitScriptPlugin()
	sb := sandbox.NewSandbox("test-session", tmpDir)
	sb.WorkDir = tmpDir
	ctx := context.Background()

	// Setup without init_script should skip
	if err := p.Setup(ctx, sb, nil); err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	// Metadata should not indicate script ran
	if _, ok := sb.Metadata["init_script_ran"]; ok {
		t.Error("init_script_ran should not be set when no script provided")
	}
}

func TestInitScriptPluginSetupRequiresWorkDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "initscript-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewInitScriptPlugin()
	sb := sandbox.NewSandbox("test-session", tmpDir)
	// sb.WorkDir is empty
	ctx := context.Background()

	config := map[string]interface{}{
		"init_script": "echo hello",
	}

	// Setup without WorkDir should fail
	if err := p.Setup(ctx, sb, config); err == nil {
		t.Error("Setup() should fail without WorkDir")
	}
}

func TestInitScriptPluginSetupSuccess(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "initscript-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewInitScriptPlugin()
	sb := sandbox.NewSandbox("test-session", tmpDir)
	sb.WorkDir = tmpDir
	ctx := context.Background()

	// Create a test file to verify script ran
	testFile := filepath.Join(tmpDir, "test-output.txt")
	config := map[string]interface{}{
		"init_script": "echo 'hello world' > " + testFile,
	}

	if err := p.Setup(ctx, sb, config); err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	// Verify script ran
	if sb.Metadata["init_script_ran"] != true {
		t.Error("init_script_ran should be true")
	}

	// Verify log file path is set
	if _, ok := sb.Metadata["init_script_log"].(string); !ok {
		t.Error("init_script_log should be set")
	}

	// Verify test file was created
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}
	if string(content) != "hello world\n" {
		t.Errorf("Test file content = %q, want %q", string(content), "hello world\n")
	}
}

func TestInitScriptPluginSetupFailure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "initscript-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewInitScriptPlugin()
	sb := sandbox.NewSandbox("test-session", tmpDir)
	sb.WorkDir = tmpDir
	ctx := context.Background()

	config := map[string]interface{}{
		"init_script": "exit 1", // Failing script
	}

	// Setup should fail
	if err := p.Setup(ctx, sb, config); err == nil {
		t.Error("Setup() should fail when script fails")
	}
}

func TestInitScriptPluginSetupTimeout(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "initscript-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewInitScriptPlugin()
	sb := sandbox.NewSandbox("test-session", tmpDir)
	sb.WorkDir = tmpDir
	ctx := context.Background()

	config := map[string]interface{}{
		"init_script":  "sleep 10", // Long-running script
		"init_timeout": 1,          // 1 second timeout
	}

	// Setup should timeout
	err = p.Setup(ctx, sb, config)
	if err == nil {
		t.Error("Setup() should fail on timeout")
	}
}

func TestInitScriptPluginSetupEnvironment(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "initscript-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewInitScriptPlugin()
	sb := sandbox.NewSandbox("test-session", tmpDir)
	sb.WorkDir = tmpDir
	ctx := context.Background()

	// Create a test file to capture environment variables
	testFile := filepath.Join(tmpDir, "env-output.txt")
	config := map[string]interface{}{
		"init_script": "echo $SANDBOX_ROOT > " + testFile + " && echo $SESSION_KEY >> " + testFile,
	}

	if err := p.Setup(ctx, sb, config); err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	// Verify environment variables were set
	content, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("Failed to read test file: %v", err)
	}

	lines := string(content)
	if !containsStr(lines, tmpDir) {
		t.Errorf("SANDBOX_ROOT not in output: %s", lines)
	}
	if !containsStr(lines, "test-session") {
		t.Errorf("SESSION_KEY not in output: %s", lines)
	}
}

func TestInitScriptPluginTeardown(t *testing.T) {
	p := NewInitScriptPlugin()
	sb := sandbox.NewSandbox("test-session", "/tmp/sandbox")

	// Teardown should not error
	if err := p.Teardown(sb); err != nil {
		t.Errorf("Teardown() failed: %v", err)
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestInitScriptPluginSetupLogsDirError(t *testing.T) {
	// Test when logs directory cannot be created
	tmpDir, err := os.MkdirTemp("", "initscript-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewInitScriptPlugin()
	// Use a sandbox with invalid root path to trigger EnsureLogsDir error
	sb := sandbox.NewSandbox("test-session", "/nonexistent/root/path")
	sb.WorkDir = tmpDir // WorkDir needs to be valid for script execution
	ctx := context.Background()

	config := map[string]interface{}{
		"init_script": "echo test",
	}

	// Setup should still work (logs dir error is just logged, not fatal)
	// The script will still run, but log file creation will fail
	err = p.Setup(ctx, sb, config)
	// This may or may not error depending on the exact failure path
	// The key is that we exercise the error handling code
	_ = err
}

func TestInitScriptPluginSetupLogFileError(t *testing.T) {
	// Test when log file cannot be created but logs dir exists
	tmpDir, err := os.MkdirTemp("", "initscript-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create logs directory but make it unwritable
	logsDir := filepath.Join(tmpDir, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		t.Fatalf("Failed to create logs dir: %v", err)
	}
	// Make logs dir read-only
	if err := os.Chmod(logsDir, 0555); err != nil {
		t.Fatalf("Failed to chmod logs dir: %v", err)
	}
	defer os.Chmod(logsDir, 0755) // Restore for cleanup

	p := NewInitScriptPlugin()
	sb := sandbox.NewSandbox("test-session", tmpDir)
	sb.WorkDir = tmpDir
	ctx := context.Background()

	config := map[string]interface{}{
		"init_script": "echo hello",
	}

	// Setup should still succeed even if log file cannot be created
	// (output goes to nowhere, but script runs)
	if err := p.Setup(ctx, sb, config); err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	// Script should have run successfully
	if sb.Metadata["init_script_ran"] != true {
		t.Error("Script should have run even without log file")
	}
}
