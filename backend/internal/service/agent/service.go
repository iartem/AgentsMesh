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

// AgentTypeInfo is a simplified agent type for Runner initialization
type AgentTypeInfo struct {
	Slug          string `json:"slug"`
	Name          string `json:"name"`
	Executable    string `json:"executable"`
	LaunchCommand string `json:"launch_command"`
}

// GetAgentTypesForRunner returns agent types for Runner initialization handshake
// This implements the runner.AgentTypesProvider interface
func (s *Service) GetAgentTypesForRunner() []AgentTypeInfo {
	var types []*agent.AgentType
	if err := s.db.Where("is_active = ?", true).Find(&types).Error; err != nil {
		return nil
	}

	result := make([]AgentTypeInfo, 0, len(types))
	for _, t := range types {
		result = append(result, AgentTypeInfo{
			Slug:          t.Slug,
			Name:          t.Name,
			Executable:    t.Executable,
			LaunchCommand: t.LaunchCommand,
		})
	}
	return result
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

// Organization Default Config Methods

// GetOrganizationDefaultConfig returns the organization's default config for an agent type
func (s *Service) GetOrganizationDefaultConfig(ctx context.Context, orgID, agentTypeID int64) (*agent.OrganizationAgentConfig, error) {
	var config agent.OrganizationAgentConfig
	err := s.db.WithContext(ctx).
		Preload("AgentType").
		Where("organization_id = ? AND agent_type_id = ?", orgID, agentTypeID).
		First(&config).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Return empty config if not found
			return &agent.OrganizationAgentConfig{
				OrganizationID: orgID,
				AgentTypeID:    agentTypeID,
				ConfigValues:   make(agent.ConfigValues),
			}, nil
		}
		return nil, err
	}
	return &config, nil
}

// SetOrganizationDefaultConfig sets the organization's default config for an agent type
func (s *Service) SetOrganizationDefaultConfig(ctx context.Context, orgID, agentTypeID int64, configValues agent.ConfigValues) (*agent.OrganizationAgentConfig, error) {
	// Verify agent type exists
	if _, err := s.GetAgentType(ctx, agentTypeID); err != nil {
		return nil, err
	}

	// Try to find existing config
	var existing agent.OrganizationAgentConfig
	err := s.db.WithContext(ctx).
		Where("organization_id = ? AND agent_type_id = ?", orgID, agentTypeID).
		First(&existing).Error

	if err != nil {
		// Record doesn't exist, create new one
		config := &agent.OrganizationAgentConfig{
			OrganizationID: orgID,
			AgentTypeID:    agentTypeID,
			ConfigValues:   configValues,
		}
		if err := s.db.WithContext(ctx).Create(config).Error; err != nil {
			return nil, err
		}
	} else {
		// Record exists, update config_values explicitly
		err = s.db.WithContext(ctx).
			Model(&existing).
			Update("config_values", configValues).Error
		if err != nil {
			return nil, err
		}
	}

	return s.GetOrganizationDefaultConfig(ctx, orgID, agentTypeID)
}

// DeleteOrganizationDefaultConfig deletes the organization's default config for an agent type
func (s *Service) DeleteOrganizationDefaultConfig(ctx context.Context, orgID, agentTypeID int64) error {
	return s.db.WithContext(ctx).
		Where("organization_id = ? AND agent_type_id = ?", orgID, agentTypeID).
		Delete(&agent.OrganizationAgentConfig{}).Error
}

// ListOrganizationDefaultConfigs returns all default configs for an organization
func (s *Service) ListOrganizationDefaultConfigs(ctx context.Context, orgID int64) ([]*agent.OrganizationAgentConfig, error) {
	var configs []*agent.OrganizationAgentConfig
	err := s.db.WithContext(ctx).
		Preload("AgentType").
		Where("organization_id = ?", orgID).
		Find(&configs).Error
	return configs, err
}

// GetEffectiveConfig returns the effective config by merging system defaults, org defaults, and overrides
func (s *Service) GetEffectiveConfig(ctx context.Context, orgID, agentTypeID int64, overrides agent.ConfigValues) agent.ConfigValues {
	// Start with empty config
	result := make(agent.ConfigValues)

	// Get organization default config
	orgConfig, err := s.GetOrganizationDefaultConfig(ctx, orgID, agentTypeID)
	if err == nil && orgConfig.ConfigValues != nil {
		result = agent.MergeConfigs(result, orgConfig.ConfigValues)
	}

	// Apply overrides
	if overrides != nil {
		result = agent.MergeConfigs(result, overrides)
	}

	return result
}

// ============================================================================
// User Agent Credential Profile Methods
// ============================================================================

var (
	ErrCredentialProfileNotFound = errors.New("credential profile not found")
	ErrCredentialProfileExists   = errors.New("credential profile with this name already exists")
)

// CreateCredentialProfile creates a new credential profile for a user
func (s *Service) CreateCredentialProfile(ctx context.Context, userID int64, req *agent.CreateCredentialProfileRequest) (*agent.UserAgentCredentialProfile, error) {
	// Verify agent type exists
	if _, err := s.GetAgentType(ctx, req.AgentTypeID); err != nil {
		return nil, err
	}

	// Check if profile with same name exists
	var existing agent.UserAgentCredentialProfile
	err := s.db.WithContext(ctx).
		Where("user_id = ? AND agent_type_id = ? AND name = ?", userID, req.AgentTypeID, req.Name).
		First(&existing).Error
	if err == nil {
		return nil, ErrCredentialProfileExists
	}

	// If setting as default, unset other defaults for this agent type
	if req.IsDefault {
		s.db.WithContext(ctx).Model(&agent.UserAgentCredentialProfile{}).
			Where("user_id = ? AND agent_type_id = ?", userID, req.AgentTypeID).
			Update("is_default", false)
	}

	// Encrypt credentials if provided
	var encryptedCreds agent.EncryptedCredentials
	if !req.IsRunnerHost && req.Credentials != nil {
		encryptedCreds = make(agent.EncryptedCredentials)
		for k, v := range req.Credentials {
			// TODO: Actually encrypt the value using encryption service
			encryptedCreds[k] = v
		}
	}

	profile := &agent.UserAgentCredentialProfile{
		UserID:               userID,
		AgentTypeID:          req.AgentTypeID,
		Name:                 req.Name,
		Description:          req.Description,
		IsRunnerHost:         req.IsRunnerHost,
		CredentialsEncrypted: encryptedCreds,
		IsDefault:            req.IsDefault,
		IsActive:             true,
	}

	if err := s.db.WithContext(ctx).Create(profile).Error; err != nil {
		return nil, err
	}

	// Reload with AgentType
	return s.GetCredentialProfile(ctx, userID, profile.ID)
}

// GetCredentialProfile returns a credential profile by ID
func (s *Service) GetCredentialProfile(ctx context.Context, userID, profileID int64) (*agent.UserAgentCredentialProfile, error) {
	var profile agent.UserAgentCredentialProfile
	err := s.db.WithContext(ctx).
		Preload("AgentType").
		Where("id = ? AND user_id = ?", profileID, userID).
		First(&profile).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCredentialProfileNotFound
		}
		return nil, err
	}
	return &profile, nil
}

// UpdateCredentialProfile updates an existing credential profile
func (s *Service) UpdateCredentialProfile(ctx context.Context, userID, profileID int64, req *agent.UpdateCredentialProfileRequest) (*agent.UserAgentCredentialProfile, error) {
	profile, err := s.GetCredentialProfile(ctx, userID, profileID)
	if err != nil {
		return nil, err
	}

	// Check name uniqueness if changing
	if req.Name != nil && *req.Name != profile.Name {
		var existing agent.UserAgentCredentialProfile
		err := s.db.WithContext(ctx).
			Where("user_id = ? AND agent_type_id = ? AND name = ? AND id != ?", userID, profile.AgentTypeID, *req.Name, profileID).
			First(&existing).Error
		if err == nil {
			return nil, ErrCredentialProfileExists
		}
	}

	// If setting as default, unset other defaults
	if req.IsDefault != nil && *req.IsDefault && !profile.IsDefault {
		s.db.WithContext(ctx).Model(&agent.UserAgentCredentialProfile{}).
			Where("user_id = ? AND agent_type_id = ? AND id != ?", userID, profile.AgentTypeID, profileID).
			Update("is_default", false)
	}

	// Build updates
	updates := make(map[string]interface{})
	if req.Name != nil {
		updates["name"] = *req.Name
	}
	if req.Description != nil {
		updates["description"] = *req.Description
	}
	if req.IsRunnerHost != nil {
		updates["is_runner_host"] = *req.IsRunnerHost
		if *req.IsRunnerHost {
			// Clear credentials when switching to RunnerHost
			updates["credentials_encrypted"] = nil
		}
	}
	if req.IsDefault != nil {
		updates["is_default"] = *req.IsDefault
	}
	if req.IsActive != nil {
		updates["is_active"] = *req.IsActive
	}

	// Update credentials if provided
	if req.Credentials != nil {
		encryptedCreds := make(agent.EncryptedCredentials)
		for k, v := range req.Credentials {
			// TODO: Actually encrypt the value using encryption service
			encryptedCreds[k] = v
		}
		updates["credentials_encrypted"] = encryptedCreds
	}

	if len(updates) > 0 {
		if err := s.db.WithContext(ctx).Model(profile).Updates(updates).Error; err != nil {
			return nil, err
		}
	}

	return s.GetCredentialProfile(ctx, userID, profileID)
}

// DeleteCredentialProfile deletes a credential profile
func (s *Service) DeleteCredentialProfile(ctx context.Context, userID, profileID int64) error {
	result := s.db.WithContext(ctx).
		Where("id = ? AND user_id = ?", profileID, userID).
		Delete(&agent.UserAgentCredentialProfile{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrCredentialProfileNotFound
	}
	return nil
}

// ListCredentialProfiles returns all credential profiles for a user, grouped by agent type
func (s *Service) ListCredentialProfiles(ctx context.Context, userID int64) ([]*agent.CredentialProfilesByAgentType, error) {
	var profiles []*agent.UserAgentCredentialProfile
	err := s.db.WithContext(ctx).
		Preload("AgentType").
		Where("user_id = ? AND is_active = ?", userID, true).
		Order("agent_type_id, is_default DESC, name").
		Find(&profiles).Error
	if err != nil {
		return nil, err
	}

	// Group by agent type
	groupedMap := make(map[int64]*agent.CredentialProfilesByAgentType)
	for _, p := range profiles {
		group, exists := groupedMap[p.AgentTypeID]
		if !exists {
			group = &agent.CredentialProfilesByAgentType{
				AgentTypeID: p.AgentTypeID,
				Profiles:    make([]*agent.CredentialProfileResponse, 0),
			}
			if p.AgentType != nil {
				group.AgentTypeName = p.AgentType.Name
				group.AgentTypeSlug = p.AgentType.Slug
			}
			groupedMap[p.AgentTypeID] = group
		}
		group.Profiles = append(group.Profiles, p.ToResponse())
	}

	// Convert map to slice
	result := make([]*agent.CredentialProfilesByAgentType, 0, len(groupedMap))
	for _, group := range groupedMap {
		result = append(result, group)
	}

	return result, nil
}

// ListCredentialProfilesForAgentType returns all credential profiles for a specific agent type
func (s *Service) ListCredentialProfilesForAgentType(ctx context.Context, userID, agentTypeID int64) ([]*agent.UserAgentCredentialProfile, error) {
	var profiles []*agent.UserAgentCredentialProfile
	err := s.db.WithContext(ctx).
		Preload("AgentType").
		Where("user_id = ? AND agent_type_id = ? AND is_active = ?", userID, agentTypeID, true).
		Order("is_default DESC, name").
		Find(&profiles).Error
	return profiles, err
}

// GetDefaultCredentialProfile returns the default credential profile for a user and agent type
func (s *Service) GetDefaultCredentialProfile(ctx context.Context, userID, agentTypeID int64) (*agent.UserAgentCredentialProfile, error) {
	var profile agent.UserAgentCredentialProfile
	err := s.db.WithContext(ctx).
		Preload("AgentType").
		Where("user_id = ? AND agent_type_id = ? AND is_default = ? AND is_active = ?", userID, agentTypeID, true, true).
		First(&profile).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCredentialProfileNotFound
		}
		return nil, err
	}
	return &profile, nil
}

// SetDefaultCredentialProfile sets a profile as the default for its agent type
func (s *Service) SetDefaultCredentialProfile(ctx context.Context, userID, profileID int64) (*agent.UserAgentCredentialProfile, error) {
	profile, err := s.GetCredentialProfile(ctx, userID, profileID)
	if err != nil {
		return nil, err
	}

	// Unset other defaults
	s.db.WithContext(ctx).Model(&agent.UserAgentCredentialProfile{}).
		Where("user_id = ? AND agent_type_id = ? AND id != ?", userID, profile.AgentTypeID, profileID).
		Update("is_default", false)

	// Set this as default
	if err := s.db.WithContext(ctx).Model(profile).Update("is_default", true).Error; err != nil {
		return nil, err
	}

	return s.GetCredentialProfile(ctx, userID, profileID)
}

// GetEffectiveCredentialsForPod returns the credentials to be injected for a pod
// Returns nil if using RunnerHost mode
func (s *Service) GetEffectiveCredentialsForPod(ctx context.Context, userID, agentTypeID int64, profileID *int64) (agent.EncryptedCredentials, bool, error) {
	var profile *agent.UserAgentCredentialProfile
	var err error

	if profileID != nil && *profileID > 0 {
		// Use specified profile
		profile, err = s.GetCredentialProfile(ctx, userID, *profileID)
		if err != nil {
			return nil, false, err
		}
	} else {
		// Use default profile
		profile, err = s.GetDefaultCredentialProfile(ctx, userID, agentTypeID)
		if err != nil {
			if errors.Is(err, ErrCredentialProfileNotFound) {
				// No default profile, use RunnerHost mode
				return nil, true, nil
			}
			return nil, false, err
		}
	}

	if profile.IsRunnerHost {
		return nil, true, nil
	}

	return profile.CredentialsEncrypted, false, nil
}
