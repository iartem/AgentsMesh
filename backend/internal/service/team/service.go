package team

import (
	"context"
	"errors"

	"github.com/anthropics/agentmesh/backend/internal/domain/organization"
	"github.com/anthropics/agentmesh/backend/internal/domain/user"
	"gorm.io/gorm"
)

var (
	ErrTeamNotFound     = errors.New("team not found")
	ErrTeamNameExists   = errors.New("team name already exists in organization")
	ErrNotTeamMember    = errors.New("not a team member")
	ErrCannotRemoveLead = errors.New("cannot remove team lead")
)

// Service handles team operations
type Service struct {
	db *gorm.DB
}

// NewService creates a new team service
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// CreateRequest represents team creation request
type CreateRequest struct {
	OrganizationID int64
	Name           string
	Description    string
}

// Create creates a new team
func (s *Service) Create(ctx context.Context, creatorID int64, req *CreateRequest) (*organization.Team, error) {
	// Check if team name already exists in organization
	var existing organization.Team
	if err := s.db.WithContext(ctx).Where("organization_id = ? AND name = ?", req.OrganizationID, req.Name).First(&existing).Error; err == nil {
		return nil, ErrTeamNameExists
	}

	team := &organization.Team{
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		Description:    req.Description,
	}

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(team).Error; err != nil {
			return err
		}

		// Add creator as team lead
		member := &organization.TeamMember{
			TeamID: team.ID,
			UserID: creatorID,
			Role:   organization.TeamRoleLead,
		}
		return tx.Create(member).Error
	})

	if err != nil {
		return nil, err
	}

	return team, nil
}

// GetByID returns a team by ID
func (s *Service) GetByID(ctx context.Context, id int64) (*organization.Team, error) {
	var team organization.Team
	if err := s.db.WithContext(ctx).First(&team, id).Error; err != nil {
		return nil, ErrTeamNotFound
	}
	return &team, nil
}

// Update updates a team
func (s *Service) Update(ctx context.Context, id int64, updates map[string]interface{}) (*organization.Team, error) {
	if err := s.db.WithContext(ctx).Model(&organization.Team{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// Delete deletes a team
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&organization.Team{}, id).Error
}

// ListByOrganization returns teams for an organization
func (s *Service) ListByOrganization(ctx context.Context, orgID int64) ([]*organization.Team, error) {
	var teams []*organization.Team
	err := s.db.WithContext(ctx).Where("organization_id = ?", orgID).Find(&teams).Error
	return teams, err
}

// ListByUser returns teams a user belongs to
func (s *Service) ListByUser(ctx context.Context, orgID, userID int64) ([]*organization.Team, error) {
	var teams []*organization.Team
	err := s.db.WithContext(ctx).
		Joins("JOIN team_members ON team_members.team_id = teams.id").
		Where("teams.organization_id = ? AND team_members.user_id = ?", orgID, userID).
		Find(&teams).Error
	return teams, err
}

// AddMember adds a member to a team
func (s *Service) AddMember(ctx context.Context, teamID, userID int64, role string) error {
	if role == "" {
		role = organization.TeamRoleMember
	}
	member := &organization.TeamMember{
		TeamID: teamID,
		UserID: userID,
		Role:   role,
	}
	return s.db.WithContext(ctx).Create(member).Error
}

// RemoveMember removes a member from a team
func (s *Service) RemoveMember(ctx context.Context, teamID, userID int64) error {
	// Check if user is team lead
	var member organization.TeamMember
	if err := s.db.WithContext(ctx).Where("team_id = ? AND user_id = ?", teamID, userID).First(&member).Error; err == nil {
		if member.Role == organization.TeamRoleLead {
			// Check if there are other leads
			var leadCount int64
			s.db.WithContext(ctx).Model(&organization.TeamMember{}).Where("team_id = ? AND role = ?", teamID, organization.TeamRoleLead).Count(&leadCount)
			if leadCount <= 1 {
				return ErrCannotRemoveLead
			}
		}
	}
	return s.db.WithContext(ctx).Where("team_id = ? AND user_id = ?", teamID, userID).Delete(&organization.TeamMember{}).Error
}

// UpdateMemberRole updates a member's role in a team
func (s *Service) UpdateMemberRole(ctx context.Context, teamID, userID int64, role string) error {
	return s.db.WithContext(ctx).Model(&organization.TeamMember{}).
		Where("team_id = ? AND user_id = ?", teamID, userID).
		Update("role", role).Error
}

// GetMember returns a team member
func (s *Service) GetMember(ctx context.Context, teamID, userID int64) (*organization.TeamMember, error) {
	var member organization.TeamMember
	if err := s.db.WithContext(ctx).Where("team_id = ? AND user_id = ?", teamID, userID).First(&member).Error; err != nil {
		return nil, ErrNotTeamMember
	}
	return &member, nil
}

// ListMembers returns members of a team
func (s *Service) ListMembers(ctx context.Context, teamID int64) ([]*user.User, error) {
	var users []*user.User
	err := s.db.WithContext(ctx).
		Joins("JOIN team_members ON team_members.user_id = users.id").
		Where("team_members.team_id = ?", teamID).
		Find(&users).Error
	return users, err
}

// IsMember checks if a user is a member of the team
func (s *Service) IsMember(ctx context.Context, teamID, userID int64) (bool, error) {
	var count int64
	s.db.WithContext(ctx).Model(&organization.TeamMember{}).Where("team_id = ? AND user_id = ?", teamID, userID).Count(&count)
	return count > 0, nil
}

// IsLead checks if a user is a lead of the team
func (s *Service) IsLead(ctx context.Context, teamID, userID int64) (bool, error) {
	var member organization.TeamMember
	if err := s.db.WithContext(ctx).Where("team_id = ? AND user_id = ?", teamID, userID).First(&member).Error; err != nil {
		return false, nil
	}
	return member.Role == organization.TeamRoleLead, nil
}

// GetUserTeamIDs returns team IDs a user belongs to in an organization
func (s *Service) GetUserTeamIDs(ctx context.Context, orgID, userID int64) ([]int64, error) {
	var teamIDs []int64
	err := s.db.WithContext(ctx).
		Model(&organization.TeamMember{}).
		Joins("JOIN teams ON teams.id = team_members.team_id").
		Where("teams.organization_id = ? AND team_members.user_id = ?", orgID, userID).
		Pluck("team_members.team_id", &teamIDs).Error
	return teamIDs, err
}
