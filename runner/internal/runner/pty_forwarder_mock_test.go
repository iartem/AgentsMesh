package runner

import (
	"io"
	"sync"
	"testing"
	"time"
)

// mockReadCloser implements io.ReadCloser for testing PTYForwarder
type mockReadCloser struct {
	data      []byte
	offset    int
	closed    bool
	mu        sync.Mutex
	readDelay time.Duration
	readError error
}

func newMockReadCloser(data []byte) *mockReadCloser {
	return &mockReadCloser{data: data}
}

func (m *mockReadCloser) Read(p []byte) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.readDelay > 0 {
		m.mu.Unlock()
		time.Sleep(m.readDelay)
		m.mu.Lock()
	}

	if m.readError != nil {
		return 0, m.readError
	}

	if m.closed {
		return 0, io.EOF
	}

	if m.offset >= len(m.data) {
		return 0, io.EOF
	}

	n := copy(p, m.data[m.offset:])
	m.offset += n
	return n, nil
}

func (m *mockReadCloser) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockReadCloser) addData(data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data = append(m.data, data...)
}

// mockTerminalSession implements the minimal interface needed for PTYForwarder
type mockTerminalSession struct {
	reader  io.ReadCloser
	mu      sync.Mutex
}

func newMockTerminalSession(reader io.ReadCloser) *mockTerminalSession {
	return &mockTerminalSession{reader: reader}
}

func (m *mockTerminalSession) Read(p []byte) (int, error) {
	return m.reader.Read(p)
}

// --- Tests for PTYForwarder with mock session ---

func TestPTYForwarderStartWithMockSession(t *testing.T) {
	mockReader := newMockReadCloser([]byte("test output"))
	mockHandler := newMockOutputHandler()

	cfg := &PTYForwarderConfig{
		Session:       nil, // nil session will cause read loop to exit
		SessionKey:    "test-session",
		Handler:       mockHandler,
		FlushInterval: 10 * time.Millisecond,
	}

	forwarder := NewPTYForwarder(cfg)

	// Start the forwarder
	forwarder.Start()

	// Wait for loops to start
	time.Sleep(50 * time.Millisecond)

	// Stop the forwarder
	forwarder.Stop()

	// Should not panic
	_ = mockReader
}

func TestPTYForwarderFlushLoopWithData(t *testing.T) {
	mockHandler := newMockOutputHandler()

	cfg := &PTYForwarderConfig{
		Session:       nil,
		SessionKey:    "test-session",
		Handler:       mockHandler,
		FlushInterval: 20 * time.Millisecond,
	}

	forwarder := NewPTYForwarder(cfg)

	// Buffer some data
	forwarder.bufferOutput([]byte("test data 1"))
	forwarder.bufferOutput([]byte("test data 2"))

	// Start flush loop
	done := make(chan struct{})
	go func() {
		forwarder.flushTicker = time.NewTicker(forwarder.flushInterval)
		for {
			select {
			case <-done:
				return
			case <-forwarder.flushTicker.C:
				forwarder.flush()
			}
		}
	}()

	// Wait for flush
	time.Sleep(50 * time.Millisecond)

	// Stop
	close(done)

	// Check handler received data
	mockHandler.mu.Lock()
	outputCount := len(mockHandler.outputs)
	mockHandler.mu.Unlock()

	if outputCount < 1 {
		t.Errorf("expected at least 1 output, got %d", outputCount)
	}
}

func TestPTYForwarderFlushLockedWithBackpressure(t *testing.T) {
	mockHandler := newMockOutputHandler()

	cfg := &PTYForwarderConfig{
		Session:          nil,
		SessionKey:       "test-session",
		Handler:          mockHandler,
		BackpressureWait: 1 * time.Millisecond,
	}

	forwarder := NewPTYForwarder(cfg)

	// Buffer data
	forwarder.bufferOutput([]byte("test"))

	// Set permanent backpressure
	forwarder.setBackpressure(true)

	// Flush should skip
	forwarder.flush()

	// Buffer should still have data
	if forwarder.GetBufferedSize() == 0 {
		t.Error("buffer should still have data when backpressure is on")
	}
}

func TestPTYForwarderFlushSetsBackpressureOnFalse(t *testing.T) {
	mockHandler := newMockOutputHandler()
	mockHandler.shouldBackpressure = true // Handler returns false

	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "test-session",
		Handler:    mockHandler,
	}

	forwarder := NewPTYForwarder(cfg)

	// Buffer data
	forwarder.bufferOutput([]byte("test"))

	// Flush should set backpressure when handler returns false
	forwarder.flush()

	if !forwarder.isBackpressure() {
		t.Error("backpressure should be set when handler returns false")
	}
}

func TestPTYForwarderFlushClearsBackpressureOnSuccess(t *testing.T) {
	mockHandler := newMockOutputHandler()
	mockHandler.shouldBackpressure = false // Handler returns true

	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "test-session",
		Handler:    mockHandler,
	}

	forwarder := NewPTYForwarder(cfg)

	// Buffer data
	forwarder.bufferOutput([]byte("test"))

	// Flush should clear backpressure on success
	forwarder.flush()

	if forwarder.isBackpressure() {
		t.Error("backpressure should be cleared on successful send")
	}

	// Buffer should be empty
	if forwarder.GetBufferedSize() != 0 {
		t.Errorf("buffer size = %d, want 0", forwarder.GetBufferedSize())
	}
}

func TestPTYForwarderStopClosesChannel(t *testing.T) {
	mockHandler := newMockOutputHandler()

	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "test-session",
		Handler:    mockHandler,
	}

	forwarder := NewPTYForwarder(cfg)

	// Stop should close done channel
	forwarder.Stop()

	// Verify channel is closed
	select {
	case <-forwarder.done:
		// Channel is closed, as expected
	default:
		t.Error("done channel should be closed after Stop")
	}
}

func TestPTYForwarderBufferHalfFullTriggersFlush(t *testing.T) {
	mockHandler := newMockOutputHandler()

	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "test-session",
		Handler:    mockHandler,
		BufferSize: 100,
	}

	forwarder := NewPTYForwarder(cfg)

	// Write more than half buffer size
	data := make([]byte, 60)
	for i := range data {
		data[i] = byte('A' + (i % 26))
	}
	forwarder.bufferOutput(data)

	// Should have triggered auto-flush
	mockHandler.mu.Lock()
	outputCount := len(mockHandler.outputs)
	mockHandler.mu.Unlock()

	if outputCount != 1 {
		t.Errorf("output count = %d, want 1", outputCount)
	}
}

func TestPTYForwarderConcurrentAccess(t *testing.T) {
	mockHandler := newMockOutputHandler()

	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "test-session",
		Handler:    mockHandler,
		BufferSize: 1024 * 64,
	}

	forwarder := NewPTYForwarder(cfg)

	// Concurrent writes
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				forwarder.bufferOutput([]byte("data"))
				forwarder.flush()
			}
		}(i)
	}

	wg.Wait()

	// Should not panic or deadlock
}

func TestPTYForwarderBackpressureConcurrentAccess(t *testing.T) {
	mockHandler := newMockOutputHandler()

	cfg := &PTYForwarderConfig{
		Session:    nil,
		SessionKey: "test-session",
		Handler:    mockHandler,
	}

	forwarder := NewPTYForwarder(cfg)

	// Concurrent reads and writes to backpressure state
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				forwarder.setBackpressure(true)
				forwarder.setBackpressure(false)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = forwarder.isBackpressure()
			}
		}()
	}

	wg.Wait()

	// Should not panic or have race conditions
}
