//go:build desktop

package tray

import (
	"context"
	"fmt"
	"time"

	"github.com/anthropics/agentsmesh/runner/internal/updater"
)

// initUpdateChecker initializes the background update checker.
func (t *TrayApp) initUpdateChecker() {
	if !t.cfg.AutoUpdate.Enabled {
		log.Info("Auto-update is disabled")
		return
	}

	// Create updater
	opts := []updater.Option{}
	if t.cfg.AutoUpdate.Channel == "beta" {
		opts = append(opts, updater.WithPrerelease(true))
	}
	u := updater.New(t.version, opts...)

	// Create graceful updater with pod counter
	podCounter := func() int {
		t.mu.RLock()
		r := t.runner
		t.mu.RUnlock()
		if r != nil {
			return r.GetActivePodCount()
		}
		return 0
	}

	t.gracefulUpdate = updater.NewGracefulUpdater(
		u, podCounter,
		updater.WithMaxWaitTime(t.cfg.AutoUpdate.MaxWaitTime),
		updater.WithStatusCallback(t.onUpdateStatus),
	)

	// Create background checker
	t.updateChecker = updater.NewBackgroundChecker(
		u, t.gracefulUpdate, t.cfg.AutoUpdate.CheckInterval,
		updater.WithOnUpdate(t.onUpdateAvailable),
		updater.WithAutoApply(t.cfg.AutoUpdate.AutoApply),
	)

	// Start background checks
	t.updateChecker.Start(context.Background())
	log.Info("Auto-update enabled", "checkInterval", t.cfg.AutoUpdate.CheckInterval)
}

// onUpdateAvailable is called when a new update is found.
func (t *TrayApp) onUpdateAvailable(info *updater.UpdateInfo) {
	t.mu.Lock()
	t.updateInfo = info
	t.mu.Unlock()

	t.updateItem.SetTitle(fmt.Sprintf("Update Available (%s)", info.LatestVersion))

	if t.console != nil {
		t.console.AddLog("info", fmt.Sprintf("Update available: %s -> %s", info.CurrentVersion, info.LatestVersion))
	}
}

// onUpdateStatus is called when the update status changes.
func (t *TrayApp) onUpdateStatus(state updater.State, info *updater.UpdateInfo, activePods int) {
	switch state {
	case updater.StateDraining:
		t.updateItem.SetTitle(fmt.Sprintf("⏳ Waiting to Update (%d pods active)", activePods))
	case updater.StateDownloading:
		t.updateItem.SetTitle("🔄 Downloading Update...")
	case updater.StateApplying:
		t.updateItem.SetTitle("🔄 Applying Update...")
	case updater.StateRestarting:
		t.updateItem.SetTitle("🔄 Restarting...")
	case updater.StateIdle:
		t.mu.RLock()
		hasUpdate := t.updateInfo != nil && t.updateInfo.HasUpdate
		t.mu.RUnlock()
		if hasUpdate {
			t.updateItem.SetTitle(fmt.Sprintf("Update Available (%s)", t.updateInfo.LatestVersion))
		} else {
			t.updateItem.SetTitle("Check for Updates...")
		}
	}
}

// checkForUpdates manually triggers an update check.
func (t *TrayApp) checkForUpdates() {
	t.updateItem.SetTitle("Checking for Updates...")

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// Use existing background checker if available
		var info *updater.UpdateInfo
		var err error

		if t.updateChecker != nil {
			info, err = t.updateChecker.CheckNow(ctx)
		} else {
			// Fallback: create a new updater if checker not initialized
			opts := []updater.Option{}
			if t.cfg.AutoUpdate.Channel == "beta" {
				opts = append(opts, updater.WithPrerelease(true))
			}
			u := updater.New(t.version, opts...)
			info, err = u.CheckForUpdate(ctx)
		}

		if err != nil {
			log.Error("Update check failed", "error", err)
			t.updateItem.SetTitle("Check for Updates...")
			if t.console != nil {
				t.console.AddLog("error", fmt.Sprintf("Update check failed: %v", err))
			}
			return
		}

		if !info.HasUpdate {
			t.updateItem.SetTitle("Up to Date ✓")
			if t.console != nil {
				t.console.AddLog("info", "You are running the latest version")
			}
			// Reset title after a few seconds
			go func() {
				time.Sleep(3 * time.Second)
				t.updateItem.SetTitle("Check for Updates...")
			}()
			return
		}

		t.onUpdateAvailable(info)
	}()
}
