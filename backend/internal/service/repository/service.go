package repository

import (
	"context"
	"errors"

	"github.com/anthropics/agentmesh/backend/internal/domain/gitprovider"
	gitproviderService "github.com/anthropics/agentmesh/backend/internal/service/gitprovider"
	"gorm.io/gorm"
)

var (
	ErrRepositoryNotFound = errors.New("repository not found")
	ErrRepositoryExists   = errors.New("repository already exists")
)

// Service handles repository operations
type Service struct {
	db                 *gorm.DB
	gitProviderService *gitproviderService.Service
}

// NewService creates a new repository service
func NewService(db *gorm.DB, gitProviderSvc *gitproviderService.Service) *Service {
	return &Service{
		db:                 db,
		gitProviderService: gitProviderSvc,
	}
}

// CreateRequest represents repository creation request
type CreateRequest struct {
	OrganizationID int64
	TeamID         *int64
	GitProviderID  int64
	ExternalID     string
	Name           string
	FullPath       string
	DefaultBranch  string
	TicketPrefix   *string
}

// Create creates a new repository configuration
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*gitprovider.Repository, error) {
	// Check if repository already exists
	var existing gitprovider.Repository
	if err := s.db.WithContext(ctx).Where("git_provider_id = ? AND external_id = ?", req.GitProviderID, req.ExternalID).First(&existing).Error; err == nil {
		return nil, ErrRepositoryExists
	}

	repo := &gitprovider.Repository{
		OrganizationID: req.OrganizationID,
		TeamID:         req.TeamID,
		GitProviderID:  req.GitProviderID,
		ExternalID:     req.ExternalID,
		Name:           req.Name,
		FullPath:       req.FullPath,
		DefaultBranch:  req.DefaultBranch,
		TicketPrefix:   req.TicketPrefix,
		IsActive:       true,
	}

	if repo.DefaultBranch == "" {
		repo.DefaultBranch = "main"
	}

	if err := s.db.WithContext(ctx).Create(repo).Error; err != nil {
		return nil, err
	}

	return repo, nil
}

// GetByID returns a repository by ID
func (s *Service) GetByID(ctx context.Context, id int64) (*gitprovider.Repository, error) {
	var repo gitprovider.Repository
	if err := s.db.WithContext(ctx).Preload("GitProvider").First(&repo, id).Error; err != nil {
		return nil, ErrRepositoryNotFound
	}
	return &repo, nil
}

// Update updates a repository
func (s *Service) Update(ctx context.Context, id int64, updates map[string]interface{}) (*gitprovider.Repository, error) {
	if err := s.db.WithContext(ctx).Model(&gitprovider.Repository{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// Delete deletes a repository
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&gitprovider.Repository{}, id).Error
}

// ListByOrganization returns repositories for an organization
func (s *Service) ListByOrganization(ctx context.Context, orgID int64, teamID *int64) ([]*gitprovider.Repository, error) {
	query := s.db.WithContext(ctx).Preload("GitProvider").Where("organization_id = ? AND is_active = ?", orgID, true)

	if teamID != nil {
		query = query.Where("team_id = ?", *teamID)
	}

	var repos []*gitprovider.Repository
	err := query.Find(&repos).Error
	return repos, err
}

// ListByTeam returns repositories for a team
func (s *Service) ListByTeam(ctx context.Context, teamID int64) ([]*gitprovider.Repository, error) {
	var repos []*gitprovider.Repository
	err := s.db.WithContext(ctx).
		Preload("GitProvider").
		Where("team_id = ? AND is_active = ?", teamID, true).
		Find(&repos).Error
	return repos, err
}

// AssignToTeam assigns a repository to a team
func (s *Service) AssignToTeam(ctx context.Context, repoID int64, teamID *int64) error {
	return s.db.WithContext(ctx).Model(&gitprovider.Repository{}).
		Where("id = ?", repoID).
		Update("team_id", teamID).Error
}

// GetByExternalID returns a repository by git provider and external ID
func (s *Service) GetByExternalID(ctx context.Context, providerID int64, externalID string) (*gitprovider.Repository, error) {
	var repo gitprovider.Repository
	if err := s.db.WithContext(ctx).
		Preload("GitProvider").
		Where("git_provider_id = ? AND external_id = ?", providerID, externalID).
		First(&repo).Error; err != nil {
		return nil, ErrRepositoryNotFound
	}
	return &repo, nil
}

// SyncFromProvider syncs repository info from git provider
func (s *Service) SyncFromProvider(ctx context.Context, repoID int64, accessToken string) (*gitprovider.Repository, error) {
	repo, err := s.GetByID(ctx, repoID)
	if err != nil {
		return nil, err
	}

	client, err := s.gitProviderService.GetClient(ctx, repo.GitProviderID, accessToken)
	if err != nil {
		return nil, err
	}

	project, err := client.GetProject(ctx, repo.ExternalID)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{
		"name":           project.Name,
		"full_path":      project.FullPath,
		"default_branch": project.DefaultBranch,
	}

	return s.Update(ctx, repoID, updates)
}

// GetCloneURL returns the clone URL for a repository
func (s *Service) GetCloneURL(ctx context.Context, repoID int64) (string, error) {
	repo, err := s.GetByID(ctx, repoID)
	if err != nil {
		return "", err
	}

	// Build clone URL based on provider type
	baseURL := repo.GitProvider.BaseURL
	fullPath := repo.FullPath

	switch repo.GitProvider.ProviderType {
	case "github":
		return "https://github.com/" + fullPath + ".git", nil
	case "gitlab":
		return baseURL + "/" + fullPath + ".git", nil
	case "gitee":
		return "https://gitee.com/" + fullPath + ".git", nil
	default:
		return baseURL + "/" + fullPath + ".git", nil
	}
}

// ImportFromProvider imports a repository from git provider
func (s *Service) ImportFromProvider(ctx context.Context, orgID int64, providerID int64, externalID string, accessToken string) (*gitprovider.Repository, error) {
	// Check if already imported
	if repo, err := s.GetByExternalID(ctx, providerID, externalID); err == nil {
		return repo, nil
	}

	client, err := s.gitProviderService.GetClient(ctx, providerID, accessToken)
	if err != nil {
		return nil, err
	}

	project, err := client.GetProject(ctx, externalID)
	if err != nil {
		return nil, err
	}

	return s.Create(ctx, &CreateRequest{
		OrganizationID: orgID,
		GitProviderID:  providerID,
		ExternalID:     externalID,
		Name:           project.Name,
		FullPath:       project.FullPath,
		DefaultBranch:  project.DefaultBranch,
	})
}

// ListBranches lists branches for a repository
func (s *Service) ListBranches(ctx context.Context, repoID int64, accessToken string) ([]string, error) {
	repo, err := s.GetByID(ctx, repoID)
	if err != nil {
		return nil, err
	}

	client, err := s.gitProviderService.GetClient(ctx, repo.GitProviderID, accessToken)
	if err != nil {
		return nil, err
	}

	branches, err := client.ListBranches(ctx, repo.ExternalID)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, b := range branches {
		names = append(names, b.Name)
	}
	return names, nil
}

// GetNextTicketNumber returns the next ticket number for a repository
func (s *Service) GetNextTicketNumber(ctx context.Context, repoID int64) (int, error) {
	var maxNumber int
	s.db.WithContext(ctx).
		Table("tickets").
		Where("repository_id = ?", repoID).
		Select("COALESCE(MAX(number), 0)").
		Scan(&maxNumber)
	return maxNumber + 1, nil
}
