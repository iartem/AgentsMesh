package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/anthropics/agentmesh/runner/internal/sandbox"
)

// MCPPlugin generates MCP configuration and adds --mcp-config to launch args.
type MCPPlugin struct {
	mcpPort int // MCP HTTP Server port
}

// MCPConfig represents the MCP configuration file structure.
type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// MCPServerConfig represents a single MCP server configuration.
type MCPServerConfig struct {
	Type    string            `json:"type"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers,omitempty"`
}

// NewMCPPlugin creates a new MCPPlugin.
func NewMCPPlugin(mcpPort int) *MCPPlugin {
	if mcpPort == 0 {
		mcpPort = 19000 // Default port
	}
	return &MCPPlugin{
		mcpPort: mcpPort,
	}
}

func (p *MCPPlugin) Name() string {
	return "mcp"
}

func (p *MCPPlugin) Order() int {
	return 50 // After EnvPlugin (40)
}

func (p *MCPPlugin) Setup(ctx context.Context, sb *sandbox.Sandbox, config map[string]interface{}) error {
	// Generate MCP configuration
	mcpConfig := MCPConfig{
		MCPServers: map[string]MCPServerConfig{
			"agentmesh-collaboration": {
				Type: "http",
				URL:  fmt.Sprintf("http://127.0.0.1:%d/mcp", p.mcpPort),
				Headers: map[string]string{
					"X-Session-Key": sb.SessionKey,
				},
			},
		},
	}

	// Write config file
	configPath := filepath.Join(sb.RootPath, "mcp-config.json")
	configData, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal MCP config: %w", err)
	}

	if err := os.WriteFile(configPath, configData, 0644); err != nil {
		return fmt.Errorf("failed to write MCP config: %w", err)
	}

	// Add --mcp-config to launch args
	sb.LaunchArgs = append(sb.LaunchArgs, "--mcp-config", configPath)

	// Record metadata
	sb.Metadata["mcp_config_path"] = configPath
	sb.Metadata["mcp_port"] = p.mcpPort

	log.Printf("[mcp] Generated MCP config at %s (port: %d)", configPath, p.mcpPort)
	return nil
}

func (p *MCPPlugin) Teardown(sb *sandbox.Sandbox) error {
	// No cleanup needed - file will be removed with sandbox
	return nil
}
