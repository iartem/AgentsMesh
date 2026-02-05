package updater

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for rollback and error propagation

func TestGracefulUpdater_ApplyPendingUpdate_RestartErrorPropagation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "graceful-reliability-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	execPath := filepath.Join(tmpDir, "runner")
	pendingPath := filepath.Join(tmpDir, "pending-binary")

	err = os.WriteFile(execPath, []byte("old binary"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(pendingPath, []byte("new binary"), 0755)
	require.NoError(t, err)

	mock := &MockReleaseDetector{}
	u := New("1.0.0",
		WithReleaseDetector(mock),
		WithExecPathFunc(func() (string, error) { return execPath, nil }),
	)

	restartErr := errors.New("simulated restart failure")
	g := NewGracefulUpdater(u, nil, WithRestartFunc(func() (int, error) {
		return 0, restartErr
	}))

	g.mu.Lock()
	g.pendingPath = pendingPath
	g.pendingInfo = &UpdateInfo{LatestVersion: "v2.0.0", CurrentVersion: "v1.0.0"}
	g.mu.Unlock()

	// Apply should now return the restart error
	err = g.applyPendingUpdate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restart failed")
	assert.Contains(t, err.Error(), "simulated restart failure")
	// State should be reset to Idle after failure
	assert.Equal(t, StateIdle, g.State())
}

func TestGracefulUpdater_ApplyPendingUpdate_HealthCheckFailed_Rollback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "graceful-reliability-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	execPath := filepath.Join(tmpDir, "runner")
	pendingPath := filepath.Join(tmpDir, "pending-binary")

	err = os.WriteFile(execPath, []byte("old binary"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(pendingPath, []byte("new binary"), 0755)
	require.NoError(t, err)

	mock := &MockReleaseDetector{}
	u := New("1.0.0",
		WithReleaseDetector(mock),
		WithExecPathFunc(func() (string, error) { return execPath, nil }),
	)

	healthCheckErr := errors.New("health check failed: process crashed")
	g := NewGracefulUpdater(u, nil,
		WithRestartFunc(func() (int, error) {
			return 99999, nil // Return a fake PID (process won't exist)
		}),
		WithHealthChecker(func(ctx context.Context, pid int) error {
			return healthCheckErr
		}),
		WithHealthTimeout(100*time.Millisecond),
	)

	g.mu.Lock()
	g.pendingPath = pendingPath
	g.pendingInfo = &UpdateInfo{LatestVersion: "v2.0.0", CurrentVersion: "v1.0.0"}
	g.mu.Unlock()

	err = g.applyPendingUpdate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "health check failed")
	assert.Equal(t, StateIdle, g.State())

	// Verify rollback was attempted (binary should be restored)
	content, err := os.ReadFile(execPath)
	require.NoError(t, err)
	assert.Equal(t, "old binary", string(content))
}

func TestGracefulUpdater_ApplyPendingUpdate_RestartFailed_Rollback(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "graceful-reliability-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	execPath := filepath.Join(tmpDir, "runner")
	pendingPath := filepath.Join(tmpDir, "pending-binary")

	err = os.WriteFile(execPath, []byte("old binary"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(pendingPath, []byte("new binary"), 0755)
	require.NoError(t, err)

	mock := &MockReleaseDetector{}
	u := New("1.0.0",
		WithReleaseDetector(mock),
		WithExecPathFunc(func() (string, error) { return execPath, nil }),
	)

	g := NewGracefulUpdater(u, nil,
		WithRestartFunc(func() (int, error) {
			return 0, errors.New("failed to start new process")
		}),
	)

	g.mu.Lock()
	g.pendingPath = pendingPath
	g.pendingInfo = &UpdateInfo{LatestVersion: "v2.0.0", CurrentVersion: "v1.0.0"}
	g.mu.Unlock()

	err = g.applyPendingUpdate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "restart failed")
	assert.Equal(t, StateIdle, g.State())

	// Verify rollback was attempted (binary should be restored)
	content, err := os.ReadFile(execPath)
	require.NoError(t, err)
	assert.Equal(t, "old binary", string(content))
}
