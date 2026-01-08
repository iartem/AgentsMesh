package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// JobPriority defines the priority level for jobs
type JobPriority int

const (
	PriorityLow JobPriority = iota
	PriorityNormal
	PriorityHigh
	PriorityCritical
)

// Job represents a background job to be processed
type Job struct {
	ID       string
	Type     string
	Payload  map[string]interface{}
	Priority JobPriority
	MaxRetry int
	Timeout  time.Duration

	// Internal fields
	retryCount int
	createdAt  time.Time
	startedAt  time.Time
}

// JobHandler is the function signature for job handlers
type JobHandler func(ctx context.Context, job *Job) error

// JobResult represents the result of a job execution
type JobResult struct {
	JobID     string
	JobType   string
	Success   bool
	Error     error
	Duration  time.Duration
	Retried   bool
}

// WorkerPool manages a pool of workers for processing jobs
type WorkerPool struct {
	handlers map[string]JobHandler
	jobs     chan *Job
	results  chan JobResult
	logger   *slog.Logger
	mu       sync.RWMutex
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc

	// Configuration
	workerCount int
	maxQueueSize int
}

// WorkerPoolConfig holds configuration for the worker pool
type WorkerPoolConfig struct {
	WorkerCount  int
	MaxQueueSize int
}

// DefaultWorkerPoolConfig returns default configuration
func DefaultWorkerPoolConfig() WorkerPoolConfig {
	return WorkerPoolConfig{
		WorkerCount:  4,
		MaxQueueSize: 1000,
	}
}

// NewWorkerPool creates a new worker pool
func NewWorkerPool(logger *slog.Logger, cfg WorkerPoolConfig) *WorkerPool {
	ctx, cancel := context.WithCancel(context.Background())

	return &WorkerPool{
		handlers:     make(map[string]JobHandler),
		jobs:         make(chan *Job, cfg.MaxQueueSize),
		results:      make(chan JobResult, cfg.MaxQueueSize),
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
		workerCount:  cfg.WorkerCount,
		maxQueueSize: cfg.MaxQueueSize,
	}
}

// RegisterHandler registers a handler for a job type
func (wp *WorkerPool) RegisterHandler(jobType string, handler JobHandler) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	wp.handlers[jobType] = handler
	wp.logger.Info("job handler registered", "type", jobType)
}

// Start begins the worker pool
func (wp *WorkerPool) Start() {
	wp.logger.Info("starting worker pool",
		"workers", wp.workerCount,
		"queue_size", wp.maxQueueSize)

	for i := 0; i < wp.workerCount; i++ {
		wp.wg.Add(1)
		go wp.worker(i)
	}
}

// Stop gracefully stops the worker pool
func (wp *WorkerPool) Stop() {
	wp.logger.Info("stopping worker pool")
	wp.cancel()
	close(wp.jobs)

	// Wait for workers with timeout
	done := make(chan struct{})
	go func() {
		wp.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		wp.logger.Info("worker pool stopped gracefully")
	case <-time.After(30 * time.Second):
		wp.logger.Warn("worker pool did not stop in time")
	}

	close(wp.results)
}

// Submit adds a job to the queue
func (wp *WorkerPool) Submit(job *Job) error {
	if job.ID == "" {
		job.ID = fmt.Sprintf("%s-%d", job.Type, time.Now().UnixNano())
	}
	job.createdAt = time.Now()

	if job.Timeout == 0 {
		job.Timeout = 5 * time.Minute
	}

	select {
	case wp.jobs <- job:
		wp.logger.Debug("job submitted",
			"job_id", job.ID,
			"type", job.Type,
			"priority", job.Priority)
		return nil
	default:
		return fmt.Errorf("job queue is full")
	}
}

// Results returns the results channel for monitoring
func (wp *WorkerPool) Results() <-chan JobResult {
	return wp.results
}

// worker processes jobs from the queue
func (wp *WorkerPool) worker(id int) {
	defer wp.wg.Done()

	wp.logger.Debug("worker started", "worker_id", id)

	for job := range wp.jobs {
		select {
		case <-wp.ctx.Done():
			return
		default:
			result := wp.processJob(job)
			wp.results <- result
		}
	}
}

// processJob executes a single job
func (wp *WorkerPool) processJob(job *Job) JobResult {
	job.startedAt = time.Now()
	result := JobResult{
		JobID:   job.ID,
		JobType: job.Type,
	}

	// Get handler
	wp.mu.RLock()
	handler, exists := wp.handlers[job.Type]
	wp.mu.RUnlock()

	if !exists {
		result.Error = fmt.Errorf("no handler for job type: %s", job.Type)
		result.Success = false
		result.Duration = time.Since(job.startedAt)
		return result
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(wp.ctx, job.Timeout)
	defer cancel()

	// Execute with panic recovery
	func() {
		defer func() {
			if r := recover(); r != nil {
				result.Error = fmt.Errorf("panic in job handler: %v", r)
			}
		}()
		result.Error = handler(ctx, job)
	}()

	result.Duration = time.Since(job.startedAt)
	result.Success = result.Error == nil

	// Handle retry
	if result.Error != nil && job.retryCount < job.MaxRetry {
		job.retryCount++
		result.Retried = true

		wp.logger.Warn("job failed, retrying",
			"job_id", job.ID,
			"type", job.Type,
			"retry", job.retryCount,
			"max_retry", job.MaxRetry,
			"error", result.Error)

		// Re-submit with exponential backoff
		go func() {
			backoff := time.Duration(job.retryCount) * time.Second
			time.Sleep(backoff)
			_ = wp.Submit(job)
		}()
	} else if result.Error != nil {
		wp.logger.Error("job failed permanently",
			"job_id", job.ID,
			"type", job.Type,
			"error", result.Error,
			"duration", result.Duration)
	} else {
		wp.logger.Debug("job completed",
			"job_id", job.ID,
			"type", job.Type,
			"duration", result.Duration)
	}

	return result
}

// QueueLength returns the current number of jobs in the queue
func (wp *WorkerPool) QueueLength() int {
	return len(wp.jobs)
}

// GetHandlerTypes returns all registered handler types
func (wp *WorkerPool) GetHandlerTypes() []string {
	wp.mu.RLock()
	defer wp.mu.RUnlock()

	types := make([]string, 0, len(wp.handlers))
	for t := range wp.handlers {
		types = append(types, t)
	}
	return types
}
