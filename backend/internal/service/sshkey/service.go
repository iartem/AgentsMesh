package sshkey

import (
	"context"
	"errors"
	"fmt"

	"github.com/anthropics/agentmesh/backend/internal/domain/sshkey"
	"gorm.io/gorm"
)

var (
	ErrSSHKeyNotFound    = errors.New("SSH key not found")
	ErrSSHKeyNameExists  = errors.New("SSH key name already exists")
	ErrInvalidPrivateKey = errors.New("invalid private key format")
)

// Service handles SSH key operations
type Service struct {
	db *gorm.DB
}

// NewService creates a new SSH key service
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// CreateRequest represents SSH key creation request
type CreateRequest struct {
	OrganizationID int64
	Name           string
	PrivateKey     *string // If nil, generate a new key pair
}

// Create creates a new SSH key (either from provided private key or generate new)
func (s *Service) Create(ctx context.Context, req *CreateRequest) (*sshkey.SSHKey, error) {
	// Check if name already exists
	var existing sshkey.SSHKey
	if err := s.db.WithContext(ctx).Where("organization_id = ? AND name = ?", req.OrganizationID, req.Name).First(&existing).Error; err == nil {
		return nil, ErrSSHKeyNameExists
	}

	var publicKey, privateKey, fingerprint string

	if req.PrivateKey != nil {
		// Validate and extract public key from provided private key
		if err := sshkey.ValidatePrivateKey(*req.PrivateKey); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrInvalidPrivateKey, err)
		}

		var err error
		publicKey, err = sshkey.ExtractPublicKeyFromPrivate(*req.PrivateKey)
		if err != nil {
			return nil, fmt.Errorf("failed to extract public key: %w", err)
		}

		fingerprint, err = sshkey.CalculateFingerprint(publicKey)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate fingerprint: %w", err)
		}

		privateKey = *req.PrivateKey
	} else {
		// Generate a new key pair
		keyPair, err := sshkey.GenerateKeyPair(req.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to generate key pair: %w", err)
		}
		publicKey = keyPair.PublicKey
		privateKey = keyPair.PrivateKey
		fingerprint = keyPair.Fingerprint
	}

	key := &sshkey.SSHKey{
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		PublicKey:      publicKey,
		PrivateKeyEnc:  privateKey, // TODO: encrypt in P2
		Fingerprint:    fingerprint,
	}

	if err := s.db.WithContext(ctx).Create(key).Error; err != nil {
		return nil, err
	}

	return key, nil
}

// GetByID returns an SSH key by ID
func (s *Service) GetByID(ctx context.Context, id int64) (*sshkey.SSHKey, error) {
	var key sshkey.SSHKey
	if err := s.db.WithContext(ctx).First(&key, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSSHKeyNotFound
		}
		return nil, err
	}
	return &key, nil
}

// GetByIDAndOrg returns an SSH key by ID and organization ID (for authorization)
func (s *Service) GetByIDAndOrg(ctx context.Context, id, orgID int64) (*sshkey.SSHKey, error) {
	var key sshkey.SSHKey
	if err := s.db.WithContext(ctx).Where("id = ? AND organization_id = ?", id, orgID).First(&key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSSHKeyNotFound
		}
		return nil, err
	}
	return &key, nil
}

// ListByOrganization returns all SSH keys for an organization
func (s *Service) ListByOrganization(ctx context.Context, orgID int64) ([]*sshkey.SSHKey, error) {
	var keys []*sshkey.SSHKey
	err := s.db.WithContext(ctx).Where("organization_id = ?", orgID).Order("created_at DESC").Find(&keys).Error
	return keys, err
}

// Update updates an SSH key (only name can be updated)
func (s *Service) Update(ctx context.Context, id int64, name string) (*sshkey.SSHKey, error) {
	// Check if new name conflicts with existing key
	var existing sshkey.SSHKey
	key, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if key.Name != name {
		if err := s.db.WithContext(ctx).Where("organization_id = ? AND name = ? AND id != ?", key.OrganizationID, name, id).First(&existing).Error; err == nil {
			return nil, ErrSSHKeyNameExists
		}
	}

	if err := s.db.WithContext(ctx).Model(&sshkey.SSHKey{}).Where("id = ?", id).Update("name", name).Error; err != nil {
		return nil, err
	}

	return s.GetByID(ctx, id)
}

// Delete deletes an SSH key
func (s *Service) Delete(ctx context.Context, id int64) error {
	result := s.db.WithContext(ctx).Delete(&sshkey.SSHKey{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrSSHKeyNotFound
	}
	return nil
}

// GetPrivateKey returns the decrypted private key
func (s *Service) GetPrivateKey(ctx context.Context, id int64) (string, error) {
	key, err := s.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	// TODO: decrypt in P2
	return key.PrivateKeyEnc, nil
}

// ExistsInOrganization checks if an SSH key exists in an organization
func (s *Service) ExistsInOrganization(ctx context.Context, id, orgID int64) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).Model(&sshkey.SSHKey{}).Where("id = ? AND organization_id = ?", id, orgID).Count(&count).Error
	return count > 0, err
}
