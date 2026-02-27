package runner

import (
	"github.com/anthropics/agentsmesh/runner/internal/client"
	"github.com/anthropics/agentsmesh/runner/internal/logger"
	"github.com/anthropics/agentsmesh/runner/internal/terminal/vt"
)

// createOSCHandler creates an OSC handler that sends terminal notifications to the server.
func (h *RunnerMessageHandler) createOSCHandler(podKey string) vt.OSCHandler {
	return func(oscType int, params []string) {
		log := logger.TerminalTrace()

		switch oscType {
		case 777:
			// OSC 777;notify;title;body - iTerm2/Kitty notification format
			if len(params) >= 3 && params[0] == "notify" {
				title := params[1]
				body := params[2]
				log.Trace("OSC 777 notification detected", "pod_key", podKey, "title", title, "body", body)
				if err := h.conn.SendOSCNotification(podKey, title, body); err != nil {
					log.Error("Failed to send OSC notification", "pod_key", podKey, "error", err)
				}
			}

		case 9:
			// OSC 9;message - ConEmu/Windows Terminal notification format
			if len(params) >= 1 {
				body := params[0]
				log.Trace("OSC 9 notification detected", "pod_key", podKey, "body", body)
				if err := h.conn.SendOSCNotification(podKey, "Notification", body); err != nil {
					log.Error("Failed to send OSC notification", "pod_key", podKey, "error", err)
				}
			}

		case 0, 2:
			// OSC 0/2;title - Window/tab title
			if len(params) >= 1 {
				title := params[0]
				log.Trace("OSC title change detected", "pod_key", podKey, "title", title)
				if err := h.conn.SendOSCTitle(podKey, title); err != nil {
					log.Error("Failed to send OSC title", "pod_key", podKey, "error", err)
				}
			}
		}
	}
}

// createExitHandler creates an exit handler that notifies server when pod exits.
func (h *RunnerMessageHandler) createExitHandler(podKey string) func(int) {
	return func(exitCode int) {
		log := logger.Pod()
		log.Info("Pod exited", "pod_key", podKey, "exit_code", exitCode)

		var earlyOutput string

		pod := h.podStore.Delete(podKey)
		if pod != nil {
			pod.SetStatus(PodStatusStopped)

			if pod.PTYLogger != nil {
				pod.PTYLogger.Close()
			}

			pod.StopStateDetector()

			// Stop aggregator BEFORE disconnecting relay, so the final flush
			// can still be sent through the relay if it's connected.
			if pod.Aggregator != nil {
				pod.Aggregator.Stop()

				// Retrieve any early output that was buffered before the relay connected.
				// This captures error messages from fast-exiting processes.
				if buf := pod.Aggregator.DrainEarlyBuffer(); len(buf) > 0 {
					earlyOutput = string(buf)
					log.Info("Captured early output from fast-exiting process",
						"pod_key", podKey, "bytes", len(buf))
				}
			}

			pod.DisconnectRelay()
		}

		// Include early output in the termination event so the backend can display
		// why the process failed (e.g., invalid CLI arguments).
		if err := h.conn.SendPodTerminated(podKey, int32(exitCode), earlyOutput); err != nil {
			log.Error("Failed to send pod terminated event", "error", err)
		}
	}
}

// Event sending methods

func (h *RunnerMessageHandler) sendPodCreated(podKey string, pid int, sandboxPath, branchName string, cols, rows uint16) {
	if h.conn == nil {
		return
	}
	if err := h.conn.SendPodCreated(podKey, int32(pid), sandboxPath, branchName); err != nil {
		logger.Pod().Error("Failed to send pod created event", "error", err)
	}
}

func (h *RunnerMessageHandler) sendPodTerminated(podKey string) {
	if h.conn == nil {
		return
	}
	if err := h.conn.SendPodTerminated(podKey, 0, ""); err != nil {
		logger.Pod().Error("Failed to send pod terminated event", "error", err)
	}
}

func (h *RunnerMessageHandler) sendPtyResized(podKey string, cols, rows uint16) {
	if h.conn == nil {
		return
	}
	if err := h.conn.SendPtyResized(podKey, int32(cols), int32(rows)); err != nil {
		logger.Terminal().Error("Failed to send pty resized event", "error", err)
	}
}

func (h *RunnerMessageHandler) sendPodError(podKey, errorMsg string) {
	if h.conn == nil {
		return
	}
	if err := h.conn.SendError(podKey, "error", errorMsg); err != nil {
		logger.Pod().Error("Failed to send error event", "error", err)
	}
}

func (h *RunnerMessageHandler) sendPodErrorWithCode(podKey string, podErr *client.PodError) {
	if h.conn == nil {
		return
	}
	if err := h.conn.SendError(podKey, podErr.Code, podErr.Message); err != nil {
		logger.Pod().Error("Failed to send error event", "error", err)
	}
}
