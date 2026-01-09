package user

import (
	"time"
)

// User represents a user in the system (globally unique)
type User struct {
	ID           int64   `gorm:"primaryKey" json:"id"`
	Email        string  `gorm:"size:255;not null;uniqueIndex" json:"email"`
	Username     string  `gorm:"size:255;not null;uniqueIndex" json:"username"`
	Name         *string `gorm:"size:255" json:"name,omitempty"`
	AvatarURL    *string `gorm:"type:text" json:"avatar_url,omitempty"`
	PasswordHash *string `gorm:"size:255" json:"-"` // Never expose in JSON

	IsActive    bool       `gorm:"not null;default:true" json:"is_active"`
	LastLoginAt *time.Time `json:"last_login_at,omitempty"`

	// Email verification fields
	IsEmailVerified            bool       `gorm:"not null;default:false" json:"is_email_verified"`
	EmailVerificationToken     *string    `gorm:"size:255" json:"-"`
	EmailVerificationExpiresAt *time.Time `json:"-"`

	// Password reset fields
	PasswordResetToken     *string    `gorm:"size:255" json:"-"`
	PasswordResetExpiresAt *time.Time `json:"-"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`

	// Associations
	Identities []Identity `gorm:"foreignKey:UserID" json:"identities,omitempty"`
}

func (User) TableName() string {
	return "users"
}

// Identity represents an OAuth identity linked to a user
type Identity struct {
	ID     int64 `gorm:"primaryKey" json:"id"`
	UserID int64 `gorm:"not null;index" json:"user_id"`

	Provider         string  `gorm:"size:50;not null" json:"provider"` // github, google, gitlab, gitee
	ProviderUserID   string  `gorm:"size:255;not null" json:"provider_user_id"`
	ProviderUsername *string `gorm:"size:255" json:"provider_username,omitempty"`

	AccessTokenEncrypted  *string    `gorm:"type:text" json:"-"`
	RefreshTokenEncrypted *string    `gorm:"type:text" json:"-"`
	TokenExpiresAt        *time.Time `json:"token_expires_at,omitempty"`

	CreatedAt time.Time `gorm:"not null;default:now()" json:"created_at"`
	UpdatedAt time.Time `gorm:"not null;default:now()" json:"updated_at"`

	// Associations
	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (Identity) TableName() string {
	return "user_identities"
}

// UserWithOrgs represents a user with their organization memberships
type UserWithOrgs struct {
	User
	Organizations []UserOrganization `json:"organizations,omitempty"`
}

// UserOrganization represents a user's membership in an organization
type UserOrganization struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	Role     string `json:"role"`
	LogoURL  string `json:"logo_url,omitempty"`
	JoinedAt string `json:"joined_at"`
}
