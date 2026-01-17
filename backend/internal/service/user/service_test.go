package user

import (
	"context"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		t.Fatalf("failed to connect database: %v", err)
	}

	// Create user_git_credentials table first (referenced by users)
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_git_credentials (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			name TEXT NOT NULL,
			credential_type TEXT NOT NULL,
			repository_provider_id INTEGER,
			pat_encrypted TEXT,
			public_key TEXT,
			private_key_encrypted TEXT,
			fingerprint TEXT,
			host_pattern TEXT,
			is_default INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatalf("failed to create user_git_credentials table: %v", err)
	}

	// Create tables manually for SQLite compatibility
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			email TEXT NOT NULL UNIQUE,
			username TEXT NOT NULL UNIQUE,
			name TEXT,
			avatar_url TEXT,
			password_hash TEXT,
			is_active INTEGER NOT NULL DEFAULT 1,
			is_system_admin INTEGER NOT NULL DEFAULT 0,
			last_login_at DATETIME,
			is_email_verified INTEGER NOT NULL DEFAULT 0,
			email_verification_token TEXT,
			email_verification_expires_at DATETIME,
			password_reset_token TEXT,
			password_reset_expires_at DATETIME,
			default_git_credential_id INTEGER REFERENCES user_git_credentials(id),
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatalf("failed to create users table: %v", err)
	}

	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_identities (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			provider TEXT NOT NULL,
			provider_user_id TEXT NOT NULL,
			provider_username TEXT,
			access_token_encrypted TEXT,
			refresh_token_encrypted TEXT,
			token_expires_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatalf("failed to create user_identities table: %v", err)
	}

	return db
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)

	if service == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestCreateUser(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		Email:    "test@example.com",
		Username: "testuser",
		Name:     "Test User",
		Password: "password123",
	}

	user, err := service.Create(ctx, req)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	if user == nil {
		t.Fatal("expected non-nil user")
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected Email 'test@example.com', got %s", user.Email)
	}
	if user.Username != "testuser" {
		t.Errorf("expected Username 'testuser', got %s", user.Username)
	}
	if *user.Name != "Test User" {
		t.Errorf("expected Name 'Test User', got %s", *user.Name)
	}
	if !user.IsActive {
		t.Error("expected user to be active")
	}
}

func TestCreateUserDuplicateEmail(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		Email:    "test@example.com",
		Username: "testuser1",
	}
	service.Create(ctx, req)

	// Try to create user with same email
	req2 := &CreateRequest{
		Email:    "test@example.com",
		Username: "testuser2",
	}
	_, err := service.Create(ctx, req2)
	if err != ErrEmailAlreadyExists {
		t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestCreateUserDuplicateUsername(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		Email:    "test1@example.com",
		Username: "testuser",
	}
	service.Create(ctx, req)

	// Try to create user with same username
	req2 := &CreateRequest{
		Email:    "test2@example.com",
		Username: "testuser",
	}
	_, err := service.Create(ctx, req2)
	if err != ErrUsernameExists {
		t.Errorf("expected ErrUsernameExists, got %v", err)
	}
}

func TestGetByID(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create a user
	req := &CreateRequest{
		Email:    "test@example.com",
		Username: "testuser",
	}
	created, _ := service.Create(ctx, req)

	// Get the user
	user, err := service.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if user.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, user.ID)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	_, err := service.GetByID(ctx, 99999)
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestGetByEmail(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		Email:    "test@example.com",
		Username: "testuser",
	}
	service.Create(ctx, req)

	user, err := service.GetByEmail(ctx, "test@example.com")
	if err != nil {
		t.Fatalf("failed to get user by email: %v", err)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected Email 'test@example.com', got %s", user.Email)
	}
}

func TestGetByEmailNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	_, err := service.GetByEmail(ctx, "nonexistent@example.com")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestGetByUsername(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		Email:    "test@example.com",
		Username: "testuser",
	}
	service.Create(ctx, req)

	user, err := service.GetByUsername(ctx, "testuser")
	if err != nil {
		t.Fatalf("failed to get user by username: %v", err)
	}
	if user.Username != "testuser" {
		t.Errorf("expected Username 'testuser', got %s", user.Username)
	}
}

func TestGetByUsernameNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	_, err := service.GetByUsername(ctx, "nonexistent")
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestUpdateUser(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create a user
	req := &CreateRequest{
		Email:    "test@example.com",
		Username: "testuser",
	}
	created, _ := service.Create(ctx, req)

	// Update the user
	newName := "Updated Name"
	updates := map[string]interface{}{
		"name": newName,
	}

	updated, err := service.Update(ctx, created.ID, updates)
	if err != nil {
		t.Fatalf("failed to update user: %v", err)
	}

	if *updated.Name != "Updated Name" {
		t.Errorf("expected Name 'Updated Name', got %s", *updated.Name)
	}
}

func TestDeleteUser(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create a user
	req := &CreateRequest{
		Email:    "test@example.com",
		Username: "testuser",
	}
	created, _ := service.Create(ctx, req)

	// Delete the user
	err := service.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to delete user: %v", err)
	}

	// Verify deletion
	_, err = service.GetByID(ctx, created.ID)
	if err != ErrUserNotFound {
		t.Errorf("expected ErrUserNotFound, got %v", err)
	}
}

func TestAuthenticate(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create a user with password
	req := &CreateRequest{
		Email:    "test@example.com",
		Username: "testuser",
		Password: "password123",
	}
	service.Create(ctx, req)

	// Authenticate
	user, err := service.Authenticate(ctx, "test@example.com", "password123")
	if err != nil {
		t.Fatalf("failed to authenticate: %v", err)
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected Email 'test@example.com', got %s", user.Email)
	}
}

func TestAuthenticateInvalidPassword(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		Email:    "test@example.com",
		Username: "testuser",
		Password: "password123",
	}
	service.Create(ctx, req)

	_, err := service.Authenticate(ctx, "test@example.com", "wrongpassword")
	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthenticateUserNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	_, err := service.Authenticate(ctx, "nonexistent@example.com", "password")
	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestAuthenticateNoPassword(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create user without password
	req := &CreateRequest{
		Email:    "test@example.com",
		Username: "testuser",
	}
	service.Create(ctx, req)

	_, err := service.Authenticate(ctx, "test@example.com", "password")
	if err != ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestUpdatePassword(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		Email:    "test@example.com",
		Username: "testuser",
		Password: "oldpassword",
	}
	created, _ := service.Create(ctx, req)

	// Update password
	err := service.UpdatePassword(ctx, created.ID, "newpassword")
	if err != nil {
		t.Fatalf("failed to update password: %v", err)
	}

	// Should be able to authenticate with new password
	_, err = service.Authenticate(ctx, "test@example.com", "newpassword")
	if err != nil {
		t.Errorf("expected successful authentication, got %v", err)
	}
}

func TestGetOrCreateByOAuth(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create new user via OAuth
	user, isNew, err := service.GetOrCreateByOAuth(ctx, "github", "12345", "githubuser", "test@example.com", "Test User", "https://example.com/avatar.png")
	if err != nil {
		t.Fatalf("failed to get or create user: %v", err)
	}
	if !isNew {
		t.Error("expected isNew to be true")
	}
	if user.Email != "test@example.com" {
		t.Errorf("expected Email 'test@example.com', got %s", user.Email)
	}

	// Get existing user via OAuth
	user2, isNew2, err := service.GetOrCreateByOAuth(ctx, "github", "12345", "githubuser", "test@example.com", "Test User", "")
	if err != nil {
		t.Fatalf("failed to get existing user: %v", err)
	}
	if isNew2 {
		t.Error("expected isNew to be false")
	}
	if user2.ID != user.ID {
		t.Errorf("expected same user ID %d, got %d", user.ID, user2.ID)
	}
}

func TestListIdentities(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create user with OAuth
	user, _, _ := service.GetOrCreateByOAuth(ctx, "github", "12345", "githubuser", "test@example.com", "Test", "")

	// List identities
	identities, err := service.ListIdentities(ctx, user.ID)
	if err != nil {
		t.Fatalf("failed to list identities: %v", err)
	}
	if len(identities) != 1 {
		t.Errorf("expected 1 identity, got %d", len(identities))
	}
}

func TestGetIdentity(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	user, _, _ := service.GetOrCreateByOAuth(ctx, "github", "12345", "githubuser", "test@example.com", "Test", "")

	identity, err := service.GetIdentity(ctx, user.ID, "github")
	if err != nil {
		t.Fatalf("failed to get identity: %v", err)
	}
	if identity.Provider != "github" {
		t.Errorf("expected Provider 'github', got %s", identity.Provider)
	}
}

func TestDeleteIdentity(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	user, _, _ := service.GetOrCreateByOAuth(ctx, "github", "12345", "githubuser", "test@example.com", "Test", "")

	err := service.DeleteIdentity(ctx, user.ID, "github")
	if err != nil {
		t.Fatalf("failed to delete identity: %v", err)
	}

	identities, _ := service.ListIdentities(ctx, user.ID)
	if len(identities) != 0 {
		t.Errorf("expected 0 identities, got %d", len(identities))
	}
}

// Note: Search tests are skipped because SQLite doesn't support ILIKE
// The Search function is tested through integration tests with PostgreSQL

func TestUpdateIdentityTokens(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	user, _, _ := service.GetOrCreateByOAuth(ctx, "github", "12345", "githubuser", "test@example.com", "Test", "")

	err := service.UpdateIdentityTokens(ctx, user.ID, "github", "new_access_token", "new_refresh_token", nil)
	if err != nil {
		t.Fatalf("failed to update identity tokens: %v", err)
	}

	identity, _ := service.GetIdentity(ctx, user.ID, "github")
	if identity.AccessTokenEncrypted == nil || *identity.AccessTokenEncrypted != "new_access_token" {
		t.Error("expected access token to be updated")
	}
}

func TestAuthenticateInactiveUser(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		Email:    "inactive@example.com",
		Username: "inactiveuser",
		Password: "password123",
	}
	created, _ := service.Create(ctx, req)

	// Deactivate user
	db.Exec("UPDATE users SET is_active = 0 WHERE id = ?", created.ID)

	_, err := service.Authenticate(ctx, "inactive@example.com", "password123")
	if err != ErrUserInactive {
		t.Errorf("expected ErrUserInactive, got %v", err)
	}
}

func TestGetOrCreateByOAuthExistingEmail(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create user via regular signup
	service.Create(ctx, &CreateRequest{
		Email:    "existing@example.com",
		Username: "existing",
	})

	// Now OAuth with same email should link to existing user
	user, isNew, err := service.GetOrCreateByOAuth(ctx, "gitlab", "99999", "gitlabuser", "existing@example.com", "Existing User", "")
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if isNew {
		t.Error("expected isNew to be false for existing email")
	}
	if user.Username != "existing" {
		t.Errorf("expected username 'existing', got %s", user.Username)
	}
}

func TestGetOrCreateByOAuthDuplicateUsername(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create user with username
	service.Create(ctx, &CreateRequest{
		Email:    "first@example.com",
		Username: "duplicate",
	})

	// OAuth with same username should get a modified username
	user, isNew, err := service.GetOrCreateByOAuth(ctx, "github", "11111", "duplicate", "second@example.com", "Second User", "")
	if err != nil {
		t.Fatalf("failed: %v", err)
	}
	if !isNew {
		t.Error("expected isNew to be true")
	}
	// Username should be modified to avoid collision
	if user.Username == "duplicate" {
		t.Error("expected modified username due to collision")
	}
}

func TestCreateUserWithoutName(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		Email:    "noname@example.com",
		Username: "noname",
		// No Name provided
	}

	user, err := service.Create(ctx, req)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	if user.Name != nil {
		t.Error("expected Name to be nil")
	}
}

func TestGetIdentityNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		Email:    "noid@example.com",
		Username: "noid",
	}
	created, _ := service.Create(ctx, req)

	_, err := service.GetIdentity(ctx, created.ID, "github")
	if err == nil {
		t.Error("expected error for non-existent identity")
	}
}

func TestErrorVariables(t *testing.T) {
	// Test that error variables are properly defined
	if ErrUserNotFound.Error() != "user not found" {
		t.Errorf("unexpected error message: %s", ErrUserNotFound.Error())
	}
	if ErrEmailAlreadyExists.Error() != "email already exists" {
		t.Errorf("unexpected error message: %s", ErrEmailAlreadyExists.Error())
	}
	if ErrUsernameExists.Error() != "username already exists" {
		t.Errorf("unexpected error message: %s", ErrUsernameExists.Error())
	}
	if ErrInvalidCredentials.Error() != "invalid credentials" {
		t.Errorf("unexpected error message: %s", ErrInvalidCredentials.Error())
	}
	if ErrUserInactive.Error() != "user is inactive" {
		t.Errorf("unexpected error message: %s", ErrUserInactive.Error())
	}
}
