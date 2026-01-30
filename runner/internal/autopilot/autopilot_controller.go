// Package autopilot implements the AutopilotController for supervised Pod automation.
// AutopilotController orchestrates Pod execution by detecting when the controlled pod
// is waiting for input and automatically providing the next instruction.
package autopilot

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/anthropics/agentsmesh/runner/internal/logger"
	runnerv1 "github.com/anthropics/agentsmesh/proto/gen/go/runner/v1"
)

// Phase represents the current phase of an AutopilotController.
type Phase string

const (
	PhaseInitializing    Phase = "initializing"
	PhaseRunning         Phase = "running"
	PhasePaused          Phase = "paused"
	PhaseUserTakeover    Phase = "user_takeover"
	PhaseWaitingApproval Phase = "waiting_approval"
	PhaseCompleted       Phase = "completed"
	PhaseFailed          Phase = "failed"
	PhaseStopped         Phase = "stopped"
	PhaseMaxIterations   Phase = "max_iterations"
)

// Status represents the current status of an AutopilotController.
type Status struct {
	Phase            Phase
	CurrentIteration int
	MaxIterations    int
	PodStatus        string
	StartedAt        time.Time
	LastIterationAt  time.Time
	LastDecision     string // Last Control decision type
	LastDecisionMsg  string // Last Control decision message
}

// EventReporter is the interface for reporting Autopilot events.
type EventReporter interface {
	ReportAutopilotStatus(event *runnerv1.AutopilotStatusEvent)
	ReportAutopilotIteration(event *runnerv1.AutopilotIterationEvent)
	ReportAutopilotCreated(event *runnerv1.AutopilotCreatedEvent)
	ReportAutopilotTerminated(event *runnerv1.AutopilotTerminatedEvent)
	ReportAutopilotThinking(event *runnerv1.AutopilotThinkingEvent)
}

// PodController provides methods to interact with the controlled Pod.
type PodController interface {
	// SendTerminalText sends text to the pod's terminal.
	SendTerminalText(text string) error
	// GetWorkDir returns the pod's working directory.
	GetWorkDir() string
	// GetPodKey returns the pod's key.
	GetPodKey() string
	// GetAgentStatus returns the pod's agent status (executing/waiting/not_running).
	GetAgentStatus() string
	// GetStateDetector returns a StateDetector for the pod.
	// Returns nil if state detection is not available.
	GetStateDetector() StateDetector
}

// AutopilotController is a supervised automation controller that orchestrates
// a Pod to complete tasks autonomously.
//
// This is a thin coordinator that delegates to specialized components:
// - PhaseManager: lifecycle phase transitions
// - IterationController: iteration counting and max iteration protection
// - UserInteractionHandler: takeover/handback/approve handling
// - StateDetectorCoordinator: terminal state detection
// - ControlRunner: control process execution
// - ProgressTracker: file/git change tracking
type AutopilotController struct {
	key    string
	podKey string
	config *runnerv1.AutopilotConfig

	// Pod controller
	podCtrl PodController

	// MCP port for control process to connect to
	mcpPort int

	// MCP config file path for control process
	mcpConfigPath string

	// Component delegates (SRP compliance)
	phaseMgr         *PhaseManager
	iterCtrl         *IterationController
	userHandler      *UserInteractionHandler
	stateCoordinator *StateDetectorCoordinator
	controlRunner    *ControlRunner
	promptBuilder    *PromptBuilder
	progressTracker  *ProgressTracker

	// Status mutex for LastDecision fields
	decisionMu      sync.RWMutex
	lastDecision    string
	lastDecisionMsg string

	// Lifecycle
	ctx    context.Context
	cancel context.CancelFunc

	// Event reporting
	reporter EventReporter

	// Logger
	log *slog.Logger
}

// Config contains configuration for creating an AutopilotController.
type Config struct {
	AutopilotKey string
	PodKey       string
	ProtoConfig  *runnerv1.AutopilotConfig
	PodCtrl      PodController
	Reporter     EventReporter
	MCPPort      int // MCP HTTP Server port for control process
}

// NewAutopilotController creates a new AutopilotController instance.
func NewAutopilotController(cfg Config) *AutopilotController {
	ctx, cancel := context.WithCancel(context.Background())
	log := logger.Autopilot()

	mcpPort := cfg.MCPPort
	if mcpPort == 0 {
		mcpPort = DefaultMCPPort
	}

	// Create MCP config file for Control Agent
	mcpConfigPath, err := createMCPConfigFile(cfg.PodCtrl.GetWorkDir(), cfg.PodKey, mcpPort)
	if err != nil {
		log.Warn("Failed to create MCP config file, Control will use curl fallback",
			"error", err)
	}

	ac := &AutopilotController{
		key:           cfg.AutopilotKey,
		podKey:        cfg.PodKey,
		config:        cfg.ProtoConfig,
		podCtrl:       cfg.PodCtrl,
		mcpPort:       mcpPort,
		mcpConfigPath: mcpConfigPath,
		ctx:           ctx,
		cancel:        cancel,
		reporter:      cfg.Reporter,
		log:           log,
	}

	// Initialize IterationController
	ac.iterCtrl = NewIterationController(IterationControllerConfig{
		MaxIterations: int(cfg.ProtoConfig.MaxIterations),
		MinTriggerGap: 5 * time.Second,
		Reporter:      cfg.Reporter,
		AutopilotKey:  cfg.AutopilotKey,
		PodKey:        cfg.PodKey,
		Logger:        log,
	})

	// Initialize PhaseManager with status getter callback
	ac.phaseMgr = NewPhaseManager(PhaseManagerConfig{
		AutopilotKey: cfg.AutopilotKey,
		PodKey:       cfg.PodKey,
		Reporter:     cfg.Reporter,
		StatusGetter: ac.buildAutopilotStatus,
	})

	// Initialize UserInteractionHandler
	// Note: OnResumeCallback set after creation due to circular reference
	ac.userHandler = NewUserInteractionHandler(UserInteractionConfig{
		PhaseManager:        ac.phaseMgr,
		IterationController: ac.iterCtrl,
		Logger:              log,
		OnResumeCallback:    nil, // Set below
	})

	// Initialize PromptBuilder
	ac.promptBuilder = NewPromptBuilder(PromptBuilderConfig{
		InitialPrompt:       cfg.ProtoConfig.InitialPrompt,
		CustomTemplate:      cfg.ProtoConfig.ControlPromptTemplate,
		MCPPort:             mcpPort,
		PodKey:              cfg.PodKey,
		GetMaxIterations:    ac.iterCtrl.GetMaxIterations,
		GetCurrentIteration: ac.iterCtrl.GetCurrentIteration,
	})

	// Initialize ControlRunner
	ac.controlRunner = NewControlRunner(ControlRunnerConfig{
		WorkDir:        cfg.PodCtrl.GetWorkDir(),
		AgentType:      cfg.ProtoConfig.ControlAgentType,
		MCPConfigPath:  mcpConfigPath,
		PromptBuilder:  ac.promptBuilder,
		DecisionParser: NewDecisionParser(),
		Logger:         log,
	})

	// Initialize ProgressTracker
	ac.progressTracker = NewProgressTracker(ProgressTrackerConfig{
		WorkDir: cfg.PodCtrl.GetWorkDir(),
		Logger:  log,
	})

	// Initialize StateDetectorCoordinator
	var detector StateDetector
	if cfg.PodCtrl != nil {
		detector = cfg.PodCtrl.GetStateDetector()
	}
	ac.stateCoordinator = NewStateDetectorCoordinator(StateDetectorCoordinatorConfig{
		Detector:     detector,
		OnWaiting:    ac.OnPodWaiting,
		CheckPeriod:  500 * time.Millisecond,
		Logger:       log,
		AutopilotKey: cfg.AutopilotKey,
	})

	// Start state detection
	ac.stateCoordinator.Start()

	// Set up resume callback now that all components are initialized
	ac.userHandler.SetOnResumeCallback(ac.onResumeFromUserInteraction)

	return ac
}

// onResumeFromUserInteraction is called after user handback or approve.
// It checks if the Pod is waiting and triggers an iteration if needed.
func (ac *AutopilotController) onResumeFromUserInteraction() {
	// Small delay to allow state to stabilize, with context cancellation check
	select {
	case <-time.After(DefaultResumeDelay):
		// Continue after delay
	case <-ac.ctx.Done():
		// Controller stopped during delay, abort
		return
	}

	// Check if Pod is waiting for input
	if ac.podCtrl != nil && ac.podCtrl.GetAgentStatus() == "waiting" {
		ac.log.Info("Pod is waiting after resume, triggering iteration",
			"autopilot_key", ac.key)
		ac.OnPodWaiting()
	}
}

// Key returns the AutopilotController's key.
func (ac *AutopilotController) Key() string {
	return ac.key
}

// PodKey returns the associated Pod's key.
func (ac *AutopilotController) PodKey() string {
	return ac.podKey
}

// GetStatus returns a copy of the current status.
func (ac *AutopilotController) GetStatus() Status {
	ac.decisionMu.RLock()
	lastDecision := ac.lastDecision
	lastDecisionMsg := ac.lastDecisionMsg
	ac.decisionMu.RUnlock()

	return Status{
		Phase:            ac.phaseMgr.GetPhase(),
		CurrentIteration: ac.iterCtrl.GetCurrentIteration(),
		MaxIterations:    ac.iterCtrl.GetMaxIterations(),
		PodStatus:        ac.podCtrl.GetAgentStatus(),
		StartedAt:        ac.iterCtrl.GetStartedAt(),
		LastIterationAt:  ac.iterCtrl.GetLastIterationAt(),
		LastDecision:     lastDecision,
		LastDecisionMsg:  lastDecisionMsg,
	}
}

// Start initializes and starts the AutopilotController.
// It checks if the Pod is waiting and sends the initial prompt if so.
func (ac *AutopilotController) Start() error {
	ac.log.Info("Starting AutopilotController", "autopilot_key", ac.key, "pod_key", ac.podKey)

	// Report created event
	if ac.reporter != nil {
		ac.reporter.ReportAutopilotCreated(&runnerv1.AutopilotCreatedEvent{
			AutopilotKey: ac.key,
			PodKey:       ac.podKey,
		})
	}

	// Check Pod current status
	agentStatus := ac.podCtrl.GetAgentStatus()
	ac.log.Info("Pod current status", "status", agentStatus)

	if agentStatus == "waiting" {
		// Pod is waiting for input, send initial_prompt
		ac.sendInitialPrompt()
	}
	// If executing, we'll wait for the next waiting event

	ac.phaseMgr.SetPhase(PhaseRunning)
	return nil
}

// Stop stops the AutopilotController.
func (ac *AutopilotController) Stop() {
	if !ac.phaseMgr.SetPhaseWithoutReport(PhaseStopped) {
		// Already stopped
		return
	}

	ac.stateCoordinator.Stop()
	ac.cancel()

	// Cleanup MCP config file
	if ac.mcpConfigPath != "" {
		if err := os.Remove(ac.mcpConfigPath); err != nil && !os.IsNotExist(err) {
			ac.log.Warn("Failed to cleanup MCP config file",
				"path", ac.mcpConfigPath,
				"error", err)
		}
	}

	ac.log.Info("AutopilotController stopped", "autopilot_key", ac.key)

	if ac.reporter != nil {
		ac.reporter.ReportAutopilotTerminated(&runnerv1.AutopilotTerminatedEvent{
			AutopilotKey: ac.key,
			Reason:       "stopped",
		})
	}
}

// Pause pauses the AutopilotController.
func (ac *AutopilotController) Pause() {
	if ac.phaseMgr.GetPhase() == PhaseRunning {
		ac.phaseMgr.SetPhase(PhasePaused)
		ac.log.Info("AutopilotController paused", "autopilot_key", ac.key)
	}
}

// Resume resumes a paused AutopilotController.
func (ac *AutopilotController) Resume() {
	if ac.phaseMgr.GetPhase() == PhasePaused {
		ac.phaseMgr.SetPhase(PhaseRunning)
		ac.log.Info("AutopilotController resumed", "autopilot_key", ac.key)
	}
}

// Takeover allows the user to take control.
func (ac *AutopilotController) Takeover() {
	ac.userHandler.Takeover()
	ac.log.Info("User takeover", "autopilot_key", ac.key)
}

// Handback returns control to AutopilotController.
func (ac *AutopilotController) Handback() {
	ac.userHandler.Handback()
	ac.log.Info("User handback", "autopilot_key", ac.key)
}

// Approve handles approval when Control requests human help (NEED_HUMAN_HELP).
func (ac *AutopilotController) Approve(continueExecution bool, additionalIterations int32) {
	ac.userHandler.Approve(continueExecution, additionalIterations)
}

// OnPodWaiting is called when the Pod transitions to waiting state.
// This is the main event-driven entry point triggered by StateDetectorCoordinator.
// Includes deduplication to prevent rapid re-triggering.
func (ac *AutopilotController) OnPodWaiting() {
	// Check trigger deduplication
	if !ac.iterCtrl.CheckTriggerDedup() {
		return
	}

	// Check if user has taken over
	if ac.userHandler.IsUserTakeover() {
		ac.log.Debug("Skipping iteration - user takeover", "autopilot_key", ac.key)
		return
	}

	// Check if phase allows iteration
	if !ac.phaseMgr.CanProcessIteration() {
		return
	}

	// Check max iterations (the only hard protection)
	if ac.iterCtrl.HasReachedMaxIterations() {
		ac.phaseMgr.SetPhase(PhaseMaxIterations)
		ac.log.Info("Max iterations reached", "autopilot_key", ac.key)
		if ac.reporter != nil {
			ac.reporter.ReportAutopilotTerminated(&runnerv1.AutopilotTerminatedEvent{
				AutopilotKey: ac.key,
				Reason:       "max_iterations",
			})
		}
		return
	}

	// Increment iteration
	iteration, ok := ac.iterCtrl.IncrementIteration()
	if !ok {
		// Max iterations reached during increment
		ac.phaseMgr.SetPhase(PhaseMaxIterations)
		ac.log.Info("Max iterations reached", "autopilot_key", ac.key)
		if ac.reporter != nil {
			ac.reporter.ReportAutopilotTerminated(&runnerv1.AutopilotTerminatedEvent{
				AutopilotKey: ac.key,
				Reason:       "max_iterations",
			})
		}
		return
	}

	// Report iteration started
	ac.iterCtrl.ReportIterationEvent(iteration, "started", "", nil)

	// Run single decision in a goroutine
	go ac.runSingleDecision(iteration)
}

// sendInitialPrompt starts the first iteration when Pod is waiting.
// This launches the control process which will use MCP tools to interact with Pod.
func (ac *AutopilotController) sendInitialPrompt() {
	ac.log.Info("Starting initial iteration", "autopilot_key", ac.key)

	// Update trigger time to prevent OnPodWaiting from double-triggering
	ac.iterCtrl.UpdateTriggerTime()

	// Set initial iteration
	iteration := ac.iterCtrl.SetInitialIteration()

	// Report iteration started
	ac.iterCtrl.ReportIterationEvent(iteration, "started", "", nil)

	// Run the control process
	go ac.runSingleDecision(iteration)
}

// runSingleDecision executes a single decision cycle using the control process.
// On error, it retries internally without consuming additional iteration quota.
func (ac *AutopilotController) runSingleDecision(iteration int) {
	ac.log.Info("Running single decision", "autopilot_key", ac.key, "iteration", iteration)

	// Internal retry loop - errors don't consume iteration quota
	for {
		startTime := time.Now()

		// Create timeout context
		timeout := time.Duration(ac.config.IterationTimeoutSeconds) * time.Second
		if timeout == 0 {
			timeout = DefaultIterationTimeout
		}
		ctx, cancel := context.WithTimeout(ac.ctx, timeout)

		// Run control process
		decision, err := ac.controlRunner.RunControlProcess(ctx, iteration)
		duration := time.Since(startTime)
		cancel()

		if err != nil {
			// Control execution failure - log and report
			ac.log.Error("Control process failed", "error", err, "iteration", iteration)
			ac.iterCtrl.ReportIterationEvent(iteration, "error", err.Error(), nil)

			// Record error and check if max consecutive errors exceeded
			if ac.iterCtrl.RecordError() {
				ac.log.Error("Max consecutive errors exceeded, stopping autopilot",
					"autopilot_key", ac.key,
					"consecutive_errors", ac.iterCtrl.GetConsecutiveErrors())
				ac.phaseMgr.SetPhase(PhaseFailed)
				if ac.reporter != nil {
					ac.reporter.ReportAutopilotTerminated(&runnerv1.AutopilotTerminatedEvent{
						AutopilotKey: ac.key,
						Reason:       "max_consecutive_errors",
					})
				}
				return
			}

			// Check if Pod is still waiting and we should retry
			if ac.podCtrl == nil || ac.podCtrl.GetAgentStatus() != "waiting" {
				ac.log.Info("Pod not waiting, skipping retry", "autopilot_key", ac.key)
				return
			}

			// Retry with exponential backoff (doesn't consume iteration quota)
			retryDelay := ac.iterCtrl.GetRetryDelay()
			ac.log.Info("Retrying control process with backoff (same iteration)",
				"autopilot_key", ac.key,
				"iteration", iteration,
				"retry_delay", retryDelay,
				"consecutive_errors", ac.iterCtrl.GetConsecutiveErrors())

			// Wait with context cancellation check
			select {
			case <-time.After(retryDelay):
				continue // Retry the loop
			case <-ac.ctx.Done():
				return // Controller stopped
			}
		}

		// Success - reset consecutive errors
		ac.iterCtrl.ResetErrors()

		// Process successful decision
		ac.processSuccessfulDecision(decision, iteration, duration)
		return
	}
}

// processSuccessfulDecision handles the aftermath of a successful control process run.
func (ac *AutopilotController) processSuccessfulDecision(decision *ControlDecision, iteration int, duration time.Duration) {
	// Capture progress snapshot AFTER iteration to detect actual changes
	if ac.progressTracker != nil {
		snapshot := ac.progressTracker.CaptureSnapshot()
		ac.log.Debug("Progress snapshot captured after iteration",
			"iteration", iteration,
			"files_changed", len(snapshot.FilesModified),
			"has_changes", snapshot.GitDiff.HasChanges)

		// Use detected file changes if decision doesn't provide them
		if len(decision.FilesChanged) == 0 && len(snapshot.FilesModified) > 0 {
			decision.FilesChanged = snapshot.FilesModified
		}
	}

	// Update last decision
	ac.decisionMu.Lock()
	ac.lastDecision = string(decision.Type)
	ac.lastDecisionMsg = decision.Summary
	ac.decisionMu.Unlock()

	// Handle Control decision
	ac.handleDecision(decision, iteration, duration)
}

// handleDecision processes a Control decision and updates state accordingly.
func (ac *AutopilotController) handleDecision(decision *ControlDecision, iteration int, duration time.Duration) {
	// Report thinking event to expose Control Agent's decision process
	ac.reportThinkingEvent(decision, iteration)

	switch decision.Type {
	case DecisionCompleted:
		ac.phaseMgr.SetPhase(PhaseCompleted)
		ac.log.Info("Task completed", "autopilot_key", ac.key)
		ac.iterCtrl.ReportIterationEvent(iteration, "completed", decision.Summary, decision.FilesChanged)
		if ac.reporter != nil {
			ac.reporter.ReportAutopilotTerminated(&runnerv1.AutopilotTerminatedEvent{
				AutopilotKey: ac.key,
				Reason:       "completed",
			})
		}

	case DecisionNeedHumanHelp:
		ac.phaseMgr.SetPhase(PhaseWaitingApproval)
		ac.log.Warn("Control requests human help",
			"autopilot_key", ac.key,
			"reason", decision.Summary)
		ac.iterCtrl.ReportIterationEvent(iteration, "need_human_help", decision.Summary, nil)

	case DecisionGiveUp:
		ac.phaseMgr.SetPhase(PhaseFailed)
		ac.log.Warn("Control gave up",
			"autopilot_key", ac.key,
			"reason", decision.Summary)
		ac.iterCtrl.ReportIterationEvent(iteration, "give_up", decision.Summary, nil)
		if ac.reporter != nil {
			ac.reporter.ReportAutopilotTerminated(&runnerv1.AutopilotTerminatedEvent{
				AutopilotKey: ac.key,
				Reason:       "failed",
			})
		}

	case DecisionContinue:
		// Normal case - waiting for next Pod waiting state
		ac.iterCtrl.ReportIterationEvent(iteration, "action_sent", decision.Summary, decision.FilesChanged)
		ac.log.Info("Decision completed",
			"autopilot_key", ac.key,
			"iteration", iteration,
			"duration_ms", duration.Milliseconds(),
			"files_changed", len(decision.FilesChanged))
		// StateDetector will trigger next iteration when Pod is ready
	}
}

// reportThinkingEvent sends an AutopilotThinkingEvent to expose the Control Agent's decision process.
func (ac *AutopilotController) reportThinkingEvent(decision *ControlDecision, iteration int) {
	if ac.reporter == nil {
		return
	}

	event := &runnerv1.AutopilotThinkingEvent{
		AutopilotKey: ac.key,
		Iteration:    int32(iteration),
		DecisionType: string(decision.Type),
		Reasoning:    decision.Reasoning,
		Confidence:   decision.Confidence,
	}

	// Add action if present
	if decision.Action != nil {
		event.Action = &runnerv1.AutopilotAction{
			Type:    decision.Action.Type,
			Content: decision.Action.Content,
			Reason:  decision.Action.Reason,
		}
	}

	// Add progress if present
	if decision.Progress != nil {
		event.Progress = &runnerv1.AutopilotProgress{
			Summary:        decision.Progress.Summary,
			CompletedSteps: decision.Progress.CompletedSteps,
			RemainingSteps: decision.Progress.RemainingSteps,
			Percent:        int32(decision.Progress.Percent),
		}
	}

	// Add help request if present
	if decision.HelpRequest != nil {
		event.HelpRequest = &runnerv1.AutopilotHelpRequest{
			Reason:          decision.HelpRequest.Reason,
			Context:         decision.HelpRequest.Context,
			TerminalExcerpt: decision.HelpRequest.TerminalExcerpt,
		}
		for _, s := range decision.HelpRequest.Suggestions {
			event.HelpRequest.Suggestions = append(event.HelpRequest.Suggestions, &runnerv1.AutopilotHelpSuggestion{
				Action: s.Action,
				Label:  s.Label,
			})
		}
	}

	ac.reporter.ReportAutopilotThinking(event)
}

// buildAutopilotStatus builds an AutopilotStatus proto from current state.
// Used as a callback by PhaseManager for status reporting.
func (ac *AutopilotController) buildAutopilotStatus() *runnerv1.AutopilotStatus {
	status := ac.iterCtrl.GetStatus()
	status.Phase = string(ac.phaseMgr.GetPhase())
	status.PodStatus = ac.podCtrl.GetAgentStatus()
	return status
}

// =============================================================================
// Test Helper Methods (unexported)
// These methods provide access to internal components for testing purposes.
// They delegate to the appropriate component and are not part of the public API.
// =============================================================================

func (ac *AutopilotController) extractSessionID(output string) {
	if sessionID := ExtractSessionID(output); sessionID != "" {
		ac.controlRunner.SetSessionID(sessionID)
	}
}

func (ac *AutopilotController) getSessionID() string {
	return ac.controlRunner.GetSessionID()
}

func (ac *AutopilotController) buildInitialPrompt() string {
	return ac.promptBuilder.BuildInitialPrompt()
}

func (ac *AutopilotController) buildResumePrompt(iteration int) string {
	return ac.promptBuilder.BuildResumePrompt(iteration)
}

func (ac *AutopilotController) parseDecision(output string) *ControlDecision {
	return NewDecisionParser().ParseDecision(output)
}

func (ac *AutopilotController) setPhaseForTest(phase Phase) {
	ac.phaseMgr.SetPhaseWithoutReport(phase)
}

func (ac *AutopilotController) setIterationForTest(iteration int) {
	ac.iterCtrl.mu.Lock()
	ac.iterCtrl.currentIter = iteration
	ac.iterCtrl.mu.Unlock()
}

// GetProgressSummary returns a human-readable summary of task progress.
func (ac *AutopilotController) GetProgressSummary() string {
	if ac.progressTracker == nil {
		return "No progress tracking available"
	}
	return ac.progressTracker.GenerateSummary()
}

// IsStuck checks if no progress has been made for the specified duration.
func (ac *AutopilotController) IsStuck(threshold time.Duration) bool {
	if ac.progressTracker == nil {
		return false
	}
	return ac.progressTracker.IsStuck(threshold)
}

// GetChangedFiles returns files changed during the autopilot session.
func (ac *AutopilotController) GetChangedFiles() []string {
	if ac.progressTracker == nil {
		return nil
	}
	startedAt := ac.iterCtrl.GetStartedAt()
	return ac.progressTracker.GetChangedFilesSince(startedAt)
}

// createMCPConfigFile creates an MCP configuration file for the Control Agent.
// This allows the Control Agent to use MCP tools directly instead of curl.
// Returns the path to the created config file, or empty string on error.
func createMCPConfigFile(workDir, podKey string, mcpPort int) (string, error) {
	// Create .mcp.json in the working directory
	configPath := filepath.Join(workDir, ".mcp.json")

	// MCP config structure for Claude Code
	// Using HTTP transport to connect to our local MCP server
	config := map[string]interface{}{
		"mcpServers": map[string]interface{}{
			"autopilot-control": map[string]interface{}{
				"type": "http",
				"url":  fmt.Sprintf("http://127.0.0.1:%d/mcp", mcpPort),
				"headers": map[string]string{
					"Content-Type": "application/json",
					"X-Pod-Key":    podKey,
				},
			},
		},
	}

	// Write config file
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}

	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return "", err
	}

	return configPath, nil
}
