package agent

import (
	"context"
	"testing"

	"github.com/anthropics/agentmesh/backend/internal/domain/agent"
	"github.com/anthropics/agentmesh/backend/pkg/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupCredentialTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	require.NoError(t, err)

	// Create tables
	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS user_agent_credentials (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_id INTEGER NOT NULL,
			agent_type_id INTEGER NOT NULL,
			credentials_encrypted TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(user_id, agent_type_id)
		)
	`).Error
	require.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS organization_agents (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			organization_id INTEGER NOT NULL,
			agent_type_id INTEGER NOT NULL,
			is_enabled INTEGER NOT NULL DEFAULT 0,
			is_default INTEGER NOT NULL DEFAULT 0,
			credentials_encrypted TEXT,
			custom_launch_args TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(organization_id, agent_type_id)
		)
	`).Error
	require.NoError(t, err)

	err = db.Exec(`
		CREATE TABLE IF NOT EXISTS agent_types (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			slug TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			description TEXT,
			launch_command TEXT,
			default_args TEXT,
			is_builtin INTEGER NOT NULL DEFAULT 0,
			is_active INTEGER NOT NULL DEFAULT 1,
			credential_schema TEXT,
			status_detection TEXT,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Error
	require.NoError(t, err)

	return db
}

func TestNewCredentialService(t *testing.T) {
	db := setupCredentialTestDB(t)
	encryptor := crypto.NewEncryptor("test-key-32-bytes-long-1234567!")

	svc := NewCredentialService(db, encryptor)
	assert.NotNil(t, svc)
	assert.Equal(t, db, svc.db)
	assert.Equal(t, encryptor, svc.encryptor)
}

func TestSetAndGetUserCredentials(t *testing.T) {
	ctx := context.Background()
	db := setupCredentialTestDB(t)
	encryptor := crypto.NewEncryptor("test-key-32-bytes-long-1234567!")
	svc := NewCredentialService(db, encryptor)

	t.Run("set and get user credentials", func(t *testing.T) {
		creds := map[string]string{
			"api_key":   "sk-test-key-123",
			"auth_token": "auth-token-456",
		}

		err := svc.SetUserCredentials(ctx, 1, 1, creds)
		require.NoError(t, err)

		retrieved, err := svc.GetUserCredentials(ctx, 1, 1)
		require.NoError(t, err)
		assert.Equal(t, creds["api_key"], retrieved["api_key"])
		assert.Equal(t, creds["auth_token"], retrieved["auth_token"])
	})

	t.Run("update existing credentials", func(t *testing.T) {
		creds := map[string]string{
			"api_key": "new-key-789",
		}

		err := svc.SetUserCredentials(ctx, 1, 1, creds)
		require.NoError(t, err)

		retrieved, err := svc.GetUserCredentials(ctx, 1, 1)
		require.NoError(t, err)
		assert.Equal(t, "new-key-789", retrieved["api_key"])
	})

	t.Run("get non-existent credentials", func(t *testing.T) {
		_, err := svc.GetUserCredentials(ctx, 999, 1)
		assert.Error(t, err)
		assert.Equal(t, ErrCredentialsNotFound, err)
	})
}

func TestSetAndGetOrganizationCredentials(t *testing.T) {
	ctx := context.Background()
	db := setupCredentialTestDB(t)
	encryptor := crypto.NewEncryptor("test-key-32-bytes-long-1234567!")
	svc := NewCredentialService(db, encryptor)

	// First create an organization agent record
	orgAgent := &agent.OrganizationAgent{
		OrganizationID: 1,
		AgentTypeID:    1,
		IsEnabled:      true,
	}
	db.Create(orgAgent)

	t.Run("set and get organization credentials", func(t *testing.T) {
		creds := map[string]string{
			"api_key": "org-api-key-123",
		}

		err := svc.SetOrganizationCredentials(ctx, 1, 1, creds)
		require.NoError(t, err)

		// Reload from DB to verify
		var updated agent.OrganizationAgent
		db.First(&updated, orgAgent.ID)
		assert.NotNil(t, updated.CredentialsEncrypted)
	})

	t.Run("get non-existent org credentials", func(t *testing.T) {
		_, err := svc.GetOrganizationCredentials(ctx, 999, 1)
		assert.Error(t, err)
		assert.Equal(t, ErrCredentialsNotFound, err)
	})
}

func TestCredentialService_GetEffectiveCredentials(t *testing.T) {
	ctx := context.Background()
	db := setupCredentialTestDB(t)
	encryptor := crypto.NewEncryptor("test-key-32-bytes-long-1234567!")
	svc := NewCredentialService(db, encryptor)

	// Setup organization credentials
	orgAgent := &agent.OrganizationAgent{
		OrganizationID: 1,
		AgentTypeID:    1,
		IsEnabled:      true,
	}
	db.Create(orgAgent)
	svc.SetOrganizationCredentials(ctx, 1, 1, map[string]string{"api_key": "org-key"})

	t.Run("returns user credentials when available", func(t *testing.T) {
		err := svc.SetUserCredentials(ctx, 10, 1, map[string]string{"api_key": "user-key"})
		require.NoError(t, err)

		creds, err := svc.GetEffectiveCredentials(ctx, 10, 1, 1)
		require.NoError(t, err)
		assert.Equal(t, "user-key", creds["api_key"])
	})

	t.Run("falls back to org credentials", func(t *testing.T) {
		// User without credentials
		creds, err := svc.GetEffectiveCredentials(ctx, 999, 1, 1)
		require.NoError(t, err)
		assert.Equal(t, "org-key", creds["api_key"])
	})

	t.Run("returns error when no credentials", func(t *testing.T) {
		_, err := svc.GetEffectiveCredentials(ctx, 999, 999, 1)
		assert.Error(t, err)
	})
}

func TestDeleteUserCredentials(t *testing.T) {
	ctx := context.Background()
	db := setupCredentialTestDB(t)
	encryptor := crypto.NewEncryptor("test-key-32-bytes-long-1234567!")
	svc := NewCredentialService(db, encryptor)

	// Create credentials first
	err := svc.SetUserCredentials(ctx, 1, 1, map[string]string{"api_key": "test"})
	require.NoError(t, err)

	// Delete
	err = svc.DeleteUserCredentials(ctx, 1, 1)
	require.NoError(t, err)

	// Verify deleted
	_, err = svc.GetUserCredentials(ctx, 1, 1)
	assert.Error(t, err)
	assert.Equal(t, ErrCredentialsNotFound, err)
}

func TestGetEnvVarsForSession(t *testing.T) {
	ctx := context.Background()
	db := setupCredentialTestDB(t)
	encryptor := crypto.NewEncryptor("test-key-32-bytes-long-1234567!")
	svc := NewCredentialService(db, encryptor)

	// Create agent type with credential schema
	agentType := &agent.AgentType{
		Slug:          "claude-code",
		Name:          "Claude Code",
		LaunchCommand: "claude",
		IsActive:      true,
		CredentialSchema: []agent.CredentialField{
			{Name: "api_key", Type: "secret", EnvVar: "ANTHROPIC_API_KEY", Required: true},
			{Name: "auth_token", Type: "secret", EnvVar: "CLAUDE_AUTH_TOKEN", Required: false},
		},
	}
	db.Create(agentType)

	t.Run("returns env vars from user credentials", func(t *testing.T) {
		// Set user credentials
		err := svc.SetUserCredentials(ctx, 100, agentType.ID, map[string]string{
			"api_key":    "sk-ant-user123",
			"auth_token": "user-token-456",
		})
		require.NoError(t, err)

		envVars, err := svc.GetEnvVarsForSession(ctx, 100, 1, agentType.ID)
		require.NoError(t, err)
		assert.Equal(t, "sk-ant-user123", envVars["ANTHROPIC_API_KEY"])
		assert.Equal(t, "user-token-456", envVars["CLAUDE_AUTH_TOKEN"])
	})

	t.Run("returns nil for no credentials", func(t *testing.T) {
		envVars, err := svc.GetEnvVarsForSession(ctx, 999, 999, agentType.ID)
		require.NoError(t, err)
		assert.Nil(t, envVars)
	})

	t.Run("returns error for non-existent agent type", func(t *testing.T) {
		_, err := svc.GetEnvVarsForSession(ctx, 1, 1, 99999)
		assert.Error(t, err)
	})
}

func TestEncryptionWithoutEncryptor(t *testing.T) {
	ctx := context.Background()
	db := setupCredentialTestDB(t)
	// No encryptor
	svc := NewCredentialService(db, nil)

	t.Run("stores and retrieves plain credentials", func(t *testing.T) {
		creds := map[string]string{
			"api_key": "plain-key",
		}

		err := svc.SetUserCredentials(ctx, 1, 1, creds)
		require.NoError(t, err)

		retrieved, err := svc.GetUserCredentials(ctx, 1, 1)
		require.NoError(t, err)
		assert.Equal(t, "plain-key", retrieved["api_key"])
	})
}

func TestDecryptCredentialsMap(t *testing.T) {
	db := setupCredentialTestDB(t)
	encryptor := crypto.NewEncryptor("test-key-32-bytes-long-1234567!")
	svc := NewCredentialService(db, encryptor)

	t.Run("returns error for nil credentials", func(t *testing.T) {
		_, err := svc.decryptCredentialsMap(nil)
		assert.Error(t, err)
		assert.Equal(t, ErrCredentialsNotFound, err)
	})

	t.Run("handles unencrypted values gracefully", func(t *testing.T) {
		// Values that aren't encrypted should still work
		creds := agent.EncryptedCredentials{
			"plain_key": "plain_value",
		}

		result, err := svc.decryptCredentialsMap(creds)
		require.NoError(t, err)
		assert.Equal(t, "plain_value", result["plain_key"])
	})
}

func TestErrorConstants(t *testing.T) {
	assert.Equal(t, "credentials not found", ErrCredentialsNotFound.Error())
	assert.Equal(t, "failed to decrypt credentials", ErrDecryptionFailed.Error())
}
