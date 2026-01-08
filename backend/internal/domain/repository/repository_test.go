package repository

import (
	"testing"
	"time"
)

// --- Test Repository ---

func TestRepositoryTableName(t *testing.T) {
	r := Repository{}
	if r.TableName() != "repositories" {
		t.Errorf("expected 'repositories', got %s", r.TableName())
	}
}

func TestRepositoryStruct(t *testing.T) {
	now := time.Now()
	teamID := int64(10)
	ticketPrefix := "AM"

	r := Repository{
		ID:             1,
		OrganizationID: 100,
		TeamID:         &teamID,
		GitProviderID:  5,
		ExternalID:     "12345",
		Name:           "my-repo",
		FullPath:       "org/my-repo",
		DefaultBranch:  "main",
		TicketPrefix:   &ticketPrefix,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if r.ID != 1 {
		t.Errorf("expected ID 1, got %d", r.ID)
	}
	if r.OrganizationID != 100 {
		t.Errorf("expected OrganizationID 100, got %d", r.OrganizationID)
	}
	if *r.TeamID != 10 {
		t.Errorf("expected TeamID 10, got %d", *r.TeamID)
	}
	if r.Name != "my-repo" {
		t.Errorf("expected Name 'my-repo', got %s", r.Name)
	}
	if r.FullPath != "org/my-repo" {
		t.Errorf("expected FullPath 'org/my-repo', got %s", r.FullPath)
	}
	if r.DefaultBranch != "main" {
		t.Errorf("expected DefaultBranch 'main', got %s", r.DefaultBranch)
	}
	if *r.TicketPrefix != "AM" {
		t.Errorf("expected TicketPrefix 'AM', got %s", *r.TicketPrefix)
	}
}

func TestRepositoryWithNilOptionalFields(t *testing.T) {
	r := Repository{
		ID:             1,
		OrganizationID: 100,
		GitProviderID:  5,
		ExternalID:     "12345",
		Name:           "my-repo",
		FullPath:       "org/my-repo",
		DefaultBranch:  "main",
		IsActive:       true,
	}

	if r.TeamID != nil {
		t.Error("expected TeamID to be nil")
	}
	if r.TicketPrefix != nil {
		t.Error("expected TicketPrefix to be nil")
	}
}

// --- Test Branch ---

func TestBranchStruct(t *testing.T) {
	b := Branch{
		Name:      "feature/test",
		IsDefault: false,
		Commit:    "abc123def456",
	}

	if b.Name != "feature/test" {
		t.Errorf("expected Name 'feature/test', got %s", b.Name)
	}
	if b.IsDefault {
		t.Error("expected IsDefault false")
	}
	if b.Commit != "abc123def456" {
		t.Errorf("expected Commit 'abc123def456', got %s", b.Commit)
	}
}

func TestBranchDefaultBranch(t *testing.T) {
	b := Branch{
		Name:      "main",
		IsDefault: true,
		Commit:    "def456",
	}

	if !b.IsDefault {
		t.Error("expected IsDefault true")
	}
}

// --- Test WebhookConfig ---

func TestWebhookConfigStruct(t *testing.T) {
	wh := WebhookConfig{
		ID:        "wh-123",
		URL:       "https://example.com/webhook",
		Events:    []string{"push", "merge_request"},
		IsActive:  true,
		CreatedAt: "2024-01-01T00:00:00Z",
	}

	if wh.ID != "wh-123" {
		t.Errorf("expected ID 'wh-123', got %s", wh.ID)
	}
	if wh.URL != "https://example.com/webhook" {
		t.Errorf("expected URL 'https://example.com/webhook', got %s", wh.URL)
	}
	if len(wh.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(wh.Events))
	}
	if wh.Events[0] != "push" {
		t.Errorf("expected first event 'push', got %s", wh.Events[0])
	}
	if wh.Events[1] != "merge_request" {
		t.Errorf("expected second event 'merge_request', got %s", wh.Events[1])
	}
	if !wh.IsActive {
		t.Error("expected IsActive true")
	}
}

func TestWebhookConfigWithEmptyEvents(t *testing.T) {
	wh := WebhookConfig{
		ID:       "wh-456",
		URL:      "https://example.com/webhook",
		Events:   []string{},
		IsActive: false,
	}

	if len(wh.Events) != 0 {
		t.Errorf("expected 0 events, got %d", len(wh.Events))
	}
	if wh.IsActive {
		t.Error("expected IsActive false")
	}
}

// --- Benchmark Tests ---

func BenchmarkRepositoryTableName(b *testing.B) {
	r := Repository{}
	for i := 0; i < b.N; i++ {
		r.TableName()
	}
}

func BenchmarkBranchCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Branch{
			Name:      "feature/test",
			IsDefault: false,
			Commit:    "abc123def456",
		}
	}
}
