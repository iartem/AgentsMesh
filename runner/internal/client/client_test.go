package client

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// --- Test Client (client.go) ---

func TestNewClient(t *testing.T) {
	client := New("http://localhost:8080", "test-node", "test-token")

	if client == nil {
		t.Fatal("New returned nil")
	}

	if client.serverURL != "http://localhost:8080" {
		t.Errorf("serverURL: got %v, want http://localhost:8080", client.serverURL)
	}

	if client.nodeID != "test-node" {
		t.Errorf("nodeID: got %v, want test-node", client.nodeID)
	}

	if client.authToken != "test-token" {
		t.Errorf("authToken: got %v, want test-token", client.authToken)
	}

	if client.messages == nil {
		t.Error("messages channel should not be nil")
	}

	if client.stopChan == nil {
		t.Error("stopChan should not be nil")
	}
}

func TestClientSetAuthToken(t *testing.T) {
	client := New("http://localhost:8080", "test-node", "old-token")

	client.SetAuthToken("new-token")

	if client.authToken != "new-token" {
		t.Errorf("authToken: got %v, want new-token", client.authToken)
	}
}

func TestClientRegister(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %v, want POST", r.Method)
		}

		if r.URL.Path != "/api/v1/runners/register" {
			t.Errorf("path: got %v, want /api/v1/runners/register", r.URL.Path)
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		if body["node_id"] != "test-node" {
			t.Errorf("node_id: got %v, want test-node", body["node_id"])
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"auth_token": "returned-token",
			"runner_id":  123,
		})
	}))
	defer server.Close()

	client := New(server.URL, "test-node", "")
	token, err := client.Register(context.Background(), "reg-token", "Test Runner", 5)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if token != "returned-token" {
		t.Errorf("token: got %v, want returned-token", token)
	}

	if client.authToken != "returned-token" {
		t.Errorf("authToken: got %v, want returned-token", client.authToken)
	}
}

func TestClientRegisterError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid request"))
	}))
	defer server.Close()

	client := New(server.URL, "test-node", "")
	_, err := client.Register(context.Background(), "reg-token", "Test Runner", 5)

	if err == nil {
		t.Error("expected error for bad request")
	}
}

func TestClientMessages(t *testing.T) {
	client := New("http://localhost:8080", "test-node", "test-token")

	ch := client.Messages()
	if ch == nil {
		t.Error("Messages() should not return nil")
	}
}

func TestClientSendNotConnected(t *testing.T) {
	client := New("http://localhost:8080", "test-node", "test-token")

	err := client.Send(&Message{Type: MessageTypeHeartbeat})

	if err == nil {
		t.Error("expected error when not connected")
	}

	if !strings.Contains(err.Error(), "not connected") {
		t.Errorf("error should mention not connected: %v", err)
	}
}

func TestClientSendHeartbeat(t *testing.T) {
	client := New("http://localhost:8080", "test-node", "test-token")

	// Should error since not connected
	err := client.SendHeartbeat(5)
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestClientSendTerminalOutput(t *testing.T) {
	client := New("http://localhost:8080", "test-node", "test-token")

	// Should error since not connected
	err := client.SendTerminalOutput("pod-1", []byte("output"))
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestClientSendPodStatus(t *testing.T) {
	client := New("http://localhost:8080", "test-node", "test-token")

	// Should error since not connected
	err := client.SendPodStatus("pod-1", "running", map[string]interface{}{"key": "value"})
	if err == nil {
		t.Error("expected error when not connected")
	}
}

func TestClientIsConnected(t *testing.T) {
	client := New("http://localhost:8080", "test-node", "test-token")

	if client.IsConnected() {
		t.Error("should not be connected initially")
	}
}

func TestClientClose(t *testing.T) {
	client := New("http://localhost:8080", "test-node", "test-token")

	// Should not error even when not connected
	err := client.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestMessageStruct(t *testing.T) {
	msg := Message{
		Type:      MessageTypeHeartbeat,
		Data:      json.RawMessage(`{"test": "data"}`),
		Timestamp: time.Now().UnixMilli(),
	}

	if msg.Type != MessageTypeHeartbeat {
		t.Errorf("Type: got %v, want %v", msg.Type, MessageTypeHeartbeat)
	}

	if msg.Timestamp == 0 {
		t.Error("Timestamp should be set")
	}
}

func TestMessageTypeConstants(t *testing.T) {
	// Test that message type constants match backend protocol
	if MessageTypePodStart != "create_pod" {
		t.Errorf("MessageTypePodStart: got %v, want create_pod", MessageTypePodStart)
	}
	if MessageTypePodStop != "terminate_pod" {
		t.Errorf("MessageTypePodStop: got %v, want terminate_pod", MessageTypePodStop)
	}
	if MessageTypePodStatus != "agent_status" {
		t.Errorf("MessageTypePodStatus: got %v, want agent_status", MessageTypePodStatus)
	}
	if MessageTypePodList != "pod_list" {
		t.Errorf("MessageTypePodList: got %v, want pod_list", MessageTypePodList)
	}
	if MessageTypeTerminalInput != "terminal_input" {
		t.Errorf("MessageTypeTerminalInput: got %v, want terminal_input", MessageTypeTerminalInput)
	}
	if MessageTypeTerminalOutput != "terminal_output" {
		t.Errorf("MessageTypeTerminalOutput: got %v, want terminal_output", MessageTypeTerminalOutput)
	}
	if MessageTypeTerminalResize != "terminal_resize" {
		t.Errorf("MessageTypeTerminalResize: got %v, want terminal_resize", MessageTypeTerminalResize)
	}
	if MessageTypeHeartbeat != "heartbeat" {
		t.Errorf("MessageTypeHeartbeat: got %v, want heartbeat", MessageTypeHeartbeat)
	}
	if MessageTypeError != "error" {
		t.Errorf("MessageTypeError: got %v, want error", MessageTypeError)
	}
}

// --- Test Protocol (protocol.go) ---

func TestProtocolMessageTypes(t *testing.T) {
	if MsgTypeHeartbeat != "heartbeat" {
		t.Errorf("MsgTypeHeartbeat: got %v, want heartbeat", MsgTypeHeartbeat)
	}
	if MsgTypePodCreated != "pod_created" {
		t.Errorf("MsgTypePodCreated: got %v, want pod_created", MsgTypePodCreated)
	}
	if MsgTypePodTerminated != "pod_terminated" {
		t.Errorf("MsgTypePodTerminated: got %v, want pod_terminated", MsgTypePodTerminated)
	}
	if MsgTypeStatusChange != "status_change" {
		t.Errorf("MsgTypeStatusChange: got %v, want status_change", MsgTypeStatusChange)
	}
	if MsgTypePodList != "pod_list" {
		t.Errorf("MsgTypePodList: got %v, want pod_list", MsgTypePodList)
	}
	if MsgTypeTerminalOutput != "terminal_output" {
		t.Errorf("MsgTypeTerminalOutput: got %v, want terminal_output", MsgTypeTerminalOutput)
	}
	if MsgTypePtyResized != "pty_resized" {
		t.Errorf("MsgTypePtyResized: got %v, want pty_resized", MsgTypePtyResized)
	}
	if MsgTypeCreatePod != "create_pod" {
		t.Errorf("MsgTypeCreatePod: got %v, want create_pod", MsgTypeCreatePod)
	}
	if MsgTypeTerminatePod != "terminate_pod" {
		t.Errorf("MsgTypeTerminatePod: got %v, want terminate_pod", MsgTypeTerminatePod)
	}
	if MsgTypeListPods != "list_pods" {
		t.Errorf("MsgTypeListPods: got %v, want list_pods", MsgTypeListPods)
	}
	if MsgTypeTerminalInput != "terminal_input" {
		t.Errorf("MsgTypeTerminalInput: got %v, want terminal_input", MsgTypeTerminalInput)
	}
	if MsgTypeTerminalResize != "terminal_resize" {
		t.Errorf("MsgTypeTerminalResize: got %v, want terminal_resize", MsgTypeTerminalResize)
	}
}

func TestProtocolMessageStruct(t *testing.T) {
	msg := ProtocolMessage{
		Type:      MsgTypeHeartbeat,
		Timestamp: time.Now().UnixMilli(),
		Data:      json.RawMessage(`{"test": "data"}`),
	}

	if msg.Type != MsgTypeHeartbeat {
		t.Errorf("Type: got %v, want %v", msg.Type, MsgTypeHeartbeat)
	}
	if msg.Timestamp == 0 {
		t.Error("Timestamp should be set")
	}
}

func TestHeartbeatData(t *testing.T) {
	data := HeartbeatData{
		NodeID: "test-node",
		Pods: []PodInfo{
			{PodKey: "pod-1", Status: "running"},
		},
	}

	if data.NodeID != "test-node" {
		t.Errorf("NodeID: got %v, want test-node", data.NodeID)
	}
	if len(data.Pods) != 1 {
		t.Errorf("Pods length: got %v, want 1", len(data.Pods))
	}
}

func TestPodInfoStruct(t *testing.T) {
	info := PodInfo{
		PodKey:       "pod-1",
		Status:       "running",
		ClaudeStatus: "active",
		Pid:          12345,
		ClientCount:  3,
	}

	if info.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", info.PodKey)
	}
	if info.Pid != 12345 {
		t.Errorf("Pid: got %v, want 12345", info.Pid)
	}
}

func TestPreparationConfig(t *testing.T) {
	config := PreparationConfig{
		Script:         "echo hello",
		TimeoutSeconds: 300,
	}

	if config.Script != "echo hello" {
		t.Errorf("Script: got %v, want echo hello", config.Script)
	}
}

func TestCreatePodRequest(t *testing.T) {
	req := CreatePodRequest{
		PodKey:           "pod-1",
		InitialCommand:   "claude-code",
		InitialPrompt:    "Hello",
		PermissionMode:   "plan",
		WorkingDir:       "/workspace",
		TicketIdentifier: "TICKET-123",
		WorktreeSuffix:   "-v1",
		EnvVars:          map[string]string{"KEY": "VALUE"},
		PreparationConfig: &PreparationConfig{
			Script:         "npm install",
			TimeoutSeconds: 600,
		},
	}

	if req.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", req.PodKey)
	}
	if req.PermissionMode != "plan" {
		t.Errorf("PermissionMode: got %v, want plan", req.PermissionMode)
	}
	if req.EnvVars["KEY"] != "VALUE" {
		t.Errorf("EnvVars[KEY]: got %v, want VALUE", req.EnvVars["KEY"])
	}
}

func TestCreatePodRequestWithPluginConfig(t *testing.T) {
	req := CreatePodRequest{
		PodKey:         "pod-2",
		InitialCommand: "claude",
		PluginConfig: map[string]interface{}{
			"repository_url":    "https://github.com/org/repo.git",
			"branch":            "develop",
			"ticket_identifier": "TICKET-456",
			"git_token":         "ghp_token",
			"init_script":       "npm install && npm run build",
			"init_timeout":      300,
			"env_vars": map[string]string{
				"NODE_ENV": "development",
			},
		},
	}

	if req.PodKey != "pod-2" {
		t.Errorf("PodKey: got %v, want pod-2", req.PodKey)
	}
	if req.PluginConfig == nil {
		t.Fatal("PluginConfig should not be nil")
	}
	if req.PluginConfig["repository_url"] != "https://github.com/org/repo.git" {
		t.Errorf("repository_url: got %v, want https://github.com/org/repo.git", req.PluginConfig["repository_url"])
	}
	if req.PluginConfig["branch"] != "develop" {
		t.Errorf("branch: got %v, want develop", req.PluginConfig["branch"])
	}
	if req.PluginConfig["ticket_identifier"] != "TICKET-456" {
		t.Errorf("ticket_identifier: got %v, want TICKET-456", req.PluginConfig["ticket_identifier"])
	}
	if req.PluginConfig["git_token"] != "ghp_token" {
		t.Errorf("git_token: got %v, want ghp_token", req.PluginConfig["git_token"])
	}
	if req.PluginConfig["init_script"] != "npm install && npm run build" {
		t.Errorf("init_script: got %v, want npm install && npm run build", req.PluginConfig["init_script"])
	}
	if req.PluginConfig["init_timeout"] != 300 {
		t.Errorf("init_timeout: got %v, want 300", req.PluginConfig["init_timeout"])
	}
}

func TestCreatePodRequestPluginConfigJSON(t *testing.T) {
	// Test that PluginConfig serializes correctly to JSON
	req := CreatePodRequest{
		PodKey: "pod-json",
		PluginConfig: map[string]interface{}{
			"repository_url": "https://github.com/test/repo.git",
			"nested": map[string]interface{}{
				"key": "value",
			},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	var parsed CreatePodRequest
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if parsed.PluginConfig["repository_url"] != "https://github.com/test/repo.git" {
		t.Errorf("repository_url after roundtrip: got %v, want https://github.com/test/repo.git", parsed.PluginConfig["repository_url"])
	}

	nested, ok := parsed.PluginConfig["nested"].(map[string]interface{})
	if !ok {
		t.Fatal("nested should be a map")
	}
	if nested["key"] != "value" {
		t.Errorf("nested.key: got %v, want value", nested["key"])
	}
}

func TestTerminatePodRequest(t *testing.T) {
	req := TerminatePodRequest{
		PodKey: "pod-1",
	}

	if req.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", req.PodKey)
	}
}

func TestPodCreatedEvent(t *testing.T) {
	event := PodCreatedEvent{
		PodKey:       "pod-1",
		Pid:          12345,
		WorktreePath: "/workspace/worktrees/pod-1",
		BranchName:   "feature/test",
		PtyCols:      120,
		PtyRows:      40,
	}

	if event.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", event.PodKey)
	}
	if event.PtyCols != 120 {
		t.Errorf("PtyCols: got %v, want 120", event.PtyCols)
	}
}

func TestPodTerminatedEvent(t *testing.T) {
	event := PodTerminatedEvent{
		PodKey: "pod-1",
	}

	if event.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", event.PodKey)
	}
}

func TestStatusChangeEvent(t *testing.T) {
	event := StatusChangeEvent{
		PodKey:       "pod-1",
		ClaudeStatus: "active",
		ClaudePid:    12345,
	}

	if event.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", event.PodKey)
	}
	if event.ClaudeStatus != "active" {
		t.Errorf("ClaudeStatus: got %v, want active", event.ClaudeStatus)
	}
}

func TestTerminalOutputEvent(t *testing.T) {
	event := TerminalOutputEvent{
		PodKey: "pod-1",
		Data:   "SGVsbG8gV29ybGQ=", // Base64 encoded
	}

	if event.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", event.PodKey)
	}
}

func TestTerminalInputRequest(t *testing.T) {
	req := TerminalInputRequest{
		PodKey: "pod-1",
		Data:   "SGVsbG8=",
	}

	if req.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", req.PodKey)
	}
}

func TestTerminalResizeRequest(t *testing.T) {
	req := TerminalResizeRequest{
		PodKey: "pod-1",
		Cols:   120,
		Rows:   40,
	}

	if req.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", req.PodKey)
	}
	if req.Cols != 120 {
		t.Errorf("Cols: got %v, want 120", req.Cols)
	}
}

func TestPtyResizedEvent(t *testing.T) {
	event := PtyResizedEvent{
		PodKey: "pod-1",
		Cols:   120,
		Rows:   40,
	}

	if event.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", event.PodKey)
	}
}

// --- Test Interfaces (interfaces.go) ---

func TestNewGorillaDialer(t *testing.T) {
	dialer := NewGorillaDialer()

	if dialer == nil {
		t.Fatal("NewGorillaDialer returned nil")
	}

	if dialer.Dialer == nil {
		t.Error("Dialer should not be nil")
	}
}

// --- Test ReconnectStrategy (reconnect_strategy.go) ---

func TestNewReconnectStrategy(t *testing.T) {
	strategy := NewReconnectStrategy(time.Second, time.Minute)

	if strategy == nil {
		t.Fatal("NewReconnectStrategy returned nil")
	}

	if strategy.initialInterval != time.Second {
		t.Errorf("initialInterval: got %v, want %v", strategy.initialInterval, time.Second)
	}

	if strategy.maxInterval != time.Minute {
		t.Errorf("maxInterval: got %v, want %v", strategy.maxInterval, time.Minute)
	}

	if strategy.currentInterval != time.Second {
		t.Errorf("currentInterval: got %v, want %v", strategy.currentInterval, time.Second)
	}

	if strategy.attemptCount != 0 {
		t.Errorf("attemptCount: got %v, want 0", strategy.attemptCount)
	}
}

func TestReconnectStrategyNextDelay(t *testing.T) {
	strategy := NewReconnectStrategy(time.Second, 8*time.Second)

	// First delay should be 1 second
	delay1 := strategy.NextDelay()
	if delay1 != time.Second {
		t.Errorf("first delay: got %v, want %v", delay1, time.Second)
	}
	if strategy.AttemptCount() != 1 {
		t.Errorf("attempt count after 1: got %v, want 1", strategy.AttemptCount())
	}

	// Second delay should be 2 seconds
	delay2 := strategy.NextDelay()
	if delay2 != 2*time.Second {
		t.Errorf("second delay: got %v, want %v", delay2, 2*time.Second)
	}

	// Third delay should be 4 seconds
	delay3 := strategy.NextDelay()
	if delay3 != 4*time.Second {
		t.Errorf("third delay: got %v, want %v", delay3, 4*time.Second)
	}

	// Fourth delay should be 8 seconds (capped at max)
	delay4 := strategy.NextDelay()
	if delay4 != 8*time.Second {
		t.Errorf("fourth delay: got %v, want %v", delay4, 8*time.Second)
	}

	// Fifth delay should still be 8 seconds (max)
	delay5 := strategy.NextDelay()
	if delay5 != 8*time.Second {
		t.Errorf("fifth delay: got %v, want %v", delay5, 8*time.Second)
	}
}

func TestReconnectStrategyReset(t *testing.T) {
	strategy := NewReconnectStrategy(time.Second, time.Minute)

	// Use a few attempts
	strategy.NextDelay()
	strategy.NextDelay()
	strategy.NextDelay()

	// Reset
	strategy.Reset()

	if strategy.AttemptCount() != 0 {
		t.Errorf("attempt count after reset: got %v, want 0", strategy.AttemptCount())
	}

	if strategy.CurrentInterval() != time.Second {
		t.Errorf("current interval after reset: got %v, want %v", strategy.CurrentInterval(), time.Second)
	}
}

func TestReconnectStrategyCurrentInterval(t *testing.T) {
	strategy := NewReconnectStrategy(time.Second, time.Minute)

	if strategy.CurrentInterval() != time.Second {
		t.Errorf("initial interval: got %v, want %v", strategy.CurrentInterval(), time.Second)
	}

	strategy.NextDelay()

	if strategy.CurrentInterval() != 2*time.Second {
		t.Errorf("interval after first delay: got %v, want %v", strategy.CurrentInterval(), 2*time.Second)
	}
}

// --- Test MessageRouter (message_router.go) ---

// mockHandler is a mock implementation of MessageHandler
type mockHandler struct {
	createPodCalled    bool
	terminatePodCalled bool
	listPodsCalled     bool
	terminalInputCalled    bool
	terminalResizeCalled   bool
	lastCreateReq          CreatePodRequest
	lastTerminateReq       TerminatePodRequest
	lastInputReq           TerminalInputRequest
	lastResizeReq          TerminalResizeRequest
	pods                   []PodInfo
	mu                     sync.Mutex
}

func (m *mockHandler) OnCreatePod(req CreatePodRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.createPodCalled = true
	m.lastCreateReq = req
	return nil
}

func (m *mockHandler) OnTerminatePod(req TerminatePodRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.terminatePodCalled = true
	m.lastTerminateReq = req
	return nil
}

func (m *mockHandler) OnListPods() []PodInfo {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.listPodsCalled = true
	return m.pods
}

func (m *mockHandler) OnTerminalInput(req TerminalInputRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.terminalInputCalled = true
	m.lastInputReq = req
	return nil
}

func (m *mockHandler) OnTerminalResize(req TerminalResizeRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.terminalResizeCalled = true
	m.lastResizeReq = req
	return nil
}

// mockEventSender is a mock implementation of EventSender
type mockEventSender struct {
	sentEvents []struct {
		msgType MessageType
		data    interface{}
	}
	mu sync.Mutex
}

func (m *mockEventSender) SendEvent(msgType MessageType, data interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentEvents = append(m.sentEvents, struct {
		msgType MessageType
		data    interface{}
	}{msgType, data})
	return nil
}

func TestNewMessageRouter(t *testing.T) {
	handler := &mockHandler{}
	sender := &mockEventSender{}

	router := NewMessageRouter(handler, sender)

	if router == nil {
		t.Fatal("NewMessageRouter returned nil")
	}

	if router.handler != handler {
		t.Error("handler not set correctly")
	}

	if router.eventSender != sender {
		t.Error("eventSender not set correctly")
	}
}

func TestMessageRouterRouteCreatePod(t *testing.T) {
	handler := &mockHandler{}
	sender := &mockEventSender{}
	router := NewMessageRouter(handler, sender)

	reqData, _ := json.Marshal(CreatePodRequest{
		PodKey:         "pod-1",
		InitialCommand: "claude-code",
	})

	msg := ProtocolMessage{
		Type: MsgTypeCreatePod,
		Data: reqData,
	}

	router.Route(msg)

	if !handler.createPodCalled {
		t.Error("OnCreatePod should be called")
	}

	if handler.lastCreateReq.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", handler.lastCreateReq.PodKey)
	}
}

func TestMessageRouterRouteTerminatePod(t *testing.T) {
	handler := &mockHandler{}
	sender := &mockEventSender{}
	router := NewMessageRouter(handler, sender)

	reqData, _ := json.Marshal(TerminatePodRequest{
		PodKey: "pod-1",
	})

	msg := ProtocolMessage{
		Type: MsgTypeTerminatePod,
		Data: reqData,
	}

	router.Route(msg)

	if !handler.terminatePodCalled {
		t.Error("OnTerminatePod should be called")
	}
}

func TestMessageRouterRouteListPods(t *testing.T) {
	handler := &mockHandler{
		pods: []PodInfo{
			{PodKey: "pod-1", Status: "running"},
		},
	}
	sender := &mockEventSender{}
	router := NewMessageRouter(handler, sender)

	msg := ProtocolMessage{
		Type: MsgTypeListPods,
	}

	router.Route(msg)

	if !handler.listPodsCalled {
		t.Error("OnListPods should be called")
	}

	if len(sender.sentEvents) != 1 {
		t.Errorf("sentEvents length: got %v, want 1", len(sender.sentEvents))
	}

	if sender.sentEvents[0].msgType != MsgTypePodList {
		t.Errorf("sent event type: got %v, want %v", sender.sentEvents[0].msgType, MsgTypePodList)
	}
}

func TestMessageRouterRouteTerminalInput(t *testing.T) {
	handler := &mockHandler{}
	sender := &mockEventSender{}
	router := NewMessageRouter(handler, sender)

	reqData, _ := json.Marshal(TerminalInputRequest{
		PodKey: "pod-1",
		Data:   "SGVsbG8=",
	})

	msg := ProtocolMessage{
		Type: MsgTypeTerminalInput,
		Data: reqData,
	}

	router.Route(msg)

	if !handler.terminalInputCalled {
		t.Error("OnTerminalInput should be called")
	}
}

func TestMessageRouterRouteTerminalResize(t *testing.T) {
	handler := &mockHandler{}
	sender := &mockEventSender{}
	router := NewMessageRouter(handler, sender)

	reqData, _ := json.Marshal(TerminalResizeRequest{
		PodKey: "pod-1",
		Cols:   120,
		Rows:   40,
	})

	msg := ProtocolMessage{
		Type: MsgTypeTerminalResize,
		Data: reqData,
	}

	router.Route(msg)

	if !handler.terminalResizeCalled {
		t.Error("OnTerminalResize should be called")
	}

	if handler.lastResizeReq.Cols != 120 {
		t.Errorf("Cols: got %v, want 120", handler.lastResizeReq.Cols)
	}
}

func TestMessageRouterRouteUnknown(t *testing.T) {
	handler := &mockHandler{}
	sender := &mockEventSender{}
	router := NewMessageRouter(handler, sender)

	msg := ProtocolMessage{
		Type: "unknown_type",
	}

	// Should not panic
	router.Route(msg)

	// None of the handlers should be called
	if handler.createPodCalled || handler.terminatePodCalled ||
		handler.listPodsCalled || handler.terminalInputCalled || handler.terminalResizeCalled {
		t.Error("no handler should be called for unknown type")
	}
}

func TestMessageRouterRouteNilHandler(t *testing.T) {
	router := NewMessageRouter(nil, nil)

	msg := ProtocolMessage{
		Type: MsgTypeCreatePod,
	}

	// Should not panic
	router.Route(msg)
}

func TestMessageRouterRouteInvalidJSON(t *testing.T) {
	handler := &mockHandler{}
	sender := &mockEventSender{}
	router := NewMessageRouter(handler, sender)

	msg := ProtocolMessage{
		Type: MsgTypeCreatePod,
		Data: json.RawMessage(`invalid json`),
	}

	// Should not panic
	router.Route(msg)

	// Handler should not be called due to JSON parsing error
	if handler.createPodCalled {
		t.Error("handler should not be called for invalid JSON")
	}
}

// --- Test ServerConnection (connection.go) ---

// mockWebSocketConn is a mock implementation of WebSocketConn
type mockWebSocketConn struct {
	readMessages  [][]byte
	readIndex     int
	writtenData   [][]byte
	closed        bool
	readError     error
	writeError    error
	readDeadline  time.Time
	mu            sync.Mutex
}

func (m *mockWebSocketConn) ReadMessage() (int, []byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.readError != nil {
		return 0, nil, m.readError
	}

	if m.readIndex >= len(m.readMessages) {
		// Block forever or return error
		return 0, nil, websocket.ErrCloseSent
	}

	data := m.readMessages[m.readIndex]
	m.readIndex++
	return websocket.TextMessage, data, nil
}

func (m *mockWebSocketConn) WriteMessage(messageType int, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.writeError != nil {
		return m.writeError
	}

	m.writtenData = append(m.writtenData, data)
	return nil
}

func (m *mockWebSocketConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockWebSocketConn) SetReadDeadline(t time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.readDeadline = t
	return nil
}

// mockWebSocketDialer is a mock implementation of WebSocketDialer
type mockWebSocketDialer struct {
	conn      WebSocketConn
	dialError error
	dialCalls int
	mu        sync.Mutex
}

func (m *mockWebSocketDialer) Dial(urlStr string, requestHeader http.Header) (WebSocketConn, *http.Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.dialCalls++

	if m.dialError != nil {
		return nil, nil, m.dialError
	}

	return m.conn, nil, nil
}

func TestNewServerConnection(t *testing.T) {
	conn := NewServerConnection("ws://localhost:8080", "test-node", "test-token", "test-org")

	if conn == nil {
		t.Fatal("NewServerConnection returned nil")
	}

	if conn.serverURL != "ws://localhost:8080" {
		t.Errorf("serverURL: got %v, want ws://localhost:8080", conn.serverURL)
	}

	if conn.nodeID != "test-node" {
		t.Errorf("nodeID: got %v, want test-node", conn.nodeID)
	}

	if conn.authToken != "test-token" {
		t.Errorf("authToken: got %v, want test-token", conn.authToken)
	}

	if conn.orgSlug != "test-org" {
		t.Errorf("orgSlug: got %v, want test-org", conn.orgSlug)
	}

	if conn.sendCh == nil {
		t.Error("sendCh should not be nil")
	}

	if conn.stopCh == nil {
		t.Error("stopCh should not be nil")
	}

	if conn.reconnectStrategy == nil {
		t.Error("reconnectStrategy should not be nil")
	}
}

func TestServerConnectionWithDialer(t *testing.T) {
	conn := NewServerConnection("ws://localhost:8080", "test-node", "test-token", "test-org")
	dialer := &mockWebSocketDialer{}

	result := conn.WithDialer(dialer)

	if result != conn {
		t.Error("WithDialer should return the same connection")
	}

	if conn.dialer != dialer {
		t.Error("dialer not set correctly")
	}
}

func TestServerConnectionSetHandler(t *testing.T) {
	conn := NewServerConnection("ws://localhost:8080", "test-node", "test-token", "test-org")
	handler := &mockHandler{}

	conn.SetHandler(handler)

	if conn.handler != handler {
		t.Error("handler not set correctly")
	}

	if conn.router == nil {
		t.Error("router should be created")
	}
}

func TestServerConnectionSetHeartbeatInterval(t *testing.T) {
	conn := NewServerConnection("ws://localhost:8080", "test-node", "test-token", "test-org")

	conn.SetHeartbeatInterval(10 * time.Second)

	if conn.heartbeatInterval != 10*time.Second {
		t.Errorf("heartbeatInterval: got %v, want %v", conn.heartbeatInterval, 10*time.Second)
	}
}

func TestServerConnectionConnect(t *testing.T) {
	mockConn := &mockWebSocketConn{}
	mockDialer := &mockWebSocketDialer{conn: mockConn}

	conn := NewServerConnection("ws://localhost:8080", "test-node", "test-token", "test-org")
	conn.WithDialer(mockDialer)

	err := conn.Connect()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if mockDialer.dialCalls != 1 {
		t.Errorf("dial calls: got %v, want 1", mockDialer.dialCalls)
	}

	if conn.conn != mockConn {
		t.Error("connection not set correctly")
	}
}

func TestServerConnectionConnectError(t *testing.T) {
	mockDialer := &mockWebSocketDialer{
		dialError: websocket.ErrBadHandshake,
	}

	conn := NewServerConnection("ws://localhost:8080", "test-node", "test-token", "test-org")
	conn.WithDialer(mockDialer)

	err := conn.Connect()

	if err == nil {
		t.Error("expected error for failed connection")
	}
}

func TestServerConnectionSend(t *testing.T) {
	conn := NewServerConnection("ws://localhost:8080", "test-node", "test-token", "test-org")

	msg := ProtocolMessage{
		Type: MsgTypeHeartbeat,
	}

	conn.Send(msg)

	// Message should be in the send channel
	select {
	case received := <-conn.sendCh:
		if received.Type != MsgTypeHeartbeat {
			t.Errorf("Type: got %v, want %v", received.Type, MsgTypeHeartbeat)
		}
		if received.Timestamp == 0 {
			t.Error("Timestamp should be set")
		}
	default:
		t.Error("message should be in send channel")
	}
}

func TestServerConnectionSendWithBackpressure(t *testing.T) {
	conn := NewServerConnection("ws://localhost:8080", "test-node", "test-token", "test-org")

	msg := ProtocolMessage{
		Type: MsgTypeHeartbeat,
	}

	ok := conn.SendWithBackpressure(msg)

	if !ok {
		t.Error("SendWithBackpressure should return true")
	}
}

func TestServerConnectionSendEvent(t *testing.T) {
	conn := NewServerConnection("ws://localhost:8080", "test-node", "test-token", "test-org")

	data := map[string]interface{}{"key": "value"}
	err := conn.SendEvent(MsgTypeHeartbeat, data)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Message should be in the send channel
	select {
	case received := <-conn.sendCh:
		if received.Type != MsgTypeHeartbeat {
			t.Errorf("Type: got %v, want %v", received.Type, MsgTypeHeartbeat)
		}
	default:
		t.Error("message should be in send channel")
	}
}

func TestServerConnectionQueueMethods(t *testing.T) {
	conn := NewServerConnection("ws://localhost:8080", "test-node", "test-token", "test-org")

	// Initial state
	if conn.QueueLength() != 0 {
		t.Errorf("initial QueueLength: got %v, want 0", conn.QueueLength())
	}

	if conn.QueueCapacity() != 100 {
		t.Errorf("QueueCapacity: got %v, want 100", conn.QueueCapacity())
	}

	// Add a message
	conn.Send(ProtocolMessage{Type: MsgTypeHeartbeat})

	if conn.QueueLength() != 1 {
		t.Errorf("QueueLength after send: got %v, want 1", conn.QueueLength())
	}
}

func TestServerConnectionStop(t *testing.T) {
	mockConn := &mockWebSocketConn{}
	mockDialer := &mockWebSocketDialer{conn: mockConn}

	conn := NewServerConnection("ws://localhost:8080", "test-node", "test-token", "test-org")
	conn.WithDialer(mockDialer)
	conn.Connect()

	conn.Stop()

	// Verify connection was closed
	if !mockConn.closed {
		t.Error("connection should be closed")
	}
}

func TestServerConnectionStopTwice(t *testing.T) {
	conn := NewServerConnection("ws://localhost:8080", "test-node", "test-token", "test-org")

	// Should not panic when called twice
	conn.Stop()
	conn.Stop()
}

func TestServerConnectionSendBufferFull(t *testing.T) {
	// Create connection with a small buffer
	conn := &ServerConnection{
		sendCh: make(chan ProtocolMessage, 1),
		stopCh: make(chan struct{}),
	}

	// Fill the buffer
	conn.Send(ProtocolMessage{Type: MsgTypeHeartbeat})

	// This should be dropped (not block)
	conn.Send(ProtocolMessage{Type: MsgTypeHeartbeat})

	// Only one message should be in the channel
	if len(conn.sendCh) != 1 {
		t.Errorf("sendCh length: got %v, want 1", len(conn.sendCh))
	}
}

// --- Benchmark Tests ---

func BenchmarkReconnectStrategyNextDelay(b *testing.B) {
	strategy := NewReconnectStrategy(time.Second, time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		strategy.NextDelay()
		if i%10 == 0 {
			strategy.Reset()
		}
	}
}

func BenchmarkMessageRouterRoute(b *testing.B) {
	handler := &mockHandler{}
	sender := &mockEventSender{}
	router := NewMessageRouter(handler, sender)

	reqData, _ := json.Marshal(CreatePodRequest{
		PodKey: "pod-1",
	})

	msg := ProtocolMessage{
		Type: MsgTypeCreatePod,
		Data: reqData,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		router.Route(msg)
	}
}

func BenchmarkServerConnectionSend(b *testing.B) {
	conn := NewServerConnection("ws://localhost:8080", "test-node", "test-token", "test-org")

	msg := ProtocolMessage{
		Type: MsgTypeHeartbeat,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Drain channel to prevent blocking
		select {
		case <-conn.sendCh:
		default:
		}
		conn.Send(msg)
	}
}
