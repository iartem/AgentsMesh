-- Add identity_id foreign key to user_repository_providers
-- This links OAuth-based providers to their identity for access token retrieval

ALTER TABLE user_repository_providers
ADD COLUMN identity_id BIGINT REFERENCES user_identities(id) ON DELETE SET NULL;

CREATE INDEX idx_user_repository_providers_identity ON user_repository_providers(identity_id);

-- Drop the old user_git_connections table (confirmed no important data)
DROP TABLE IF EXISTS user_git_connections;
