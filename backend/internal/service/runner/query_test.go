package runner

import (
	"context"
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetByNodeID(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create a runner
	r := &runner.Runner{
		OrganizationID: 1,
		NodeID:         "test-node-123",
		Status:         runner.RunnerStatusOnline,
	}
	require.NoError(t, db.Create(r).Error)

	t.Run("returns runner by node ID", func(t *testing.T) {
		result, err := service.GetByNodeID(ctx, "test-node-123")
		require.NoError(t, err)
		assert.Equal(t, r.ID, result.ID)
		assert.Equal(t, "test-node-123", result.NodeID)
	})

	t.Run("returns error for non-existent node ID", func(t *testing.T) {
		_, err := service.GetByNodeID(ctx, "non-existent-node")
		assert.Equal(t, ErrRunnerNotFound, err)
	})
}

func TestUpdateLastSeen(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create a runner with offline status
	r := &runner.Runner{
		OrganizationID: 1,
		NodeID:         "test-node",
		Status:         runner.RunnerStatusOffline,
	}
	require.NoError(t, db.Create(r).Error)

	// Update last seen
	err := service.UpdateLastSeen(ctx, r.ID)
	require.NoError(t, err)

	// Verify the runner was updated
	var updated runner.Runner
	require.NoError(t, db.First(&updated, r.ID).Error)

	assert.Equal(t, runner.RunnerStatusOnline, updated.Status)
	assert.NotNil(t, updated.LastHeartbeat)
}

func TestGetRunner(t *testing.T) {
	db := setupTestDB(t)
	service := NewService(db)
	ctx := context.Background()

	// Create a runner
	r := &runner.Runner{
		OrganizationID: 1,
		NodeID:         "test-node",
		Status:         runner.RunnerStatusOnline,
	}
	require.NoError(t, db.Create(r).Error)

	t.Run("returns runner from database", func(t *testing.T) {
		result, err := service.GetRunner(ctx, r.ID)
		require.NoError(t, err)
		assert.Equal(t, r.ID, result.ID)
		assert.Equal(t, "test-node", result.NodeID)
	})

	t.Run("returns error for non-existent runner", func(t *testing.T) {
		_, err := service.GetRunner(ctx, 99999)
		assert.Equal(t, ErrRunnerNotFound, err)
	})

	t.Run("returns runner from cache when available", func(t *testing.T) {
		// Add to active runners cache
		cachedRunner := &runner.Runner{
			ID:             r.ID,
			OrganizationID: 1,
			NodeID:         "cached-node",
		}
		service.activeRunners.Store(r.ID, &ActiveRunner{
			Runner: cachedRunner,
		})

		result, err := service.GetRunner(ctx, r.ID)
		require.NoError(t, err)
		assert.Equal(t, "cached-node", result.NodeID) // Should return cached version
	})
}

// Note: TestListRunners, TestListAvailableRunners, TestSelectAvailableRunner are defined in registration_test.go

func TestSelectAvailableRunnerForAgent(t *testing.T) {
	ctx := context.Background()

	t.Run("returns runner from cache when it supports the agent", func(t *testing.T) {
		db := setupTestDB(t)
		service := NewService(db)

		r := &runner.Runner{
			ID:              1,
			OrganizationID:  1,
			NodeID:          "runner-1",
			Status:          runner.RunnerStatusOnline,
			IsEnabled:       true,
			MaxConcurrentPods: 5,
			AvailableAgents: runner.StringSlice{"claude-code", "aider"},
		}
		service.activeRunners.Store(r.ID, &ActiveRunner{
			Runner:   r,
			LastPing: time.Now(),
			PodCount: 1,
		})

		result, err := service.SelectAvailableRunnerForAgent(ctx, 1, "claude-code")
		require.NoError(t, err)
		assert.Equal(t, int64(1), result.ID)
	})

	t.Run("skips cached runner that does not support the agent", func(t *testing.T) {
		db := setupTestDB(t)
		service := NewService(db)

		// This runner only supports "aider", not "claude-code"
		r := &runner.Runner{
			ID:              1,
			OrganizationID:  1,
			NodeID:          "runner-1",
			Status:          runner.RunnerStatusOnline,
			IsEnabled:       true,
			MaxConcurrentPods: 5,
			AvailableAgents: runner.StringSlice{"aider"},
		}
		service.activeRunners.Store(r.ID, &ActiveRunner{
			Runner:   r,
			LastPing: time.Now(),
			PodCount: 0,
		})

		// No DB runners either → expect ErrNoRunnerForAgent
		// Note: SQLite doesn't support @> operator, so DB fallback will fail.
		// We verify that cache filtering works by ensuring the aider-only runner is skipped.
		_, err := service.SelectAvailableRunnerForAgent(ctx, 1, "claude-code")
		assert.Error(t, err)
	})

	t.Run("selects least-loaded runner from cache", func(t *testing.T) {
		db := setupTestDB(t)
		service := NewService(db)

		r1 := &runner.Runner{
			ID: 1, OrganizationID: 1, NodeID: "runner-1",
			Status: runner.RunnerStatusOnline, IsEnabled: true, MaxConcurrentPods: 5,
			AvailableAgents: runner.StringSlice{"claude-code"},
		}
		r2 := &runner.Runner{
			ID: 2, OrganizationID: 1, NodeID: "runner-2",
			Status: runner.RunnerStatusOnline, IsEnabled: true, MaxConcurrentPods: 5,
			AvailableAgents: runner.StringSlice{"claude-code"},
		}

		service.activeRunners.Store(r1.ID, &ActiveRunner{Runner: r1, LastPing: time.Now(), PodCount: 3})
		service.activeRunners.Store(r2.ID, &ActiveRunner{Runner: r2, LastPing: time.Now(), PodCount: 1})

		result, err := service.SelectAvailableRunnerForAgent(ctx, 1, "claude-code")
		require.NoError(t, err)
		assert.Equal(t, int64(2), result.ID) // runner-2 has fewer pods
	})

	t.Run("ignores runners from different organization", func(t *testing.T) {
		db := setupTestDB(t)
		service := NewService(db)

		r := &runner.Runner{
			ID: 1, OrganizationID: 999, NodeID: "runner-other-org",
			Status: runner.RunnerStatusOnline, IsEnabled: true, MaxConcurrentPods: 5,
			AvailableAgents: runner.StringSlice{"claude-code"},
		}
		service.activeRunners.Store(r.ID, &ActiveRunner{Runner: r, LastPing: time.Now(), PodCount: 0})

		_, err := service.SelectAvailableRunnerForAgent(ctx, 1, "claude-code")
		assert.Error(t, err)
	})

	t.Run("ignores disabled runners in cache", func(t *testing.T) {
		db := setupTestDB(t)
		service := NewService(db)

		r := &runner.Runner{
			ID: 1, OrganizationID: 1, NodeID: "runner-disabled",
			Status: runner.RunnerStatusOnline, IsEnabled: false, MaxConcurrentPods: 5,
			AvailableAgents: runner.StringSlice{"claude-code"},
		}
		service.activeRunners.Store(r.ID, &ActiveRunner{Runner: r, LastPing: time.Now(), PodCount: 0})

		_, err := service.SelectAvailableRunnerForAgent(ctx, 1, "claude-code")
		assert.Error(t, err)
	})

	t.Run("ignores runners at capacity in cache", func(t *testing.T) {
		db := setupTestDB(t)
		service := NewService(db)

		r := &runner.Runner{
			ID: 1, OrganizationID: 1, NodeID: "runner-full",
			Status: runner.RunnerStatusOnline, IsEnabled: true, MaxConcurrentPods: 2,
			AvailableAgents: runner.StringSlice{"claude-code"},
		}
		service.activeRunners.Store(r.ID, &ActiveRunner{Runner: r, LastPing: time.Now(), PodCount: 2})

		_, err := service.SelectAvailableRunnerForAgent(ctx, 1, "claude-code")
		assert.Error(t, err)
	})

	t.Run("ignores stale runners in cache", func(t *testing.T) {
		db := setupTestDB(t)
		service := NewService(db)

		r := &runner.Runner{
			ID: 1, OrganizationID: 1, NodeID: "runner-stale",
			Status: runner.RunnerStatusOnline, IsEnabled: true, MaxConcurrentPods: 5,
			AvailableAgents: runner.StringSlice{"claude-code"},
		}
		// Last ping was 2 minutes ago → stale (>90s)
		service.activeRunners.Store(r.ID, &ActiveRunner{Runner: r, LastPing: time.Now().Add(-2 * time.Minute), PodCount: 0})

		_, err := service.SelectAvailableRunnerForAgent(ctx, 1, "claude-code")
		assert.Error(t, err)
	})
}
