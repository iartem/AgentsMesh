package gitprovider

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

	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS git_providers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			provider_type TEXT NOT NULL,
			name TEXT NOT NULL,
			base_url TEXT NOT NULL,
			client_id TEXT,
			client_secret_encrypted TEXT,
			bot_token_encrypted TEXT,
			is_default INTEGER NOT NULL DEFAULT 0,
			is_active INTEGER NOT NULL DEFAULT 1,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatalf("failed to create git_providers table: %v", err)
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

func TestCreate(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		OrganizationID: 1,
		ProviderType:   "gitlab",
		Name:           "GitLab",
		BaseURL:        "https://gitlab.com",
		IsDefault:      true,
	}

	provider, err := service.Create(ctx, req)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	if provider.Name != "GitLab" {
		t.Errorf("expected name 'GitLab', got %s", provider.Name)
	}
	if !provider.IsDefault {
		t.Error("expected provider to be default")
	}
}

func TestCreateDuplicateName(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		OrganizationID: 1,
		ProviderType:   "gitlab",
		Name:           "GitLab",
		BaseURL:        "https://gitlab.com",
	}
	service.Create(ctx, req)

	// Try to create with same name
	_, err := service.Create(ctx, req)
	if err != ErrProviderNameExists {
		t.Errorf("expected ErrProviderNameExists, got %v", err)
	}
}

func TestGetByID(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		OrganizationID: 1,
		ProviderType:   "gitlab",
		Name:           "GitLab",
		BaseURL:        "https://gitlab.com",
	}
	created, _ := service.Create(ctx, req)

	provider, err := service.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to get provider: %v", err)
	}
	if provider.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, provider.ID)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	_, err := service.GetByID(ctx, 999)
	if err != ErrProviderNotFound {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestUpdate(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		OrganizationID: 1,
		ProviderType:   "gitlab",
		Name:           "GitLab",
		BaseURL:        "https://gitlab.com",
	}
	created, _ := service.Create(ctx, req)

	updates := map[string]interface{}{
		"name": "GitLab Updated",
	}
	updated, err := service.Update(ctx, created.ID, updates)
	if err != nil {
		t.Fatalf("failed to update provider: %v", err)
	}
	if updated.Name != "GitLab Updated" {
		t.Errorf("expected name 'GitLab Updated', got %s", updated.Name)
	}
}

func TestDelete(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		OrganizationID: 1,
		ProviderType:   "gitlab",
		Name:           "GitLab",
		BaseURL:        "https://gitlab.com",
	}
	created, _ := service.Create(ctx, req)

	err := service.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to delete provider: %v", err)
	}

	_, err = service.GetByID(ctx, created.ID)
	if err != ErrProviderNotFound {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestListByOrganization(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req1 := &CreateRequest{
		OrganizationID: 1,
		ProviderType:   "gitlab",
		Name:           "GitLab",
		BaseURL:        "https://gitlab.com",
	}
	service.Create(ctx, req1)

	req2 := &CreateRequest{
		OrganizationID: 1,
		ProviderType:   "github",
		Name:           "GitHub",
		BaseURL:        "https://github.com",
	}
	service.Create(ctx, req2)

	providers, err := service.ListByOrganization(ctx, 1)
	if err != nil {
		t.Fatalf("failed to list providers: %v", err)
	}
	if len(providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(providers))
	}
}

func TestGetDefault(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		OrganizationID: 1,
		ProviderType:   "gitlab",
		Name:           "GitLab",
		BaseURL:        "https://gitlab.com",
		IsDefault:      true,
	}
	service.Create(ctx, req)

	provider, err := service.GetDefault(ctx, 1)
	if err != nil {
		t.Fatalf("failed to get default: %v", err)
	}
	if !provider.IsDefault {
		t.Error("expected default provider")
	}
}

func TestSetDefault(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req1 := &CreateRequest{
		OrganizationID: 1,
		ProviderType:   "gitlab",
		Name:           "GitLab",
		BaseURL:        "https://gitlab.com",
		IsDefault:      true,
	}
	provider1, _ := service.Create(ctx, req1)

	req2 := &CreateRequest{
		OrganizationID: 1,
		ProviderType:   "github",
		Name:           "GitHub",
		BaseURL:        "https://github.com",
	}
	provider2, _ := service.Create(ctx, req2)

	// Set provider2 as default
	err := service.SetDefault(ctx, 1, provider2.ID)
	if err != nil {
		t.Fatalf("failed to set default: %v", err)
	}

	// Check provider1 is no longer default
	p1, _ := service.GetByID(ctx, provider1.ID)
	if p1.IsDefault {
		t.Error("expected provider1 to not be default")
	}

	// Check provider2 is now default
	p2, _ := service.GetByID(ctx, provider2.ID)
	if !p2.IsDefault {
		t.Error("expected provider2 to be default")
	}
}

func TestCreateWithOptionalFields(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	clientID := "client-123"
	clientSecret := "encrypted-secret"
	botToken := "encrypted-token"

	req := &CreateRequest{
		OrganizationID:      1,
		ProviderType:        "gitlab",
		Name:                "GitLab",
		BaseURL:             "https://gitlab.com",
		ClientID:            &clientID,
		ClientSecretEncrypt: &clientSecret,
		BotTokenEncrypt:     &botToken,
	}

	provider, err := service.Create(ctx, req)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	if provider.ClientID == nil || *provider.ClientID != "client-123" {
		t.Error("expected ClientID to be set")
	}
}

func TestErrorVariables(t *testing.T) {
	if ErrProviderNotFound.Error() != "git provider not found" {
		t.Errorf("unexpected error message: %s", ErrProviderNotFound.Error())
	}
	if ErrProviderNameExists.Error() != "provider name already exists" {
		t.Errorf("unexpected error message: %s", ErrProviderNameExists.Error())
	}
}

func TestGetDefaultNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// No default provider exists
	_, err := service.GetDefault(ctx, 999)
	if err != ErrProviderNotFound {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestUpdateNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	_, err := service.Update(ctx, 99999, map[string]interface{}{"name": "test"})
	if err != ErrProviderNotFound {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestCreateUnsetsOtherDefaults(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create first default provider
	req1 := &CreateRequest{
		OrganizationID: 1,
		ProviderType:   "gitlab",
		Name:           "GitLab",
		BaseURL:        "https://gitlab.com",
		IsDefault:      true,
	}
	provider1, _ := service.Create(ctx, req1)

	// Create second default provider - should unset first
	req2 := &CreateRequest{
		OrganizationID: 1,
		ProviderType:   "github",
		Name:           "GitHub",
		BaseURL:        "https://github.com",
		IsDefault:      true,
	}
	provider2, _ := service.Create(ctx, req2)

	// Check provider1 is no longer default
	p1, _ := service.GetByID(ctx, provider1.ID)
	if p1.IsDefault {
		t.Error("expected provider1 to not be default after creating new default")
	}

	// Check provider2 is default
	p2, _ := service.GetByID(ctx, provider2.ID)
	if !p2.IsDefault {
		t.Error("expected provider2 to be default")
	}
}

func TestGetClientNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	_, err := service.GetClient(ctx, 99999, "access_token")
	if err != ErrProviderNotFound {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestGetClientWithBotTokenNotFound(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	_, err := service.GetClientWithBotToken(ctx, 99999)
	if err != ErrProviderNotFound {
		t.Errorf("expected ErrProviderNotFound, got %v", err)
	}
}

func TestGetClientWithBotTokenNoBotToken(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create provider without bot token
	req := &CreateRequest{
		OrganizationID: 1,
		ProviderType:   "gitlab",
		Name:           "GitLab",
		BaseURL:        "https://gitlab.com",
		// No BotTokenEncrypt
	}
	provider, _ := service.Create(ctx, req)

	_, err := service.GetClientWithBotToken(ctx, provider.ID)
	if err == nil {
		t.Error("expected error for missing bot token")
	}
	if err.Error() != "bot token not configured" {
		t.Errorf("expected 'bot token not configured', got %v", err)
	}
}

func TestListByOrganizationEmpty(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	providers, err := service.ListByOrganization(ctx, 999)
	if err != nil {
		t.Fatalf("failed to list providers: %v", err)
	}
	if len(providers) != 0 {
		t.Errorf("expected 0 providers, got %d", len(providers))
	}
}

func TestCreateWithoutDefault(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	req := &CreateRequest{
		OrganizationID: 1,
		ProviderType:   "gitlab",
		Name:           "GitLab",
		BaseURL:        "https://gitlab.com",
		IsDefault:      false,
	}

	provider, err := service.Create(ctx, req)
	if err != nil {
		t.Fatalf("failed to create provider: %v", err)
	}
	if provider.IsDefault {
		t.Error("expected provider to not be default")
	}
}
