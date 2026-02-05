package updater

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"
)

// ScheduleUpdate checks for updates and schedules a graceful update if available.
// It downloads the update, then waits for all pods to finish before applying.
// If maxWaitTime is reached, the update is postponed to the next check cycle.
func (g *GracefulUpdater) ScheduleUpdate(ctx context.Context) error {
	// Atomically check and set state to avoid race condition
	g.mu.Lock()
	if g.state != StateIdle {
		currentState := g.state
		g.mu.Unlock()
		return fmt.Errorf("update already in progress (state: %s)", currentState)
	}
	g.state = StateChecking
	g.mu.Unlock()

	// Notify status change
	if g.onStatus != nil {
		g.onStatus(StateChecking, nil, 0)
	}
	info, err := g.updater.CheckForUpdate(ctx)
	if err != nil {
		g.setState(StateIdle)
		return fmt.Errorf("failed to check for updates: %w", err)
	}

	if !info.HasUpdate {
		g.setState(StateIdle)
		return nil
	}

	log.Printf("[updater] New version available: %s -> %s", info.CurrentVersion, info.LatestVersion)

	// Download update
	g.setState(StateDownloading)
	g.mu.Lock()
	g.pendingInfo = info
	g.mu.Unlock()

	tmpPath, err := g.updater.Download(ctx, info.LatestVersion, nil)
	if err != nil {
		g.setState(StateIdle)
		g.mu.Lock()
		g.pendingInfo = nil
		g.mu.Unlock()
		return fmt.Errorf("failed to download update: %w", err)
	}

	g.mu.Lock()
	g.pendingPath = tmpPath
	g.mu.Unlock()

	log.Printf("[updater] Update downloaded to %s, waiting for pods to finish...", tmpPath)

	// Wait for pods to finish
	return g.waitAndApply(ctx)
}

// waitAndApply waits for all pods to finish, then applies the update.
func (g *GracefulUpdater) waitAndApply(ctx context.Context) error {
	g.setState(StateDraining)
	g.mu.Lock()
	g.draining = true

	// Create cancellable context for draining
	drainCtx, cancel := context.WithTimeout(ctx, g.maxWaitTime)
	g.cancelDrain = cancel
	g.mu.Unlock()

	defer func() {
		cancel() // Always release context resources
		g.mu.Lock()
		g.draining = false
		g.cancelDrain = nil
		g.mu.Unlock()
	}()

	// Poll until no pods are active or timeout
	ticker := time.NewTicker(g.pollInterval)
	defer ticker.Stop()

	for {
		activePods := 0
		if g.podCounter != nil {
			activePods = g.podCounter()
		}

		if activePods == 0 {
			log.Printf("[updater] No active pods, applying update...")
			break
		}

		log.Printf("[updater] Waiting for %d active pod(s) to finish...", activePods)

		// Notify status
		if g.onStatus != nil {
			g.mu.RLock()
			info := g.pendingInfo
			g.mu.RUnlock()
			g.onStatus(StateDraining, info, activePods)
		}

		select {
		case <-drainCtx.Done():
			// Timeout or cancelled
			if drainCtx.Err() == context.DeadlineExceeded {
				log.Printf("[updater] Max wait time reached with %d active pods, postponing update", activePods)
				// Clean up pending update
				g.mu.Lock()
				if g.pendingPath != "" {
					os.Remove(g.pendingPath)
					g.pendingPath = ""
				}
				g.pendingInfo = nil
				g.mu.Unlock()
				g.setState(StateIdle)
				return fmt.Errorf("update postponed: max wait time reached with active pods")
			}
			// Cancelled
			g.mu.Lock()
			if g.pendingPath != "" {
				os.Remove(g.pendingPath)
				g.pendingPath = ""
			}
			g.pendingInfo = nil
			g.mu.Unlock()
			g.setState(StateIdle)
			return fmt.Errorf("update cancelled")
		case <-ticker.C:
			// Continue polling
		}
	}

	// Apply the update
	return g.applyPendingUpdate()
}

// ForceUpdate applies the update immediately without waiting for pods.
func (g *GracefulUpdater) ForceUpdate(ctx context.Context) error {
	g.mu.Lock()
	if g.state != StateIdle && g.state != StateDraining {
		currentState := g.state
		g.mu.Unlock()
		return fmt.Errorf("cannot force update in state: %s", currentState)
	}

	// Cancel ongoing drain if any
	if g.cancelDrain != nil {
		g.cancelDrain()
	}

	// Check if we have a pending update while still holding the lock
	hasPending := g.pendingPath != ""
	if !hasPending {
		// Set state to checking before releasing lock to prevent race
		g.state = StateChecking
	}
	g.mu.Unlock()

	// If we have a pending update, apply it
	if hasPending {
		return g.applyPendingUpdate()
	}

	// Notify status change (state already set above)
	if g.onStatus != nil {
		g.onStatus(StateChecking, nil, 0)
	}
	info, err := g.updater.CheckForUpdate(ctx)
	if err != nil {
		g.setState(StateIdle)
		return err
	}

	if !info.HasUpdate {
		g.setState(StateIdle)
		return fmt.Errorf("no update available")
	}

	g.setState(StateDownloading)
	g.mu.Lock()
	g.pendingInfo = info
	g.mu.Unlock()

	tmpPath, err := g.updater.Download(ctx, info.LatestVersion, nil)
	if err != nil {
		g.setState(StateIdle)
		return err
	}

	g.mu.Lock()
	g.pendingPath = tmpPath
	g.mu.Unlock()

	return g.applyPendingUpdate()
}
