package aggregator

import (
	"sync"
	"testing"
	"time"
)

// Tests for basic aggregation functionality

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
		{0.0, 16 * time.Millisecond},  // No load: base delay
		{0.5, 64 * time.Millisecond},  // 50% load: 16 * (1 + 0.25*12) = 16 * 4 = 64
		{0.8, 124 * time.Millisecond}, // 80% load: 16 * (1 + 0.64*12) = 16 * 8.68 ≈ 139
		{1.0, 200 * time.Millisecond}, // 100% load: capped at maxDelay
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

func TestSmartAggregator_NilQueueUsageFn(t *testing.T) {
	done := make(chan struct{})

	agg := NewSmartAggregator(
		func(data []byte) {
			close(done)
		},
		nil,
		WithSmartBaseDelay(10*time.Millisecond),
	)

	agg.Write([]byte("test"))

	select {
	case <-done:
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
		WithSmartBaseDelay(1*time.Second),
	)
	defer agg.Stop()

	agg.Write([]byte("data"))
	agg.Flush()

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
