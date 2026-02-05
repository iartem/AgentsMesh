package v1

import (
	"encoding/json"
	"testing"
)

func TestRunnerSummary_JSON(t *testing.T) {
	runner := RunnerSummary{
		ID:                1,
		NodeID:            "dev-machine",
		Description:       "Development runner",
		Status:            "online",
		CurrentPods:       2,
		MaxConcurrentPods: 5,
		AvailableAgents: []AgentTypeSummary{
			{
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
			},
		},
	}

	// Test JSON marshaling
	data, err := json.Marshal(runner)
	if err != nil {
		t.Fatalf("failed to marshal runner: %v", err)
	}

	// Test JSON unmarshaling
	var decoded RunnerSummary
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal runner: %v", err)
	}

	if decoded.ID != runner.ID {
		t.Errorf("decoded ID: got %v, want %v", decoded.ID, runner.ID)
	}
	if decoded.NodeID != runner.NodeID {
		t.Errorf("decoded NodeID: got %v, want %v", decoded.NodeID, runner.NodeID)
	}
	if len(decoded.AvailableAgents) != 1 {
		t.Fatalf("decoded AvailableAgents count: got %v, want 1", len(decoded.AvailableAgents))
	}

	agent := decoded.AvailableAgents[0]
	if agent.Slug != "claude-code" {
		t.Errorf("decoded agent Slug: got %v, want claude-code", agent.Slug)
	}
	if len(agent.Config) != 1 {
		t.Fatalf("decoded agent Config count: got %v, want 1", len(agent.Config))
	}
}

func TestRunnerSummary_OmitEmpty(t *testing.T) {
	// Test that omitempty fields are properly excluded
	runner := RunnerSummary{
		ID:                1,
		NodeID:            "test",
		Status:            "online",
		CurrentPods:       0,
		MaxConcurrentPods: 5,
		AvailableAgents:   []AgentTypeSummary{},
		// Description is empty, should be omitted
	}

	data, err := json.Marshal(runner)
	if err != nil {
		t.Fatalf("failed to marshal runner: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Description should be omitted when empty
	if _, exists := decoded["description"]; exists && decoded["description"] != "" {
		t.Error("description should be omitted when empty")
	}
}

func TestAgentTypeSummary_NilUserConfig(t *testing.T) {
	agent := AgentTypeSummary{
		ID:         1,
		Slug:       "test-agent",
		Name:       "Test Agent",
		Config:     nil,
		UserConfig: nil,
	}

	data, err := json.Marshal(agent)
	if err != nil {
		t.Fatalf("failed to marshal agent: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	// Config and UserConfig should be omitted when nil
	if _, exists := decoded["config"]; exists && decoded["config"] != nil {
		t.Error("config should be omitted when nil")
	}
	if _, exists := decoded["user_config"]; exists && decoded["user_config"] != nil {
		t.Error("user_config should be omitted when nil")
	}
}

// Test response JSON keys for token efficiency
func TestRunnerSummary_NoRedundantFields(t *testing.T) {
	runner := RunnerSummary{
		ID:                1,
		NodeID:            "test",
		Status:            "online",
		CurrentPods:       0,
		MaxConcurrentPods: 5,
		AvailableAgents:   []AgentTypeSummary{},
	}

	data, err := json.Marshal(runner)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)

	// Verify removed fields are not present
	removedFields := []string{
		"last_heartbeat",
		"runner_version",
		"is_enabled",
		"host_info",
		"created_at",
		"updated_at",
	}

	for _, field := range removedFields {
		if contains(jsonStr, "\""+field+"\"") {
			t.Errorf("field %s should not be present in JSON output", field)
		}
	}

	// Verify required fields are present
	requiredFields := []string{
		"id",
		"node_id",
		"status",
		"current_pods",
		"max_concurrent_pods",
		"available_agents",
	}

	for _, field := range requiredFields {
		if !contains(jsonStr, "\""+field+"\"") {
			t.Errorf("field %s should be present in JSON output", field)
		}
	}
}

func TestConfigFieldSummary_NoValidationOrShowWhen(t *testing.T) {
	config := ConfigFieldSummary{
		Name:     "model",
		Type:     "select",
		Default:  "default",
		Options:  []string{"a", "b"},
		Required: true,
	}

	data, err := json.Marshal(config)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	jsonStr := string(data)

	// Verify removed fields are not present
	removedFields := []string{
		"validation",
		"show_when",
	}

	for _, field := range removedFields {
		if contains(jsonStr, "\""+field+"\"") {
			t.Errorf("field %s should not be present in config JSON output", field)
		}
	}
}
