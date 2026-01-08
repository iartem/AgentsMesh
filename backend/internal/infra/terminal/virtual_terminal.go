package terminal

import (
	"bytes"
	"regexp"
	"strings"
	"sync"
)

// VirtualTerminal provides a virtual terminal emulator
// that converts raw PTY output with ANSI escape sequences
// into clean text for agent observation.
//
// Note: This is a simplified implementation. For full ANSI support,
// consider using a proper terminal emulator library.
type VirtualTerminal struct {
	mu sync.RWMutex

	cols int
	rows int

	// Screen buffer (current visible content)
	screen [][]rune

	// Cursor position
	cursorX int
	cursorY int

	// History buffer (scrolled-off lines)
	history     []string
	maxHistory  int

	// Flag to track if we've received any data
	hasData bool
}

// ANSI escape sequence pattern
var ansiPattern = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\][^\x07]*\x07|\x1b[PX^_][^\x1b]*\x1b\\`)

// NewVirtualTerminal creates a new virtual terminal
func NewVirtualTerminal(cols, rows, maxHistory int) *VirtualTerminal {
	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}
	if maxHistory <= 0 {
		maxHistory = 10000
	}

	vt := &VirtualTerminal{
		cols:       cols,
		rows:       rows,
		maxHistory: maxHistory,
		history:    make([]string, 0),
	}
	vt.initScreen()
	return vt
}

// initScreen initializes/resets the screen buffer
func (vt *VirtualTerminal) initScreen() {
	vt.screen = make([][]rune, vt.rows)
	for i := range vt.screen {
		vt.screen[i] = make([]rune, vt.cols)
		for j := range vt.screen[i] {
			vt.screen[i][j] = ' '
		}
	}
	vt.cursorX = 0
	vt.cursorY = 0
}

// Feed processes raw PTY data
func (vt *VirtualTerminal) Feed(data []byte) {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	vt.hasData = true

	// Decode bytes to string
	text := string(data)

	// Process character by character
	for _, ch := range text {
		vt.processChar(ch)
	}
}

// processChar processes a single character
func (vt *VirtualTerminal) processChar(ch rune) {
	switch ch {
	case '\n':
		vt.newLine()
	case '\r':
		vt.cursorX = 0
	case '\b':
		if vt.cursorX > 0 {
			vt.cursorX--
		}
	case '\t':
		// Move to next tab stop (every 8 columns)
		vt.cursorX = ((vt.cursorX / 8) + 1) * 8
		if vt.cursorX >= vt.cols {
			vt.cursorX = vt.cols - 1
		}
	case '\x1b':
		// Start of escape sequence - handled by stripping later
	default:
		if ch >= ' ' && ch != '\x7f' {
			vt.putChar(ch)
		}
	}
}

// putChar puts a character at the current cursor position
func (vt *VirtualTerminal) putChar(ch rune) {
	if vt.cursorX >= vt.cols {
		vt.newLine()
	}
	if vt.cursorY >= 0 && vt.cursorY < vt.rows && vt.cursorX >= 0 && vt.cursorX < vt.cols {
		vt.screen[vt.cursorY][vt.cursorX] = ch
	}
	vt.cursorX++
}

// newLine moves to the next line, scrolling if necessary
func (vt *VirtualTerminal) newLine() {
	vt.cursorX = 0
	vt.cursorY++
	if vt.cursorY >= vt.rows {
		vt.scroll()
		vt.cursorY = vt.rows - 1
	}
}

// scroll scrolls the screen up by one line
func (vt *VirtualTerminal) scroll() {
	// Save top line to history
	line := strings.TrimRight(string(vt.screen[0]), " ")
	if line != "" {
		vt.history = append(vt.history, line)
		// Trim history if too large
		if len(vt.history) > vt.maxHistory {
			vt.history = vt.history[1:]
		}
	}

	// Scroll screen up
	for i := 0; i < vt.rows-1; i++ {
		vt.screen[i] = vt.screen[i+1]
	}

	// Clear bottom line
	vt.screen[vt.rows-1] = make([]rune, vt.cols)
	for j := range vt.screen[vt.rows-1] {
		vt.screen[vt.rows-1][j] = ' '
	}
}

// Resize resizes the terminal
func (vt *VirtualTerminal) Resize(cols, rows int) {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	if cols <= 0 {
		cols = 80
	}
	if rows <= 0 {
		rows = 24
	}

	vt.cols = cols
	vt.rows = rows
	vt.initScreen()
}

// GetDisplay returns the current screen content
func (vt *VirtualTerminal) GetDisplay() string {
	vt.mu.RLock()
	defer vt.mu.RUnlock()

	if !vt.hasData {
		return ""
	}

	var lines []string
	for _, row := range vt.screen {
		line := strings.TrimRight(string(row), " ")
		lines = append(lines, line)
	}

	// Remove trailing empty lines
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	return strings.Join(lines, "\n")
}

// GetOutput returns recent terminal output (history + current screen)
func (vt *VirtualTerminal) GetOutput(lines int) string {
	vt.mu.RLock()
	defer vt.mu.RUnlock()

	if !vt.hasData {
		return ""
	}

	var result []string

	// Add from history
	result = append(result, vt.history...)

	// Add current screen content (non-empty lines only)
	for _, row := range vt.screen {
		line := strings.TrimRight(string(row), " ")
		if line != "" {
			result = append(result, line)
		}
	}

	// Return last N lines
	if len(result) > lines {
		result = result[len(result)-lines:]
	}

	return strings.Join(result, "\n")
}

// GetScreenSnapshot returns a snapshot of the current screen
func (vt *VirtualTerminal) GetScreenSnapshot() string {
	return vt.GetDisplay()
}

// Clear clears the terminal and history
func (vt *VirtualTerminal) Clear() {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	vt.initScreen()
	vt.history = make([]string, 0)
	vt.hasData = false
}

// CursorPosition returns the current cursor position
func (vt *VirtualTerminal) CursorPosition() (row, col int) {
	vt.mu.RLock()
	defer vt.mu.RUnlock()
	return vt.cursorY, vt.cursorX
}

// Cols returns the terminal width
func (vt *VirtualTerminal) Cols() int {
	vt.mu.RLock()
	defer vt.mu.RUnlock()
	return vt.cols
}

// Rows returns the terminal height
func (vt *VirtualTerminal) Rows() int {
	vt.mu.RLock()
	defer vt.mu.RUnlock()
	return vt.rows
}

// StripANSI removes ANSI escape sequences from text
func StripANSI(text string) string {
	return ansiPattern.ReplaceAllString(text, "")
}

// StripANSIBytes removes ANSI escape sequences from bytes
func StripANSIBytes(data []byte) []byte {
	return bytes.ReplaceAll(
		bytes.ReplaceAll(data, []byte("\x1b["), []byte("")),
		[]byte("\x1b"), []byte(""),
	)
}
