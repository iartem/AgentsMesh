package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/anthropics/agentmesh/backend/internal/domain/sshkey"
	"github.com/anthropics/agentmesh/backend/internal/middleware"
	sshkeyService "github.com/anthropics/agentmesh/backend/internal/service/sshkey"
	"github.com/gin-gonic/gin"
)

func setupSSHKeyHandlerTest() (*SSHKeyHandler, *sshkeyService.MockService, *gin.Engine) {
	mockSvc := sshkeyService.NewMockService()
	handler := NewSSHKeyHandler(mockSvc)

	router := gin.New()
	return handler, mockSvc, router
}

func setTenantContext(c *gin.Context, orgID int64, userID int64) {
	tc := &middleware.TenantContext{
		OrganizationID:   orgID,
		OrganizationSlug: "test-org",
		UserID:           userID,
		UserRole:         "owner",
	}
	c.Set("tenant", tc)
}

func TestNewSSHKeyHandler(t *testing.T) {
	handler, _, _ := setupSSHKeyHandlerTest()
	if handler == nil {
		t.Fatal("expected non-nil handler")
	}
}

func TestListSSHKeys(t *testing.T) {
	handler, mockSvc, router := setupSSHKeyHandlerTest()

	// Add test keys
	mockSvc.AddKey(&sshkey.SSHKey{
		ID:             1,
		OrganizationID: 1,
		Name:           "key-1",
		PublicKey:      "ssh-rsa AAAA...",
		Fingerprint:    "SHA256:abc123",
		CreatedAt:      time.Now(),
	})
	mockSvc.AddKey(&sshkey.SSHKey{
		ID:             2,
		OrganizationID: 1,
		Name:           "key-2",
		PublicKey:      "ssh-rsa BBBB...",
		Fingerprint:    "SHA256:def456",
		CreatedAt:      time.Now(),
	})

	router.GET("/ssh-keys", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.ListSSHKeys(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/ssh-keys", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	keys := resp["ssh_keys"].([]interface{})
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

func TestListSSHKeysEmpty(t *testing.T) {
	handler, _, router := setupSSHKeyHandlerTest()

	router.GET("/ssh-keys", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.ListSSHKeys(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/ssh-keys", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Response should contain ssh_keys key
	if _, exists := resp["ssh_keys"]; !exists {
		t.Error("expected ssh_keys key in response")
	}
}

func TestListSSHKeysError(t *testing.T) {
	handler, mockSvc, router := setupSSHKeyHandlerTest()
	mockSvc.SetListErr(sshkeyService.ErrSSHKeyNotFound)

	router.GET("/ssh-keys", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.ListSSHKeys(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/ssh-keys", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestCreateSSHKey(t *testing.T) {
	handler, _, router := setupSSHKeyHandlerTest()

	router.POST("/ssh-keys", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.CreateSSHKey(c)
	})

	body := `{"name": "my-new-key"}`
	req := httptest.NewRequest(http.MethodPost, "/ssh-keys", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	sshKey := resp["ssh_key"].(map[string]interface{})
	if sshKey["name"] != "my-new-key" {
		t.Errorf("expected name 'my-new-key', got %v", sshKey["name"])
	}
}

func TestCreateSSHKeyWithPrivateKey(t *testing.T) {
	handler, _, router := setupSSHKeyHandlerTest()

	router.POST("/ssh-keys", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.CreateSSHKey(c)
	})

	// Using an invalid private key for this test - the mock will validate it
	body := `{"name": "imported-key", "private_key": "invalid-key"}`
	req := httptest.NewRequest(http.MethodPost, "/ssh-keys", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should fail with bad request due to invalid key
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateSSHKeyInvalidJSON(t *testing.T) {
	handler, _, router := setupSSHKeyHandlerTest()

	router.POST("/ssh-keys", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.CreateSSHKey(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/ssh-keys", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateSSHKeyNameTooShort(t *testing.T) {
	handler, _, router := setupSSHKeyHandlerTest()

	router.POST("/ssh-keys", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.CreateSSHKey(c)
	})

	body := `{"name": "a"}` // Too short (min 2)
	req := httptest.NewRequest(http.MethodPost, "/ssh-keys", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateSSHKeyDuplicateName(t *testing.T) {
	handler, mockSvc, router := setupSSHKeyHandlerTest()

	// Add existing key
	mockSvc.AddKey(&sshkey.SSHKey{
		ID:             1,
		OrganizationID: 1,
		Name:           "existing-key",
		PublicKey:      "ssh-rsa AAAA...",
		Fingerprint:    "SHA256:abc123",
	})

	router.POST("/ssh-keys", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.CreateSSHKey(c)
	})

	body := `{"name": "existing-key"}`
	req := httptest.NewRequest(http.MethodPost, "/ssh-keys", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSSHKey(t *testing.T) {
	handler, mockSvc, router := setupSSHKeyHandlerTest()

	mockSvc.AddKey(&sshkey.SSHKey{
		ID:             1,
		OrganizationID: 1,
		Name:           "test-key",
		PublicKey:      "ssh-rsa AAAA...",
		Fingerprint:    "SHA256:abc123",
	})

	router.GET("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.GetSSHKey(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/ssh-keys/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	sshKey := resp["ssh_key"].(map[string]interface{})
	if sshKey["name"] != "test-key" {
		t.Errorf("expected name 'test-key', got %v", sshKey["name"])
	}
}

func TestGetSSHKeyInvalidID(t *testing.T) {
	handler, _, router := setupSSHKeyHandlerTest()

	router.GET("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.GetSSHKey(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/ssh-keys/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestGetSSHKeyNotFound(t *testing.T) {
	handler, _, router := setupSSHKeyHandlerTest()

	router.GET("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.GetSSHKey(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/ssh-keys/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetSSHKeyWrongOrganization(t *testing.T) {
	handler, mockSvc, router := setupSSHKeyHandlerTest()

	// Key belongs to org 2
	mockSvc.AddKey(&sshkey.SSHKey{
		ID:             1,
		OrganizationID: 2,
		Name:           "other-org-key",
		PublicKey:      "ssh-rsa AAAA...",
		Fingerprint:    "SHA256:abc123",
	})

	router.GET("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1) // User is in org 1
		handler.GetSSHKey(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/ssh-keys/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestUpdateSSHKey(t *testing.T) {
	handler, mockSvc, router := setupSSHKeyHandlerTest()

	mockSvc.AddKey(&sshkey.SSHKey{
		ID:             1,
		OrganizationID: 1,
		Name:           "old-name",
		PublicKey:      "ssh-rsa AAAA...",
		Fingerprint:    "SHA256:abc123",
	})

	router.PUT("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.UpdateSSHKey(c)
	})

	body := `{"name": "new-name"}`
	req := httptest.NewRequest(http.MethodPut, "/ssh-keys/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	sshKey := resp["ssh_key"].(map[string]interface{})
	if sshKey["name"] != "new-name" {
		t.Errorf("expected name 'new-name', got %v", sshKey["name"])
	}
}

func TestUpdateSSHKeyInvalidID(t *testing.T) {
	handler, _, router := setupSSHKeyHandlerTest()

	router.PUT("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.UpdateSSHKey(c)
	})

	body := `{"name": "new-name"}`
	req := httptest.NewRequest(http.MethodPut, "/ssh-keys/invalid", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateSSHKeyInvalidJSON(t *testing.T) {
	handler, mockSvc, router := setupSSHKeyHandlerTest()

	mockSvc.AddKey(&sshkey.SSHKey{
		ID:             1,
		OrganizationID: 1,
		Name:           "old-name",
		PublicKey:      "ssh-rsa AAAA...",
		Fingerprint:    "SHA256:abc123",
	})

	router.PUT("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.UpdateSSHKey(c)
	})

	req := httptest.NewRequest(http.MethodPut, "/ssh-keys/1", bytes.NewBufferString("invalid"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateSSHKeyNotFound(t *testing.T) {
	handler, _, router := setupSSHKeyHandlerTest()

	router.PUT("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.UpdateSSHKey(c)
	})

	body := `{"name": "new-name"}`
	req := httptest.NewRequest(http.MethodPut, "/ssh-keys/999", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestUpdateSSHKeyDuplicateName(t *testing.T) {
	handler, mockSvc, router := setupSSHKeyHandlerTest()

	mockSvc.AddKey(&sshkey.SSHKey{
		ID:             1,
		OrganizationID: 1,
		Name:           "key-1",
		PublicKey:      "ssh-rsa AAAA...",
		Fingerprint:    "SHA256:abc123",
	})
	mockSvc.AddKey(&sshkey.SSHKey{
		ID:             2,
		OrganizationID: 1,
		Name:           "key-2",
		PublicKey:      "ssh-rsa BBBB...",
		Fingerprint:    "SHA256:def456",
	})

	router.PUT("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.UpdateSSHKey(c)
	})

	body := `{"name": "key-2"}` // Try to rename key-1 to key-2 (conflict)
	req := httptest.NewRequest(http.MethodPut, "/ssh-keys/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteSSHKey(t *testing.T) {
	handler, mockSvc, router := setupSSHKeyHandlerTest()

	mockSvc.AddKey(&sshkey.SSHKey{
		ID:             1,
		OrganizationID: 1,
		Name:           "to-delete",
		PublicKey:      "ssh-rsa AAAA...",
		Fingerprint:    "SHA256:abc123",
	})

	router.DELETE("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.DeleteSSHKey(c)
	})

	req := httptest.NewRequest(http.MethodDelete, "/ssh-keys/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["message"] != "SSH key deleted" {
		t.Errorf("expected message 'SSH key deleted', got %v", resp["message"])
	}
}

func TestDeleteSSHKeyInvalidID(t *testing.T) {
	handler, _, router := setupSSHKeyHandlerTest()

	router.DELETE("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.DeleteSSHKey(c)
	})

	req := httptest.NewRequest(http.MethodDelete, "/ssh-keys/invalid", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestDeleteSSHKeyNotFound(t *testing.T) {
	handler, _, router := setupSSHKeyHandlerTest()

	router.DELETE("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.DeleteSSHKey(c)
	})

	req := httptest.NewRequest(http.MethodDelete, "/ssh-keys/999", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteSSHKeyWrongOrganization(t *testing.T) {
	handler, mockSvc, router := setupSSHKeyHandlerTest()

	// Key belongs to org 2
	mockSvc.AddKey(&sshkey.SSHKey{
		ID:             1,
		OrganizationID: 2,
		Name:           "other-org-key",
		PublicKey:      "ssh-rsa AAAA...",
		Fingerprint:    "SHA256:abc123",
	})

	router.DELETE("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1) // User is in org 1
		handler.DeleteSSHKey(c)
	})

	req := httptest.NewRequest(http.MethodDelete, "/ssh-keys/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCreateSSHKeyServiceError(t *testing.T) {
	handler, mockSvc, router := setupSSHKeyHandlerTest()
	mockSvc.SetCreateErr(sshkeyService.ErrSSHKeyNotFound) // Generic service error

	router.POST("/ssh-keys", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.CreateSSHKey(c)
	})

	body := `{"name": "my-new-key"}`
	req := httptest.NewRequest(http.MethodPost, "/ssh-keys", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetSSHKeyServiceError(t *testing.T) {
	handler, mockSvc, router := setupSSHKeyHandlerTest()

	// Add key first so it exists
	mockSvc.AddKey(&sshkey.SSHKey{
		ID:             1,
		OrganizationID: 1,
		Name:           "test-key",
		PublicKey:      "ssh-rsa AAAA...",
		Fingerprint:    "SHA256:abc123",
	})

	// Now set error for subsequent calls
	mockSvc.SetGetErr(sshkeyService.ErrSSHKeyNotFound)

	router.GET("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.GetSSHKey(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/ssh-keys/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 404 since ErrSSHKeyNotFound is set
	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateSSHKeyServiceError(t *testing.T) {
	handler, mockSvc, router := setupSSHKeyHandlerTest()

	mockSvc.AddKey(&sshkey.SSHKey{
		ID:             1,
		OrganizationID: 1,
		Name:           "old-name",
		PublicKey:      "ssh-rsa AAAA...",
		Fingerprint:    "SHA256:abc123",
	})

	// Set error after the initial GetByIDAndOrg succeeds
	mockSvc.SetUpdateErr(sshkeyService.ErrSSHKeyNotFound)

	router.PUT("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.UpdateSSHKey(c)
	})

	body := `{"name": "new-name"}`
	req := httptest.NewRequest(http.MethodPut, "/ssh-keys/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteSSHKeyServiceError(t *testing.T) {
	handler, mockSvc, router := setupSSHKeyHandlerTest()

	mockSvc.AddKey(&sshkey.SSHKey{
		ID:             1,
		OrganizationID: 1,
		Name:           "to-delete",
		PublicKey:      "ssh-rsa AAAA...",
		Fingerprint:    "SHA256:abc123",
	})

	mockSvc.SetDeleteErr(sshkeyService.ErrSSHKeyNotFound)

	router.DELETE("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1)
		handler.DeleteSSHKey(c)
	})

	req := httptest.NewRequest(http.MethodDelete, "/ssh-keys/1", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

func TestUpdateSSHKeyWrongOrganization(t *testing.T) {
	handler, mockSvc, router := setupSSHKeyHandlerTest()

	// Key belongs to org 2
	mockSvc.AddKey(&sshkey.SSHKey{
		ID:             1,
		OrganizationID: 2,
		Name:           "other-org-key",
		PublicKey:      "ssh-rsa AAAA...",
		Fingerprint:    "SHA256:abc123",
	})

	router.PUT("/ssh-keys/:id", func(c *gin.Context) {
		setTenantContext(c, 1, 1) // User is in org 1
		handler.UpdateSSHKey(c)
	})

	body := `{"name": "new-name"}`
	req := httptest.NewRequest(http.MethodPut, "/ssh-keys/1", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}
