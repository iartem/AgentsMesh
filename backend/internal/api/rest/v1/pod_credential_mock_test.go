package v1

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/anthropics/agentsmesh/backend/internal/domain/user"
	userService "github.com/anthropics/agentsmesh/backend/internal/service/user"
	"github.com/gin-gonic/gin"
)

// ==================== Mock UserService for Testing ====================

// mockUserService implements UserServiceForPod for testing
type mockUserService struct {
	// Mock data
	defaultGitCredential *user.GitCredential
	decryptedCredential  *userService.DecryptedCredential
	providerToken        string

	// Error returns
	getDefaultGitCredentialErr error
	getDecryptedCredentialErr  error
	getProviderTokenErr        error
}

func (m *mockUserService) GetDefaultGitCredential(ctx context.Context, userID int64) (*user.GitCredential, error) {
	if m.getDefaultGitCredentialErr != nil {
		return nil, m.getDefaultGitCredentialErr
	}
	return m.defaultGitCredential, nil
}

func (m *mockUserService) GetDecryptedCredentialToken(ctx context.Context, userID, credentialID int64) (*userService.DecryptedCredential, error) {
	if m.getDecryptedCredentialErr != nil {
		return nil, m.getDecryptedCredentialErr
	}
	return m.decryptedCredential, nil
}

func (m *mockUserService) GetDecryptedProviderTokenByTypeAndURL(ctx context.Context, userID int64, providerType, baseURL string) (string, error) {
	if m.getProviderTokenErr != nil {
		return "", m.getProviderTokenErr
	}
	return m.providerToken, nil
}

// Ensure mockUserService implements UserServiceForPod
var _ UserServiceForPod = (*mockUserService)(nil)

// ==================== Test Helpers ====================

// createCredentialTestContext creates a gin context with user ID set
func createCredentialTestContext(userID int64) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	c.Request = c.Request.WithContext(context.Background())

	// Set user ID in context
	c.Set("user_id", userID)

	return c, w
}
