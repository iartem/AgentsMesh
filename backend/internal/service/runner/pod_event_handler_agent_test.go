package runner

import (
	"testing"

	"github.com/anthropics/agentsmesh/backend/internal/domain/agentpod"
	"github.com/anthropics/agentsmesh/backend/internal/domain/runner"
	runnerv1 "github.com/anthropics/agentsmesh/proto/gen/go/runner/v1"
)

func TestHandleAgentStatus(t *testing.T) {
	pc, _, _ := setupPodEventHandlerDeps(t)

	// Create a runner
	r := &runner.Runner{
		OrganizationID: 1,
		NodeID:         "agent-status-node",
		Status:         "online",
	}
	if err := pc.db.Create(r).Error; err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	// Create a running pod
	pc.db.Exec(`INSERT INTO pods (pod_key, runner_id, status) VALUES (?, ?, ?)`,
		"agent-pod-1", r.ID, agentpod.StatusRunning)

	// Track status change callback
	var callbackAgentStatus string
	pc.SetStatusChangeCallback(func(podKey string, status string, agentStatus string) {
		callbackAgentStatus = agentStatus
	})

	// Handle agent status change (using Proto type)
	data := &runnerv1.AgentStatusEvent{
		PodKey: "agent-pod-1",
		Status: "thinking",
	}

	pc.handleAgentStatus(r.ID, data)

	// Verify pod was updated
	var agentStatus string
	pc.db.Raw(`SELECT agent_status FROM pods WHERE pod_key = ?`, "agent-pod-1").
		Scan(&agentStatus)

	if agentStatus != "thinking" {
		t.Errorf("agent_status: got %q, want %q", agentStatus, "thinking")
	}

	// Verify callback was called
	if callbackAgentStatus != "thinking" {
		t.Errorf("callback agentStatus: got %q, want %q", callbackAgentStatus, "thinking")
	}
}

func TestHandleAgentStatusPreservesPtyPid(t *testing.T) {
	pc, _, _ := setupPodEventHandlerDeps(t)

	// Create a runner
	r := &runner.Runner{
		OrganizationID: 1,
		NodeID:         "agent-nopid-node",
		Status:         "online",
	}
	if err := pc.db.Create(r).Error; err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	// Create a running pod with existing pid
	pc.db.Exec(`INSERT INTO pods (pod_key, runner_id, status, pty_pid) VALUES (?, ?, ?, ?)`,
		"agent-nopid-1", r.ID, agentpod.StatusRunning, 11111)

	// Handle agent status change (using Proto type)
	data := &runnerv1.AgentStatusEvent{
		PodKey: "agent-nopid-1",
		Status: "idle",
	}

	pc.handleAgentStatus(r.ID, data)

	// Verify agent_status was updated but pid was not changed
	var agentStatus string
	var pid int
	pc.db.Raw(`SELECT agent_status, pty_pid FROM pods WHERE pod_key = ?`, "agent-nopid-1").
		Row().Scan(&agentStatus, &pid)

	if agentStatus != "idle" {
		t.Errorf("agent_status: got %q, want %q", agentStatus, "idle")
	}
	if pid != 11111 {
		t.Errorf("pty_pid should not change: got %d, want 11111", pid)
	}
}

func TestHandleRunnerDisconnect(t *testing.T) {
	pc, _, _ := setupPodEventHandlerDeps(t)

	// Create a runner
	r := &runner.Runner{
		OrganizationID: 1,
		NodeID:         "disconnect-node",
		Status:         "online",
	}
	if err := pc.db.Create(r).Error; err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	// Create a running pod
	pc.db.Exec(`INSERT INTO pods (pod_key, runner_id, status) VALUES (?, ?, ?)`,
		"disconnect-pod-1", r.ID, agentpod.StatusRunning)

	// Handle runner disconnect
	pc.handleRunnerDisconnect(r.ID)

	// Verify runner was marked as offline
	var updated runner.Runner
	pc.db.First(&updated, r.ID)
	if updated.Status != "offline" {
		t.Errorf("runner status: got %q, want %q", updated.Status, "offline")
	}

	// Verify pod is NOT immediately orphaned (by design)
	var podStatus string
	pc.db.Raw(`SELECT status FROM pods WHERE pod_key = ?`, "disconnect-pod-1").Scan(&podStatus)
	if podStatus != agentpod.StatusRunning {
		t.Errorf("pod should still be running (not immediately orphaned): got %q", podStatus)
	}
}

func TestHandlePodInitProgress(t *testing.T) {
	pc, _, _ := setupPodEventHandlerDeps(t)

	// Track callback invocation
	var callbackPodKey, callbackPhase, callbackMessage string
	var callbackProgress int
	pc.SetInitProgressCallback(func(podKey, phase string, progress int, message string) {
		callbackPodKey = podKey
		callbackPhase = phase
		callbackProgress = progress
		callbackMessage = message
	})

	// Handle pod init progress event
	data := &runnerv1.PodInitProgressEvent{
		PodKey:   "init-pod-1",
		Phase:    "pulling_image",
		Progress: 50,
		Message:  "Pulling container image...",
	}

	pc.handlePodInitProgress(1, data)

	// Verify callback was called with correct data
	if callbackPodKey != "init-pod-1" {
		t.Errorf("callback podKey: got %q, want %q", callbackPodKey, "init-pod-1")
	}
	if callbackPhase != "pulling_image" {
		t.Errorf("callback phase: got %q, want %q", callbackPhase, "pulling_image")
	}
	if callbackProgress != 50 {
		t.Errorf("callback progress: got %d, want %d", callbackProgress, 50)
	}
	if callbackMessage != "Pulling container image..." {
		t.Errorf("callback message: got %q", callbackMessage)
	}
}

func TestHandlePodInitProgressNoCallback(t *testing.T) {
	pc, _, _ := setupPodEventHandlerDeps(t)

	// No callback set - should not panic
	data := &runnerv1.PodInitProgressEvent{
		PodKey:   "init-pod-2",
		Phase:    "init",
		Progress: 10,
	}

	// This should not panic
	pc.handlePodInitProgress(1, data)
}
