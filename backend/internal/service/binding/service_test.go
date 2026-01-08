package binding

import (
	"context"
	"testing"

	"github.com/anthropics/agentmesh/backend/internal/domain/channel"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// MockSessionQuerier implements SessionQuerier for testing
type MockSessionQuerier struct {
	sessions map[string]map[string]interface{}
	err      error
}

func NewMockSessionQuerier() *MockSessionQuerier {
	return &MockSessionQuerier{
		sessions: make(map[string]map[string]interface{}),
	}
}

func (m *MockSessionQuerier) AddSession(key string, info map[string]interface{}) {
	m.sessions[key] = info
}

func (m *MockSessionQuerier) GetSessionInfo(ctx context.Context, sessionKey string) (map[string]interface{}, error) {
	if m.err != nil {
		return nil, m.err
	}
	if info, ok := m.sessions[sessionKey]; ok {
		return info, nil
	}
	return nil, nil
}

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	// Create session_bindings table
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS session_bindings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			initiator_session TEXT NOT NULL,
			target_session TEXT NOT NULL,
			granted_scopes TEXT,
			pending_scopes TEXT,
			status TEXT NOT NULL DEFAULT 'pending',
			requested_at DATETIME,
			responded_at DATETIME,
			expires_at DATETIME,
			rejection_reason TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatalf("failed to create session_bindings table: %v", err)
	}

	return db
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)
	querier := NewMockSessionQuerier()
	service := NewService(db, querier)

	if service == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestNewServiceWithoutQuerier(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)

	if service == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestValidateScopes(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)

	t.Run("valid scopes", func(t *testing.T) {
		err := service.validateScopes([]string{channel.BindingScopeTerminalRead})
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("valid multiple scopes", func(t *testing.T) {
		err := service.validateScopes([]string{channel.BindingScopeTerminalRead, channel.BindingScopeTerminalWrite})
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
	})

	t.Run("invalid scope", func(t *testing.T) {
		err := service.validateScopes([]string{"invalid:scope"})
		if err != ErrInvalidScope {
			t.Errorf("expected ErrInvalidScope, got %v", err)
		}
	})
}

func TestRequestBinding(t *testing.T) {
	db := setupTestDB(t)
	querier := NewMockSessionQuerier()
	service := NewService(db, querier)
	ctx := context.Background()

	t.Run("creates pending binding", func(t *testing.T) {
		binding, err := service.RequestBinding(ctx, 1, "session-1", "session-2",
			[]string{channel.BindingScopeTerminalRead}, "")
		if err != nil {
			t.Fatalf("failed to request binding: %v", err)
		}
		if binding.Status != channel.BindingStatusPending {
			t.Errorf("expected status pending, got %s", binding.Status)
		}
		if binding.InitiatorSession != "session-1" {
			t.Errorf("expected initiator session-1, got %s", binding.InitiatorSession)
		}
	})

	t.Run("self-binding returns error", func(t *testing.T) {
		_, err := service.RequestBinding(ctx, 1, "session-1", "session-1",
			[]string{channel.BindingScopeTerminalRead}, "")
		if err != ErrSelfBinding {
			t.Errorf("expected ErrSelfBinding, got %v", err)
		}
	})

	t.Run("invalid scope returns error", func(t *testing.T) {
		_, err := service.RequestBinding(ctx, 1, "session-a", "session-b",
			[]string{"invalid:scope"}, "")
		if err != ErrInvalidScope {
			t.Errorf("expected ErrInvalidScope, got %v", err)
		}
	})

	t.Run("same user auto approves", func(t *testing.T) {
		querier.AddSession("user-session-1", map[string]interface{}{"user_id": int64(1)})
		querier.AddSession("user-session-2", map[string]interface{}{"user_id": int64(1)})

		binding, err := service.RequestBinding(ctx, 1, "user-session-1", "user-session-2",
			[]string{channel.BindingScopeTerminalRead}, "")
		if err != nil {
			t.Fatalf("failed to request binding: %v", err)
		}
		if binding.Status != channel.BindingStatusActive {
			t.Errorf("expected status active for same user, got %s", binding.Status)
		}
	})
}

func TestCreateAutoBinding(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()

	t.Run("creates active binding", func(t *testing.T) {
		binding, err := service.CreateAutoBinding(ctx, 1, "auto-1", "auto-2",
			[]string{channel.BindingScopeTerminalRead, channel.BindingScopeTerminalWrite})
		if err != nil {
			t.Fatalf("failed to create auto binding: %v", err)
		}
		if binding.Status != channel.BindingStatusActive {
			t.Errorf("expected status active, got %s", binding.Status)
		}
		if len(binding.GrantedScopes) != 2 {
			t.Errorf("expected 2 granted scopes, got %d", len(binding.GrantedScopes))
		}
	})

	t.Run("self-binding returns error", func(t *testing.T) {
		_, err := service.CreateAutoBinding(ctx, 1, "auto-same", "auto-same",
			[]string{channel.BindingScopeTerminalRead})
		if err != ErrSelfBinding {
			t.Errorf("expected ErrSelfBinding, got %v", err)
		}
	})

	t.Run("returns existing binding", func(t *testing.T) {
		binding1, _ := service.CreateAutoBinding(ctx, 1, "exist-1", "exist-2",
			[]string{channel.BindingScopeTerminalRead})
		binding2, err := service.CreateAutoBinding(ctx, 1, "exist-1", "exist-2",
			[]string{channel.BindingScopeTerminalWrite})
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if binding2.ID != binding1.ID {
			t.Error("expected same binding to be returned")
		}
	})
}

func TestGetBinding(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()

	t.Run("returns binding by ID", func(t *testing.T) {
		created, _ := service.CreateAutoBinding(ctx, 1, "get-1", "get-2",
			[]string{channel.BindingScopeTerminalRead})

		binding, err := service.GetBinding(ctx, created.ID)
		if err != nil {
			t.Fatalf("failed to get binding: %v", err)
		}
		if binding.ID != created.ID {
			t.Errorf("expected ID %d, got %d", created.ID, binding.ID)
		}
	})

	t.Run("returns error for non-existent binding", func(t *testing.T) {
		_, err := service.GetBinding(ctx, 99999)
		if err != ErrBindingNotFound {
			t.Errorf("expected ErrBindingNotFound, got %v", err)
		}
	})
}

func TestGetActiveBinding(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()

	t.Run("returns active binding", func(t *testing.T) {
		service.CreateAutoBinding(ctx, 1, "active-1", "active-2",
			[]string{channel.BindingScopeTerminalRead})

		binding, err := service.GetActiveBinding(ctx, "active-1", "active-2")
		if err != nil {
			t.Fatalf("failed to get active binding: %v", err)
		}
		if !binding.IsActive() {
			t.Error("expected binding to be active")
		}
	})

	t.Run("returns error for pending binding", func(t *testing.T) {
		service.RequestBinding(ctx, 1, "pending-1", "pending-2",
			[]string{channel.BindingScopeTerminalRead}, channel.BindingPolicyExplicitOnly)

		_, err := service.GetActiveBinding(ctx, "pending-1", "pending-2")
		if err != ErrBindingNotFound {
			t.Errorf("expected ErrBindingNotFound for pending binding, got %v", err)
		}
	})
}

func TestAcceptBinding(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()

	t.Run("accepts pending binding", func(t *testing.T) {
		pending, _ := service.RequestBinding(ctx, 1, "accept-1", "accept-2",
			[]string{channel.BindingScopeTerminalRead}, channel.BindingPolicyExplicitOnly)

		accepted, err := service.AcceptBinding(ctx, pending.ID, "accept-2")
		if err != nil {
			t.Fatalf("failed to accept binding: %v", err)
		}
		if accepted.Status != channel.BindingStatusActive {
			t.Errorf("expected status active, got %s", accepted.Status)
		}
		if len(accepted.GrantedScopes) != 1 {
			t.Errorf("expected 1 granted scope, got %d", len(accepted.GrantedScopes))
		}
	})

	t.Run("wrong session returns error", func(t *testing.T) {
		pending, _ := service.RequestBinding(ctx, 1, "wrong-1", "wrong-2",
			[]string{channel.BindingScopeTerminalRead}, channel.BindingPolicyExplicitOnly)

		_, err := service.AcceptBinding(ctx, pending.ID, "wrong-1") // Should be wrong-2
		if err != ErrNotAuthorized {
			t.Errorf("expected ErrNotAuthorized, got %v", err)
		}
	})

	t.Run("accepting non-pending returns error", func(t *testing.T) {
		active, _ := service.CreateAutoBinding(ctx, 1, "not-pending-1", "not-pending-2",
			[]string{channel.BindingScopeTerminalRead})

		_, err := service.AcceptBinding(ctx, active.ID, "not-pending-2")
		if err != ErrBindingNotPending {
			t.Errorf("expected ErrBindingNotPending, got %v", err)
		}
	})
}

func TestRejectBinding(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()

	t.Run("rejects pending binding", func(t *testing.T) {
		pending, _ := service.RequestBinding(ctx, 1, "reject-1", "reject-2",
			[]string{channel.BindingScopeTerminalRead}, channel.BindingPolicyExplicitOnly)

		rejected, err := service.RejectBinding(ctx, pending.ID, "reject-2", "not interested")
		if err != nil {
			t.Fatalf("failed to reject binding: %v", err)
		}
		if rejected.Status != channel.BindingStatusRejected {
			t.Errorf("expected status rejected, got %s", rejected.Status)
		}
		if rejected.RejectionReason == nil || *rejected.RejectionReason != "not interested" {
			t.Error("expected rejection reason to be set")
		}
	})

	t.Run("wrong session returns error", func(t *testing.T) {
		pending, _ := service.RequestBinding(ctx, 1, "reject-wrong-1", "reject-wrong-2",
			[]string{channel.BindingScopeTerminalRead}, channel.BindingPolicyExplicitOnly)

		_, err := service.RejectBinding(ctx, pending.ID, "reject-wrong-1", "")
		if err != ErrNotAuthorized {
			t.Errorf("expected ErrNotAuthorized, got %v", err)
		}
	})
}

func TestUnbind(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()

	t.Run("unbinds active binding", func(t *testing.T) {
		service.CreateAutoBinding(ctx, 1, "unbind-1", "unbind-2",
			[]string{channel.BindingScopeTerminalRead})

		success, err := service.Unbind(ctx, "unbind-1", "unbind-2")
		if err != nil {
			t.Fatalf("failed to unbind: %v", err)
		}
		if !success {
			t.Error("expected unbind to succeed")
		}

		// Verify it's no longer active
		_, err = service.GetActiveBinding(ctx, "unbind-1", "unbind-2")
		if err != ErrBindingNotFound {
			t.Error("expected binding to be inactive")
		}
	})

	t.Run("unbinds in reverse direction", func(t *testing.T) {
		service.CreateAutoBinding(ctx, 1, "unbind-rev-1", "unbind-rev-2",
			[]string{channel.BindingScopeTerminalRead})

		success, err := service.Unbind(ctx, "unbind-rev-2", "unbind-rev-1")
		if err != nil {
			t.Fatalf("failed to unbind: %v", err)
		}
		if !success {
			t.Error("expected unbind to succeed")
		}
	})

	t.Run("returns false for non-existent binding", func(t *testing.T) {
		success, err := service.Unbind(ctx, "nonexistent-1", "nonexistent-2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if success {
			t.Error("expected unbind to return false for non-existent binding")
		}
	})
}

func TestIsBound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()

	t.Run("returns true for bound sessions", func(t *testing.T) {
		service.CreateAutoBinding(ctx, 1, "bound-1", "bound-2",
			[]string{channel.BindingScopeTerminalRead})

		bound, err := service.IsBound(ctx, "bound-1", "bound-2")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if !bound {
			t.Error("expected sessions to be bound")
		}
	})

	t.Run("returns true in reverse direction", func(t *testing.T) {
		service.CreateAutoBinding(ctx, 1, "bound-rev-1", "bound-rev-2",
			[]string{channel.BindingScopeTerminalRead})

		bound, err := service.IsBound(ctx, "bound-rev-2", "bound-rev-1")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if !bound {
			t.Error("expected sessions to be bound in reverse")
		}
	})

	t.Run("returns false for unbound sessions", func(t *testing.T) {
		bound, err := service.IsBound(ctx, "unbound-1", "unbound-2")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if bound {
			t.Error("expected sessions to not be bound")
		}
	})
}

func TestGetBindingsForSession(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()

	t.Run("returns all bindings for session", func(t *testing.T) {
		service.CreateAutoBinding(ctx, 1, "list-main", "list-1",
			[]string{channel.BindingScopeTerminalRead})
		service.CreateAutoBinding(ctx, 1, "list-main", "list-2",
			[]string{channel.BindingScopeTerminalRead})

		bindings, err := service.GetBindingsForSession(ctx, "list-main", nil)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if len(bindings) != 2 {
			t.Errorf("expected 2 bindings, got %d", len(bindings))
		}
	})

	t.Run("filters by status", func(t *testing.T) {
		service.CreateAutoBinding(ctx, 1, "filter-main", "filter-1",
			[]string{channel.BindingScopeTerminalRead})
		service.RequestBinding(ctx, 1, "filter-main", "filter-2",
			[]string{channel.BindingScopeTerminalRead}, channel.BindingPolicyExplicitOnly)

		activeStatus := channel.BindingStatusActive
		bindings, err := service.GetBindingsForSession(ctx, "filter-main", &activeStatus)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if len(bindings) != 1 {
			t.Errorf("expected 1 active binding, got %d", len(bindings))
		}
	})
}

func TestGetBoundSessions(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()

	t.Run("returns bound session keys", func(t *testing.T) {
		service.CreateAutoBinding(ctx, 1, "hub", "spoke-1",
			[]string{channel.BindingScopeTerminalRead})
		service.CreateAutoBinding(ctx, 1, "hub", "spoke-2",
			[]string{channel.BindingScopeTerminalRead})

		sessions, err := service.GetBoundSessions(ctx, "hub")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if len(sessions) != 2 {
			t.Errorf("expected 2 bound sessions, got %d", len(sessions))
		}
	})

	t.Run("returns empty for unbound session", func(t *testing.T) {
		sessions, err := service.GetBoundSessions(ctx, "isolated")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if len(sessions) != 0 {
			t.Errorf("expected 0 bound sessions, got %d", len(sessions))
		}
	})
}

func TestGetPendingRequests(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()

	t.Run("returns pending requests for target", func(t *testing.T) {
		service.RequestBinding(ctx, 1, "req-1", "target",
			[]string{channel.BindingScopeTerminalRead}, channel.BindingPolicyExplicitOnly)
		service.RequestBinding(ctx, 1, "req-2", "target",
			[]string{channel.BindingScopeTerminalRead}, channel.BindingPolicyExplicitOnly)

		pending, err := service.GetPendingRequests(ctx, "target")
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if len(pending) != 2 {
			t.Errorf("expected 2 pending requests, got %d", len(pending))
		}
	})
}

func TestRequestScopes(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()

	t.Run("requests additional scopes", func(t *testing.T) {
		binding, _ := service.CreateAutoBinding(ctx, 1, "scope-req-1", "scope-req-2",
			[]string{channel.BindingScopeTerminalRead})

		updated, err := service.RequestScopes(ctx, binding.ID, "scope-req-1",
			[]string{channel.BindingScopeTerminalWrite})
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		// Since same user is not set, scope should be pending
		if len(updated.PendingScopes) != 1 {
			t.Errorf("expected 1 pending scope, got %d", len(updated.PendingScopes))
		}
	})

	t.Run("wrong session returns error", func(t *testing.T) {
		binding, _ := service.CreateAutoBinding(ctx, 1, "scope-wrong-1", "scope-wrong-2",
			[]string{channel.BindingScopeTerminalRead})

		_, err := service.RequestScopes(ctx, binding.ID, "scope-wrong-2",
			[]string{channel.BindingScopeTerminalWrite})
		if err != ErrNotAuthorized {
			t.Errorf("expected ErrNotAuthorized, got %v", err)
		}
	})

	t.Run("invalid scope returns error", func(t *testing.T) {
		binding, _ := service.CreateAutoBinding(ctx, 1, "scope-inv-1", "scope-inv-2",
			[]string{channel.BindingScopeTerminalRead})

		_, err := service.RequestScopes(ctx, binding.ID, "scope-inv-1",
			[]string{"invalid:scope"})
		if err != ErrInvalidScope {
			t.Errorf("expected ErrInvalidScope, got %v", err)
		}
	})
}

func TestApproveScopes(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()

	t.Run("approves pending scopes", func(t *testing.T) {
		binding, _ := service.CreateAutoBinding(ctx, 1, "approve-1", "approve-2",
			[]string{channel.BindingScopeTerminalRead})
		binding, _ = service.RequestScopes(ctx, binding.ID, "approve-1",
			[]string{channel.BindingScopeTerminalWrite})

		approved, err := service.ApproveScopes(ctx, binding.ID, "approve-2",
			[]string{channel.BindingScopeTerminalWrite})
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if !approved.HasScope(channel.BindingScopeTerminalWrite) {
			t.Error("expected write scope to be granted")
		}
	})

	t.Run("wrong session returns error", func(t *testing.T) {
		binding, _ := service.CreateAutoBinding(ctx, 1, "approve-wrong-1", "approve-wrong-2",
			[]string{channel.BindingScopeTerminalRead})
		binding, _ = service.RequestScopes(ctx, binding.ID, "approve-wrong-1",
			[]string{channel.BindingScopeTerminalWrite})

		_, err := service.ApproveScopes(ctx, binding.ID, "approve-wrong-1",
			[]string{channel.BindingScopeTerminalWrite})
		if err != ErrNotAuthorized {
			t.Errorf("expected ErrNotAuthorized, got %v", err)
		}
	})

	t.Run("no valid pending scopes returns error", func(t *testing.T) {
		binding, _ := service.CreateAutoBinding(ctx, 1, "approve-none-1", "approve-none-2",
			[]string{channel.BindingScopeTerminalRead})

		_, err := service.ApproveScopes(ctx, binding.ID, "approve-none-2",
			[]string{channel.BindingScopeTerminalWrite})
		if err != ErrNoValidPendingScopes {
			t.Errorf("expected ErrNoValidPendingScopes, got %v", err)
		}
	})
}

func TestHasScope(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()

	t.Run("returns true for granted scope", func(t *testing.T) {
		service.CreateAutoBinding(ctx, 1, "has-1", "has-2",
			[]string{channel.BindingScopeTerminalRead})

		hasScope, err := service.HasScope(ctx, "has-1", "has-2", channel.BindingScopeTerminalRead)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if !hasScope {
			t.Error("expected to have scope")
		}
	})

	t.Run("returns false for missing scope", func(t *testing.T) {
		service.CreateAutoBinding(ctx, 1, "miss-1", "miss-2",
			[]string{channel.BindingScopeTerminalRead})

		hasScope, err := service.HasScope(ctx, "miss-1", "miss-2", channel.BindingScopeTerminalWrite)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if hasScope {
			t.Error("expected to not have write scope")
		}
	})

	t.Run("returns false for no binding", func(t *testing.T) {
		hasScope, err := service.HasScope(ctx, "no-bind-1", "no-bind-2", channel.BindingScopeTerminalRead)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if hasScope {
			t.Error("expected false for no binding")
		}
	})
}

func TestCleanupExpiredBindings(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil)
	ctx := context.Background()

	t.Run("cleans up expired bindings", func(t *testing.T) {
		// Create a pending binding and manually set expires_at to past
		service.RequestBinding(ctx, 1, "expired-1", "expired-2",
			[]string{channel.BindingScopeTerminalRead}, channel.BindingPolicyExplicitOnly)

		db.Exec("UPDATE session_bindings SET expires_at = datetime('now', '-1 day') WHERE initiator_session = ?", "expired-1")

		count, err := service.CleanupExpiredBindings(ctx)
		if err != nil {
			t.Fatalf("failed: %v", err)
		}
		if count != 1 {
			t.Errorf("expected 1 expired binding cleaned, got %d", count)
		}
	})
}

func TestEvaluatePolicy(t *testing.T) {
	db := setupTestDB(t)
	querier := NewMockSessionQuerier()
	service := NewService(db, querier)
	ctx := context.Background()

	t.Run("explicit only policy returns pending", func(t *testing.T) {
		autoApprove, status := service.evaluatePolicy(ctx, "s1", "s2", channel.BindingPolicyExplicitOnly)
		if autoApprove {
			t.Error("expected no auto-approve for explicit only")
		}
		if status != channel.BindingStatusPending {
			t.Errorf("expected pending status, got %s", status)
		}
	})

	t.Run("same user auto approves", func(t *testing.T) {
		querier.AddSession("same-user-1", map[string]interface{}{"user_id": int64(100)})
		querier.AddSession("same-user-2", map[string]interface{}{"user_id": int64(100)})

		autoApprove, status := service.evaluatePolicy(ctx, "same-user-1", "same-user-2", "")
		if !autoApprove {
			t.Error("expected auto-approve for same user")
		}
		if status != channel.BindingStatusActive {
			t.Errorf("expected active status, got %s", status)
		}
	})

	t.Run("same project auto approves with policy", func(t *testing.T) {
		querier.AddSession("proj-1", map[string]interface{}{"user_id": int64(1), "project_id": int64(10)})
		querier.AddSession("proj-2", map[string]interface{}{"user_id": int64(2), "project_id": int64(10)})

		autoApprove, status := service.evaluatePolicy(ctx, "proj-1", "proj-2", channel.BindingPolicySameProjectAuto)
		if !autoApprove {
			t.Error("expected auto-approve for same project")
		}
		if status != channel.BindingStatusActive {
			t.Errorf("expected active status, got %s", status)
		}
	})
}

func TestErrorVariables(t *testing.T) {
	if ErrBindingNotFound.Error() != "binding not found" {
		t.Errorf("unexpected error message: %s", ErrBindingNotFound.Error())
	}
	if ErrBindingExists.Error() != "binding already exists" {
		t.Errorf("unexpected error message: %s", ErrBindingExists.Error())
	}
	if ErrSelfBinding.Error() != "cannot bind a session to itself" {
		t.Errorf("unexpected error message: %s", ErrSelfBinding.Error())
	}
	if ErrInvalidScope.Error() != "invalid scope" {
		t.Errorf("unexpected error message: %s", ErrInvalidScope.Error())
	}
	if ErrNotAuthorized.Error() != "not authorized for this operation" {
		t.Errorf("unexpected error message: %s", ErrNotAuthorized.Error())
	}
	if ErrBindingNotPending.Error() != "binding is not pending" {
		t.Errorf("unexpected error message: %s", ErrBindingNotPending.Error())
	}
	if ErrBindingNotActive.Error() != "binding is not active" {
		t.Errorf("unexpected error message: %s", ErrBindingNotActive.Error())
	}
	if ErrNoValidPendingScopes.Error() != "no valid pending scopes to approve" {
		t.Errorf("unexpected error message: %s", ErrNoValidPendingScopes.Error())
	}
}
