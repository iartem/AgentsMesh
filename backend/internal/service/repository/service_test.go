package repository

import (
	"context"
	"testing"

	gitproviderService "github.com/anthropics/agentmesh/backend/internal/service/gitprovider"
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

	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS repositories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			team_id INTEGER,
			git_provider_id INTEGER NOT NULL,
			external_id TEXT NOT NULL,
			name TEXT NOT NULL,
			full_path TEXT NOT NULL,
			default_branch TEXT NOT NULL DEFAULT 'main',
			ticket_prefix TEXT,
			is_active INTEGER NOT NULL DEFAULT 1,
			last_synced_at DATETIME,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatalf("failed to create repositories table: %v", err)
	}

	return db
}

func seedGitProvider(t *testing.T, db *gorm.DB) int64 {
	err := db.Exec(`INSERT INTO git_providers (id, organization_id, provider_type, name, base_url, is_default, is_active)
		VALUES (1, 1, 'gitlab', 'GitLab', 'https://gitlab.com', 1, 1)`).Error
	if err != nil {
		t.Fatalf("failed to seed git provider: %v", err)
	}
	return 1
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)

	if service == nil {
		t.Fatal("expected non-nil service")
	}
}

func TestCreate(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	providerID := seedGitProvider(t, db)

	req := &CreateRequest{
		OrganizationID: 1,
		GitProviderID:  providerID,
		ExternalID:     "12345",
		Name:           "test-repo",
		FullPath:       "org/test-repo",
		DefaultBranch:  "main",
	}

	repo, err := service.Create(ctx, req)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	if repo.Name != "test-repo" {
		t.Errorf("expected name 'test-repo', got %s", repo.Name)
	}
}

func TestCreateDuplicate(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	providerID := seedGitProvider(t, db)

	req := &CreateRequest{
		OrganizationID: 1,
		GitProviderID:  providerID,
		ExternalID:     "12345",
		Name:           "test-repo",
		FullPath:       "org/test-repo",
	}
	service.Create(ctx, req)

	// Try to create duplicate
	_, err := service.Create(ctx, req)
	if err != ErrRepositoryExists {
		t.Errorf("expected ErrRepositoryExists, got %v", err)
	}
}

func TestCreateWithDefaultBranch(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	providerID := seedGitProvider(t, db)

	req := &CreateRequest{
		OrganizationID: 1,
		GitProviderID:  providerID,
		ExternalID:     "12345",
		Name:           "test-repo",
		FullPath:       "org/test-repo",
		// No DefaultBranch - should default to "main"
	}

	repo, err := service.Create(ctx, req)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	if repo.DefaultBranch != "main" {
		t.Errorf("expected default branch 'main', got %s", repo.DefaultBranch)
	}
}

func TestGetByID(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	providerID := seedGitProvider(t, db)

	req := &CreateRequest{
		OrganizationID: 1,
		GitProviderID:  providerID,
		ExternalID:     "12345",
		Name:           "test-repo",
		FullPath:       "org/test-repo",
	}
	created, _ := service.Create(ctx, req)

	repo, err := service.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to get repository: %v", err)
	}
	if repo.ID != created.ID {
		t.Errorf("expected ID %d, got %d", created.ID, repo.ID)
	}
}

func TestGetByIDNotFound(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	_, err := service.GetByID(ctx, 999)
	if err != ErrRepositoryNotFound {
		t.Errorf("expected ErrRepositoryNotFound, got %v", err)
	}
}

func TestUpdate(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	providerID := seedGitProvider(t, db)

	req := &CreateRequest{
		OrganizationID: 1,
		GitProviderID:  providerID,
		ExternalID:     "12345",
		Name:           "test-repo",
		FullPath:       "org/test-repo",
	}
	created, _ := service.Create(ctx, req)

	updates := map[string]interface{}{
		"name": "updated-repo",
	}
	updated, err := service.Update(ctx, created.ID, updates)
	if err != nil {
		t.Fatalf("failed to update repository: %v", err)
	}
	if updated.Name != "updated-repo" {
		t.Errorf("expected name 'updated-repo', got %s", updated.Name)
	}
}

func TestDelete(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	providerID := seedGitProvider(t, db)

	req := &CreateRequest{
		OrganizationID: 1,
		GitProviderID:  providerID,
		ExternalID:     "12345",
		Name:           "test-repo",
		FullPath:       "org/test-repo",
	}
	created, _ := service.Create(ctx, req)

	err := service.Delete(ctx, created.ID)
	if err != nil {
		t.Fatalf("failed to delete repository: %v", err)
	}

	_, err = service.GetByID(ctx, created.ID)
	if err != ErrRepositoryNotFound {
		t.Errorf("expected ErrRepositoryNotFound, got %v", err)
	}
}

func TestListByOrganization(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	providerID := seedGitProvider(t, db)

	req1 := &CreateRequest{
		OrganizationID: 1,
		GitProviderID:  providerID,
		ExternalID:     "12345",
		Name:           "repo-1",
		FullPath:       "org/repo-1",
	}
	service.Create(ctx, req1)

	req2 := &CreateRequest{
		OrganizationID: 1,
		GitProviderID:  providerID,
		ExternalID:     "12346",
		Name:           "repo-2",
		FullPath:       "org/repo-2",
	}
	service.Create(ctx, req2)

	repos, err := service.ListByOrganization(ctx, 1, nil)
	if err != nil {
		t.Fatalf("failed to list repositories: %v", err)
	}
	if len(repos) != 2 {
		t.Errorf("expected 2 repositories, got %d", len(repos))
	}
}

func TestListByOrganizationWithTeam(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	providerID := seedGitProvider(t, db)

	teamID := int64(1)
	req1 := &CreateRequest{
		OrganizationID: 1,
		TeamID:         &teamID,
		GitProviderID:  providerID,
		ExternalID:     "12345",
		Name:           "repo-1",
		FullPath:       "org/repo-1",
	}
	service.Create(ctx, req1)

	req2 := &CreateRequest{
		OrganizationID: 1,
		GitProviderID:  providerID,
		ExternalID:     "12346",
		Name:           "repo-2",
		FullPath:       "org/repo-2",
	}
	service.Create(ctx, req2)

	repos, err := service.ListByOrganization(ctx, 1, &teamID)
	if err != nil {
		t.Fatalf("failed to list repositories: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("expected 1 repository, got %d", len(repos))
	}
}

func TestListByTeam(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	providerID := seedGitProvider(t, db)

	teamID := int64(1)
	req := &CreateRequest{
		OrganizationID: 1,
		TeamID:         &teamID,
		GitProviderID:  providerID,
		ExternalID:     "12345",
		Name:           "repo-1",
		FullPath:       "org/repo-1",
	}
	service.Create(ctx, req)

	repos, err := service.ListByTeam(ctx, 1)
	if err != nil {
		t.Fatalf("failed to list repositories by team: %v", err)
	}
	if len(repos) != 1 {
		t.Errorf("expected 1 repository, got %d", len(repos))
	}
}

func TestGetByExternalID(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	providerID := seedGitProvider(t, db)

	req := &CreateRequest{
		OrganizationID: 1,
		GitProviderID:  providerID,
		ExternalID:     "12345",
		Name:           "test-repo",
		FullPath:       "org/test-repo",
	}
	service.Create(ctx, req)

	repo, err := service.GetByExternalID(ctx, providerID, "12345")
	if err != nil {
		t.Fatalf("failed to get by external ID: %v", err)
	}
	if repo.ExternalID != "12345" {
		t.Errorf("expected external ID '12345', got %s", repo.ExternalID)
	}
}

func TestGetByExternalIDNotFound(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	_, err := service.GetByExternalID(ctx, 1, "nonexistent")
	if err != ErrRepositoryNotFound {
		t.Errorf("expected ErrRepositoryNotFound, got %v", err)
	}
}

func TestAssignToTeam(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	providerID := seedGitProvider(t, db)

	req := &CreateRequest{
		OrganizationID: 1,
		GitProviderID:  providerID,
		ExternalID:     "12345",
		Name:           "test-repo",
		FullPath:       "org/test-repo",
	}
	created, _ := service.Create(ctx, req)

	teamID := int64(1)
	err := service.AssignToTeam(ctx, created.ID, &teamID)
	if err != nil {
		t.Fatalf("failed to assign to team: %v", err)
	}

	repo, _ := service.GetByID(ctx, created.ID)
	if repo.TeamID == nil || *repo.TeamID != 1 {
		t.Error("expected repository to be assigned to team 1")
	}
}

func TestCreateWithTicketPrefix(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	providerID := seedGitProvider(t, db)

	prefix := "PROJ"
	req := &CreateRequest{
		OrganizationID: 1,
		GitProviderID:  providerID,
		ExternalID:     "12345",
		Name:           "test-repo",
		FullPath:       "org/test-repo",
		TicketPrefix:   &prefix,
	}

	repo, err := service.Create(ctx, req)
	if err != nil {
		t.Fatalf("failed to create repository: %v", err)
	}
	if repo.TicketPrefix == nil || *repo.TicketPrefix != "PROJ" {
		t.Error("expected ticket prefix 'PROJ'")
	}
}

func TestErrorVariables(t *testing.T) {
	if ErrRepositoryNotFound.Error() != "repository not found" {
		t.Errorf("unexpected error message: %s", ErrRepositoryNotFound.Error())
	}
	if ErrRepositoryExists.Error() != "repository already exists" {
		t.Errorf("unexpected error message: %s", ErrRepositoryExists.Error())
	}
}

func seedGitProviderWithType(t *testing.T, db *gorm.DB, providerType string) int64 {
	var count int64
	db.Raw("SELECT COUNT(*) FROM git_providers").Scan(&count)
	id := count + 1

	err := db.Exec(`INSERT INTO git_providers (id, organization_id, provider_type, name, base_url, is_default, is_active)
		VALUES (?, 1, ?, 'Provider', 'https://test.com', 1, 1)`, id, providerType).Error
	if err != nil {
		t.Fatalf("failed to seed git provider: %v", err)
	}
	return id
}

func TestGetCloneURL(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	t.Run("github clone URL", func(t *testing.T) {
		providerID := seedGitProviderWithType(t, db, "github")
		req := &CreateRequest{
			OrganizationID: 1,
			GitProviderID:  providerID,
			ExternalID:     "gh_12345",
			Name:           "github-repo",
			FullPath:       "owner/repo",
		}
		created, _ := service.Create(ctx, req)

		cloneURL, err := service.GetCloneURL(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetCloneURL failed: %v", err)
		}
		if cloneURL != "https://github.com/owner/repo.git" {
			t.Errorf("expected 'https://github.com/owner/repo.git', got %s", cloneURL)
		}
	})

	t.Run("gitlab clone URL", func(t *testing.T) {
		providerID := seedGitProviderWithType(t, db, "gitlab")
		req := &CreateRequest{
			OrganizationID: 1,
			GitProviderID:  providerID,
			ExternalID:     "gl_12345",
			Name:           "gitlab-repo",
			FullPath:       "group/project",
		}
		created, _ := service.Create(ctx, req)

		cloneURL, err := service.GetCloneURL(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetCloneURL failed: %v", err)
		}
		if cloneURL != "https://test.com/group/project.git" {
			t.Errorf("expected 'https://test.com/group/project.git', got %s", cloneURL)
		}
	})

	t.Run("gitee clone URL", func(t *testing.T) {
		providerID := seedGitProviderWithType(t, db, "gitee")
		req := &CreateRequest{
			OrganizationID: 1,
			GitProviderID:  providerID,
			ExternalID:     "gitee_12345",
			Name:           "gitee-repo",
			FullPath:       "user/gitee-repo",
		}
		created, _ := service.Create(ctx, req)

		cloneURL, err := service.GetCloneURL(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetCloneURL failed: %v", err)
		}
		if cloneURL != "https://gitee.com/user/gitee-repo.git" {
			t.Errorf("expected 'https://gitee.com/user/gitee-repo.git', got %s", cloneURL)
		}
	})

	t.Run("default clone URL", func(t *testing.T) {
		providerID := seedGitProviderWithType(t, db, "custom")
		req := &CreateRequest{
			OrganizationID: 1,
			GitProviderID:  providerID,
			ExternalID:     "custom_12345",
			Name:           "custom-repo",
			FullPath:       "org/custom-repo",
		}
		created, _ := service.Create(ctx, req)

		cloneURL, err := service.GetCloneURL(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetCloneURL failed: %v", err)
		}
		if cloneURL != "https://test.com/org/custom-repo.git" {
			t.Errorf("expected 'https://test.com/org/custom-repo.git', got %s", cloneURL)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := service.GetCloneURL(ctx, 99999)
		if err != ErrRepositoryNotFound {
			t.Errorf("expected ErrRepositoryNotFound, got %v", err)
		}
	})
}

func TestGetNextTicketNumber(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	// Create tickets table for testing
	err := db.Exec(`
		CREATE TABLE IF NOT EXISTS tickets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			repository_id INTEGER NOT NULL,
			number INTEGER NOT NULL,
			identifier TEXT NOT NULL,
			title TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	if err != nil {
		t.Fatalf("failed to create tickets table: %v", err)
	}

	providerID := seedGitProvider(t, db)

	req := &CreateRequest{
		OrganizationID: 1,
		GitProviderID:  providerID,
		ExternalID:     "ticket_12345",
		Name:           "ticket-repo",
		FullPath:       "org/ticket-repo",
	}
	created, _ := service.Create(ctx, req)

	t.Run("first ticket number", func(t *testing.T) {
		num, err := service.GetNextTicketNumber(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetNextTicketNumber failed: %v", err)
		}
		if num != 1 {
			t.Errorf("expected 1, got %d", num)
		}
	})

	t.Run("after existing tickets", func(t *testing.T) {
		// Insert some tickets
		db.Exec("INSERT INTO tickets (repository_id, number, identifier, title) VALUES (?, 1, 'TKT-1', 'First')", created.ID)
		db.Exec("INSERT INTO tickets (repository_id, number, identifier, title) VALUES (?, 5, 'TKT-5', 'Fifth')", created.ID)
		db.Exec("INSERT INTO tickets (repository_id, number, identifier, title) VALUES (?, 3, 'TKT-3', 'Third')", created.ID)

		num, err := service.GetNextTicketNumber(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetNextTicketNumber failed: %v", err)
		}
		if num != 6 {
			t.Errorf("expected 6, got %d", num)
		}
	})
}

func TestSyncFromProviderNotFound(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	_, err := service.SyncFromProvider(ctx, 99999, "access_token")
	if err != ErrRepositoryNotFound {
		t.Errorf("expected ErrRepositoryNotFound, got %v", err)
	}
}

func TestListBranchesNotFound(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	_, err := service.ListBranches(ctx, 99999, "access_token")
	if err != ErrRepositoryNotFound {
		t.Errorf("expected ErrRepositoryNotFound, got %v", err)
	}
}

func TestImportFromProviderExisting(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	providerID := seedGitProvider(t, db)

	// Create existing repository
	req := &CreateRequest{
		OrganizationID: 1,
		GitProviderID:  providerID,
		ExternalID:     "import_12345",
		Name:           "existing-repo",
		FullPath:       "org/existing-repo",
	}
	existing, _ := service.Create(ctx, req)

	// Try to import same repository - should return existing
	repo, err := service.ImportFromProvider(ctx, 1, providerID, "import_12345", "access_token")
	if err != nil {
		t.Fatalf("ImportFromProvider failed: %v", err)
	}
	if repo.ID != existing.ID {
		t.Errorf("expected to return existing repo ID %d, got %d", existing.ID, repo.ID)
	}
}

func TestUpdateNotFound(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	_, err := service.Update(ctx, 99999, map[string]interface{}{"name": "test"})
	if err != ErrRepositoryNotFound {
		t.Errorf("expected ErrRepositoryNotFound, got %v", err)
	}
}

func TestAssignToTeamUnassign(t *testing.T) {
	db := setupTestDB(t)
	gitProviderSvc := gitproviderService.NewService(db)
	service := NewService(db, gitProviderSvc)
	ctx := context.Background()

	providerID := seedGitProvider(t, db)
	teamID := int64(1)

	req := &CreateRequest{
		OrganizationID: 1,
		TeamID:         &teamID,
		GitProviderID:  providerID,
		ExternalID:     "unassign_12345",
		Name:           "test-repo",
		FullPath:       "org/test-repo",
	}
	created, _ := service.Create(ctx, req)

	// Unassign from team
	err := service.AssignToTeam(ctx, created.ID, nil)
	if err != nil {
		t.Fatalf("failed to unassign from team: %v", err)
	}

	repo, _ := service.GetByID(ctx, created.ID)
	if repo.TeamID != nil {
		t.Error("expected repository to have no team")
	}
}
