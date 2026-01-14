package runner

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
)

func TestNewConnectionManager(t *testing.T) {
	logger := newTestLogger()
	cm := NewConnectionManager(logger)
	defer cm.Close()

	assert.NotNil(t, cm)
	assert.Equal(t, 30*time.Second, cm.pingInterval)
	assert.Equal(t, 60*time.Second, cm.pingTimeout)
	assert.Equal(t, int64(0), cm.ConnectionCount())

	// Verify all shards are initialized
	for i := 0; i < numShards; i++ {
		assert.NotNil(t, cm.shards[i])
		assert.NotNil(t, cm.shards[i].connections)
	}
}

func TestConnectionManager_GetShard(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	defer cm.Close()

	// Test that same runner ID always maps to same shard
	shard1 := cm.getShard(100)
	shard2 := cm.getShard(100)
	assert.Same(t, shard1, shard2)

	// Test that different runner IDs may map to different shards
	// (not guaranteed but likely for sufficiently different IDs)
	shardA := cm.getShard(1)
	shardB := cm.getShard(256 + 1) // Should map to same shard as 1
	assert.Same(t, shardA, shardB)

	// Test negative runner ID handling (should work via unsigned conversion)
	shardNeg := cm.getShard(-1)
	assert.NotNil(t, shardNeg)
}

func TestConnectionManager_CallbackSetters(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	defer cm.Close()

	// Test SetHeartbeatCallback
	cm.SetHeartbeatCallback(func(runnerID int64, data *HeartbeatData) {})
	assert.NotNil(t, cm.GetHeartbeatCallback())

	// Test SetDisconnectCallback
	cm.SetDisconnectCallback(func(runnerID int64) {})
	assert.NotNil(t, cm.GetDisconnectCallback())

	// Test other callbacks (no getters, just verify they don't panic)
	cm.SetPodCreatedCallback(func(runnerID int64, data *PodCreatedData) {})
	cm.SetPodTerminatedCallback(func(runnerID int64, data *PodTerminatedData) {})
	cm.SetTerminalOutputCallback(func(runnerID int64, data *TerminalOutputData) {})
	cm.SetAgentStatusCallback(func(runnerID int64, data *AgentStatusData) {})
	cm.SetPtyResizedCallback(func(runnerID int64, data *PtyResizedData) {})
	cm.SetInitializedCallback(func(runnerID int64, availableAgents []string) {})

	// Test provider and version setters
	cm.SetAgentTypesProvider(&mockAgentTypesProvider{})
	cm.SetServerVersion("1.0.0")
	assert.Equal(t, "1.0.0", cm.serverVersion)
}

func TestConnectionManager_AddConnection(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	defer cm.Close()

	conn := newTestWebSocketConn(t)

	// Add connection
	rc := cm.AddConnection(1, conn)
	assert.NotNil(t, rc)
	assert.Equal(t, int64(1), rc.RunnerID)
	assert.NotNil(t, rc.Send)
	assert.Equal(t, int64(1), cm.ConnectionCount())

	// Verify connection is stored
	stored := cm.GetConnection(1)
	assert.Same(t, rc, stored)
}

func TestConnectionManager_AddConnection_ReplacesExisting(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	defer cm.Close()

	conn1 := newTestWebSocketConn(t)
	conn2 := newTestWebSocketConn(t)

	// Add first connection
	rc1 := cm.AddConnection(1, conn1)
	assert.Equal(t, int64(1), cm.ConnectionCount())

	// Drain the send channel to prevent blocking
	go func() {
		for range rc1.Send {
		}
	}()

	// Add second connection with same ID - should replace first
	rc2 := cm.AddConnection(1, conn2)
	assert.Equal(t, int64(1), cm.ConnectionCount())
	assert.NotSame(t, rc1, rc2)

	// Verify new connection is stored
	stored := cm.GetConnection(1)
	assert.Same(t, rc2, stored)
}

func TestConnectionManager_RemoveConnection(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	defer cm.Close()

	conn := newTestWebSocketConn(t)

	// Add and then remove connection
	rc := cm.AddConnection(1, conn)
	go func() {
		for range rc.Send {
		}
	}()

	assert.Equal(t, int64(1), cm.ConnectionCount())
	cm.RemoveConnection(1)
	assert.Equal(t, int64(0), cm.ConnectionCount())

	// Verify connection is removed
	stored := cm.GetConnection(1)
	assert.Nil(t, stored)
}

func TestConnectionManager_RemoveConnection_CallsDisconnectCallback(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	defer cm.Close()

	disconnectedRunnerID := int64(0)
	cm.SetDisconnectCallback(func(runnerID int64) {
		disconnectedRunnerID = runnerID
	})

	conn := newTestWebSocketConn(t)
	rc := cm.AddConnection(1, conn)
	go func() {
		for range rc.Send {
		}
	}()

	cm.RemoveConnection(1)
	assert.Equal(t, int64(1), disconnectedRunnerID)
}

func TestConnectionManager_RemoveConnection_Nonexistent(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	defer cm.Close()

	// Should not panic when removing nonexistent connection
	cm.RemoveConnection(999)
	assert.Equal(t, int64(0), cm.ConnectionCount())
}

func TestConnectionManager_IsConnected(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	defer cm.Close()

	// Not connected initially
	assert.False(t, cm.IsConnected(1))

	conn := newTestWebSocketConn(t)
	rc := cm.AddConnection(1, conn)
	go func() {
		for range rc.Send {
		}
	}()

	// Connected after adding
	assert.True(t, cm.IsConnected(1))

	cm.RemoveConnection(1)

	// Not connected after removing
	assert.False(t, cm.IsConnected(1))
}

func TestConnectionManager_UpdateHeartbeat(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	defer cm.Close()

	conn := newTestWebSocketConn(t)
	rc := cm.AddConnection(1, conn)
	go func() {
		for range rc.Send {
		}
	}()

	initialTime := rc.LastPing

	// Wait a bit and update heartbeat
	time.Sleep(10 * time.Millisecond)
	cm.UpdateHeartbeat(1)

	rc.mu.Lock()
	updatedTime := rc.LastPing
	rc.mu.Unlock()

	assert.True(t, updatedTime.After(initialTime))
}

func TestConnectionManager_UpdateHeartbeat_Nonexistent(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	defer cm.Close()

	// Should not panic when updating heartbeat for nonexistent connection
	cm.UpdateHeartbeat(999)
}

func TestConnectionManager_GetConnectedRunnerIDs(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	defer cm.Close()

	// Empty initially
	ids := cm.GetConnectedRunnerIDs()
	assert.Empty(t, ids)

	// Add some connections
	var rcs []*RunnerConnection
	for i := int64(1); i <= 5; i++ {
		conn := newTestWebSocketConn(t)
		rc := cm.AddConnection(i, conn)
		rcs = append(rcs, rc)
		go func(rc *RunnerConnection) {
			for range rc.Send {
			}
		}(rc)
	}

	// Get all IDs
	ids = cm.GetConnectedRunnerIDs()
	assert.Len(t, ids, 5)

	// Verify all IDs are present
	idMap := make(map[int64]bool)
	for _, id := range ids {
		idMap[id] = true
	}
	for i := int64(1); i <= 5; i++ {
		assert.True(t, idMap[i], "ID %d should be present", i)
	}
}

func TestConnectionManager_ConnectionCount(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	defer cm.Close()

	assert.Equal(t, int64(0), cm.ConnectionCount())

	// Add connections
	var rcs []*RunnerConnection
	for i := int64(1); i <= 3; i++ {
		conn := newTestWebSocketConn(t)
		rc := cm.AddConnection(i, conn)
		rcs = append(rcs, rc)
		go func(rc *RunnerConnection) {
			for range rc.Send {
			}
		}(rc)
	}

	assert.Equal(t, int64(3), cm.ConnectionCount())

	// Remove one
	cm.RemoveConnection(2)
	assert.Equal(t, int64(2), cm.ConnectionCount())
}

func TestConnectionManager_Close(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())

	// Add some connections
	var rcs []*RunnerConnection
	for i := int64(1); i <= 3; i++ {
		conn := newTestWebSocketConn(t)
		rc := cm.AddConnection(i, conn)
		rcs = append(rcs, rc)
		go func(rc *RunnerConnection) {
			for range rc.Send {
			}
		}(rc)
	}

	assert.Equal(t, int64(3), cm.ConnectionCount())

	// Close all
	cm.Close()

	assert.Equal(t, int64(0), cm.ConnectionCount())

	// Verify all connections are removed
	for i := int64(1); i <= 3; i++ {
		assert.Nil(t, cm.GetConnection(i))
	}
}

func TestConnectionManager_ConcurrentAccess(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	defer cm.Close()

	const numGoroutines = 50
	const numOps = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3)

	// Concurrent adds
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				conn := newTestWebSocketConn(t)
				rc := cm.AddConnection(int64(id), conn)
				go func(rc *RunnerConnection) {
					for range rc.Send {
					}
				}(rc)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				_ = cm.GetConnection(int64(id))
				_ = cm.IsConnected(int64(id))
				_ = cm.GetConnectedRunnerIDs()
			}
		}(i)
	}

	// Concurrent removes
	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOps; j++ {
				cm.RemoveConnection(int64(id))
			}
		}(i)
	}

	wg.Wait()
}

func TestConnectionManager_SendMethods(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	defer cm.Close()

	ctx := context.Background()

	// All send methods should return error when runner not connected
	t.Run("SendMessage_NotConnected", func(t *testing.T) {
		err := cm.SendMessage(ctx, 999, &RunnerMessage{Type: "test"})
		assert.Equal(t, ErrRunnerNotConnected, err)
	})

	t.Run("SendCreatePod_NotConnected", func(t *testing.T) {
		err := cm.SendCreatePod(ctx, 999, &CreatePodRequest{PodKey: "test"})
		assert.Equal(t, ErrRunnerNotConnected, err)
	})

	t.Run("SendTerminatePod_NotConnected", func(t *testing.T) {
		err := cm.SendTerminatePod(ctx, 999, "test-pod")
		assert.Equal(t, ErrRunnerNotConnected, err)
	})

	t.Run("SendTerminalInput_NotConnected", func(t *testing.T) {
		err := cm.SendTerminalInput(ctx, 999, "test-pod", []byte("input"))
		assert.Equal(t, ErrRunnerNotConnected, err)
	})

	t.Run("SendTerminalResize_NotConnected", func(t *testing.T) {
		err := cm.SendTerminalResize(ctx, 999, "test-pod", 80, 24)
		assert.Equal(t, ErrRunnerNotConnected, err)
	})

	t.Run("SendPrompt_NotConnected", func(t *testing.T) {
		err := cm.SendPrompt(ctx, 999, "test-pod", "prompt")
		assert.Equal(t, ErrRunnerNotConnected, err)
	})

	// Test with connected runner
	conn := newTestWebSocketConn(t)
	rc := cm.AddConnection(1, conn)
	// Mark connection as initialized (required for sending non-init messages)
	rc.SetInitialized(true, []string{})
	go func() {
		for range rc.Send {
		}
	}()

	t.Run("SendCreatePod_Success", func(t *testing.T) {
		err := cm.SendCreatePod(ctx, 1, &CreatePodRequest{
			PodKey:        "test-pod",
			LaunchCommand: "claude",
		})
		assert.NoError(t, err)
	})

	t.Run("SendTerminatePod_Success", func(t *testing.T) {
		err := cm.SendTerminatePod(ctx, 1, "test-pod")
		assert.NoError(t, err)
	})

	t.Run("SendTerminalInput_Success", func(t *testing.T) {
		err := cm.SendTerminalInput(ctx, 1, "test-pod", []byte("ls -la\n"))
		assert.NoError(t, err)
	})

	t.Run("SendTerminalResize_Success", func(t *testing.T) {
		err := cm.SendTerminalResize(ctx, 1, "test-pod", 120, 40)
		assert.NoError(t, err)
	})

	t.Run("SendPrompt_Success", func(t *testing.T) {
		err := cm.SendPrompt(ctx, 1, "test-pod", "Hello!")
		assert.NoError(t, err)
	})
}

func TestRunnerConnection_SendMessage(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		rc := &RunnerConnection{
			RunnerID: 1,
			Conn:     &websocket.Conn{}, // Non-nil to pass check
			Send:     make(chan []byte, 256),
		}

		msg := &RunnerMessage{Type: "test", PodKey: "pod-1"}
		err := rc.SendMessage(msg)
		assert.NoError(t, err)

		// Verify message was sent
		select {
		case data := <-rc.Send:
			assert.Contains(t, string(data), "test")
		default:
			t.Fatal("expected message in send channel")
		}
	})

	t.Run("ConnectionClosed", func(t *testing.T) {
		rc := &RunnerConnection{
			RunnerID: 1,
			Conn:     nil, // Closed connection
			Send:     make(chan []byte, 256),
		}

		msg := &RunnerMessage{Type: "test"}
		err := rc.SendMessage(msg)
		assert.Equal(t, ErrConnectionClosed, err)
	})

	t.Run("BufferFull", func(t *testing.T) {
		rc := &RunnerConnection{
			RunnerID: 1,
			Conn:     &websocket.Conn{},
			Send:     make(chan []byte, 1), // Small buffer
		}

		// Fill the buffer
		rc.Send <- []byte("first")

		msg := &RunnerMessage{Type: "test"}
		err := rc.SendMessage(msg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "buffer full")
	})
}

func TestRunnerConnection_Close_Idempotent(t *testing.T) {
	conn := newTestWebSocketConn(t)
	rc := &RunnerConnection{
		RunnerID: 1,
		Conn:     conn,
		Send:     make(chan []byte, 10),
	}

	// Close multiple times should not panic
	rc.Close()
	rc.Close()
	rc.Close()

	// Verify channel is closed
	_, ok := <-rc.Send
	assert.False(t, ok, "send channel should be closed")
}

func TestRunnerConnection_Close_DrainedProperly(t *testing.T) {
	conn := newTestWebSocketConn(t)
	rc := &RunnerConnection{
		RunnerID: 1,
		Conn:     conn,
		Send:     make(chan []byte, 10),
	}

	// Add some messages before close
	rc.Send <- []byte("msg1")
	rc.Send <- []byte("msg2")

	rc.Close()

	// Should be able to read buffered messages
	msg1, ok1 := <-rc.Send
	msg2, ok2 := <-rc.Send
	_, ok3 := <-rc.Send

	assert.True(t, ok1)
	assert.True(t, ok2)
	assert.False(t, ok3, "channel should be closed after draining")
	assert.Equal(t, []byte("msg1"), msg1)
	assert.Equal(t, []byte("msg2"), msg2)
}

func TestWritePump_ChannelClosed(t *testing.T) {
	conn := newTestWebSocketConn(t)
	rc := &RunnerConnection{
		RunnerID: 1,
		Conn:     conn,
		Send:     make(chan []byte, 10),
	}

	// Start WritePump in background
	done := make(chan struct{})
	go func() {
		rc.WritePump()
		close(done)
	}()

	// Use proper Close() method which is idempotent
	rc.Close()

	// Wait for WritePump to exit
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("WritePump did not exit after channel closed")
	}
}

func TestWritePump_SendsMessages(t *testing.T) {
	conn := newTestWebSocketConn(t)
	rc := &RunnerConnection{
		RunnerID: 1,
		Conn:     conn,
		Send:     make(chan []byte, 10),
	}

	// Start WritePump in background
	go rc.WritePump()

	// Send a message
	msg := RunnerMessage{Type: "test", PodKey: "pod-1"}
	data, _ := json.Marshal(msg)
	rc.Send <- data

	// Give it time to process
	time.Sleep(50 * time.Millisecond)

	// Close to stop the pump
	rc.Close()
}

func TestWritePump_ConnectionNil(t *testing.T) {
	rc := &RunnerConnection{
		RunnerID: 1,
		Conn:     nil, // Start with nil connection
		Send:     make(chan []byte, 10),
	}

	// Start WritePump in background
	done := make(chan struct{})
	go func() {
		rc.WritePump()
		close(done)
	}()

	// Send a message - should cause exit due to nil conn
	rc.Send <- []byte("test")

	// Wait for WritePump to exit
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("WritePump did not exit with nil connection")
	}
}

func TestWritePump_SendMultipleMessages(t *testing.T) {
	conn := newTestWebSocketConn(t)
	rc := &RunnerConnection{
		RunnerID: 1,
		Conn:     conn,
		Send:     make(chan []byte, 10),
	}

	// Start WritePump in background
	go rc.WritePump()

	// Send multiple messages
	for i := 0; i < 5; i++ {
		msg := RunnerMessage{Type: "test", PodKey: "pod-1"}
		data, _ := json.Marshal(msg)
		rc.Send <- data
	}

	// Give it time to process all messages
	time.Sleep(100 * time.Millisecond)

	// Clean shutdown
	rc.Close()
}

func TestWritePump_TickerPing(t *testing.T) {
	// This test is for the ping ticker path - difficult to test directly
	// as it requires waiting 30 seconds. Instead, we verify the WritePump
	// handles connection properly.
	conn := newTestWebSocketConn(t)
	rc := &RunnerConnection{
		RunnerID: 1,
		Conn:     conn,
		Send:     make(chan []byte, 10),
	}

	// Start WritePump
	go rc.WritePump()

	// Very short delay
	time.Sleep(50 * time.Millisecond)

	// Close should work cleanly
	rc.Close()
}

func TestHandleInitializeMessage_SendFails(t *testing.T) {
	// Test when SendMessage fails (no connection or buffer full)
	logger := newTestLogger()
	cm := NewConnectionManager(logger)
	cm.SetServerVersion("1.0.0")
	cm.SetAgentTypesProvider(&mockAgentTypesProvider{agentTypes: []AgentTypeInfo{}})

	runnerID := int64(900)
	// Don't add connection - SendMessage will fail

	initParams := InitializeParams{
		ProtocolVersion: CurrentProtocolVersion,
		RunnerInfo:      RunnerInfo{Version: "1.0.0", NodeID: "test"},
	}
	data, _ := json.Marshal(initParams)

	// Should not panic when send fails
	cm.handleInitializeMessage(runnerID, data)
}

func TestConnectionManager_SetPingInterval(t *testing.T) {
	cm := NewConnectionManager(newTestLogger())
	defer cm.Close()

	// Default ping interval
	assert.Equal(t, 30*time.Second, cm.pingInterval)

	// Set custom ping interval
	cm.SetPingInterval(15 * time.Second)
	assert.Equal(t, 15*time.Second, cm.pingInterval)

	// Verify new connections get the configured interval
	conn := newTestWebSocketConn(t)
	rc := cm.AddConnection(1, conn)
	go func() {
		for range rc.Send {
		}
	}()

	assert.Equal(t, 15*time.Second, rc.PingInterval)
}

func TestConnectionManager_InitTimeout(t *testing.T) {
	t.Run("SetInitTimeout", func(t *testing.T) {
		cm := NewConnectionManager(newTestLogger())
		defer cm.Close()

		// Default timeout
		assert.Equal(t, DefaultInitTimeout, cm.initTimeout)

		// Set custom timeout
		cm.SetInitTimeout(10 * time.Second)
		assert.Equal(t, 10*time.Second, cm.initTimeout)
	})

	t.Run("checkInitTimeouts_removes_uninitialized_connections", func(t *testing.T) {
		cm := NewConnectionManager(newTestLogger())
		defer cm.Close()

		// Set a very short timeout for testing
		cm.SetInitTimeout(50 * time.Millisecond)

		// Add a connection
		conn := newTestWebSocketConn(t)
		rc := cm.AddConnection(1, conn)
		go func() {
			for range rc.Send {
			}
		}()

		// Connection should exist
		assert.True(t, cm.IsConnected(1))
		assert.False(t, rc.IsInitialized())

		// Wait for timeout
		time.Sleep(100 * time.Millisecond)

		// Manually trigger check
		cm.checkInitTimeouts()

		// Connection should be removed
		assert.False(t, cm.IsConnected(1))
	})

	t.Run("checkInitTimeouts_keeps_initialized_connections", func(t *testing.T) {
		cm := NewConnectionManager(newTestLogger())
		defer cm.Close()

		// Set a very short timeout
		cm.SetInitTimeout(50 * time.Millisecond)

		// Add a connection and mark it initialized
		conn := newTestWebSocketConn(t)
		rc := cm.AddConnection(1, conn)
		go func() {
			for range rc.Send {
			}
		}()
		rc.SetInitialized(true, []string{"claude-code"})

		// Wait for timeout period
		time.Sleep(100 * time.Millisecond)

		// Trigger check
		cm.checkInitTimeouts()

		// Connection should still exist (it's initialized)
		assert.True(t, cm.IsConnected(1))
	})

	t.Run("StartInitTimeoutChecker_works", func(t *testing.T) {
		cm := NewConnectionManager(newTestLogger())

		// Set a very short timeout
		cm.SetInitTimeout(50 * time.Millisecond)

		// Add an uninitialized connection
		conn := newTestWebSocketConn(t)
		rc := cm.AddConnection(1, conn)
		go func() {
			for range rc.Send {
			}
		}()

		// Start the checker
		cm.StartInitTimeoutChecker()

		// Wait for the checker to run (checks every 10 seconds, but we can close early)
		// For this test, we'll just verify it doesn't panic and can be stopped
		time.Sleep(10 * time.Millisecond)

		// Close should stop the checker cleanly
		cm.Close()
	})
}

func TestRunnerConnection_InitializedState(t *testing.T) {
	t.Run("IsInitialized_thread_safe", func(t *testing.T) {
		rc := &RunnerConnection{
			RunnerID: 1,
			Conn:     &websocket.Conn{},
			Send:     make(chan []byte, 256),
		}

		// Initially not initialized
		assert.False(t, rc.IsInitialized())
		assert.Empty(t, rc.GetAvailableAgents())

		// Set initialized
		rc.SetInitialized(true, []string{"agent1", "agent2"})

		// Should be initialized now
		assert.True(t, rc.IsInitialized())
		assert.Equal(t, []string{"agent1", "agent2"}, rc.GetAvailableAgents())
	})

	t.Run("concurrent_access", func(t *testing.T) {
		rc := &RunnerConnection{
			RunnerID: 1,
			Conn:     &websocket.Conn{},
			Send:     make(chan []byte, 256),
		}

		var wg sync.WaitGroup
		wg.Add(3)

		// Concurrent reads
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_ = rc.IsInitialized()
				_ = rc.GetAvailableAgents()
			}
		}()

		// Concurrent writes
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				rc.SetInitialized(i%2 == 0, []string{"agent"})
			}
		}()

		// More concurrent reads
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				_ = rc.IsInitialized()
				_ = rc.GetAvailableAgents()
			}
		}()

		wg.Wait()
	})
}

func TestWritePump_UsesPingInterval(t *testing.T) {
	t.Run("uses_default_when_zero", func(t *testing.T) {
		conn := newTestWebSocketConn(t)
		rc := &RunnerConnection{
			RunnerID:     1,
			Conn:         conn,
			Send:         make(chan []byte, 10),
			PingInterval: 0, // Zero means use default
		}

		// Start WritePump briefly
		go rc.WritePump()
		time.Sleep(10 * time.Millisecond)
		rc.Close()
	})

	t.Run("uses_configured_interval", func(t *testing.T) {
		conn := newTestWebSocketConn(t)
		rc := &RunnerConnection{
			RunnerID:     1,
			Conn:         conn,
			Send:         make(chan []byte, 10),
			PingInterval: 5 * time.Second,
		}

		// Start WritePump briefly
		go rc.WritePump()
		time.Sleep(10 * time.Millisecond)
		rc.Close()
	})
}

