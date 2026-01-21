// Package terminal provides terminal management for PTY sessions.
package terminal

import (
	"bytes"
	"sync"
	"time"
)

// ANSI clear screen sequence: ESC[2J
var clearScreenSeq = []byte{0x1b, '[', '2', 'J'}

// SmartAggregator intelligently aggregates TUI output with adaptive frame rate.
//
// Key features:
// - Time-window aggregation (base 16ms = 60 FPS)
// - Clear screen detection for frame boundary identification
// - High load: only keep latest complete frame (discard old frames)
// - Adaptive delay based on queue pressure (16ms → 200ms)
//
// Design principle: TUI apps (like Claude Code) use full-screen redraws,
// so dropping intermediate frames doesn't affect final display.
type SmartAggregator struct {
	mu       sync.Mutex
	buffer   bytes.Buffer
	timer    *time.Timer
	onFlush  func([]byte)
	stopped  bool

	// Configuration
	baseDelay    time.Duration  // Base delay 16ms (60 FPS)
	maxDelay     time.Duration  // Max delay 200ms (5 FPS)
	maxSize      int            // Max buffer size 64KB
	queueUsageFn func() float64 // Queue usage callback
}

// SmartAggregatorOption is a functional option for SmartAggregator.
type SmartAggregatorOption func(*SmartAggregator)

// WithSmartBaseDelay sets the base delay for aggregation.
func WithSmartBaseDelay(d time.Duration) SmartAggregatorOption {
	return func(a *SmartAggregator) {
		a.baseDelay = d
	}
}

// WithSmartMaxDelay sets the maximum delay for aggregation.
func WithSmartMaxDelay(d time.Duration) SmartAggregatorOption {
	return func(a *SmartAggregator) {
		a.maxDelay = d
	}
}

// WithSmartMaxSize sets the maximum buffer size.
func WithSmartMaxSize(size int) SmartAggregatorOption {
	return func(a *SmartAggregator) {
		a.maxSize = size
	}
}

// NewSmartAggregator creates a new smart aggregator.
//
// Parameters:
// - onFlush: callback invoked with aggregated data
// - queueUsageFn: returns queue usage ratio (0.0 to 1.0), used for adaptive delay
func NewSmartAggregator(onFlush func([]byte), queueUsageFn func() float64, opts ...SmartAggregatorOption) *SmartAggregator {
	a := &SmartAggregator{
		onFlush:      onFlush,
		queueUsageFn: queueUsageFn,
		baseDelay:    16 * time.Millisecond,  // 60 FPS
		maxDelay:     200 * time.Millisecond, // 5 FPS
		maxSize:      64 * 1024,              // 64KB
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// Write adds data to the aggregation buffer.
// Thread-safe: can be called from multiple goroutines.
func (a *SmartAggregator) Write(data []byte) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.stopped {
		return
	}

	a.buffer.Write(data)

	// Get current queue usage
	usage := 0.0
	if a.queueUsageFn != nil {
		usage = a.queueUsageFn()
	}

	// High load: discard old frames, keep only latest complete frame
	if usage > 0.5 {
		a.discardOldFrames()
	}

	// Calculate adaptive delay based on queue pressure
	delay := a.calculateDelay(usage)

	// Schedule flush timer
	if a.timer == nil {
		a.timer = time.AfterFunc(delay, a.timerFlush)
	}

	// Flush immediately if buffer exceeds max size
	if a.buffer.Len() >= a.maxSize {
		a.flushLocked()
	}
}

// discardOldFrames keeps only the content after the last clear screen sequence.
// This ensures we send complete frames, avoiding screen tearing.
func (a *SmartAggregator) discardOldFrames() {
	data := a.buffer.Bytes()
	if idx := bytes.LastIndex(data, clearScreenSeq); idx > 0 {
		// Keep only content from the last clear screen onwards
		newData := make([]byte, len(data)-idx)
		copy(newData, data[idx:])
		a.buffer.Reset()
		a.buffer.Write(newData)
	}
}

// calculateDelay computes adaptive delay based on queue usage.
//
// Load mapping:
// - 0%   → 16ms  (60 FPS) - smooth performance
// - 50%  → 50ms  (20 FPS) - moderate throttling
// - 80%  → 100ms (10 FPS) - significant throttling
// - 100% → 200ms (5 FPS)  - maximum throttling
func (a *SmartAggregator) calculateDelay(usage float64) time.Duration {
	// Quadratic scaling: factor = 1 + usage² * 12
	// This gives smooth ramp-up with aggressive throttling at high load
	factor := 1.0 + (usage * usage * 12)
	delay := time.Duration(float64(a.baseDelay) * factor)
	if delay > a.maxDelay {
		return a.maxDelay
	}
	return delay
}

// timerFlush is called when the timer fires.
func (a *SmartAggregator) timerFlush() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.stopped {
		a.flushLocked()
	}
}

// flushLocked flushes the buffer. Must be called with lock held.
func (a *SmartAggregator) flushLocked() {
	if a.timer != nil {
		a.timer.Stop()
		a.timer = nil
	}

	if a.buffer.Len() == 0 {
		return
	}

	// Copy data and reset buffer
	data := make([]byte, a.buffer.Len())
	copy(data, a.buffer.Bytes())
	a.buffer.Reset()

	// Call callback outside lock to avoid deadlock
	if a.onFlush != nil {
		// Use goroutine to avoid blocking the lock
		go a.onFlush(data)
	}
}

// Flush forces an immediate flush of the buffer.
// Thread-safe: can be called from any goroutine.
func (a *SmartAggregator) Flush() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.stopped {
		a.flushLocked()
	}
}

// Stop stops the aggregator and flushes remaining data.
// After Stop(), Write() calls are ignored.
func (a *SmartAggregator) Stop() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.stopped {
		return
	}
	a.stopped = true
	a.flushLocked()
}

// IsStopped returns whether the aggregator has been stopped.
func (a *SmartAggregator) IsStopped() bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.stopped
}

// BufferLen returns the current buffer length (for testing/debugging).
func (a *SmartAggregator) BufferLen() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.buffer.Len()
}
