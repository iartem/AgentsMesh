package devmesh

import (
	"testing"
	"time"
)

// --- Test DevMeshNode ---

func TestDevMeshNodeStruct(t *testing.T) {
	now := time.Now()
	model := "opus"
	ticketID := int64(20)
	repoID := int64(5)
	position := &NodePosition{X: 100.5, Y: 200.5}

	node := DevMeshNode{
		SessionKey:   "sess-123",
		Status:       "running",
		AgentStatus:  "working",
		Model:        &model,
		TicketID:     &ticketID,
		RepositoryID: &repoID,
		CreatedByID:  50,
		RunnerID:     10,
		StartedAt:    &now,
		Position:     position,
	}

	if node.SessionKey != "sess-123" {
		t.Errorf("expected SessionKey 'sess-123', got %s", node.SessionKey)
	}
	if node.Status != "running" {
		t.Errorf("expected Status 'running', got %s", node.Status)
	}
	if node.AgentStatus != "working" {
		t.Errorf("expected AgentStatus 'working', got %s", node.AgentStatus)
	}
	if *node.Model != "opus" {
		t.Errorf("expected Model 'opus', got %s", *node.Model)
	}
	if *node.TicketID != 20 {
		t.Errorf("expected TicketID 20, got %d", *node.TicketID)
	}
	if node.CreatedByID != 50 {
		t.Errorf("expected CreatedByID 50, got %d", node.CreatedByID)
	}
	if node.RunnerID != 10 {
		t.Errorf("expected RunnerID 10, got %d", node.RunnerID)
	}
	if node.Position.X != 100.5 {
		t.Errorf("expected Position.X 100.5, got %f", node.Position.X)
	}
}

func TestDevMeshNodeWithNilOptionalFields(t *testing.T) {
	node := DevMeshNode{
		SessionKey:  "sess-456",
		Status:      "initializing",
		AgentStatus: "unknown",
		CreatedByID: 50,
		RunnerID:    10,
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
	if node.StartedAt != nil {
		t.Error("expected StartedAt to be nil")
	}
	if node.Position != nil {
		t.Error("expected Position to be nil")
	}
}

// --- Test NodePosition ---

func TestNodePositionStruct(t *testing.T) {
	pos := NodePosition{
		X: 150.25,
		Y: 300.75,
	}

	if pos.X != 150.25 {
		t.Errorf("expected X 150.25, got %f", pos.X)
	}
	if pos.Y != 300.75 {
		t.Errorf("expected Y 300.75, got %f", pos.Y)
	}
}

func TestNodePositionZeroValues(t *testing.T) {
	pos := NodePosition{}

	if pos.X != 0 {
		t.Errorf("expected X 0, got %f", pos.X)
	}
	if pos.Y != 0 {
		t.Errorf("expected Y 0, got %f", pos.Y)
	}
}

// --- Test DevMeshEdge ---

func TestDevMeshEdgeStruct(t *testing.T) {
	edge := DevMeshEdge{
		ID:            1,
		Source:        "sess-init",
		Target:        "sess-target",
		GrantedScopes: []string{"terminal:read", "terminal:write"},
		PendingScopes: []string{"file:read"},
		Status:        "active",
	}

	if edge.ID != 1 {
		t.Errorf("expected ID 1, got %d", edge.ID)
	}
	if edge.Source != "sess-init" {
		t.Errorf("expected Source 'sess-init', got %s", edge.Source)
	}
	if edge.Target != "sess-target" {
		t.Errorf("expected Target 'sess-target', got %s", edge.Target)
	}
	if len(edge.GrantedScopes) != 2 {
		t.Errorf("expected 2 GrantedScopes, got %d", len(edge.GrantedScopes))
	}
	if len(edge.PendingScopes) != 1 {
		t.Errorf("expected 1 PendingScopes, got %d", len(edge.PendingScopes))
	}
	if edge.Status != "active" {
		t.Errorf("expected Status 'active', got %s", edge.Status)
	}
}

func TestDevMeshEdgeWithEmptyScopes(t *testing.T) {
	edge := DevMeshEdge{
		ID:            2,
		Source:        "sess-a",
		Target:        "sess-b",
		GrantedScopes: []string{},
		Status:        "pending",
	}

	if len(edge.GrantedScopes) != 0 {
		t.Errorf("expected 0 GrantedScopes, got %d", len(edge.GrantedScopes))
	}
	if edge.PendingScopes != nil && len(edge.PendingScopes) != 0 {
		t.Errorf("expected empty PendingScopes")
	}
}

// --- Test ChannelInfo ---

func TestChannelInfoStruct(t *testing.T) {
	desc := "Development channel"

	info := ChannelInfo{
		ID:           1,
		Name:         "dev-channel",
		Description:  &desc,
		SessionKeys:  []string{"sess-1", "sess-2", "sess-3"},
		MessageCount: 150,
		IsArchived:   false,
	}

	if info.ID != 1 {
		t.Errorf("expected ID 1, got %d", info.ID)
	}
	if info.Name != "dev-channel" {
		t.Errorf("expected Name 'dev-channel', got %s", info.Name)
	}
	if *info.Description != "Development channel" {
		t.Errorf("expected Description 'Development channel', got %s", *info.Description)
	}
	if len(info.SessionKeys) != 3 {
		t.Errorf("expected 3 SessionKeys, got %d", len(info.SessionKeys))
	}
	if info.MessageCount != 150 {
		t.Errorf("expected MessageCount 150, got %d", info.MessageCount)
	}
	if info.IsArchived {
		t.Error("expected IsArchived false")
	}
}

// --- Test DevMeshTopology ---

func TestDevMeshTopologyStruct(t *testing.T) {
	topology := DevMeshTopology{
		Nodes: []DevMeshNode{
			{SessionKey: "sess-1", Status: "running"},
			{SessionKey: "sess-2", Status: "running"},
		},
		Edges: []DevMeshEdge{
			{ID: 1, Source: "sess-1", Target: "sess-2", Status: "active"},
		},
		Channels: []ChannelInfo{
			{ID: 1, Name: "general", MessageCount: 50},
		},
	}

	if len(topology.Nodes) != 2 {
		t.Errorf("expected 2 Nodes, got %d", len(topology.Nodes))
	}
	if len(topology.Edges) != 1 {
		t.Errorf("expected 1 Edge, got %d", len(topology.Edges))
	}
	if len(topology.Channels) != 1 {
		t.Errorf("expected 1 Channel, got %d", len(topology.Channels))
	}
}

func TestDevMeshTopologyEmpty(t *testing.T) {
	topology := DevMeshTopology{
		Nodes:    []DevMeshNode{},
		Edges:    []DevMeshEdge{},
		Channels: []ChannelInfo{},
	}

	if len(topology.Nodes) != 0 {
		t.Errorf("expected 0 Nodes, got %d", len(topology.Nodes))
	}
	if len(topology.Edges) != 0 {
		t.Errorf("expected 0 Edges, got %d", len(topology.Edges))
	}
	if len(topology.Channels) != 0 {
		t.Errorf("expected 0 Channels, got %d", len(topology.Channels))
	}
}

// --- Test ChannelSession ---

func TestChannelSessionTableName(t *testing.T) {
	cs := ChannelSession{}
	if cs.TableName() != "channel_sessions" {
		t.Errorf("expected 'channel_sessions', got %s", cs.TableName())
	}
}

func TestChannelSessionStruct(t *testing.T) {
	now := time.Now()

	cs := ChannelSession{
		ID:         1,
		ChannelID:  10,
		SessionKey: "sess-123",
		JoinedAt:   now,
	}

	if cs.ID != 1 {
		t.Errorf("expected ID 1, got %d", cs.ID)
	}
	if cs.ChannelID != 10 {
		t.Errorf("expected ChannelID 10, got %d", cs.ChannelID)
	}
	if cs.SessionKey != "sess-123" {
		t.Errorf("expected SessionKey 'sess-123', got %s", cs.SessionKey)
	}
}

// --- Test ChannelAccess ---

func TestChannelAccessTableName(t *testing.T) {
	ca := ChannelAccess{}
	if ca.TableName() != "channel_access" {
		t.Errorf("expected 'channel_access', got %s", ca.TableName())
	}
}

func TestChannelAccessStruct(t *testing.T) {
	now := time.Now()
	sessionKey := "sess-123"
	userID := int64(50)

	ca := ChannelAccess{
		ID:         1,
		ChannelID:  10,
		SessionKey: &sessionKey,
		UserID:     &userID,
		LastAccess: now,
	}

	if ca.ID != 1 {
		t.Errorf("expected ID 1, got %d", ca.ID)
	}
	if ca.ChannelID != 10 {
		t.Errorf("expected ChannelID 10, got %d", ca.ChannelID)
	}
	if *ca.SessionKey != "sess-123" {
		t.Errorf("expected SessionKey 'sess-123', got %s", *ca.SessionKey)
	}
	if *ca.UserID != 50 {
		t.Errorf("expected UserID 50, got %d", *ca.UserID)
	}
}

// --- Test CreateSessionForTicketRequest ---

func TestCreateSessionForTicketRequestStruct(t *testing.T) {
	teamID := int64(10)

	req := CreateSessionForTicketRequest{
		OrganizationID: 100,
		TeamID:         &teamID,
		TicketID:       20,
		RunnerID:       5,
		CreatedByID:    50,
		InitialPrompt:  "Start working on ticket",
		Model:          "opus",
		PermissionMode: "bypassPermissions",
		ThinkLevel:     "ultrathink",
	}

	if req.OrganizationID != 100 {
		t.Errorf("expected OrganizationID 100, got %d", req.OrganizationID)
	}
	if *req.TeamID != 10 {
		t.Errorf("expected TeamID 10, got %d", *req.TeamID)
	}
	if req.TicketID != 20 {
		t.Errorf("expected TicketID 20, got %d", req.TicketID)
	}
	if req.Model != "opus" {
		t.Errorf("expected Model 'opus', got %s", req.Model)
	}
	if req.ThinkLevel != "ultrathink" {
		t.Errorf("expected ThinkLevel 'ultrathink', got %s", req.ThinkLevel)
	}
}

// --- Test TicketSessionInfo ---

func TestTicketSessionInfoStruct(t *testing.T) {
	info := TicketSessionInfo{
		TicketID: 20,
		Sessions: []DevMeshNode{
			{SessionKey: "sess-1", Status: "running"},
			{SessionKey: "sess-2", Status: "completed"},
		},
	}

	if info.TicketID != 20 {
		t.Errorf("expected TicketID 20, got %d", info.TicketID)
	}
	if len(info.Sessions) != 2 {
		t.Errorf("expected 2 Sessions, got %d", len(info.Sessions))
	}
}

// --- Test BatchTicketSessionsResponse ---

func TestBatchTicketSessionsResponseStruct(t *testing.T) {
	resp := BatchTicketSessionsResponse{
		TicketSessions: map[int64][]DevMeshNode{
			1: {{SessionKey: "sess-1", Status: "running"}},
			2: {{SessionKey: "sess-2", Status: "running"}, {SessionKey: "sess-3", Status: "completed"}},
		},
	}

	if len(resp.TicketSessions) != 2 {
		t.Errorf("expected 2 ticket entries, got %d", len(resp.TicketSessions))
	}
	if len(resp.TicketSessions[1]) != 1 {
		t.Errorf("expected 1 session for ticket 1, got %d", len(resp.TicketSessions[1]))
	}
	if len(resp.TicketSessions[2]) != 2 {
		t.Errorf("expected 2 sessions for ticket 2, got %d", len(resp.TicketSessions[2]))
	}
}

// --- Benchmark Tests ---

func BenchmarkChannelSessionTableName(b *testing.B) {
	cs := ChannelSession{}
	for i := 0; i < b.N; i++ {
		cs.TableName()
	}
}

func BenchmarkChannelAccessTableName(b *testing.B) {
	ca := ChannelAccess{}
	for i := 0; i < b.N; i++ {
		ca.TableName()
	}
}

func BenchmarkDevMeshTopologyCreation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = DevMeshTopology{
			Nodes:    []DevMeshNode{{SessionKey: "sess-1", Status: "running"}},
			Edges:    []DevMeshEdge{{ID: 1, Source: "sess-1", Target: "sess-2"}},
			Channels: []ChannelInfo{{ID: 1, Name: "general"}},
		}
	}
}
