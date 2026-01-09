package plugins

import (
	"context"
	"os"
	"testing"

	"github.com/anthropics/agentmesh/runner/internal/sandbox"
)

func TestNewEnvPlugin(t *testing.T) {
	p := NewEnvPlugin()

	if p == nil {
		t.Fatal("NewEnvPlugin() returned nil")
	}
}

func TestEnvPluginName(t *testing.T) {
	p := NewEnvPlugin()

	if p.Name() != "env" {
		t.Errorf("Name() = %q, want %q", p.Name(), "env")
	}
}

func TestEnvPluginOrder(t *testing.T) {
	p := NewEnvPlugin()

	if p.Order() != 40 {
		t.Errorf("Order() = %d, want 40", p.Order())
	}
}

func TestEnvPluginSetupSkipsWithoutEnvVars(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "env-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewEnvPlugin()
	sb := sandbox.NewSandbox("test-session", tmpDir)
	ctx := context.Background()

	// Initial EnvVars should be empty
	initialLen := len(sb.EnvVars)

	// Setup without env_vars should skip
	if err := p.Setup(ctx, sb, nil); err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	// EnvVars should remain the same
	if len(sb.EnvVars) != initialLen {
		t.Errorf("len(EnvVars) = %d, want %d", len(sb.EnvVars), initialLen)
	}
}

func TestEnvPluginSetupInjectsEnvVars(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "env-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewEnvPlugin()
	sb := sandbox.NewSandbox("test-session", tmpDir)
	ctx := context.Background()

	config := map[string]interface{}{
		"env_vars": map[string]interface{}{
			"KEY1":   "value1",
			"KEY2":   "value2",
			"NUMBER": 123, // Non-string values should be ignored
		},
	}

	if err := p.Setup(ctx, sb, config); err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	// Verify string values were added
	if sb.EnvVars["KEY1"] != "value1" {
		t.Errorf("EnvVars[KEY1] = %q, want %q", sb.EnvVars["KEY1"], "value1")
	}
	if sb.EnvVars["KEY2"] != "value2" {
		t.Errorf("EnvVars[KEY2] = %q, want %q", sb.EnvVars["KEY2"], "value2")
	}

	// Verify non-string values were ignored
	if _, exists := sb.EnvVars["NUMBER"]; exists {
		t.Error("Non-string value should not be added to EnvVars")
	}
}

func TestEnvPluginSetupEmptyEnvVars(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "env-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewEnvPlugin()
	sb := sandbox.NewSandbox("test-session", tmpDir)
	ctx := context.Background()

	config := map[string]interface{}{
		"env_vars": map[string]interface{}{}, // Empty env_vars
	}

	initialLen := len(sb.EnvVars)

	// Setup with empty env_vars should skip
	if err := p.Setup(ctx, sb, config); err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	// EnvVars should remain the same
	if len(sb.EnvVars) != initialLen {
		t.Errorf("len(EnvVars) = %d, want %d", len(sb.EnvVars), initialLen)
	}
}

func TestEnvPluginSetupWrongType(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "env-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewEnvPlugin()
	sb := sandbox.NewSandbox("test-session", tmpDir)
	ctx := context.Background()

	config := map[string]interface{}{
		"env_vars": "not a map", // Wrong type
	}

	initialLen := len(sb.EnvVars)

	// Setup with wrong type should skip gracefully
	if err := p.Setup(ctx, sb, config); err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	// EnvVars should remain the same
	if len(sb.EnvVars) != initialLen {
		t.Errorf("len(EnvVars) = %d, want %d", len(sb.EnvVars), initialLen)
	}
}

func TestEnvPluginTeardown(t *testing.T) {
	p := NewEnvPlugin()
	sb := sandbox.NewSandbox("test-session", "/tmp/sandbox")

	// Teardown should not error
	if err := p.Teardown(sb); err != nil {
		t.Errorf("Teardown() failed: %v", err)
	}
}
