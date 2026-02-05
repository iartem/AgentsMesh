package agent

import (
	"testing"
)

func TestAgentBuilderRegistry(t *testing.T) {
	t.Run("creates with default builders", func(t *testing.T) {
		registry := NewAgentBuilderRegistry()

		// Check that all built-in builders are registered
		slugs := []string{"claude-code", "codex-cli", "gemini-cli", "aider", "opencode"}
		for _, slug := range slugs {
			if !registry.Has(slug) {
				t.Errorf("Registry should have builder for %s", slug)
			}
		}
	})

	t.Run("returns fallback for unknown slug", func(t *testing.T) {
		registry := NewAgentBuilderRegistry()

		builder := registry.Get("unknown-agent")
		if builder == nil {
			t.Error("Should return fallback builder for unknown slug")
		}
		if builder.Slug() != "default" {
			t.Errorf("Fallback builder slug = %s, want default", builder.Slug())
		}
	})

	t.Run("register custom builder", func(t *testing.T) {
		registry := NewAgentBuilderRegistry()

		customBuilder := NewBaseAgentBuilder("custom-agent")
		registry.Register(customBuilder)

		if !registry.Has("custom-agent") {
			t.Error("Should have custom-agent after registration")
		}
	})

	t.Run("list returns all slugs", func(t *testing.T) {
		registry := NewAgentBuilderRegistry()

		slugs := registry.List()
		if len(slugs) < 5 {
			t.Errorf("List should return at least 5 slugs, got %d", len(slugs))
		}
	})

	t.Run("set fallback", func(t *testing.T) {
		registry := NewAgentBuilderRegistry()

		customFallback := NewBaseAgentBuilder("custom-fallback")
		registry.SetFallback(customFallback)

		builder := registry.Get("unknown-agent")
		if builder.Slug() != "custom-fallback" {
			t.Errorf("Should use custom fallback, got %s", builder.Slug())
		}
	})
}

func TestBuilderSlugs(t *testing.T) {
	tests := []struct {
		name     string
		builder  AgentBuilder
		expected string
	}{
		{"ClaudeCode", NewClaudeCodeBuilder(), ClaudeCodeSlug},
		{"CodexCLI", NewCodexCLIBuilder(), CodexCLISlug},
		{"GeminiCLI", NewGeminiCLIBuilder(), GeminiCLISlug},
		{"Aider", NewAiderBuilder(), AiderSlug},
		{"OpenCode", NewOpenCodeBuilder(), OpenCodeSlug},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.builder.Slug() != tt.expected {
				t.Errorf("Slug() = %s, want %s", tt.builder.Slug(), tt.expected)
			}
		})
	}
}
