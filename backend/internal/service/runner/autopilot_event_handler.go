package runner

import (
	"context"
	"encoding/json"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/agentpod"
	runnerv1 "github.com/anthropics/agentsmesh/proto/gen/go/runner/v1"
)

// AutopilotController event callbacks

// onAutopilotStatusChange is the callback for AutopilotController status changes to notify realtime
var onAutopilotStatusChange func(
	autopilotControllerKey string,
	podKey string,
	phase string,
	iteration int32,
	maxIterations int32,
	circuitBreakerState string,
	circuitBreakerReason string,
	userTakeover bool,
)

// onAutopilotIteration is the callback for AutopilotController iteration events to notify realtime
var onAutopilotIterationChange func(
	autopilotControllerKey string,
	iteration int32,
	phase string,
	summary string,
	filesChanged []string,
	durationMs int64,
)

// onAutopilotThinkingChange is the callback for AutopilotController thinking events to notify realtime
var onAutopilotThinkingChange func(runnerID int64, data *runnerv1.AutopilotThinkingEvent)

// SetAutopilotStatusChangeCallback sets the callback for AutopilotController status changes
func (pc *PodCoordinator) SetAutopilotStatusChangeCallback(fn func(
	autopilotControllerKey string,
	podKey string,
	phase string,
	iteration int32,
	maxIterations int32,
	circuitBreakerState string,
	circuitBreakerReason string,
	userTakeover bool,
)) {
	onAutopilotStatusChange = fn
}

// SetAutopilotIterationChangeCallback sets the callback for AutopilotController iteration events
func (pc *PodCoordinator) SetAutopilotIterationChangeCallback(fn func(
	autopilotControllerKey string,
	iteration int32,
	phase string,
	summary string,
	filesChanged []string,
	durationMs int64,
)) {
	onAutopilotIterationChange = fn
}

// SetAutopilotThinkingCallback sets the callback for AutopilotController thinking events
func (pc *PodCoordinator) SetAutopilotThinkingCallback(fn func(runnerID int64, data *runnerv1.AutopilotThinkingEvent)) {
	onAutopilotThinkingChange = fn
}

// handleAutopilotControllerStatus handles AutopilotController status events from runner
func (pc *PodCoordinator) handleAutopilotControllerStatus(runnerID int64, data *runnerv1.AutopilotStatusEvent) {
	ctx := context.Background()

	status := data.GetStatus()
	if status == nil {
		pc.logger.Warn("autopilot pod status event missing status",
			"autopilot_controller_key", data.GetAutopilotKey())
		return
	}

	now := time.Now()

	// Determine user_takeover from phase
	userTakeover := status.GetPhase() == agentpod.AutopilotPhaseUserTakeover

	updates := map[string]interface{}{
		"phase":                 status.GetPhase(),
		"current_iteration":     status.GetCurrentIteration(),
		"circuit_breaker_state": status.GetCircuitBreakerState(),
		"user_takeover":         userTakeover,
		"updated_at":            now,
	}

	// Update circuit breaker reason if present
	if reason := status.GetCircuitBreakerReason(); reason != "" {
		updates["circuit_breaker_reason"] = reason
	}

	// Update last_iteration_at if iteration changed
	if status.GetLastIterationAt() > 0 {
		updates["last_iteration_at"] = time.Unix(status.GetLastIterationAt(), 0)
	}

	// Update started_at if provided and not already set
	if status.GetStartedAt() > 0 {
		updates["started_at"] = time.Unix(status.GetStartedAt(), 0)
	}

	// Set completed_at for terminal states
	if status.GetPhase() == agentpod.AutopilotPhaseCompleted ||
		status.GetPhase() == agentpod.AutopilotPhaseFailed ||
		status.GetPhase() == agentpod.AutopilotPhaseStopped {
		updates["completed_at"] = now
	}

	// Set approval_request_at for waiting_approval state
	if status.GetPhase() == agentpod.AutopilotPhaseWaitingApproval {
		updates["approval_request_at"] = now
	}

	if err := pc.db.WithContext(ctx).
		Model(&agentpod.AutopilotController{}).
		Where("autopilot_controller_key = ?", data.GetAutopilotKey()).
		Updates(updates).Error; err != nil {
		pc.logger.Error("failed to update autopilot pod status",
			"autopilot_controller_key", data.GetAutopilotKey(),
			"error", err)
		return
	}

	pc.logger.Info("autopilot pod status updated",
		"autopilot_controller_key", data.GetAutopilotKey(),
		"pod_key", data.GetPodKey(),
		"phase", status.GetPhase(),
		"iteration", status.GetCurrentIteration(),
		"circuit_breaker", status.GetCircuitBreakerState())

	// Notify via callback (to publish realtime event)
	if onAutopilotStatusChange != nil {
		onAutopilotStatusChange(
			data.GetAutopilotKey(),
			data.GetPodKey(),
			status.GetPhase(),
			status.GetCurrentIteration(),
			status.GetMaxIterations(),
			status.GetCircuitBreakerState(),
			status.GetCircuitBreakerReason(),
			userTakeover,
		)
	}
}

// handleAutopilotControllerCreated handles AutopilotController creation confirmation from runner
func (pc *PodCoordinator) handleAutopilotControllerCreated(runnerID int64, data *runnerv1.AutopilotCreatedEvent) {
	ctx := context.Background()

	now := time.Now()
	updates := map[string]interface{}{
		"phase":      agentpod.AutopilotPhaseRunning,
		"started_at": now,
		"updated_at": now,
	}

	if err := pc.db.WithContext(ctx).
		Model(&agentpod.AutopilotController{}).
		Where("autopilot_controller_key = ?", data.GetAutopilotKey()).
		Updates(updates).Error; err != nil {
		pc.logger.Error("failed to update autopilot pod on creation",
			"autopilot_controller_key", data.GetAutopilotKey(),
			"error", err)
		return
	}

	pc.logger.Info("autopilot pod created",
		"autopilot_controller_key", data.GetAutopilotKey(),
		"pod_key", data.GetPodKey(),
		"runner_id", runnerID)

	// Notify via callback
	if onAutopilotStatusChange != nil {
		// Fetch the full AutopilotController to get max_iterations
		var rp agentpod.AutopilotController
		if err := pc.db.WithContext(ctx).
			Where("autopilot_controller_key = ?", data.GetAutopilotKey()).
			First(&rp).Error; err == nil {
			onAutopilotStatusChange(
				data.GetAutopilotKey(),
				data.GetPodKey(),
				agentpod.AutopilotPhaseRunning,
				0,
				rp.MaxIterations,
				agentpod.CircuitBreakerClosed,
				"",
				false,
			)
		}
	}
}

// handleAutopilotControllerTerminated handles AutopilotController termination event from runner
func (pc *PodCoordinator) handleAutopilotControllerTerminated(runnerID int64, data *runnerv1.AutopilotTerminatedEvent) {
	ctx := context.Background()

	now := time.Now()
	phase := agentpod.AutopilotPhaseStopped
	if reason := data.GetReason(); reason != "" {
		if reason == "completed" {
			phase = agentpod.AutopilotPhaseCompleted
		} else if reason == "failed" {
			phase = agentpod.AutopilotPhaseFailed
		}
	}

	updates := map[string]interface{}{
		"phase":        phase,
		"completed_at": now,
		"updated_at":   now,
	}

	if err := pc.db.WithContext(ctx).
		Model(&agentpod.AutopilotController{}).
		Where("autopilot_controller_key = ?", data.GetAutopilotKey()).
		Updates(updates).Error; err != nil {
		pc.logger.Error("failed to update autopilot pod on termination",
			"autopilot_controller_key", data.GetAutopilotKey(),
			"error", err)
		return
	}

	pc.logger.Info("autopilot pod terminated",
		"autopilot_controller_key", data.GetAutopilotKey(),
		"runner_id", runnerID,
		"reason", data.GetReason())

	// Notify via callback
	if onAutopilotStatusChange != nil {
		// Fetch the full AutopilotController to get details
		var rp agentpod.AutopilotController
		if err := pc.db.WithContext(ctx).
			Where("autopilot_controller_key = ?", data.GetAutopilotKey()).
			First(&rp).Error; err == nil {
			onAutopilotStatusChange(
				data.GetAutopilotKey(),
				rp.PodKey,
				phase,
				rp.CurrentIteration,
				rp.MaxIterations,
				rp.CircuitBreakerState,
				"",
				rp.UserTakeover,
			)
		}
	}
}

// handleAutopilotIteration handles AutopilotController iteration events from runner
func (pc *PodCoordinator) handleAutopilotIteration(runnerID int64, data *runnerv1.AutopilotIterationEvent) {
	ctx := context.Background()

	// Get AutopilotController ID
	var rp agentpod.AutopilotController
	if err := pc.db.WithContext(ctx).
		Where("autopilot_controller_key = ?", data.GetAutopilotKey()).
		First(&rp).Error; err != nil {
		pc.logger.Error("failed to find autopilot pod for iteration",
			"autopilot_controller_key", data.GetAutopilotKey(),
			"error", err)
		return
	}

	// Serialize files changed as JSON
	var filesChangedJSON *string
	if len(data.GetFilesChanged()) > 0 {
		if jsonBytes, err := json.Marshal(data.GetFilesChanged()); err == nil {
			s := string(jsonBytes)
			filesChangedJSON = &s
		}
	}

	// Create iteration record
	var summary *string
	if s := data.GetSummary(); s != "" {
		summary = &s
	}

	iteration := &agentpod.AutopilotIteration{
		AutopilotControllerID:   rp.ID,
		Iteration:    data.GetIteration(),
		Phase:        data.GetPhase(),
		Summary:      summary,
		FilesChanged: filesChangedJSON,
		DurationMs:   data.GetDurationMs(),
	}

	if err := pc.db.WithContext(ctx).Create(iteration).Error; err != nil {
		pc.logger.Error("failed to create autopilot iteration record",
			"autopilot_controller_key", data.GetAutopilotKey(),
			"iteration", data.GetIteration(),
			"error", err)
		return
	}

	// Update AutopilotController current_iteration and last_iteration_at
	now := time.Now()
	if err := pc.db.WithContext(ctx).
		Model(&agentpod.AutopilotController{}).
		Where("autopilot_controller_key = ?", data.GetAutopilotKey()).
		Updates(map[string]interface{}{
			"current_iteration": data.GetIteration(),
			"last_iteration_at": now,
			"updated_at":        now,
		}).Error; err != nil {
		pc.logger.Error("failed to update autopilot pod iteration count",
			"autopilot_controller_key", data.GetAutopilotKey(),
			"error", err)
	}

	pc.logger.Debug("autopilot iteration recorded",
		"autopilot_controller_key", data.GetAutopilotKey(),
		"iteration", data.GetIteration(),
		"phase", data.GetPhase(),
		"summary", data.GetSummary())

	// Notify via callback (to publish realtime event)
	if onAutopilotIterationChange != nil {
		onAutopilotIterationChange(
			data.GetAutopilotKey(),
			data.GetIteration(),
			data.GetPhase(),
			data.GetSummary(),
			data.GetFilesChanged(),
			data.GetDurationMs(),
		)
	}

	// Also send status_changed event to update current_iteration in frontend
	// The frontend relies on autopilot:status_changed to update the iteration counter display
	if onAutopilotStatusChange != nil {
		onAutopilotStatusChange(
			data.GetAutopilotKey(),
			rp.PodKey,
			rp.Phase, // Keep current phase
			data.GetIteration(),
			rp.MaxIterations,
			rp.CircuitBreakerState,
			"",
			rp.UserTakeover,
		)
	}
}

// handleAutopilotThinking handles AutopilotController thinking events from runner
// This event exposes the Control Agent's decision-making process to the user
func (pc *PodCoordinator) handleAutopilotThinking(runnerID int64, data *runnerv1.AutopilotThinkingEvent) {
	pc.logger.Debug("autopilot thinking received",
		"autopilot_controller_key", data.GetAutopilotKey(),
		"iteration", data.GetIteration(),
		"decision_type", data.GetDecisionType(),
		"reasoning", data.GetReasoning())

	// Notify via callback (to publish realtime event)
	if onAutopilotThinkingChange != nil {
		onAutopilotThinkingChange(runnerID, data)
	}
}
