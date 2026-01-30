// Package terminal provides terminal state detection for AI agents.
package terminal

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"
	"sync"
	"time"
)

// Note: AgentState, StateNotRunning, StateExecuting, StateWaiting, and StateChangeCallback
// are defined in state_detector.go to avoid redeclaration

// MultiSignalDetector detects agent state by fusing multiple signals.
// This approach is Agent-agnostic and doesn't depend on specific implementations.
//
// Signals and their weights:
//   - Output Activity (0.4): Most reliable - based on terminal output patterns
//   - Screen Stability (0.3): Terminal content hasn't changed
//   - Prompt Detection (0.3): Generic prompt patterns detected
//   - OSC Hints (optional): Boost confidence if available, but don't depend on it
//
// State Machine:
//
//	                 output received
//	    ┌──────────────────────────────────────┐
//	    │                                      │
//	    ▼                                      │
//	┌────────┐    confidence > threshold  ┌─────────┐
//	│Executing│ ───────────────────────►  │ Waiting │
//	└────────┘                            └─────────┘
//	    ▲                                      │
//	    │        output received               │
//	    └──────────────────────────────────────┘
type MultiSignalDetector struct {
	mu sync.RWMutex

	// Sub-detectors
	activityDetector *OutputActivityDetector
	promptDetector   *PromptDetector

	// Screen stability tracking
	lastScreenHash   string
	lastScreenTime   time.Time
	screenStableTime time.Duration

	// OSC title tracking (optional signal)
	lastOSCTitle     string
	lastOSCTitleTime time.Time

	// Configuration
	config MultiSignalConfig

	// Current state
	currentState    AgentState
	stateChangeTime time.Time
	lastCheckTime   time.Time

	// Callback
	onStateChange StateChangeCallback

	// Screen content for prompt detection
	screenLines []string
}

// MultiSignalConfig contains configuration for MultiSignalDetector.
type MultiSignalConfig struct {
	// ActivityWeight is the weight for output activity signal (default: 0.4)
	ActivityWeight float64
	// StabilityWeight is the weight for screen stability signal (default: 0.3)
	StabilityWeight float64
	// PromptWeight is the weight for prompt detection signal (default: 0.3)
	PromptWeight float64

	// MinStableTime is the minimum time screen must be stable (default: 500ms)
	MinStableTime time.Duration
	// WaitingThreshold is the confidence threshold to transition to waiting (default: 0.6)
	WaitingThreshold float64

	// IdleThreshold for activity detector (default: 500ms)
	IdleThreshold time.Duration
	// ConfirmThreshold for activity detector (default: 500ms)
	ConfirmThreshold time.Duration

	// MaxPromptLength for prompt detector (default: 100)
	MaxPromptLength int

	// OnStateChange callback
	OnStateChange StateChangeCallback
}

// NewMultiSignalDetector creates a new multi-signal detector.
func NewMultiSignalDetector(cfg MultiSignalConfig) *MultiSignalDetector {
	// Apply defaults
	if cfg.ActivityWeight == 0 {
		cfg.ActivityWeight = 0.4
	}
	if cfg.StabilityWeight == 0 {
		cfg.StabilityWeight = 0.3
	}
	if cfg.PromptWeight == 0 {
		cfg.PromptWeight = 0.3
	}
	if cfg.MinStableTime == 0 {
		cfg.MinStableTime = 500 * time.Millisecond
	}
	if cfg.WaitingThreshold == 0 {
		cfg.WaitingThreshold = 0.6
	}
	if cfg.IdleThreshold == 0 {
		cfg.IdleThreshold = 500 * time.Millisecond
	}
	if cfg.ConfirmThreshold == 0 {
		cfg.ConfirmThreshold = 500 * time.Millisecond
	}
	if cfg.MaxPromptLength == 0 {
		cfg.MaxPromptLength = 100
	}

	// Create sub-detectors
	activityDetector := NewOutputActivityDetector(OutputActivityConfig{
		IdleThreshold:    cfg.IdleThreshold,
		ConfirmThreshold: cfg.ConfirmThreshold,
	})

	promptDetector := NewPromptDetector(PromptDetectorConfig{
		MaxPromptLength: cfg.MaxPromptLength,
	})

	return &MultiSignalDetector{
		activityDetector: activityDetector,
		promptDetector:   promptDetector,
		config:           cfg,
		currentState:     StateNotRunning,
		onStateChange:    cfg.OnStateChange,
	}
}

// OnOutput should be called whenever terminal output is received.
// This is the primary input signal.
func (d *MultiSignalDetector) OnOutput(bytes int) {
	d.mu.Lock()
	currentState := d.currentState

	// Forward to activity detector
	d.activityDetector.OnOutput(bytes)

	// If we were in NotRunning or Waiting, transition to Executing
	if d.currentState != StateExecuting {
		d.setState(StateExecuting)
	}
	d.mu.Unlock()

	// Debug logging OUTSIDE lock to avoid blocking PTY output
	slog.Debug("MultiSignalDetector OnOutput called",
		"module", "terminal",
		"bytes", bytes,
		"current_state", currentState)
}

// OnScreenUpdate should be called when the terminal screen content changes.
// Provide the current screen lines for analysis.
func (d *MultiSignalDetector) OnScreenUpdate(lines []string) {
	// Compute screen hash OUTSIDE lock to minimize lock contention
	// This is safe because computeScreenHash only reads the lines slice
	hash := d.computeScreenHash(lines)
	now := time.Now()

	// Minimal lock scope - only update state
	d.mu.Lock()
	hashChanged := hash != d.lastScreenHash
	if hashChanged {
		// Screen changed
		d.lastScreenHash = hash
		d.lastScreenTime = now
		d.screenStableTime = 0
	} else {
		// Screen stable
		d.screenStableTime = now.Sub(d.lastScreenTime)
	}
	// Store lines for prompt detection
	d.screenLines = lines
	d.mu.Unlock()

	// Debug logging OUTSIDE lock to avoid blocking PTY output
	if hashChanged && len(lines) > 0 {
		// Find non-empty lines for debugging
		var nonEmptyLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if len(trimmed) > 0 {
				// Truncate long lines
				if len(trimmed) > 60 {
					trimmed = trimmed[:60] + "..."
				}
				nonEmptyLines = append(nonEmptyLines, trimmed)
			}
		}
		// Only log last few non-empty lines
		if len(nonEmptyLines) > 5 {
			nonEmptyLines = nonEmptyLines[len(nonEmptyLines)-5:]
		}
		slog.Debug("MultiSignalDetector screen update",
			"module", "terminal",
			"hash_changed", hashChanged,
			"hash", hash[:8],
			"non_empty_count", len(nonEmptyLines),
			"last_non_empty_lines", nonEmptyLines)
	}
}

// OnOSCTitle should be called when an OSC title update is received.
// This is an optional signal that can boost confidence.
func (d *MultiSignalDetector) OnOSCTitle(title string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.lastOSCTitle = title
	d.lastOSCTitleTime = time.Now()
}

// DetectState analyzes all signals and returns the current agent state.
// This should be called periodically (e.g., every 200-300ms).
func (d *MultiSignalDetector) DetectState() AgentState {
	d.mu.Lock()

	d.lastCheckTime = time.Now()

	// Update activity detector state
	activityState := d.activityDetector.CheckState()

	// Calculate confidence for "waiting" state (captures values for logging)
	confidence, logData := d.calculateWaitingConfidenceLocked(activityState)

	// State transition logic
	switch d.currentState {
	case StateNotRunning:
		// Stay in NotRunning until output is received (handled in OnOutput)

	case StateExecuting:
		// Check if we should transition to Waiting
		if confidence >= d.config.WaitingThreshold {
			d.setState(StateWaiting)
		}

	case StateWaiting:
		// Check if we should transition back to Executing
		// This happens in OnOutput when new output is received
		// But we can also check if confidence dropped significantly
		if confidence < d.config.WaitingThreshold*0.5 {
			d.setState(StateExecuting)
		}
	}

	currentState := d.currentState
	d.mu.Unlock()

	// Debug logging OUTSIDE lock to avoid blocking PTY output
	slog.Debug("MultiSignalDetector confidence calculation",
		"module", "terminal",
		"activity_state", logData.activityState,
		"activity_contrib", logData.activityContrib,
		"screen_stable_time", logData.screenStableTime,
		"stability_contrib", logData.stabilityContrib,
		"prompt_detected", logData.promptDetected,
		"prompt_type", logData.promptType,
		"prompt_contrib", logData.promptContrib,
		"total_confidence", logData.confidence,
		"threshold", logData.threshold,
		"screen_lines_count", logData.screenLinesCount)

	return currentState
}

// confidenceLogData holds data for logging confidence calculation results.
// This allows logging to happen outside the lock.
type confidenceLogData struct {
	activityState    ActivityState
	activityContrib  float64
	screenStableTime time.Duration
	stabilityContrib float64
	promptDetected   bool
	promptType       PromptType
	promptContrib    float64
	confidence       float64
	threshold        float64
	screenLinesCount int
}

// calculateWaitingConfidenceLocked calculates the confidence that the agent is waiting.
// Must be called with d.mu held. Returns confidence and log data for deferred logging.
func (d *MultiSignalDetector) calculateWaitingConfidenceLocked(activityState ActivityState) (float64, confidenceLogData) {
	var confidence float64
	var activityContrib, stabilityContrib, promptContrib float64

	// Signal 1: Output Activity (weight: 0.4)
	// If output has stopped, this contributes to waiting confidence
	switch activityState {
	case ActivityStateIdle:
		activityContrib = d.config.ActivityWeight * 1.0
	case ActivityStatePotentialIdle:
		activityContrib = d.config.ActivityWeight * 0.7
	case ActivityStateActive:
		activityContrib = d.config.ActivityWeight * 0.0
	}
	confidence += activityContrib

	// Signal 2: Screen Stability (weight: 0.3)
	// If screen hasn't changed for a while, this contributes to waiting confidence
	if d.screenStableTime >= d.config.MinStableTime {
		// Scale based on how long it's been stable
		stableRatio := float64(d.screenStableTime) / float64(d.config.MinStableTime*2)
		if stableRatio > 1.0 {
			stableRatio = 1.0
		}
		stabilityContrib = d.config.StabilityWeight * stableRatio
		confidence += stabilityContrib
	}

	// Signal 3: Prompt Detection (weight: 0.3)
	// If a prompt is detected, this contributes to waiting confidence
	var promptResult PromptResult
	if len(d.screenLines) > 0 {
		promptResult = d.promptDetector.DetectPrompt(d.screenLines)
		if promptResult.IsPrompt {
			promptContrib = d.config.PromptWeight * promptResult.Confidence
			confidence += promptContrib
		}
	}

	// Optional: OSC Title Boost
	// If OSC title suggests waiting (e.g., contains waiting indicators), add a small boost
	if d.lastOSCTitle != "" && time.Since(d.lastOSCTitleTime) < 5*time.Second {
		if d.oscSuggestsWaiting(d.lastOSCTitle) {
			confidence += 0.1 // Small boost, don't depend on it
		}
	}

	// Cap at 1.0
	if confidence > 1.0 {
		confidence = 1.0
	}

	// Capture log data for deferred logging outside lock
	logData := confidenceLogData{
		activityState:    activityState,
		activityContrib:  activityContrib,
		screenStableTime: d.screenStableTime,
		stabilityContrib: stabilityContrib,
		promptDetected:   promptResult.IsPrompt,
		promptType:       promptResult.PromptType,
		promptContrib:    promptContrib,
		confidence:       confidence,
		threshold:        d.config.WaitingThreshold,
		screenLinesCount: len(d.screenLines),
	}

	return confidence, logData
}

// oscSuggestsWaiting checks if the OSC title suggests the agent is waiting.
// This is a heuristic and not meant to be relied upon.
func (d *MultiSignalDetector) oscSuggestsWaiting(title string) bool {
	// Look for common waiting indicators
	// These are generic patterns, not specific to any agent
	waitingPatterns := []string{
		"waiting",
		"input",
		"prompt",
		"✳", // Common "idle/ready" indicator
		"⏳", // Waiting indicator
	}

	for _, pattern := range waitingPatterns {
		if containsIgnoreCase(title, pattern) {
			return true
		}
	}

	return false
}

// GetState returns the current state without performing detection.
func (d *MultiSignalDetector) GetState() AgentState {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.currentState
}

// SetCallback sets the state change callback.
func (d *MultiSignalDetector) SetCallback(cb StateChangeCallback) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onStateChange = cb
}

// Reset resets the detector to initial state.
func (d *MultiSignalDetector) Reset() {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.activityDetector.Reset()
	d.lastScreenHash = ""
	d.lastScreenTime = time.Time{}
	d.screenStableTime = 0
	d.lastOSCTitle = ""
	d.lastOSCTitleTime = time.Time{}
	d.currentState = StateNotRunning
	d.stateChangeTime = time.Time{}
	d.screenLines = nil
}

// SetProcessRunning should be called when the agent process starts/stops.
func (d *MultiSignalDetector) SetProcessRunning(running bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !running {
		d.setState(StateNotRunning)
	} else if d.currentState == StateNotRunning {
		// Process started, wait for output to transition to Executing
		// Don't transition immediately - wait for actual output
	}
}

// GetActivityDetector returns the underlying activity detector for direct access.
func (d *MultiSignalDetector) GetActivityDetector() *OutputActivityDetector {
	return d.activityDetector
}

// GetPromptDetector returns the underlying prompt detector for direct access.
func (d *MultiSignalDetector) GetPromptDetector() *PromptDetector {
	return d.promptDetector
}

// GetLastPromptResult returns the last prompt detection result.
func (d *MultiSignalDetector) GetLastPromptResult() PromptResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if len(d.screenLines) == 0 {
		return PromptResult{}
	}
	return d.promptDetector.DetectPrompt(d.screenLines)
}

// GetScreenStableTime returns how long the screen has been stable.
func (d *MultiSignalDetector) GetScreenStableTime() time.Duration {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.screenStableTime
}

// GetIdleDuration returns how long since the last output.
func (d *MultiSignalDetector) GetIdleDuration() time.Duration {
	return d.activityDetector.IdleDuration()
}

// setState updates the current state and triggers callback.
func (d *MultiSignalDetector) setState(newState AgentState) {
	if d.currentState == newState {
		return
	}

	prevState := d.currentState
	d.currentState = newState
	d.stateChangeTime = time.Now()

	if d.onStateChange != nil {
		cb := d.onStateChange
		go cb(newState, prevState)
	}
}

// computeScreenHash computes a hash of the screen content.
// It normalizes spinner/animation characters to produce stable hashes when
// only the animation frame changes (e.g., Claude Code's "Tinkering..." spinner).
func (d *MultiSignalDetector) computeScreenHash(lines []string) string {
	h := sha256.New()
	for _, line := range lines {
		// Normalize the line by replacing common spinner characters with a placeholder
		normalized := normalizeSpinnerChars(line)
		h.Write([]byte(normalized))
		h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

// normalizeSpinnerChars replaces common spinner/animation characters with a placeholder.
// This helps maintain screen stability detection even when animations are running.
func normalizeSpinnerChars(line string) string {
	// Common spinner characters used by CLI tools
	spinnerChars := []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏', // dots
		'⣾', '⣽', '⣻', '⢿', '⡿', '⣟', '⣯', '⣷', // braille
		'◐', '◓', '◑', '◒', // circle quarters
		'◴', '◷', '◶', '◵', // other circles
		'◰', '◳', '◲', '◱', // squares
		'▖', '▘', '▝', '▗', // corners
		'⠐', '⠂', '⠈', '⠁', '⠠', '⠄', '⠤', '⠤', // braille small
		'*', '·', '•', '●', '○', '◎', '◉', // bullets
		'✻', '✽', '✼', '✾', '✿', '❀', // flowers
		'⏸', '⏵', '⏴', '▶', '◀', '⏹', '⏺', // media controls
	}

	// Create a map for fast lookup
	spinnerSet := make(map[rune]bool)
	for _, c := range spinnerChars {
		spinnerSet[c] = true
	}

	// Replace spinner chars with a placeholder
	result := make([]rune, 0, len(line))
	for _, r := range line {
		if spinnerSet[r] {
			result = append(result, '·') // normalize to single placeholder
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// containsIgnoreCase checks if s contains substr (case-insensitive).
func containsIgnoreCase(s, substr string) bool {
	// Simple lowercase comparison
	sLower := toLower(s)
	substrLower := toLower(substr)

	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

// toLower converts a string to lowercase (ASCII only for performance).
func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}
