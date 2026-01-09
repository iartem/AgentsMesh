package gitprovider

import (
	"context"
	"errors"

	"github.com/anthropics/agentmesh/backend/internal/domain/gitprovider"
	"github.com/anthropics/agentmesh/backend/internal/infra/git"
	"gorm.io/gorm"
)

var (
	ErrProviderNotFound  = errors.New("git provider not found")
	ErrProviderNameExists = errors.New("provider name already exists")
)

// Service handles git provider operations
type Service struct {
	db *gorm.DB
}

// NewService creates a new git provider service
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// CreateRequest represents git provider creation request
type CreateRequest struct {
	OrganizationID      int64
	ProviderType        string
	Name                string
	BaseURL             string
	ClientID            *string
	ClientSecretEncrypt *string
	BotTokenEncrypt     *string
	SSHKeyID            *int64 // For SSH type providers
	IsDefault           bool
}

// Create creates a new git provider configuration
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*gitprovider.GitProvider, error) {
	// Check if name already exists
	var existing gitprovider.GitProvider
	if err := s.db.WithContext(ctx).Where("organization_id = ? AND name = ?", req.OrganizationID, req.Name).First(&existing).Error; err == nil {
		return nil, ErrProviderNameExists
	}

	provider := &gitprovider.GitProvider{
		OrganizationID:        req.OrganizationID,
		ProviderType:          req.ProviderType,
		Name:                  req.Name,
		BaseURL:               req.BaseURL,
		ClientID:              req.ClientID,
		ClientSecretEncrypted: req.ClientSecretEncrypt,
		BotTokenEncrypted:     req.BotTokenEncrypt,
		SSHKeyID:              req.SSHKeyID,
		IsDefault:             req.IsDefault,
		IsActive:              true,
	}

	// If setting as default, unset other defaults
	if req.IsDefault {
		s.db.WithContext(ctx).Model(&gitprovider.GitProvider{}).
			Where("organization_id = ?", req.OrganizationID).
			Update("is_default", false)
	}

	if err := s.db.WithContext(ctx).Create(provider).Error; err != nil {
		return nil, err
	}

	return provider, nil
}

// GetByID returns a git provider by ID
func (s *Service) GetByID(ctx context.Context, id int64) (*gitprovider.GitProvider, error) {
	var provider gitprovider.GitProvider
	if err := s.db.WithContext(ctx).First(&provider, id).Error; err != nil {
		return nil, ErrProviderNotFound
	}
	return &provider, nil
}

// Update updates a git provider
func (s *Service) Update(ctx context.Context, id int64, updates map[string]interface{}) (*gitprovider.GitProvider, error) {
	if err := s.db.WithContext(ctx).Model(&gitprovider.GitProvider{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// Delete deletes a git provider
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&gitprovider.GitProvider{}, id).Error
}

// ListByOrganization returns git providers for an organization
func (s *Service) ListByOrganization(ctx context.Context, orgID int64) ([]*gitprovider.GitProvider, error) {
	var providers []*gitprovider.GitProvider
	err := s.db.WithContext(ctx).Where("organization_id = ? AND is_active = ?", orgID, true).Find(&providers).Error
	return providers, err
}

// GetDefault returns the default git provider for an organization
func (s *Service) GetDefault(ctx context.Context, orgID int64) (*gitprovider.GitProvider, error) {
	var provider gitprovider.GitProvider
	if err := s.db.WithContext(ctx).Where("organization_id = ? AND is_default = ? AND is_active = ?", orgID, true, true).First(&provider).Error; err != nil {
		return nil, ErrProviderNotFound
	}
	return &provider, nil
}

// SetDefault sets a git provider as default
func (s *Service) SetDefault(ctx context.Context, orgID, providerID int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Unset all defaults
		if err := tx.Model(&gitprovider.GitProvider{}).
			Where("organization_id = ?", orgID).
			Update("is_default", false).Error; err != nil {
			return err
		}

		// Set new default
		return tx.Model(&gitprovider.GitProvider{}).
			Where("id = ?", providerID).
			Update("is_default", true).Error
	})
}

// GetClient returns a git client for the provider
func (s *Service) GetClient(ctx context.Context, providerID int64, accessToken string) (git.Provider, error) {
	provider, err := s.GetByID(ctx, providerID)
	if err != nil {
		return nil, err
	}

	return git.NewProvider(provider.ProviderType, provider.BaseURL, accessToken)
}

// GetClientWithBotToken returns a git client using the bot token
func (s *Service) GetClientWithBotToken(ctx context.Context, providerID int64) (git.Provider, error) {
	provider, err := s.GetByID(ctx, providerID)
	if err != nil {
		return nil, err
	}

	if provider.BotTokenEncrypted == nil {
		return nil, errors.New("bot token not configured")
	}

	// Decrypt bot token (should use proper decryption)
	botToken := *provider.BotTokenEncrypted // TODO: decrypt

	return git.NewProvider(provider.ProviderType, provider.BaseURL, botToken)
}

// TestConnection tests the connection to a git provider
func (s *Service) TestConnection(ctx context.Context, providerType, baseURL, accessToken string) error {
	client, err := git.NewProvider(providerType, baseURL, accessToken)
	if err != nil {
		return err
	}

	_, err = client.GetCurrentUser(ctx)
	return err
}
