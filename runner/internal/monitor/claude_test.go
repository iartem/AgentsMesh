package monitor

import (
	"sync"
	"testing"
	"time"
)

// mockInspector is a mock implementation of process.Inspector for testing
type mockInspector struct {
	isRunning        map[int]bool
	processNames     map[int]string
	childProcesses   map[int][]int
	processStates    map[int]string
	hasOpenFilesMap  map[int]bool
	mu               sync.Mutex
}

func newMockInspector() *mockInspector {
	return &mockInspector{
		isRunning:        make(map[int]bool),
		processNames:     make(map[int]string),
		childProcesses:   make(map[int][]int),
		processStates:    make(map[int]string),
		hasOpenFilesMap:  make(map[int]bool),
	}
}

func (m *mockInspector) GetChildProcesses(pid int) []int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.childProcesses[pid]
}

func (m *mockInspector) GetProcessName(pid int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.processNames[pid]
}

func (m *mockInspector) IsRunning(pid int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isRunning[pid]
}

func (m *mockInspector) GetState(pid int) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.processStates[pid]
}

func (m *mockInspector) HasOpenFiles(pid int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.hasOpenFilesMap[pid]
}

// --- Test Constants ---

func TestClaudeStatusConstants(t *testing.T) {
	if StatusUnknown != "unknown" {
		t.Errorf("StatusUnknown: got %v, want unknown", StatusUnknown)
	}
	if StatusNotRunning != "not_running" {
		t.Errorf("StatusNotRunning: got %v, want not_running", StatusNotRunning)
	}
	if StatusExecuting != "executing" {
		t.Errorf("StatusExecuting: got %v, want executing", StatusExecuting)
	}
	if StatusWaiting != "waiting" {
		t.Errorf("StatusWaiting: got %v, want waiting", StatusWaiting)
	}
}

// --- Test PodStatus ---

func TestPodStatusStruct(t *testing.T) {
	now := time.Now()
	status := PodStatus{
		PodID:    "pod-1",
		Pid:          12345,
		ClaudeStatus: StatusExecuting,
		ClaudePid:    67890,
		IsRunning:    true,
		UpdatedAt:    now,
	}

	if status.PodID != "pod-1" {
		t.Errorf("PodID: got %v, want pod-1", status.PodID)
	}

	if status.Pid != 12345 {
		t.Errorf("Pid: got %v, want 12345", status.Pid)
	}

	if status.ClaudeStatus != StatusExecuting {
		t.Errorf("ClaudeStatus: got %v, want executing", status.ClaudeStatus)
	}

	if !status.IsRunning {
		t.Error("IsRunning should be true")
	}
}

// --- Test Monitor ---

func TestNewMonitor(t *testing.T) {
	monitor := NewMonitor(time.Second)

	if monitor == nil {
		t.Fatal("NewMonitor returned nil")
	}

	if monitor.interval != time.Second {
		t.Errorf("interval: got %v, want %v", monitor.interval, time.Second)
	}

	if monitor.statuses == nil {
		t.Error("statuses map should be initialized")
	}

	if monitor.inspector == nil {
		t.Error("inspector should be initialized")
	}
}

func TestNewMonitorWithInspector(t *testing.T) {
	inspector := newMockInspector()
	monitor := NewMonitorWithInspector(time.Second, inspector)

	if monitor == nil {
		t.Fatal("NewMonitorWithInspector returned nil")
	}

	if monitor.inspector != inspector {
		t.Error("inspector should be the provided one")
	}
}

func TestMonitorSetCallback(t *testing.T) {
	monitor := NewMonitor(time.Second)

	callback := func(status PodStatus) {
		// callback implementation
	}

	monitor.SetCallback(callback)

	// SetCallback internally calls Subscribe("default", callback)
	// Verify by checking subscribers map size
	monitor.subMu.RLock()
	hasDefaultSubscriber := monitor.subscribers["default"] != nil
	monitor.subMu.RUnlock()

	if !hasDefaultSubscriber {
		t.Error("callback should be set as default subscriber")
	}
}

func TestMonitorSubscribeUnsubscribe(t *testing.T) {
	monitor := NewMonitor(time.Second)

	var callCount int
	callback := func(status PodStatus) {
		callCount++
	}

	// Subscribe
	monitor.Subscribe("test-sub", callback)

	monitor.subMu.RLock()
	hasSubscriber := monitor.subscribers["test-sub"] != nil
	monitor.subMu.RUnlock()

	if !hasSubscriber {
		t.Error("subscriber should be registered")
	}

	// Unsubscribe
	monitor.Unsubscribe("test-sub")

	monitor.subMu.RLock()
	hasSubscriberAfterUnsub := monitor.subscribers["test-sub"] != nil
	monitor.subMu.RUnlock()

	if hasSubscriberAfterUnsub {
		t.Error("subscriber should be removed after unsubscribe")
	}
}

func TestMonitorRegisterPod(t *testing.T) {
	monitor := NewMonitor(time.Second)

	monitor.RegisterPod("pod-1", 12345)

	status, ok := monitor.GetStatus("pod-1")
	if !ok {
		t.Fatal("pod should be registered")
	}

	if status.PodID != "pod-1" {
		t.Errorf("PodID: got %v, want pod-1", status.PodID)
	}

	if status.Pid != 12345 {
		t.Errorf("Pid: got %v, want 12345", status.Pid)
	}

	if status.ClaudeStatus != StatusUnknown {
		t.Errorf("ClaudeStatus: got %v, want unknown", status.ClaudeStatus)
	}

	if !status.IsRunning {
		t.Error("IsRunning should be true")
	}
}

func TestMonitorUnregisterPod(t *testing.T) {
	monitor := NewMonitor(time.Second)

	monitor.RegisterPod("pod-1", 12345)
	monitor.UnregisterPod("pod-1")

	_, ok := monitor.GetStatus("pod-1")
	if ok {
		t.Error("pod should be unregistered")
	}
}

func TestMonitorGetStatusNotFound(t *testing.T) {
	monitor := NewMonitor(time.Second)

	_, ok := monitor.GetStatus("nonexistent")
	if ok {
		t.Error("should return false for nonexistent pod")
	}
}

func TestMonitorGetAllStatuses(t *testing.T) {
	monitor := NewMonitor(time.Second)

	monitor.RegisterPod("pod-1", 12345)
	monitor.RegisterPod("pod-2", 67890)

	statuses := monitor.GetAllStatuses()

	if len(statuses) != 2 {
		t.Errorf("statuses length: got %v, want 2", len(statuses))
	}
}

func TestMonitorGetAllStatusesEmpty(t *testing.T) {
	monitor := NewMonitor(time.Second)

	statuses := monitor.GetAllStatuses()

	if len(statuses) != 0 {
		t.Errorf("statuses should be empty, got %v", len(statuses))
	}
}

func TestMonitorStartStop(t *testing.T) {
	monitor := NewMonitor(100 * time.Millisecond)

	monitor.Start()

	// Give it time to run a few cycles
	time.Sleep(250 * time.Millisecond)

	monitor.Stop()

	// Should not panic when called twice
	monitor.Stop()
}

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
	inspector.processStates[11111] = "R"            // child is running

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

func TestMonitorHasActiveChildrenRunning(t *testing.T) {
	inspector := newMockInspector()
	monitor := NewMonitorWithInspector(time.Second, inspector)

	inspector.childProcesses[1] = []int{2}
	inspector.processStates[2] = "R" // Running state

	if !monitor.hasActiveChildren(1) {
		t.Error("hasActiveChildren should return true for running child")
	}
}

func TestMonitorHasActiveChildrenWithOpenFiles(t *testing.T) {
	inspector := newMockInspector()
	monitor := NewMonitorWithInspector(time.Second, inspector)

	inspector.childProcesses[1] = []int{2}
	inspector.processStates[2] = "S" // Sleeping
	inspector.hasOpenFilesMap[2] = true

	if !monitor.hasActiveChildren(1) {
		t.Error("hasActiveChildren should return true for child with open files")
	}
}

func TestMonitorHasActiveChildrenRecursive(t *testing.T) {
	inspector := newMockInspector()
	monitor := NewMonitorWithInspector(time.Second, inspector)

	inspector.childProcesses[1] = []int{2}
	inspector.processStates[2] = "S"
	inspector.childProcesses[2] = []int{3}
	inspector.processStates[3] = "R" // Grandchild is running

	if !monitor.hasActiveChildren(1) {
		t.Error("hasActiveChildren should return true for active grandchild")
	}
}

func TestMonitorHasActiveChildrenNone(t *testing.T) {
	inspector := newMockInspector()
	monitor := NewMonitorWithInspector(time.Second, inspector)

	inspector.childProcesses[1] = []int{2}
	inspector.processStates[2] = "S" // Sleeping
	inspector.hasOpenFilesMap[2] = false

	if monitor.hasActiveChildren(1) {
		t.Error("hasActiveChildren should return false when no active children")
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

// --- Benchmark Tests ---

func BenchmarkMonitorCheckAllPods(b *testing.B) {
	inspector := newMockInspector()
	monitor := NewMonitorWithInspector(time.Second, inspector)

	// Register 10 pods
	for i := 0; i < 10; i++ {
		pid := 12345 + i
		inspector.isRunning[pid] = true
		monitor.RegisterPod("pod-"+string(rune('0'+i)), pid)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.checkAllPods()
	}
}

func BenchmarkMonitorGetStatus(b *testing.B) {
	monitor := NewMonitor(time.Second)
	monitor.RegisterPod("pod-1", 12345)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.GetStatus("pod-1")
	}
}
