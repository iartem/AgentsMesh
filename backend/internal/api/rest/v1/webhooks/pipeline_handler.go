package webhooks

import (
	"fmt"
	"log/slog"
)

// Pipeline status constants
const (
	PipelineStatusPending  = "pending"
	PipelineStatusRunning  = "running"
	PipelineStatusSuccess  = "success"
	PipelineStatusFailed   = "failed"
	PipelineStatusCanceled = "canceled"
	PipelineStatusSkipped  = "skipped"
	PipelineStatusManual   = "manual"
)

// PipelineHandler handles pipeline webhook events
type PipelineHandler struct {
	logger *slog.Logger
}

// NewPipelineHandler creates a new pipeline handler
func NewPipelineHandler(logger *slog.Logger) *PipelineHandler {
	return &PipelineHandler{logger: logger}
}

// CanHandle checks if this is a pipeline event we should process
func (h *PipelineHandler) CanHandle(ctx *WebhookContext) bool {
	// Only handle pipeline events with valid IDs
	return ctx.ObjectKind == "pipeline" && ctx.PipelineID > 0
}

// Handle processes the pipeline event
func (h *PipelineHandler) Handle(ctx *WebhookContext) (map[string]interface{}, error) {
	h.logger.Info("processing pipeline event",
		"project_id", ctx.ProjectID,
		"pipeline_id", ctx.PipelineID,
		"status", ctx.PipelineStatus)

	// Extract pipeline URL
	var pipelineURL string
	if objAttrs, ok := ctx.Payload["object_attributes"].(map[string]interface{}); ok {
		if url, ok := objAttrs["url"].(string); ok {
			pipelineURL = url
		}
	}

	result := map[string]interface{}{
		"status":          "ok",
		"pipeline_id":     ctx.PipelineID,
		"pipeline_status": ctx.PipelineStatus,
		"pipeline_url":    pipelineURL,
	}

	// Here you would typically:
	// 1. Update PipelineWatcher in Redis
	// 2. Update related TaskExecution status
	// 3. Notify interested parties via WebSocket

	return result, nil
}

// MergeRequestHandler handles merge request webhook events
type MergeRequestHandler struct {
	logger *slog.Logger
}

// NewMergeRequestHandler creates a new merge request handler
func NewMergeRequestHandler(logger *slog.Logger) *MergeRequestHandler {
	return &MergeRequestHandler{logger: logger}
}

// CanHandle checks if this is an MR event we should process
func (h *MergeRequestHandler) CanHandle(ctx *WebhookContext) bool {
	if ctx.ObjectKind != "merge_request" {
		return false
	}

	// Check if there's a source branch
	if objAttrs, ok := ctx.Payload["object_attributes"].(map[string]interface{}); ok {
		if sourceBranch, ok := objAttrs["source_branch"].(string); ok && sourceBranch != "" {
			return true
		}
	}

	return false
}

// Handle processes the merge request event
func (h *MergeRequestHandler) Handle(ctx *WebhookContext) (map[string]interface{}, error) {
	objAttrs, ok := ctx.Payload["object_attributes"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("missing object_attributes in MR webhook")
	}

	sourceBranch := objAttrs["source_branch"].(string)
	action := ""
	if a, ok := objAttrs["action"].(string); ok {
		action = a
	}

	h.logger.Info("processing MR event",
		"mr_iid", ctx.MRIID,
		"action", action,
		"source_branch", sourceBranch)

	result := map[string]interface{}{
		"status":        "ok",
		"mr_iid":        ctx.MRIID,
		"action":        action,
		"source_branch": sourceBranch,
	}

	// Extract additional MR data
	if title, ok := objAttrs["title"].(string); ok {
		result["title"] = title
	}
	if state, ok := objAttrs["state"].(string); ok {
		result["state"] = state
	}
	if targetBranch, ok := objAttrs["target_branch"].(string); ok {
		result["target_branch"] = targetBranch
	}
	if url, ok := objAttrs["url"].(string); ok {
		result["mr_url"] = url
	}

	// Here you would typically:
	// 1. Find associated session by branch name
	// 2. Find associated ticket
	// 3. Create or update TicketMergeRequest record
	// 4. Notify via WebSocket

	return result, nil
}

// PushHandler handles push webhook events
type PushHandler struct {
	logger *slog.Logger
}

// NewPushHandler creates a new push handler
func NewPushHandler(logger *slog.Logger) *PushHandler {
	return &PushHandler{logger: logger}
}

// CanHandle checks if this is a push event we should process
func (h *PushHandler) CanHandle(ctx *WebhookContext) bool {
	return ctx.ObjectKind == "push"
}

// Handle processes the push event
func (h *PushHandler) Handle(ctx *WebhookContext) (map[string]interface{}, error) {
	// Extract push info
	var ref, before, after string
	var totalCommits int

	if r, ok := ctx.Payload["ref"].(string); ok {
		ref = r
	}
	if b, ok := ctx.Payload["before"].(string); ok {
		before = b
	}
	if a, ok := ctx.Payload["after"].(string); ok {
		after = a
	}
	if commits, ok := ctx.Payload["commits"].([]interface{}); ok {
		totalCommits = len(commits)
	}

	h.logger.Info("processing push event",
		"project_id", ctx.ProjectID,
		"ref", ref,
		"commits", totalCommits)

	result := map[string]interface{}{
		"status":        "ok",
		"ref":           ref,
		"before":        before,
		"after":         after,
		"total_commits": totalCommits,
	}

	// Here you would typically:
	// 1. Check if branch is associated with a session
	// 2. Update session branch status
	// 3. Trigger any sync tasks

	return result, nil
}

// SetupDefaultHandlers configures the default webhook handlers
func SetupDefaultHandlers(registry *HandlerRegistry, logger *slog.Logger) {
	// Register pipeline handler
	registry.Register("pipeline", NewPipelineHandler(logger))

	// Register merge request handler
	registry.Register("merge_request", NewMergeRequestHandler(logger))

	// Register push handler (composite with sub-handlers)
	pushHandler := NewCompositeHandler(logger)
	pushHandler.AddSubHandler(NewPushHandler(logger))
	registry.Register("push", pushHandler)
}
