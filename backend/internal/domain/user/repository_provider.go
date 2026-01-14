package user

import (
	"time"
)

// RepositoryProvider represents a user's connection to a Git service provider
// Used for importing repositories (fetching repository lists via API)
// This replaces organization-level git_providers
type RepositoryProvider struct {
	ID     int64 `gorm:"primaryKey" json:"id"`
	UserID int64 `gorm:"not null;index" json:"user_id"`

	// Provider info
	ProviderType string `gorm:"size:50;not null" json:"provider_type"` // github, gitlab, gitee
	Name         string `gorm:"size:100;not null" json:"name"`         // User-defined name
	BaseURL      string `gorm:"size:255;not null" json:"base_url"`     // https://github.com, https://gitlab.company.com

	// OAuth identity reference (for OAuth-based providers)
	// When set, access token is retrieved from the linked Identity
	IdentityID *int64    `gorm:"index" json:"identity_id,omitempty"`
	Identity   *Identity `gorm:"foreignKey:IdentityID" json:"identity,omitempty"`

	// OAuth configuration (for API access)
	ClientID              *string `gorm:"size:255" json:"client_id,omitempty"`
	ClientSecretEncrypted *string `gorm:"type:text" json:"-"`

	// Bot token (alternative to OAuth)
	BotTokenEncrypted *string `gorm:"type:text" json:"-"`

	// Status
	IsDefault bool `gorm:"not null;default:false" json:"is_default"`
	IsActive  bool `gorm:"not null;default:true" json:"is_active"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`

	// Associations
	User           *User            `gorm:"foreignKey:UserID" json:"user,omitempty"`
	GitCredentials []*GitCredential `gorm:"foreignKey:RepositoryProviderID" json:"git_credentials,omitempty"`
}

func (RepositoryProvider) TableName() string {
	return "user_repository_providers"
}

// RepositoryProviderResponse is the API response for a repository provider
type RepositoryProviderResponse struct {
	ID           int64  `json:"id"`
	ProviderType string `json:"provider_type"`
	Name         string `json:"name"`
	BaseURL      string `json:"base_url"`
	HasClientID  bool   `json:"has_client_id"`
	HasBotToken  bool   `json:"has_bot_token"`
	HasIdentity  bool   `json:"has_identity"`  // Has linked OAuth identity with access token
	IsDefault    bool   `json:"is_default"`
	IsActive     bool   `json:"is_active"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// ToResponse converts RepositoryProvider to API response
func (p *RepositoryProvider) ToResponse() *RepositoryProviderResponse {
	// HasIdentity is true only if linked Identity has an access token
	hasIdentity := p.IdentityID != nil &&
		p.Identity != nil &&
		p.Identity.AccessTokenEncrypted != nil &&
		*p.Identity.AccessTokenEncrypted != ""

	return &RepositoryProviderResponse{
		ID:           p.ID,
		ProviderType: p.ProviderType,
		Name:         p.Name,
		BaseURL:      p.BaseURL,
		HasClientID:  p.ClientID != nil && *p.ClientID != "",
		HasBotToken:  p.BotTokenEncrypted != nil && *p.BotTokenEncrypted != "",
		HasIdentity:  hasIdentity,
		IsDefault:    p.IsDefault,
		IsActive:     p.IsActive,
		CreatedAt:    p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    p.UpdatedAt.Format(time.RFC3339),
	}
}

// Provider types
const (
	ProviderTypeGitHub = "github"
	ProviderTypeGitLab = "gitlab"
	ProviderTypeGitee  = "gitee"
)

// ValidProviderTypes returns valid provider types
func ValidProviderTypes() []string {
	return []string{ProviderTypeGitHub, ProviderTypeGitLab, ProviderTypeGitee}
}

// IsValidProviderType checks if the provider type is valid
func IsValidProviderType(providerType string) bool {
	for _, t := range ValidProviderTypes() {
		if t == providerType {
			return true
		}
	}
	return false
}
