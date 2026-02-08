package monitor

import (
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/runner/internal/terminal"
)

// Tests for constants and basic structs

func TestAgentStateConstants(t *testing.T) {
	if terminal.StateUnknown != "unknown" {
		t.Errorf("StateUnknown: got %v, want unknown", terminal.StateUnknown)
	}
	if terminal.StateNotRunning != "not_running" {
		t.Errorf("StateNotRunning: got %v, want not_running", terminal.StateNotRunning)
	}
	if terminal.StateExecuting != "executing" {
		t.Errorf("StateExecuting: got %v, want executing", terminal.StateExecuting)
	}
	if terminal.StateWaiting != "waiting" {
		t.Errorf("StateWaiting: got %v, want waiting", terminal.StateWaiting)
	}
}

func TestPodStatusStruct(t *testing.T) {
	now := time.Now()
	status := PodStatus{
		PodID:       "pod-1",
		Pid:         12345,
		AgentStatus: terminal.StateExecuting,
		AgentPid:    67890,
		IsRunning:   true,
		UpdatedAt:   now,
	}

	if status.PodID != "pod-1" {
		t.Errorf("PodID: got %v, want pod-1", status.PodID)
	}

	if status.Pid != 12345 {
		t.Errorf("Pid: got %v, want 12345", status.Pid)
	}

	if status.AgentStatus != terminal.StateExecuting {
		t.Errorf("AgentStatus: got %v, want executing", status.AgentStatus)
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
