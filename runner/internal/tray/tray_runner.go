//go:build desktop

package tray

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/anthropics/agentsmesh/runner/internal/runner"
)

func (t *TrayApp) startRunner() {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return
	}
	t.mu.Unlock()

	// Create runner instance
	r, err := runner.New(t.cfg)
	if err != nil {
		log.Error("Failed to create runner", "error", err)
		t.updateStatus(false, false, err)
		return
	}

	t.mu.Lock()
	t.runner = r
	t.ctx, t.cancel = context.WithCancel(context.Background())
	t.running = true
	t.mu.Unlock()

	// Update UI
	t.updateStatus(true, false, nil)

	// Start runner in background
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		// Simulate connection after a short delay
		go func() {
			time.Sleep(2 * time.Second)
			t.mu.Lock()
			if t.running {
				t.connected = true
			}
			t.mu.Unlock()
			t.updateStatus(true, true, nil)
		}()

		if err := r.Run(t.ctx); err != nil {
			log.Error("Runner error", "error", err)
			t.updateStatus(false, false, err)
		}

		t.mu.Lock()
		t.running = false
		t.connected = false
		t.mu.Unlock()
		t.updateStatus(false, false, nil)
	}()
}

func (t *TrayApp) stopRunner() {
	t.mu.Lock()
	if !t.running || t.cancel == nil {
		t.mu.Unlock()
		return
	}
	cancel := t.cancel
	t.mu.Unlock()

	cancel()
	t.wg.Wait()

	t.updateStatus(false, false, nil)
}

func (t *TrayApp) openWebConsole() {
	// Use local console URL
	url := fmt.Sprintf("http://127.0.0.1:%d", DefaultConsolePort)
	if t.console != nil {
		url = t.console.GetURL()
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}

	if cmd != nil {
		if err := cmd.Start(); err != nil {
			log.Error("Failed to open web console", "error", err)
		}
	}
}

func (t *TrayApp) openLogs() {
	// Open web console (logs are shown on the main page)
	url := fmt.Sprintf("http://127.0.0.1:%d#logs", DefaultConsolePort)
	if t.console != nil {
		url = t.console.GetURL() + "#logs"
	}

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}

	if cmd != nil {
		if err := cmd.Start(); err != nil {
			log.Error("Failed to open logs", "error", err)
		}
	}
}

func (t *TrayApp) toggleAutoStart() {
	if t.autoStartItem.Checked() {
		t.autoStartItem.Uncheck()
		// TODO: Remove from system auto-start
		log.Info("Auto-start disabled")
	} else {
		t.autoStartItem.Check()
		// TODO: Add to system auto-start
		log.Info("Auto-start enabled")
	}
}
