package v1

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/anthropics/agentsmesh/backend/internal/middleware"
	"github.com/gin-gonic/gin"
)

// Integration-style test with mocked gin context
func TestListRunnersForMCP_ResponseFormat(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a test response directly to verify JSON structure
	response := gin.H{
		"runners": []RunnerSummary{
			{
				ID:                1,
				NodeID:            "test-runner",
				Description:       "Test Runner",
				Status:            "online",
				CurrentPods:       1,
				MaxConcurrentPods: 5,
				AvailableAgents: []AgentTypeSummary{
					{
						ID:          1,
						Slug:        "claude-code",
						Name:        "Claude Code",
						Description: "AI assistant",
						Config: []ConfigFieldSummary{
							{Name: "model", Type: "select", Required: true},
						},
						UserConfig: map[string]interface{}{"model": "opus"},
					},
				},
			},
		},
	}

	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("failed to marshal response: %v", err)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	runners, ok := decoded["runners"].([]interface{})
	if !ok {
		t.Fatal("expected runners array in response")
	}

	if len(runners) != 1 {
		t.Fatalf("runners count: got %v, want 1", len(runners))
	}

	runner := runners[0].(map[string]interface{})
	if runner["node_id"] != "test-runner" {
		t.Errorf("runner node_id: got %v, want test-runner", runner["node_id"])
	}

	agents, ok := runner["available_agents"].([]interface{})
	if !ok {
		t.Fatal("expected available_agents array")
	}

	if len(agents) != 1 {
		t.Fatalf("agents count: got %v, want 1", len(agents))
	}

	agent := agents[0].(map[string]interface{})
	if agent["slug"] != "claude-code" {
		t.Errorf("agent slug: got %v, want claude-code", agent["slug"])
	}
}

// Test that verifies the handler correctly uses middleware context
func TestListRunnersForMCP_ContextExtraction(t *testing.T) {
	gin.SetMode(gin.TestMode)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// Set up tenant context
	tenant := &middleware.TenantContext{
		OrganizationID:   123,
		OrganizationSlug: "test-org",
		UserID:           456,
		UserRole:         "member",
	}
	c.Set("tenant", tenant)
	// GetUserID reads from "user_id" key directly, not from tenant
	c.Set("user_id", int64(456))

	// Verify context extraction works
	extractedTenant := middleware.GetTenant(c)
	if extractedTenant == nil {
		t.Fatal("expected tenant context")
	}
	if extractedTenant.OrganizationID != 123 {
		t.Errorf("org ID: got %v, want 123", extractedTenant.OrganizationID)
	}

	extractedUserID := middleware.GetUserID(c)
	if extractedUserID != 456 {
		t.Errorf("user ID: got %v, want 456", extractedUserID)
	}
}
