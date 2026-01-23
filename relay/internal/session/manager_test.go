package session

import (
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNewManager(t *testing.T) {
	m := NewManager(30*time.Second, 10, nil)
	if m == nil || m.config.KeepAliveDuration != 30*time.Second || m.config.MaxBrowsersPerPod != 10 {
		t.Error("manager init failed")
	}
	if m.GetSession("x") != nil || m.GetSessionByPodKey("x") != nil {
		t.Error("should return nil")
	}
}

func TestManager_Stats(t *testing.T) {
	m := NewManager(30*time.Second, 10, nil)
	s := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)
	s.browsersMu.Lock()
	s.browsers["b1"] = &BrowserConn{ID: "b1"}
	s.browsersMu.Unlock()
	m.mu.Lock()
	m.sessions["s1"] = s
	m.pendingRunners["pr"] = &pendingRunner{}
	m.pendingBrowsers["pb"] = &pendingBrowser{}
	m.mu.Unlock()
	stats := m.Stats()
	if stats.ActiveSessions != 1 || stats.TotalBrowsers != 1 || stats.PendingRunners != 1 || stats.PendingBrowsers != 1 {
		t.Errorf("stats wrong: %+v", stats)
	}
}

func TestManager_CloseSession(t *testing.T) {
	m := NewManager(30*time.Second, 10, nil)
	closed := false
	s := NewTerminalSession("s1", "p1", 30*time.Second, nil, func(id string) { closed = true })
	m.mu.Lock()
	m.sessions["s1"] = s
	m.mu.Unlock()
	m.CloseSession("s1")
	if m.GetSession("s1") != nil || !closed {
		t.Error("close failed")
	}
	m.CloseSession("x")
}

func TestManager_CloseSessionByPodKey(t *testing.T) {
	m := NewManager(30*time.Second, 10, nil)
	s := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)
	m.mu.Lock()
	m.sessions["s1"] = s
	m.mu.Unlock()
	m.CloseSessionByPodKey("p1")
	if m.GetSessionByPodKey("p1") != nil {
		t.Error("should be closed")
	}
	m.CloseSessionByPodKey("x")
}

func TestManager_HandleRunnerConnect(t *testing.T) {
	m := NewManager(30*time.Second, 10, nil)
	// Pending
	s, _, c := newWSPair()
	defer c()
	if s == nil {
		t.Skip("ws failed")
	}
	m.HandleRunnerConnect("s1", "p1", s)
	if m.Stats().PendingRunners != 1 {
		t.Error("should be pending")
	}
	// Existing session
	m2 := NewManager(30*time.Second, 10, nil)
	sess := NewTerminalSession("s2", "p2", 30*time.Second, nil, nil)
	m2.mu.Lock()
	m2.sessions["s2"] = sess
	m2.mu.Unlock()
	s2, _, c2 := newWSPair()
	defer c2()
	if s2 != nil {
		m2.HandleRunnerConnect("s2", "p2", s2)
		if sess.GetRunnerConn() != s2 {
			t.Error("runner should be set")
		}
	}
	// With pending browser
	m3 := NewManager(30*time.Second, 10, nil)
	bs, _, bc := newWSPair()
	defer bc()
	rs, _, rc := newWSPair()
	defer rc()
	if bs != nil && rs != nil {
		m3.mu.Lock()
		m3.pendingBrowsers["s3"] = &pendingBrowser{conn: bs, browserID: "b1", podKey: "p3", sessionID: "s3", createdAt: time.Now()}
		m3.mu.Unlock()
		m3.HandleRunnerConnect("s3", "p3", rs)
		if m3.Stats().ActiveSessions != 1 {
			t.Error("session should be created")
		}
	}
}

func TestManager_HandleBrowserConnect(t *testing.T) {
	m := NewManager(30*time.Second, 10, nil)
	// Pending
	s, _, c := newWSPair()
	defer c()
	if s == nil {
		t.Skip("ws failed")
	}
	m.HandleBrowserConnect("s1", "p1", "b1", s)
	if m.Stats().PendingBrowsers != 1 {
		t.Error("should be pending")
	}
	// Existing session
	m2 := NewManager(30*time.Second, 10, nil)
	sess := NewTerminalSession("s2", "p2", 30*time.Second, nil, nil)
	m2.mu.Lock()
	m2.sessions["s2"] = sess
	m2.mu.Unlock()
	s2, _, c2 := newWSPair()
	defer c2()
	if s2 != nil {
		m2.HandleBrowserConnect("s2", "p2", "b2", s2)
		if sess.BrowserCount() != 1 {
			t.Error("browser should be added")
		}
	}
	// With pending runner
	m3 := NewManager(30*time.Second, 10, nil)
	rs, _, rc := newWSPair()
	defer rc()
	bs, _, bc := newWSPair()
	defer bc()
	if rs != nil && bs != nil {
		m3.mu.Lock()
		m3.pendingRunners["s3"] = &pendingRunner{conn: rs, podKey: "p3", sessionID: "s3", createdAt: time.Now()}
		m3.mu.Unlock()
		m3.HandleBrowserConnect("s3", "p3", "b3", bs)
		if m3.Stats().ActiveSessions != 1 {
			t.Error("session should be created")
		}
	}
	// Max browsers
	m4 := NewManager(30*time.Second, 1, nil)
	s4 := NewTerminalSession("s4", "p4", 30*time.Second, nil, nil)
	s4.browsersMu.Lock()
	s4.browsers["b4"] = &BrowserConn{ID: "b4"}
	s4.browsersMu.Unlock()
	m4.mu.Lock()
	m4.sessions["s4"] = s4
	m4.mu.Unlock()
	ws, _, wc := newWSPair()
	defer wc()
	if ws != nil {
		if _, ok := m4.HandleBrowserConnect("s4", "p4", "b5", ws).(*MaxBrowsersError); !ok {
			t.Error("expected MaxBrowsersError")
		}
	}
}

func TestManager_GetSessionByPodKey(t *testing.T) {
	m := NewManager(30*time.Second, 10, nil)
	s := NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)
	m.mu.Lock()
	m.sessions["s1"] = s
	m.mu.Unlock()
	if m.GetSessionByPodKey("p1") != s || m.GetSessionByPodKey("x") != nil {
		t.Error("GetSessionByPodKey failed")
	}
}

func TestManager_onSessionClosed(t *testing.T) {
	m := NewManager(30*time.Second, 10, nil)
	m.mu.Lock()
	m.sessions["s1"] = NewTerminalSession("s1", "p1", 30*time.Second, nil, nil)
	m.mu.Unlock()
	m.onSessionClosed("s1")
	if m.GetSession("s1") != nil {
		t.Error("should be removed")
	}
}

func TestMaxBrowsersError(t *testing.T) {
	if (&MaxBrowsersError{Max: 10}).Error() != "maximum browsers per pod reached" {
		t.Error("wrong message")
	}
}

func TestPendingStructs(t *testing.T) {
	pr := &pendingRunner{podKey: "p1", sessionID: "s1", createdAt: time.Now(), conn: &websocket.Conn{}}
	pb := &pendingBrowser{browserID: "b1", podKey: "p2", sessionID: "s2", createdAt: time.Now(), conn: &websocket.Conn{}}
	if pr.podKey != "p1" || pb.browserID != "b1" {
		t.Error("fields wrong")
	}
}
