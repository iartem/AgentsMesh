package sshkey

import (
	"context"
	"sync"

	"github.com/anthropics/agentmesh/backend/internal/domain/sshkey"
)

// MockService is a mock implementation of SSH key service for testing
type MockService struct {
	mu        sync.RWMutex
	keys      map[int64]*sshkey.SSHKey
	nextID    int64
	createErr error
	getErr    error
	listErr   error
	updateErr error
	deleteErr error
}

// NewMockService creates a new mock SSH key service
func NewMockService() *MockService {
	return &MockService{
		keys:   make(map[int64]*sshkey.SSHKey),
		nextID: 1,
	}
}

// SetCreateErr sets the error to return on Create
func (m *MockService) SetCreateErr(err error) {
	m.createErr = err
}

// SetGetErr sets the error to return on Get operations
func (m *MockService) SetGetErr(err error) {
	m.getErr = err
}

// SetListErr sets the error to return on List
func (m *MockService) SetListErr(err error) {
	m.listErr = err
}

// SetUpdateErr sets the error to return on Update
func (m *MockService) SetUpdateErr(err error) {
	m.updateErr = err
}

// SetDeleteErr sets the error to return on Delete
func (m *MockService) SetDeleteErr(err error) {
	m.deleteErr = err
}

// AddKey adds a key to the mock store
func (m *MockService) AddKey(key *sshkey.SSHKey) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if key.ID == 0 {
		key.ID = m.nextID
		m.nextID++
	}
	m.keys[key.ID] = key
}

// Create creates a new SSH key
func (m *MockService) Create(ctx context.Context, req *CreateRequest) (*sshkey.SSHKey, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check for duplicate name
	for _, key := range m.keys {
		if key.OrganizationID == req.OrganizationID && key.Name == req.Name {
			return nil, ErrSSHKeyNameExists
		}
	}

	// Validate private key if provided
	if req.PrivateKey != nil {
		if err := sshkey.ValidatePrivateKey(*req.PrivateKey); err != nil {
			return nil, ErrInvalidPrivateKey
		}
	}

	key := &sshkey.SSHKey{
		ID:             m.nextID,
		OrganizationID: req.OrganizationID,
		Name:           req.Name,
		PublicKey:      "ssh-rsa AAAA... mock-key",
		PrivateKeyEnc:  "mock-private-key",
		Fingerprint:    "SHA256:mock-fingerprint",
	}
	m.nextID++
	m.keys[key.ID] = key
	return key, nil
}

// GetByID returns an SSH key by ID
func (m *MockService) GetByID(ctx context.Context, id int64) (*sshkey.SSHKey, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	key, ok := m.keys[id]
	if !ok {
		return nil, ErrSSHKeyNotFound
	}
	return key, nil
}

// GetByIDAndOrg returns an SSH key by ID and organization ID
func (m *MockService) GetByIDAndOrg(ctx context.Context, id, orgID int64) (*sshkey.SSHKey, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	key, ok := m.keys[id]
	if !ok || key.OrganizationID != orgID {
		return nil, ErrSSHKeyNotFound
	}
	return key, nil
}

// ListByOrganization returns all SSH keys for an organization
func (m *MockService) ListByOrganization(ctx context.Context, orgID int64) ([]*sshkey.SSHKey, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var keys []*sshkey.SSHKey
	for _, key := range m.keys {
		if key.OrganizationID == orgID {
			keys = append(keys, key)
		}
	}
	return keys, nil
}

// Update updates an SSH key name
func (m *MockService) Update(ctx context.Context, id int64, name string) (*sshkey.SSHKey, error) {
	if m.updateErr != nil {
		return nil, m.updateErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key, ok := m.keys[id]
	if !ok {
		return nil, ErrSSHKeyNotFound
	}

	// Check for duplicate name
	for _, k := range m.keys {
		if k.ID != id && k.OrganizationID == key.OrganizationID && k.Name == name {
			return nil, ErrSSHKeyNameExists
		}
	}

	key.Name = name
	return key, nil
}

// Delete deletes an SSH key
func (m *MockService) Delete(ctx context.Context, id int64) error {
	if m.deleteErr != nil {
		return m.deleteErr
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.keys[id]; !ok {
		return ErrSSHKeyNotFound
	}
	delete(m.keys, id)
	return nil
}

// GetPrivateKey returns the decrypted private key
func (m *MockService) GetPrivateKey(ctx context.Context, id int64) (string, error) {
	if m.getErr != nil {
		return "", m.getErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	key, ok := m.keys[id]
	if !ok {
		return "", ErrSSHKeyNotFound
	}
	return key.PrivateKeyEnc, nil
}

// ExistsInOrganization checks if an SSH key exists in an organization
func (m *MockService) ExistsInOrganization(ctx context.Context, id, orgID int64) (bool, error) {
	if m.getErr != nil {
		return false, m.getErr
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	key, ok := m.keys[id]
	if !ok {
		return false, nil
	}
	return key.OrganizationID == orgID, nil
}
