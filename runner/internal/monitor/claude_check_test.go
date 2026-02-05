package monitor

import (
	"sync"
	"testing"
	"time"
)

// Tests for pod status checking and process inspection

func TestMonitorCheckPodNotRunning(t *testing.T) {
	inspector := newMockInspector()
	monitor := NewMonitorWithInspector(50*time.Millisecond, inspector)

	// Register a pod with a non-running process
	inspector.isRunning[12345] = false

	monitor.SetCallback(func(status PodStatus) {
		// callback for status changes
	})

	monitor.RegisterPod("pod-1", 12345)
	monitor.Start()

	// Wait for check to happen
	time.Sleep(150 * time.Millisecond)
	monitor.Stop()

	status, _ := monitor.GetStatus("pod-1")
	if status.IsRunning {
		t.Error("pod should not be running")
	}

	if status.ClaudeStatus != StatusNotRunning {
		t.Errorf("ClaudeStatus: got %v, want not_running", status.ClaudeStatus)
	}
}

func TestMonitorCheckPodRunningNoClause(t *testing.T) {
	inspector := newMockInspector()
	monitor := NewMonitorWithInspector(50*time.Millisecond, inspector)

	// Register a pod with a running process but no claude child
	inspector.isRunning[12345] = true
	inspector.childProcesses[12345] = []int{} // No children

	monitor.RegisterPod("pod-1", 12345)
	monitor.Start()

	// Wait for check to happen
	time.Sleep(150 * time.Millisecond)
	monitor.Stop()

	status, _ := monitor.GetStatus("pod-1")
	if !status.IsRunning {
		t.Error("pod should be running")
	}

	if status.ClaudeStatus != StatusNotRunning {
		t.Errorf("ClaudeStatus without claude child: got %v, want not_running", status.ClaudeStatus)
	}
}

func TestMonitorCheckPodWithClaudeExecuting(t *testing.T) {
	inspector := newMockInspector()
	monitor := NewMonitorWithInspector(50*time.Millisecond, inspector)

	// Register a pod with claude running and executing
	inspector.isRunning[12345] = true
	inspector.childProcesses[12345] = []int{67890}
	inspector.processNames[67890] = "claude"
	inspector.childProcesses[67890] = []int{11111} // claude has children
	inspector.processStates[11111] = "R"           // child is running

	monitor.RegisterPod("pod-1", 12345)
	monitor.Start()

	// Wait for check to happen
	time.Sleep(150 * time.Millisecond)
	monitor.Stop()

	status, _ := monitor.GetStatus("pod-1")
	if status.ClaudePid != 67890 {
		t.Errorf("ClaudePid: got %v, want 67890", status.ClaudePid)
	}

	if status.ClaudeStatus != StatusExecuting {
		t.Errorf("ClaudeStatus: got %v, want executing", status.ClaudeStatus)
	}
}

func TestMonitorCheckPodWithClaudeWaiting(t *testing.T) {
	inspector := newMockInspector()
	monitor := NewMonitorWithInspector(50*time.Millisecond, inspector)

	// Register a pod with claude running but waiting (no active children)
	inspector.isRunning[12345] = true
	inspector.childProcesses[12345] = []int{67890}
	inspector.processNames[67890] = "claude"
	inspector.childProcesses[67890] = []int{} // No children

	monitor.RegisterPod("pod-1", 12345)
	monitor.Start()

	// Wait for check to happen
	time.Sleep(150 * time.Millisecond)
	monitor.Stop()

	status, _ := monitor.GetStatus("pod-1")
	if status.ClaudeStatus != StatusWaiting {
		t.Errorf("ClaudeStatus: got %v, want waiting", status.ClaudeStatus)
	}
}

func TestMonitorFindClaudeProcessRecursive(t *testing.T) {
	inspector := newMockInspector()
	monitor := NewMonitorWithInspector(time.Second, inspector)

	// Claude is nested under another process
	inspector.childProcesses[1] = []int{2}
	inspector.processNames[2] = "bash"
	inspector.childProcesses[2] = []int{3}
	inspector.processNames[3] = "claude"

	claudePid := monitor.findClaudeProcess(1)
	if claudePid != 3 {
		t.Errorf("findClaudeProcess: got %v, want 3", claudePid)
	}
}

func TestMonitorFindClaudeProcessNotFound(t *testing.T) {
	inspector := newMockInspector()
	monitor := NewMonitorWithInspector(time.Second, inspector)

	inspector.childProcesses[1] = []int{2}
	inspector.processNames[2] = "bash"

	claudePid := monitor.findClaudeProcess(1)
	if claudePid != 0 {
		t.Errorf("findClaudeProcess: got %v, want 0", claudePid)
	}
}

func TestMonitorStatusChangeCallback(t *testing.T) {
	inspector := newMockInspector()
	monitor := NewMonitorWithInspector(50*time.Millisecond, inspector)

	var receivedStatuses []PodStatus
	var mu sync.Mutex

	monitor.SetCallback(func(status PodStatus) {
		mu.Lock()
		receivedStatuses = append(receivedStatuses, status)
		mu.Unlock()
	})

	// Start with running process
	inspector.isRunning[12345] = true
	inspector.childProcesses[12345] = []int{}

	monitor.RegisterPod("pod-1", 12345)
	monitor.Start()

	// Wait for initial check
	time.Sleep(100 * time.Millisecond)

	// Change to have claude running
	inspector.mu.Lock()
	inspector.childProcesses[12345] = []int{67890}
	inspector.processNames[67890] = "claude"
	inspector.mu.Unlock()

	// Wait for another check
	time.Sleep(100 * time.Millisecond)
	monitor.Stop()

	mu.Lock()
	defer mu.Unlock()

	// Should have received at least one status change
	if len(receivedStatuses) == 0 {
		t.Error("should have received status changes")
	}
}
