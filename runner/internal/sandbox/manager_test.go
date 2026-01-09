package sandbox

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	workspace := "/tmp/test-workspace"

	tests := []struct {
		name         string
		mcpPort      int
		expectedPort int
	}{
		{
			name:         "with custom port",
			mcpPort:      9999,
			expectedPort: 9999,
		},
		{
			name:         "with default port",
			mcpPort:      0,
			expectedPort: 19000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager(workspace, tt.mcpPort)

			if m.sandboxesDir != filepath.Join(workspace, "sandboxes") {
				t.Errorf("sandboxesDir = %q, want %q", m.sandboxesDir, filepath.Join(workspace, "sandboxes"))
			}
			if m.reposDir != filepath.Join(workspace, "repos") {
				t.Errorf("reposDir = %q, want %q", m.reposDir, filepath.Join(workspace, "repos"))
			}
			if m.mcpPort != tt.expectedPort {
				t.Errorf("mcpPort = %d, want %d", m.mcpPort, tt.expectedPort)
			}
			if m.plugins == nil {
				t.Error("plugins should not be nil")
			}
			if m.sandboxes == nil {
				t.Error("sandboxes should not be nil")
			}
		})
	}
}

func TestManagerRegisterPlugin(t *testing.T) {
	m := NewManager("/tmp/test", 19000)

	p1 := &mockPlugin{name: "plugin1", order: 10}
	p2 := &mockPlugin{name: "plugin2", order: 5}

	m.RegisterPlugin(p1)
	m.RegisterPlugin(p2)

	if len(m.plugins) != 2 {
		t.Errorf("len(plugins) = %d, want 2", len(m.plugins))
	}
}

func TestManagerGetReposDir(t *testing.T) {
	m := NewManager("/tmp/test-workspace", 19000)
	expected := "/tmp/test-workspace/repos"

	if m.GetReposDir() != expected {
		t.Errorf("GetReposDir() = %q, want %q", m.GetReposDir(), expected)
	}
}

func TestManagerGetMCPPort(t *testing.T) {
	m := NewManager("/tmp/test-workspace", 8888)

	if m.GetMCPPort() != 8888 {
		t.Errorf("GetMCPPort() = %d, want 8888", m.GetMCPPort())
	}
}

func TestManagerCreate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(tmpDir, 19000)

	// Register a simple mock plugin
	p := &mockPlugin{name: "test-plugin", order: 10}
	m.RegisterPlugin(p)

	ctx := context.Background()
	sessionKey := "test-session"
	config := map[string]interface{}{
		"test_key": "test_value",
	}

	// Create sandbox
	sb, err := m.Create(ctx, sessionKey, config)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Verify sandbox was created
	if sb.SessionKey != sessionKey {
		t.Errorf("SessionKey = %q, want %q", sb.SessionKey, sessionKey)
	}

	// Verify sandbox directory exists
	expectedPath := filepath.Join(tmpDir, "sandboxes", sessionKey)
	if sb.RootPath != expectedPath {
		t.Errorf("RootPath = %q, want %q", sb.RootPath, expectedPath)
	}
	if _, err := os.Stat(sb.RootPath); os.IsNotExist(err) {
		t.Error("Sandbox directory was not created")
	}

	// Verify plugin was executed
	if !p.setupCalled {
		t.Error("Plugin Setup() was not called")
	}

	// Verify sandbox is stored
	if len(m.sandboxes) != 1 {
		t.Errorf("len(sandboxes) = %d, want 1", len(m.sandboxes))
	}
}

func TestManagerCreateDuplicate(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(tmpDir, 19000)
	ctx := context.Background()
	sessionKey := "test-session"

	// Create first sandbox
	sb1, err := m.Create(ctx, sessionKey, nil)
	if err != nil {
		t.Fatalf("First Create() failed: %v", err)
	}

	// Create second sandbox with same key - should return existing
	sb2, err := m.Create(ctx, sessionKey, nil)
	if err != nil {
		t.Fatalf("Second Create() failed: %v", err)
	}

	if sb1 != sb2 {
		t.Error("Second Create() should return existing sandbox")
	}
}

func TestManagerCreatePluginFailure(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(tmpDir, 19000)

	// Register a failing plugin
	p := &failingPlugin{name: "failing-plugin"}
	m.RegisterPlugin(p)

	ctx := context.Background()
	sessionKey := "test-session"

	// Create should fail
	_, err = m.Create(ctx, sessionKey, nil)
	if err == nil {
		t.Error("Create() should fail when plugin fails")
	}

	// Sandbox directory should be cleaned up
	sandboxPath := filepath.Join(tmpDir, "sandboxes", sessionKey)
	if _, err := os.Stat(sandboxPath); !os.IsNotExist(err) {
		t.Error("Sandbox directory should be removed after plugin failure")
	}
}

func TestManagerGet(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(tmpDir, 19000)
	ctx := context.Background()
	sessionKey := "test-session"

	// Get non-existent sandbox
	_, exists := m.Get(sessionKey)
	if exists {
		t.Error("Get() should return false for non-existent sandbox")
	}

	// Create sandbox
	_, err = m.Create(ctx, sessionKey, nil)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Get existing sandbox
	sb, exists := m.Get(sessionKey)
	if !exists {
		t.Error("Get() should return true for existing sandbox")
	}
	if sb.SessionKey != sessionKey {
		t.Errorf("SessionKey = %q, want %q", sb.SessionKey, sessionKey)
	}
}

func TestManagerCleanup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(tmpDir, 19000)

	p := &mockPlugin{name: "test-plugin", order: 10}
	m.RegisterPlugin(p)

	ctx := context.Background()
	sessionKey := "test-session"

	// Create sandbox
	sb, err := m.Create(ctx, sessionKey, nil)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	sandboxPath := sb.RootPath

	// Cleanup
	if err := m.Cleanup(sessionKey); err != nil {
		t.Fatalf("Cleanup() failed: %v", err)
	}

	// Verify plugin teardown was called
	if !p.teardownCalled {
		t.Error("Plugin Teardown() was not called")
	}

	// Verify sandbox directory was removed
	if _, err := os.Stat(sandboxPath); !os.IsNotExist(err) {
		t.Error("Sandbox directory was not removed")
	}

	// Verify sandbox was removed from map
	_, exists := m.Get(sessionKey)
	if exists {
		t.Error("Sandbox should be removed from map after cleanup")
	}
}

func TestManagerCleanupNonExistent(t *testing.T) {
	m := NewManager("/tmp/test", 19000)

	// Cleanup non-existent sandbox should not error
	if err := m.Cleanup("non-existent"); err != nil {
		t.Errorf("Cleanup() should not fail for non-existent sandbox: %v", err)
	}
}

func TestManagerList(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(tmpDir, 19000)
	ctx := context.Background()

	// Empty list
	if len(m.List()) != 0 {
		t.Error("List() should be empty initially")
	}

	// Create sandboxes
	m.Create(ctx, "session-1", nil)
	m.Create(ctx, "session-2", nil)

	// Verify list
	list := m.List()
	if len(list) != 2 {
		t.Errorf("len(List()) = %d, want 2", len(list))
	}
}

func TestManagerPluginOrder(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(tmpDir, 19000)

	var setupOrder []string

	// Register plugins in random order
	p3 := &orderTrackingPlugin{name: "plugin3", order: 30, setupOrder: &setupOrder}
	p1 := &orderTrackingPlugin{name: "plugin1", order: 10, setupOrder: &setupOrder}
	p2 := &orderTrackingPlugin{name: "plugin2", order: 20, setupOrder: &setupOrder}

	m.RegisterPlugin(p3)
	m.RegisterPlugin(p1)
	m.RegisterPlugin(p2)

	ctx := context.Background()
	_, err = m.Create(ctx, "test-session", nil)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Verify plugins were executed in order
	expected := []string{"plugin1", "plugin2", "plugin3"}
	if len(setupOrder) != len(expected) {
		t.Fatalf("len(setupOrder) = %d, want %d", len(setupOrder), len(expected))
	}
	for i, name := range expected {
		if setupOrder[i] != name {
			t.Errorf("setupOrder[%d] = %q, want %q", i, setupOrder[i], name)
		}
	}
}

// Mock implementations for testing

type mockPlugin struct {
	name           string
	order          int
	setupCalled    bool
	teardownCalled bool
}

func (p *mockPlugin) Name() string { return p.name }
func (p *mockPlugin) Order() int   { return p.order }
func (p *mockPlugin) Setup(ctx context.Context, sb *Sandbox, config map[string]interface{}) error {
	p.setupCalled = true
	return nil
}
func (p *mockPlugin) Teardown(sb *Sandbox) error {
	p.teardownCalled = true
	return nil
}

type failingPlugin struct {
	name string
}

func (p *failingPlugin) Name() string { return p.name }
func (p *failingPlugin) Order() int   { return 10 }
func (p *failingPlugin) Setup(ctx context.Context, sb *Sandbox, config map[string]interface{}) error {
	return os.ErrPermission // Simulate error
}
func (p *failingPlugin) Teardown(sb *Sandbox) error {
	return nil
}

type orderTrackingPlugin struct {
	name       string
	order      int
	setupOrder *[]string
}

func (p *orderTrackingPlugin) Name() string { return p.name }
func (p *orderTrackingPlugin) Order() int   { return p.order }
func (p *orderTrackingPlugin) Setup(ctx context.Context, sb *Sandbox, config map[string]interface{}) error {
	*p.setupOrder = append(*p.setupOrder, p.name)
	return nil
}
func (p *orderTrackingPlugin) Teardown(sb *Sandbox) error {
	return nil
}

func TestManagerLoadExisting(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(tmpDir, 19000)
	ctx := context.Background()
	sessionKey := "test-session"

	// Create sandbox first
	sb, err := m.Create(ctx, sessionKey, nil)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}
	sb.WorkDir = "/some/work/dir"
	sb.EnvVars["TEST_KEY"] = "test_value"
	if err := sb.Save(); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Remove from manager's map to simulate restart
	m.mu.Lock()
	delete(m.sandboxes, sessionKey)
	m.mu.Unlock()

	// LoadExisting should restore from disk
	sb2, err := m.LoadExisting(sessionKey)
	if err != nil {
		t.Fatalf("LoadExisting() failed: %v", err)
	}

	if sb2.SessionKey != sessionKey {
		t.Errorf("SessionKey = %q, want %q", sb2.SessionKey, sessionKey)
	}
	if sb2.WorkDir != "/some/work/dir" {
		t.Errorf("WorkDir = %q, want %q", sb2.WorkDir, "/some/work/dir")
	}
	if sb2.EnvVars["TEST_KEY"] != "test_value" {
		t.Errorf("EnvVars[TEST_KEY] = %q, want %q", sb2.EnvVars["TEST_KEY"], "test_value")
	}
}

func TestManagerLoadExistingAlreadyLoaded(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(tmpDir, 19000)
	ctx := context.Background()
	sessionKey := "test-session"

	// Create sandbox
	sb1, err := m.Create(ctx, sessionKey, nil)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// LoadExisting should return the same sandbox
	sb2, err := m.LoadExisting(sessionKey)
	if err != nil {
		t.Fatalf("LoadExisting() failed: %v", err)
	}

	if sb1 != sb2 {
		t.Error("LoadExisting() should return existing sandbox")
	}
}

func TestManagerLoadExistingNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(tmpDir, 19000)

	// LoadExisting for non-existent sandbox should fail
	_, err = m.LoadExisting("non-existent")
	if err == nil {
		t.Error("LoadExisting() should fail for non-existent sandbox")
	}
}

func TestManagerTeardownPluginError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "manager-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	m := NewManager(tmpDir, 19000)

	// Register a plugin that fails on teardown
	p := &teardownFailingPlugin{name: "failing-teardown"}
	m.RegisterPlugin(p)

	ctx := context.Background()
	sessionKey := "test-session"

	// Create sandbox
	_, err = m.Create(ctx, sessionKey, nil)
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	// Cleanup should still succeed (teardown errors are logged but not fatal)
	if err := m.Cleanup(sessionKey); err != nil {
		t.Errorf("Cleanup() failed: %v", err)
	}
}

type teardownFailingPlugin struct {
	name string
}

func (p *teardownFailingPlugin) Name() string { return p.name }
func (p *teardownFailingPlugin) Order() int   { return 10 }
func (p *teardownFailingPlugin) Setup(ctx context.Context, sb *Sandbox, config map[string]interface{}) error {
	return nil
}
func (p *teardownFailingPlugin) Teardown(sb *Sandbox) error {
	return os.ErrPermission // Simulate error
}
