// Package terminal provides terminal state detection for AI agents.
package terminal

import (
	"crypto/sha256"
	"regexp"
	"strings"
	"sync"
	"time"
)

// AgentState represents the detected state of an AI agent running in the terminal.
type AgentState string

const (
	// StateNotRunning indicates the agent is not running (not in alt screen).
	StateNotRunning AgentState = "not_running"
	// StateExecuting indicates the agent is actively executing (screen changing).
	StateExecuting AgentState = "executing"
	// StateWaiting indicates the agent is waiting for user input.
	StateWaiting AgentState = "waiting"
)

// StateChangeCallback is called when the agent state changes.
type StateChangeCallback func(newState AgentState, previousState AgentState)

// TerminalStateDetector detects the state of an AI agent (like Claude Code)
// by analyzing terminal output patterns. This provides more reliable detection
// than process-based monitoring since Claude Code spends most time in text
// processing without creating child processes.
//
// Detection Strategy:
//  1. Check if terminal is in alt screen mode (Claude Code TUI)
//  2. Compute hash of visible screen content
//  3. If screen content is stable for idleThreshold duration, check for input prompt
//  4. Input prompt detection indicates "waiting" state
type TerminalStateDetector struct {
	vt *VirtualTerminal

	mu              sync.RWMutex
	lastOutputTime  time.Time
	lastScreenHash  [32]byte
	idleThreshold   time.Duration
	currentState    AgentState
	stableStartTime time.Time // When screen became stable

	// Callback for state changes
	callback StateChangeCallback

	// Input prompt patterns for different AI agents
	promptPatterns []*regexp.Regexp

	// Minimum stable time before considering as "waiting"
	minStableTime time.Duration
}

// StateDetectorOption is a functional option for configuring TerminalStateDetector.
type StateDetectorOption func(*TerminalStateDetector)

// WithIdleThreshold sets the idle threshold duration.
// Default is 3 seconds.
func WithIdleThreshold(d time.Duration) StateDetectorOption {
	return func(sd *TerminalStateDetector) {
		sd.idleThreshold = d
	}
}

// WithMinStableTime sets the minimum stable time before considering as "waiting".
// Default is 2 seconds.
func WithMinStableTime(d time.Duration) StateDetectorOption {
	return func(sd *TerminalStateDetector) {
		sd.minStableTime = d
	}
}

// WithStateChangeCallback sets the callback for state changes.
func WithStateChangeCallback(cb StateChangeCallback) StateDetectorOption {
	return func(sd *TerminalStateDetector) {
		sd.callback = cb
	}
}

// WithPromptPatterns sets custom input prompt patterns.
// Default patterns detect Claude Code's input prompt.
func WithPromptPatterns(patterns []*regexp.Regexp) StateDetectorOption {
	return func(sd *TerminalStateDetector) {
		sd.promptPatterns = patterns
	}
}

// NewTerminalStateDetector creates a new terminal state detector.
func NewTerminalStateDetector(vt *VirtualTerminal, opts ...StateDetectorOption) *TerminalStateDetector {
	sd := &TerminalStateDetector{
		vt:            vt,
		idleThreshold: 3 * time.Second,
		minStableTime: 2 * time.Second,
		currentState:  StateNotRunning,
		// Default prompt patterns for Claude Code
		// Claude Code shows ">" at the bottom when waiting for input
		// Also check for common input prompts
		promptPatterns: []*regexp.Regexp{
			regexp.MustCompile(`^\s*>\s*$`),                          // Simple ">" prompt
			regexp.MustCompile(`^\s*>\s+$`),                          // "> " with trailing space
			regexp.MustCompile(`(?i)^.*\(y/n\)\s*$`),                 // Yes/No prompt
			regexp.MustCompile(`(?i)^.*\[y/n\]\s*$`),                 // [Y/N] prompt
			regexp.MustCompile(`(?i)press.*to continue`),             // Press key prompt
			regexp.MustCompile(`(?i)waiting for.*input`),             // Waiting for input
			regexp.MustCompile(`(?i)enter.*to proceed`),              // Enter to proceed
			regexp.MustCompile(`^\s*│\s*>\s*│\s*$`),                  // Claude Code box input: │ > │
			regexp.MustCompile(`^\s*│\s*>\s+│\s*$`),                  // Claude Code box input with space
			regexp.MustCompile(`^\s*\S+\s*>\s*$`),                    // "user >" style prompt
			regexp.MustCompile(`^>$`),                                // Bare ">" character
			regexp.MustCompile(`^\s*╰[─━]+>\s*$`),                    // Box drawing prompt ending
			regexp.MustCompile(`(?i)^.*permission.*\?\s*$`),          // Permission request
			regexp.MustCompile(`(?i)^.*approve.*\?\s*$`),             // Approval request
			regexp.MustCompile(`(?i)^.*allow.*\?\s*$`),               // Allow request
		},
	}

	for _, opt := range opts {
		opt(sd)
	}

	return sd
}

// OnOutput should be called whenever the terminal receives new output.
// This updates the internal state tracking.
func (sd *TerminalStateDetector) OnOutput() {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	now := time.Now()
	sd.lastOutputTime = now

	// If we were stable, reset stability tracking
	if !sd.stableStartTime.IsZero() {
		sd.stableStartTime = time.Time{}
	}

	// If in alt screen, we're definitely running
	if sd.vt.IsAltScreen() && sd.currentState == StateNotRunning {
		sd.setState(StateExecuting)
	}
}

// DetectState analyzes the current terminal state and returns the detected agent state.
// This method should be called periodically (e.g., every 500ms) to check for state changes.
func (sd *TerminalStateDetector) DetectState() AgentState {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	// 1. Check if in alt screen (TUI mode)
	if !sd.vt.IsAltScreen() {
		if sd.currentState != StateNotRunning {
			sd.setState(StateNotRunning)
		}
		return StateNotRunning
	}

	// 2. Compute current screen hash
	currentHash := sd.computeScreenHash()

	// 3. Check if screen content has changed
	if currentHash != sd.lastScreenHash {
		// Screen changed - agent is executing
		sd.lastScreenHash = currentHash
		sd.lastOutputTime = time.Now()
		sd.stableStartTime = time.Time{} // Reset stability
		if sd.currentState != StateExecuting {
			sd.setState(StateExecuting)
		}
		return StateExecuting
	}

	// 4. Screen is stable - check if it's been stable long enough
	now := time.Now()
	if sd.stableStartTime.IsZero() {
		sd.stableStartTime = now
	}

	stableDuration := now.Sub(sd.stableStartTime)
	idleDuration := now.Sub(sd.lastOutputTime)

	// Not stable long enough yet
	if stableDuration < sd.minStableTime || idleDuration < sd.idleThreshold {
		return sd.currentState
	}

	// 5. Screen has been stable - check for input prompt
	if sd.hasInputPrompt() {
		if sd.currentState != StateWaiting {
			sd.setState(StateWaiting)
		}
		return StateWaiting
	}

	// Stable but no prompt detected - could be processing or loading
	// Keep current state (don't flip-flop)
	return sd.currentState
}

// GetState returns the current detected state without performing detection.
func (sd *TerminalStateDetector) GetState() AgentState {
	sd.mu.RLock()
	defer sd.mu.RUnlock()
	return sd.currentState
}

// SetCallback sets the state change callback.
func (sd *TerminalStateDetector) SetCallback(cb StateChangeCallback) {
	sd.mu.Lock()
	defer sd.mu.Unlock()
	sd.callback = cb
}

// computeScreenHash computes a hash of the visible screen content.
// This is used to detect when the screen has changed.
func (sd *TerminalStateDetector) computeScreenHash() [32]byte {
	display := sd.vt.GetDisplay()
	return sha256.Sum256([]byte(display))
}

// hasInputPrompt checks if the terminal is showing an input prompt.
// It looks at the bottom few lines of the terminal for prompt patterns.
func (sd *TerminalStateDetector) hasInputPrompt() bool {
	snapshot := sd.vt.GetSnapshot()
	if snapshot == nil {
		return false
	}

	// Check the bottom 5 lines for input prompts
	linesToCheck := 5
	startLine := len(snapshot.Lines) - linesToCheck
	if startLine < 0 {
		startLine = 0
	}

	for i := startLine; i < len(snapshot.Lines); i++ {
		line := strings.TrimRight(snapshot.Lines[i], " \t")
		if line == "" {
			continue
		}

		for _, pattern := range sd.promptPatterns {
			if pattern.MatchString(line) {
				return true
			}
		}
	}

	return false
}

// setState updates the current state and triggers the callback if state changed.
func (sd *TerminalStateDetector) setState(newState AgentState) {
	if sd.currentState == newState {
		return
	}

	previousState := sd.currentState
	sd.currentState = newState

	if sd.callback != nil {
		// Call callback in a goroutine to avoid blocking
		cb := sd.callback
		go cb(newState, previousState)
	}
}

// GetBottomLines returns the bottom N lines of the terminal.
// Useful for external prompt pattern matching.
func (sd *TerminalStateDetector) GetBottomLines(n int) []string {
	snapshot := sd.vt.GetSnapshot()
	if snapshot == nil {
		return nil
	}

	startLine := len(snapshot.Lines) - n
	if startLine < 0 {
		startLine = 0
	}

	result := make([]string, 0, n)
	for i := startLine; i < len(snapshot.Lines); i++ {
		result = append(result, snapshot.Lines[i])
	}
	return result
}

// GetTimeSinceLastOutput returns the duration since the last output was received.
func (sd *TerminalStateDetector) GetTimeSinceLastOutput() time.Duration {
	sd.mu.RLock()
	defer sd.mu.RUnlock()

	if sd.lastOutputTime.IsZero() {
		return 0
	}
	return time.Since(sd.lastOutputTime)
}

// Reset resets the detector state.
func (sd *TerminalStateDetector) Reset() {
	sd.mu.Lock()
	defer sd.mu.Unlock()

	sd.lastOutputTime = time.Time{}
	sd.lastScreenHash = [32]byte{}
	sd.currentState = StateNotRunning
	sd.stableStartTime = time.Time{}
}
