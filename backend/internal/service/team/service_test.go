package team

import (
	"context"
	"testing"

	"github.com/anthropics/agentmesh/backend/internal/domain/organization"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	// Create tables manually for SQLite compatibility
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS teams (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatalf("failed to create teams table: %v", err)
	}

	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS team_members (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			team_id INTEGER NOT NULL,
			user_id INTEGER NOT NULL,
			role TEXT NOT NULL DEFAULT 'member'
		)
	`).Error
	if err != nil {
		t.Fatalf("failed to create team_members table: %v", err)
	}

	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			username TEXT NOT NULL UNIQUE,
			name TEXT,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	return db
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)

	if service == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestCreateTeam(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		OrganizationID: 1,
		Name:           "Engineering",
		Description:    "Engineering team",
	}

	team, err := service.Create(ctx, 1, req)
	if err != nil {
		t.Fatalf("failed to create team: %v", err)
	}

	if team == nil {
		t.Fatal("expected non-nil team")
	}
	if team.Name != "Engineering" {
		t.Errorf("expected Name 'Engineering', got %s", team.Name)
	}
	if team.Description != "Engineering team" {
		t.Errorf("expected Description 'Engineering team', got %s", team.Description)
	}
	if team.OrganizationID != 1 {
		t.Errorf("expected OrganizationID 1, got %d", team.OrganizationID)
	}
}

func TestCreateTeamDuplicateName(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{OrganizationID: 1, Name: "Engineering"}
	service.Create(ctx, 1, req)

	// Try to create team with same name in same org
	req2 := &CreateRequest{OrganizationID: 1, Name: "Engineering"}
	_, err := service.Create(ctx, 2, req2)
	if err != ErrTeamNameExists {
		t.Errorf("expected ErrTeamNameExists, got %v", err)
	}
}

func TestCreateTeamSameNameDifferentOrg(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{OrganizationID: 1, Name: "Engineering"}
	service.Create(ctx, 1, req)

	// Can create team with same name in different org
	req2 := &CreateRequest{OrganizationID: 2, Name: "Engineering"}
	team, err := service.Create(ctx, 2, req2)
	if err != nil {
		t.Fatalf("should be able to create team with same name in different org: %v", err)
	}
	if team.Name != "Engineering" {
		t.Errorf("expected Name 'Engineering', got %s", team.Name)
	}
}

func TestGetByID(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{OrganizationID: 1, Name: "Engineering"}
	created, _ := service.Create(ctx, 1, req)

	team, err := service.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to get team: %v", err)
	}
	if team.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, team.ID)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	_, err := service.GetByID(ctx, 99999)
	if err != ErrTeamNotFound {
		t.Errorf("expected ErrTeamNotFound, got %v", err)
	}
}

func TestUpdateTeam(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{OrganizationID: 1, Name: "Engineering"}
	created, _ := service.Create(ctx, 1, req)

	updates := map[string]interface{}{
		"name":        "Backend",
		"description": "Backend team",
	}
	updated, err := service.Update(ctx, created.ID, updates)
	if err != nil {
		t.Fatalf("failed to update team: %v", err)
	}
	if updated.Name != "Backend" {
		t.Errorf("expected Name 'Backend', got %s", updated.Name)
	}
}

func TestDeleteTeam(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{OrganizationID: 1, Name: "Engineering"}
	created, _ := service.Create(ctx, 1, req)

	err := service.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to delete team: %v", err)
	}

	_, err = service.GetByID(ctx, created.ID)
	if err != ErrTeamNotFound {
		t.Errorf("expected ErrTeamNotFound, got %v", err)
	}
}

func TestListByOrganization(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	service.Create(ctx, 1, &CreateRequest{OrganizationID: 1, Name: "Team1"})
	service.Create(ctx, 1, &CreateRequest{OrganizationID: 1, Name: "Team2"})
	service.Create(ctx, 1, &CreateRequest{OrganizationID: 2, Name: "Team3"})

	teams, err := service.ListByOrganization(ctx, 1)
	if err != nil {
		t.Fatalf("failed to list teams: %v", err)
	}
	if len(teams) != 2 {
		t.Errorf("expected 2 teams, got %d", len(teams))
	}
}

func TestAddMember(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{OrganizationID: 1, Name: "Engineering"}
	team, _ := service.Create(ctx, 1, req)

	// Add a new member
	err := service.AddMember(ctx, team.ID, 2, organization.TeamRoleMember)
	if err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	member, err := service.GetMember(ctx, team.ID, 2)
	if err != nil {
		t.Fatalf("failed to get member: %v", err)
	}
	if member.Role != "member" {
		t.Errorf("expected Role 'member', got %s", member.Role)
	}
}

func TestAddMemberDefaultRole(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{OrganizationID: 1, Name: "Engineering"}
	team, _ := service.Create(ctx, 1, req)

	// Add a member with empty role
	err := service.AddMember(ctx, team.ID, 2, "")
	if err != nil {
		t.Fatalf("failed to add member: %v", err)
	}

	member, _ := service.GetMember(ctx, team.ID, 2)
	if member.Role != "member" {
		t.Errorf("expected default Role 'member', got %s", member.Role)
	}
}

func TestRemoveMember(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{OrganizationID: 1, Name: "Engineering"}
	team, _ := service.Create(ctx, 1, req)
	service.AddMember(ctx, team.ID, 2, organization.TeamRoleMember)

	err := service.RemoveMember(ctx, team.ID, 2)
	if err != nil {
		t.Fatalf("failed to remove member: %v", err)
	}

	_, err = service.GetMember(ctx, team.ID, 2)
	if err != ErrNotTeamMember {
		t.Errorf("expected ErrNotTeamMember, got %v", err)
	}
}

func TestRemoveMemberLastLead(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{OrganizationID: 1, Name: "Engineering"}
	team, _ := service.Create(ctx, 1, req) // Creator is lead

	// Try to remove the only lead
	err := service.RemoveMember(ctx, team.ID, 1)
	if err != ErrCannotRemoveLead {
		t.Errorf("expected ErrCannotRemoveLead, got %v", err)
	}
}

func TestRemoveMemberWithMultipleLeads(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{OrganizationID: 1, Name: "Engineering"}
	team, _ := service.Create(ctx, 1, req)
	service.AddMember(ctx, team.ID, 2, organization.TeamRoleLead)

	// Can remove a lead if there are multiple
	err := service.RemoveMember(ctx, team.ID, 1)
	if err != nil {
		t.Fatalf("should be able to remove lead when multiple leads exist: %v", err)
	}
}

func TestUpdateMemberRole(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{OrganizationID: 1, Name: "Engineering"}
	team, _ := service.Create(ctx, 1, req)
	service.AddMember(ctx, team.ID, 2, organization.TeamRoleMember)

	err := service.UpdateMemberRole(ctx, team.ID, 2, organization.TeamRoleLead)
	if err != nil {
		t.Fatalf("failed to update member role: %v", err)
	}

	member, _ := service.GetMember(ctx, team.ID, 2)
	if member.Role != "lead" {
		t.Errorf("expected Role 'lead', got %s", member.Role)
	}
}

func TestGetMemberNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{OrganizationID: 1, Name: "Engineering"}
	team, _ := service.Create(ctx, 1, req)

	_, err := service.GetMember(ctx, team.ID, 999)
	if err != ErrNotTeamMember {
		t.Errorf("expected ErrNotTeamMember, got %v", err)
	}
}

func TestIsMember(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{OrganizationID: 1, Name: "Engineering"}
	team, _ := service.Create(ctx, 1, req)

	// Creator is member
	isMember, _ := service.IsMember(ctx, team.ID, 1)
	if !isMember {
		t.Error("expected creator to be member")
	}

	// Non-member is not member
	isMember, _ = service.IsMember(ctx, team.ID, 999)
	if isMember {
		t.Error("expected non-member not to be member")
	}
}

func TestIsLead(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{OrganizationID: 1, Name: "Engineering"}
	team, _ := service.Create(ctx, 1, req)
	service.AddMember(ctx, team.ID, 2, organization.TeamRoleMember)

	// Creator is lead
	isLead, _ := service.IsLead(ctx, team.ID, 1)
	if !isLead {
		t.Error("expected creator to be lead")
	}

	// Member is not lead
	isLead, _ = service.IsLead(ctx, team.ID, 2)
	if isLead {
		t.Error("expected member not to be lead")
	}

	// Non-member is not lead
	isLead, _ = service.IsLead(ctx, team.ID, 999)
	if isLead {
		t.Error("expected non-member not to be lead")
	}
}

func TestGetUserTeamIDs(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create teams in org 1 (user 1 is creator/lead)
	service.Create(ctx, 1, &CreateRequest{OrganizationID: 1, Name: "Team1"})
	service.Create(ctx, 1, &CreateRequest{OrganizationID: 1, Name: "Team2"})
	// Create team in org 2 (user 1 is creator/lead but in different org)
	service.Create(ctx, 1, &CreateRequest{OrganizationID: 2, Name: "Team3"})

	// Get team IDs for user 1 in org 1 - should be 2 teams
	teamIDs, err := service.GetUserTeamIDs(ctx, 1, 1)
	if err != nil {
		t.Fatalf("failed to get user team IDs: %v", err)
	}
	if len(teamIDs) != 2 {
		t.Errorf("expected 2 team IDs in org 1, got %d", len(teamIDs))
	}

	// Get team IDs for user 1 in org 2 - should be 1 team
	teamIDs, err = service.GetUserTeamIDs(ctx, 2, 1)
	if err != nil {
		t.Fatalf("failed to get user team IDs: %v", err)
	}
	if len(teamIDs) != 1 {
		t.Errorf("expected 1 team ID in org 2, got %d", len(teamIDs))
	}
}

func TestErrorVariables(t *testing.T) {
	if ErrTeamNotFound.Error() != "team not found" {
		t.Errorf("unexpected error message: %s", ErrTeamNotFound.Error())
	}
	if ErrTeamNameExists.Error() != "team name already exists in organization" {
		t.Errorf("unexpected error message: %s", ErrTeamNameExists.Error())
	}
	if ErrNotTeamMember.Error() != "not a team member" {
		t.Errorf("unexpected error message: %s", ErrNotTeamMember.Error())
	}
	if ErrCannotRemoveLead.Error() != "cannot remove team lead" {
		t.Errorf("unexpected error message: %s", ErrCannotRemoveLead.Error())
	}
}
