package autopilot

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	runnerv1 "github.com/anthropics/agentsmesh/proto/gen/go/runner/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAutopilotController_HandleDecision_Completed(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test that requires shell execution in CI environment")
	}
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "autopilot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create mock agent that returns TASK_COMPLETED
	scriptPath := filepath.Join(tmpDir, "mock_agent")
	script := `#!/bin/bash
echo "TASK_COMPLETED"
echo "All tasks done."
`
	err = os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)

	protoConfig := &runnerv1.AutopilotConfig{
		InitialPrompt:    "Test",
		MaxIterations:    10,
		ControlAgentType: scriptPath,
	}

	workerCtrl := &MockPodController{
		workDir:     tmpDir,
		podKey:      "worker-123",
		agentStatus: "waiting",
	}

	reporter := &MockEventReporter{}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  protoConfig,
		PodCtrl:   workerCtrl,
		Reporter:     reporter,
		MCPPort:      19000,
	})

	err = rp.Start()
	require.NoError(t, err)

	// Wait for decision to complete
	time.Sleep(500 * time.Millisecond)

	status := rp.GetStatus()
	assert.Equal(t, PhaseCompleted, status.Phase)

	// Check terminated event
	hasTerminated := false
	for _, e := range reporter.GetTerminatedEvents() {
		if e.Reason == "completed" {
			hasTerminated = true
			break
		}
	}
	assert.True(t, hasTerminated)

	rp.Stop()
}

func TestAutopilotController_HandleDecision_NeedHumanHelp(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test that requires shell execution in CI environment")
	}
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "autopilot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create mock agent that returns NEED_HUMAN_HELP
	scriptPath := filepath.Join(tmpDir, "mock_agent")
	script := `#!/bin/bash
echo "NEED_HUMAN_HELP"
echo "Need credentials."
`
	err = os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)

	protoConfig := &runnerv1.AutopilotConfig{
		InitialPrompt:    "Test",
		MaxIterations:    10,
		ControlAgentType: scriptPath,
	}

	workerCtrl := &MockPodController{
		workDir:     tmpDir,
		podKey:      "worker-123",
		agentStatus: "waiting",
	}

	reporter := &MockEventReporter{}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  protoConfig,
		PodCtrl:   workerCtrl,
		Reporter:     reporter,
		MCPPort:      19000,
	})

	err = rp.Start()
	require.NoError(t, err)

	// Wait for decision to complete
	time.Sleep(500 * time.Millisecond)

	status := rp.GetStatus()
	assert.Equal(t, PhaseWaitingApproval, status.Phase)

	rp.Stop()
}

func TestAutopilotController_HandleDecision_GiveUp(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test that requires shell execution in CI environment")
	}
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "autopilot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create mock agent that returns GIVE_UP
	scriptPath := filepath.Join(tmpDir, "mock_agent")
	script := `#!/bin/bash
echo "GIVE_UP"
echo "Cannot complete."
`
	err = os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)

	protoConfig := &runnerv1.AutopilotConfig{
		InitialPrompt:    "Test",
		MaxIterations:    10,
		ControlAgentType: scriptPath,
	}

	workerCtrl := &MockPodController{
		workDir:     tmpDir,
		podKey:      "worker-123",
		agentStatus: "waiting",
	}

	reporter := &MockEventReporter{}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  protoConfig,
		PodCtrl:   workerCtrl,
		Reporter:     reporter,
		MCPPort:      19000,
	})

	err = rp.Start()
	require.NoError(t, err)

	// Wait for decision to complete
	time.Sleep(500 * time.Millisecond)

	status := rp.GetStatus()
	assert.Equal(t, PhaseFailed, status.Phase)

	// Check terminated event
	hasTerminated := false
	for _, e := range reporter.GetTerminatedEvents() {
		if e.Reason == "failed" {
			hasTerminated = true
			break
		}
	}
	assert.True(t, hasTerminated)

	rp.Stop()
}

func TestAutopilotController_HandleDecision_Continue(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test that requires shell execution in CI environment")
	}
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "autopilot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create mock agent that returns CONTINUE
	scriptPath := filepath.Join(tmpDir, "mock_agent")
	script := `#!/bin/bash
echo "CONTINUE"
echo "Working on it."
`
	err = os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)

	protoConfig := &runnerv1.AutopilotConfig{
		InitialPrompt:    "Test",
		MaxIterations:    10,
		ControlAgentType: scriptPath,
	}

	workerCtrl := &MockPodController{
		workDir:     tmpDir,
		podKey:      "worker-123",
		agentStatus: "waiting",
	}

	reporter := &MockEventReporter{}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  protoConfig,
		PodCtrl:   workerCtrl,
		Reporter:     reporter,
		MCPPort:      19000,
	})

	err = rp.Start()
	require.NoError(t, err)

	// Wait for decision to complete
	time.Sleep(500 * time.Millisecond)

	status := rp.GetStatus()
	// Should remain running after CONTINUE
	assert.Equal(t, PhaseRunning, status.Phase)
	assert.Equal(t, "CONTINUE", status.LastDecision)

	rp.Stop()
}

func TestAutopilotController_OnPodWaiting_IncrementAfterMaxReached(t *testing.T) {
	protoConfig := &runnerv1.AutopilotConfig{
		InitialPrompt: "Test",
		MaxIterations: 1,
	}

	workerCtrl := &MockPodController{
		workDir:     "/workspace",
		podKey:      "worker-123",
		agentStatus: "executing",
	}

	reporter := &MockEventReporter{}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  protoConfig,
		PodCtrl:   workerCtrl,
		Reporter:     reporter,
	})
	_ = rp.Start()

	// First call - should increment to 1
	rp.OnPodWaiting()
	assert.Equal(t, 1, rp.GetStatus().CurrentIteration)

	// Wait for trigger dedup
	time.Sleep(6 * time.Second)

	// Second call - should hit max iterations
	rp.OnPodWaiting()

	status := rp.GetStatus()
	assert.Equal(t, PhaseMaxIterations, status.Phase)
}

func TestAutopilotController_RunSingleDecision_ControlFailureRetry(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test that requires shell execution in CI environment")
	}
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "autopilot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create mock agent that fails
	scriptPath := filepath.Join(tmpDir, "mock_agent")
	script := `#!/bin/bash
exit 1
`
	err = os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)

	protoConfig := &runnerv1.AutopilotConfig{
		InitialPrompt:           "Test",
		MaxIterations:           10,
		ControlAgentType:        scriptPath,
		IterationTimeoutSeconds: 5,
	}

	// Worker returns waiting status to trigger retry
	workerCtrl := &MockPodController{
		workDir:     tmpDir,
		podKey:      "worker-123",
		agentStatus: "waiting",
	}

	reporter := &MockEventReporter{}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  protoConfig,
		PodCtrl:   workerCtrl,
		Reporter:     reporter,
		MCPPort:      19000,
	})

	err = rp.Start()
	require.NoError(t, err)

	// Wait for retry attempt
	time.Sleep(3 * time.Second)

	// Should have error events
	hasError := false
	for _, e := range reporter.GetIterationEvents() {
		if e.Phase == "error" {
			hasError = true
			break
		}
	}
	assert.True(t, hasError)

	rp.Stop()
}

func TestAutopilotController_RunSingleDecision_WorkerNotWaitingAfterFailure(t *testing.T) {
	if os.Getenv("CI") != "" {
		t.Skip("Skipping test that requires shell execution in CI environment")
	}
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "autopilot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create mock agent that fails
	scriptPath := filepath.Join(tmpDir, "mock_agent")
	script := `#!/bin/bash
exit 1
`
	err = os.WriteFile(scriptPath, []byte(script), 0755)
	require.NoError(t, err)

	protoConfig := &runnerv1.AutopilotConfig{
		InitialPrompt:           "Test",
		MaxIterations:           10,
		ControlAgentType:        scriptPath,
		IterationTimeoutSeconds: 5,
	}

	// Worker returns executing status - should NOT retry
	workerCtrl := &MockPodController{
		workDir:     tmpDir,
		podKey:      "worker-123",
		agentStatus: "executing",
	}

	reporter := &MockEventReporter{}

	rp := NewAutopilotController(Config{
		AutopilotKey:  "autopilot-123",
		PodKey: "worker-123",
		ProtoConfig:  protoConfig,
		PodCtrl:   workerCtrl,
		Reporter:     reporter,
		MCPPort:      19000,
	})

	// Manually trigger OnPodWaiting
	rp.OnPodWaiting()

	// Wait for failure
	time.Sleep(500 * time.Millisecond)

	// Should only have 1 iteration attempt (no retry because worker is executing)
	assert.Equal(t, 1, rp.GetStatus().CurrentIteration)

	rp.Stop()
}
