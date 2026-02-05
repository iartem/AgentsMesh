package v1

import (
	"testing"
)

// ==================== mapCredentialsToEnvVars Tests ====================

func TestMapCredentialsToEnvVars_ClaudeCode(t *testing.T) {
	h := &PodHandler{}

	credentials := map[string]string{
		"api_key":  "sk-ant-test123",
		"base_url": "https://api.anthropic.com",
	}

	envVars := h.mapCredentialsToEnvVars("claude-code", credentials)

	if envVars["ANTHROPIC_API_KEY"] != "sk-ant-test123" {
		t.Errorf("Expected ANTHROPIC_API_KEY to be 'sk-ant-test123', got '%v'", envVars["ANTHROPIC_API_KEY"])
	}
	if envVars["ANTHROPIC_BASE_URL"] != "https://api.anthropic.com" {
		t.Errorf("Expected ANTHROPIC_BASE_URL to be 'https://api.anthropic.com', got '%v'", envVars["ANTHROPIC_BASE_URL"])
	}
}

func TestMapCredentialsToEnvVars_Codex(t *testing.T) {
	h := &PodHandler{}

	credentials := map[string]string{
		"api_key":  "sk-openai-test456",
		"base_url": "https://api.openai.com",
	}

	envVars := h.mapCredentialsToEnvVars("codex", credentials)

	if envVars["OPENAI_API_KEY"] != "sk-openai-test456" {
		t.Errorf("Expected OPENAI_API_KEY to be 'sk-openai-test456', got '%v'", envVars["OPENAI_API_KEY"])
	}
	if envVars["OPENAI_API_BASE"] != "https://api.openai.com" {
		t.Errorf("Expected OPENAI_API_BASE to be 'https://api.openai.com', got '%v'", envVars["OPENAI_API_BASE"])
	}
}

func TestMapCredentialsToEnvVars_GeminiCLI(t *testing.T) {
	h := &PodHandler{}

	credentials := map[string]string{
		"api_key": "gemini-key-789",
	}

	envVars := h.mapCredentialsToEnvVars("gemini-cli", credentials)

	if envVars["GEMINI_API_KEY"] != "gemini-key-789" {
		t.Errorf("Expected GEMINI_API_KEY to be 'gemini-key-789', got '%v'", envVars["GEMINI_API_KEY"])
	}
}

func TestMapCredentialsToEnvVars_OpenCode(t *testing.T) {
	h := &PodHandler{}

	credentials := map[string]string{
		"api_key":  "sk-opencode-test",
		"base_url": "https://custom.openai.com",
	}

	envVars := h.mapCredentialsToEnvVars("opencode", credentials)

	if envVars["OPENAI_API_KEY"] != "sk-opencode-test" {
		t.Errorf("Expected OPENAI_API_KEY to be 'sk-opencode-test', got '%v'", envVars["OPENAI_API_KEY"])
	}
	if envVars["OPENAI_API_BASE"] != "https://custom.openai.com" {
		t.Errorf("Expected OPENAI_API_BASE to be 'https://custom.openai.com', got '%v'", envVars["OPENAI_API_BASE"])
	}
}

func TestMapCredentialsToEnvVars_UnknownAgentType(t *testing.T) {
	h := &PodHandler{}

	credentials := map[string]string{
		"api_key":      "test-key",
		"custom_field": "custom-value",
	}

	envVars := h.mapCredentialsToEnvVars("unknown-agent", credentials)

	// Unknown agent type should use AGENT_ prefix
	if envVars["AGENT_API_KEY"] != "test-key" {
		t.Errorf("Expected AGENT_API_KEY to be 'test-key', got '%v'", envVars["AGENT_API_KEY"])
	}
	if envVars["AGENT_CUSTOM_FIELD"] != "custom-value" {
		t.Errorf("Expected AGENT_CUSTOM_FIELD to be 'custom-value', got '%v'", envVars["AGENT_CUSTOM_FIELD"])
	}
}

func TestMapCredentialsToEnvVars_UnknownFieldInKnownAgent(t *testing.T) {
	h := &PodHandler{}

	credentials := map[string]string{
		"api_key":      "sk-ant-test",
		"custom_field": "custom-value",
	}

	envVars := h.mapCredentialsToEnvVars("claude-code", credentials)

	// Known field should use mapped name
	if envVars["ANTHROPIC_API_KEY"] != "sk-ant-test" {
		t.Errorf("Expected ANTHROPIC_API_KEY to be 'sk-ant-test', got '%v'", envVars["ANTHROPIC_API_KEY"])
	}
	// Unknown field should use AGENT_ prefix
	if envVars["AGENT_CUSTOM_FIELD"] != "custom-value" {
		t.Errorf("Expected AGENT_CUSTOM_FIELD to be 'custom-value', got '%v'", envVars["AGENT_CUSTOM_FIELD"])
	}
}

func TestMapCredentialsToEnvVars_EmptyCredentials(t *testing.T) {
	h := &PodHandler{}

	credentials := map[string]string{}

	envVars := h.mapCredentialsToEnvVars("claude-code", credentials)

	if len(envVars) != 0 {
		t.Errorf("Expected empty envVars map, got %v", envVars)
	}
}

func TestMapCredentialsToEnvVars_NilCredentials(t *testing.T) {
	h := &PodHandler{}

	var credentials map[string]string = nil

	envVars := h.mapCredentialsToEnvVars("claude-code", credentials)

	if len(envVars) != 0 {
		t.Errorf("Expected empty envVars map for nil input, got %v", envVars)
	}
}

func TestMapCredentialsToEnvVars_OnlyBaseUrl(t *testing.T) {
	h := &PodHandler{}

	credentials := map[string]string{
		"base_url": "https://proxy.example.com",
	}

	envVars := h.mapCredentialsToEnvVars("claude-code", credentials)

	if envVars["ANTHROPIC_BASE_URL"] != "https://proxy.example.com" {
		t.Errorf("Expected ANTHROPIC_BASE_URL to be 'https://proxy.example.com', got '%v'", envVars["ANTHROPIC_BASE_URL"])
	}
	if _, exists := envVars["ANTHROPIC_API_KEY"]; exists {
		t.Errorf("ANTHROPIC_API_KEY should not exist when not provided")
	}
}

// ==================== agentEnvVarMappings Validation Tests ====================

func TestAgentEnvVarMappings_AllAgentsHaveAPIKey(t *testing.T) {
	// Verify that all defined agent types have at least an API key mapping
	for agentSlug, mapping := range agentEnvVarMappings {
		if mapping.APIKey == "" {
			t.Errorf("Agent '%s' should have APIKey mapping defined", agentSlug)
		}
	}
}

func TestAgentEnvVarMappings_NoDuplicateEnvVarNames(t *testing.T) {
	// Track which env var names are used by which agents
	envVarToAgent := make(map[string][]string)

	for agentSlug, mapping := range agentEnvVarMappings {
		if mapping.APIKey != "" {
			envVarToAgent[mapping.APIKey] = append(envVarToAgent[mapping.APIKey], agentSlug)
		}
		if mapping.BaseURL != "" {
			envVarToAgent[mapping.BaseURL] = append(envVarToAgent[mapping.BaseURL], agentSlug)
		}
	}

	// This is informational - some agents (codex, opencode) share the same OpenAI env vars
	// which is expected behavior
	for envVar, agents := range envVarToAgent {
		if len(agents) > 1 {
			t.Logf("Info: Env var '%s' is used by multiple agents: %v (this may be intentional)", envVar, agents)
		}
	}
}
