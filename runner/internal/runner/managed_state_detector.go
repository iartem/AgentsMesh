package runner

import (
	"context"
	"time"

	"github.com/anthropics/agentsmesh/runner/internal/autopilot"
	"github.com/anthropics/agentsmesh/runner/internal/logger"
	"github.com/anthropics/agentsmesh/runner/internal/terminal/detector"
	"github.com/anthropics/agentsmesh/runner/internal/terminal/vt"
)

// ManagedStateDetector wraps detector.MultiSignalDetector and adds:
// - Background detection loop for timeout-based state transitions
// - Lifecycle management (Start/Stop)
// - Configuration of detection parameters
//
// This implements autopilot.StateDetector interface.
type ManagedStateDetector struct {
	detector *detector.MultiSignalDetector
	vt       *vt.VirtualTerminal
	ctx      context.Context
	cancel   context.CancelFunc
}

// Compile-time interface check
var _ autopilot.StateDetector = (*ManagedStateDetector)(nil)

// NewManagedStateDetector creates a new managed state detector.
// It starts a background goroutine to periodically run detection for timeout-based transitions.
func NewManagedStateDetector(vt *vt.VirtualTerminal) *ManagedStateDetector {
	detector := detector.NewMultiSignalDetector(detector.MultiSignalConfig{
		// Responsive detection thresholds
		IdleThreshold:    500 * time.Millisecond,
		ConfirmThreshold: 500 * time.Millisecond,
		MinStableTime:    300 * time.Millisecond,
		WaitingThreshold: 0.6,
	})

	ctx, cancel := context.WithCancel(context.Background())
	m := &ManagedStateDetector{
		detector: detector,
		vt:       vt,
		ctx:      ctx,
		cancel:   cancel,
	}

	// Start background detection loop
	go m.runDetectionLoop()

	return m
}

// runDetectionLoop periodically runs state detection.
// Screen content is pushed via OnScreenUpdate from OutputHandler (single-direction data flow).
// This loop only triggers periodic DetectState() for timeout-based transitions.
func (m *ManagedStateDetector) runDetectionLoop() {
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.detector.DetectState()
		}
	}
}

// OnScreenUpdate should be called after VirtualTerminal.Feed with the current screen lines.
// This implements single-direction data flow: PTY → VT.Feed → OnScreenUpdate → StateDetector
func (m *ManagedStateDetector) OnScreenUpdate(lines []string) {
	m.detector.OnScreenUpdate(lines)
}

// OnOutput should be called when terminal output is received.
func (m *ManagedStateDetector) OnOutput(bytes int) {
	logger.TerminalTrace().Trace("ManagedStateDetector.OnOutput", "bytes", bytes)
	m.detector.OnOutput(bytes)
}

// OnOSCTitle should be called when an OSC title update is received.
func (m *ManagedStateDetector) OnOSCTitle(title string) {
	m.detector.OnOSCTitle(title)
}

// DetectState analyzes and returns the current agent state.
func (m *ManagedStateDetector) DetectState() detector.AgentState {
	return m.detector.DetectState()
}

// GetState returns the current state without performing detection.
func (m *ManagedStateDetector) GetState() detector.AgentState {
	return m.detector.GetState()
}

// SetCallback sets the state change callback.
func (m *ManagedStateDetector) SetCallback(cb detector.StateChangeCallback) {
	m.detector.SetCallback(cb)
}

// Reset resets the detector state.
func (m *ManagedStateDetector) Reset() {
	m.detector.Reset()
}

// Stop stops the background detection loop.
func (m *ManagedStateDetector) Stop() {
	m.cancel()
}
