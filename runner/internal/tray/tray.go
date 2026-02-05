//go:build desktop

// Package tray provides system tray functionality for the desktop mode.
package tray

import (
	"context"
	"fmt"
	"sync"

	"github.com/getlantern/systray"

	"github.com/anthropics/agentsmesh/runner/internal/config"
	"github.com/anthropics/agentsmesh/runner/internal/console"
	"github.com/anthropics/agentsmesh/runner/internal/logger"
	"github.com/anthropics/agentsmesh/runner/internal/runner"
	"github.com/anthropics/agentsmesh/runner/internal/updater"
)

// Module logger for tray
var log = logger.Tray()

const (
	// DefaultConsolePort is the default port for the web console.
	DefaultConsolePort = 19080
)

// TrayApp represents the system tray application.
type TrayApp struct {
	cfg     *config.Config
	version string
	runner  *runner.Runner
	console *console.Server

	// State
	running   bool
	connected bool
	mu        sync.RWMutex

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	// Update management
	updateChecker  *updater.BackgroundChecker
	gracefulUpdate *updater.GracefulUpdater
	updateInfo     *updater.UpdateInfo

	// Menu items
	statusItem    *systray.MenuItem
	toggleItem    *systray.MenuItem
	webItem       *systray.MenuItem
	logsItem      *systray.MenuItem
	updateItem    *systray.MenuItem
	autoStartItem *systray.MenuItem
	quitItem      *systray.MenuItem
}

// New creates a new tray application.
func New(cfg *config.Config) *TrayApp {
	return &TrayApp{
		cfg:     cfg,
		version: "dev",
	}
}

// NewWithVersion creates a new tray application with version info.
func NewWithVersion(cfg *config.Config, version string) *TrayApp {
	return &TrayApp{
		cfg:     cfg,
		version: version,
	}
}

// Run starts the tray application (blocking).
func (t *TrayApp) Run() {
	systray.Run(t.onReady, t.onExit)
}

func (t *TrayApp) onReady() {
	// Set icon
	systray.SetIcon(getIcon())
	systray.SetTitle("AgentsMesh Runner")
	systray.SetTooltip("AgentsMesh Runner")

	// Start web console
	t.console = console.New(t.cfg, DefaultConsolePort, t.version)
	if err := t.console.Start(); err != nil {
		log.Error("Failed to start web console", "error", err)
	} else {
		log.Info("Web console available", "url", t.console.GetURL())
	}

	// Build menu
	t.statusItem = systray.AddMenuItem("Status: Stopped", "Current runner status")
	t.statusItem.Disable()

	systray.AddSeparator()

	t.toggleItem = systray.AddMenuItem("Start Runner", "Start/Stop the runner")
	t.webItem = systray.AddMenuItem("Open Web Console", "Open AgentsMesh web console")
	t.logsItem = systray.AddMenuItem("View Logs", "View runner logs")

	systray.AddSeparator()

	t.updateItem = systray.AddMenuItem("Check for Updates...", "Check for available updates")
	t.autoStartItem = systray.AddMenuItemCheckbox("Start at Login", "Auto-start runner at login", false)

	systray.AddSeparator()

	t.quitItem = systray.AddMenuItem("Quit", "Quit AgentsMesh Runner")

	// Initialize update checker
	t.initUpdateChecker()

	// Handle menu events
	go t.handleEvents()

	// Auto-start runner
	t.startRunner()
}

func (t *TrayApp) onExit() {
	log.Info("Tray exiting")

	// Stop update checker
	if t.updateChecker != nil {
		t.updateChecker.Stop()
	}

	t.stopRunner()

	// Stop web console
	if t.console != nil {
		t.console.Stop()
	}
}

func (t *TrayApp) handleEvents() {
	for {
		select {
		case <-t.toggleItem.ClickedCh:
			t.mu.RLock()
			isRunning := t.running
			t.mu.RUnlock()

			if isRunning {
				t.stopRunner()
			} else {
				t.startRunner()
			}

		case <-t.webItem.ClickedCh:
			t.openWebConsole()

		case <-t.logsItem.ClickedCh:
			t.openLogs()

		case <-t.updateItem.ClickedCh:
			t.checkForUpdates()

		case <-t.autoStartItem.ClickedCh:
			t.toggleAutoStart()

		case <-t.quitItem.ClickedCh:
			systray.Quit()
			return
		}
	}
}

func (t *TrayApp) updateStatus(running, connected bool, err error) {
	t.mu.Lock()
	t.running = running
	t.connected = connected
	t.mu.Unlock()

	var statusText string
	var errMsg string
	if err != nil {
		statusText = fmt.Sprintf("Status: Error - %v", err)
		errMsg = err.Error()
		systray.SetIcon(getIconError())
	} else if running && connected {
		statusText = "Status: Connected"
		systray.SetIcon(getIconConnected())
	} else if running {
		statusText = "Status: Connecting..."
		systray.SetIcon(getIconConnecting())
	} else {
		statusText = "Status: Stopped"
		systray.SetIcon(getIcon())
	}

	t.statusItem.SetTitle(statusText)

	if running {
		t.toggleItem.SetTitle("Stop Runner")
	} else {
		t.toggleItem.SetTitle("Start Runner")
	}

	// Update console status
	if t.console != nil {
		t.console.UpdateStatus(running, connected, 0, 0, errMsg)
		if err != nil {
			t.console.AddLog("error", errMsg)
		} else if connected {
			t.console.AddLog("info", "Connected to server")
		} else if running {
			t.console.AddLog("info", "Connecting to server...")
		} else {
			t.console.AddLog("info", "Runner stopped")
		}
	}
}
