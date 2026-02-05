package runner

import (
	"context"
	"testing"

	"github.com/anthropics/agentsmesh/backend/internal/domain/agentpod"
	"github.com/anthropics/agentsmesh/backend/internal/domain/runner"
	runnerv1 "github.com/anthropics/agentsmesh/proto/gen/go/runner/v1"
)

func TestHandleHeartbeat(t *testing.T) {
	pc, _, _ := setupPodEventHandlerDeps(t)

	// Create a runner
	r := &runner.Runner{
		OrganizationID: 1,
		NodeID:         "heartbeat-node",
		Status:         "online",
	}
	if err := pc.db.Create(r).Error; err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	// Create a pod
	pc.db.Exec(`INSERT INTO pods (pod_key, runner_id, status) VALUES (?, ?, ?)`,
		"heartbeat-pod-1", r.ID, agentpod.StatusRunning)

	// Send heartbeat (using Proto type)
	data := &runnerv1.HeartbeatData{
		Pods: []*runnerv1.PodInfo{
			{PodKey: "heartbeat-pod-1", Status: "running"},
		},
	}

	pc.handleHeartbeat(r.ID, data)

	// Verify heartbeat was recorded (check buffer)
	if pc.heartbeatBatcher.BufferSize() != 1 {
		t.Errorf("heartbeat should be recorded, buffer size: %d", pc.heartbeatBatcher.BufferSize())
	}
}

func TestHandleHeartbeatReconcilePods(t *testing.T) {
	pc, _, tr := setupPodEventHandlerDeps(t)

	// Create a runner
	r := &runner.Runner{
		OrganizationID: 1,
		NodeID:         "reconcile-node",
		Status:         "online",
	}
	if err := pc.db.Create(r).Error; err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	// Create pods in DB
	pc.db.Exec(`INSERT INTO pods (pod_key, runner_id, status) VALUES (?, ?, ?)`,
		"reconcile-pod-1", r.ID, agentpod.StatusRunning)
	pc.db.Exec(`INSERT INTO pods (pod_key, runner_id, status) VALUES (?, ?, ?)`,
		"reconcile-pod-2", r.ID, agentpod.StatusRunning)

	// Send heartbeat with only pod-1 (using Proto type)
	data := &runnerv1.HeartbeatData{
		Pods: []*runnerv1.PodInfo{
			{PodKey: "reconcile-pod-1", Status: "running"},
		},
	}

	pc.handleHeartbeat(r.ID, data)

	// Verify pod-1 is still running and registered
	var status1 string
	pc.db.Raw(`SELECT status FROM pods WHERE pod_key = ?`, "reconcile-pod-1").Scan(&status1)
	if status1 != agentpod.StatusRunning {
		t.Errorf("pod-1 status: got %q, want %q", status1, agentpod.StatusRunning)
	}
	if !tr.IsPodRegistered("reconcile-pod-1") {
		t.Error("pod-1 should be registered with terminal router")
	}

	// Verify pod-2 is orphaned
	var status2 string
	pc.db.Raw(`SELECT status FROM pods WHERE pod_key = ?`, "reconcile-pod-2").Scan(&status2)
	if status2 != agentpod.StatusOrphaned {
		t.Errorf("pod-2 status: got %q, want %q", status2, agentpod.StatusOrphaned)
	}
}

func TestHandleHeartbeatRestoreOrphanedPod(t *testing.T) {
	pc, _, _ := setupPodEventHandlerDeps(t)

	// Create a runner
	r := &runner.Runner{
		OrganizationID: 1,
		NodeID:         "restore-node",
		Status:         "online",
	}
	if err := pc.db.Create(r).Error; err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	// Create an orphaned pod
	pc.db.Exec(`INSERT INTO pods (pod_key, runner_id, status) VALUES (?, ?, ?)`,
		"orphan-pod-1", r.ID, agentpod.StatusOrphaned)

	// Send heartbeat reporting the orphaned pod as running (using Proto type)
	data := &runnerv1.HeartbeatData{
		Pods: []*runnerv1.PodInfo{
			{PodKey: "orphan-pod-1", Status: "running"},
		},
	}

	pc.handleHeartbeat(r.ID, data)

	// Verify pod was restored
	var status string
	pc.db.Raw(`SELECT status FROM pods WHERE pod_key = ?`, "orphan-pod-1").Scan(&status)
	if status != agentpod.StatusRunning {
		t.Errorf("orphaned pod should be restored: got %q, want %q", status, agentpod.StatusRunning)
	}
}

func TestReconcilePods(t *testing.T) {
	pc, _, tr := setupPodEventHandlerDeps(t)

	// Create a runner
	r := &runner.Runner{
		OrganizationID: 1,
		NodeID:         "reconcile-test-node",
		Status:         "online",
	}
	if err := pc.db.Create(r).Error; err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	// Create multiple pods
	pc.db.Exec(`INSERT INTO pods (pod_key, runner_id, status) VALUES (?, ?, ?)`,
		"recon-pod-1", r.ID, agentpod.StatusRunning)
	pc.db.Exec(`INSERT INTO pods (pod_key, runner_id, status) VALUES (?, ?, ?)`,
		"recon-pod-2", r.ID, agentpod.StatusRunning)
	pc.db.Exec(`INSERT INTO pods (pod_key, runner_id, status) VALUES (?, ?, ?)`,
		"recon-pod-3", r.ID, agentpod.StatusInitializing)

	ctx := context.Background()
	reportedPods := map[string]bool{
		"recon-pod-1": true,
		// pod-2 and pod-3 are NOT reported
	}

	pc.reconcilePods(ctx, r.ID, reportedPods)

	// Verify pod-1 is registered
	if !tr.IsPodRegistered("recon-pod-1") {
		t.Error("pod-1 should be registered")
	}

	// Verify pod-2 and pod-3 are orphaned
	var status2, status3 string
	pc.db.Raw(`SELECT status FROM pods WHERE pod_key = ?`, "recon-pod-2").Scan(&status2)
	pc.db.Raw(`SELECT status FROM pods WHERE pod_key = ?`, "recon-pod-3").Scan(&status3)

	if status2 != agentpod.StatusOrphaned {
		t.Errorf("pod-2 should be orphaned: got %q", status2)
	}
	if status3 != agentpod.StatusOrphaned {
		t.Errorf("pod-3 should be orphaned: got %q", status3)
	}
}

func TestReconcilePodsCompletedNotAffected(t *testing.T) {
	pc, _, _ := setupPodEventHandlerDeps(t)

	// Create a runner
	r := &runner.Runner{
		OrganizationID: 1,
		NodeID:         "completed-node",
		Status:         "online",
	}
	if err := pc.db.Create(r).Error; err != nil {
		t.Fatalf("failed to create runner: %v", err)
	}

	// Create a completed pod (should not be affected by reconciliation)
	pc.db.Exec(`INSERT INTO pods (pod_key, runner_id, status) VALUES (?, ?, ?)`,
		"completed-pod-1", r.ID, agentpod.StatusCompleted)

	ctx := context.Background()
	reportedPods := map[string]bool{} // Empty - no pods reported

	pc.reconcilePods(ctx, r.ID, reportedPods)

	// Verify completed pod is NOT changed
	var status string
	pc.db.Raw(`SELECT status FROM pods WHERE pod_key = ?`, "completed-pod-1").Scan(&status)
	if status != agentpod.StatusCompleted {
		t.Errorf("completed pod should not be affected: got %q", status)
	}
}
