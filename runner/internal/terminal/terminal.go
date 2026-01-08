package terminal

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// Options for creating a new terminal (legacy API)
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

// Terminal represents a PTY terminal session (legacy API)
// For new code, prefer using Session and Manager
type Terminal struct {
	cmd      *exec.Cmd
	pty      *os.File
	mu       sync.Mutex
	closed   bool
	onOutput func([]byte)
	onExit   func(int)
}

// New creates a new terminal instance (legacy API)
func New(opts Options) (*Terminal, error) {
	if opts.Command == "" {
		return nil, fmt.Errorf("command is required")
	}

	// Build command
	cmd := exec.Command(opts.Command, opts.Args...)
	cmd.Dir = opts.WorkDir

	// Build environment
	env := os.Environ()
	for k, v := range opts.Env {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.Env = env

	return &Terminal{
		cmd:      cmd,
		onOutput: opts.OnOutput,
		onExit:   opts.OnExit,
	}, nil
}

// Start starts the terminal process
func (t *Terminal) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return fmt.Errorf("terminal is closed")
	}

	// Start with PTY
	ptmx, err := pty.Start(t.cmd)
	if err != nil {
		return fmt.Errorf("failed to start pty: %w", err)
	}
	t.pty = ptmx

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

// Resize resizes the terminal
func (t *Terminal) Resize(rows, cols int) error {
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

// PID returns the process ID
func (t *Terminal) PID() int {
	if t.cmd != nil && t.cmd.Process != nil {
		return t.cmd.Process.Pid
	}
	return 0
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

// readOutput reads output from the PTY and sends to handler
func (t *Terminal) readOutput() {
	buf := make([]byte, 4096)
	for {
		n, err := t.pty.Read(buf)
		if err != nil {
			if err != io.EOF {
				// Only log if not a normal close
				t.mu.Lock()
				closed := t.closed
				t.mu.Unlock()
				if !closed {
					// Log error but don't break - might be temporary
				}
			}
			break
		}

		if n > 0 && t.onOutput != nil {
			// Make a copy of the data
			data := make([]byte, n)
			copy(data, buf[:n])
			t.onOutput(data)
		}
	}
}

// waitExit waits for the process to exit
func (t *Terminal) waitExit() {
	exitCode := 0
	if err := t.cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = -1
		}
	}

	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()

	if t.pty != nil {
		t.pty.Close()
	}

	if t.onExit != nil {
		t.onExit(exitCode)
	}
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
