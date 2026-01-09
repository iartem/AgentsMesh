package plugins

import (
	"context"

	"github.com/anthropics/agentmesh/runner/internal/sandbox"
)

// EnvPlugin injects environment variables into the sandbox.
type EnvPlugin struct{}

// NewEnvPlugin creates a new EnvPlugin.
func NewEnvPlugin() *EnvPlugin {
	return &EnvPlugin{}
}

func (p *EnvPlugin) Name() string {
	return "env"
}

func (p *EnvPlugin) Order() int {
	return 40 // After InitScriptPlugin (30)
}

func (p *EnvPlugin) Setup(ctx context.Context, sb *sandbox.Sandbox, config map[string]interface{}) error {
	envVars := sandbox.GetMapConfig(config, "env_vars")
	if envVars == nil || len(envVars) == 0 {
		return nil
	}

	for k, v := range envVars {
		if strVal, ok := v.(string); ok {
			sb.EnvVars[k] = strVal
		}
	}

	return nil
}

func (p *EnvPlugin) Teardown(sb *sandbox.Sandbox) error {
	// No cleanup needed
	return nil
}
