package gitprovider

import (
	"encoding/json"
	"testing"
	"time"
)

// ===========================================
// WebhookConfig Tests
// ===========================================

func TestWebhookConfig_ToStatus_Nil(t *testing.T) {
	var wc *WebhookConfig
	status := wc.ToStatus()

	if status == nil {
		t.Fatal("expected non-nil status")
	}
	if status.Registered {
		t.Error("expected Registered to be false for nil config")
	}
}

func TestWebhookConfig_ToStatus_AutoRegistered(t *testing.T) {
	wc := &WebhookConfig{
		ID:        "wh_123",
		URL:       "https://example.com/webhooks/org/gitlab/1",
		Secret:    "secret123",
		Events:    []string{"merge_request", "pipeline"},
		IsActive:  true,
		CreatedAt: "2026-02-06T10:00:00Z",
	}

	status := wc.ToStatus()

	if !status.Registered {
		t.Error("expected Registered to be true")
	}
	if status.WebhookID != "wh_123" {
		t.Errorf("unexpected WebhookID: %s", status.WebhookID)
	}
	if status.WebhookURL != "https://example.com/webhooks/org/gitlab/1" {
		t.Errorf("unexpected WebhookURL: %s", status.WebhookURL)
	}
	if len(status.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(status.Events))
	}
	if !status.IsActive {
		t.Error("expected IsActive to be true")
	}
	if status.NeedsManualSetup {
		t.Error("expected NeedsManualSetup to be false")
	}
	if status.RegisteredAt != "2026-02-06T10:00:00Z" {
		t.Errorf("unexpected RegisteredAt: %s", status.RegisteredAt)
	}
}

func TestWebhookConfig_ToStatus_ManualSetup(t *testing.T) {
	wc := &WebhookConfig{
		URL:              "https://example.com/webhooks/org/gitlab/1",
		Secret:           "secret123",
		Events:           []string{"merge_request", "pipeline"},
		IsActive:         false,
		NeedsManualSetup: true,
		LastError:        "OAuth token not available",
	}

	status := wc.ToStatus()

	// NeedsManualSetup should make Registered true (config exists, just needs manual setup)
	if !status.Registered {
		t.Error("expected Registered to be true when NeedsManualSetup is true")
	}
	if status.WebhookID != "" {
		t.Errorf("expected empty WebhookID, got %s", status.WebhookID)
	}
	if !status.NeedsManualSetup {
		t.Error("expected NeedsManualSetup to be true")
	}
	if status.IsActive {
		t.Error("expected IsActive to be false")
	}
	if status.LastError != "OAuth token not available" {
		t.Errorf("unexpected LastError: %s", status.LastError)
	}
}

func TestWebhookConfig_ToStatus_NoIDNoManualSetup(t *testing.T) {
	wc := &WebhookConfig{
		URL:              "https://example.com/webhooks/org/gitlab/1",
		Events:           []string{"merge_request"},
		IsActive:         false,
		NeedsManualSetup: false,
	}

	status := wc.ToStatus()

	// Without ID and without NeedsManualSetup, Registered should be false
	if status.Registered {
		t.Error("expected Registered to be false without ID or NeedsManualSetup")
	}
}

func TestWebhookConfig_ToStatus_SecretNotExposed(t *testing.T) {
	wc := &WebhookConfig{
		ID:     "wh_123",
		URL:    "https://example.com/webhooks/org/gitlab/1",
		Secret: "super_secret_value",
	}

	status := wc.ToStatus()

	// Verify that WebhookStatus does not have a Secret field
	// This is a compile-time check via struct definition, but we can also
	// verify via JSON marshaling
	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal status: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if _, hasSecret := parsed["secret"]; hasSecret {
		t.Error("WebhookStatus should not expose secret field")
	}
}

// ===========================================
// WebhookConfig JSON Serialization Tests
// ===========================================

func TestWebhookConfig_JSONSerialization(t *testing.T) {
	wc := &WebhookConfig{
		ID:               "wh_abc123",
		URL:              "https://example.com/webhooks/test",
		Secret:           "secret_value",
		Events:           []string{"merge_request", "pipeline", "push"},
		IsActive:         true,
		NeedsManualSetup: false,
		LastError:        "",
		CreatedAt:        "2026-02-06T10:00:00Z",
	}

	data, err := json.Marshal(wc)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed WebhookConfig
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.ID != wc.ID {
		t.Errorf("ID mismatch: %s vs %s", parsed.ID, wc.ID)
	}
	if parsed.URL != wc.URL {
		t.Errorf("URL mismatch: %s vs %s", parsed.URL, wc.URL)
	}
	if parsed.Secret != wc.Secret {
		t.Errorf("Secret mismatch: %s vs %s", parsed.Secret, wc.Secret)
	}
	if len(parsed.Events) != len(wc.Events) {
		t.Errorf("Events length mismatch: %d vs %d", len(parsed.Events), len(wc.Events))
	}
	if parsed.IsActive != wc.IsActive {
		t.Error("IsActive mismatch")
	}
	if parsed.NeedsManualSetup != wc.NeedsManualSetup {
		t.Error("NeedsManualSetup mismatch")
	}
}

func TestWebhookConfig_JSONOmitEmpty(t *testing.T) {
	wc := &WebhookConfig{
		ID:       "wh_123",
		URL:      "https://example.com/webhooks/test",
		Events:   []string{"merge_request"},
		IsActive: true,
		// Secret, LastError, CreatedAt are empty
	}

	data, err := json.Marshal(wc)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	// With omitempty, these fields should not appear
	if _, has := parsed["secret"]; has {
		t.Error("expected secret to be omitted when empty")
	}
	if _, has := parsed["last_error"]; has {
		t.Error("expected last_error to be omitted when empty")
	}
	if _, has := parsed["created_at"]; has {
		t.Error("expected created_at to be omitted when empty")
	}
}

// ===========================================
// WebhookStatus JSON Serialization Tests
// ===========================================

func TestWebhookStatus_JSONSerialization(t *testing.T) {
	status := &WebhookStatus{
		Registered:       true,
		WebhookID:        "wh_test",
		WebhookURL:       "https://example.com/webhooks/test",
		Events:           []string{"merge_request", "pipeline"},
		IsActive:         true,
		NeedsManualSetup: false,
		LastError:        "",
		RegisteredAt:     "2026-02-06T10:00:00Z",
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed WebhookStatus
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.Registered != status.Registered {
		t.Error("Registered mismatch")
	}
	if parsed.WebhookID != status.WebhookID {
		t.Errorf("WebhookID mismatch: %s vs %s", parsed.WebhookID, status.WebhookID)
	}
	if parsed.IsActive != status.IsActive {
		t.Error("IsActive mismatch")
	}
}

func TestWebhookStatus_JSONOmitEmpty(t *testing.T) {
	status := &WebhookStatus{
		Registered: false,
		IsActive:   false,
		// All other fields are empty
	}

	data, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	// With omitempty, these fields should not appear
	if _, has := parsed["webhook_id"]; has {
		t.Error("expected webhook_id to be omitted when empty")
	}
	if _, has := parsed["webhook_url"]; has {
		t.Error("expected webhook_url to be omitted when empty")
	}
	if _, has := parsed["events"]; has {
		t.Error("expected events to be omitted when empty/nil")
	}
	if _, has := parsed["last_error"]; has {
		t.Error("expected last_error to be omitted when empty")
	}
	if _, has := parsed["registered_at"]; has {
		t.Error("expected registered_at to be omitted when empty")
	}
}

// ===========================================
// Repository Tests
// ===========================================

func TestRepository_TableName(t *testing.T) {
	repo := Repository{}
	if repo.TableName() != "repositories" {
		t.Errorf("unexpected table name: %s", repo.TableName())
	}
}

func TestRepository_Fields(t *testing.T) {
	now := time.Now()
	ticketPrefix := "PROJ"
	userID := int64(123)

	repo := &Repository{
		ID:               1,
		OrganizationID:   100,
		ProviderType:     ProviderTypeGitLab,
		ProviderBaseURL:  "https://gitlab.com",
		CloneURL:         "https://gitlab.com/org/repo.git",
		ExternalID:       "12345",
		Name:             "test-repo",
		FullPath:         "org/test-repo",
		DefaultBranch:    "main",
		TicketPrefix:     &ticketPrefix,
		Visibility:       "organization",
		ImportedByUserID: &userID,
		IsActive:         true,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if repo.ID != 1 {
		t.Errorf("unexpected ID: %d", repo.ID)
	}
	if repo.OrganizationID != 100 {
		t.Errorf("unexpected OrganizationID: %d", repo.OrganizationID)
	}
	if repo.ProviderType != "gitlab" {
		t.Errorf("unexpected ProviderType: %s", repo.ProviderType)
	}
	if repo.ProviderBaseURL != "https://gitlab.com" {
		t.Errorf("unexpected ProviderBaseURL: %s", repo.ProviderBaseURL)
	}
	if repo.TicketPrefix == nil || *repo.TicketPrefix != "PROJ" {
		t.Error("unexpected TicketPrefix")
	}
	if repo.ImportedByUserID == nil || *repo.ImportedByUserID != 123 {
		t.Error("unexpected ImportedByUserID")
	}
}

func TestRepository_WithWebhookConfig(t *testing.T) {
	repo := &Repository{
		ID:              1,
		OrganizationID:  100,
		ProviderType:    ProviderTypeGitHub,
		ProviderBaseURL: "https://github.com",
		Name:            "test-repo",
		FullPath:        "org/test-repo",
		WebhookConfig: &WebhookConfig{
			ID:       "wh_123",
			URL:      "https://example.com/webhooks/org/github/1",
			Secret:   "secret",
			Events:   []string{"pull_request", "push"},
			IsActive: true,
		},
	}

	if repo.WebhookConfig == nil {
		t.Fatal("expected WebhookConfig to be set")
	}
	if repo.WebhookConfig.ID != "wh_123" {
		t.Errorf("unexpected WebhookConfig.ID: %s", repo.WebhookConfig.ID)
	}
	if len(repo.WebhookConfig.Events) != 2 {
		t.Errorf("expected 2 events, got %d", len(repo.WebhookConfig.Events))
	}
}

func TestRepository_JSONSerialization(t *testing.T) {
	ticketPrefix := "TEST"
	repo := &Repository{
		ID:              1,
		OrganizationID:  100,
		ProviderType:    ProviderTypeGitLab,
		ProviderBaseURL: "https://gitlab.com",
		CloneURL:        "https://gitlab.com/org/repo.git",
		ExternalID:      "12345",
		Name:            "test-repo",
		FullPath:        "org/test-repo",
		DefaultBranch:   "main",
		TicketPrefix:    &ticketPrefix,
		Visibility:      "organization",
		IsActive:        true,
		WebhookConfig: &WebhookConfig{
			ID:       "wh_123",
			URL:      "https://example.com/webhooks",
			Events:   []string{"merge_request"},
			IsActive: true,
		},
	}

	data, err := json.Marshal(repo)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if parsed["id"].(float64) != 1 {
		t.Errorf("unexpected id: %v", parsed["id"])
	}
	if parsed["provider_type"] != "gitlab" {
		t.Errorf("unexpected provider_type: %v", parsed["provider_type"])
	}
	if parsed["name"] != "test-repo" {
		t.Errorf("unexpected name: %v", parsed["name"])
	}

	webhookConfig := parsed["webhook_config"].(map[string]interface{})
	if webhookConfig["id"] != "wh_123" {
		t.Errorf("unexpected webhook_config.id: %v", webhookConfig["id"])
	}
}

// ===========================================
// Provider Constants Tests
// ===========================================

func TestProviderConstants(t *testing.T) {
	if ProviderTypeGitHub != "github" {
		t.Errorf("unexpected ProviderTypeGitHub: %s", ProviderTypeGitHub)
	}
	if ProviderTypeGitLab != "gitlab" {
		t.Errorf("unexpected ProviderTypeGitLab: %s", ProviderTypeGitLab)
	}
	if ProviderTypeGitee != "gitee" {
		t.Errorf("unexpected ProviderTypeGitee: %s", ProviderTypeGitee)
	}
	if ProviderTypeSSH != "ssh" {
		t.Errorf("unexpected ProviderTypeSSH: %s", ProviderTypeSSH)
	}
}

// ===========================================
// Edge Cases
// ===========================================

func TestWebhookConfig_EmptyEvents(t *testing.T) {
	wc := &WebhookConfig{
		ID:     "wh_123",
		URL:    "https://example.com/webhooks",
		Events: []string{},
	}

	status := wc.ToStatus()

	if !status.Registered {
		t.Error("expected Registered to be true when ID is set")
	}
	if len(status.Events) != 0 {
		t.Errorf("expected 0 events, got %d", len(status.Events))
	}
}

func TestWebhookConfig_NilEvents(t *testing.T) {
	wc := &WebhookConfig{
		ID:     "wh_123",
		URL:    "https://example.com/webhooks",
		Events: nil,
	}

	status := wc.ToStatus()

	if !status.Registered {
		t.Error("expected Registered to be true when ID is set")
	}
	if status.Events != nil {
		t.Error("expected nil events")
	}
}

func TestRepository_NilWebhookConfig(t *testing.T) {
	repo := &Repository{
		ID:              1,
		OrganizationID:  100,
		ProviderType:    ProviderTypeGitHub,
		ProviderBaseURL: "https://github.com",
		Name:            "test-repo",
		FullPath:        "org/test-repo",
		WebhookConfig:   nil,
	}

	if repo.WebhookConfig != nil {
		t.Error("expected WebhookConfig to be nil")
	}

	// This should not panic
	var status *WebhookStatus
	if repo.WebhookConfig != nil {
		status = repo.WebhookConfig.ToStatus()
	} else {
		status = (&WebhookConfig{}).ToStatus() // Should give empty status
		// Actually nil WebhookConfig.ToStatus() should be handled
	}

	if status.Registered {
		t.Error("expected empty status to have Registered=false")
	}
}

func TestRepository_Visibility(t *testing.T) {
	tests := []struct {
		visibility string
		expected   string
	}{
		{"organization", "organization"},
		{"private", "private"},
		{"public", "public"},
	}

	for _, tt := range tests {
		repo := &Repository{Visibility: tt.visibility}
		if repo.Visibility != tt.expected {
			t.Errorf("expected visibility %s, got %s", tt.expected, repo.Visibility)
		}
	}
}

func TestRepository_PreparationFields(t *testing.T) {
	script := "npm install"
	timeout := 600

	repo := &Repository{
		ID:                 1,
		OrganizationID:     100,
		ProviderType:       ProviderTypeGitHub,
		ProviderBaseURL:    "https://github.com",
		Name:               "test-repo",
		FullPath:           "org/test-repo",
		PreparationScript:  &script,
		PreparationTimeout: &timeout,
	}

	if repo.PreparationScript == nil || *repo.PreparationScript != "npm install" {
		t.Error("unexpected PreparationScript")
	}
	if repo.PreparationTimeout == nil || *repo.PreparationTimeout != 600 {
		t.Error("unexpected PreparationTimeout")
	}
}

func TestRepository_SoftDelete(t *testing.T) {
	now := time.Now()
	repo := &Repository{
		ID:              1,
		OrganizationID:  100,
		ProviderType:    ProviderTypeGitHub,
		ProviderBaseURL: "https://github.com",
		Name:            "test-repo",
		FullPath:        "org/test-repo",
		DeletedAt:       &now,
	}

	if repo.DeletedAt == nil {
		t.Error("expected DeletedAt to be set")
	}
}
