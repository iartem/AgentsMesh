package user

import (
	"context"
	"errors"
	"time"

	"github.com/anthropics/agentmesh/backend/internal/domain/user"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrUserNotFound       = errors.New("user not found")
	ErrEmailAlreadyExists = errors.New("email already exists")
	ErrUsernameExists     = errors.New("username already exists")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrUserInactive       = errors.New("user is inactive")
)

// Service handles user operations
type Service struct {
	db *gorm.DB
}

// NewService creates a new user service
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// CreateRequest represents user creation request
type CreateRequest struct {
	Email    string
	Username string
	Name     string
	Password string
}

// Create creates a new user
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*user.User, error) {
	// Check if email already exists
	var existing user.User
	if err := s.db.WithContext(ctx).Where("email = ?", req.Email).First(&existing).Error; err == nil {
		return nil, ErrEmailAlreadyExists
	}

	// Check if username already exists
	if err := s.db.WithContext(ctx).Where("username = ?", req.Username).First(&existing).Error; err == nil {
		return nil, ErrUsernameExists
	}

	// Hash password if provided
	var passwordHash string
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		passwordHash = string(hash)
	}

	u := &user.User{
		Email:    req.Email,
		Username: req.Username,
		IsActive: true,
	}
	if req.Name != "" {
		u.Name = &req.Name
	}
	if passwordHash != "" {
		u.PasswordHash = &passwordHash
	}

	if err := s.db.WithContext(ctx).Create(u).Error; err != nil {
		return nil, err
	}

	return u, nil
}

// GetByID returns a user by ID
func (s *Service) GetByID(ctx context.Context, id int64) (*user.User, error) {
	var u user.User
	if err := s.db.WithContext(ctx).First(&u, id).Error; err != nil {
		return nil, ErrUserNotFound
	}
	return &u, nil
}

// GetByEmail returns a user by email
func (s *Service) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	var u user.User
	if err := s.db.WithContext(ctx).Where("email = ?", email).First(&u).Error; err != nil {
		return nil, ErrUserNotFound
	}
	return &u, nil
}

// GetByUsername returns a user by username
func (s *Service) GetByUsername(ctx context.Context, username string) (*user.User, error) {
	var u user.User
	if err := s.db.WithContext(ctx).Where("username = ?", username).First(&u).Error; err != nil {
		return nil, ErrUserNotFound
	}
	return &u, nil
}

// Update updates a user
func (s *Service) Update(ctx context.Context, id int64, updates map[string]interface{}) (*user.User, error) {
	if err := s.db.WithContext(ctx).Model(&user.User{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return nil, err
	}
	return s.GetByID(ctx, id)
}

// UpdatePassword updates a user's password
func (s *Service) UpdatePassword(ctx context.Context, id int64, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Model(&user.User{}).Where("id = ?", id).Update("password_hash", string(hash)).Error
}

// Delete deletes a user
func (s *Service) Delete(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Delete(&user.User{}, id).Error
}

// Authenticate authenticates a user by email and password
func (s *Service) Authenticate(ctx context.Context, email, password string) (*user.User, error) {
	u, err := s.GetByEmail(ctx, email)
	if err != nil {
		return nil, ErrInvalidCredentials
	}

	if !u.IsActive {
		return nil, ErrUserInactive
	}

	if u.PasswordHash == nil || *u.PasswordHash == "" {
		return nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(*u.PasswordHash), []byte(password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	// Update last login
	now := time.Now()
	s.db.WithContext(ctx).Model(u).Update("last_login_at", now)

	return u, nil
}

// GetOrCreateByOAuth gets or creates a user from OAuth identity
func (s *Service) GetOrCreateByOAuth(ctx context.Context, provider, providerUserID, providerUsername, email, name, avatarURL string) (*user.User, bool, error) {
	// Check if identity already exists
	var identity user.Identity
	if err := s.db.WithContext(ctx).Where("provider = ? AND provider_user_id = ?", provider, providerUserID).First(&identity).Error; err == nil {
		// Identity exists, get user
		u, err := s.GetByID(ctx, identity.UserID)
		return u, false, err
	}

	// Check if user with email exists
	var u *user.User
	var isNew bool
	existing, err := s.GetByEmail(ctx, email)
	if err == nil {
		u = existing
	} else {
		// Create new user
		username := providerUsername
		if username == "" {
			username = email
		}

		// Ensure username is unique
		for i := 0; i < 100; i++ {
			if _, err := s.GetByUsername(ctx, username); err != nil {
				break
			}
			username = providerUsername + "_" + string(rune('0'+i))
		}

		u = &user.User{
			Email:    email,
			Username: username,
			IsActive: true,
		}
		if name != "" {
			u.Name = &name
		}
		if avatarURL != "" {
			u.AvatarURL = &avatarURL
		}

		if err := s.db.WithContext(ctx).Create(u).Error; err != nil {
			return nil, false, err
		}
		isNew = true
	}

	// Create identity
	identity = user.Identity{
		UserID:         u.ID,
		Provider:       provider,
		ProviderUserID: providerUserID,
	}
	if providerUsername != "" {
		identity.ProviderUsername = &providerUsername
	}

	if err := s.db.WithContext(ctx).Create(&identity).Error; err != nil {
		return nil, false, err
	}

	return u, isNew, nil
}

// UpdateIdentityTokens updates OAuth tokens for an identity
func (s *Service) UpdateIdentityTokens(ctx context.Context, userID int64, provider, accessToken, refreshToken string, expiresAt *time.Time) error {
	return s.db.WithContext(ctx).Model(&user.Identity{}).
		Where("user_id = ? AND provider = ?", userID, provider).
		Updates(map[string]interface{}{
			"access_token_encrypted":  accessToken, // Should be encrypted
			"refresh_token_encrypted": refreshToken,
			"token_expires_at":        expiresAt,
		}).Error
}

// GetIdentity returns an OAuth identity
func (s *Service) GetIdentity(ctx context.Context, userID int64, provider string) (*user.Identity, error) {
	var identity user.Identity
	if err := s.db.WithContext(ctx).Where("user_id = ? AND provider = ?", userID, provider).First(&identity).Error; err != nil {
		return nil, err
	}
	return &identity, nil
}

// ListIdentities returns all identities for a user
func (s *Service) ListIdentities(ctx context.Context, userID int64) ([]*user.Identity, error) {
	var identities []*user.Identity
	err := s.db.WithContext(ctx).Where("user_id = ?", userID).Find(&identities).Error
	return identities, err
}

// DeleteIdentity deletes an OAuth identity
func (s *Service) DeleteIdentity(ctx context.Context, userID int64, provider string) error {
	return s.db.WithContext(ctx).Where("user_id = ? AND provider = ?", userID, provider).Delete(&user.Identity{}).Error
}

// Search searches for users
func (s *Service) Search(ctx context.Context, query string, limit int) ([]*user.User, error) {
	var users []*user.User
	err := s.db.WithContext(ctx).
		Where("username ILIKE ? OR name ILIKE ? OR email ILIKE ?", "%"+query+"%", "%"+query+"%", "%"+query+"%").
		Limit(limit).
		Find(&users).Error
	return users, err
}
