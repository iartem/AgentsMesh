package runner

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestNewConnectionManager(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	if cm == nil {
		t.Fatal("NewConnectionManager returned nil")
	}
	if cm.connections == nil {
		t.Error("connections map should be initialized")
	}
	if cm.pingInterval != 30*time.Second {
		t.Errorf("pingInterval = %v, want 30s", cm.pingInterval)
	}
	if cm.pingTimeout != 60*time.Second {
		t.Errorf("pingTimeout = %v, want 60s", cm.pingTimeout)
	}
}

func TestConnectionManagerSetCallbacks(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	heartbeatCalled := false
	cm.SetHeartbeatCallback(func(runnerID int64, data *HeartbeatData) {
		heartbeatCalled = true
	})
	if cm.onHeartbeat == nil {
		t.Error("onHeartbeat should be set")
	}

	sessionCreatedCalled := false
	cm.SetSessionCreatedCallback(func(runnerID int64, data *SessionCreatedData) {
		sessionCreatedCalled = true
	})
	if cm.onSessionCreated == nil {
		t.Error("onSessionCreated should be set")
	}

	sessionTerminatedCalled := false
	cm.SetSessionTerminatedCallback(func(runnerID int64, data *SessionTerminatedData) {
		sessionTerminatedCalled = true
	})
	if cm.onSessionTerminated == nil {
		t.Error("onSessionTerminated should be set")
	}

	terminalOutputCalled := false
	cm.SetTerminalOutputCallback(func(runnerID int64, data *TerminalOutputData) {
		terminalOutputCalled = true
	})
	if cm.onTerminalOutput == nil {
		t.Error("onTerminalOutput should be set")
	}

	agentStatusCalled := false
	cm.SetAgentStatusCallback(func(runnerID int64, data *AgentStatusData) {
		agentStatusCalled = true
	})
	if cm.onAgentStatus == nil {
		t.Error("onAgentStatus should be set")
	}

	ptyResizedCalled := false
	cm.SetPtyResizedCallback(func(runnerID int64, data *PtyResizedData) {
		ptyResizedCalled = true
	})
	if cm.onPtyResized == nil {
		t.Error("onPtyResized should be set")
	}

	disconnectCalled := false
	cm.SetDisconnectCallback(func(runnerID int64) {
		disconnectCalled = true
	})
	if cm.onDisconnect == nil {
		t.Error("onDisconnect should be set")
	}

	// Test they are not called yet
	if heartbeatCalled || sessionCreatedCalled || sessionTerminatedCalled ||
		terminalOutputCalled || agentStatusCalled || ptyResizedCalled || disconnectCalled {
		t.Error("callbacks should not be called yet")
	}
}

func TestConnectionManagerIsConnected(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	if cm.IsConnected(1) {
		t.Error("runner 1 should not be connected")
	}

	// Add a mock connection
	cm.mu.Lock()
	cm.connections[1] = &RunnerConnection{RunnerID: 1, Send: make(chan []byte, 256)}
	cm.mu.Unlock()

	if !cm.IsConnected(1) {
		t.Error("runner 1 should be connected")
	}
}

func TestConnectionManagerGetConnection(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	conn := cm.GetConnection(1)
	if conn != nil {
		t.Error("should return nil for nonexistent connection")
	}

	mockConn := &RunnerConnection{RunnerID: 1, Send: make(chan []byte, 256)}
	cm.mu.Lock()
	cm.connections[1] = mockConn
	cm.mu.Unlock()

	conn = cm.GetConnection(1)
	if conn != mockConn {
		t.Error("should return the connection")
	}
}

func TestConnectionManagerUpdateHeartbeat(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	// Update nonexistent - should not panic
	cm.UpdateHeartbeat(1)

	// Add connection and update
	mockConn := &RunnerConnection{RunnerID: 1, Send: make(chan []byte, 256)}
	cm.mu.Lock()
	cm.connections[1] = mockConn
	cm.mu.Unlock()

	before := time.Now()
	cm.UpdateHeartbeat(1)
	after := time.Now()

	if mockConn.LastPing.Before(before) || mockConn.LastPing.After(after) {
		t.Error("LastPing should be updated to current time")
	}
}

func TestConnectionManagerGetConnectedRunnerIDs(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	ids := cm.GetConnectedRunnerIDs()
	if len(ids) != 0 {
		t.Errorf("expected 0 runners, got %d", len(ids))
	}

	cm.mu.Lock()
	cm.connections[1] = &RunnerConnection{RunnerID: 1, Send: make(chan []byte, 256)}
	cm.connections[2] = &RunnerConnection{RunnerID: 2, Send: make(chan []byte, 256)}
	cm.mu.Unlock()

	ids = cm.GetConnectedRunnerIDs()
	if len(ids) != 2 {
		t.Errorf("expected 2 runners, got %d", len(ids))
	}
}

func TestConnectionManagerClose(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	cm.mu.Lock()
	cm.connections[1] = &RunnerConnection{RunnerID: 1, Send: make(chan []byte, 256)}
	cm.connections[2] = &RunnerConnection{RunnerID: 2, Send: make(chan []byte, 256)}
	cm.mu.Unlock()

	cm.Close()

	if len(cm.connections) != 0 {
		t.Errorf("connections should be empty after Close, got %d", len(cm.connections))
	}
}

func TestConnectionManagerRemoveConnection(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	disconnectCalled := false
	cm.SetDisconnectCallback(func(runnerID int64) {
		disconnectCalled = true
	})

	// Remove nonexistent - should not panic
	cm.RemoveConnection(1)
	if disconnectCalled {
		t.Error("disconnect callback should not be called for nonexistent connection")
	}

	// Add and remove
	cm.mu.Lock()
	cm.connections[1] = &RunnerConnection{RunnerID: 1, Send: make(chan []byte, 256)}
	cm.mu.Unlock()

	cm.RemoveConnection(1)
	if !disconnectCalled {
		t.Error("disconnect callback should be called")
	}
	if cm.IsConnected(1) {
		t.Error("connection should be removed")
	}
}

func TestConnectionManagerHandleMessageInvalidType(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	// PingMessage should be ignored
	cm.HandleMessage(1, websocket.PingMessage, []byte{})
}

func TestConnectionManagerHandleMessageInvalidJSON(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	// Should not panic
	cm.HandleMessage(1, websocket.TextMessage, []byte("invalid json"))
}

func TestConnectionManagerHandleMessageHeartbeat(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	heartbeatCalled := false
	var receivedData *HeartbeatData
	cm.SetHeartbeatCallback(func(runnerID int64, data *HeartbeatData) {
		heartbeatCalled = true
		receivedData = data
	})

	// Add mock connection for heartbeat update
	cm.mu.Lock()
	cm.connections[1] = &RunnerConnection{RunnerID: 1, Send: make(chan []byte, 256)}
	cm.mu.Unlock()

	hbData := HeartbeatData{
		RunnerVersion: "1.0.0",
		Sessions:      []HeartbeatSession{{SessionKey: "s1"}},
	}
	dataBytes, _ := json.Marshal(hbData)
	msg := RunnerMessage{
		Type: MsgTypeHeartbeat,
		Data: dataBytes,
	}
	msgBytes, _ := json.Marshal(msg)

	cm.HandleMessage(1, websocket.TextMessage, msgBytes)

	if !heartbeatCalled {
		t.Error("heartbeat callback should be called")
	}
	if receivedData == nil || receivedData.RunnerVersion != "1.0.0" {
		t.Error("heartbeat data not received correctly")
	}
}

func TestConnectionManagerHandleMessageSessionCreated(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	sessionCreatedCalled := false
	cm.SetSessionCreatedCallback(func(runnerID int64, data *SessionCreatedData) {
		sessionCreatedCalled = true
	})

	scData := SessionCreatedData{SessionID: "s1", Pid: 12345}
	dataBytes, _ := json.Marshal(scData)
	msg := RunnerMessage{Type: MsgTypeSessionCreated, Data: dataBytes}
	msgBytes, _ := json.Marshal(msg)

	cm.HandleMessage(1, websocket.TextMessage, msgBytes)

	if !sessionCreatedCalled {
		t.Error("session created callback should be called")
	}
}

func TestConnectionManagerHandleMessageSessionTerminated(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	sessionTerminatedCalled := false
	cm.SetSessionTerminatedCallback(func(runnerID int64, data *SessionTerminatedData) {
		sessionTerminatedCalled = true
	})

	stData := SessionTerminatedData{SessionID: "s1", ExitCode: 0}
	dataBytes, _ := json.Marshal(stData)
	msg := RunnerMessage{Type: MsgTypeSessionTerminated, Data: dataBytes}
	msgBytes, _ := json.Marshal(msg)

	cm.HandleMessage(1, websocket.TextMessage, msgBytes)

	if !sessionTerminatedCalled {
		t.Error("session terminated callback should be called")
	}
}

func TestConnectionManagerHandleMessageTerminalOutput(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	terminalOutputCalled := false
	cm.SetTerminalOutputCallback(func(runnerID int64, data *TerminalOutputData) {
		terminalOutputCalled = true
	})

	toData := TerminalOutputData{SessionID: "s1", Data: []byte("output")}
	dataBytes, _ := json.Marshal(toData)
	msg := RunnerMessage{Type: MsgTypeTerminalOutput, Data: dataBytes}
	msgBytes, _ := json.Marshal(msg)

	cm.HandleMessage(1, websocket.TextMessage, msgBytes)

	if !terminalOutputCalled {
		t.Error("terminal output callback should be called")
	}
}

func TestConnectionManagerHandleMessageAgentStatus(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	agentStatusCalled := false
	cm.SetAgentStatusCallback(func(runnerID int64, data *AgentStatusData) {
		agentStatusCalled = true
	})

	asData := AgentStatusData{SessionID: "s1", Status: "running"}
	dataBytes, _ := json.Marshal(asData)
	msg := RunnerMessage{Type: MsgTypeAgentStatus, Data: dataBytes}
	msgBytes, _ := json.Marshal(msg)

	cm.HandleMessage(1, websocket.TextMessage, msgBytes)

	if !agentStatusCalled {
		t.Error("agent status callback should be called")
	}
}

func TestConnectionManagerHandleMessagePtyResized(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	ptyResizedCalled := false
	cm.SetPtyResizedCallback(func(runnerID int64, data *PtyResizedData) {
		ptyResizedCalled = true
	})

	prData := PtyResizedData{SessionID: "s1", Cols: 80, Rows: 24}
	dataBytes, _ := json.Marshal(prData)
	msg := RunnerMessage{Type: MsgTypePtyResized, Data: dataBytes}
	msgBytes, _ := json.Marshal(msg)

	cm.HandleMessage(1, websocket.TextMessage, msgBytes)

	if !ptyResizedCalled {
		t.Error("PTY resized callback should be called")
	}
}

func TestConnectionManagerHandleMessageUnknown(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	msg := RunnerMessage{Type: "unknown_type"}
	msgBytes, _ := json.Marshal(msg)

	// Should not panic
	cm.HandleMessage(1, websocket.TextMessage, msgBytes)
}

func TestConnectionManagerSendMessageNotConnected(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	err := cm.SendMessage(nil, 1, &RunnerMessage{Type: MsgTypeCreateSession})
	if err != ErrRunnerNotConnected {
		t.Errorf("err = %v, want ErrRunnerNotConnected", err)
	}
}

func TestRunnerConnectionSendMessage(t *testing.T) {
	rc := &RunnerConnection{
		RunnerID: 1,
		Send:     make(chan []byte, 256),
	}

	// Connection nil
	err := rc.SendMessage(&RunnerMessage{Type: MsgTypeCreateSession})
	if err != ErrConnectionClosed {
		t.Errorf("err = %v, want ErrConnectionClosed", err)
	}
}

func TestRunnerConnectionClose(t *testing.T) {
	rc := &RunnerConnection{
		RunnerID: 1,
		Conn:     nil,
		Send:     make(chan []byte, 256),
	}

	// Should not panic
	rc.Close()
}

func TestRunnerMessageStruct(t *testing.T) {
	msg := RunnerMessage{
		Type:      MsgTypeHeartbeat,
		SessionID: "session-1",
		Timestamp: time.Now().UnixMilli(),
	}

	if msg.Type != MsgTypeHeartbeat {
		t.Errorf("Type = %s, want %s", msg.Type, MsgTypeHeartbeat)
	}
}

func TestMessageTypeConstants(t *testing.T) {
	// From runner
	if MsgTypeHeartbeat != "heartbeat" {
		t.Errorf("MsgTypeHeartbeat = %s, want heartbeat", MsgTypeHeartbeat)
	}
	if MsgTypeSessionCreated != "session_created" {
		t.Errorf("MsgTypeSessionCreated = %s, want session_created", MsgTypeSessionCreated)
	}
	if MsgTypeSessionTerminated != "session_terminated" {
		t.Errorf("MsgTypeSessionTerminated = %s, want session_terminated", MsgTypeSessionTerminated)
	}
	if MsgTypeTerminalOutput != "terminal_output" {
		t.Errorf("MsgTypeTerminalOutput = %s, want terminal_output", MsgTypeTerminalOutput)
	}
	if MsgTypeAgentStatus != "agent_status" {
		t.Errorf("MsgTypeAgentStatus = %s, want agent_status", MsgTypeAgentStatus)
	}
	if MsgTypePtyResized != "pty_resized" {
		t.Errorf("MsgTypePtyResized = %s, want pty_resized", MsgTypePtyResized)
	}
	if MsgTypeError != "error" {
		t.Errorf("MsgTypeError = %s, want error", MsgTypeError)
	}

	// To runner
	if MsgTypeCreateSession != "create_session" {
		t.Errorf("MsgTypeCreateSession = %s, want create_session", MsgTypeCreateSession)
	}
	if MsgTypeTerminateSession != "terminate_session" {
		t.Errorf("MsgTypeTerminateSession = %s, want terminate_session", MsgTypeTerminateSession)
	}
	if MsgTypeTerminalInput != "terminal_input" {
		t.Errorf("MsgTypeTerminalInput = %s, want terminal_input", MsgTypeTerminalInput)
	}
	if MsgTypeTerminalResize != "terminal_resize" {
		t.Errorf("MsgTypeTerminalResize = %s, want terminal_resize", MsgTypeTerminalResize)
	}
	if MsgTypeSendPrompt != "send_prompt" {
		t.Errorf("MsgTypeSendPrompt = %s, want send_prompt", MsgTypeSendPrompt)
	}
}

func TestDataStructs(t *testing.T) {
	t.Run("HeartbeatData", func(t *testing.T) {
		data := HeartbeatData{
			Sessions:      []HeartbeatSession{{SessionKey: "s1"}},
			RunnerVersion: "1.0.0",
		}
		if len(data.Sessions) != 1 {
			t.Error("Sessions not set correctly")
		}
	})

	t.Run("SessionCreatedData", func(t *testing.T) {
		data := SessionCreatedData{
			SessionID:    "s1",
			Pid:          12345,
			BranchName:   "main",
			WorktreePath: "/path/to/worktree",
			Cols:         80,
			Rows:         24,
		}
		if data.SessionID != "s1" || data.Pid != 12345 {
			t.Error("fields not set correctly")
		}
	})

	t.Run("SessionTerminatedData", func(t *testing.T) {
		data := SessionTerminatedData{SessionID: "s1", ExitCode: 1}
		if data.ExitCode != 1 {
			t.Error("ExitCode not set correctly")
		}
	})

	t.Run("TerminalOutputData", func(t *testing.T) {
		data := TerminalOutputData{SessionID: "s1", Data: []byte("output")}
		if string(data.Data) != "output" {
			t.Error("Data not set correctly")
		}
	})

	t.Run("AgentStatusData", func(t *testing.T) {
		data := AgentStatusData{SessionID: "s1", Status: "running", Pid: 123}
		if data.Status != "running" {
			t.Error("Status not set correctly")
		}
	})

	t.Run("PtyResizedData", func(t *testing.T) {
		data := PtyResizedData{SessionID: "s1", Cols: 80, Rows: 24}
		if data.Cols != 80 || data.Rows != 24 {
			t.Error("dimensions not set correctly")
		}
	})

	t.Run("CreateSessionRequest", func(t *testing.T) {
		req := CreateSessionRequest{
			SessionID:     "s1",
			RepoPath:      "/path/to/repo",
			BranchName:    "main",
			InitialPrompt: "hello",
			AgentType:     "claude",
			EnvVars:       map[string]string{"KEY": "VALUE"},
			Cols:          80,
			Rows:          24,
		}
		if req.AgentType != "claude" {
			t.Error("AgentType not set correctly")
		}
	})

	t.Run("TerminalInputRequest", func(t *testing.T) {
		req := TerminalInputRequest{SessionID: "s1", Data: []byte("input")}
		if string(req.Data) != "input" {
			t.Error("Data not set correctly")
		}
	})

	t.Run("TerminalResizeRequest", func(t *testing.T) {
		req := TerminalResizeRequest{SessionID: "s1", Cols: 100, Rows: 50}
		if req.Cols != 100 || req.Rows != 50 {
			t.Error("dimensions not set correctly")
		}
	})
}

func TestErrorVariables(t *testing.T) {
	if ErrRunnerNotConnected == nil {
		t.Error("ErrRunnerNotConnected should not be nil")
	}
	if ErrRunnerNotConnected.Error() != "runner not connected" {
		t.Error("ErrRunnerNotConnected message incorrect")
	}

	if ErrConnectionClosed == nil {
		t.Error("ErrConnectionClosed should not be nil")
	}
	if ErrConnectionClosed.Error() != "connection closed" {
		t.Error("ErrConnectionClosed message incorrect")
	}
}
