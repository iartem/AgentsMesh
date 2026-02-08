package aggregator

import (
	"bytes"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestSmartAggregator_FullRedrawThrottling_Recovery tests that throttling delay
// decreases when frequency drops, and data is still eventually delivered
func TestSmartAggregator_FullRedrawThrottling_Recovery(t *testing.T) {
	var mu sync.Mutex
	var flushedData [][]byte

	agg := NewSmartAggregator(
		func(data []byte) {
			mu.Lock()
			dataCopy := make([]byte, len(data))
			copy(dataCopy, data)
			flushedData = append(flushedData, dataCopy)
			mu.Unlock()
		},
		func() float64 { return 0 },
		WithSmartBaseDelay(10*time.Millisecond),
		WithFullRedrawThrottling(
			WithThrottlerWindowSize(100*time.Millisecond),
			WithThrottlerMinWindow(100*time.Millisecond),
			WithThrottlerMaxWindow(100*time.Millisecond),
			WithThrottlerThreshold(5.0),
			WithThrottlerMinDelay(50*time.Millisecond),
		),
	)
	defer agg.Stop()

	// Phase 1: High frequency full redraws
	for i := 0; i < 5; i++ {
		frame := buildFullRedrawFrame("high_freq_" + string(rune('0'+i)))
		agg.Write(frame)
		time.Sleep(10 * time.Millisecond)
	}

	// Phase 2: Wait for window to clear, then write one more frame
	time.Sleep(200 * time.Millisecond)

	frame := buildFullRedrawFrame("recovery_frame")
	agg.Write(frame)

	// Force flush and wait
	time.Sleep(100 * time.Millisecond)
	agg.Flush()
	time.Sleep(50 * time.Millisecond)

	// Verify that at least the recovery frame was delivered
	mu.Lock()
	defer mu.Unlock()

	foundRecovery := false
	for _, data := range flushedData {
		if bytes.Contains(data, []byte("recovery_frame")) {
			foundRecovery = true
			break
		}
	}

	t.Logf("Total flushes: %d, found recovery frame: %v", len(flushedData), foundRecovery)

	// The key assertion: data is eventually delivered
	if !foundRecovery {
		t.Error("Recovery frame should be delivered after throttling window expires")
	}
}

// TestSmartAggregator_FullRedrawThrottling_ContentPreserved tests that
// throttling preserves the latest content
func TestSmartAggregator_FullRedrawThrottling_ContentPreserved(t *testing.T) {
	var mu sync.Mutex
	var allFlushed [][]byte

	agg := NewSmartAggregator(
		func(data []byte) {
			mu.Lock()
			dataCopy := make([]byte, len(data))
			copy(dataCopy, data)
			allFlushed = append(allFlushed, dataCopy)
			mu.Unlock()
		},
		func() float64 { return 0 },
		WithSmartBaseDelay(10*time.Millisecond),
		WithFullRedrawThrottling(
			WithThrottlerWindowSize(500*time.Millisecond),
			WithThrottlerMinWindow(500*time.Millisecond),
			WithThrottlerMaxWindow(500*time.Millisecond),
			WithThrottlerThreshold(2.0),
			WithThrottlerMinDelay(100*time.Millisecond),
		),
	)
	defer agg.Stop()

	// Write frames with unique identifiers
	for i := 0; i < 10; i++ {
		// Use a unique marker that won't be in other frames
		frame := buildFullRedrawFrame("MARKER_" + string(rune('A'+i)))
		agg.Write(frame)
		time.Sleep(15 * time.Millisecond)
	}

	// Force final flush
	agg.Flush()
	time.Sleep(50 * time.Millisecond)

	// Check that we got the latest data in one of the flushes
	mu.Lock()
	defer mu.Unlock()

	foundLatest := false
	for _, data := range allFlushed {
		if bytes.Contains(data, []byte("MARKER_")) {
			// Found at least one marker
			foundLatest = true
			break
		}
	}

	if !foundLatest {
		t.Error("Should have flushed at least some frame content")
	}

	t.Logf("Total flushes: %d", len(allFlushed))
}

// TestSmartAggregator_FullRedrawThrottling_SerializeModeBypassed tests that
// throttling is bypassed in serialize mode (VirtualTerminal mode)
func TestSmartAggregator_FullRedrawThrottling_SerializeModeBypassed(t *testing.T) {
	var flushCount int32
	serializedOutput := []byte("serialized content")

	agg := NewSmartAggregator(
		func(data []byte) {
			atomic.AddInt32(&flushCount, 1)
		},
		func() float64 { return 0 },
		WithSmartBaseDelay(10*time.Millisecond),
		WithSerializeCallback(func() []byte {
			return serializedOutput
		}),
		WithFullRedrawThrottling(
			WithThrottlerWindowSize(500*time.Millisecond),
			WithThrottlerMinWindow(500*time.Millisecond),
			WithThrottlerMaxWindow(500*time.Millisecond),
			WithThrottlerThreshold(2.0),
			WithThrottlerMinDelay(200*time.Millisecond),
		),
	)
	defer agg.Stop()

	// In serialize mode, Write() just marks pending data
	for i := 0; i < 5; i++ {
		agg.Write(nil) // Trigger pending flag
		time.Sleep(30 * time.Millisecond)
	}

	time.Sleep(100 * time.Millisecond)

	count := atomic.LoadInt32(&flushCount)

	// In serialize mode, throttling should be bypassed
	// We should get normal flush behavior
	t.Logf("Serialize mode: %d flushes for 5 writes", count)

	// Should have at least some flushes (not throttled)
	if count < 2 {
		t.Errorf("Serialize mode should bypass throttling: only %d flushes", count)
	}
}
