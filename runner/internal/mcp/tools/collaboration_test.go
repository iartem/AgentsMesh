package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// --- Test CollaborationMessage ---

func TestCollaborationMessageStruct(t *testing.T) {
	msg := CollaborationMessage{
		ID:      "msg-123",
		From:    "agent-1",
		To:      "agent-2",
		Type:    "request",
		Content: "Hello",
		Metadata: map[string]interface{}{
			"key": "value",
		},
		Read: false,
	}

	if msg.ID != "msg-123" {
		t.Errorf("ID: got %v, want msg-123", msg.ID)
	}

	if msg.From != "agent-1" {
		t.Errorf("From: got %v, want agent-1", msg.From)
	}

	if msg.To != "agent-2" {
		t.Errorf("To: got %v, want agent-2", msg.To)
	}
}

// --- Test CollaborationStore ---

func TestNewCollaborationStore(t *testing.T) {
	store := NewCollaborationStore("")

	if store == nil {
		t.Fatal("NewCollaborationStore returned nil")
	}

	if store.messages == nil {
		t.Error("messages should be initialized")
	}

	if store.state == nil {
		t.Error("state should be initialized")
	}
}

func TestNewCollaborationStoreWithPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	persistPath := filepath.Join(tmpDir, "collab.json")

	store := NewCollaborationStore(persistPath)

	if store.filePath != persistPath {
		t.Errorf("filePath: got %v, want %v", store.filePath, persistPath)
	}
}

func TestCollaborationStoreSendMessage(t *testing.T) {
	store := NewCollaborationStore("")

	id := store.SendMessage("agent-1", "agent-2", "request", "Hello", nil)

	if id == "" {
		t.Error("SendMessage should return an ID")
	}

	messages := store.GetMessages("agent-2", false)
	if len(messages) != 1 {
		t.Errorf("messages count: got %v, want 1", len(messages))
	}

	if messages[0].From != "agent-1" {
		t.Errorf("From: got %v, want agent-1", messages[0].From)
	}

	if messages[0].Content != "Hello" {
		t.Errorf("Content: got %v, want Hello", messages[0].Content)
	}
}

func TestCollaborationStoreGetMessagesBroadcast(t *testing.T) {
	store := NewCollaborationStore("")

	store.SendMessage("agent-1", "*", "info", "Broadcast message", nil)

	// Any agent should see broadcast messages
	messages := store.GetMessages("agent-2", false)
	if len(messages) != 1 {
		t.Errorf("messages count: got %v, want 1", len(messages))
	}

	if messages[0].To != "*" {
		t.Errorf("To: got %v, want *", messages[0].To)
	}
}

func TestCollaborationStoreGetMessagesUnreadOnly(t *testing.T) {
	store := NewCollaborationStore("")

	id1 := store.SendMessage("agent-1", "agent-2", "request", "Message 1", nil)
	// Small delay to ensure different timestamp for second message
	time.Sleep(1 * time.Millisecond)
	id2 := store.SendMessage("agent-1", "agent-2", "request", "Message 2", nil)

	// Verify both messages exist
	allMessages := store.GetMessages("agent-2", false)
	if len(allMessages) != 2 {
		t.Fatalf("all messages count: got %v, want 2", len(allMessages))
	}

	// Mark first message as read
	store.MarkAsRead([]string{id1})

	// Get unread only - should return only message 2
	unreadMessages := store.GetMessages("agent-2", true)
	if len(unreadMessages) != 1 {
		t.Fatalf("unread messages count: got %v, want 1 (id1=%s, id2=%s)", len(unreadMessages), id1, id2)
	}

	if unreadMessages[0].Content != "Message 2" {
		t.Errorf("Content: got %v, want Message 2", unreadMessages[0].Content)
	}

	if unreadMessages[0].ID != id2 {
		t.Errorf("ID: got %v, want %v", unreadMessages[0].ID, id2)
	}
}

func TestCollaborationStoreMarkAsRead(t *testing.T) {
	store := NewCollaborationStore("")

	id := store.SendMessage("agent-1", "agent-2", "request", "Hello", nil)

	// Verify message is unread
	messages := store.GetMessages("agent-2", true)
	if len(messages) != 1 {
		t.Errorf("unread messages before: got %v, want 1", len(messages))
	}

	// Mark as read
	store.MarkAsRead([]string{id})

	// Verify message is read
	messages = store.GetMessages("agent-2", true)
	if len(messages) != 0 {
		t.Errorf("unread messages after: got %v, want 0", len(messages))
	}
}

func TestCollaborationStoreState(t *testing.T) {
	store := NewCollaborationStore("")

	// Set state
	store.SetState("key1", "value1")
	store.SetState("key2", 123)

	// Get state
	value, ok := store.GetState("key1")
	if !ok {
		t.Error("key1 should exist")
	}
	if value != "value1" {
		t.Errorf("key1 value: got %v, want value1", value)
	}

	// Get non-existent key
	_, ok = store.GetState("nonexistent")
	if ok {
		t.Error("nonexistent should not exist")
	}
}

func TestCollaborationStoreGetAllState(t *testing.T) {
	store := NewCollaborationStore("")

	store.SetState("key1", "value1")
	store.SetState("key2", "value2")

	allState := store.GetAllState()

	if len(allState) != 2 {
		t.Errorf("allState length: got %v, want 2", len(allState))
	}

	if allState["key1"] != "value1" {
		t.Errorf("key1: got %v, want value1", allState["key1"])
	}
}

func TestCollaborationStorePersistence(t *testing.T) {
	tmpDir := t.TempDir()
	persistPath := filepath.Join(tmpDir, "collab.json")

	// Create store and add data
	store1 := NewCollaborationStore(persistPath)
	store1.SendMessage("agent-1", "agent-2", "request", "Hello", nil)
	store1.SetState("key", "value")

	// Create new store and verify data loaded
	store2 := NewCollaborationStore(persistPath)

	messages := store2.GetMessages("agent-2", false)
	if len(messages) != 1 {
		t.Errorf("persisted messages: got %v, want 1", len(messages))
	}

	value, ok := store2.GetState("key")
	if !ok || value != "value" {
		t.Errorf("persisted state: got %v, want value", value)
	}
}

func TestCollaborationStoreLoadNonExistent(t *testing.T) {
	tmpDir := t.TempDir()
	persistPath := filepath.Join(tmpDir, "nonexistent.json")

	// Should not panic or error
	store := NewCollaborationStore(persistPath)

	if len(store.messages) != 0 {
		t.Error("messages should be empty for non-existent file")
	}
}

func TestCollaborationStoreLoadInvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	persistPath := filepath.Join(tmpDir, "invalid.json")

	// Write invalid JSON
	os.WriteFile(persistPath, []byte("invalid json"), 0644)

	// Should not panic or error
	store := NewCollaborationStore(persistPath)

	if len(store.messages) != 0 {
		t.Error("messages should be empty for invalid JSON")
	}
}

// --- Test Collaboration Tools ---

func TestCollaborationTools(t *testing.T) {
	store := NewCollaborationStore("")
	tools := CollaborationTools(store, "test-agent")

	if len(tools) != 6 {
		t.Errorf("tools count: got %v, want 6", len(tools))
	}

	// Verify tool names
	expectedNames := []string{
		"send_message",
		"get_messages",
		"mark_messages_read",
		"set_shared_state",
		"get_shared_state",
		"broadcast",
	}

	toolNames := make(map[string]bool)
	for _, tool := range tools {
		toolNames[tool.Name] = true
	}

	for _, expected := range expectedNames {
		if !toolNames[expected] {
			t.Errorf("expected tool %s not found", expected)
		}
	}
}

func TestSendMessageTool(t *testing.T) {
	store := NewCollaborationStore("")
	tool := SendMessageTool(store, "agent-1")

	if tool.Name != "send_message" {
		t.Errorf("Name: got %v, want send_message", tool.Name)
	}

	if tool.Handler == nil {
		t.Error("Handler should not be nil")
	}

	// Test handler
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"to":      "agent-2",
		"type":    "request",
		"content": "Hello",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, ok := result.(*ToolResult)
	if !ok {
		t.Fatal("result should be *ToolResult")
	}

	if toolResult.IsError {
		t.Error("should not return error")
	}
}

func TestSendMessageToolMissingArgs(t *testing.T) {
	store := NewCollaborationStore("")
	tool := SendMessageTool(store, "agent-1")

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"to": "agent-2",
		// Missing content
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, _ := result.(*ToolResult)
	if !toolResult.IsError {
		t.Error("should return error for missing content")
	}
}

func TestGetMessagesTool(t *testing.T) {
	store := NewCollaborationStore("")
	store.SendMessage("agent-1", "agent-2", "request", "Hello", nil)

	tool := GetMessagesTool(store, "agent-2")

	result, err := tool.Handler(context.Background(), map[string]interface{}{})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Error("should not return error")
	}
}

func TestGetMessagesToolEmpty(t *testing.T) {
	store := NewCollaborationStore("")
	tool := GetMessagesTool(store, "agent-2")

	result, _ := tool.Handler(context.Background(), map[string]interface{}{})

	toolResult, _ := result.(*ToolResult)
	if toolResult.Content[0].Text != "No messages" {
		t.Errorf("Content: got %v, want 'No messages'", toolResult.Content[0].Text)
	}
}

func TestMarkMessagesReadTool(t *testing.T) {
	store := NewCollaborationStore("")
	id := store.SendMessage("agent-1", "agent-2", "request", "Hello", nil)

	tool := MarkMessagesReadTool(store)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"message_ids": []interface{}{id},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Error("should not return error")
	}
}

func TestMarkMessagesReadToolInvalidArg(t *testing.T) {
	store := NewCollaborationStore("")
	tool := MarkMessagesReadTool(store)

	result, _ := tool.Handler(context.Background(), map[string]interface{}{
		"message_ids": "not an array",
	})

	toolResult, _ := result.(*ToolResult)
	if !toolResult.IsError {
		t.Error("should return error for invalid argument")
	}
}

func TestSetSharedStateTool(t *testing.T) {
	store := NewCollaborationStore("")
	tool := SetSharedStateTool(store)

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"key":   "mykey",
		"value": "myvalue",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Error("should not return error")
	}

	// Verify state was set
	value, ok := store.GetState("mykey")
	if !ok || value != "myvalue" {
		t.Errorf("state not set correctly")
	}
}

func TestSetSharedStateToolMissingKey(t *testing.T) {
	store := NewCollaborationStore("")
	tool := SetSharedStateTool(store)

	result, _ := tool.Handler(context.Background(), map[string]interface{}{
		"value": "myvalue",
	})

	toolResult, _ := result.(*ToolResult)
	if !toolResult.IsError {
		t.Error("should return error for missing key")
	}
}

func TestGetSharedStateTool(t *testing.T) {
	store := NewCollaborationStore("")
	store.SetState("mykey", "myvalue")

	tool := GetSharedStateTool(store)

	// Get specific key
	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"key": "mykey",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Error("should not return error")
	}
}

func TestGetSharedStateToolNotFound(t *testing.T) {
	store := NewCollaborationStore("")
	tool := GetSharedStateTool(store)

	result, _ := tool.Handler(context.Background(), map[string]interface{}{
		"key": "nonexistent",
	})

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Error("should not return error for not found")
	}
}

func TestGetSharedStateToolAll(t *testing.T) {
	store := NewCollaborationStore("")
	store.SetState("key1", "value1")
	store.SetState("key2", "value2")

	tool := GetSharedStateTool(store)

	// Get all state (no key specified)
	result, _ := tool.Handler(context.Background(), map[string]interface{}{})

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Error("should not return error")
	}
}

func TestGetSharedStateToolEmpty(t *testing.T) {
	store := NewCollaborationStore("")
	tool := GetSharedStateTool(store)

	result, _ := tool.Handler(context.Background(), map[string]interface{}{})

	toolResult, _ := result.(*ToolResult)
	if toolResult.Content[0].Text != "No shared state" {
		t.Errorf("Content: got %v, want 'No shared state'", toolResult.Content[0].Text)
	}
}

func TestBroadcastTool(t *testing.T) {
	store := NewCollaborationStore("")
	tool := BroadcastTool(store, "agent-1")

	result, err := tool.Handler(context.Background(), map[string]interface{}{
		"type":    "info",
		"content": "Broadcast message",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	toolResult, _ := result.(*ToolResult)
	if toolResult.IsError {
		t.Error("should not return error")
	}

	// Verify broadcast was sent to "*"
	messages := store.GetMessages("agent-2", false)
	if len(messages) != 1 {
		t.Errorf("broadcast messages: got %v, want 1", len(messages))
	}

	if messages[0].To != "*" {
		t.Errorf("To: got %v, want *", messages[0].To)
	}
}

func TestBroadcastToolMissingContent(t *testing.T) {
	store := NewCollaborationStore("")
	tool := BroadcastTool(store, "agent-1")

	result, _ := tool.Handler(context.Background(), map[string]interface{}{
		"type": "info",
	})

	toolResult, _ := result.(*ToolResult)
	if !toolResult.IsError {
		t.Error("should return error for missing content")
	}
}

// --- Benchmark Tests ---

func BenchmarkCollaborationStoreSendMessage(b *testing.B) {
	store := NewCollaborationStore("")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.SendMessage("agent-1", "agent-2", "request", "Hello", nil)
	}
}

func BenchmarkCollaborationStoreGetMessages(b *testing.B) {
	store := NewCollaborationStore("")

	// Add some messages
	for i := 0; i < 100; i++ {
		store.SendMessage("agent-1", "agent-2", "request", "Hello", nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.GetMessages("agent-2", false)
	}
}

func BenchmarkCollaborationStoreSetState(b *testing.B) {
	store := NewCollaborationStore("")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.SetState("key", "value")
	}
}

func BenchmarkCollaborationStoreGetState(b *testing.B) {
	store := NewCollaborationStore("")
	store.SetState("key", "value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.GetState("key")
	}
}
