package devmesh

import (
	"context"
	"testing"

	"github.com/anthropics/agentmesh/backend/internal/domain/session"
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

	// Create tables
	db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		session_key TEXT NOT NULL UNIQUE,
		organization_id INTEGER NOT NULL,
		team_id INTEGER,
		ticket_id INTEGER,
		repository_id INTEGER,
		runner_id INTEGER,
		agent_type_id INTEGER,
		custom_agent_type_id INTEGER,
		created_by_id INTEGER NOT NULL,
		pty_p_id INTEGER,
		status TEXT NOT NULL DEFAULT 'pending',
		agent_status TEXT NOT NULL DEFAULT 'unknown',
		agent_p_id INTEGER,
		started_at DATETIME,
		finished_at DATETIME,
		last_activity DATETIME,
		initial_prompt TEXT NOT NULL DEFAULT '',
		branch_name TEXT,
		worktree_path TEXT,
		model TEXT,
		permission_mode TEXT,
		think_level TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS channel_sessions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		channel_id INTEGER NOT NULL,
		session_key TEXT NOT NULL,
		joined_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS channel_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		channel_id INTEGER NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS channel_access (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		channel_id INTEGER NOT NULL,
		session_key TEXT,
		user_id INTEGER,
		last_access DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)

	return db
}

func TestNewService(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)

	if service == nil {
		t.Fatal("expected non-nil service")
	}
	if service.db != db {
		t.Error("expected service.db to be the provided db")
	}
}

func TestSessionToNode(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)

	ticketID := int64(100)
	repoID := int64(200)
	model := "claude-3-sonnet"
	sess := &session.Session{
		SessionKey:   "test-session-key",
		Status:       "running",
		AgentStatus:  "working",
		Model:        &model,
		TicketID:     &ticketID,
		RepositoryID: &repoID,
		CreatedByID:  1,
		RunnerID:     5,
	}

	node := service.sessionToNode(sess)

	if node.SessionKey != "test-session-key" {
		t.Errorf("SessionKey = %s, want test-session-key", node.SessionKey)
	}
	if node.Status != "running" {
		t.Errorf("Status = %s, want running", node.Status)
	}
	if node.AgentStatus != "working" {
		t.Errorf("AgentStatus = %s, want working", node.AgentStatus)
	}
	if node.Model == nil || *node.Model != "claude-3-sonnet" {
		t.Errorf("Model mismatch")
	}
	if node.TicketID == nil || *node.TicketID != 100 {
		t.Error("TicketID mismatch")
	}
	if node.RepositoryID == nil || *node.RepositoryID != 200 {
		t.Error("RepositoryID mismatch")
	}
}

func TestGetChannelSessions(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)
	ctx := context.Background()

	// Add some channel sessions
	db.Exec(`INSERT INTO channel_sessions (channel_id, session_key) VALUES (1, 'session-1'), (1, 'session-2'), (2, 'session-3')`)

	keys := service.getChannelSessions(ctx, 1)
	if len(keys) != 2 {
		t.Errorf("len(keys) = %d, want 2", len(keys))
	}
}

func TestGetChannelMessageCount(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)
	ctx := context.Background()

	// Add some messages
	db.Exec(`INSERT INTO channel_messages (channel_id, content) VALUES (1, 'msg1'), (1, 'msg2'), (1, 'msg3'), (2, 'msg4')`)

	count := service.getChannelMessageCount(ctx, 1)
	if count != 3 {
		t.Errorf("count = %d, want 3", count)
	}
}

func TestJoinChannel(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)
	ctx := context.Background()

	err := service.JoinChannel(ctx, 1, "test-session")
	if err != nil {
		t.Fatalf("JoinChannel() error = %v", err)
	}

	// Verify
	var cs ChannelSession
	if err := db.First(&cs).Error; err != nil {
		t.Fatalf("failed to find channel session: %v", err)
	}
	if cs.SessionKey != "test-session" {
		t.Errorf("SessionKey = %s, want test-session", cs.SessionKey)
	}
}

func TestLeaveChannel(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)
	ctx := context.Background()

	// Join first
	service.JoinChannel(ctx, 1, "test-session")

	// Leave
	err := service.LeaveChannel(ctx, 1, "test-session")
	if err != nil {
		t.Fatalf("LeaveChannel() error = %v", err)
	}

	// Verify removed
	var count int64
	db.Model(&ChannelSession{}).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 channel sessions, got %d", count)
	}
}

func TestRecordChannelAccess(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)
	ctx := context.Background()

	sessionKey := "test-session"
	userID := int64(1)

	err := service.RecordChannelAccess(ctx, 1, &sessionKey, &userID)
	if err != nil {
		t.Fatalf("RecordChannelAccess() error = %v", err)
	}

	// Verify
	var ca ChannelAccess
	if err := db.First(&ca).Error; err != nil {
		t.Fatalf("failed to find channel access: %v", err)
	}
	if ca.ChannelID != 1 {
		t.Errorf("ChannelID = %d, want 1", ca.ChannelID)
	}
}

func TestBatchGetTicketSessions(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)
	ctx := context.Background()

	// Create sessions
	ticketID1 := int64(100)
	ticketID2 := int64(200)

	db.Create(&session.Session{SessionKey: "sess-1", OrganizationID: 1, TicketID: &ticketID1, Status: "running", CreatedByID: 1})
	db.Create(&session.Session{SessionKey: "sess-2", OrganizationID: 1, TicketID: &ticketID1, Status: "terminated", CreatedByID: 1})
	db.Create(&session.Session{SessionKey: "sess-3", OrganizationID: 1, TicketID: &ticketID2, Status: "running", CreatedByID: 1})

	result, err := service.BatchGetTicketSessions(ctx, []int64{100, 200, 300})
	if err != nil {
		t.Fatalf("BatchGetTicketSessions() error = %v", err)
	}

	if len(result.TicketSessions[100]) != 2 {
		t.Errorf("ticket 100 sessions = %d, want 2", len(result.TicketSessions[100]))
	}
	if len(result.TicketSessions[200]) != 1 {
		t.Errorf("ticket 200 sessions = %d, want 1", len(result.TicketSessions[200]))
	}
	if len(result.TicketSessions[300]) != 0 {
		t.Errorf("ticket 300 sessions = %d, want 0", len(result.TicketSessions[300]))
	}
}

func TestErrorVariables(t *testing.T) {
	if ErrTicketNotFound == nil {
		t.Error("ErrTicketNotFound should not be nil")
	}
	if ErrRunnerNotFound == nil {
		t.Error("ErrRunnerNotFound should not be nil")
	}
}

// ChannelSession for testing (local type to avoid import cycle)
type ChannelSession struct {
	ID         int64  `gorm:"primaryKey"`
	ChannelID  int64  `gorm:"not null"`
	SessionKey string `gorm:"not null"`
}

func (ChannelSession) TableName() string {
	return "channel_sessions"
}

// ChannelAccess for testing
type ChannelAccess struct {
	ID         int64   `gorm:"primaryKey"`
	ChannelID  int64   `gorm:"not null"`
	SessionKey *string
	UserID     *int64
}

func (ChannelAccess) TableName() string {
	return "channel_access"
}

// Mock the channel.Message for count query
func init() {
	// Register table name mapping if needed
}

func TestSessionToNode_NilValues(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)

	// Test with minimal session (nil optional fields)
	sess := &session.Session{
		SessionKey:  "minimal-session",
		Status:      "pending",
		AgentStatus: "unknown",
		CreatedByID: 1,
	}

	node := service.sessionToNode(sess)

	if node.SessionKey != "minimal-session" {
		t.Errorf("SessionKey = %s, want minimal-session", node.SessionKey)
	}
	if node.Model != nil {
		t.Error("expected Model to be nil")
	}
	if node.TicketID != nil {
		t.Error("expected TicketID to be nil")
	}
	if node.RepositoryID != nil {
		t.Error("expected RepositoryID to be nil")
	}
}

func TestGetChannelSessions_Empty(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)
	ctx := context.Background()

	keys := service.getChannelSessions(ctx, 999)
	if len(keys) != 0 {
		t.Errorf("expected 0 keys for non-existent channel, got %d", len(keys))
	}
}

func TestGetChannelMessageCount_Empty(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)
	ctx := context.Background()

	count := service.getChannelMessageCount(ctx, 999)
	if count != 0 {
		t.Errorf("expected 0 messages for non-existent channel, got %d", count)
	}
}

func TestJoinChannel_Duplicate(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)
	ctx := context.Background()

	// Join twice - should create two records (no unique constraint in test)
	service.JoinChannel(ctx, 1, "test-session")
	err := service.JoinChannel(ctx, 1, "test-session")
	if err != nil {
		t.Fatalf("JoinChannel() duplicate error = %v", err)
	}

	var count int64
	db.Model(&ChannelSession{}).Where("channel_id = ?", 1).Count(&count)
	if count != 2 {
		t.Errorf("expected 2 channel sessions, got %d", count)
	}
}

func TestLeaveChannel_NotExists(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)
	ctx := context.Background()

	// Leave a channel we never joined - should not error
	err := service.LeaveChannel(ctx, 999, "nonexistent")
	if err != nil {
		t.Errorf("LeaveChannel() should not error for non-existent: %v", err)
	}
}

func TestRecordChannelAccess_NilSession(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)
	ctx := context.Background()

	userID := int64(1)
	err := service.RecordChannelAccess(ctx, 1, nil, &userID)
	if err != nil {
		t.Fatalf("RecordChannelAccess() error = %v", err)
	}

	var ca ChannelAccess
	db.First(&ca)
	if ca.SessionKey != nil {
		t.Error("expected SessionKey to be nil")
	}
	if ca.UserID == nil || *ca.UserID != 1 {
		t.Error("expected UserID to be 1")
	}
}

func TestRecordChannelAccess_NilUser(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)
	ctx := context.Background()

	sessionKey := "session-key"
	err := service.RecordChannelAccess(ctx, 2, &sessionKey, nil)
	if err != nil {
		t.Fatalf("RecordChannelAccess() error = %v", err)
	}

	var ca ChannelAccess
	db.Where("channel_id = ?", 2).First(&ca)
	if ca.SessionKey == nil || *ca.SessionKey != "session-key" {
		t.Error("expected SessionKey to be 'session-key'")
	}
	if ca.UserID != nil {
		t.Error("expected UserID to be nil")
	}
}

func TestBatchGetTicketSessions_Empty(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)
	ctx := context.Background()

	result, err := service.BatchGetTicketSessions(ctx, []int64{})
	if err != nil {
		t.Fatalf("BatchGetTicketSessions() error = %v", err)
	}
	if len(result.TicketSessions) != 0 {
		t.Errorf("expected 0 entries, got %d", len(result.TicketSessions))
	}
}

func TestBatchGetTicketSessions_NoSessions(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)
	ctx := context.Background()

	result, err := service.BatchGetTicketSessions(ctx, []int64{1, 2, 3})
	if err != nil {
		t.Fatalf("BatchGetTicketSessions() error = %v", err)
	}
	// Should have entries for all requested IDs even if empty
	for _, id := range []int64{1, 2, 3} {
		if _, ok := result.TicketSessions[id]; !ok {
			t.Errorf("expected entry for ticket ID %d", id)
		}
		if len(result.TicketSessions[id]) != 0 {
			t.Errorf("expected 0 sessions for ticket %d, got %d", id, len(result.TicketSessions[id]))
		}
	}
}

func TestBatchGetTicketSessions_SessionWithNilTicket(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)
	ctx := context.Background()

	// Create session with nil ticket_id
	db.Create(&session.Session{SessionKey: "no-ticket", OrganizationID: 1, Status: "running", CreatedByID: 1})

	result, err := service.BatchGetTicketSessions(ctx, []int64{100})
	if err != nil {
		t.Fatalf("BatchGetTicketSessions() error = %v", err)
	}
	// Session with nil ticket_id should not be included
	if len(result.TicketSessions[100]) != 0 {
		t.Errorf("expected 0 sessions for ticket 100, got %d", len(result.TicketSessions[100]))
	}
}

func TestServiceFields(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db, nil, nil, nil)

	// Verify nil services are accepted
	if service.sessionService != nil {
		t.Error("expected sessionService to be nil")
	}
	if service.channelService != nil {
		t.Error("expected channelService to be nil")
	}
	if service.bindingService != nil {
		t.Error("expected bindingService to be nil")
	}
}
