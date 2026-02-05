package terminal

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"

	"github.com/anthropics/agentsmesh/runner/internal/logger"
)

// Options for creating a new terminal.
type Options struct {
	Command  string
	Args     []string
	WorkDir  string
	Env      map[string]string
	Rows     int
	Cols     int
	OnOutput func([]byte)
	OnExit   func(int)
}

// Terminal represents a PTY terminal session.
type Terminal struct {
	cmd      *exec.Cmd
	pty      *os.File
	mu       sync.Mutex
	closed   bool
	onOutput func([]byte)
	onExit   func(int)

	// Terminal size (set at creation, used when starting PTY)
	rows int
	cols int

	// Backpressure control (ttyd-style flow control)
	// When paused, readOutput() blocks to prevent unbounded memory growth
	readPaused  bool          // Whether PTY reading is paused
	readPauseMu sync.RWMutex  // Protects readPaused flag
	resumeCh    chan struct{} // Signal to resume reading
}

// New creates a new terminal instance.
func New(opts Options) (*Terminal, error) {
	if opts.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Build command
	cmd := exec.Command(opts.Command, opts.Args...)
	cmd.Dir = opts.WorkDir

	// Build environment
	env := os.Environ()

	// Ensure terminal supports colors (critical for CLI tools like claude, ls, etc.)
	env = append(env, "TERM=xterm-256color")
	env = append(env, "COLORTERM=truecolor")

	for k, v := range opts.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	// Default terminal size if not specified
	rows := opts.Rows
	cols := opts.Cols
	if rows <= 0 {
		rows = 24
	}
	if cols <= 0 {
		cols = 80
	}

	return &Terminal{
		cmd:      cmd,
		onOutput: opts.OnOutput,
		onExit:   opts.OnExit,
		rows:     rows,
		cols:     cols,
		resumeCh: make(chan struct{}, 1), // Buffered to avoid blocking
	}, nil
}

// Start starts the terminal process
func (t *Terminal) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return fmt.Errorf("terminal is closed")
	}

	log := logger.Terminal()
	log.Debug("Starting command", "path", t.cmd.Path, "args", t.cmd.Args, "dir", t.cmd.Dir, "cols", t.cols, "rows", t.rows)

	// Start with PTY and initial size
	// Use StartWithSize to set correct terminal dimensions from the beginning
	// This is critical for TUI applications like Claude Code that render based on terminal size
	winSize := &pty.Winsize{
		Rows: uint16(t.rows),
		Cols: uint16(t.cols),
	}
	ptmx, err := pty.StartWithSize(t.cmd, winSize)
	if err != nil {
		return fmt.Errorf("failed to start pty: %w", err)
	}
	t.pty = ptmx

	log.Debug("PTY started", "pid", t.cmd.Process.Pid, "cols", t.cols, "rows", t.rows)

	// Start output reader
	go t.readOutput()

	// Wait for process exit
	go t.waitExit()

	return nil
}

// Write writes data to the terminal
func (t *Terminal) Write(data []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed || t.pty == nil {
		return fmt.Errorf("terminal is not running")
	}

	_, err := t.pty.Write(data)
	return err
}

// Resize resizes the terminal.
// Parameters follow the standard convention: cols (width) first, then rows (height).
// This matches xterm.js, ANSI standards, and most terminal libraries.
func (t *Terminal) Resize(cols, rows int) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed || t.pty == nil {
		return fmt.Errorf("terminal is not running")
	}

	return pty.Setsize(t.pty, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

// Redraw triggers a terminal redraw by temporarily changing the terminal size.
// This is used to restore terminal state after server restart.
// We use resize +1/-1 instead of just SIGWINCH because some programs (like Claude Code)
// don't respond to SIGWINCH when they're in an idle/waiting state.
func (t *Terminal) Redraw() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed || t.pty == nil {
		return fmt.Errorf("terminal is not running")
	}

	// Get current size
	size, err := pty.GetsizeFull(t.pty)
	if err != nil {
		return fmt.Errorf("failed to get terminal size: %w", err)
	}

	// Resize to cols+1 to trigger redraw
	if err := pty.Setsize(t.pty, &pty.Winsize{
		Rows: size.Rows,
		Cols: size.Cols + 1,
	}); err != nil {
		return fmt.Errorf("failed to expand terminal: %w", err)
	}

	// Small delay to ensure the resize is processed
	time.Sleep(50 * time.Millisecond)

	// Resize back to original size
	if err := pty.Setsize(t.pty, &pty.Winsize{
		Rows: size.Rows,
		Cols: size.Cols,
	}); err != nil {
		return fmt.Errorf("failed to restore terminal size: %w", err)
	}

	return nil
}

// PID returns the process ID
func (t *Terminal) PID() int {
	if t.cmd != nil && t.cmd.Process != nil {
		return t.cmd.Process.Pid
	}
	return 0
}

// IsClosed returns whether the terminal is closed.
func (t *Terminal) IsClosed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.closed
}

// Stop stops the terminal
func (t *Terminal) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return
	}
	t.closed = true

	// Try graceful shutdown first
	if t.cmd != nil && t.cmd.Process != nil {
		t.cmd.Process.Signal(syscall.SIGTERM)
	}

	// Close PTY
	if t.pty != nil {
		t.pty.Close()
	}
}

// SetOutputHandler sets the output handler callback.
// Must be called before Start().
func (t *Terminal) SetOutputHandler(handler func([]byte)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onOutput = handler
}

// SetExitHandler sets the exit handler callback.
// Must be called before Start().
func (t *Terminal) SetExitHandler(handler func(int)) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onExit = handler
}

// PauseRead pauses PTY reading (backpressure signal from consumer).
// This implements ttyd-style flow control: when the consumer can't keep up,
// we stop reading from the PTY to prevent unbounded memory growth.
// The readOutput goroutine will block until ResumeRead is called.
func (t *Terminal) PauseRead() {
	t.readPauseMu.Lock()
	wasPaused := t.readPaused
	t.readPaused = true
	t.readPauseMu.Unlock()

	if !wasPaused {
		logger.Terminal().Debug("PTY read paused (backpressure)")
	}
}

// ResumeRead resumes PTY reading after backpressure is released.
// This signals the readOutput goroutine to continue reading.
func (t *Terminal) ResumeRead() {
	t.readPauseMu.Lock()
	wasPaused := t.readPaused
	t.readPaused = false
	t.readPauseMu.Unlock()

	if wasPaused {
		// Signal the resume channel (non-blocking)
		select {
		case t.resumeCh <- struct{}{}:
		default:
			// Channel already has a signal pending
		}
		logger.Terminal().Debug("PTY read resumed")
	}
}

// IsReadPaused returns whether PTY reading is currently paused.
func (t *Terminal) IsReadPaused() bool {
	t.readPauseMu.RLock()
	defer t.readPauseMu.RUnlock()
	return t.readPaused
}

// IsRaw checks if terminal is in raw mode
func IsRaw(fd int) bool {
	return term.IsTerminal(fd)
}

// MakeRaw puts terminal in raw mode
func MakeRaw(fd int) (*term.State, error) {
	return term.MakeRaw(fd)
}

// Restore restores terminal state
func Restore(fd int, state *term.State) error {
	return term.Restore(fd, state)
}
