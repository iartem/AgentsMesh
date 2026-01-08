package agent

import (
	"context"
	"errors"

	"github.com/anthropics/agentmesh/backend/internal/domain/agent"
	"gorm.io/gorm"
)

var (
	ErrAgentTypeNotFound   = errors.New("agent type not found")
	ErrAgentSlugExists     = errors.New("agent type slug already exists")
	ErrCredentialsRequired = errors.New("required credentials missing")
)

// Service handles agent type operations
type Service struct {
	db *gorm.DB
}

// NewService creates a new agent service
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// ListBuiltinAgentTypes returns all builtin agent types
func (s *Service) ListBuiltinAgentTypes(ctx context.Context) ([]*agent.AgentType, error) {
	var types []*agent.AgentType
	err := s.db.WithContext(ctx).Where("is_builtin = ? AND is_active = ?", true, true).Find(&types).Error
	return types, err
}

// GetAgentType returns an agent type by ID
func (s *Service) GetAgentType(ctx context.Context, id int64) (*agent.AgentType, error) {
	var agentType agent.AgentType
	if err := s.db.WithContext(ctx).First(&agentType, id).Error; err != nil {
		return nil, ErrAgentTypeNotFound
	}
	return &agentType, nil
}

// GetAgentTypeBySlug returns an agent type by slug
func (s *Service) GetAgentTypeBySlug(ctx context.Context, slug string) (*agent.AgentType, error) {
	var agentType agent.AgentType
	if err := s.db.WithContext(ctx).Where("slug = ?", slug).First(&agentType).Error; err != nil {
		return nil, ErrAgentTypeNotFound
	}
	return &agentType, nil
}

// EnableAgentForOrganization enables an agent type for an organization
func (s *Service) EnableAgentForOrganization(ctx context.Context, orgID, agentTypeID int64, isDefault bool) (*agent.OrganizationAgent, error) {
	orgAgent := &agent.OrganizationAgent{
		OrganizationID: orgID,
		AgentTypeID:    agentTypeID,
		IsEnabled:      true,
		IsDefault:      isDefault,
	}

	// If setting as default, unset other defaults
	if isDefault {
		s.db.WithContext(ctx).Model(&agent.OrganizationAgent{}).
			Where("organization_id = ?", orgID).
			Update("is_default", false)
	}

	err := s.db.WithContext(ctx).
		Where("organization_id = ? AND agent_type_id = ?", orgID, agentTypeID).
		Assign(orgAgent).
		FirstOrCreate(orgAgent).Error

	return orgAgent, err
}

// DisableAgentForOrganization disables an agent type for an organization
func (s *Service) DisableAgentForOrganization(ctx context.Context, orgID, agentTypeID int64) error {
	return s.db.WithContext(ctx).Model(&agent.OrganizationAgent{}).
		Where("organization_id = ? AND agent_type_id = ?", orgID, agentTypeID).
		Update("is_enabled", false).Error
}

// ListOrganizationAgents returns enabled agents for an organization
func (s *Service) ListOrganizationAgents(ctx context.Context, orgID int64) ([]*agent.OrganizationAgent, error) {
	var agents []*agent.OrganizationAgent
	err := s.db.WithContext(ctx).
		Preload("AgentType").
		Where("organization_id = ? AND is_enabled = ?", orgID, true).
		Find(&agents).Error
	return agents, err
}

// GetDefaultAgentForOrganization returns the default agent for an organization
func (s *Service) GetDefaultAgentForOrganization(ctx context.Context, orgID int64) (*agent.OrganizationAgent, error) {
	var orgAgent agent.OrganizationAgent
	if err := s.db.WithContext(ctx).
		Preload("AgentType").
		Where("organization_id = ? AND is_default = ? AND is_enabled = ?", orgID, true, true).
		First(&orgAgent).Error; err != nil {
		return nil, ErrAgentTypeNotFound
	}
	return &orgAgent, nil
}

// SetOrganizationCredentials sets organization-level credentials for an agent
func (s *Service) SetOrganizationCredentials(ctx context.Context, orgID, agentTypeID int64, credentials agent.EncryptedCredentials) error {
	return s.db.WithContext(ctx).Model(&agent.OrganizationAgent{}).
		Where("organization_id = ? AND agent_type_id = ?", orgID, agentTypeID).
		Update("credentials_encrypted", credentials).Error
}

// GetOrganizationCredentials returns organization-level credentials for an agent
func (s *Service) GetOrganizationCredentials(ctx context.Context, orgID, agentTypeID int64) (*agent.OrganizationAgent, error) {
	var orgAgent agent.OrganizationAgent
	if err := s.db.WithContext(ctx).
		Where("organization_id = ? AND agent_type_id = ?", orgID, agentTypeID).
		First(&orgAgent).Error; err != nil {
		return nil, err
	}
	return &orgAgent, nil
}

// SetUserCredentials sets user-level credentials for an agent
func (s *Service) SetUserCredentials(ctx context.Context, userID, agentTypeID int64, credentials agent.EncryptedCredentials) error {
	userCreds := &agent.UserAgentCredential{
		UserID:               userID,
		AgentTypeID:          agentTypeID,
		CredentialsEncrypted: credentials,
	}

	return s.db.WithContext(ctx).
		Where("user_id = ? AND agent_type_id = ?", userID, agentTypeID).
		Assign(userCreds).
		FirstOrCreate(userCreds).Error
}

// GetUserCredentials returns user-level credentials for an agent
func (s *Service) GetUserCredentials(ctx context.Context, userID, agentTypeID int64) (*agent.UserAgentCredential, error) {
	var userCreds agent.UserAgentCredential
	if err := s.db.WithContext(ctx).
		Where("user_id = ? AND agent_type_id = ?", userID, agentTypeID).
		First(&userCreds).Error; err != nil {
		return nil, err
	}
	return &userCreds, nil
}

// DeleteUserCredentials deletes user-level credentials for an agent
func (s *Service) DeleteUserCredentials(ctx context.Context, userID, agentTypeID int64) error {
	return s.db.WithContext(ctx).
		Where("user_id = ? AND agent_type_id = ?", userID, agentTypeID).
		Delete(&agent.UserAgentCredential{}).Error
}

// GetEffectiveCredentials returns the effective credentials for a user/agent combination
// User credentials override organization credentials
func (s *Service) GetEffectiveCredentials(ctx context.Context, orgID, userID, agentTypeID int64) (agent.EncryptedCredentials, error) {
	result := make(agent.EncryptedCredentials)

	// Get organization credentials first
	orgAgent, err := s.GetOrganizationCredentials(ctx, orgID, agentTypeID)
	if err == nil && orgAgent.CredentialsEncrypted != nil {
		for k, v := range orgAgent.CredentialsEncrypted {
			result[k] = v
		}
	}

	// Override with user credentials
	userCreds, err := s.GetUserCredentials(ctx, userID, agentTypeID)
	if err == nil && userCreds.CredentialsEncrypted != nil {
		for k, v := range userCreds.CredentialsEncrypted {
			result[k] = v
		}
	}

	return result, nil
}

// CreateCustomAgentType creates a custom agent type for an organization
func (s *Service) CreateCustomAgentType(ctx context.Context, orgID int64, req *CreateCustomAgentRequest) (*agent.CustomAgentType, error) {
	// Check if slug already exists
	var existing agent.CustomAgentType
	if err := s.db.WithContext(ctx).Where("organization_id = ? AND slug = ?", orgID, req.Slug).First(&existing).Error; err == nil {
		return nil, ErrAgentSlugExists
	}

	customAgent := &agent.CustomAgentType{
		OrganizationID:   orgID,
		Slug:             req.Slug,
		Name:             req.Name,
		Description:      req.Description,
		LaunchCommand:    req.LaunchCommand,
		DefaultArgs:      req.DefaultArgs,
		CredentialSchema: req.CredentialSchema,
		StatusDetection:  req.StatusDetection,
		IsActive:         true,
	}

	if err := s.db.WithContext(ctx).Create(customAgent).Error; err != nil {
		return nil, err
	}

	return customAgent, nil
}

// CreateCustomAgentRequest represents a custom agent creation request
type CreateCustomAgentRequest struct {
	Slug             string
	Name             string
	Description      *string
	LaunchCommand    string
	DefaultArgs      *string
	CredentialSchema agent.CredentialSchema
	StatusDetection  agent.StatusDetection
}

// UpdateCustomAgentType updates a custom agent type
func (s *Service) UpdateCustomAgentType(ctx context.Context, id int64, updates map[string]interface{}) (*agent.CustomAgentType, error) {
	if err := s.db.WithContext(ctx).Model(&agent.CustomAgentType{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}

	var customAgent agent.CustomAgentType
	if err := s.db.WithContext(ctx).First(&customAgent, id).Error; err != nil {
		return nil, err
	}
	return &customAgent, nil
}

// DeleteCustomAgentType deletes a custom agent type
func (s *Service) DeleteCustomAgentType(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&agent.CustomAgentType{}, id).Error
}

// ListCustomAgentTypes returns custom agent types for an organization
func (s *Service) ListCustomAgentTypes(ctx context.Context, orgID int64) ([]*agent.CustomAgentType, error) {
	var types []*agent.CustomAgentType
	err := s.db.WithContext(ctx).Where("organization_id = ? AND is_active = ?", orgID, true).Find(&types).Error
	return types, err
}

// GetCustomAgentType returns a custom agent type by ID
func (s *Service) GetCustomAgentType(ctx context.Context, id int64) (*agent.CustomAgentType, error) {
	var customAgent agent.CustomAgentType
	if err := s.db.WithContext(ctx).First(&customAgent, id).Error; err != nil {
		return nil, ErrAgentTypeNotFound
	}
	return &customAgent, nil
}
