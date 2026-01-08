package team

import (
	"testing"
	"time"
)

// --- Test Team ---

func TestTeamTableName(t *testing.T) {
	team := Team{}
	if team.TableName() != "teams" {
		t.Errorf("expected 'teams', got %s", team.TableName())
	}
}

func TestTeamStruct(t *testing.T) {
	now := time.Now()
	desc := "Engineering team description"

	team := Team{
		ID:             1,
		OrganizationID: 100,
		Name:           "Engineering",
		Description:    &desc,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if team.ID != 1 {
		t.Errorf("expected ID 1, got %d", team.ID)
	}
	if team.OrganizationID != 100 {
		t.Errorf("expected OrganizationID 100, got %d", team.OrganizationID)
	}
	if team.Name != "Engineering" {
		t.Errorf("expected Name 'Engineering', got %s", team.Name)
	}
	if *team.Description != "Engineering team description" {
		t.Errorf("expected Description 'Engineering team description', got %s", *team.Description)
	}
}

func TestTeamWithNilDescription(t *testing.T) {
	team := Team{
		ID:             1,
		OrganizationID: 100,
		Name:           "Design",
	}

	if team.Description != nil {
		t.Error("expected Description to be nil")
	}
}

func TestTeamWithMembers(t *testing.T) {
	team := Team{
		ID:             1,
		OrganizationID: 100,
		Name:           "Backend",
		Members: []TeamMember{
			{ID: 1, TeamID: 1, UserID: 10, Role: "lead"},
			{ID: 2, TeamID: 1, UserID: 20, Role: "member"},
		},
	}

	if len(team.Members) != 2 {
		t.Errorf("expected 2 members, got %d", len(team.Members))
	}
}

// --- Test TeamMember ---

func TestTeamMemberTableName(t *testing.T) {
	tm := TeamMember{}
	if tm.TableName() != "team_members" {
		t.Errorf("expected 'team_members', got %s", tm.TableName())
	}
}

func TestTeamMemberStruct(t *testing.T) {
	tm := TeamMember{
		ID:     1,
		TeamID: 10,
		UserID: 50,
		Role:   "lead",
	}

	if tm.ID != 1 {
		t.Errorf("expected ID 1, got %d", tm.ID)
	}
	if tm.TeamID != 10 {
		t.Errorf("expected TeamID 10, got %d", tm.TeamID)
	}
	if tm.UserID != 50 {
		t.Errorf("expected UserID 50, got %d", tm.UserID)
	}
	if tm.Role != "lead" {
		t.Errorf("expected Role 'lead', got %s", tm.Role)
	}
}

func TestTeamMemberRoles(t *testing.T) {
	roles := []string{"lead", "member"}

	for _, role := range roles {
		tm := TeamMember{Role: role}
		if tm.Role != role {
			t.Errorf("expected Role '%s', got %s", role, tm.Role)
		}
	}
}

func TestTeamMemberWithTeamAssociation(t *testing.T) {
	team := &Team{
		ID:             1,
		OrganizationID: 100,
		Name:           "Frontend",
	}

	tm := TeamMember{
		ID:     1,
		TeamID: 1,
		UserID: 50,
		Role:   "member",
		Team:   team,
	}

	if tm.Team == nil {
		t.Error("expected Team association to be set")
	}
	if tm.Team.Name != "Frontend" {
		t.Errorf("expected Team.Name 'Frontend', got %s", tm.Team.Name)
	}
}

// --- Test TeamWithMembers ---

func TestTeamWithMembersStruct(t *testing.T) {
	now := time.Now()

	twm := TeamWithMembers{
		Team: Team{
			ID:             1,
			OrganizationID: 100,
			Name:           "DevOps",
			CreatedAt:      now,
			UpdatedAt:      now,
		},
		MemberCount: 5,
	}

	if twm.ID != 1 {
		t.Errorf("expected ID 1, got %d", twm.ID)
	}
	if twm.Name != "DevOps" {
		t.Errorf("expected Name 'DevOps', got %s", twm.Name)
	}
	if twm.MemberCount != 5 {
		t.Errorf("expected MemberCount 5, got %d", twm.MemberCount)
	}
}

func TestTeamWithMembersZeroCount(t *testing.T) {
	twm := TeamWithMembers{
		Team: Team{
			ID:             1,
			OrganizationID: 100,
			Name:           "NewTeam",
		},
		MemberCount: 0,
	}

	if twm.MemberCount != 0 {
		t.Errorf("expected MemberCount 0, got %d", twm.MemberCount)
	}
}

// --- Benchmark Tests ---

func BenchmarkTeamTableName(b *testing.B) {
	team := Team{}
	for i := 0; i < b.N; i++ {
		team.TableName()
	}
}

func BenchmarkTeamMemberTableName(b *testing.B) {
	tm := TeamMember{}
	for i := 0; i < b.N; i++ {
		tm.TableName()
	}
}

func BenchmarkTeamCreation(b *testing.B) {
	now := time.Now()
	desc := "Test description"
	for i := 0; i < b.N; i++ {
		_ = Team{
			ID:             int64(i),
			OrganizationID: 100,
			Name:           "Test Team",
			Description:    &desc,
			CreatedAt:      now,
			UpdatedAt:      now,
		}
	}
}
