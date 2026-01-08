package channel

import (
	"testing"
	"time"

	"github.com/lib/pq"
)

// --- Test Channel ---

func TestChannelTableName(t *testing.T) {
	c := Channel{}
	if c.TableName() != "channels" {
		t.Errorf("expected 'channels', got %s", c.TableName())
	}
}

func TestChannelStruct(t *testing.T) {
	now := time.Now()
	desc := "Test channel"
	doc := "Shared doc content"
	teamID := int64(10)
	repoID := int64(5)
	ticketID := int64(20)
	createdBySession := "sess-123"
	createdByUserID := int64(50)

	c := Channel{
		ID:               1,
		OrganizationID:   100,
		TeamID:           &teamID,
		Name:             "Development",
		Description:      &desc,
		Document:         &doc,
		RepositoryID:     &repoID,
		TicketID:         &ticketID,
		CreatedBySession: &createdBySession,
		CreatedByUserID:  &createdByUserID,
		IsArchived:       false,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if c.ID != 1 {
		t.Errorf("expected ID 1, got %d", c.ID)
	}
	if c.Name != "Development" {
		t.Errorf("expected Name 'Development', got %s", c.Name)
	}
	if *c.Description != "Test channel" {
		t.Errorf("expected Description 'Test channel', got %s", *c.Description)
	}
}

// --- Test Message Type Constants ---

func TestMessageTypeConstants(t *testing.T) {
	tests := []struct {
		constant string
		expected string
	}{
		{MessageTypeText, "text"},
		{MessageTypeSystem, "system"},
		{MessageTypeCode, "code"},
		{MessageTypeCommand, "command"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
		}
	}
}

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
	senderSession := "sess-sender"
	senderUserID := int64(50)

	m := Message{
		ID:            1,
		ChannelID:     10,
		SenderSession: &senderSession,
		SenderUserID:  &senderUserID,
		MessageType:   MessageTypeText,
		Content:       "Hello, world!",
		Metadata:      MessageMetadata{"mention": "@user"},
		CreatedAt:     now,
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

// --- Test Binding Status Constants ---

func TestBindingStatusConstants(t *testing.T) {
	tests := []struct {
		constant string
		expected string
	}{
		{BindingStatusPending, "pending"},
		{BindingStatusActive, "active"},
		{BindingStatusRejected, "rejected"},
		{BindingStatusInactive, "inactive"},
		{BindingStatusExpired, "expired"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
		}
	}
}

func TestBindingStatusAliases(t *testing.T) {
	if BindingStatusApproved != BindingStatusActive {
		t.Error("BindingStatusApproved should equal BindingStatusActive")
	}
	if BindingStatusRevoked != BindingStatusInactive {
		t.Error("BindingStatusRevoked should equal BindingStatusInactive")
	}
}

// --- Test Binding Scope Constants ---

func TestBindingScopeConstants(t *testing.T) {
	if BindingScopeTerminalRead != "terminal:read" {
		t.Errorf("expected 'terminal:read', got %s", BindingScopeTerminalRead)
	}
	if BindingScopeTerminalWrite != "terminal:write" {
		t.Errorf("expected 'terminal:write', got %s", BindingScopeTerminalWrite)
	}
}

func TestValidBindingScopes(t *testing.T) {
	if !ValidBindingScopes[BindingScopeTerminalRead] {
		t.Error("expected terminal:read to be valid")
	}
	if !ValidBindingScopes[BindingScopeTerminalWrite] {
		t.Error("expected terminal:write to be valid")
	}
	if ValidBindingScopes["invalid:scope"] {
		t.Error("expected invalid:scope to be invalid")
	}
}

// --- Test Binding Policy Constants ---

func TestBindingPolicyConstants(t *testing.T) {
	tests := []struct {
		constant string
		expected string
	}{
		{BindingPolicySameUserAuto, "same_user_auto"},
		{BindingPolicySameProjectAuto, "same_project_auto"},
		{BindingPolicyExplicitOnly, "explicit_only"},
	}

	for _, tt := range tests {
		if tt.constant != tt.expected {
			t.Errorf("expected '%s', got '%s'", tt.expected, tt.constant)
		}
	}
}

// --- Test SessionBinding ---

func TestSessionBindingTableName(t *testing.T) {
	sb := SessionBinding{}
	if sb.TableName() != "session_bindings" {
		t.Errorf("expected 'session_bindings', got %s", sb.TableName())
	}
}

func TestSessionBindingHasScope(t *testing.T) {
	sb := &SessionBinding{
		GrantedScopes: pq.StringArray{BindingScopeTerminalRead, BindingScopeTerminalWrite},
	}

	if !sb.HasScope(BindingScopeTerminalRead) {
		t.Error("expected HasScope(terminal:read) = true")
	}
	if !sb.HasScope(BindingScopeTerminalWrite) {
		t.Error("expected HasScope(terminal:write) = true")
	}
	if sb.HasScope("invalid:scope") {
		t.Error("expected HasScope(invalid:scope) = false")
	}
}

func TestSessionBindingHasPendingScope(t *testing.T) {
	sb := &SessionBinding{
		PendingScopes: pq.StringArray{BindingScopeTerminalWrite},
	}

	if !sb.HasPendingScope(BindingScopeTerminalWrite) {
		t.Error("expected HasPendingScope(terminal:write) = true")
	}
	if sb.HasPendingScope(BindingScopeTerminalRead) {
		t.Error("expected HasPendingScope(terminal:read) = false")
	}
}

func TestSessionBindingIsActive(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{BindingStatusActive, true},
		{BindingStatusPending, false},
		{BindingStatusRejected, false},
		{BindingStatusInactive, false},
	}

	for _, tt := range tests {
		sb := &SessionBinding{Status: tt.status}
		if sb.IsActive() != tt.expected {
			t.Errorf("status %s: expected IsActive() = %v", tt.status, tt.expected)
		}
	}
}

func TestSessionBindingIsPending(t *testing.T) {
	tests := []struct {
		status   string
		expected bool
	}{
		{BindingStatusPending, true},
		{BindingStatusActive, false},
		{BindingStatusRejected, false},
	}

	for _, tt := range tests {
		sb := &SessionBinding{Status: tt.status}
		if sb.IsPending() != tt.expected {
			t.Errorf("status %s: expected IsPending() = %v", tt.status, tt.expected)
		}
	}
}

func TestSessionBindingCanObserve(t *testing.T) {
	tests := []struct {
		name          string
		status        string
		grantedScopes []string
		expected      bool
	}{
		{"active with read", BindingStatusActive, []string{BindingScopeTerminalRead}, true},
		{"active without read", BindingStatusActive, []string{BindingScopeTerminalWrite}, false},
		{"pending with read", BindingStatusPending, []string{BindingScopeTerminalRead}, false},
		{"active with no scopes", BindingStatusActive, []string{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := &SessionBinding{
				Status:        tt.status,
				GrantedScopes: pq.StringArray(tt.grantedScopes),
			}
			if sb.CanObserve() != tt.expected {
				t.Errorf("expected CanObserve() = %v, got %v", tt.expected, sb.CanObserve())
			}
		})
	}
}

func TestSessionBindingCanControl(t *testing.T) {
	tests := []struct {
		name          string
		status        string
		grantedScopes []string
		expected      bool
	}{
		{"active with write", BindingStatusActive, []string{BindingScopeTerminalWrite}, true},
		{"active without write", BindingStatusActive, []string{BindingScopeTerminalRead}, false},
		{"pending with write", BindingStatusPending, []string{BindingScopeTerminalWrite}, false},
		{"active with both", BindingStatusActive, []string{BindingScopeTerminalRead, BindingScopeTerminalWrite}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := &SessionBinding{
				Status:        tt.status,
				GrantedScopes: pq.StringArray(tt.grantedScopes),
			}
			if sb.CanControl() != tt.expected {
				t.Errorf("expected CanControl() = %v, got %v", tt.expected, sb.CanControl())
			}
		})
	}
}

func TestSessionBindingStruct(t *testing.T) {
	now := time.Now()
	reason := "User declined"

	sb := SessionBinding{
		ID:               1,
		OrganizationID:   100,
		InitiatorSession: "sess-init",
		TargetSession:    "sess-target",
		GrantedScopes:    pq.StringArray{BindingScopeTerminalRead},
		PendingScopes:    pq.StringArray{BindingScopeTerminalWrite},
		Status:           BindingStatusPending,
		RequestedAt:      &now,
		RejectionReason:  &reason,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	if sb.ID != 1 {
		t.Errorf("expected ID 1, got %d", sb.ID)
	}
	if sb.InitiatorSession != "sess-init" {
		t.Errorf("expected InitiatorSession 'sess-init', got %s", sb.InitiatorSession)
	}
	if sb.TargetSession != "sess-target" {
		t.Errorf("expected TargetSession 'sess-target', got %s", sb.TargetSession)
	}
}

// --- Benchmark Tests ---

func BenchmarkSessionBindingHasScope(b *testing.B) {
	sb := &SessionBinding{
		GrantedScopes: pq.StringArray{BindingScopeTerminalRead, BindingScopeTerminalWrite},
	}
	for i := 0; i < b.N; i++ {
		sb.HasScope(BindingScopeTerminalRead)
	}
}

func BenchmarkSessionBindingIsActive(b *testing.B) {
	sb := &SessionBinding{Status: BindingStatusActive}
	for i := 0; i < b.N; i++ {
		sb.IsActive()
	}
}

func BenchmarkSessionBindingCanObserve(b *testing.B) {
	sb := &SessionBinding{
		Status:        BindingStatusActive,
		GrantedScopes: pq.StringArray{BindingScopeTerminalRead},
	}
	for i := 0; i < b.N; i++ {
		sb.CanObserve()
	}
}

func BenchmarkMessageMetadataScan(b *testing.B) {
	data := []byte(`{"language":"go","filename":"main.go"}`)
	for i := 0; i < b.N; i++ {
		var mm MessageMetadata
		mm.Scan(data)
	}
}
