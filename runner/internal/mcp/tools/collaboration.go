package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// CollaborationMessage represents a message between agents.
type CollaborationMessage struct {
	ID        string                 `json:"id"`
	From      string                 `json:"from"`
	To        string                 `json:"to"`
	Type      string                 `json:"type"`
	Content   string                 `json:"content"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	Timestamp time.Time              `json:"timestamp"`
	Read      bool                   `json:"read"`
}

// CollaborationStore provides shared state between agents.
type CollaborationStore struct {
	messages []CollaborationMessage
	state    map[string]interface{}
	mu       sync.RWMutex
	filePath string // Optional persistence path
}

// NewCollaborationStore creates a new collaboration store.
func NewCollaborationStore(persistPath string) *CollaborationStore {
	store := &CollaborationStore{
		messages: make([]CollaborationMessage, 0),
		state:    make(map[string]interface{}),
		filePath: persistPath,
	}

	// Load persisted state if available
	if persistPath != "" {
		store.load()
	}

	return store
}

// SendMessage sends a message to another agent.
func (s *CollaborationStore) SendMessage(from, to, msgType, content string, metadata map[string]interface{}) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := fmt.Sprintf("%d-%s", time.Now().UnixNano(), from)

	msg := CollaborationMessage{
		ID:        id,
		From:      from,
		To:        to,
		Type:      msgType,
		Content:   content,
		Metadata:  metadata,
		Timestamp: time.Now(),
		Read:      false,
	}

	s.messages = append(s.messages, msg)
	s.persist()

	return id
}

// GetMessages retrieves messages for a specific agent.
func (s *CollaborationStore) GetMessages(agentID string, unreadOnly bool) []CollaborationMessage {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []CollaborationMessage
	for _, msg := range s.messages {
		if msg.To == agentID || msg.To == "*" { // "*" is broadcast
			if !unreadOnly || !msg.Read {
				result = append(result, msg)
			}
		}
	}

	return result
}

// MarkAsRead marks messages as read.
func (s *CollaborationStore) MarkAsRead(messageIDs []string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	idSet := make(map[string]bool)
	for _, id := range messageIDs {
		idSet[id] = true
	}

	for i := range s.messages {
		if idSet[s.messages[i].ID] {
			s.messages[i].Read = true
		}
	}

	s.persist()
}

// SetState sets a shared state value.
func (s *CollaborationStore) SetState(key string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.state[key] = value
	s.persist()
}

// GetState gets a shared state value.
func (s *CollaborationStore) GetState(key string) (interface{}, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	value, ok := s.state[key]
	return value, ok
}

// GetAllState returns all shared state.
func (s *CollaborationStore) GetAllState() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]interface{})
	for k, v := range s.state {
		result[k] = v
	}
	return result
}

// persist saves state to disk.
func (s *CollaborationStore) persist() {
	if s.filePath == "" {
		return
	}

	data := struct {
		Messages []CollaborationMessage `json:"messages"`
		State    map[string]interface{} `json:"state"`
	}{
		Messages: s.messages,
		State:    s.state,
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}

	dir := filepath.Dir(s.filePath)
	os.MkdirAll(dir, 0755)
	os.WriteFile(s.filePath, bytes, 0644)
}

// load loads state from disk.
func (s *CollaborationStore) load() {
	if s.filePath == "" {
		return
	}

	data, err := os.ReadFile(s.filePath)
	if err != nil {
		return
	}

	var loaded struct {
		Messages []CollaborationMessage `json:"messages"`
		State    map[string]interface{} `json:"state"`
	}

	if err := json.Unmarshal(data, &loaded); err != nil {
		return
	}

	s.messages = loaded.Messages
	if loaded.State != nil {
		s.state = loaded.State
	}
}

// CollaborationTools returns tools for agent collaboration.
func CollaborationTools(store *CollaborationStore, agentID string) []*Tool {
	return []*Tool{
		SendMessageTool(store, agentID),
		GetMessagesTool(store, agentID),
		MarkMessagesReadTool(store),
		SetSharedStateTool(store),
		GetSharedStateTool(store),
		BroadcastTool(store, agentID),
	}
}

// SendMessageTool creates a tool for sending messages to other agents.
func SendMessageTool(store *CollaborationStore, senderID string) *Tool {
	return &Tool{
		Name:        "send_message",
		Description: "Send a message to another agent for collaboration",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"to": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the recipient agent",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"description": "The type of message (e.g., 'request', 'response', 'info')",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The message content",
				},
			},
			"required": []string{"to", "type", "content"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			to, _ := args["to"].(string)
			msgType, _ := args["type"].(string)
			content, _ := args["content"].(string)

			if to == "" || content == "" {
				return NewErrorResult(fmt.Errorf("to and content are required")), nil
			}

			id := store.SendMessage(senderID, to, msgType, content, nil)
			return NewTextResult(fmt.Sprintf("Message sent with ID: %s", id)), nil
		},
	}
}

// GetMessagesTool creates a tool for retrieving messages.
func GetMessagesTool(store *CollaborationStore, agentID string) *Tool {
	return &Tool{
		Name:        "get_messages",
		Description: "Get messages sent to this agent",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"unread_only": map[string]interface{}{
					"type":        "boolean",
					"description": "Only return unread messages (default: false)",
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			unreadOnly := false
			if v, ok := args["unread_only"].(bool); ok {
				unreadOnly = v
			}

			messages := store.GetMessages(agentID, unreadOnly)

			if len(messages) == 0 {
				return NewTextResult("No messages"), nil
			}

			data, _ := json.MarshalIndent(messages, "", "  ")
			return NewTextResult(string(data)), nil
		},
	}
}

// MarkMessagesReadTool creates a tool for marking messages as read.
func MarkMessagesReadTool(store *CollaborationStore) *Tool {
	return &Tool{
		Name:        "mark_messages_read",
		Description: "Mark messages as read by their IDs",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message_ids": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "List of message IDs to mark as read",
				},
			},
			"required": []string{"message_ids"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			idsRaw, ok := args["message_ids"].([]interface{})
			if !ok {
				return NewErrorResult(fmt.Errorf("message_ids must be an array")), nil
			}

			var ids []string
			for _, id := range idsRaw {
				if s, ok := id.(string); ok {
					ids = append(ids, s)
				}
			}

			store.MarkAsRead(ids)
			return NewTextResult(fmt.Sprintf("Marked %d messages as read", len(ids))), nil
		},
	}
}

// SetSharedStateTool creates a tool for setting shared state.
func SetSharedStateTool(store *CollaborationStore) *Tool {
	return &Tool{
		Name:        "set_shared_state",
		Description: "Set a shared state value that other agents can access",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key": map[string]interface{}{
					"type":        "string",
					"description": "The key for the state value",
				},
				"value": map[string]interface{}{
					"description": "The value to store (any JSON-serializable value)",
				},
			},
			"required": []string{"key", "value"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			key, _ := args["key"].(string)
			value := args["value"]

			if key == "" {
				return NewErrorResult(fmt.Errorf("key is required")), nil
			}

			store.SetState(key, value)
			return NewTextResult(fmt.Sprintf("State '%s' updated", key)), nil
		},
	}
}

// GetSharedStateTool creates a tool for getting shared state.
func GetSharedStateTool(store *CollaborationStore) *Tool {
	return &Tool{
		Name:        "get_shared_state",
		Description: "Get a shared state value or all state",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"key": map[string]interface{}{
					"type":        "string",
					"description": "The key to retrieve (optional, omit to get all state)",
				},
			},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			key, hasKey := args["key"].(string)

			if hasKey && key != "" {
				value, ok := store.GetState(key)
				if !ok {
					return NewTextResult(fmt.Sprintf("No state found for key '%s'", key)), nil
				}
				data, _ := json.MarshalIndent(value, "", "  ")
				return NewTextResult(string(data)), nil
			}

			// Return all state
			allState := store.GetAllState()
			if len(allState) == 0 {
				return NewTextResult("No shared state"), nil
			}

			data, _ := json.MarshalIndent(allState, "", "  ")
			return NewTextResult(string(data)), nil
		},
	}
}

// BroadcastTool creates a tool for broadcasting messages to all agents.
func BroadcastTool(store *CollaborationStore, senderID string) *Tool {
	return &Tool{
		Name:        "broadcast",
		Description: "Broadcast a message to all agents",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"type": map[string]interface{}{
					"type":        "string",
					"description": "The type of message",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The message content",
				},
			},
			"required": []string{"type", "content"},
		},
		Handler: func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
			msgType, _ := args["type"].(string)
			content, _ := args["content"].(string)

			if content == "" {
				return NewErrorResult(fmt.Errorf("content is required")), nil
			}

			id := store.SendMessage(senderID, "*", msgType, content, nil)
			return NewTextResult(fmt.Sprintf("Broadcast sent with ID: %s", id)), nil
		},
	}
}
