package monitor

import (
	"testing"
	"time"
)

// Tests for constants and basic structs

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

func TestPodStatusStruct(t *testing.T) {
	now := time.Now()
	status := PodStatus{
		PodID:        "pod-1",
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
