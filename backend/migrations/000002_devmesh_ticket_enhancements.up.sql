-- AgentMesh Database Migration
-- Migration: 000002_devmesh_ticket_enhancements
-- Description: Add DevMesh and Ticket enhancements for migration from Mainline

-- ==========================================
-- 1. Ticket Enhancements
-- ==========================================

-- Add severity and estimate fields to tickets
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS severity VARCHAR(20);
ALTER TABLE tickets ADD COLUMN IF NOT EXISTS estimate INT;

CREATE INDEX IF NOT EXISTS idx_tickets_severity ON tickets(severity);

-- ==========================================
-- 2. Session Enhancements
-- ==========================================

-- Add model, permission_mode, think_level, agent_pid to sessions if not exist
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS model VARCHAR(50);
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS permission_mode VARCHAR(50);
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS think_level VARCHAR(50);
ALTER TABLE sessions ADD COLUMN IF NOT EXISTS agent_pid INT;

-- ==========================================
-- 3. Session Binding Enhancements
-- ==========================================

-- Add pending_scopes and timestamp fields to session_bindings if not exist
ALTER TABLE session_bindings ADD COLUMN IF NOT EXISTS pending_scopes TEXT[];
ALTER TABLE session_bindings ADD COLUMN IF NOT EXISTS requested_at TIMESTAMPTZ;
ALTER TABLE session_bindings ADD COLUMN IF NOT EXISTS responded_at TIMESTAMPTZ;
ALTER TABLE session_bindings ADD COLUMN IF NOT EXISTS expires_at TIMESTAMPTZ;
ALTER TABLE session_bindings ADD COLUMN IF NOT EXISTS rejection_reason VARCHAR(500);

-- ==========================================
-- 4. Ticket Merge Request Enhancements
-- ==========================================

-- Add pipeline and merge info to ticket_merge_requests
ALTER TABLE ticket_merge_requests ADD COLUMN IF NOT EXISTS pipeline_status VARCHAR(50);
ALTER TABLE ticket_merge_requests ADD COLUMN IF NOT EXISTS pipeline_id BIGINT;
ALTER TABLE ticket_merge_requests ADD COLUMN IF NOT EXISTS pipeline_url TEXT;
ALTER TABLE ticket_merge_requests ADD COLUMN IF NOT EXISTS merge_commit_sha VARCHAR(40);
ALTER TABLE ticket_merge_requests ADD COLUMN IF NOT EXISTS merged_at TIMESTAMPTZ;
ALTER TABLE ticket_merge_requests ADD COLUMN IF NOT EXISTS merged_by_id BIGINT;
ALTER TABLE ticket_merge_requests ADD COLUMN IF NOT EXISTS last_synced_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_ticket_mrs_pipeline ON ticket_merge_requests(pipeline_status);

-- ==========================================
-- 5. Ticket Commits Table
-- ==========================================

CREATE TABLE IF NOT EXISTS ticket_commits (
    id BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    repository_id BIGINT NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    session_id BIGINT REFERENCES sessions(id) ON DELETE SET NULL,

    commit_sha VARCHAR(40) NOT NULL,
    commit_message TEXT,
    commit_url TEXT,
    author_name VARCHAR(255),
    author_email VARCHAR(255),
    committed_at TIMESTAMPTZ,

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ticket_commits_ticket ON ticket_commits(ticket_id);
CREATE INDEX IF NOT EXISTS idx_ticket_commits_repo_sha ON ticket_commits(repository_id, commit_sha);

-- ==========================================
-- 6. Ticket Relations Table
-- ==========================================

CREATE TABLE IF NOT EXISTS ticket_relations (
    id BIGSERIAL PRIMARY KEY,
    organization_id BIGINT NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    source_ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    target_ticket_id BIGINT NOT NULL REFERENCES tickets(id) ON DELETE CASCADE,
    relation_type VARCHAR(50) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),

    UNIQUE(source_ticket_id, target_ticket_id, relation_type)
);

CREATE INDEX IF NOT EXISTS idx_ticket_relations_source ON ticket_relations(source_ticket_id);
CREATE INDEX IF NOT EXISTS idx_ticket_relations_target ON ticket_relations(target_ticket_id);

-- ==========================================
-- 7. Channel Sessions Table (for DevMesh)
-- ==========================================

CREATE TABLE IF NOT EXISTS channel_sessions (
    id BIGSERIAL PRIMARY KEY,
    channel_id BIGINT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    session_key VARCHAR(100) NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(channel_id, session_key)
);

CREATE INDEX IF NOT EXISTS idx_channel_sessions_channel ON channel_sessions(channel_id);
CREATE INDEX IF NOT EXISTS idx_channel_sessions_session ON channel_sessions(session_key);

-- ==========================================
-- 8. Channel Access Tracking (for DevMesh)
-- ==========================================

CREATE TABLE IF NOT EXISTS channel_access (
    id BIGSERIAL PRIMARY KEY,
    channel_id BIGINT NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    session_key VARCHAR(100),
    user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    last_access TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_channel_access_channel ON channel_access(channel_id);
CREATE INDEX IF NOT EXISTS idx_channel_access_session ON channel_access(session_key);
