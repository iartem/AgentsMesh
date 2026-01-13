package v1

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropics/agentmesh/backend/internal/middleware"
	"github.com/anthropics/agentmesh/backend/internal/service/runner"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupRunnerHandlerTest(t *testing.T) (*RunnerHandler, *gin.Engine, *gorm.DB) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("Failed to connect database: %v", err)
	}

	// Create tables
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS runners (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			node_id TEXT NOT NULL,
			description TEXT,
			auth_token_hash TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'offline',
			last_heartbeat DATETIME,
			current_pods INTEGER NOT NULL DEFAULT 0,
			max_concurrent_pods INTEGER NOT NULL DEFAULT 5,
			runner_version TEXT,
			host_info TEXT,
			is_enabled INTEGER NOT NULL DEFAULT 1,
			capabilities TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatalf("Failed to create runners table: %v", err)
	}

	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS runner_registration_tokens (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			token_hash TEXT NOT NULL UNIQUE,
			description TEXT,
			created_by_id INTEGER NOT NULL,
			is_active INTEGER NOT NULL DEFAULT 1,
			max_uses INTEGER,
			used_count INTEGER NOT NULL DEFAULT 0,
			expires_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatalf("Failed to create runner_registration_tokens table: %v", err)
	}

	runnerSvc := runner.NewService(db)
	handler := NewRunnerHandler(runnerSvc)

	router := gin.New()
	return handler, router, db
}

// setTenantContext sets tenant context for testing
func setTenantContext(c *gin.Context, orgID, userID int64, role string) {
	c.Set("tenant", &middleware.TenantContext{
		OrganizationID:   orgID,
		OrganizationSlug: "test-org",
		UserID:           userID,
		UserRole:         role,
	})
}

func TestCreateRegistrationToken_EmptyBody(t *testing.T) {
	handler, router, _ := setupRunnerHandlerTest(t)

	router.POST("/orgs/:slug/runners/tokens", func(c *gin.Context) {
		setTenantContext(c, 1, 1, "owner")
		handler.CreateRegistrationToken(c)
	})

	// Test with completely empty body (no Content-Type, no body)
	t.Run("empty body without content-type", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgs/test-org/runners/tokens", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return 201 Created, not 400 Bad Request
		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		if _, ok := resp["token"]; !ok {
			t.Error("Response should contain 'token' field")
		}
	})

	// Test with empty JSON body
	t.Run("empty JSON body", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/orgs/test-org/runners/tokens", bytes.NewBufferString("{}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	// Test with partial body (only description)
	t.Run("partial body with description only", func(t *testing.T) {
		body := `{"description": "Test token"}`
		req := httptest.NewRequest(http.MethodPost, "/orgs/test-org/runners/tokens", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	// Test with full body
	t.Run("full body with all fields", func(t *testing.T) {
		body := `{"description": "Full token", "max_uses": 5}`
		req := httptest.NewRequest(http.MethodPost, "/orgs/test-org/runners/tokens", bytes.NewBufferString(body))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

func TestCreateRegistrationToken_PermissionDenied(t *testing.T) {
	handler, router, _ := setupRunnerHandlerTest(t)

	router.POST("/orgs/:slug/runners/tokens", func(c *gin.Context) {
		setTenantContext(c, 1, 1, "member") // member role, not admin/owner
		handler.CreateRegistrationToken(c)
	})

	req := httptest.NewRequest(http.MethodPost, "/orgs/test-org/runners/tokens", bytes.NewBufferString("{}"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	// Should return 403 Forbidden for non-admin users
	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d. Body: %s", w.Code, w.Body.String())
	}
}

func TestCreateRegistrationToken_AdminAllowed(t *testing.T) {
	handler, _, _ := setupRunnerHandlerTest(t)

	// Test admin role
	t.Run("admin role allowed", func(t *testing.T) {
		router := gin.New()
		router.POST("/orgs/:slug/runners/tokens", func(c *gin.Context) {
			setTenantContext(c, 1, 1, "admin")
			handler.CreateRegistrationToken(c)
		})

		req := httptest.NewRequest(http.MethodPost, "/orgs/test-org/runners/tokens", bytes.NewBufferString("{}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201 for admin, got %d. Body: %s", w.Code, w.Body.String())
		}
	})

	// Test owner role
	t.Run("owner role allowed", func(t *testing.T) {
		router := gin.New()
		router.POST("/orgs/:slug/runners/tokens", func(c *gin.Context) {
			setTenantContext(c, 1, 1, "owner")
			handler.CreateRegistrationToken(c)
		})

		req := httptest.NewRequest(http.MethodPost, "/orgs/test-org/runners/tokens", bytes.NewBufferString("{}"))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201 for owner, got %d. Body: %s", w.Code, w.Body.String())
		}
	})
}

func TestListRunners(t *testing.T) {
	handler, router, _ := setupRunnerHandlerTest(t)

	router.GET("/orgs/:slug/runners", func(c *gin.Context) {
		setTenantContext(c, 1, 1, "member")
		handler.ListRunners(c)
	})

	req := httptest.NewRequest(http.MethodGet, "/orgs/test-org/runners", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d. Body: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if _, ok := resp["runners"]; !ok {
		t.Error("Response should contain 'runners' field")
	}
}
