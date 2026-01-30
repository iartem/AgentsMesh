package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/infra/eventbus"
	"github.com/anthropics/agentsmesh/backend/internal/infra/websocket"
	"github.com/anthropics/agentsmesh/backend/internal/service/relay"
	"github.com/anthropics/agentsmesh/backend/internal/service/runner"
	runnerv1 "github.com/anthropics/agentsmesh/proto/gen/go/runner/v1"
	"gorm.io/gorm"
)

// protoToEventbusThinking converts Proto AutopilotThinkingEvent to eventbus data
func protoToEventbusThinking(data *runnerv1.AutopilotThinkingEvent) *eventbus.AutopilotThinkingData {
	result := &eventbus.AutopilotThinkingData{
		AutopilotControllerKey: data.GetAutopilotKey(),
		Iteration:              data.GetIteration(),
		DecisionType:           data.GetDecisionType(),
		Reasoning:              data.GetReasoning(),
		Confidence:             data.GetConfidence(),
	}

	// Convert action
	if action := data.GetAction(); action != nil {
		result.Action = &eventbus.AutopilotActionData{
			Type:    action.GetType(),
			Content: action.GetContent(),
			Reason:  action.GetReason(),
		}
	}

	// Convert progress
	if progress := data.GetProgress(); progress != nil {
		result.Progress = &eventbus.AutopilotProgressData{
			Summary:        progress.GetSummary(),
			CompletedSteps: progress.GetCompletedSteps(),
			RemainingSteps: progress.GetRemainingSteps(),
			Percent:        progress.GetPercent(),
		}
	}

	// Convert help request
	if helpReq := data.GetHelpRequest(); helpReq != nil {
		result.HelpRequest = &eventbus.AutopilotHelpRequestData{
			Reason:          helpReq.GetReason(),
			Context:         helpReq.GetContext(),
			TerminalExcerpt: helpReq.GetTerminalExcerpt(),
		}
		for _, s := range helpReq.GetSuggestions() {
			result.HelpRequest.Suggestions = append(result.HelpRequest.Suggestions, eventbus.AutopilotHelpSuggestionData{
				Action: s.GetAction(),
				Label:  s.GetLabel(),
			})
		}
	}

	return result
}

// setupEventBusHub sets up the integration between EventBus and WebSocket Hub
// EventBus publishes events, Hub routes them to appropriate WebSocket clients
func setupEventBusHub(eb *eventbus.EventBus, hub *websocket.Hub) {
	// Subscribe to entity events (broadcast to organization)
	eb.SubscribeCategory(eventbus.CategoryEntity, func(event *eventbus.Event) {
		data, err := json.Marshal(event)
		if err != nil {
			slog.Error("failed to marshal event for hub", "error", err, "type", event.Type)
			return
		}
		hub.BroadcastToOrg(event.OrganizationID, data)
	})

	// Subscribe to notification events (targeted to specific users)
	eb.SubscribeCategory(eventbus.CategoryNotification, func(event *eventbus.Event) {
		slog.Info("notification event received",
			"type", event.Type,
			"target_user_id", event.TargetUserID,
			"org_id", event.OrganizationID,
		)

		data, err := json.Marshal(event)
		if err != nil {
			slog.Error("failed to marshal notification event", "error", err, "type", event.Type)
			return
		}

		// Send to specific target user
		if event.TargetUserID != nil {
			slog.Info("sending notification to user",
				"user_id", *event.TargetUserID,
				"type", event.Type,
				"user_client_count", hub.GetUserClientCount(*event.TargetUserID),
			)
			hub.SendToUser(*event.TargetUserID, data)
		}

		// Send to multiple target users
		for _, userID := range event.TargetUserIDs {
			hub.SendToUser(userID, data)
		}
	})

	// Subscribe to system events (broadcast to organization)
	eb.SubscribeCategory(eventbus.CategorySystem, func(event *eventbus.Event) {
		data, err := json.Marshal(event)
		if err != nil {
			slog.Error("failed to marshal system event", "error", err, "type", event.Type)
			return
		}
		hub.BroadcastToOrg(event.OrganizationID, data)
	})
}

// setupRunnerEventCallbacks sets up runner connection manager callbacks to publish events
func setupRunnerEventCallbacks(db *gorm.DB, runnerConnMgr *runner.RunnerConnectionManager, eventBus *eventbus.EventBus) {
	// Wrap heartbeat callback to detect runner coming online (using Proto type)
	originalHeartbeatCallback := runnerConnMgr.GetHeartbeatCallback()
	runnerConnMgr.SetHeartbeatCallback(func(runnerID int64, data *runnerv1.HeartbeatData) {
		// Call original callback first
		if originalHeartbeatCallback != nil {
			originalHeartbeatCallback(runnerID, data)
		}

		// Publish runner online event (heartbeat indicates runner is online)
		// Only publish if this is likely a new connection or status change
		var r struct {
			OrganizationID int64  `gorm:"column:organization_id"`
			NodeID         string `gorm:"column:node_id"`
			Status         string `gorm:"column:status"`
		}
		if err := db.Table("runners").Where("id = ?", runnerID).First(&r).Error; err != nil {
			return // Silently ignore - runner might not exist yet
		}

		// Only publish event if status was offline (changed to online)
		if r.Status != "online" {
			eventData := &eventbus.RunnerStatusData{
				RunnerID:    runnerID,
				NodeID:      r.NodeID,
				Status:      "online",
				CurrentPods: len(data.Pods),
			}
			event, err := eventbus.NewEntityEvent(eventbus.EventRunnerOnline, r.OrganizationID, "runner", fmt.Sprintf("%d", runnerID), eventData)
			if err != nil {
				slog.Error("failed to create runner online event", "error", err)
			} else if err := eventBus.Publish(context.Background(), event); err != nil {
				slog.Error("failed to publish runner online event", "error", err)
			}
		}
	})

	// Wrap disconnect callback to publish runner offline events
	originalDisconnectCallback := runnerConnMgr.GetDisconnectCallback()
	runnerConnMgr.SetDisconnectCallback(func(runnerID int64) {
		// Query runner first before status changes
		var r struct {
			OrganizationID int64  `gorm:"column:organization_id"`
			NodeID         string `gorm:"column:node_id"`
		}
		if err := db.Table("runners").Where("id = ?", runnerID).First(&r).Error; err == nil {
			// Publish runner offline event
			eventData := &eventbus.RunnerStatusData{
				RunnerID: runnerID,
				NodeID:   r.NodeID,
				Status:   "offline",
			}
			event, err := eventbus.NewEntityEvent(eventbus.EventRunnerOffline, r.OrganizationID, "runner", fmt.Sprintf("%d", runnerID), eventData)
			if err != nil {
				slog.Error("failed to create runner offline event", "error", err)
			} else if err := eventBus.Publish(context.Background(), event); err != nil {
				slog.Error("failed to publish runner offline event", "error", err)
			}
		}

		// Call original callback
		if originalDisconnectCallback != nil {
			originalDisconnectCallback(runnerID)
		}
	})
}

// setupPodEventCallbacks sets up pod coordinator callbacks to publish events
func setupPodEventCallbacks(db *gorm.DB, podCoordinator *runner.PodCoordinator, eventBus *eventbus.EventBus) {
	podCoordinator.SetStatusChangeCallback(func(podKey string, status string, agentStatus string) {
		// Query pod to get organization ID and other metadata
		var pod struct {
			OrganizationID int64  `gorm:"column:organization_id"`
			CreatedByID    int64  `gorm:"column:created_by_id"`
			Status         string `gorm:"column:status"`
			AgentStatus    string `gorm:"column:agent_status"`
		}
		if err := db.Table("pods").Where("pod_key = ?", podKey).First(&pod).Error; err != nil {
			slog.Error("failed to get pod for event", "pod_key", podKey, "error", err)
			return
		}

		// Determine event type
		// Note: pod.Status is the previous status (before this callback updates it)
		var eventType eventbus.EventType
		if agentStatus != "" {
			eventType = eventbus.EventPodAgentChanged
		} else if status == "completed" || status == "terminated" {
			eventType = eventbus.EventPodTerminated
		} else if pod.Status == "initializing" && status == "running" {
			// Pod transitioned from initializing to running - this is a newly created pod starting
			eventType = eventbus.EventPodCreated
		} else {
			eventType = eventbus.EventPodStatusChanged
		}

		// Create and publish event
		data := &eventbus.PodStatusChangedData{
			PodKey:         podKey,
			Status:         status,
			PreviousStatus: "",
			AgentStatus:    agentStatus,
		}
		event, err := eventbus.NewEntityEvent(eventType, pod.OrganizationID, "pod", podKey, data)
		if err != nil {
			slog.Error("failed to create pod event", "error", err)
			return
		}
		if err := eventBus.Publish(context.Background(), event); err != nil {
			slog.Error("failed to publish pod event", "error", err)
		}

		// Also publish task:completed notification for finished agents
		if agentStatus == "finished" || agentStatus == "error" {
			notifData := &eventbus.TaskCompletedData{
				PodKey:      podKey,
				AgentStatus: agentStatus,
			}
			notifEvent, err := eventbus.NewNotificationEvent(
				eventbus.EventTaskCompleted,
				pod.OrganizationID,
				&pod.CreatedByID,
				nil,
				"pod",
				podKey,
				notifData,
			)
			if err != nil {
				slog.Error("failed to create task completed event", "error", err)
				return
			}
			if err := eventBus.Publish(context.Background(), notifEvent); err != nil {
				slog.Error("failed to publish task completed event", "error", err)
			}
		}
	})

	// Set up init progress callback
	podCoordinator.SetInitProgressCallback(func(podKey string, phase string, progress int, message string) {
		// Query pod to get organization ID
		var pod struct {
			OrganizationID int64 `gorm:"column:organization_id"`
		}
		if err := db.Table("pods").Where("pod_key = ?", podKey).First(&pod).Error; err != nil {
			slog.Error("failed to get pod for init progress event", "pod_key", podKey, "error", err)
			return
		}

		// Create and publish init progress event
		data := &eventbus.PodInitProgressData{
			PodKey:   podKey,
			Phase:    phase,
			Progress: progress,
			Message:  message,
		}
		event, err := eventbus.NewEntityEvent(eventbus.EventPodInitProgress, pod.OrganizationID, "pod", podKey, data)
		if err != nil {
			slog.Error("failed to create pod init progress event", "error", err)
			return
		}
		if err := eventBus.Publish(context.Background(), event); err != nil {
			slog.Error("failed to publish pod init progress event", "error", err)
		}
	})

	// Set up AutopilotController status change callback
	podCoordinator.SetAutopilotStatusChangeCallback(func(
		autopilotKey string,
		podKey string,
		phase string,
		iteration int32,
		maxIterations int32,
		circuitBreakerState string,
		circuitBreakerReason string,
		userTakeover bool,
	) {
		// Query AutopilotController to get organization ID
		var autopilot struct {
			OrganizationID int64 `gorm:"column:organization_id"`
		}
		if err := db.Table("autopilot_controllers").Where("autopilot_controller_key = ?", autopilotKey).First(&autopilot).Error; err != nil {
			slog.Error("failed to get autopilot for status event", "autopilot_key", autopilotKey, "error", err)
			return
		}

		// Create and publish AutopilotController status event
		data := &eventbus.AutopilotStatusChangedData{
			AutopilotControllerKey: autopilotKey,
			PodKey:                 podKey,
			Phase:                  phase,
			CurrentIteration:       iteration,
			MaxIterations:          maxIterations,
			CircuitBreakerState:    circuitBreakerState,
			CircuitBreakerReason:   circuitBreakerReason,
			UserTakeover:           userTakeover,
		}
		event, err := eventbus.NewEntityEvent(eventbus.EventAutopilotStatusChanged, autopilot.OrganizationID, "autopilot_controller", autopilotKey, data)
		if err != nil {
			slog.Error("failed to create autopilot status event", "error", err)
			return
		}
		if err := eventBus.Publish(context.Background(), event); err != nil {
			slog.Error("failed to publish autopilot status event", "error", err)
		}
	})

	// Set up AutopilotController iteration callback
	podCoordinator.SetAutopilotIterationChangeCallback(func(
		autopilotKey string,
		iteration int32,
		phase string,
		summary string,
		filesChanged []string,
		durationMs int64,
	) {
		// Query AutopilotController to get organization ID
		var autopilot struct {
			OrganizationID int64 `gorm:"column:organization_id"`
		}
		if err := db.Table("autopilot_controllers").Where("autopilot_controller_key = ?", autopilotKey).First(&autopilot).Error; err != nil {
			slog.Error("failed to get autopilot for iteration event", "autopilot_key", autopilotKey, "error", err)
			return
		}

		// Create and publish AutopilotController iteration event
		data := &eventbus.AutopilotIterationData{
			AutopilotControllerKey: autopilotKey,
			Iteration:              iteration,
			Phase:                  phase,
			Summary:                summary,
			FilesChanged:           filesChanged,
			DurationMs:             durationMs,
		}
		event, err := eventbus.NewEntityEvent(eventbus.EventAutopilotIteration, autopilot.OrganizationID, "autopilot_controller", autopilotKey, data)
		if err != nil {
			slog.Error("failed to create autopilot iteration event", "error", err)
			return
		}
		if err := eventBus.Publish(context.Background(), event); err != nil {
			slog.Error("failed to publish autopilot iteration event", "error", err)
		}
	})

	// Set up AutopilotController thinking callback
	podCoordinator.SetAutopilotThinkingCallback(func(runnerID int64, protoData *runnerv1.AutopilotThinkingEvent) {
		autopilotKey := protoData.GetAutopilotKey()

		// Query AutopilotController to get organization ID
		var autopilot struct {
			OrganizationID int64 `gorm:"column:organization_id"`
		}
		if err := db.Table("autopilot_controllers").Where("autopilot_controller_key = ?", autopilotKey).First(&autopilot).Error; err != nil {
			slog.Error("failed to get autopilot for thinking event", "autopilot_key", autopilotKey, "error", err)
			return
		}

		// Convert Proto to eventbus data
		data := protoToEventbusThinking(protoData)

		// Create and publish AutopilotController thinking event
		event, err := eventbus.NewEntityEvent(eventbus.EventAutopilotThinking, autopilot.OrganizationID, "autopilot_controller", autopilotKey, data)
		if err != nil {
			slog.Error("failed to create autopilot thinking event", "error", err)
			return
		}
		if err := eventBus.Publish(context.Background(), event); err != nil {
			slog.Error("failed to publish autopilot thinking event", "error", err)
		}

		slog.Debug("published autopilot thinking event",
			"autopilot_key", autopilotKey,
			"decision_type", data.DecisionType,
			"iteration", data.Iteration)
	})
}

// setupRelayTokenRefreshCallback sets up the callback for relay token refresh requests.
// When a runner's relay token expires during reconnection, it sends a RequestRelayToken event.
// This callback generates a new token and sends a SubscribeTerminal command back to the runner.
func setupRelayTokenRefreshCallback(
	db *gorm.DB,
	runnerConnMgr *runner.RunnerConnectionManager,
	tokenGenerator *relay.TokenGenerator,
	commandSender runner.RunnerCommandSender,
) {
	runnerConnMgr.SetRequestRelayTokenCallback(func(runnerID int64, data *runnerv1.RequestRelayTokenEvent) {
		slog.Info("Received relay token refresh request",
			"runner_id", runnerID,
			"pod_key", data.PodKey,
			"session_id", data.SessionId,
		)

		// Get pod info to find organization ID and verify status
		var pod struct {
			OrganizationID int64  `gorm:"column:organization_id"`
			RunnerID       int64  `gorm:"column:runner_id"`
			Status         string `gorm:"column:status"`
		}
		if err := db.Table("pods").Where("pod_key = ?", data.PodKey).First(&pod).Error; err != nil {
			slog.Error("failed to get pod for relay token refresh",
				"pod_key", data.PodKey,
				"error", err,
			)
			return
		}

		// Verify the runner owns this pod
		if pod.RunnerID != runnerID {
			slog.Warn("runner does not own pod for relay token refresh",
				"runner_id", runnerID,
				"pod_runner_id", pod.RunnerID,
				"pod_key", data.PodKey,
			)
			return
		}

		// Check pod is still active
		if pod.Status != "running" && pod.Status != "initializing" && pod.Status != "disconnected" {
			slog.Warn("pod is not active for relay token refresh",
				"pod_key", data.PodKey,
				"status", pod.Status,
			)
			return
		}

		// Generate a new runner token
		// userID=0 indicates this is a runner token (not a browser token)
		newToken, err := tokenGenerator.GenerateToken(
			data.PodKey,
			data.SessionId,
			runnerID,
			0, // userID=0 for runner token
			pod.OrganizationID,
			time.Hour,
		)
		if err != nil {
			slog.Error("failed to generate new relay token",
				"pod_key", data.PodKey,
				"error", err,
			)
			return
		}

		// Send SubscribeTerminal command with new token back to runner
		if err := commandSender.SendSubscribeTerminal(
			context.Background(),
			runnerID,
			data.PodKey,
			data.RelayUrl,
			data.SessionId,
			newToken,
			true,  // include snapshot (runner will resend after reconnect)
			1000,  // snapshot history lines
		); err != nil {
			slog.Error("failed to send subscribe terminal with new token",
				"runner_id", runnerID,
				"pod_key", data.PodKey,
				"error", err,
			)
			return
		}

		slog.Info("Sent new relay token to runner",
			"runner_id", runnerID,
			"pod_key", data.PodKey,
			"session_id", data.SessionId,
		)
	})
}
