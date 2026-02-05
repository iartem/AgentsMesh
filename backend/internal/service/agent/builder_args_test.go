package agent

import (
	"testing"

	"github.com/anthropics/agentsmesh/backend/internal/domain/agent"
)

func TestBaseAgentBuilder_BuildLaunchArgs(t *testing.T) {
	builder := NewBaseAgentBuilder("test")

	t.Run("builds args from command template", func(t *testing.T) {
		ctx := &BuildContext{
			AgentType: &agent.AgentType{
				CommandTemplate: agent.CommandTemplate{
					Args: []agent.ArgRule{
						{Args: []string{"--model", "opus"}},
						{Args: []string{"--verbose"}},
					},
				},
			},
			Config:      agent.ConfigValues{},
			TemplateCtx: map[string]interface{}{},
		}

		args, err := builder.BuildLaunchArgs(ctx)
		if err != nil {
			t.Fatalf("BuildLaunchArgs failed: %v", err)
		}

		if len(args) != 3 {
			t.Fatalf("Args length = %d, want 3", len(args))
		}
		if args[0] != "--model" || args[1] != "opus" || args[2] != "--verbose" {
			t.Errorf("Args = %v, unexpected values", args)
		}
	})

	t.Run("skips args when condition not met", func(t *testing.T) {
		ctx := &BuildContext{
			AgentType: &agent.AgentType{
				CommandTemplate: agent.CommandTemplate{
					Args: []agent.ArgRule{
						{
							Condition: &agent.Condition{
								Field:    "debug",
								Operator: "eq",
								Value:    true,
							},
							Args: []string{"--debug"},
						},
					},
				},
			},
			Config:      agent.ConfigValues{"debug": false},
			TemplateCtx: map[string]interface{}{},
		}

		args, err := builder.BuildLaunchArgs(ctx)
		if err != nil {
			t.Fatalf("BuildLaunchArgs failed: %v", err)
		}

		if len(args) != 0 {
			t.Errorf("Args should be empty when condition not met, got %v", args)
		}
	})

	t.Run("renders template variables", func(t *testing.T) {
		ctx := &BuildContext{
			AgentType: &agent.AgentType{
				CommandTemplate: agent.CommandTemplate{
					Args: []agent.ArgRule{
						{Args: []string{"--model", "{{.config.model}}"}},
					},
				},
			},
			Config: agent.ConfigValues{"model": "sonnet"},
			TemplateCtx: map[string]interface{}{
				"config": agent.ConfigValues{"model": "sonnet"},
			},
		}

		args, err := builder.BuildLaunchArgs(ctx)
		if err != nil {
			t.Fatalf("BuildLaunchArgs failed: %v", err)
		}

		if len(args) != 2 {
			t.Fatalf("Args length = %d, want 2", len(args))
		}
		if args[1] != "sonnet" {
			t.Errorf("Model arg = %s, want sonnet", args[1])
		}
	})
}
