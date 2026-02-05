package channel

import (
	"testing"
	"time"
)

// --- Test MessageMetadata ---

func TestMessageMetadataScanNil(t *testing.T) {
	var mm MessageMetadata
	err := mm.Scan(nil)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if mm != nil {
		t.Error("expected nil MessageMetadata")
	}
}

func TestMessageMetadataScanValid(t *testing.T) {
	var mm MessageMetadata
	err := mm.Scan([]byte(`{"language":"go","filename":"main.go"}`))
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if mm["language"] != "go" {
		t.Errorf("expected language 'go', got %v", mm["language"])
	}
}

func TestMessageMetadataScanInvalidType(t *testing.T) {
	var mm MessageMetadata
	err := mm.Scan("not bytes")
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestMessageMetadataValueNil(t *testing.T) {
	var mm MessageMetadata
	val, err := mm.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val != nil {
		t.Error("expected nil value")
	}
}

func TestMessageMetadataValueValid(t *testing.T) {
	mm := MessageMetadata{"language": "go"}
	val, err := mm.Value()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if val == nil {
		t.Error("expected non-nil value")
	}
}

// --- Test Message ---

func TestMessageTableName(t *testing.T) {
	m := Message{}
	if m.TableName() != "channel_messages" {
		t.Errorf("expected 'channel_messages', got %s", m.TableName())
	}
}

func TestMessageStruct(t *testing.T) {
	now := time.Now()
	senderPod := "pod-sender"
	senderUserID := int64(50)

	m := Message{
		ID:           1,
		ChannelID:    10,
		SenderPod:    &senderPod,
		SenderUserID: &senderUserID,
		MessageType:  MessageTypeText,
		Content:      "Hello, world!",
		Metadata:     MessageMetadata{"mention": "@user"},
		CreatedAt:    now,
	}

	if m.ID != 1 {
		t.Errorf("expected ID 1, got %d", m.ID)
	}
	if m.ChannelID != 10 {
		t.Errorf("expected ChannelID 10, got %d", m.ChannelID)
	}
	if m.Content != "Hello, world!" {
		t.Errorf("expected Content 'Hello, world!', got %s", m.Content)
	}
	if m.MessageType != "text" {
		t.Errorf("expected MessageType 'text', got %s", m.MessageType)
	}
}

// --- Test Benchmark ---

func BenchmarkMessageMetadataScan(b *testing.B) {
	data := []byte(`{"language":"go","filename":"main.go"}`)
	for i := 0; i < b.N; i++ {
		var mm MessageMetadata
		mm.Scan(data)
	}
}
