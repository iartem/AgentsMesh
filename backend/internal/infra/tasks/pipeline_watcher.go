package tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis key constants
const (
	WatchingSetKey            = "pipelines:watching"
	PipelineKeyPrefix         = "pipeline:"
	PollerLockKey             = "poller:lock"
	PollerHeartbeatKey        = "poller:heartbeat"
	LockTimeoutSeconds        = 60
	HeartbeatTTLSeconds       = 30
	CompletedPipelineTTL      = 24 * time.Hour
	RecentUpdateThreshold     = 5 * time.Second
)

// Terminal pipeline statuses
var TerminalStatuses = map[string]bool{
	"success":  true,
	"failed":   true,
	"canceled": true,
	"skipped":  true,
}

// WatchedPipeline represents a pipeline being watched
type WatchedPipeline struct {
	ProjectID    string                 `json:"project_id"`
	PipelineID   string                 `json:"pipeline_id"`
	TaskType     string                 `json:"task_type"`
	TaskID       int64                  `json:"task_id"`
	Status       string                 `json:"status"`
	WebURL       string                 `json:"web_url,omitempty"`
	UpdatedAt    time.Time              `json:"updated_at"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	ArtifactPath string                 `json:"artifact_path,omitempty"`
	JobName      string                 `json:"job_name,omitempty"`
	ResultJSON   string                 `json:"result_json,omitempty"`
}

// PipelineWatcher manages pipeline status watching in Redis
type PipelineWatcher struct {
	redis  *redis.Client
	logger *slog.Logger
}

// NewPipelineWatcher creates a new pipeline watcher
func NewPipelineWatcher(redisClient *redis.Client, logger *slog.Logger) *PipelineWatcher {
	return &PipelineWatcher{
		redis:  redisClient,
		logger: logger,
	}
}

// Watch starts watching a pipeline
func (pw *PipelineWatcher) Watch(ctx context.Context, projectID, pipelineID string, taskType string, taskID int64, metadata map[string]interface{}) error {
	key := fmt.Sprintf("%s:%s", projectID, pipelineID)
	hashKey := PipelineKeyPrefix + key

	// Add to watching set
	if err := pw.redis.SAdd(ctx, WatchingSetKey, key).Err(); err != nil {
		return fmt.Errorf("failed to add to watching set: %w", err)
	}

	// Store pipeline metadata
	data := map[string]interface{}{
		"project_id":  projectID,
		"pipeline_id": pipelineID,
		"task_type":   taskType,
		"task_id":     taskID,
		"status":      "pending",
		"updated_at":  time.Now().UTC().Format(time.RFC3339),
	}

	// Add metadata fields
	if metadata != nil {
		if artifactPath, ok := metadata["artifact_path"].(string); ok {
			data["artifact_path"] = artifactPath
		}
		if jobName, ok := metadata["job_name"].(string); ok {
			data["job_name"] = jobName
		}
		if metaJSON, err := json.Marshal(metadata); err == nil {
			data["metadata"] = string(metaJSON)
		}
	}

	if err := pw.redis.HSet(ctx, hashKey, data).Err(); err != nil {
		return fmt.Errorf("failed to store pipeline data: %w", err)
	}

	pw.logger.Info("pipeline watch started",
		"project_id", projectID,
		"pipeline_id", pipelineID,
		"task_type", taskType,
		"task_id", taskID)

	return nil
}

// UpdateStatus updates the status of a watched pipeline
func (pw *PipelineWatcher) UpdateStatus(ctx context.Context, projectID, pipelineID, status string, webURL string) error {
	key := fmt.Sprintf("%s:%s", projectID, pipelineID)
	hashKey := PipelineKeyPrefix + key

	data := map[string]interface{}{
		"status":     status,
		"updated_at": time.Now().UTC().Format(time.RFC3339),
	}
	if webURL != "" {
		data["web_url"] = webURL
	}

	if err := pw.redis.HSet(ctx, hashKey, data).Err(); err != nil {
		return fmt.Errorf("failed to update pipeline status: %w", err)
	}

	// If terminal status, remove from watching set and set TTL
	if TerminalStatuses[status] {
		pw.redis.SRem(ctx, WatchingSetKey, key)
		pw.redis.Expire(ctx, hashKey, CompletedPipelineTTL)
	}

	return nil
}

// GetPipeline retrieves pipeline data from Redis
func (pw *PipelineWatcher) GetPipeline(ctx context.Context, projectID, pipelineID string) (*WatchedPipeline, error) {
	key := fmt.Sprintf("%s:%s", projectID, pipelineID)
	hashKey := PipelineKeyPrefix + key

	data, err := pw.redis.HGetAll(ctx, hashKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get pipeline data: %w", err)
	}

	if len(data) == 0 {
		return nil, nil
	}

	pipeline := &WatchedPipeline{
		ProjectID:    data["project_id"],
		PipelineID:   data["pipeline_id"],
		TaskType:     data["task_type"],
		Status:       data["status"],
		WebURL:       data["web_url"],
		ArtifactPath: data["artifact_path"],
		JobName:      data["job_name"],
		ResultJSON:   data["result_json"],
	}

	if taskID, err := strconv.ParseInt(data["task_id"], 10, 64); err == nil {
		pipeline.TaskID = taskID
	}

	if updatedAt, err := time.Parse(time.RFC3339, data["updated_at"]); err == nil {
		pipeline.UpdatedAt = updatedAt
	}

	if metadataStr := data["metadata"]; metadataStr != "" {
		_ = json.Unmarshal([]byte(metadataStr), &pipeline.Metadata)
	}

	return pipeline, nil
}

// GetCompletedPipelines retrieves all completed pipelines of a specific type
func (pw *PipelineWatcher) GetCompletedPipelines(ctx context.Context, taskType string) ([]*WatchedPipeline, error) {
	// Get all watching keys
	keys, err := pw.redis.SMembers(ctx, WatchingSetKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get watching set: %w", err)
	}

	var completed []*WatchedPipeline

	for _, key := range keys {
		hashKey := PipelineKeyPrefix + key
		data, err := pw.redis.HGetAll(ctx, hashKey).Result()
		if err != nil {
			pw.logger.Warn("failed to get pipeline data", "key", key, "error", err)
			continue
		}

		// Filter by task type
		if data["task_type"] != taskType {
			continue
		}

		// Check if status is terminal
		status := data["status"]
		if !TerminalStatuses[status] {
			continue
		}

		pipeline := &WatchedPipeline{
			ProjectID:    data["project_id"],
			PipelineID:   data["pipeline_id"],
			TaskType:     data["task_type"],
			Status:       status,
			WebURL:       data["web_url"],
			ArtifactPath: data["artifact_path"],
			JobName:      data["job_name"],
			ResultJSON:   data["result_json"],
		}

		if taskID, err := strconv.ParseInt(data["task_id"], 10, 64); err == nil {
			pipeline.TaskID = taskID
		}

		if updatedAt, err := time.Parse(time.RFC3339, data["updated_at"]); err == nil {
			pipeline.UpdatedAt = updatedAt
		}

		completed = append(completed, pipeline)
	}

	return completed, nil
}

// GetWatchingCount returns the number of pipelines being watched
func (pw *PipelineWatcher) GetWatchingCount(ctx context.Context) (int64, error) {
	return pw.redis.SCard(ctx, WatchingSetKey).Result()
}

// MarkProcessed marks a pipeline as processed (removes from watching)
func (pw *PipelineWatcher) MarkProcessed(ctx context.Context, projectID, pipelineID string) error {
	key := fmt.Sprintf("%s:%s", projectID, pipelineID)
	hashKey := PipelineKeyPrefix + key

	// Remove from watching set
	if err := pw.redis.SRem(ctx, WatchingSetKey, key).Err(); err != nil {
		return fmt.Errorf("failed to remove from watching set: %w", err)
	}

	// Update processed flag
	if err := pw.redis.HSet(ctx, hashKey, "processed", "true").Err(); err != nil {
		return fmt.Errorf("failed to mark as processed: %w", err)
	}

	// Set TTL for cleanup
	pw.redis.Expire(ctx, hashKey, CompletedPipelineTTL)

	return nil
}

// StoreArtifact stores artifact data for a pipeline
func (pw *PipelineWatcher) StoreArtifact(ctx context.Context, projectID, pipelineID string, resultJSON string) error {
	key := fmt.Sprintf("%s:%s", projectID, pipelineID)
	hashKey := PipelineKeyPrefix + key

	return pw.redis.HSet(ctx, hashKey, "result_json", resultJSON).Err()
}

// Unwatch removes a pipeline from watching
func (pw *PipelineWatcher) Unwatch(ctx context.Context, projectID, pipelineID string) error {
	key := fmt.Sprintf("%s:%s", projectID, pipelineID)
	hashKey := PipelineKeyPrefix + key

	// Remove from watching set
	pw.redis.SRem(ctx, WatchingSetKey, key)

	// Delete the hash
	pw.redis.Del(ctx, hashKey)

	return nil
}

// GetWatchingKeys returns all keys being watched
func (pw *PipelineWatcher) GetWatchingKeys(ctx context.Context) ([]string, error) {
	return pw.redis.SMembers(ctx, WatchingSetKey).Result()
}

// IsRecentlyUpdated checks if a pipeline was updated recently (by webhook)
func (pw *PipelineWatcher) IsRecentlyUpdated(ctx context.Context, projectID, pipelineID string) (bool, error) {
	key := fmt.Sprintf("%s:%s", projectID, pipelineID)
	hashKey := PipelineKeyPrefix + key

	updatedAtStr, err := pw.redis.HGet(ctx, hashKey, "updated_at").Result()
	if err == redis.Nil {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	updatedAt, err := time.Parse(time.RFC3339, updatedAtStr)
	if err != nil {
		return false, nil
	}

	return time.Since(updatedAt) < RecentUpdateThreshold, nil
}
