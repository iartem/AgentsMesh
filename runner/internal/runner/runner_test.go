package runner

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/config"
)

// --- Test Constants ---

func TestPodStatusConstants(t *testing.T) {
	if PodStatusInitializing != "initializing" {
		t.Errorf("PodStatusInitializing: got %v, want initializing", PodStatusInitializing)
	}
	if PodStatusRunning != "running" {
		t.Errorf("PodStatusRunning: got %v, want running", PodStatusRunning)
	}
	if PodStatusStopped != "stopped" {
		t.Errorf("PodStatusStopped: got %v, want stopped", PodStatusStopped)
	}
	if PodStatusFailed != "failed" {
		t.Errorf("PodStatusFailed: got %v, want failed", PodStatusFailed)
	}
}

// --- Test Pod Struct ---

func TestPodStruct(t *testing.T) {
	now := time.Now()
	pod := Pod{
		ID:               "pod-1",
		PodKey:       "key-123",
		AgentType:        "claude-code",
		RepositoryURL:    "https://github.com/test/repo.git",
		Branch:           "main",
		WorktreePath:     "/workspace/worktrees/pod-1",
		InitialPrompt:    "Hello",
		Terminal:         nil,
		StartedAt:        now,
		Status:           PodStatusRunning,
		TicketIdentifier: "TICKET-123",
	}

	if pod.ID != "pod-1" {
		t.Errorf("ID: got %v, want pod-1", pod.ID)
	}
	if pod.PodKey != "key-123" {
		t.Errorf("PodKey: got %v, want key-123", pod.PodKey)
	}
	if pod.AgentType != "claude-code" {
		t.Errorf("AgentType: got %v, want claude-code", pod.AgentType)
	}
	if pod.Status != PodStatusRunning {
		t.Errorf("Status: got %v, want running", pod.Status)
	}
	if pod.TicketIdentifier != "TICKET-123" {
		t.Errorf("TicketIdentifier: got %v, want TICKET-123", pod.TicketIdentifier)
	}
}

func TestPodAllFields(t *testing.T) {
	now := time.Now()
	forwarder := &PTYForwarder{podKey: "test"}

	pod := &Pod{
		ID:               "id-1",
		PodKey:       "key-1",
		AgentType:        "claude-code",
		RepositoryURL:    "https://github.com/test/repo.git",
		Branch:           "feature/test",
		WorktreePath:     "/workspace/worktrees/test",
		InitialPrompt:    "Hello, Claude!",
		Terminal:         nil,
		StartedAt:        now,
		Status:           PodStatusRunning,
		TicketIdentifier: "TICKET-123",
		OnOutput:         func([]byte) {},
		OnExit:           func(int) {},
		Forwarder:        forwarder,
	}

	if pod.OnOutput == nil {
		t.Error("OnOutput should not be nil")
	}
	if pod.OnExit == nil {
		t.Error("OnExit should not be nil")
	}
	if pod.Forwarder == nil {
		t.Error("Forwarder should not be nil")
	}
}

func TestPodWithCallbacks(t *testing.T) {
	outputCalled := false
	exitCalled := false

	pod := &Pod{
		ID:     "pod-1",
		Status: PodStatusRunning,
		OnOutput: func(data []byte) {
			outputCalled = true
		},
		OnExit: func(exitCode int) {
			exitCalled = true
		},
	}

	if pod.OnOutput != nil {
		pod.OnOutput([]byte("test"))
	}
	if pod.OnExit != nil {
		pod.OnExit(0)
	}

	if !outputCalled {
		t.Error("OnOutput should be called")
	}
	if !exitCalled {
		t.Error("OnExit should be called")
	}
}

// --- Test Payload Structs ---

func TestPodStartPayload(t *testing.T) {
	payload := PodStartPayload{
		PodKey:       "pod-1",
		AgentType:        "claude-code",
		LaunchCommand:    "claude",
		LaunchArgs:       []string{"--headless"},
		EnvVars:          map[string]string{"API_KEY": "secret"},
		RepositoryURL:    "https://github.com/test/repo.git",
		Branch:           "main",
		InitialPrompt:    "Hello",
		Rows:             24,
		Cols:             80,
		TicketIdentifier: "TICKET-123",
		PrepScript:       "npm install",
		PrepTimeout:      300,
	}

	if payload.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", payload.PodKey)
	}
	if payload.AgentType != "claude-code" {
		t.Errorf("AgentType: got %v, want claude-code", payload.AgentType)
	}
	if len(payload.LaunchArgs) != 1 {
		t.Errorf("LaunchArgs length: got %v, want 1", len(payload.LaunchArgs))
	}
	if payload.EnvVars["API_KEY"] != "secret" {
		t.Errorf("EnvVars[API_KEY]: got %v, want secret", payload.EnvVars["API_KEY"])
	}
}

func TestPodStartPayloadJSON(t *testing.T) {
	jsonStr := `{
		"pod_key": "pod-1",
		"agent_type": "claude-code",
		"launch_command": "claude",
		"launch_args": ["--headless"],
		"env_vars": {"API_KEY": "secret"},
		"rows": 24,
		"cols": 80,
		"ticket_identifier": "TICKET-123"
	}`

	var payload PodStartPayload
	if err := json.Unmarshal([]byte(jsonStr), &payload); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if payload.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", payload.PodKey)
	}
	if payload.TicketIdentifier != "TICKET-123" {
		t.Errorf("TicketIdentifier: got %v, want TICKET-123", payload.TicketIdentifier)
	}
}

func TestPodStopPayload(t *testing.T) {
	payload := PodStopPayload{PodKey: "pod-1"}

	if payload.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", payload.PodKey)
	}
}

func TestPodStopPayloadJSON(t *testing.T) {
	jsonStr := `{"pod_key": "pod-1"}`

	var payload PodStopPayload
	if err := json.Unmarshal([]byte(jsonStr), &payload); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if payload.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", payload.PodKey)
	}
}

func TestTerminalInputPayload(t *testing.T) {
	payload := TerminalInputPayload{
		PodKey: "pod-1",
		Data:       []byte("hello"),
	}

	if payload.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", payload.PodKey)
	}
	if string(payload.Data) != "hello" {
		t.Errorf("Data: got %v, want hello", string(payload.Data))
	}
}

func TestTerminalResizePayload(t *testing.T) {
	payload := TerminalResizePayload{
		PodKey: "pod-1",
		Rows:       40,
		Cols:       120,
	}

	if payload.PodKey != "pod-1" {
		t.Errorf("PodKey: got %v, want pod-1", payload.PodKey)
	}
	if payload.Rows != 40 {
		t.Errorf("Rows: got %v, want 40", payload.Rows)
	}
	if payload.Cols != 120 {
		t.Errorf("Cols: got %v, want 120", payload.Cols)
	}
}

func TestPodListPayloadJSON(t *testing.T) {
	jsonStr := `{"request_id": "req-123"}`

	var payload PodListPayload
	if err := json.Unmarshal([]byte(jsonStr), &payload); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if payload.RequestID != "req-123" {
		t.Errorf("RequestID: got %v, want req-123", payload.RequestID)
	}
}

// --- Test Runner Struct ---

func TestRunnerStruct(t *testing.T) {
	r := &Runner{
		pods: make(map[string]*Pod),
		stopChan: make(chan struct{}),
	}

	if r.pods == nil {
		t.Error("pods should be initialized")
	}
	if r.stopChan == nil {
		t.Error("stopChan should be initialized")
	}
}

func TestRunnerStructFields(t *testing.T) {
	r := &Runner{
		cfg: &config.Config{
			NodeID:                "node-1",
			MaxConcurrentPods: 5,
		},
		pods: make(map[string]*Pod),
		stopChan: make(chan struct{}),
	}

	if r.cfg == nil {
		t.Error("cfg should not be nil")
	}
	if r.cfg.NodeID != "node-1" {
		t.Errorf("cfg.NodeID = %v, want node-1", r.cfg.NodeID)
	}
	if r.pods == nil {
		t.Error("pods should not be nil")
	}
}

// --- Test Runner Methods ---

func TestRunnerStopAllPodsEmpty(t *testing.T) {
	store := NewInMemoryPodStore()
	r := &Runner{
		pods:     make(map[string]*Pod),
		podStore: store,
	}

	// Should not panic with empty pods
	r.stopAllPods()
}

func TestRunnerStopAllPodsWithNilTerminal(t *testing.T) {
	store := NewInMemoryPodStore()
	store.Put("pod-1", &Pod{ID: "pod-1", PodKey: "pod-1", Terminal: nil})
	r := &Runner{
		pods:     make(map[string]*Pod),
		podStore: store,
	}

	// Should not panic
	r.stopAllPods()

	if store.Count() != 0 {
		t.Errorf("pods should be empty after stopAllPods")
	}
}

// --- Test buildWebSocketBaseURL ---

func TestBuildWebSocketBaseURLHTTP(t *testing.T) {
	result := buildWebSocketBaseURL("http://localhost:8080")
	expected := "ws://localhost:8080"
	if result != expected {
		t.Errorf("buildWebSocketBaseURL(http): got %v, want %v", result, expected)
	}
}

func TestBuildWebSocketBaseURLHTTPS(t *testing.T) {
	result := buildWebSocketBaseURL("https://api.example.com")
	expected := "wss://api.example.com"
	if result != expected {
		t.Errorf("buildWebSocketBaseURL(https): got %v, want %v", result, expected)
	}
}

// --- Test ExtendedPod ---

func TestExtendedPodStruct(t *testing.T) {
	pod := &Pod{ID: "pod-1", Status: PodStatusRunning}

	extended := ExtendedPod{
		Pod:                    pod,
		OnOutput:               func([]byte) {},
		OnExit:                 func(int) {},
		TicketIdentifier:       "TICKET-123",
		ManagedTerminalSession: nil,
	}

	if extended.ID != "pod-1" {
		t.Errorf("ID: got %v, want pod-1", extended.ID)
	}
	if extended.TicketIdentifier != "TICKET-123" {
		t.Errorf("TicketIdentifier: got %v, want TICKET-123", extended.TicketIdentifier)
	}
}

// --- Benchmarks ---

func BenchmarkPodStartPayloadUnmarshal(b *testing.B) {
	jsonStr := []byte(`{
		"pod_key": "pod-1",
		"agent_type": "claude-code",
		"launch_command": "claude",
		"launch_args": ["--headless"],
		"rows": 24,
		"cols": 80
	}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var payload PodStartPayload
		json.Unmarshal(jsonStr, &payload)
	}
}

func BenchmarkBuildWebSocketBaseURL(b *testing.B) {
	urls := []string{
		"http://localhost:8080",
		"https://api.example.com",
		"http://192.168.1.1:3000",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buildWebSocketBaseURL(urls[i%len(urls)])
	}
}
