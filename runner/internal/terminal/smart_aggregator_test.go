package terminal

import (
	"bytes"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestSmartAggregator_BasicAggregation(t *testing.T) {
	var received []byte
	var mu sync.Mutex
	done := make(chan struct{})

	agg := NewSmartAggregator(
		func(data []byte) {
			mu.Lock()
			received = append(received, data...)
			mu.Unlock()
			select {
			case done <- struct{}{}:
			default:
			}
		},
		func() float64 { return 0 }, // No queue pressure
		WithSmartBaseDelay(10*time.Millisecond),
	)

	// Write some data
	agg.Write([]byte("hello"))
	agg.Write([]byte(" world"))

	// Wait for flush
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for flush")
	}

	mu.Lock()
	defer mu.Unlock()
	if string(received) != "hello world" {
		t.Errorf("Expected 'hello world', got '%s'", string(received))
	}
}

func TestSmartAggregator_AdaptiveDelay(t *testing.T) {
	agg := NewSmartAggregator(
		func(data []byte) {},
		func() float64 { return 0 },
		WithSmartBaseDelay(16*time.Millisecond),
		WithSmartMaxDelay(200*time.Millisecond),
	)
	defer agg.Stop()

	tests := []struct {
		usage    float64
		expected time.Duration
	}{
		{0.0, 16 * time.Millisecond},   // No load: base delay
		{0.5, 64 * time.Millisecond},   // 50% load: 16 * (1 + 0.25*12) = 16 * 4 = 64
		{0.8, 124 * time.Millisecond},  // 80% load: 16 * (1 + 0.64*12) = 16 * 8.68 ≈ 139 (capped calculation)
		{1.0, 200 * time.Millisecond},  // 100% load: capped at maxDelay
	}

	for _, tc := range tests {
		delay := agg.calculateDelay(tc.usage)
		// Allow 20% tolerance for rounding
		minExpected := time.Duration(float64(tc.expected) * 0.8)
		maxExpected := time.Duration(float64(tc.expected) * 1.2)
		if tc.usage == 1.0 {
			// For max load, should be exactly maxDelay
			if delay != tc.expected {
				t.Errorf("usage=%.1f: expected %v, got %v", tc.usage, tc.expected, delay)
			}
		} else if delay < minExpected || delay > maxExpected {
			t.Errorf("usage=%.1f: expected ~%v, got %v", tc.usage, tc.expected, delay)
		}
	}
}

func TestSmartAggregator_DiscardOldFrames(t *testing.T) {
	var received []byte
	var mu sync.Mutex
	done := make(chan struct{}, 10)

	agg := NewSmartAggregator(
		func(data []byte) {
			mu.Lock()
			received = data
			mu.Unlock()
			done <- struct{}{}
		},
		func() float64 { return 0.8 }, // High pressure - triggers frame discard
		WithSmartBaseDelay(10*time.Millisecond),
	)

	// Write data with clear screen sequence in the middle
	// old frame + clear screen + new frame
	agg.Write([]byte("old content"))
	agg.Write([]byte("\x1b[2J")) // Clear screen
	agg.Write([]byte("new content"))

	// Wait for flush
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for flush")
	}

	mu.Lock()
	defer mu.Unlock()

	// Should only contain content from clear screen onwards
	expected := "\x1b[2Jnew content"
	if string(received) != expected {
		t.Errorf("Expected '%s', got '%s'", expected, string(received))
	}
}

func TestSmartAggregator_MaxSizeFlush(t *testing.T) {
	var flushCount int32
	done := make(chan struct{}, 10)

	agg := NewSmartAggregator(
		func(data []byte) {
			atomic.AddInt32(&flushCount, 1)
			done <- struct{}{}
		},
		func() float64 { return 0 },
		WithSmartMaxSize(100), // Small max size for testing
		WithSmartBaseDelay(1*time.Second), // Long delay so timer doesn't interfere
	)
	defer agg.Stop()

	// Write data exceeding max size
	data := bytes.Repeat([]byte("x"), 150)
	agg.Write(data)

	// Should flush immediately due to max size
	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Expected immediate flush on max size exceeded")
	}

	count := atomic.LoadInt32(&flushCount)
	if count < 1 {
		t.Errorf("Expected at least 1 flush, got %d", count)
	}
}

func TestSmartAggregator_Stop(t *testing.T) {
	var received []byte
	var mu sync.Mutex
	done := make(chan struct{}, 1)

	agg := NewSmartAggregator(
		func(data []byte) {
			mu.Lock()
			received = data
			mu.Unlock()
			select {
			case done <- struct{}{}:
			default:
			}
		},
		func() float64 { return 0 },
		WithSmartBaseDelay(1*time.Second), // Long delay
	)

	// Write data
	agg.Write([]byte("pending data"))

	// Stop should flush immediately
	agg.Stop()

	// Wait a bit for the goroutine
	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Expected flush on stop")
	}

	mu.Lock()
	defer mu.Unlock()
	if string(received) != "pending data" {
		t.Errorf("Expected 'pending data', got '%s'", string(received))
	}

	// Subsequent writes should be ignored
	agg.Write([]byte("ignored"))
	if agg.BufferLen() != 0 {
		t.Error("Buffer should be empty after stop")
	}
}

func TestSmartAggregator_ConcurrentWrites(t *testing.T) {
	var totalBytes int64
	var mu sync.Mutex

	agg := NewSmartAggregator(
		func(data []byte) {
			mu.Lock()
			totalBytes += int64(len(data))
			mu.Unlock()
		},
		func() float64 { return 0 },
		WithSmartBaseDelay(5*time.Millisecond),
	)

	// Concurrent writes
	var wg sync.WaitGroup
	numWriters := 10
	bytesPerWriter := 1000

	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < bytesPerWriter; j++ {
				agg.Write([]byte("x"))
			}
		}()
	}

	wg.Wait()
	agg.Stop()

	// Give some time for final flush
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	expected := int64(numWriters * bytesPerWriter)
	if totalBytes != expected {
		t.Errorf("Expected %d bytes, got %d", expected, totalBytes)
	}
}

func TestSmartAggregator_NilQueueUsageFn(t *testing.T) {
	done := make(chan struct{})

	// Test with nil queueUsageFn (should not panic)
	agg := NewSmartAggregator(
		func(data []byte) {
			close(done)
		},
		nil, // nil queue usage function
		WithSmartBaseDelay(10*time.Millisecond),
	)

	agg.Write([]byte("test"))

	select {
	case <-done:
		// Success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Timeout waiting for flush")
	}

	agg.Stop()
}

func TestSmartAggregator_Flush(t *testing.T) {
	var received []byte
	var mu sync.Mutex
	done := make(chan struct{}, 1)

	agg := NewSmartAggregator(
		func(data []byte) {
			mu.Lock()
			received = data
			mu.Unlock()
			select {
			case done <- struct{}{}:
			default:
			}
		},
		func() float64 { return 0 },
		WithSmartBaseDelay(1*time.Second), // Long delay
	)
	defer agg.Stop()

	// Write data
	agg.Write([]byte("data"))

	// Manual flush
	agg.Flush()

	// Should flush immediately
	select {
	case <-done:
	case <-time.After(50 * time.Millisecond):
		t.Fatal("Expected immediate flush")
	}

	mu.Lock()
	defer mu.Unlock()
	if string(received) != "data" {
		t.Errorf("Expected 'data', got '%s'", string(received))
	}
}

func TestSmartAggregator_IsStopped(t *testing.T) {
	agg := NewSmartAggregator(
		func(data []byte) {},
		func() float64 { return 0 },
	)

	if agg.IsStopped() {
		t.Error("Should not be stopped initially")
	}

	agg.Stop()

	if !agg.IsStopped() {
		t.Error("Should be stopped after Stop()")
	}

	// Double stop should not panic
	agg.Stop()
}
