package sshkey

import (
	"context"

	"github.com/anthropics/agentmesh/backend/internal/domain/sshkey"
)

// Interface defines the SSH key service operations
type Interface interface {
	// Create creates a new SSH key (either from provided private key or generate new)
	Create(ctx context.Context, req *CreateRequest) (*sshkey.SSHKey, error)

	// GetByID returns an SSH key by ID
	GetByID(ctx context.Context, id int64) (*sshkey.SSHKey, error)

	// GetByIDAndOrg returns an SSH key by ID and organization ID (for authorization)
	GetByIDAndOrg(ctx context.Context, id, orgID int64) (*sshkey.SSHKey, error)

	// ListByOrganization returns all SSH keys for an organization
	ListByOrganization(ctx context.Context, orgID int64) ([]*sshkey.SSHKey, error)

	// Update updates an SSH key (only name can be updated)
	Update(ctx context.Context, id int64, name string) (*sshkey.SSHKey, error)

	// Delete deletes an SSH key
	Delete(ctx context.Context, id int64) error

	// GetPrivateKey returns the decrypted private key
	GetPrivateKey(ctx context.Context, id int64) (string, error)

	// ExistsInOrganization checks if an SSH key exists in an organization
	ExistsInOrganization(ctx context.Context, id, orgID int64) (bool, error)
}

// Ensure Service implements Interface
var _ Interface = (*Service)(nil)

// Ensure MockService implements Interface
var _ Interface = (*MockService)(nil)
