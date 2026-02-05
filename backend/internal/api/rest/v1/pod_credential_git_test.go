package v1

import (
	"errors"
	"testing"

	"github.com/anthropics/agentsmesh/backend/internal/domain/user"
	userService "github.com/anthropics/agentsmesh/backend/internal/service/user"
)

// ==================== getUserGitCredential Tests ====================

func TestGetUserGitCredential_NilUserService(t *testing.T) {
	h := &PodHandler{userService: nil}
	c, _ := createCredentialTestContext(1)

	result := h.getUserGitCredential(c, 1)
	if result != nil {
		t.Errorf("Expected nil when userService is nil, got %v", result)
	}
}

func TestGetUserGitCredential_NoDefaultCredential(t *testing.T) {
	mock := &mockUserService{
		getDefaultGitCredentialErr: errors.New("not found"),
	}
	h := &PodHandler{userService: mock}
	c, _ := createCredentialTestContext(1)

	result := h.getUserGitCredential(c, 1)
	if result != nil {
		t.Errorf("Expected nil when no default credential, got %v", result)
	}
}

func TestGetUserGitCredential_NilDefaultCredential(t *testing.T) {
	mock := &mockUserService{
		defaultGitCredential: nil,
	}
	h := &PodHandler{userService: mock}
	c, _ := createCredentialTestContext(1)

	result := h.getUserGitCredential(c, 1)
	if result != nil {
		t.Errorf("Expected nil when default credential is nil, got %v", result)
	}
}

func TestGetUserGitCredential_RunnerLocalType(t *testing.T) {
	mock := &mockUserService{
		defaultGitCredential: &user.GitCredential{
			ID:             1,
			UserID:         1,
			CredentialType: "runner_local",
			Name:           "Runner Local",
		},
	}
	h := &PodHandler{userService: mock}
	c, _ := createCredentialTestContext(1)

	result := h.getUserGitCredential(c, 1)
	if result != nil {
		t.Errorf("Expected nil for runner_local credential type, got %v", result)
	}
}

func TestGetUserGitCredential_DecryptError(t *testing.T) {
	mock := &mockUserService{
		defaultGitCredential: &user.GitCredential{
			ID:             1,
			UserID:         1,
			CredentialType: "pat",
			Name:           "My PAT",
		},
		getDecryptedCredentialErr: errors.New("decrypt failed"),
	}
	h := &PodHandler{userService: mock}
	c, _ := createCredentialTestContext(1)

	result := h.getUserGitCredential(c, 1)
	if result != nil {
		t.Errorf("Expected nil when decryption fails, got %v", result)
	}
}

func TestGetUserGitCredential_Success(t *testing.T) {
	expectedCred := &userService.DecryptedCredential{
		Type:  "pat",
		Token: "ghp_test123",
	}
	mock := &mockUserService{
		defaultGitCredential: &user.GitCredential{
			ID:             1,
			UserID:         1,
			CredentialType: "pat",
			Name:           "My PAT",
		},
		decryptedCredential: expectedCred,
	}
	h := &PodHandler{userService: mock}
	c, _ := createCredentialTestContext(1)

	result := h.getUserGitCredential(c, 1)
	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.Token != "ghp_test123" {
		t.Errorf("Expected token 'ghp_test123', got '%s'", result.Token)
	}
	if result.Type != "pat" {
		t.Errorf("Expected type 'pat', got '%s'", result.Type)
	}
}

// ==================== getUserGitToken Tests ====================

func TestGetUserGitToken_NilUserService(t *testing.T) {
	h := &PodHandler{userService: nil}
	c, _ := createCredentialTestContext(1)

	result := h.getUserGitToken(c, 1, "github", "https://github.com")
	if result != "" {
		t.Errorf("Expected empty string when userService is nil, got '%s'", result)
	}
}

func TestGetUserGitToken_ProviderTokenSuccess(t *testing.T) {
	mock := &mockUserService{
		providerToken: "gho_oauth123",
	}
	h := &PodHandler{userService: mock}
	c, _ := createCredentialTestContext(1)

	result := h.getUserGitToken(c, 1, "github", "https://github.com")
	if result != "gho_oauth123" {
		t.Errorf("Expected 'gho_oauth123', got '%s'", result)
	}
}

func TestGetUserGitToken_ProviderTokenError(t *testing.T) {
	mock := &mockUserService{
		getProviderTokenErr: errors.New("provider not found"),
	}
	h := &PodHandler{userService: mock}
	c, _ := createCredentialTestContext(1)

	result := h.getUserGitToken(c, 1, "github", "https://github.com")
	if result != "" {
		t.Errorf("Expected empty string when provider not found, got '%s'", result)
	}
}

func TestGetUserGitToken_EmptyToken(t *testing.T) {
	mock := &mockUserService{
		providerToken: "",
	}
	h := &PodHandler{userService: mock}
	c, _ := createCredentialTestContext(1)

	result := h.getUserGitToken(c, 1, "gitlab", "https://gitlab.company.com")
	if result != "" {
		t.Errorf("Expected empty string for empty token, got '%s'", result)
	}
}

func TestGetUserGitToken_PrivateGitLab(t *testing.T) {
	mock := &mockUserService{
		providerToken: "glpat_private123",
	}
	h := &PodHandler{userService: mock}
	c, _ := createCredentialTestContext(1)

	result := h.getUserGitToken(c, 1, "gitlab", "https://gitlab.company.com")
	if result != "glpat_private123" {
		t.Errorf("Expected 'glpat_private123', got '%s'", result)
	}
}

func TestGetUserGitToken_Gitee(t *testing.T) {
	mock := &mockUserService{
		providerToken: "gitee_token_abc",
	}
	h := &PodHandler{userService: mock}
	c, _ := createCredentialTestContext(1)

	result := h.getUserGitToken(c, 1, "gitee", "https://gitee.com")
	if result != "gitee_token_abc" {
		t.Errorf("Expected 'gitee_token_abc', got '%s'", result)
	}
}
