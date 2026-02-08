package runner

import (
	"github.com/anthropics/agentsmesh/runner/internal/autopilot"
)

// PodControllerImpl implements autopilot.TargetPodController interface.
// It provides the AutopilotController with the ability to interact with the target Pod.
type PodControllerImpl struct {
	pod    *Pod
	runner *Runner
}

// NewPodController creates a new PodController.
func NewPodController(pod *Pod, runner *Runner) *PodControllerImpl {
	return &PodControllerImpl{
		pod:    pod,
		runner: runner,
	}
}

// SendTerminalText sends text to the pod's terminal.
func (c *PodControllerImpl) SendTerminalText(text string) error {
	if c.pod.Terminal == nil {
		return nil
	}
	// Add newline to send the command
	return c.pod.Terminal.Write([]byte(text + "\n"))
}

// GetWorkDir returns the pod's working directory.
func (c *PodControllerImpl) GetWorkDir() string {
	return c.pod.SandboxPath
}

// GetPodKey returns the pod's key.
func (c *PodControllerImpl) GetPodKey() string {
	return c.pod.PodKey
}

// GetAgentStatus returns the pod's agent status.
func (c *PodControllerImpl) GetAgentStatus() string {
	agentStatus, _, _, _ := c.runner.GetPodStatus(c.pod.PodKey)
	return agentStatus
}

// GetStateDetector returns the ManagedStateDetector for the pod.
// Returns nil if the virtual terminal is not available.
// Returns the same instance across multiple calls to ensure state continuity.
func (c *PodControllerImpl) GetStateDetector() autopilot.StateDetector {
	if c.pod.VirtualTerminal == nil {
		return nil
	}
	detector := c.pod.GetOrCreateStateDetector()
	if detector == nil {
		return nil
	}
	return detector
}

// Compile-time interface check
var _ autopilot.TargetPodController = (*PodControllerImpl)(nil)
