package ticket

import (
	"context"
	"testing"
)

// TestCreateTicket_IdentifierScopedToOrg verifies that identifier generation
// is scoped to organization_id, allowing different orgs to have the same identifier.
// This was the root cause of a production 500 bug: the old code had a global unique
// constraint on identifier but generated numbers per-org, causing conflicts.
func TestCreateTicket_IdentifierScopedToOrg(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create ticket in org 1
	tkt1, err := service.CreateTicket(ctx, &CreateTicketRequest{
		OrganizationID: 1,
		ReporterID:     1,
		Type:           "task",
		Title:          "Org1 Ticket",
		Priority:       "medium",
	})
	if err != nil {
		t.Fatalf("failed to create ticket in org 1: %v", err)
	}
	if tkt1.Identifier != "TICKET-1" {
		t.Errorf("org 1 first ticket: expected Identifier 'TICKET-1', got %s", tkt1.Identifier)
	}

	// Create ticket in org 2 — should also get TICKET-1, not conflict
	tkt2, err := service.CreateTicket(ctx, &CreateTicketRequest{
		OrganizationID: 2,
		ReporterID:     2,
		Type:           "task",
		Title:          "Org2 Ticket",
		Priority:       "medium",
	})
	if err != nil {
		t.Fatalf("failed to create ticket in org 2: %v", err)
	}
	if tkt2.Identifier != "TICKET-1" {
		t.Errorf("org 2 first ticket: expected Identifier 'TICKET-1', got %s", tkt2.Identifier)
	}

	// Create second ticket in org 1 — should get TICKET-2
	tkt3, err := service.CreateTicket(ctx, &CreateTicketRequest{
		OrganizationID: 1,
		ReporterID:     1,
		Type:           "task",
		Title:          "Org1 Second Ticket",
		Priority:       "medium",
	})
	if err != nil {
		t.Fatalf("failed to create second ticket in org 1: %v", err)
	}
	if tkt3.Identifier != "TICKET-2" {
		t.Errorf("org 1 second ticket: expected Identifier 'TICKET-2', got %s", tkt3.Identifier)
	}
	if tkt3.Number != 2 {
		t.Errorf("org 1 second ticket: expected Number 2, got %d", tkt3.Number)
	}
}

// TestCreateTicket_PrefixScopedToOrg verifies that repository-prefix identifier
// generation is also scoped to organization, not globally.
func TestCreateTicket_PrefixScopedToOrg(t *testing.T) {
	db := setupTestDB(t)
	// Create repositories table with same prefix for different repos
	db.Exec(`CREATE TABLE IF NOT EXISTS repositories (id INTEGER PRIMARY KEY, ticket_prefix TEXT)`)
	db.Exec(`INSERT INTO repositories (id, ticket_prefix) VALUES (1, 'PROJ')`)
	db.Exec(`INSERT INTO repositories (id, ticket_prefix) VALUES (2, 'PROJ')`)

	service := NewService(db)
	ctx := context.Background()

	repoID1 := int64(1)
	repoID2 := int64(2)

	// Org 1, repo 1, prefix PROJ
	tkt1, err := service.CreateTicket(ctx, &CreateTicketRequest{
		OrganizationID: 1,
		ReporterID:     1,
		Type:           "task",
		Title:          "Org1 Repo1",
		Priority:       "medium",
		RepositoryID:   &repoID1,
	})
	if err != nil {
		t.Fatalf("failed to create ticket for org1/repo1: %v", err)
	}
	if tkt1.Identifier != "PROJ-1" {
		t.Errorf("expected 'PROJ-1', got %s", tkt1.Identifier)
	}

	// Org 2, repo 2, same prefix PROJ — should also get PROJ-1
	tkt2, err := service.CreateTicket(ctx, &CreateTicketRequest{
		OrganizationID: 2,
		ReporterID:     2,
		Type:           "task",
		Title:          "Org2 Repo2",
		Priority:       "medium",
		RepositoryID:   &repoID2,
	})
	if err != nil {
		t.Fatalf("failed to create ticket for org2/repo2: %v", err)
	}
	if tkt2.Identifier != "PROJ-1" {
		t.Errorf("expected 'PROJ-1', got %s", tkt2.Identifier)
	}

	// Org 1, repo 1 again — should get PROJ-2
	tkt3, err := service.CreateTicket(ctx, &CreateTicketRequest{
		OrganizationID: 1,
		ReporterID:     1,
		Type:           "task",
		Title:          "Org1 Repo1 Second",
		Priority:       "medium",
		RepositoryID:   &repoID1,
	})
	if err != nil {
		t.Fatalf("failed to create second ticket for org1/repo1: %v", err)
	}
	if tkt3.Identifier != "PROJ-2" {
		t.Errorf("expected 'PROJ-2', got %s", tkt3.Identifier)
	}
}

// TestCreateTicket_MixedPrefixesInSameOrg verifies that different prefixes
// within the same organization maintain independent numbering.
func TestCreateTicket_MixedPrefixesInSameOrg(t *testing.T) {
	db := setupTestDB(t)
	db.Exec(`CREATE TABLE IF NOT EXISTS repositories (id INTEGER PRIMARY KEY, ticket_prefix TEXT)`)
	db.Exec(`INSERT INTO repositories (id, ticket_prefix) VALUES (1, 'PROJ')`)
	db.Exec(`INSERT INTO repositories (id, ticket_prefix) VALUES (2, 'BUG')`)

	service := NewService(db)
	ctx := context.Background()

	repoID1 := int64(1)
	repoID2 := int64(2)

	// Create PROJ-1
	tkt1, err := service.CreateTicket(ctx, &CreateTicketRequest{
		OrganizationID: 1,
		ReporterID:     1,
		Type:           "task",
		Title:          "Project Task",
		Priority:       "medium",
		RepositoryID:   &repoID1,
	})
	if err != nil {
		t.Fatalf("failed to create PROJ ticket: %v", err)
	}
	if tkt1.Identifier != "PROJ-1" {
		t.Errorf("expected 'PROJ-1', got %s", tkt1.Identifier)
	}

	// Create BUG-1 (different prefix, same org, independent numbering)
	tkt2, err := service.CreateTicket(ctx, &CreateTicketRequest{
		OrganizationID: 1,
		ReporterID:     1,
		Type:           "bug",
		Title:          "Bug Report",
		Priority:       "high",
		RepositoryID:   &repoID2,
	})
	if err != nil {
		t.Fatalf("failed to create BUG ticket: %v", err)
	}
	if tkt2.Identifier != "BUG-1" {
		t.Errorf("expected 'BUG-1', got %s", tkt2.Identifier)
	}

	// Create TICKET-1 (no repo, default prefix, independent numbering)
	tkt3, err := service.CreateTicket(ctx, &CreateTicketRequest{
		OrganizationID: 1,
		ReporterID:     1,
		Type:           "task",
		Title:          "No Repo Task",
		Priority:       "medium",
	})
	if err != nil {
		t.Fatalf("failed to create TICKET ticket: %v", err)
	}
	if tkt3.Identifier != "TICKET-1" {
		t.Errorf("expected 'TICKET-1', got %s", tkt3.Identifier)
	}

	// Create PROJ-2
	tkt4, err := service.CreateTicket(ctx, &CreateTicketRequest{
		OrganizationID: 1,
		ReporterID:     1,
		Type:           "task",
		Title:          "Project Task 2",
		Priority:       "medium",
		RepositoryID:   &repoID1,
	})
	if err != nil {
		t.Fatalf("failed to create second PROJ ticket: %v", err)
	}
	if tkt4.Identifier != "PROJ-2" {
		t.Errorf("expected 'PROJ-2', got %s", tkt4.Identifier)
	}
}
