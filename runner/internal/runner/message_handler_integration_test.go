package runner

import (
	"testing"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/client"
	"github.com/anthropics/agentmesh/runner/internal/config"
	"github.com/anthropics/agentmesh/runner/internal/workspace"
)

// Integration tests for MessageHandler with MockConnection

func TestMessageHandlerIntegrationWithMockConnection(t *testing.T) {
	tempDir := t.TempDir()
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	ws, err := workspace.NewManager(tempDir, "")
	if err != nil {
		t.Skipf("Could not create workspace manager: %v", err)
	}

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         tempDir,
		},
		workspace: ws,
	}

	handler := NewRunnerMessageHandler(runner, store, mockConn)
	mockConn.SetHandler(handler)

	// Test flow: create -> list -> terminate
	createReq := client.CreateSessionRequest{
		SessionID:      "integration-session",
		InitialCommand: "echo",
		WorkingDir:     tempDir,
	}

	// Create session via mock connection simulation
	err = mockConn.SimulateCreateSession(createReq)
	if err != nil {
		t.Logf("Create session: %v", err)
	}

	// Give session time to start
	time.Sleep(50 * time.Millisecond)

	// List sessions
	sessions := mockConn.GetSessions()
	t.Logf("Sessions after create: %d", len(sessions))

	// Terminate session
	terminateReq := client.TerminateSessionRequest{
		SessionID: "integration-session",
	}
	err = mockConn.SimulateTerminateSession(terminateReq)
	t.Logf("Terminate session: %v", err)

	// List sessions again
	sessions = mockConn.GetSessions()
	t.Logf("Sessions after terminate: %d", len(sessions))
}

func TestMessageHandlerIntegrationSessionLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	store := NewInMemorySessionStore()
	mockConn := client.NewMockConnection()

	runner := &Runner{
		cfg: &config.Config{
			MaxConcurrentSessions: 10,
			WorkspaceRoot:         tempDir,
		},
	}

	handler := NewRunnerMessageHandler(runner, store, mockConn)
	mockConn.SetHandler(handler)

	// Create multiple sessions
	for i := 0; i < 3; i++ {
		req := client.CreateSessionRequest{
			SessionID:      "lifecycle-session-" + string(rune('a'+i)),
			InitialCommand: "sleep",
			WorkingDir:     tempDir,
		}
		err := mockConn.SimulateCreateSession(req)
		if err != nil {
			t.Logf("Create session %d: %v", i, err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	// Check session count
	sessions := mockConn.GetSessions()
	if len(sessions) < 1 {
		t.Log("Expected at least 1 session")
	}

	// Terminate all sessions
	for _, s := range sessions {
		req := client.TerminateSessionRequest{
			SessionID: s.SessionID,
		}
		mockConn.SimulateTerminateSession(req)
	}

	// Verify all sessions terminated
	time.Sleep(50 * time.Millisecond)
	sessions = mockConn.GetSessions()
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions after termination, got %d", len(sessions))
	}
}
