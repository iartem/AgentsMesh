package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// --- Test Config Struct ---

func TestConfigStruct(t *testing.T) {
	cfg := Config{
		Name:    "test-server",
		Command: "/usr/bin/node",
		Args:    []string{"server.js"},
		Env:     map[string]string{"NODE_ENV": "production"},
	}

	if cfg.Name != "test-server" {
		t.Errorf("Name: got %v, want test-server", cfg.Name)
	}

	if cfg.Command != "/usr/bin/node" {
		t.Errorf("Command: got %v, want /usr/bin/node", cfg.Command)
	}

	if len(cfg.Args) != 1 {
		t.Errorf("Args length: got %v, want 1", len(cfg.Args))
	}

	if cfg.Env["NODE_ENV"] != "production" {
		t.Errorf("Env[NODE_ENV]: got %v, want production", cfg.Env["NODE_ENV"])
	}
}

func TestConfigJSON(t *testing.T) {
	jsonStr := `{
		"name": "test-server",
		"command": "/usr/bin/node",
		"args": ["server.js"],
		"env": {"NODE_ENV": "production"}
	}`

	var cfg Config
	if err := json.Unmarshal([]byte(jsonStr), &cfg); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if cfg.Name != "test-server" {
		t.Errorf("Name: got %v, want test-server", cfg.Name)
	}

	if cfg.Command != "/usr/bin/node" {
		t.Errorf("Command: got %v, want /usr/bin/node", cfg.Command)
	}
}

// --- Test Tool Struct ---

func TestToolStruct(t *testing.T) {
	tool := Tool{
		Name:        "read_file",
		Description: "Read a file from disk",
		InputSchema: []byte(`{"type": "object"}`),
	}

	if tool.Name != "read_file" {
		t.Errorf("Name: got %v, want read_file", tool.Name)
	}

	if tool.Description != "Read a file from disk" {
		t.Errorf("Description: got %v, want Read a file from disk", tool.Description)
	}
}

func TestToolJSON(t *testing.T) {
	jsonStr := `{
		"name": "read_file",
		"description": "Read a file",
		"inputSchema": {"type": "object"}
	}`

	var tool Tool
	if err := json.Unmarshal([]byte(jsonStr), &tool); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if tool.Name != "read_file" {
		t.Errorf("Name: got %v, want read_file", tool.Name)
	}
}

// --- Test Resource Struct ---

func TestResourceStruct(t *testing.T) {
	res := Resource{
		URI:         "file:///tmp/test.txt",
		Name:        "test.txt",
		Description: "Test file",
		MimeType:    "text/plain",
	}

	if res.URI != "file:///tmp/test.txt" {
		t.Errorf("URI: got %v, want file:///tmp/test.txt", res.URI)
	}

	if res.Name != "test.txt" {
		t.Errorf("Name: got %v, want test.txt", res.Name)
	}

	if res.MimeType != "text/plain" {
		t.Errorf("MimeType: got %v, want text/plain", res.MimeType)
	}
}

func TestResourceJSON(t *testing.T) {
	jsonStr := `{
		"uri": "file:///tmp/test.txt",
		"name": "test.txt",
		"mimeType": "text/plain"
	}`

	var res Resource
	if err := json.Unmarshal([]byte(jsonStr), &res); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if res.URI != "file:///tmp/test.txt" {
		t.Errorf("URI: got %v, want file:///tmp/test.txt", res.URI)
	}
}

// --- Test Request Struct ---

func TestRequestStruct(t *testing.T) {
	req := Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
		Params:  []byte(`{}`),
	}

	if req.JSONRPC != "2.0" {
		t.Errorf("JSONRPC: got %v, want 2.0", req.JSONRPC)
	}

	if req.ID != 1 {
		t.Errorf("ID: got %v, want 1", req.ID)
	}

	if req.Method != "tools/list" {
		t.Errorf("Method: got %v, want tools/list", req.Method)
	}
}

func TestRequestJSON(t *testing.T) {
	jsonStr := `{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/list",
		"params": {}
	}`

	var req Request
	if err := json.Unmarshal([]byte(jsonStr), &req); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if req.Method != "tools/list" {
		t.Errorf("Method: got %v, want tools/list", req.Method)
	}
}

// --- Test Response Struct ---

func TestResponseStruct(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  []byte(`{"tools": []}`),
		Error:   nil,
	}

	if resp.JSONRPC != "2.0" {
		t.Errorf("JSONRPC: got %v, want 2.0", resp.JSONRPC)
	}

	if resp.ID != 1 {
		t.Errorf("ID: got %v, want 1", resp.ID)
	}

	if resp.Error != nil {
		t.Error("Error should be nil")
	}
}

func TestResponseWithError(t *testing.T) {
	resp := Response{
		JSONRPC: "2.0",
		ID:      1,
		Result:  nil,
		Error: &Error{
			Code:    -32600,
			Message: "Invalid Request",
		},
	}

	if resp.Error == nil {
		t.Error("Error should not be nil")
	}

	if resp.Error.Code != -32600 {
		t.Errorf("Error.Code: got %v, want -32600", resp.Error.Code)
	}

	if resp.Error.Message != "Invalid Request" {
		t.Errorf("Error.Message: got %v, want Invalid Request", resp.Error.Message)
	}
}

// --- Test Error Struct ---

func TestErrorStruct(t *testing.T) {
	err := Error{
		Code:    -32600,
		Message: "Invalid Request",
		Data:    []byte(`"additional info"`),
	}

	if err.Code != -32600 {
		t.Errorf("Code: got %v, want -32600", err.Code)
	}

	if err.Message != "Invalid Request" {
		t.Errorf("Message: got %v, want Invalid Request", err.Message)
	}
}

func TestErrorJSON(t *testing.T) {
	jsonStr := `{
		"code": -32600,
		"message": "Invalid Request",
		"data": "extra info"
	}`

	var e Error
	if err := json.Unmarshal([]byte(jsonStr), &e); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if e.Code != -32600 {
		t.Errorf("Code: got %v, want -32600", e.Code)
	}
}

// --- Test Server ---

func TestNewServer(t *testing.T) {
	cfg := &Config{
		Name:    "test-server",
		Command: "/usr/bin/echo",
		Args:    []string{"hello"},
		Env:     map[string]string{"KEY": "VALUE"},
	}

	server := NewServer(cfg)

	if server == nil {
		t.Fatal("NewServer returned nil")
	}

	if server.name != "test-server" {
		t.Errorf("name: got %v, want test-server", server.name)
	}

	if server.command != "/usr/bin/echo" {
		t.Errorf("command: got %v, want /usr/bin/echo", server.command)
	}

	if len(server.args) != 1 {
		t.Errorf("args length: got %v, want 1", len(server.args))
	}

	if server.env["KEY"] != "VALUE" {
		t.Errorf("env[KEY]: got %v, want VALUE", server.env["KEY"])
	}

	if server.pending == nil {
		t.Error("pending should be initialized")
	}

	if server.tools == nil {
		t.Error("tools should be initialized")
	}

	if server.resources == nil {
		t.Error("resources should be initialized")
	}
}

func TestServerName(t *testing.T) {
	cfg := &Config{
		Name:    "test-server",
		Command: "/usr/bin/echo",
	}

	server := NewServer(cfg)

	if server.Name() != "test-server" {
		t.Errorf("Name(): got %v, want test-server", server.Name())
	}
}

func TestServerIsRunningInitially(t *testing.T) {
	cfg := &Config{
		Name:    "test-server",
		Command: "/usr/bin/echo",
	}

	server := NewServer(cfg)

	if server.IsRunning() {
		t.Error("server should not be running initially")
	}
}

func TestServerGetToolsEmpty(t *testing.T) {
	cfg := &Config{
		Name:    "test-server",
		Command: "/usr/bin/echo",
	}

	server := NewServer(cfg)

	tools := server.GetTools()
	if len(tools) != 0 {
		t.Errorf("tools should be empty, got %v", len(tools))
	}
}

func TestServerGetResourcesEmpty(t *testing.T) {
	cfg := &Config{
		Name:    "test-server",
		Command: "/usr/bin/echo",
	}

	server := NewServer(cfg)

	resources := server.GetResources()
	if len(resources) != 0 {
		t.Errorf("resources should be empty, got %v", len(resources))
	}
}

func TestServerStopNotRunning(t *testing.T) {
	cfg := &Config{
		Name:    "test-server",
		Command: "/usr/bin/echo",
	}

	server := NewServer(cfg)

	// Should not error when stopping not running server
	err := server.Stop()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- Test Manager ---

func TestNewManager(t *testing.T) {
	manager := NewManager()

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.servers == nil {
		t.Error("servers should be initialized")
	}
}

func TestManagerAddServer(t *testing.T) {
	manager := NewManager()

	cfg := &Config{
		Name:    "test-server",
		Command: "/usr/bin/echo",
	}

	manager.AddServer(cfg)

	server, ok := manager.GetServer("test-server")
	if !ok {
		t.Error("server should be added")
	}

	if server.Name() != "test-server" {
		t.Errorf("Name: got %v, want test-server", server.Name())
	}
}

func TestManagerGetServerNotFound(t *testing.T) {
	manager := NewManager()

	_, ok := manager.GetServer("nonexistent")
	if ok {
		t.Error("should return false for nonexistent server")
	}
}

func TestManagerListServers(t *testing.T) {
	manager := NewManager()

	manager.AddServer(&Config{Name: "server-1", Command: "/usr/bin/echo"})
	manager.AddServer(&Config{Name: "server-2", Command: "/usr/bin/echo"})

	servers := manager.ListServers()
	if len(servers) != 2 {
		t.Errorf("servers count: got %v, want 2", len(servers))
	}
}

func TestManagerListServersEmpty(t *testing.T) {
	manager := NewManager()

	servers := manager.ListServers()
	if len(servers) != 0 {
		t.Errorf("servers should be empty, got %v", len(servers))
	}
}

func TestManagerStartServerNotFound(t *testing.T) {
	manager := NewManager()

	err := manager.StartServer(nil, "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent server")
	}
}

func TestManagerStopServerNotFound(t *testing.T) {
	manager := NewManager()

	err := manager.StopServer("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent server")
	}
}

func TestManagerStopAll(t *testing.T) {
	manager := NewManager()

	manager.AddServer(&Config{Name: "server-1", Command: "/usr/bin/echo"})
	manager.AddServer(&Config{Name: "server-2", Command: "/usr/bin/echo"})

	// Should not panic
	manager.StopAll()
}

func TestManagerGetAllToolsEmpty(t *testing.T) {
	manager := NewManager()

	tools := manager.GetAllTools()
	if len(tools) != 0 {
		t.Errorf("tools should be empty, got %v", len(tools))
	}
}

func TestManagerGetAllResourcesEmpty(t *testing.T) {
	manager := NewManager()

	resources := manager.GetAllResources()
	if len(resources) != 0 {
		t.Errorf("resources should be empty, got %v", len(resources))
	}
}

func TestManagerCallToolServerNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.CallTool(nil, "nonexistent", "tool", nil)
	if err == nil {
		t.Error("expected error for nonexistent server")
	}
}

func TestManagerCallToolServerNotRunning(t *testing.T) {
	manager := NewManager()
	manager.AddServer(&Config{Name: "test-server", Command: "/usr/bin/echo"})

	_, err := manager.CallTool(nil, "test-server", "tool", nil)
	if err == nil {
		t.Error("expected error for server not running")
	}
}

func TestManagerReadResourceServerNotFound(t *testing.T) {
	manager := NewManager()

	_, _, err := manager.ReadResource(nil, "nonexistent", "file://test")
	if err == nil {
		t.Error("expected error for nonexistent server")
	}
}

func TestManagerReadResourceServerNotRunning(t *testing.T) {
	manager := NewManager()
	manager.AddServer(&Config{Name: "test-server", Command: "/usr/bin/echo"})

	_, _, err := manager.ReadResource(nil, "test-server", "file://test")
	if err == nil {
		t.Error("expected error for server not running")
	}
}

func TestManagerGetStatus(t *testing.T) {
	manager := NewManager()
	manager.AddServer(&Config{Name: "server-1", Command: "/usr/bin/echo"})
	manager.AddServer(&Config{Name: "server-2", Command: "/usr/bin/echo"})

	statuses := manager.GetStatus()
	if len(statuses) != 2 {
		t.Errorf("statuses count: got %v, want 2", len(statuses))
	}

	// All should be not running
	for _, status := range statuses {
		if status.Running {
			t.Errorf("server %s should not be running", status.Name)
		}
	}
}

func TestManagerGetStatusEmpty(t *testing.T) {
	manager := NewManager()

	statuses := manager.GetStatus()
	if len(statuses) != 0 {
		t.Errorf("statuses should be empty, got %v", len(statuses))
	}
}

func TestManagerLoadConfigNonExistent(t *testing.T) {
	manager := NewManager()

	err := manager.LoadConfig("/nonexistent/path/config.json")
	if err != nil {
		t.Errorf("should not error for nonexistent file: %v", err)
	}
}

func TestManagerLoadConfigInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.json")

	os.WriteFile(configPath, []byte("invalid json"), 0644)

	manager := NewManager()
	err := manager.LoadConfig(configPath)

	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestManagerLoadConfigValid(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	config := `{
		"mcpServers": {
			"server-1": {
				"command": "/usr/bin/echo",
				"args": ["hello"]
			},
			"server-2": {
				"command": "/usr/bin/cat"
			}
		}
	}`

	os.WriteFile(configPath, []byte(config), 0644)

	manager := NewManager()
	err := manager.LoadConfig(configPath)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	servers := manager.ListServers()
	if len(servers) != 2 {
		t.Errorf("servers count: got %v, want 2", len(servers))
	}

	// Verify servers were added
	_, ok1 := manager.GetServer("server-1")
	_, ok2 := manager.GetServer("server-2")

	if !ok1 || !ok2 {
		t.Error("servers should be added from config")
	}
}

// --- Test ServerStatus Struct ---

func TestServerStatusStruct(t *testing.T) {
	status := ServerStatus{
		Name:    "test-server",
		Running: true,
		Tools: []*Tool{
			{Name: "tool1", Description: "Tool 1"},
		},
		Resources: []*Resource{
			{URI: "file://test.txt", Name: "test.txt"},
		},
	}

	if status.Name != "test-server" {
		t.Errorf("Name: got %v, want test-server", status.Name)
	}

	if !status.Running {
		t.Error("Running should be true")
	}

	if len(status.Tools) != 1 {
		t.Errorf("Tools count: got %v, want 1", len(status.Tools))
	}

	if len(status.Resources) != 1 {
		t.Errorf("Resources count: got %v, want 1", len(status.Resources))
	}
}

func TestServerStatusJSON(t *testing.T) {
	status := ServerStatus{
		Name:    "test-server",
		Running: false,
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var unmarshaled ServerStatus
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if unmarshaled.Name != "test-server" {
		t.Errorf("Name: got %v, want test-server", unmarshaled.Name)
	}
}

// --- Benchmark Tests ---

func BenchmarkNewManager(b *testing.B) {
	for i := 0; i < b.N; i++ {
		NewManager()
	}
}

func BenchmarkManagerAddServer(b *testing.B) {
	manager := NewManager()
	cfg := &Config{Name: "test", Command: "/usr/bin/echo"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.AddServer(cfg)
	}
}

func BenchmarkManagerListServers(b *testing.B) {
	manager := NewManager()
	for i := 0; i < 10; i++ {
		manager.AddServer(&Config{
			Name:    "server-" + string(rune('0'+i)),
			Command: "/usr/bin/echo",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.ListServers()
	}
}

func BenchmarkManagerGetServer(b *testing.B) {
	manager := NewManager()
	manager.AddServer(&Config{Name: "test-server", Command: "/usr/bin/echo"})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetServer("test-server")
	}
}

func BenchmarkNewServer(b *testing.B) {
	cfg := &Config{
		Name:    "test-server",
		Command: "/usr/bin/echo",
		Args:    []string{"hello"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewServer(cfg)
	}
}
