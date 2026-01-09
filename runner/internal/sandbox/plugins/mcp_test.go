package plugins

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/anthropics/agentmesh/runner/internal/sandbox"
)

func TestNewMCPPlugin(t *testing.T) {
	tests := []struct {
		name         string
		port         int
		expectedPort int
	}{
		{
			name:         "with custom port",
			port:         8080,
			expectedPort: 8080,
		},
		{
			name:         "with default port",
			port:         0,
			expectedPort: 19000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewMCPPlugin(tt.port)

			if p == nil {
				t.Fatal("NewMCPPlugin() returned nil")
			}
			if p.mcpPort != tt.expectedPort {
				t.Errorf("mcpPort = %d, want %d", p.mcpPort, tt.expectedPort)
			}
		})
	}
}

func TestMCPPluginName(t *testing.T) {
	p := NewMCPPlugin(19000)

	if p.Name() != "mcp" {
		t.Errorf("Name() = %q, want %q", p.Name(), "mcp")
	}
}

func TestMCPPluginOrder(t *testing.T) {
	p := NewMCPPlugin(19000)

	if p.Order() != 50 {
		t.Errorf("Order() = %d, want 50", p.Order())
	}
}

func TestMCPPluginSetup(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mcp-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewMCPPlugin(9999)
	sb := sandbox.NewSandbox("test-session-123", tmpDir)
	ctx := context.Background()

	if err := p.Setup(ctx, sb, nil); err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	// Verify mcp-config.json was created
	configPath := filepath.Join(tmpDir, "mcp-config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("mcp-config.json was not created")
	}

	// Verify config content
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config file: %v", err)
	}

	var config MCPConfig
	if err := json.Unmarshal(content, &config); err != nil {
		t.Fatalf("Failed to parse config JSON: %v", err)
	}

	// Verify server configuration
	server, exists := config.MCPServers["agentmesh-collaboration"]
	if !exists {
		t.Fatal("agentmesh-collaboration server not found in config")
	}

	if server.Type != "http" {
		t.Errorf("Type = %q, want %q", server.Type, "http")
	}

	expectedURL := "http://127.0.0.1:9999/mcp"
	if server.URL != expectedURL {
		t.Errorf("URL = %q, want %q", server.URL, expectedURL)
	}

	if server.Headers["X-Session-Key"] != "test-session-123" {
		t.Errorf("X-Session-Key = %q, want %q", server.Headers["X-Session-Key"], "test-session-123")
	}

	// Verify LaunchArgs
	if len(sb.LaunchArgs) != 2 {
		t.Fatalf("len(LaunchArgs) = %d, want 2", len(sb.LaunchArgs))
	}
	if sb.LaunchArgs[0] != "--mcp-config" {
		t.Errorf("LaunchArgs[0] = %q, want %q", sb.LaunchArgs[0], "--mcp-config")
	}
	if sb.LaunchArgs[1] != configPath {
		t.Errorf("LaunchArgs[1] = %q, want %q", sb.LaunchArgs[1], configPath)
	}

	// Verify metadata
	if sb.Metadata["mcp_config_path"] != configPath {
		t.Errorf("Metadata[mcp_config_path] = %q, want %q", sb.Metadata["mcp_config_path"], configPath)
	}
	if sb.Metadata["mcp_port"] != 9999 {
		t.Errorf("Metadata[mcp_port] = %v, want 9999", sb.Metadata["mcp_port"])
	}
}

func TestMCPPluginSetupConfigFormat(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mcp-plugin-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	p := NewMCPPlugin(19000)
	sb := sandbox.NewSandbox("my-session", tmpDir)
	ctx := context.Background()

	if err := p.Setup(ctx, sb, nil); err != nil {
		t.Fatalf("Setup() failed: %v", err)
	}

	// Read and verify JSON format
	content, err := os.ReadFile(filepath.Join(tmpDir, "mcp-config.json"))
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	// Verify it's valid JSON with proper formatting (indented)
	var raw map[string]interface{}
	if err := json.Unmarshal(content, &raw); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	// Check structure
	servers, ok := raw["mcpServers"].(map[string]interface{})
	if !ok {
		t.Fatal("mcpServers not found or wrong type")
	}

	collaboration, ok := servers["agentmesh-collaboration"].(map[string]interface{})
	if !ok {
		t.Fatal("agentmesh-collaboration server not found")
	}

	// Verify all required fields
	if collaboration["type"] != "http" {
		t.Errorf("type = %v, want http", collaboration["type"])
	}
	if collaboration["url"] != "http://127.0.0.1:19000/mcp" {
		t.Errorf("url = %v, want http://127.0.0.1:19000/mcp", collaboration["url"])
	}

	headers, ok := collaboration["headers"].(map[string]interface{})
	if !ok {
		t.Fatal("headers not found")
	}
	if headers["X-Session-Key"] != "my-session" {
		t.Errorf("X-Session-Key = %v, want my-session", headers["X-Session-Key"])
	}
}

func TestMCPPluginTeardown(t *testing.T) {
	p := NewMCPPlugin(19000)
	sb := sandbox.NewSandbox("test-session", "/tmp/sandbox")

	// Teardown should not error
	if err := p.Teardown(sb); err != nil {
		t.Errorf("Teardown() failed: %v", err)
	}
}

func TestMCPConfigStructure(t *testing.T) {
	// Test MCPConfig structure serialization
	config := MCPConfig{
		MCPServers: map[string]MCPServerConfig{
			"test-server": {
				Type: "http",
				URL:  "http://localhost:8080/mcp",
				Headers: map[string]string{
					"X-Custom-Header": "value",
				},
			},
		},
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("Failed to marshal MCPConfig: %v", err)
	}

	// Verify JSON keys
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Failed to unmarshal: %v", err)
	}

	if _, ok := raw["mcpServers"]; !ok {
		t.Error("Expected 'mcpServers' key in JSON")
	}
}

func TestMCPPluginSetupWriteError(t *testing.T) {
	// Test error case when config file cannot be written
	p := NewMCPPlugin(19000)
	// Use a path that doesn't exist and can't be created
	sb := sandbox.NewSandbox("test-session", "/nonexistent/path/that/cannot/be/written")
	ctx := context.Background()

	// Setup should fail when writing config file
	err := p.Setup(ctx, sb, nil)
	if err == nil {
		t.Error("Setup() should fail when config file cannot be written")
	}
}
