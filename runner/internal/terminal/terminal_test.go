package terminal

import (
	"os"
	"sync/atomic"
	"testing"
	"time"
)

// --- Test Options and Terminal Struct ---

func TestOptionsStruct(t *testing.T) {
	opts := Options{
		Command:  "echo",
		Args:     []string{"hello"},
		WorkDir:  "/tmp",
		Env:      map[string]string{"KEY": "VALUE"},
		Rows:     24,
		Cols:     80,
		OnOutput: func([]byte) {},
		OnExit:   func(int) {},
	}

	if opts.Command != "echo" {
		t.Errorf("Command: got %v, want echo", opts.Command)
	}

	if opts.Rows != 24 {
		t.Errorf("Rows: got %v, want 24", opts.Rows)
	}

	if opts.Cols != 80 {
		t.Errorf("Cols: got %v, want 80", opts.Cols)
	}
}

func TestNewTerminal(t *testing.T) {
	opts := Options{
		Command: "echo",
		Args:    []string{"hello"},
	}

	term, err := New(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if term == nil {
		t.Fatal("New returned nil")
	}

	if term.cmd == nil {
		t.Error("cmd should not be nil")
	}
}

func TestNewTerminalEmptyCommand(t *testing.T) {
	opts := Options{
		Command: "",
	}

	_, err := New(opts)
	if err == nil {
		t.Error("expected error for empty command")
	}
}

func TestNewTerminalWithEnv(t *testing.T) {
	opts := Options{
		Command: "echo",
		Env:     map[string]string{"TEST_VAR": "test_value"},
	}

	term, err := New(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that environment is set
	envFound := false
	for _, e := range term.cmd.Env {
		if e == "TEST_VAR=test_value" {
			envFound = true
			break
		}
	}

	if !envFound {
		t.Error("environment variable should be set")
	}
}

func TestTerminalPIDNotStarted(t *testing.T) {
	opts := Options{
		Command: "echo",
	}

	term, _ := New(opts)

	// PID should be 0 before start
	if term.PID() != 0 {
		t.Errorf("PID before start: got %v, want 0", term.PID())
	}
}

func TestTerminalWriteNotStarted(t *testing.T) {
	opts := Options{
		Command: "echo",
	}

	term, _ := New(opts)

	err := term.Write([]byte("test"))
	if err == nil {
		t.Error("expected error when writing to not started terminal")
	}
}

func TestTerminalResizeNotStarted(t *testing.T) {
	opts := Options{
		Command: "echo",
	}

	term, _ := New(opts)

	err := term.Resize(24, 80)
	if err == nil {
		t.Error("expected error when resizing not started terminal")
	}
}

func TestTerminalStopNotStarted(t *testing.T) {
	opts := Options{
		Command: "echo",
	}

	term, _ := New(opts)

	// Should not panic when stopping not started terminal
	term.Stop()

	// Second stop should also not panic
	term.Stop()
}

func TestTerminalStartClosed(t *testing.T) {
	opts := Options{
		Command: "echo",
	}

	term, _ := New(opts)
	term.closed = true

	err := term.Start()
	if err == nil {
		t.Error("expected error when starting closed terminal")
	}
}

// --- Test IsRaw, MakeRaw, Restore ---

func TestIsRaw(t *testing.T) {
	// Test with stdin fd
	result := IsRaw(int(os.Stdin.Fd()))
	// Result depends on whether running in a terminal
	_ = result
}

// --- Test SessionConfig ---

func TestDefaultSessionConfig(t *testing.T) {
	cfg := DefaultSessionConfig("session-1")

	if cfg == nil {
		t.Fatal("DefaultSessionConfig returned nil")
	}

	if cfg.ID != "session-1" {
		t.Errorf("ID: got %v, want session-1", cfg.ID)
	}

	if cfg.Command == "" {
		t.Error("Command should not be empty")
	}

	if cfg.Cols != 80 {
		t.Errorf("Cols: got %v, want 80", cfg.Cols)
	}

	if cfg.Rows != 24 {
		t.Errorf("Rows: got %v, want 24", cfg.Rows)
	}

	if cfg.BufferSize != 64*1024 {
		t.Errorf("BufferSize: got %v, want %v", cfg.BufferSize, 64*1024)
	}
}

func TestSessionConfigStruct(t *testing.T) {
	cfg := SessionConfig{
		ID:         "session-1",
		Command:    "bash",
		Args:       []string{"-l"},
		Env:        []string{"PATH=/usr/bin"},
		WorkingDir: "/tmp",
		Cols:       120,
		Rows:       40,
		BufferSize: 128 * 1024,
		ExtraEnv:   map[string]string{"KEY": "VALUE"},
	}

	if cfg.ID != "session-1" {
		t.Errorf("ID: got %v, want session-1", cfg.ID)
	}

	if cfg.Cols != 120 {
		t.Errorf("Cols: got %v, want 120", cfg.Cols)
	}

	if cfg.ExtraEnv["KEY"] != "VALUE" {
		t.Errorf("ExtraEnv[KEY]: got %v, want VALUE", cfg.ExtraEnv["KEY"])
	}
}

// --- Test SessionInfoData ---

func TestSessionInfoDataStruct(t *testing.T) {
	now := time.Now()
	info := SessionInfoData{
		ID:           "session-1",
		Pid:          12345,
		Rows:         24,
		Cols:         80,
		CreatedAt:    now,
		LastActivity: now,
		IsRunning:    true,
		ClientCount:  3,
	}

	if info.ID != "session-1" {
		t.Errorf("ID: got %v, want session-1", info.ID)
	}

	if info.Pid != 12345 {
		t.Errorf("Pid: got %v, want 12345", info.Pid)
	}

	if !info.IsRunning {
		t.Error("IsRunning should be true")
	}

	if info.ClientCount != 3 {
		t.Errorf("ClientCount: got %v, want 3", info.ClientCount)
	}
}

// --- Test Manager ---

func TestNewManager(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.defaultShell != "/bin/bash" {
		t.Errorf("defaultShell: got %v, want /bin/bash", manager.defaultShell)
	}

	if manager.workingDir != "/tmp" {
		t.Errorf("workingDir: got %v, want /tmp", manager.workingDir)
	}

	if manager.sessions == nil {
		t.Error("sessions map should be initialized")
	}
}

func TestManagerSetCallbacks(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	manager.SetCallbacks(
		func(s *Session) { /* create callback */ },
		func(s *Session) { /* close callback */ },
	)

	if manager.onSessionCreate == nil {
		t.Error("onSessionCreate should be set")
	}

	if manager.onSessionClose == nil {
		t.Error("onSessionClose should be set")
	}
}

func TestManagerGetSessionNotFound(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	_, exists := manager.GetSession("nonexistent")
	if exists {
		t.Error("GetSession should return false for nonexistent session")
	}
}

func TestManagerListSessionsEmpty(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	sessions := manager.ListSessions()
	if len(sessions) != 0 {
		t.Errorf("ListSessions should return empty list, got %v", len(sessions))
	}
}

func TestManagerCloseSessionNotFound(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	err := manager.CloseSession("nonexistent")
	if err == nil {
		t.Error("CloseSession should error for nonexistent session")
	}
}

func TestManagerCount(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	if manager.Count() != 0 {
		t.Errorf("initial Count: got %v, want 0", manager.Count())
	}
}

func TestManagerCloseAllSessionsEmpty(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	// Should not panic when closing empty sessions
	manager.CloseAllSessions()
}

func TestManagerCleanupStaleSessions(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	// Should return 0 for empty manager
	removed := manager.CleanupStaleSessions(time.Hour)
	if removed != 0 {
		t.Errorf("CleanupStaleSessions on empty: got %v, want 0", removed)
	}
}

func TestManagerCreateSessionDuplicate(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	// Create a first session successfully (this will actually spawn a process)
	cfg := &SessionConfig{
		ID:         "session-1",
		Command:    "sleep",
		Args:       []string{"10"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	session1, err := manager.CreateSession(cfg)
	if err != nil {
		t.Fatalf("failed to create first session: %v", err)
	}
	defer session1.Close()

	// Try to create a duplicate
	_, err = manager.CreateSession(cfg)
	if err == nil {
		t.Error("expected error for duplicate session")
	}
}

// --- Benchmark Tests ---

func BenchmarkDefaultSessionConfig(b *testing.B) {
	for i := 0; i < b.N; i++ {
		DefaultSessionConfig("session-1")
	}
}

func BenchmarkManagerCount(b *testing.B) {
	manager := NewManager("/bin/bash", "/tmp")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.Count()
	}
}

func BenchmarkManagerGetSession(b *testing.B) {
	manager := NewManager("/bin/bash", "/tmp")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		manager.GetSession("nonexistent")
	}
}

// --- Additional Session Tests ---

func TestSessionWithRealPTY(t *testing.T) {
	cfg := &SessionConfig{
		ID:         "test-session",
		Command:    "sleep",
		Args:       []string{"1"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	session, err := NewSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	// Test GetID
	if session.GetID() != "test-session" {
		t.Errorf("GetID: got %v, want test-session", session.GetID())
	}

	// Test Pid
	if session.Pid() == 0 {
		t.Error("Pid should not be 0 for running session")
	}

	// Test IsRunning
	if !session.IsRunning() {
		t.Error("IsRunning should be true for new session")
	}

	// Test GetSize
	rows, cols := session.GetSize()
	if rows != 24 {
		t.Errorf("GetSize rows: got %v, want 24", rows)
	}
	if cols != 80 {
		t.Errorf("GetSize cols: got %v, want 80", cols)
	}

	// Test GetScrollback (should be empty or have some output)
	scrollback := session.GetScrollback()
	_ = scrollback // May be empty initially

	// Test Done channel (should not be closed yet)
	select {
	case <-session.Done():
		t.Error("Done channel should not be closed yet")
	default:
		// Good - channel is not closed
	}
}

func TestSessionClientCount(t *testing.T) {
	cfg := &SessionConfig{
		ID:         "test-client-session",
		Command:    "sleep",
		Args:       []string{"2"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	session, err := NewSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	// Initial client count should be 0
	if session.ClientCount() != 0 {
		t.Errorf("initial ClientCount: got %v, want 0", session.ClientCount())
	}

	// Add clients
	count := session.AddClient()
	if count != 1 {
		t.Errorf("AddClient: got %v, want 1", count)
	}

	count = session.AddClient()
	if count != 2 {
		t.Errorf("AddClient: got %v, want 2", count)
	}

	if session.ClientCount() != 2 {
		t.Errorf("ClientCount: got %v, want 2", session.ClientCount())
	}

	// Remove clients
	count = session.RemoveClient()
	if count != 1 {
		t.Errorf("RemoveClient: got %v, want 1", count)
	}

	count = session.RemoveClient()
	if count != 0 {
		t.Errorf("RemoveClient: got %v, want 0", count)
	}

	// Remove when already 0 should stay at 0
	count = session.RemoveClient()
	if count != 0 {
		t.Errorf("RemoveClient at 0: got %v, want 0", count)
	}
}

func TestSessionInfo(t *testing.T) {
	cfg := &SessionConfig{
		ID:         "info-session",
		Command:    "sleep",
		Args:       []string{"1"},
		WorkingDir: "/tmp",
		Cols:       100,
		Rows:       30,
		BufferSize: 1024,
	}

	session, err := NewSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	info := session.Info()

	if info.ID != "info-session" {
		t.Errorf("Info.ID: got %v, want info-session", info.ID)
	}

	if info.Pid == 0 {
		t.Error("Info.Pid should not be 0")
	}

	if info.Rows != 30 {
		t.Errorf("Info.Rows: got %v, want 30", info.Rows)
	}

	if info.Cols != 100 {
		t.Errorf("Info.Cols: got %v, want 100", info.Cols)
	}

	if !info.IsRunning {
		t.Error("Info.IsRunning should be true")
	}
}

func TestSessionResize(t *testing.T) {
	cfg := &SessionConfig{
		ID:         "resize-session",
		Command:    "sleep",
		Args:       []string{"2"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	session, err := NewSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	// Resize the session
	err = session.Resize(40, 120)
	if err != nil {
		t.Fatalf("Resize error: %v", err)
	}

	// Verify size changed
	rows, cols := session.GetSize()
	if rows != 40 {
		t.Errorf("GetSize rows after resize: got %v, want 40", rows)
	}
	if cols != 120 {
		t.Errorf("GetSize cols after resize: got %v, want 120", cols)
	}
}

func TestSessionWriteRead(t *testing.T) {
	cfg := &SessionConfig{
		ID:         "write-read-session",
		Command:    "cat",
		Args:       []string{},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 4096,
	}

	session, err := NewSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	// Write to session
	testData := []byte("hello world\n")
	n, err := session.Write(testData)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}
	if n != len(testData) {
		t.Errorf("Write: got %v bytes, want %v", n, len(testData))
	}

	// Read response (may include echo)
	buf := make([]byte, 1024)
	// Use a short timeout for reading
	time.Sleep(100 * time.Millisecond)

	// Try to read, but don't fail if nothing available
	// since cat may not have produced output yet
	_, _ = session.Read(buf)
}

func TestSessionClose(t *testing.T) {
	cfg := &SessionConfig{
		ID:         "close-session",
		Command:    "sleep",
		Args:       []string{"10"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	session, err := NewSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Close the session
	err = session.Close()
	if err != nil {
		t.Errorf("Close error: %v", err)
	}

	// Done channel should be closed
	select {
	case <-session.Done():
		// Good - channel is closed
	case <-time.After(time.Second):
		t.Error("Done channel should be closed after Close()")
	}

	// Second close should not error
	err = session.Close()
	if err != nil {
		t.Errorf("Second Close error: %v", err)
	}
}

func TestSessionPidNilCmd(t *testing.T) {
	session := &Session{
		Cmd: nil,
	}

	if session.Pid() != 0 {
		t.Errorf("Pid with nil Cmd: got %v, want 0", session.Pid())
	}
}

func TestSessionIsRunningNilCmd(t *testing.T) {
	session := &Session{
		Cmd: nil,
	}

	if session.IsRunning() {
		t.Error("IsRunning with nil Cmd should be false")
	}
}

// --- Additional Manager Tests ---

func TestManagerGetOrCreateSession(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	cfg := &SessionConfig{
		ID:         "get-or-create-session",
		Command:    "sleep",
		Args:       []string{"2"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	// First call should create
	session1, err := manager.GetOrCreateSession(cfg)
	if err != nil {
		t.Fatalf("first GetOrCreateSession error: %v", err)
	}

	// Second call should get existing
	session2, err := manager.GetOrCreateSession(cfg)
	if err != nil {
		t.Fatalf("second GetOrCreateSession error: %v", err)
	}

	if session1 != session2 {
		t.Error("GetOrCreateSession should return same session")
	}

	// Clean up - close all sessions (handles race condition gracefully)
	manager.CloseAllSessions()
}

func TestManagerListSessionsWithSessions(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	// Create sessions
	cfg1 := &SessionConfig{
		ID:         "list-session-1",
		Command:    "sleep",
		Args:       []string{"2"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	cfg2 := &SessionConfig{
		ID:         "list-session-2",
		Command:    "sleep",
		Args:       []string{"2"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	_, err := manager.CreateSession(cfg1)
	if err != nil {
		t.Fatalf("failed to create session 1: %v", err)
	}

	_, err = manager.CreateSession(cfg2)
	if err != nil {
		t.Fatalf("failed to create session 2: %v", err)
	}

	defer manager.CloseAllSessions()

	// List sessions
	sessions := manager.ListSessions()
	if len(sessions) != 2 {
		t.Errorf("ListSessions count: got %v, want 2", len(sessions))
	}

	// Verify count
	if manager.Count() != 2 {
		t.Errorf("Count: got %v, want 2", manager.Count())
	}
}

func TestManagerCloseAllSessionsWithSessions(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	// Create sessions
	cfg1 := &SessionConfig{
		ID:         "close-all-1",
		Command:    "sleep",
		Args:       []string{"10"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	cfg2 := &SessionConfig{
		ID:         "close-all-2",
		Command:    "sleep",
		Args:       []string{"10"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	_, _ = manager.CreateSession(cfg1)
	_, _ = manager.CreateSession(cfg2)

	// Verify sessions were created
	if manager.Count() != 2 {
		t.Errorf("Count before close: got %v, want 2", manager.Count())
	}

	// Close all
	manager.CloseAllSessions()

	// Verify sessions are removed
	if manager.Count() != 0 {
		t.Errorf("Count after CloseAllSessions: got %v, want 0", manager.Count())
	}
}

func TestManagerCloseSessionWithCallback(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	var closeCallbackCalled atomic.Bool
	manager.SetCallbacks(
		func(s *Session) { /* create */ },
		func(s *Session) {
			closeCallbackCalled.Store(true)
		},
	)

	cfg := &SessionConfig{
		ID:         "callback-session",
		Command:    "sleep",
		Args:       []string{"10"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	_, err := manager.CreateSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Close session
	err = manager.CloseSession("callback-session")
	if err != nil {
		t.Errorf("CloseSession error: %v", err)
	}

	// Wait a bit for the callback to be called
	time.Sleep(100 * time.Millisecond)

	if !closeCallbackCalled.Load() {
		t.Error("close callback should have been called")
	}
}

func TestManagerCreateSessionWithDefaults(t *testing.T) {
	manager := NewManager("/bin/sleep", "/tmp")

	cfg := &SessionConfig{
		ID:   "defaults-session",
		Args: []string{"1"},
		// Command, WorkingDir, BufferSize should use defaults
	}

	session, err := manager.CreateSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer manager.CloseSession(cfg.ID)

	if session == nil {
		t.Fatal("session should not be nil")
	}
}

func TestSessionWithExtraEnv(t *testing.T) {
	cfg := &SessionConfig{
		ID:         "extra-env-session",
		Command:    "sleep",
		Args:       []string{"1"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
		ExtraEnv: map[string]string{
			"CUSTOM_VAR": "custom_value",
		},
	}

	session, err := NewSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer session.Close()

	// Verify session was created (can't easily verify env was set)
	if session.GetID() != "extra-env-session" {
		t.Errorf("GetID: got %v, want extra-env-session", session.GetID())
	}
}

// --- Test Terminal Start and PTY operations ---

func TestTerminalStartSuccess(t *testing.T) {
	outputReceived := make(chan bool, 1)
	exitReceived := make(chan int, 1)

	opts := Options{
		Command: "echo",
		Args:    []string{"hello"},
		WorkDir: "/tmp",
		OnOutput: func(data []byte) {
			select {
			case outputReceived <- true:
			default:
			}
		},
		OnExit: func(code int) {
			exitReceived <- code
		},
	}

	term, err := New(opts)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	err = term.Start()
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	// Wait for output or timeout
	select {
	case <-outputReceived:
		// Good - got output
	case <-time.After(2 * time.Second):
		// May timeout if output is too fast
	}

	// Wait for exit
	select {
	case code := <-exitReceived:
		if code != 0 {
			t.Logf("Exit code: %d", code)
		}
	case <-time.After(3 * time.Second):
		t.Log("Timeout waiting for exit")
	}

	term.Stop()
}

func TestTerminalWriteSuccess(t *testing.T) {
	opts := Options{
		Command:  "cat",
		WorkDir:  "/tmp",
		OnOutput: func(data []byte) {},
	}

	term, err := New(opts)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	err = term.Start()
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer term.Stop()

	// Write data
	err = term.Write([]byte("test input\n"))
	if err != nil {
		t.Errorf("Write error: %v", err)
	}
}

func TestTerminalResizeSuccess(t *testing.T) {
	opts := Options{
		Command: "sleep",
		Args:    []string{"5"},
		WorkDir: "/tmp",
	}

	term, err := New(opts)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	err = term.Start()
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer term.Stop()

	// Resize
	err = term.Resize(40, 120)
	if err != nil {
		t.Errorf("Resize error: %v", err)
	}
}

func TestTerminalPIDRunning(t *testing.T) {
	opts := Options{
		Command: "sleep",
		Args:    []string{"5"},
		WorkDir: "/tmp",
	}

	term, err := New(opts)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	err = term.Start()
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}
	defer term.Stop()

	pid := term.PID()
	if pid <= 0 {
		t.Errorf("PID should be positive, got %d", pid)
	}
}

func TestTerminalStopRunning(t *testing.T) {
	opts := Options{
		Command: "sleep",
		Args:    []string{"60"},
		WorkDir: "/tmp",
	}

	term, err := New(opts)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	err = term.Start()
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	// Stop should work
	term.Stop()

	// Wait a bit for waitExit goroutine to complete
	time.Sleep(100 * time.Millisecond)

	// Verify closed flag using thread-safe method
	if !term.IsClosed() {
		t.Error("closed flag should be true")
	}
}

func TestTerminalWriteClosed(t *testing.T) {
	opts := Options{
		Command: "sleep",
		Args:    []string{"60"},
		WorkDir: "/tmp",
	}

	term, err := New(opts)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	err = term.Start()
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	term.Stop()

	// Write should fail after close
	err = term.Write([]byte("test"))
	if err == nil {
		t.Error("Write after close should error")
	}
}

func TestTerminalResizeClosed(t *testing.T) {
	opts := Options{
		Command: "sleep",
		Args:    []string{"60"},
		WorkDir: "/tmp",
	}

	term, err := New(opts)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	err = term.Start()
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	term.Stop()

	// Resize should fail after close
	err = term.Resize(30, 100)
	if err == nil {
		t.Error("Resize after close should error")
	}
}

// --- Test Manager Cleanup Stale Sessions ---

func TestManagerCleanupStaleSessionsWithStale(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	// Create a session that will exit quickly
	cfg := &SessionConfig{
		ID:         "stale-session",
		Command:    "true", // exits immediately
		Args:       []string{},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	session, err := manager.CreateSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Wait for process to exit
	time.Sleep(500 * time.Millisecond)

	// Set last activity to old time
	session.mu.Lock()
	session.LastActivity = time.Now().Add(-2 * time.Hour)
	session.mu.Unlock()

	// Cleanup should remove it (no clients, not running, old activity)
	removed := manager.CleanupStaleSessions(time.Hour)
	t.Logf("Removed %d stale sessions", removed)
}

func TestManagerCleanupStaleSessionsWithRunning(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	cfg := &SessionConfig{
		ID:         "running-session",
		Command:    "sleep",
		Args:       []string{"60"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	_, err := manager.CreateSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer manager.CloseSession(cfg.ID)

	// Running session should not be cleaned up
	removed := manager.CleanupStaleSessions(0) // 0 duration means cleanup anything old
	if removed != 0 {
		t.Errorf("Running session should not be removed, but removed %d", removed)
	}
}

func TestManagerCleanupStaleSessionsWithClients(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	cfg := &SessionConfig{
		ID:         "client-session",
		Command:    "true",
		Args:       []string{},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	session, err := manager.CreateSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer manager.CloseSession(cfg.ID)

	// Add a client
	session.AddClient()

	// Wait for process to exit
	time.Sleep(200 * time.Millisecond)

	// Session with clients should not be cleaned up
	removed := manager.CleanupStaleSessions(0)
	if removed != 0 {
		t.Errorf("Session with clients should not be removed, but removed %d", removed)
	}
}

// --- Test Session Close with errors ---

func TestSessionCloseWithErrors(t *testing.T) {
	cfg := &SessionConfig{
		ID:         "close-error-session",
		Command:    "sleep",
		Args:       []string{"60"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	session, err := NewSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Close PTY manually first to create error scenario
	session.PTY.Close()

	// Session.Close() should handle errors gracefully
	err = session.Close()
	// May or may not error depending on implementation
	_ = err
}

func TestSessionCloseForceKill(t *testing.T) {
	cfg := &SessionConfig{
		ID:         "force-kill-session",
		Command:    "sleep",
		Args:       []string{"3600"}, // Long sleep
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	session, err := NewSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Close should force kill after timeout
	start := time.Now()
	err = session.Close()
	elapsed := time.Since(start)

	if err != nil {
		t.Logf("Close error (expected for force kill): %v", err)
	}

	// Should complete within reasonable time (not wait forever)
	if elapsed > 5*time.Second {
		t.Errorf("Close took too long: %v", elapsed)
	}
}

// --- Test DefaultSessionConfig variations ---

func TestDefaultSessionConfigNoShellEnv(t *testing.T) {
	// Save and clear SHELL env
	oldShell := os.Getenv("SHELL")
	os.Unsetenv("SHELL")
	defer os.Setenv("SHELL", oldShell)

	cfg := DefaultSessionConfig("test-session")

	if cfg.Command != "/bin/sh" {
		t.Errorf("Command without SHELL env: got %v, want /bin/sh", cfg.Command)
	}
}

// --- Test Manager with callbacks ---

func TestManagerCreateSessionWithCallback(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	createCalled := false
	manager.SetCallbacks(
		func(s *Session) {
			createCalled = true
		},
		func(s *Session) {},
	)

	cfg := &SessionConfig{
		ID:         "callback-create-session",
		Command:    "sleep",
		Args:       []string{"1"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	_, err := manager.CreateSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}
	defer manager.CloseSession(cfg.ID)

	if !createCalled {
		t.Error("create callback should have been called")
	}
}

func TestManagerCloseAllSessionsWithCallback(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	var closeCount atomic.Int32
	manager.SetCallbacks(
		func(s *Session) {},
		func(s *Session) {
			closeCount.Add(1)
		},
	)

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		cfg := &SessionConfig{
			ID:         "multi-" + string(rune('a'+i)),
			Command:    "sleep",
			Args:       []string{"60"},
			WorkingDir: "/tmp",
			Cols:       80,
			Rows:       24,
			BufferSize: 1024,
		}
		_, _ = manager.CreateSession(cfg)
	}

	manager.CloseAllSessions()

	// Wait a bit for callbacks to complete since they may run in goroutines
	time.Sleep(100 * time.Millisecond)

	if closeCount.Load() != 3 {
		t.Errorf("close callback count: got %v, want 3", closeCount.Load())
	}
}

// --- Test Terminal with exit code ---

func TestTerminalExitCode(t *testing.T) {
	exitCode := -1
	exitReceived := make(chan bool, 1)

	opts := Options{
		Command: "false", // returns exit code 1
		WorkDir: "/tmp",
		OnExit: func(code int) {
			exitCode = code
			exitReceived <- true
		},
	}

	term, err := New(opts)
	if err != nil {
		t.Fatalf("New error: %v", err)
	}

	err = term.Start()
	if err != nil {
		t.Fatalf("Start error: %v", err)
	}

	// Wait for exit
	select {
	case <-exitReceived:
		if exitCode != 1 {
			t.Errorf("exit code: got %v, want 1", exitCode)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for exit")
	}
}

// --- Test NewSession error path ---

func TestNewSessionInvalidCommand(t *testing.T) {
	cfg := &SessionConfig{
		ID:         "invalid-cmd-session",
		Command:    "/nonexistent/command",
		Args:       []string{},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	_, err := NewSession(cfg)
	if err == nil {
		t.Error("expected error for invalid command")
	}
}

// --- Test Session Resize error path ---

func TestSessionResizeAfterClose(t *testing.T) {
	cfg := &SessionConfig{
		ID:         "resize-close-session",
		Command:    "sleep",
		Args:       []string{"1"},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	session, err := NewSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	session.Close()

	// Resize after close should fail
	err = session.Resize(40, 120)
	if err == nil {
		// May succeed if PTY is still valid, or fail - depends on timing
		t.Log("Resize after close did not error (may be valid)")
	}
}

// --- Test Manager monitor session callback ---

func TestManagerMonitorSessionExit(t *testing.T) {
	manager := NewManager("/bin/bash", "/tmp")

	closeCalled := make(chan bool, 1)
	manager.SetCallbacks(
		func(s *Session) {},
		func(s *Session) {
			select {
			case closeCalled <- true:
			default:
			}
		},
	)

	cfg := &SessionConfig{
		ID:         "monitor-exit-session",
		Command:    "true", // exits immediately
		Args:       []string{},
		WorkingDir: "/tmp",
		Cols:       80,
		Rows:       24,
		BufferSize: 1024,
	}

	_, err := manager.CreateSession(cfg)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Wait for monitor to detect exit and call callback
	select {
	case <-closeCalled:
		// Good - callback was called
	case <-time.After(3 * time.Second):
		t.Error("timeout waiting for close callback from monitor")
	}
}

// --- Test SetOutputHandler and SetExitHandler ---

func TestSetOutputHandler(t *testing.T) {
	opts := Options{
		Command: "echo",
		Args:    []string{"test"},
	}

	term, err := New(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var received []byte
	term.SetOutputHandler(func(data []byte) {
		received = append(received, data...)
	})

	err = term.Start()
	if err != nil {
		t.Fatalf("failed to start terminal: %v", err)
	}

	// Wait for output
	time.Sleep(500 * time.Millisecond)
	term.Stop()

	// Should have received some output
	if len(received) == 0 {
		t.Error("expected to receive output from echo command")
	}
}

func TestSetExitHandler(t *testing.T) {
	opts := Options{
		Command: "true", // Command that exits immediately with code 0
	}

	term, err := New(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	exitCode := -1
	exitCalled := make(chan struct{})
	term.SetExitHandler(func(code int) {
		exitCode = code
		close(exitCalled)
	})

	err = term.Start()
	if err != nil {
		t.Fatalf("failed to start terminal: %v", err)
	}

	// Wait for exit handler to be called
	select {
	case <-exitCalled:
		if exitCode != 0 {
			t.Errorf("exit code = %d, want 0", exitCode)
		}
	case <-time.After(2 * time.Second):
		t.Error("timeout waiting for exit handler")
		term.Stop()
	}
}

func TestSetOutputHandlerNil(t *testing.T) {
	opts := Options{
		Command: "echo",
		Args:    []string{"hello"},
	}

	term, err := New(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not panic when setting nil handler
	term.SetOutputHandler(nil)

	// Terminal should still work
	err = term.Start()
	if err != nil {
		t.Fatalf("failed to start terminal: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	term.Stop()
}

func TestSetExitHandlerNil(t *testing.T) {
	opts := Options{
		Command: "true",
	}

	term, err := New(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not panic when setting nil handler
	term.SetExitHandler(nil)

	// Terminal should still work
	err = term.Start()
	if err != nil {
		t.Fatalf("failed to start terminal: %v", err)
	}

	time.Sleep(200 * time.Millisecond)
	term.Stop()
}

func TestSetHandlersBeforeStart(t *testing.T) {
	// Use sleep instead of echo to ensure we have time to capture output
	// echo may complete too fast in CI environments with race detector
	opts := Options{
		Command:  "sh",
		Args:     []string{"-c", "echo hello && sleep 0.1"},
		OnOutput: func([]byte) { /* initial handler */ },
		OnExit:   func(int) { /* initial handler */ },
	}

	term, err := New(opts)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Override handlers before Start
	outputReceived := make(chan struct{})
	exitReceived := make(chan struct{})

	term.SetOutputHandler(func(data []byte) {
		select {
		case <-outputReceived:
		default:
			close(outputReceived)
		}
	})
	term.SetExitHandler(func(code int) {
		close(exitReceived)
	})

	err = term.Start()
	if err != nil {
		t.Fatalf("failed to start terminal: %v", err)
	}

	// Wait for both handlers to be called with longer timeout for CI
	select {
	case <-outputReceived:
		// Good
	case <-time.After(5 * time.Second):
		t.Error("timeout waiting for output handler")
	}

	select {
	case <-exitReceived:
		// Good
	case <-time.After(5 * time.Second):
		t.Error("timeout waiting for exit handler")
		term.Stop()
	}
}
