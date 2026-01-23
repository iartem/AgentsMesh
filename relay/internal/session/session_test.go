package session

import (
	"testing"
	"time"
)

func TestNewTerminalSession(t *testing.T) {
	session := NewTerminalSession("session-1", "pod-1", 30*time.Second, nil, nil)

	if session.ID != "session-1" || session.PodKey != "pod-1" {
		t.Error("session ID or PodKey mismatch")
	}
	if session.config.KeepAliveDuration != 30*time.Second {
		t.Errorf("KeepAliveDuration: expected %v, got %v", 30*time.Second, session.config.KeepAliveDuration)
	}
	if session.IsClosed() {
		t.Error("new session should not be closed")
	}
}

func TestTerminalSession_BrowserCount(t *testing.T) {
	session := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)
	if session.BrowserCount() != 0 {
		t.Errorf("expected 0, got %d", session.BrowserCount())
	}

	session.browsersMu.Lock()
	session.browsers["b1"] = &BrowserConn{ID: "b1"}
	session.browsers["b2"] = &BrowserConn{ID: "b2"}
	session.browsersMu.Unlock()

	if session.BrowserCount() != 2 {
		t.Errorf("expected 2, got %d", session.BrowserCount())
	}
}

func TestTerminalSession_RunnerState(t *testing.T) {
	session := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)

	if session.IsRunnerDisconnected() {
		t.Error("new session should not have runner disconnected")
	}
	if session.GetRunnerConn() != nil {
		t.Error("GetRunnerConn should return nil")
	}

	session.runnerMu.Lock()
	session.runnerDisconnected = true
	session.runnerMu.Unlock()

	if !session.IsRunnerDisconnected() {
		t.Error("should be disconnected")
	}
}

func TestTerminalSession_InputControl(t *testing.T) {
	session := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)

	// No controller - anyone can input
	if !session.CanInput("b1") || !session.CanInput("b2") {
		t.Error("anyone should input when no controller")
	}

	// Request control
	if !session.RequestControl("b1") {
		t.Error("first request should succeed")
	}
	if session.RequestControl("b2") {
		t.Error("second request should fail")
	}

	// Verify control
	if !session.CanInput("b1") {
		t.Error("controller should input")
	}
	if session.CanInput("b2") {
		t.Error("non-controller should not input")
	}

	// Release control
	session.ReleaseControl("b1")
	if !session.RequestControl("b2") {
		t.Error("should succeed after release")
	}
}

func TestTerminalSession_BufferOutput(t *testing.T) {
	session := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)
	session.bufferOutput([]byte("out1"))
	session.bufferOutput([]byte("out2"))

	buffered := session.getBufferedOutput()
	if len(buffered) != 2 || string(buffered[0]) != "out1" {
		t.Errorf("buffer mismatch: got %d items", len(buffered))
	}
}

func TestTerminalSession_BufferOutput_MaxCount(t *testing.T) {
	cfg := DefaultSessionConfig()
	cfg.OutputBufferCount = 3
	session := NewTerminalSessionWithConfig("s1", "p1", cfg, nil, nil)

	for i := 0; i < 5; i++ {
		session.bufferOutput([]byte{byte(i)})
	}

	buffered := session.getBufferedOutput()
	if len(buffered) != 3 || buffered[0][0] != 2 {
		t.Errorf("expected 3 items starting with 2, got %d", len(buffered))
	}
}

func TestTerminalSession_BufferOutput_MaxBytes(t *testing.T) {
	cfg := DefaultSessionConfig()
	cfg.OutputBufferCount = 100
	cfg.OutputBufferSize = 30
	session := NewTerminalSessionWithConfig("s1", "p1", cfg, nil, nil)

	session.bufferOutput([]byte("12345678901234567890")) // 20 bytes
	session.bufferOutput([]byte("12345678901234567890")) // 20 bytes (evicts first)

	buffered := session.getBufferedOutput()
	total := 0
	for _, b := range buffered {
		total += len(b)
	}
	if total > 30 {
		t.Errorf("total bytes %d exceeds max 30", total)
	}
}

func TestTerminalSession_Close(t *testing.T) {
	closedCalled := false
	session := NewTerminalSession("s1", "p1", 30*time.Second, nil, func(id string) {
		closedCalled = true
	})

	session.Close()
	if !session.IsClosed() || !closedCalled {
		t.Error("close failed")
	}

	// Idempotent
	session.Close()
}

func TestTerminalSession_Close_WithTimers(t *testing.T) {
	session := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)

	session.browsersMu.Lock()
	session.keepAliveTimer = time.AfterFunc(time.Hour, func() {})
	session.browsersMu.Unlock()

	session.runnerMu.Lock()
	session.runnerReconnectTimer = time.AfterFunc(time.Hour, func() {})
	session.runnerMu.Unlock()

	session.Close()
	if !session.IsClosed() {
		t.Error("should be closed")
	}
}

func TestTerminalSession_BroadcastToAllBrowsers_NoBrowsers(t *testing.T) {
	session := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)
	session.BroadcastToAllBrowsers([]byte("test")) // Should not panic
}

func TestBrowserConn(t *testing.T) {
	bc := &BrowserConn{ID: "b123", Conn: nil}
	if bc.ID != "b123" {
		t.Errorf("ID mismatch: %q", bc.ID)
	}
}
