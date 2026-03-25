package agent

import (
	"encoding/json"
	"fmt"

	runnerv1 "github.com/anthropics/agentsmesh/proto/gen/go/runner/v1"
)

const OpenCodeSlug = "opencode"

// OpenCodeBuilder is the builder for OpenCode agent.
// OpenCode CLI syntax: opencode [prompt] [options]
// Similar to Claude Code, the prompt comes before options.
type OpenCodeBuilder struct {
	*BaseAgentBuilder
}

// NewOpenCodeBuilder creates a new OpenCodeBuilder
func NewOpenCodeBuilder() *OpenCodeBuilder {
	return &OpenCodeBuilder{
		BaseAgentBuilder: NewBaseAgentBuilder(OpenCodeSlug),
	}
}

// Slug returns the agent type identifier
func (b *OpenCodeBuilder) Slug() string {
	return OpenCodeSlug
}

// HandleInitialPrompt prepends the initial prompt to launch arguments.
// OpenCode syntax: opencode [prompt] [options]
func (b *OpenCodeBuilder) HandleInitialPrompt(ctx *BuildContext, args []string) []string {
	if ctx.Request.InitialPrompt != "" {
		return append([]string{ctx.Request.InitialPrompt}, args...)
	}
	return args
}

// BuildLaunchArgs uses the base implementation
func (b *OpenCodeBuilder) BuildLaunchArgs(ctx *BuildContext) ([]string, error) {
	return b.BaseAgentBuilder.BuildLaunchArgs(ctx)
}

// SupportsMcp returns true - OpenCode supports MCP servers
func (b *OpenCodeBuilder) SupportsMcp() bool { return true }

// BuildFilesToCreate creates OpenCode-specific files including MCP configuration.
func (b *OpenCodeBuilder) BuildFilesToCreate(ctx *BuildContext) ([]*runnerv1.FileToCreate, error) {
	files, err := b.BaseAgentBuilder.BuildFilesToCreate(ctx)
	if err != nil {
		return nil, err
	}

	mcpConfig := b.buildMcpConfig(ctx)
	if mcpConfig == nil {
		return files, nil
	}

	configJSON, err := json.MarshalIndent(mcpConfig, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal opencode config: %w", err)
	}

	files = append(files, &runnerv1.FileToCreate{
		Path:    "{{.sandbox.work_dir}}/opencode.json",
		Content: string(configJSON),
		Mode:    0644,
	})

	return files, nil
}

// buildMcpConfig builds the OpenCode MCP server configuration.
// Returns nil if no MCP servers are configured.
func (b *OpenCodeBuilder) buildMcpConfig(ctx *BuildContext) map[string]interface{} {
	servers := make(map[string]interface{})

	if mcpPort, ok := ctx.TemplateCtx["mcp_port"]; ok && mcpPort != nil {
		if port, ok := mcpPort.(int); ok && port > 0 {
			agentsmeshCfg := map[string]interface{}{
				"type":    "remote",
				"url":     fmt.Sprintf("http://127.0.0.1:%d/mcp", port),
				"enabled": true,
			}
			if podKey, ok := ctx.TemplateCtx["pod_key"]; ok && podKey != nil {
				if key, ok := podKey.(string); ok && key != "" {
					agentsmeshCfg["headers"] = map[string]string{
						"X-Pod-Key": key,
					}
				}
			}
			servers["agentsmesh"] = agentsmeshCfg
		}
	}

	for _, srv := range ctx.McpServers {
		if !srv.IsEnabled {
			continue
		}

		switch srv.TransportType {
		case "stdio", "http", "sse":
		default:
			continue
		}

		serverConfig := srv.ToMcpConfig()
		if len(serverConfig) == 0 {
			continue
		}

		if _, hasType := serverConfig["type"]; !hasType {
			serverConfig["type"] = srv.TransportType
		}

		servers[srv.Slug] = serverConfig
	}

	if len(servers) == 0 {
		return nil
	}

	return map[string]interface{}{
		"$schema": "https://opencode.ai/config.json",
		"mcp":     servers,
	}
}

// BuildEnvVars uses the base implementation
func (b *OpenCodeBuilder) BuildEnvVars(ctx *BuildContext) (map[string]string, error) {
	return b.BaseAgentBuilder.BuildEnvVars(ctx)
}

// PostProcess uses the base implementation
func (b *OpenCodeBuilder) PostProcess(ctx *BuildContext, cmd *runnerv1.CreatePodCommand) error {
	return b.BaseAgentBuilder.PostProcess(ctx, cmd)
}
