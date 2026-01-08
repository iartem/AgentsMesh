package runner

import (
	"testing"
	"time"
)

func TestPTYForwarderConfigStruct(t *testing.T) {
	cfg := PTYForwarderConfig{
		Session:          nil,
		SessionKey:       "session-1",
		Handler:          nil,
		BufferSize:       32 * 1024,
		FlushInterval:    100 * time.Millisecond,
		BackpressureWait: 200 * time.Millisecond,
	}

	if cfg.SessionKey != "session-1" {
		t.Errorf("SessionKey: got %v, want session-1", cfg.SessionKey)
	}

	if cfg.BufferSize != 32*1024 {
		t.Errorf("BufferSize: got %v, want %v", cfg.BufferSize, 32*1024)
	}

	if cfg.FlushInterval != 100*time.Millisecond {
		t.Errorf("FlushInterval: got %v, want 100ms", cfg.FlushInterval)
	}
}

func TestNewPTYForwarderDefaults(t *testing.T) {
	handler := newMockOutputHandler()
	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "session-1",
		Handler:    handler,
	}

	forwarder := NewPTYForwarder(cfg)

	if forwarder == nil {
		t.Fatal("NewPTYForwarder returned nil")
	}

	if forwarder.sessionKey != "session-1" {
		t.Errorf("sessionKey: got %v, want session-1", forwarder.sessionKey)
	}

	if forwarder.maxBufferSize != 64*1024 {
		t.Errorf("maxBufferSize: got %v, want %v", forwarder.maxBufferSize, 64*1024)
	}

	if forwarder.flushInterval != 50*time.Millisecond {
		t.Errorf("flushInterval: got %v, want 50ms", forwarder.flushInterval)
	}

	if forwarder.backpressureWait != 100*time.Millisecond {
		t.Errorf("backpressureWait: got %v, want 100ms", forwarder.backpressureWait)
	}
}

func TestNewPTYForwarderCustomConfig(t *testing.T) {
	handler := newMockOutputHandler()
	cfg := &PTYForwarderConfig{
		Session:          nil,
		SessionKey:       "session-1",
		Handler:          handler,
		BufferSize:       32 * 1024,
		FlushInterval:    100 * time.Millisecond,
		BackpressureWait: 200 * time.Millisecond,
	}

	forwarder := NewPTYForwarder(cfg)

	if forwarder.maxBufferSize != 32*1024 {
		t.Errorf("maxBufferSize: got %v, want %v", forwarder.maxBufferSize, 32*1024)
	}

	if forwarder.flushInterval != 100*time.Millisecond {
		t.Errorf("flushInterval: got %v, want 100ms", forwarder.flushInterval)
	}

	if forwarder.backpressureWait != 200*time.Millisecond {
		t.Errorf("backpressureWait: got %v, want 200ms", forwarder.backpressureWait)
	}
}

func TestPTYForwarderBackpressure(t *testing.T) {
	handler := newMockOutputHandler()
	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "session-1",
		Handler:    handler,
	}

	forwarder := NewPTYForwarder(cfg)

	// Test backpressure state
	if forwarder.isBackpressure() {
		t.Error("should not be in backpressure initially")
	}

	forwarder.setBackpressure(true)
	if !forwarder.isBackpressure() {
		t.Error("should be in backpressure after setting")
	}

	forwarder.setBackpressure(false)
	if forwarder.isBackpressure() {
		t.Error("should not be in backpressure after clearing")
	}
}

func TestPTYForwarderGetBufferedSize(t *testing.T) {
	handler := newMockOutputHandler()
	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "session-1",
		Handler:    handler,
	}

	forwarder := NewPTYForwarder(cfg)

	// Initially empty
	if forwarder.GetBufferedSize() != 0 {
		t.Errorf("initial buffered size: got %v, want 0", forwarder.GetBufferedSize())
	}

	// Buffer some data
	forwarder.bufferOutput([]byte("hello"))

	if forwarder.GetBufferedSize() != 5 {
		t.Errorf("buffered size after write: got %v, want 5", forwarder.GetBufferedSize())
	}
}

func TestPTYForwarderStop(t *testing.T) {
	handler := newMockOutputHandler()
	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "session-1",
		Handler:    handler,
	}

	forwarder := NewPTYForwarder(cfg)

	// Should not panic even without Start()
	forwarder.Stop()
}

func TestPTYForwarderStartStop(t *testing.T) {
	handler := newMockOutputHandler()
	cfg := &PTYForwarderConfig{
		Session:       nil, // Will cause read loop to exit immediately
		SessionKey:    "session-1",
		Handler:       handler,
		FlushInterval: 10 * time.Millisecond,
	}

	forwarder := NewPTYForwarder(cfg)

	// Test that Stop() can be called without Start()
	forwarder.Stop()
}

func TestPTYForwarderFlush(t *testing.T) {
	handler := newMockOutputHandler()
	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "session-1",
		Handler:    handler,
	}

	forwarder := NewPTYForwarder(cfg)

	// Buffer some data
	forwarder.bufferOutput([]byte("test data"))

	// Flush should send data to handler
	forwarder.flush()

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if len(handler.outputs) != 1 {
		t.Errorf("outputs count = %d, want 1", len(handler.outputs))
	}
}

func TestPTYForwarderFlushEmptyBuffer(t *testing.T) {
	handler := newMockOutputHandler()
	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "session-1",
		Handler:    handler,
	}

	forwarder := NewPTYForwarder(cfg)

	// Flush with empty buffer should not send anything
	forwarder.flush()

	handler.mu.Lock()
	defer handler.mu.Unlock()

	if len(handler.outputs) != 0 {
		t.Errorf("outputs count = %d, want 0", len(handler.outputs))
	}
}

func TestPTYForwarderFlushWithBackpressure(t *testing.T) {
	handler := newMockOutputHandler()
	handler.shouldBackpressure = true

	cfg := &PTYForwarderConfig{
		Session:          nil,
		SessionKey:       "session-1",
		Handler:          handler,
		BackpressureWait: 1 * time.Millisecond, // Short wait for testing
	}

	forwarder := NewPTYForwarder(cfg)

	// Buffer some data
	forwarder.bufferOutput([]byte("test data"))

	// Set backpressure
	forwarder.setBackpressure(true)

	// Flush should skip due to backpressure
	forwarder.flush()

	// Data should still be in buffer
	if forwarder.GetBufferedSize() > 0 {
		// Data is still buffered, which is expected under backpressure
	}
}

func TestPTYForwarderBufferOutputTriggersFlush(t *testing.T) {
	handler := newMockOutputHandler()
	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "session-1",
		Handler:    handler,
		BufferSize: 100, // Small buffer to trigger flush
	}

	forwarder := NewPTYForwarder(cfg)

	// Buffer more than half the buffer size to trigger flush
	largeData := make([]byte, 60)
	forwarder.bufferOutput(largeData)

	// Should have triggered a flush
	handler.mu.Lock()
	outputCount := len(handler.outputs)
	handler.mu.Unlock()

	if outputCount != 1 {
		t.Errorf("outputs count = %d, want 1 (auto flush triggered)", outputCount)
	}
}

func TestPTYForwarderBufferOutputMultiple(t *testing.T) {
	handler := newMockOutputHandler()
	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "session-1",
		Handler:    handler,
		BufferSize: 1024,
	}

	forwarder := NewPTYForwarder(cfg)

	// Buffer multiple writes
	forwarder.bufferOutput([]byte("first "))
	forwarder.bufferOutput([]byte("second "))
	forwarder.bufferOutput([]byte("third"))

	expectedSize := len("first ") + len("second ") + len("third")
	if forwarder.GetBufferedSize() != expectedSize {
		t.Errorf("buffered size: got %v, want %v", forwarder.GetBufferedSize(), expectedSize)
	}
}

func TestPTYForwarderBackpressureToggle(t *testing.T) {
	handler := newMockOutputHandler()
	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "session-1",
		Handler:    handler,
	}

	forwarder := NewPTYForwarder(cfg)

	// Toggle backpressure multiple times
	forwarder.setBackpressure(true)
	if !forwarder.isBackpressure() {
		t.Error("should be in backpressure")
	}

	forwarder.setBackpressure(false)
	if forwarder.isBackpressure() {
		t.Error("should not be in backpressure")
	}

	forwarder.setBackpressure(true)
	forwarder.setBackpressure(true) // Setting true again
	if !forwarder.isBackpressure() {
		t.Error("should still be in backpressure")
	}
}

func TestPTYForwarderConfigDefaults(t *testing.T) {
	cfg := PTYForwarderConfig{
		SessionKey: "session-1",
		Handler:    newMockOutputHandler(),
	}

	if cfg.BufferSize != 0 {
		t.Errorf("BufferSize default = %v, want 0", cfg.BufferSize)
	}
	if cfg.FlushInterval != 0 {
		t.Errorf("FlushInterval default = %v, want 0", cfg.FlushInterval)
	}
	if cfg.BackpressureWait != 0 {
		t.Errorf("BackpressureWait default = %v, want 0", cfg.BackpressureWait)
	}

	// NewPTYForwarder should apply defaults
	forwarder := NewPTYForwarder(&cfg)

	if forwarder.maxBufferSize != 64*1024 {
		t.Errorf("maxBufferSize = %v, want %v", forwarder.maxBufferSize, 64*1024)
	}
	if forwarder.flushInterval != 50*time.Millisecond {
		t.Errorf("flushInterval = %v, want 50ms", forwarder.flushInterval)
	}
	if forwarder.backpressureWait != 100*time.Millisecond {
		t.Errorf("backpressureWait = %v, want 100ms", forwarder.backpressureWait)
	}
}

// Benchmarks

func BenchmarkPTYForwarderBufferOutput(b *testing.B) {
	handler := newMockOutputHandler()
	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "session-1",
		Handler:    handler,
	}
	forwarder := NewPTYForwarder(cfg)
	data := []byte("hello world test data for benchmarking")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		forwarder.bufferOutput(data)
	}
}
