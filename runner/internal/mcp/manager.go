package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// Manager manages multiple MCP servers
type Manager struct {
	servers map[string]*Server
	mu      sync.RWMutex
}

// NewManager creates a new MCP manager
func NewManager() *Manager {
	return &Manager{
		servers: make(map[string]*Server),
	}
}

// LoadConfig loads MCP server configurations from a JSON file
func (m *Manager) LoadConfig(configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No config file is okay
		}
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config struct {
		MCPServers map[string]Config `json:"mcpServers"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	for name, cfg := range config.MCPServers {
		cfg.Name = name
		m.AddServer(&cfg)
	}

	return nil
}

// AddServer adds an MCP server configuration
func (m *Manager) AddServer(cfg *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()

	server := NewServer(cfg)
	m.servers[cfg.Name] = server
}

// StartServer starts a specific MCP server
func (m *Manager) StartServer(ctx context.Context, name string) error {
	m.mu.RLock()
	server, ok := m.servers[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("server not found: %s", name)
	}

	return server.Start(ctx)
}

// StopServer stops a specific MCP server
func (m *Manager) StopServer(name string) error {
	m.mu.RLock()
	server, ok := m.servers[name]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("server not found: %s", name)
	}

	return server.Stop()
}

// StartAll starts all configured MCP servers
func (m *Manager) StartAll(ctx context.Context) error {
	m.mu.RLock()
	servers := make([]*Server, 0, len(m.servers))
	for _, s := range m.servers {
		servers = append(servers, s)
	}
	m.mu.RUnlock()

	var firstErr error
	for _, server := range servers {
		if err := server.Start(ctx); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("failed to start %s: %w", server.Name(), err)
			}
		}
	}

	return firstErr
}

// StopAll stops all running MCP servers
func (m *Manager) StopAll() {
	m.mu.RLock()
	servers := make([]*Server, 0, len(m.servers))
	for _, s := range m.servers {
		servers = append(servers, s)
	}
	m.mu.RUnlock()

	for _, server := range servers {
		server.Stop()
	}
}

// GetServer returns a server by name
func (m *Manager) GetServer(name string) (*Server, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	server, ok := m.servers[name]
	return server, ok
}

// ListServers returns all server names
func (m *Manager) ListServers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.servers))
	for name := range m.servers {
		names = append(names, name)
	}
	return names
}

// GetAllTools returns all tools from all running servers
func (m *Manager) GetAllTools() map[string][]*Tool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]*Tool)
	for name, server := range m.servers {
		if server.IsRunning() {
			result[name] = server.GetTools()
		}
	}
	return result
}

// GetAllResources returns all resources from all running servers
func (m *Manager) GetAllResources() map[string][]*Resource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string][]*Resource)
	for name, server := range m.servers {
		if server.IsRunning() {
			result[name] = server.GetResources()
		}
	}
	return result
}

// CallTool calls a tool on a specific server
func (m *Manager) CallTool(ctx context.Context, serverName, toolName string, arguments map[string]interface{}) (json.RawMessage, error) {
	m.mu.RLock()
	server, ok := m.servers[serverName]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("server not found: %s", serverName)
	}

	if !server.IsRunning() {
		return nil, fmt.Errorf("server not running: %s", serverName)
	}

	return server.CallTool(ctx, toolName, arguments)
}

// ReadResource reads a resource from a specific server
func (m *Manager) ReadResource(ctx context.Context, serverName, uri string) ([]byte, string, error) {
	m.mu.RLock()
	server, ok := m.servers[serverName]
	m.mu.RUnlock()

	if !ok {
		return nil, "", fmt.Errorf("server not found: %s", serverName)
	}

	if !server.IsRunning() {
		return nil, "", fmt.Errorf("server not running: %s", serverName)
	}

	return server.ReadResource(ctx, uri)
}

// ServerStatus represents the status of an MCP server
type ServerStatus struct {
	Name      string      `json:"name"`
	Running   bool        `json:"running"`
	Tools     []*Tool     `json:"tools,omitempty"`
	Resources []*Resource `json:"resources,omitempty"`
}

// GetStatus returns the status of all servers
func (m *Manager) GetStatus() []ServerStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	statuses := make([]ServerStatus, 0, len(m.servers))
	for name, server := range m.servers {
		status := ServerStatus{
			Name:    name,
			Running: server.IsRunning(),
		}
		if server.IsRunning() {
			status.Tools = server.GetTools()
			status.Resources = server.GetResources()
		}
		statuses = append(statuses, status)
	}
	return statuses
}
