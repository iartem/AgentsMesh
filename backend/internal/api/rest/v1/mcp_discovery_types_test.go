package v1

import (
	"testing"
)

func TestMCPDiscoveryHandler_Types(t *testing.T) {
	// Test RunnerSummary structure
	runner := RunnerSummary{
		ID:                1,
		NodeID:            "test-node",
		Description:       "Test Runner",
		Status:            "online",
		CurrentPods:       2,
		MaxConcurrentPods: 5,
		AvailableAgents:   []AgentTypeSummary{},
	}

	if runner.ID != 1 {
		t.Errorf("ID: got %v, want 1", runner.ID)
	}
	if runner.NodeID != "test-node" {
		t.Errorf("NodeID: got %v, want test-node", runner.NodeID)
	}
	if runner.Status != "online" {
		t.Errorf("Status: got %v, want online", runner.Status)
	}
}

func TestAgentTypeSummary_Structure(t *testing.T) {
	agent := AgentTypeSummary{
		ID:          1,
		Slug:        "claude-code",
		Name:        "Claude Code",
		Description: "AI coding assistant",
		Config: []ConfigFieldSummary{
			{
				Name:     "model",
				Type:     "select",
				Default:  "claude-sonnet-4-20250514",
				Options:  []string{"claude-sonnet-4-20250514", "claude-opus-4-20250514"},
				Required: true,
			},
		},
		UserConfig: map[string]interface{}{
			"model": "claude-opus-4-20250514",
		},
	}

	if agent.ID != 1 {
		t.Errorf("ID: got %v, want 1", agent.ID)
	}
	if agent.Slug != "claude-code" {
		t.Errorf("Slug: got %v, want claude-code", agent.Slug)
	}
	if len(agent.Config) != 1 {
		t.Fatalf("Config count: got %v, want 1", len(agent.Config))
	}

	config := agent.Config[0]
	if config.Name != "model" {
		t.Errorf("Config Name: got %v, want model", config.Name)
	}
	if !config.Required {
		t.Error("Config Required: expected true")
	}
	if len(config.Options) != 2 {
		t.Errorf("Config Options count: got %v, want 2", len(config.Options))
	}
}

func TestConfigFieldSummary_Structure(t *testing.T) {
	tests := []struct {
		name     string
		field    ConfigFieldSummary
		wantName string
		wantType string
	}{
		{
			name: "select field",
			field: ConfigFieldSummary{
				Name:     "model",
				Type:     "select",
				Default:  "default-value",
				Options:  []string{"opt1", "opt2"},
				Required: true,
			},
			wantName: "model",
			wantType: "select",
		},
		{
			name: "number field",
			field: ConfigFieldSummary{
				Name:     "max_turns",
				Type:     "number",
				Default:  10,
				Required: false,
			},
			wantName: "max_turns",
			wantType: "number",
		},
		{
			name: "boolean field",
			field: ConfigFieldSummary{
				Name:     "auto_commit",
				Type:     "boolean",
				Default:  false,
				Required: false,
			},
			wantName: "auto_commit",
			wantType: "boolean",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.field.Name != tt.wantName {
				t.Errorf("Name: got %v, want %v", tt.field.Name, tt.wantName)
			}
			if tt.field.Type != tt.wantType {
				t.Errorf("Type: got %v, want %v", tt.field.Type, tt.wantType)
			}
		})
	}
}

func TestNewMCPDiscoveryHandler(t *testing.T) {
	// Test handler construction (nil services for type checking only)
	handler := NewMCPDiscoveryHandler(nil, nil, nil)

	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}
