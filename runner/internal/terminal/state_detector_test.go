package terminal

import (
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewTerminalStateDetector(t *testing.T) {
	vt := NewVirtualTerminal(80, 24, 100)
	sd := NewTerminalStateDetector(vt)

	assert.NotNil(t, sd)
	assert.Equal(t, StateNotRunning, sd.GetState())
	assert.Equal(t, 3*time.Second, sd.idleThreshold)
	assert.Equal(t, 2*time.Second, sd.minStableTime)
}

func TestTerminalStateDetector_Options(t *testing.T) {
	vt := NewVirtualTerminal(80, 24, 100)

	cb := func(newState, prevState AgentState) {
		// Callback for test
	}

	customPatterns := []*regexp.Regexp{
		regexp.MustCompile(`^custom>\s*$`),
	}

	sd := NewTerminalStateDetector(vt,
		WithIdleThreshold(5*time.Second),
		WithMinStableTime(3*time.Second),
		WithStateChangeCallback(cb),
		WithPromptPatterns(customPatterns),
	)

	assert.Equal(t, 5*time.Second, sd.idleThreshold)
	assert.Equal(t, 3*time.Second, sd.minStableTime)
	assert.Len(t, sd.promptPatterns, 1)
}

func TestTerminalStateDetector_NotRunningWhenNotAltScreen(t *testing.T) {
	vt := NewVirtualTerminal(80, 24, 100)
	sd := NewTerminalStateDetector(vt)

	// Not in alt screen - should be not running
	state := sd.DetectState()
	assert.Equal(t, StateNotRunning, state)
}

func TestTerminalStateDetector_ExecutingWhenScreenChanges(t *testing.T) {
	vt := NewVirtualTerminal(80, 24, 100)
	sd := NewTerminalStateDetector(vt,
		WithIdleThreshold(100*time.Millisecond),
		WithMinStableTime(50*time.Millisecond),
	)

	// Simulate entering alt screen
	vt.Feed([]byte("\x1b[?1049h")) // DECSET 1049 - enter alt screen

	// Send some output
	vt.Feed([]byte("Processing..."))
	sd.OnOutput()

	state := sd.DetectState()
	assert.Equal(t, StateExecuting, state)
}

func TestTerminalStateDetector_WaitingWhenPromptDetected(t *testing.T) {
	vt := NewVirtualTerminal(80, 24, 100)
	sd := NewTerminalStateDetector(vt,
		WithIdleThreshold(10*time.Millisecond),
		WithMinStableTime(5*time.Millisecond),
	)

	// Simulate entering alt screen
	vt.Feed([]byte("\x1b[?1049h"))

	// Move cursor to bottom and show prompt
	vt.Feed([]byte("\x1b[24;1H>"))
	sd.OnOutput()

	// First detection - sets up initial hash
	sd.DetectState()

	// Wait for stable threshold
	time.Sleep(20 * time.Millisecond)

	// Second detection - hash unchanged, triggers stability check
	sd.DetectState()

	// Wait more for stable time to pass
	time.Sleep(20 * time.Millisecond)

	// Third detection - now stable long enough, should check for prompt
	state := sd.DetectState()
	assert.Equal(t, StateWaiting, state)
}

func TestTerminalStateDetector_StateChangeCallback(t *testing.T) {
	vt := NewVirtualTerminal(80, 24, 100)

	var mu sync.Mutex
	var states []AgentState

	sd := NewTerminalStateDetector(vt,
		WithIdleThreshold(10*time.Millisecond),
		WithMinStableTime(5*time.Millisecond),
		WithStateChangeCallback(func(newState, prevState AgentState) {
			mu.Lock()
			states = append(states, newState)
			mu.Unlock()
		}),
	)

	// Enter alt screen
	vt.Feed([]byte("\x1b[?1049h"))
	sd.OnOutput()
	sd.DetectState()

	// Wait for callback
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	assert.Contains(t, states, StateExecuting)
	mu.Unlock()
}

func TestTerminalStateDetector_PromptPatterns(t *testing.T) {
	testCases := []struct {
		name     string
		line     string
		expected bool
	}{
		{"simple >", ">", true},
		{"spaced >", "  >  ", true},
		{"y/n prompt", "Continue? (y/n)", true},
		{"Y/N prompt", "Proceed? [Y/N]", true},
		{"box input", "│ > │", true},
		{"permission", "Allow file access?", true},
		{"approve", "Approve this change?", true},
		{"random text", "Loading data...", false},
		{"empty", "", false},
	}

	vt := NewVirtualTerminal(80, 24, 100)
	sd := NewTerminalStateDetector(vt)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matched := false
			for _, pattern := range sd.promptPatterns {
				if pattern.MatchString(tc.line) {
					matched = true
					break
				}
			}
			assert.Equal(t, tc.expected, matched, "line: %q", tc.line)
		})
	}
}

func TestTerminalStateDetector_GetBottomLines(t *testing.T) {
	vt := NewVirtualTerminal(80, 24, 100)
	sd := NewTerminalStateDetector(vt)

	// Fill some lines
	for i := 0; i < 24; i++ {
		vt.Feed([]byte("\r\n"))
	}
	vt.Feed([]byte("Line 1\r\nLine 2\r\nLine 3"))

	lines := sd.GetBottomLines(3)
	require.NotNil(t, lines)
	// Check that we get some lines back (exact content depends on VT state)
	assert.LessOrEqual(t, len(lines), 3)
}

func TestTerminalStateDetector_Reset(t *testing.T) {
	vt := NewVirtualTerminal(80, 24, 100)
	sd := NewTerminalStateDetector(vt)

	// Enter alt screen and detect
	vt.Feed([]byte("\x1b[?1049h"))
	sd.OnOutput()
	sd.DetectState()

	assert.Equal(t, StateExecuting, sd.GetState())

	// Reset
	sd.Reset()

	assert.Equal(t, StateNotRunning, sd.GetState())
}

func TestTerminalStateDetector_ConcurrentAccess(t *testing.T) {
	vt := NewVirtualTerminal(80, 24, 100)
	sd := NewTerminalStateDetector(vt)

	// Enter alt screen
	vt.Feed([]byte("\x1b[?1049h"))

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			sd.OnOutput()
		}()
		go func() {
			defer wg.Done()
			sd.DetectState()
		}()
	}
	wg.Wait()

	// Should not panic and should have a valid state
	state := sd.GetState()
	assert.Contains(t, []AgentState{StateNotRunning, StateExecuting, StateWaiting}, state)
}
