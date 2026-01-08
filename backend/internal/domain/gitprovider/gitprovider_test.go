package gitprovider

import (
	"testing"
	"time"
)

// --- Test GitProvider ---

func TestGitProviderTableName(t *testing.T) {
	gp := GitProvider{}
	if gp.TableName() != "git_providers" {
		t.Errorf("expected 'git_providers', got %s", gp.TableName())
	}
}

func TestGitProviderStruct(t *testing.T) {
	now := time.Now()
	clientID := "client-123"

	gp := GitProvider{
		ID:             1,
		OrganizationID: 100,
		ProviderType:   "gitlab",
		Name:           "Company GitLab",
		BaseURL:        "https://gitlab.example.com",
		ClientID:       &clientID,
		IsDefault:      true,
		IsActive:       true,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if gp.ID != 1 {
		t.Errorf("expected ID 1, got %d", gp.ID)
	}
	if gp.OrganizationID != 100 {
		t.Errorf("expected OrganizationID 100, got %d", gp.OrganizationID)
	}
	if gp.ProviderType != "gitlab" {
		t.Errorf("expected ProviderType 'gitlab', got %s", gp.ProviderType)
	}
	if gp.Name != "Company GitLab" {
		t.Errorf("expected Name 'Company GitLab', got %s", gp.Name)
	}
	if gp.BaseURL != "https://gitlab.example.com" {
		t.Errorf("expected BaseURL 'https://gitlab.example.com', got %s", gp.BaseURL)
	}
	if *gp.ClientID != "client-123" {
		t.Errorf("expected ClientID 'client-123', got %s", *gp.ClientID)
	}
	if !gp.IsDefault {
		t.Error("expected IsDefault true")
	}
	if !gp.IsActive {
		t.Error("expected IsActive true")
	}
}

func TestGitProviderWithNilOptionalFields(t *testing.T) {
	gp := GitProvider{
		ID:             1,
		OrganizationID: 100,
		ProviderType:   "github",
		Name:           "GitHub",
		BaseURL:        "https://github.com",
		IsDefault:      false,
		IsActive:       true,
	}

	if gp.ClientID != nil {
		t.Error("expected ClientID to be nil")
	}
	if gp.ClientSecretEncrypted != nil {
		t.Error("expected ClientSecretEncrypted to be nil")
	}
	if gp.BotTokenEncrypted != nil {
		t.Error("expected BotTokenEncrypted to be nil")
	}
}

func TestGitProviderProviderTypes(t *testing.T) {
	types := []string{"gitlab", "github", "gitee"}

	for _, pt := range types {
		gp := GitProvider{ProviderType: pt}
		if gp.ProviderType != pt {
			t.Errorf("expected ProviderType '%s', got %s", pt, gp.ProviderType)
		}
	}
}

// --- Test Repository (in gitprovider package) ---

func TestRepositoryTableName(t *testing.T) {
	r := Repository{}
	if r.TableName() != "repositories" {
		t.Errorf("expected 'repositories', got %s", r.TableName())
	}
}

func TestRepositoryStruct(t *testing.T) {
	now := time.Now()
	teamID := int64(10)
	ticketPrefix := "PROJ"

	r := Repository{
		ID:             1,
		OrganizationID: 100,
		TeamID:         &teamID,
		GitProviderID:  5,
		ExternalID:     "ext-123",
		Name:           "my-project",
		FullPath:       "org/my-project",
		DefaultBranch:  "develop",
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
	if r.GitProviderID != 5 {
		t.Errorf("expected GitProviderID 5, got %d", r.GitProviderID)
	}
	if r.Name != "my-project" {
		t.Errorf("expected Name 'my-project', got %s", r.Name)
	}
	if r.DefaultBranch != "develop" {
		t.Errorf("expected DefaultBranch 'develop', got %s", r.DefaultBranch)
	}
	if *r.TicketPrefix != "PROJ" {
		t.Errorf("expected TicketPrefix 'PROJ', got %s", *r.TicketPrefix)
	}
}

func TestRepositoryWithNilOptionalFields(t *testing.T) {
	r := Repository{
		ID:             1,
		OrganizationID: 100,
		GitProviderID:  5,
		ExternalID:     "ext-456",
		Name:           "repo",
		FullPath:       "org/repo",
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

// --- Benchmark Tests ---

func BenchmarkGitProviderTableName(b *testing.B) {
	gp := GitProvider{}
	for i := 0; i < b.N; i++ {
		gp.TableName()
	}
}

func BenchmarkRepositoryTableName(b *testing.B) {
	r := Repository{}
	for i := 0; i < b.N; i++ {
		r.TableName()
	}
}
