package aggregator

import (
	"bytes"
	"sync"
	"testing"
	"time"
)

// Tests for concurrent access and stop behavior

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
		WithSmartBaseDelay(1*time.Second),
	)

	agg.Write([]byte("pending data"))
	agg.Stop()

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

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()

	expected := int64(numWriters * bytesPerWriter)
	if totalBytes != expected {
		t.Errorf("Expected %d bytes, got %d", expected, totalBytes)
	}
}

func TestSmartAggregator_LargeChunkExceedsMaxSize(t *testing.T) {
	maxSize := 100
	var flushed []byte
	var mu sync.Mutex

	agg := NewSmartAggregator(
		func(data []byte) {
			mu.Lock()
			flushed = append(flushed, data...)
			mu.Unlock()
		},
		func() float64 { return 0 },
		WithSmartMaxSize(maxSize),
		WithSmartBaseDelay(10*time.Millisecond),
	)

	agg.Write([]byte("prefix"))
	largeChunk := bytes.Repeat([]byte("L"), 200)
	agg.Write(largeChunk)

	if agg.BufferLen() > maxSize {
		t.Errorf("Buffer exceeded maxSize after large write: %d > %d",
			agg.BufferLen(), maxSize)
	}

	agg.Stop()
	time.Sleep(20 * time.Millisecond)

	t.Logf("Large chunk test: buffer stayed within %d limit", maxSize)
}
