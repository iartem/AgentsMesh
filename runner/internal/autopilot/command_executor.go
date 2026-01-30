// Package autopilot implements the AutopilotController for supervised Pod automation.
package autopilot

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// CommandExecutor abstracts command execution for testability and extensibility.
// This follows the Dependency Inversion Principle (DIP).
type CommandExecutor interface {
	// Execute runs a command and returns the combined stdout output.
	// Returns error if the command fails or times out.
	Execute(ctx context.Context, name string, args []string, workDir string) ([]byte, []byte, error)
}

// DefaultCommandExecutor is the standard implementation using os/exec.
type DefaultCommandExecutor struct{}

// NewDefaultCommandExecutor creates a new DefaultCommandExecutor.
func NewDefaultCommandExecutor() *DefaultCommandExecutor {
	return &DefaultCommandExecutor{}
}

// Execute runs a command using os/exec.
func (e *DefaultCommandExecutor) Execute(ctx context.Context, name string, args []string, workDir string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = workDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		if ctx.Err() != nil {
			return stdout.Bytes(), stderr.Bytes(), fmt.Errorf("command timed out: %w", ctx.Err())
		}
		return stdout.Bytes(), stderr.Bytes(), fmt.Errorf("command failed: %w", err)
	}

	return stdout.Bytes(), stderr.Bytes(), nil
}

// MockCommandExecutor is a test double for CommandExecutor.
type MockCommandExecutor struct {
	// ExecuteFunc is called when Execute is invoked.
	// Set this to customize behavior in tests.
	ExecuteFunc func(ctx context.Context, name string, args []string, workDir string) ([]byte, []byte, error)

	// Calls records all Execute calls for verification.
	Calls []CommandExecutorCall
}

// CommandExecutorCall records a single Execute call.
type CommandExecutorCall struct {
	Name    string
	Args    []string
	WorkDir string
}

// NewMockCommandExecutor creates a new MockCommandExecutor with default success behavior.
func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		ExecuteFunc: func(ctx context.Context, name string, args []string, workDir string) ([]byte, []byte, error) {
			return []byte("mock output"), nil, nil
		},
		Calls: make([]CommandExecutorCall, 0),
	}
}

// Execute records the call and delegates to ExecuteFunc.
func (m *MockCommandExecutor) Execute(ctx context.Context, name string, args []string, workDir string) ([]byte, []byte, error) {
	m.Calls = append(m.Calls, CommandExecutorCall{
		Name:    name,
		Args:    args,
		WorkDir: workDir,
	})
	return m.ExecuteFunc(ctx, name, args, workDir)
}

// SetOutput sets the mock to return the given output.
func (m *MockCommandExecutor) SetOutput(stdout, stderr []byte, err error) {
	m.ExecuteFunc = func(ctx context.Context, name string, args []string, workDir string) ([]byte, []byte, error) {
		return stdout, stderr, err
	}
}
