package agent

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/agentmesh/backend/internal/domain/agent"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func setupMessageTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}

	// Create agent_messages table
	db.Exec(`CREATE TABLE IF NOT EXISTS agent_messages (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		sender_session TEXT NOT NULL,
		receiver_session TEXT NOT NULL,
		message_type TEXT NOT NULL,
		content BLOB,
		status TEXT NOT NULL DEFAULT 'pending',
		correlation_id TEXT,
		parent_message_id INTEGER,
		delivered_at DATETIME,
		read_at DATETIME,
		max_retries INTEGER NOT NULL DEFAULT 3,
		delivery_attempts INTEGER NOT NULL DEFAULT 0,
		last_delivery_attempt DATETIME,
		next_retry_at DATETIME,
		delivery_error TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)

	// Create agent_message_dead_letters table
	db.Exec(`CREATE TABLE IF NOT EXISTS agent_message_dead_letters (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		original_message_id INTEGER NOT NULL,
		reason TEXT NOT NULL,
		final_attempt INTEGER NOT NULL,
		moved_at DATETIME NOT NULL,
		replayed_at DATETIME,
		replay_result TEXT,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	)`)

	return db
}

func TestNewMessageService(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)

	if svc == nil {
		t.Error("NewMessageService returned nil")
	}
	if svc.db != db {
		t.Error("Service db not set correctly")
	}
}

func TestSendMessage(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	t.Run("send basic message", func(t *testing.T) {
		content := agent.MessageContent{
			"text": "Hello, World!",
		}

		msg, err := svc.SendMessage(ctx, "session-sender", "session-receiver", "text", content, nil, nil)
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		if msg.SenderSession != "session-sender" {
			t.Errorf("SenderSession = %s, want session-sender", msg.SenderSession)
		}
		if msg.ReceiverSession != "session-receiver" {
			t.Errorf("ReceiverSession = %s, want session-receiver", msg.ReceiverSession)
		}
		if msg.MessageType != "text" {
			t.Errorf("MessageType = %s, want text", msg.MessageType)
		}
		if msg.Status != agent.MessageStatusPending {
			t.Errorf("Status = %s, want pending", msg.Status)
		}
		if msg.MaxRetries != 3 {
			t.Errorf("MaxRetries = %d, want 3", msg.MaxRetries)
		}
	})

	t.Run("send message with correlation ID", func(t *testing.T) {
		correlationID := "corr-123"
		content := agent.MessageContent{"data": "test"}

		msg, err := svc.SendMessage(ctx, "s1", "s2", "request", content, &correlationID, nil)
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}

		if msg.CorrelationID == nil || *msg.CorrelationID != correlationID {
			t.Error("CorrelationID not set correctly")
		}
	})

	t.Run("send reply message", func(t *testing.T) {
		// Send original message
		content := agent.MessageContent{"text": "original"}
		original, _ := svc.SendMessage(ctx, "s1", "s2", "text", content, nil, nil)

		// Send reply
		replyContent := agent.MessageContent{"text": "reply"}
		reply, err := svc.SendMessage(ctx, "s2", "s1", "text", replyContent, nil, &original.ID)
		if err != nil {
			t.Fatalf("SendMessage reply failed: %v", err)
		}

		if reply.ParentMessageID == nil || *reply.ParentMessageID != original.ID {
			t.Error("ParentMessageID not set correctly")
		}
	})
}

func TestGetMessage(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	content := agent.MessageContent{"text": "test"}
	created, _ := svc.SendMessage(ctx, "s1", "s2", "text", content, nil, nil)

	t.Run("get existing message", func(t *testing.T) {
		msg, err := svc.GetMessage(ctx, created.ID)
		if err != nil {
			t.Fatalf("GetMessage failed: %v", err)
		}
		if msg.ID != created.ID {
			t.Errorf("ID = %d, want %d", msg.ID, created.ID)
		}
	})

	t.Run("get non-existent message", func(t *testing.T) {
		_, err := svc.GetMessage(ctx, 99999)
		if err == nil {
			t.Error("Expected error for non-existent message")
		}
		if err != ErrMessageNotFound {
			t.Errorf("Expected ErrMessageNotFound, got %v", err)
		}
	})
}

func TestGetMessages(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	// Send multiple messages to the same receiver
	for i := 0; i < 5; i++ {
		content := agent.MessageContent{"index": i}
		svc.SendMessage(ctx, "sender", "receiver", "text", content, nil, nil)
	}

	t.Run("get all messages for receiver", func(t *testing.T) {
		messages, err := svc.GetMessages(ctx, "receiver", false, nil, 10, 0)
		if err != nil {
			t.Fatalf("GetMessages failed: %v", err)
		}
		if len(messages) != 5 {
			t.Errorf("Messages count = %d, want 5", len(messages))
		}
	})

	t.Run("get messages with limit", func(t *testing.T) {
		messages, err := svc.GetMessages(ctx, "receiver", false, nil, 3, 0)
		if err != nil {
			t.Fatalf("GetMessages failed: %v", err)
		}
		if len(messages) != 3 {
			t.Errorf("Messages count = %d, want 3", len(messages))
		}
	})

	t.Run("get messages with offset", func(t *testing.T) {
		messages, err := svc.GetMessages(ctx, "receiver", false, nil, 10, 2)
		if err != nil {
			t.Fatalf("GetMessages failed: %v", err)
		}
		if len(messages) != 3 {
			t.Errorf("Messages count = %d, want 3", len(messages))
		}
	})

	t.Run("get unread messages only", func(t *testing.T) {
		messages, err := svc.GetMessages(ctx, "receiver", true, nil, 10, 0)
		if err != nil {
			t.Fatalf("GetMessages failed: %v", err)
		}
		// All should be pending (unread)
		if len(messages) != 5 {
			t.Errorf("Messages count = %d, want 5", len(messages))
		}
	})

	t.Run("get messages with type filter", func(t *testing.T) {
		// Send a different type message
		svc.SendMessage(ctx, "sender", "receiver", "command", agent.MessageContent{}, nil, nil)

		messages, err := svc.GetMessages(ctx, "receiver", false, []string{"command"}, 10, 0)
		if err != nil {
			t.Fatalf("GetMessages failed: %v", err)
		}
		if len(messages) != 1 {
			t.Errorf("Messages count = %d, want 1", len(messages))
		}
	})
}

func TestGetUnreadMessages(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	// Send messages
	for i := 0; i < 3; i++ {
		svc.SendMessage(ctx, "sender", "receiver", "text", agent.MessageContent{}, nil, nil)
	}

	messages, err := svc.GetUnreadMessages(ctx, "receiver", 10)
	if err != nil {
		t.Fatalf("GetUnreadMessages failed: %v", err)
	}
	if len(messages) != 3 {
		t.Errorf("Messages count = %d, want 3", len(messages))
	}
}

func TestMarkRead(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	msg, _ := svc.SendMessage(ctx, "sender", "receiver", "text", agent.MessageContent{}, nil, nil)

	t.Run("mark message as read", func(t *testing.T) {
		err := svc.MarkRead(ctx, msg.ID, "receiver")
		if err != nil {
			t.Fatalf("MarkRead failed: %v", err)
		}

		updated, _ := svc.GetMessage(ctx, msg.ID)
		if updated.Status != agent.MessageStatusRead {
			t.Errorf("Status = %s, want read", updated.Status)
		}
		if updated.ReadAt == nil {
			t.Error("ReadAt should be set")
		}
	})

	t.Run("unauthorized mark read", func(t *testing.T) {
		msg2, _ := svc.SendMessage(ctx, "sender", "receiver", "text", agent.MessageContent{}, nil, nil)
		err := svc.MarkRead(ctx, msg2.ID, "other-session")
		if err == nil {
			t.Error("Expected error for unauthorized mark read")
		}
		if err != ErrNotAuthorized {
			t.Errorf("Expected ErrNotAuthorized, got %v", err)
		}
	})

	t.Run("mark non-existent message", func(t *testing.T) {
		err := svc.MarkRead(ctx, 99999, "receiver")
		if err == nil {
			t.Error("Expected error for non-existent message")
		}
	})
}

func TestMarkDelivered(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	msg, _ := svc.SendMessage(ctx, "sender", "receiver", "text", agent.MessageContent{}, nil, nil)

	err := svc.MarkDelivered(ctx, msg.ID)
	if err != nil {
		t.Fatalf("MarkDelivered failed: %v", err)
	}

	updated, _ := svc.GetMessage(ctx, msg.ID)
	if updated.Status != agent.MessageStatusDelivered {
		t.Errorf("Status = %s, want delivered", updated.Status)
	}
	if updated.DeliveredAt == nil {
		t.Error("DeliveredAt should be set")
	}
}

func TestMarkAllRead(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	// Send multiple messages
	for i := 0; i < 5; i++ {
		svc.SendMessage(ctx, "sender", "receiver", "text", agent.MessageContent{}, nil, nil)
	}

	affected, err := svc.MarkAllRead(ctx, "receiver")
	if err != nil {
		t.Fatalf("MarkAllRead failed: %v", err)
	}
	if affected != 5 {
		t.Errorf("Affected rows = %d, want 5", affected)
	}

	// Verify all are read
	messages, _ := svc.GetUnreadMessages(ctx, "receiver", 10)
	if len(messages) != 0 {
		t.Errorf("Unread count = %d, want 0", len(messages))
	}
}

func TestGetUnreadCount(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		svc.SendMessage(ctx, "sender", "receiver", "text", agent.MessageContent{}, nil, nil)
	}

	count, err := svc.GetUnreadCount(ctx, "receiver")
	if err != nil {
		t.Fatalf("GetUnreadCount failed: %v", err)
	}
	if count != 3 {
		t.Errorf("Count = %d, want 3", count)
	}
}

func TestGetConversation(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	correlationID := "conv-123"

	// Send messages with same correlation ID
	for i := 0; i < 4; i++ {
		svc.SendMessage(ctx, "s1", "s2", "text", agent.MessageContent{}, &correlationID, nil)
	}

	// Send message with different correlation ID
	otherCorr := "other-conv"
	svc.SendMessage(ctx, "s1", "s2", "text", agent.MessageContent{}, &otherCorr, nil)

	messages, err := svc.GetConversation(ctx, correlationID, 10)
	if err != nil {
		t.Fatalf("GetConversation failed: %v", err)
	}
	if len(messages) != 4 {
		t.Errorf("Messages count = %d, want 4", len(messages))
	}
}

func TestGetThread(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	// Send root message
	root, _ := svc.SendMessage(ctx, "s1", "s2", "text", agent.MessageContent{}, nil, nil)

	// Send replies
	for i := 0; i < 3; i++ {
		svc.SendMessage(ctx, "s2", "s1", "text", agent.MessageContent{}, nil, &root.ID)
	}

	t.Run("get thread", func(t *testing.T) {
		thread, err := svc.GetThread(ctx, root.ID)
		if err != nil {
			t.Fatalf("GetThread failed: %v", err)
		}
		// Should have root + 3 replies
		if len(thread) != 4 {
			t.Errorf("Thread length = %d, want 4", len(thread))
		}
	})

	t.Run("get thread for non-existent message", func(t *testing.T) {
		_, err := svc.GetThread(ctx, 99999)
		if err == nil {
			t.Error("Expected error for non-existent message")
		}
	})
}

func TestDeleteMessage(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	msg, _ := svc.SendMessage(ctx, "sender", "receiver", "text", agent.MessageContent{}, nil, nil)

	t.Run("sender can delete", func(t *testing.T) {
		err := svc.DeleteMessage(ctx, msg.ID, "sender")
		if err != nil {
			t.Fatalf("DeleteMessage failed: %v", err)
		}

		// Verify deleted
		_, err = svc.GetMessage(ctx, msg.ID)
		if err == nil {
			t.Error("Message should be deleted")
		}
	})

	t.Run("receiver cannot delete", func(t *testing.T) {
		msg2, _ := svc.SendMessage(ctx, "sender", "receiver", "text", agent.MessageContent{}, nil, nil)
		err := svc.DeleteMessage(ctx, msg2.ID, "receiver")
		if err == nil {
			t.Error("Expected error for unauthorized delete")
		}
		if err != ErrNotAuthorized {
			t.Errorf("Expected ErrNotAuthorized, got %v", err)
		}
	})

	t.Run("delete non-existent message", func(t *testing.T) {
		err := svc.DeleteMessage(ctx, 99999, "sender")
		if err == nil {
			t.Error("Expected error for non-existent message")
		}
	})
}

func TestGetSentMessages(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	// Send multiple messages from same sender
	for i := 0; i < 4; i++ {
		svc.SendMessage(ctx, "sender", "receiver", "text", agent.MessageContent{}, nil, nil)
	}

	messages, err := svc.GetSentMessages(ctx, "sender", 10, 0)
	if err != nil {
		t.Fatalf("GetSentMessages failed: %v", err)
	}
	if len(messages) != 4 {
		t.Errorf("Messages count = %d, want 4", len(messages))
	}
}

func TestGetMessagesBetween(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	// Send messages in both directions
	svc.SendMessage(ctx, "alice", "bob", "text", agent.MessageContent{}, nil, nil)
	svc.SendMessage(ctx, "bob", "alice", "text", agent.MessageContent{}, nil, nil)
	svc.SendMessage(ctx, "alice", "bob", "text", agent.MessageContent{}, nil, nil)

	// Send message to another user
	svc.SendMessage(ctx, "alice", "charlie", "text", agent.MessageContent{}, nil, nil)

	messages, err := svc.GetMessagesBetween(ctx, "alice", "bob", 10)
	if err != nil {
		t.Fatalf("GetMessagesBetween failed: %v", err)
	}
	if len(messages) != 3 {
		t.Errorf("Messages count = %d, want 3", len(messages))
	}
}

func TestGetPendingRetries(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	// Create a message with failed status and next_retry_at
	msg, _ := svc.SendMessage(ctx, "s1", "s2", "text", agent.MessageContent{}, nil, nil)

	// Update to failed status with next_retry_at
	nextRetry := time.Now().Add(-1 * time.Hour)
	db.Model(&agent.AgentMessage{}).Where("id = ?", msg.ID).Updates(map[string]interface{}{
		"status":        agent.MessageStatusFailed,
		"next_retry_at": nextRetry,
	})

	messages, err := svc.GetPendingRetries(ctx, time.Now(), 10)
	if err != nil {
		t.Fatalf("GetPendingRetries failed: %v", err)
	}
	if len(messages) != 1 {
		t.Errorf("Messages count = %d, want 1", len(messages))
	}
}

func TestRecordDeliveryFailure(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	t.Run("first failure schedules retry", func(t *testing.T) {
		msg, _ := svc.SendMessage(ctx, "s1", "s2", "text", agent.MessageContent{}, nil, nil)

		err := svc.RecordDeliveryFailure(ctx, msg.ID, "Connection refused")
		if err != nil {
			t.Fatalf("RecordDeliveryFailure failed: %v", err)
		}

		updated, _ := svc.GetMessage(ctx, msg.ID)
		if updated.Status != agent.MessageStatusFailed {
			t.Errorf("Status = %s, want failed", updated.Status)
		}
		if updated.DeliveryAttempts != 1 {
			t.Errorf("DeliveryAttempts = %d, want 1", updated.DeliveryAttempts)
		}
		if updated.NextRetryAt == nil {
			t.Error("NextRetryAt should be set")
		}
	})

	t.Run("max retries moves to dead letter", func(t *testing.T) {
		msg, _ := svc.SendMessage(ctx, "s1", "s2", "text", agent.MessageContent{}, nil, nil)
		// Set attempts to just below max
		db.Model(&agent.AgentMessage{}).Where("id = ?", msg.ID).Update("delivery_attempts", 2)

		err := svc.RecordDeliveryFailure(ctx, msg.ID, "Max retries exceeded")
		if err != nil {
			t.Fatalf("RecordDeliveryFailure failed: %v", err)
		}

		updated, _ := svc.GetMessage(ctx, msg.ID)
		if updated.Status != agent.MessageStatusDeadLetter {
			t.Errorf("Status = %s, want dead_letter", updated.Status)
		}
		if updated.NextRetryAt != nil {
			t.Error("NextRetryAt should be nil for dead letter")
		}

		// Verify dead letter entry created
		var deadLetter agent.DeadLetterEntry
		err = db.Where("original_message_id = ?", msg.ID).First(&deadLetter).Error
		if err != nil {
			t.Error("Dead letter entry should be created")
		}
	})

	t.Run("non-existent message", func(t *testing.T) {
		err := svc.RecordDeliveryFailure(ctx, 99999, "Error")
		if err == nil {
			t.Error("Expected error for non-existent message")
		}
	})
}

func TestGetDeadLetters(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	// Create messages and move to dead letter
	for i := 0; i < 3; i++ {
		msg, _ := svc.SendMessage(ctx, "s1", "s2", "text", agent.MessageContent{}, nil, nil)
		db.Model(&agent.AgentMessage{}).Where("id = ?", msg.ID).Update("delivery_attempts", 2)
		svc.RecordDeliveryFailure(ctx, msg.ID, "Failed")
	}

	entries, err := svc.GetDeadLetters(ctx, 10, 0)
	if err != nil {
		t.Fatalf("GetDeadLetters failed: %v", err)
	}
	if len(entries) != 3 {
		t.Errorf("Entries count = %d, want 3", len(entries))
	}
}

func TestReplayDeadLetter(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	// Create message and move to dead letter
	msg, _ := svc.SendMessage(ctx, "s1", "s2", "text", agent.MessageContent{}, nil, nil)
	db.Model(&agent.AgentMessage{}).Where("id = ?", msg.ID).Update("delivery_attempts", 2)
	svc.RecordDeliveryFailure(ctx, msg.ID, "Failed")

	// Get the dead letter entry
	var entry agent.DeadLetterEntry
	db.Where("original_message_id = ?", msg.ID).First(&entry)

	t.Run("replay dead letter", func(t *testing.T) {
		replayed, err := svc.ReplayDeadLetter(ctx, entry.ID)
		if err != nil {
			t.Fatalf("ReplayDeadLetter failed: %v", err)
		}

		if replayed.Status != agent.MessageStatusPending {
			t.Errorf("Status = %s, want pending", replayed.Status)
		}
		if replayed.DeliveryAttempts != 0 {
			t.Errorf("DeliveryAttempts = %d, want 0", replayed.DeliveryAttempts)
		}

		// Verify entry updated
		var updatedEntry agent.DeadLetterEntry
		db.First(&updatedEntry, entry.ID)
		if updatedEntry.ReplayedAt == nil {
			t.Error("ReplayedAt should be set")
		}
		if updatedEntry.ReplayResult == nil || *updatedEntry.ReplayResult != "Replayed successfully" {
			t.Error("ReplayResult should be set")
		}
	})

	t.Run("replay non-existent entry", func(t *testing.T) {
		_, err := svc.ReplayDeadLetter(ctx, 99999)
		if err == nil {
			t.Error("Expected error for non-existent entry")
		}
	})
}

func TestCleanupExpiredMessages(t *testing.T) {
	db := setupMessageTestDB(t)
	svc := NewMessageService(db)
	ctx := context.Background()

	// Create dead letter entries
	for i := 0; i < 3; i++ {
		msg, _ := svc.SendMessage(ctx, "s1", "s2", "text", agent.MessageContent{}, nil, nil)
		db.Model(&agent.AgentMessage{}).Where("id = ?", msg.ID).Update("delivery_attempts", 2)
		svc.RecordDeliveryFailure(ctx, msg.ID, "Failed")
	}

	// Set all to old date
	oldDate := time.Now().Add(-30 * 24 * time.Hour)
	db.Model(&agent.DeadLetterEntry{}).Where("1=1").Update("moved_at", oldDate)

	affected, err := svc.CleanupExpiredMessages(ctx, time.Now().Add(-7*24*time.Hour))
	if err != nil {
		t.Fatalf("CleanupExpiredMessages failed: %v", err)
	}
	if affected != 3 {
		t.Errorf("Affected rows = %d, want 3", affected)
	}

	// Verify all cleaned up
	entries, _ := svc.GetDeadLetters(ctx, 10, 0)
	if len(entries) != 0 {
		t.Errorf("Entries remaining = %d, want 0", len(entries))
	}
}

func TestMessageServiceErrors(t *testing.T) {
	tests := []struct {
		err      error
		expected string
	}{
		{ErrMessageNotFound, "message not found"},
		{ErrNotAuthorized, "not authorized to access this message"},
	}

	for _, tt := range tests {
		if tt.err.Error() != tt.expected {
			t.Errorf("Error message = %s, want %s", tt.err.Error(), tt.expected)
		}
	}
}
