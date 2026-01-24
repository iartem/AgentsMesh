package runner

import (
	"context"
	"errors"
	"testing"
	"time"

	runnerv1 "github.com/anthropics/agentsmesh/proto/gen/go/runner/v1"
)

func TestNewSandboxQueryService(t *testing.T) {
	svc := NewSandboxQueryService(nil)
	if svc == nil {
		t.Fatal("Expected non-nil service")
	}
	defer svc.Stop()

	if svc.done == nil {
		t.Error("done channel should be initialized")
	}
}

func TestSandboxQueryService_RegisterQuery(t *testing.T) {
	svc := NewSandboxQueryService(nil)
	defer svc.Stop()

	requestID := "test-request-123"
	ch := svc.RegisterQuery(requestID)

	if ch == nil {
		t.Fatal("Expected non-nil channel")
	}

	// Verify query was stored
	_, ok := svc.pendingQueries.Load(requestID)
	if !ok {
		t.Error("Query should be stored in pending queries")
	}
}

func TestSandboxQueryService_CompleteQuery(t *testing.T) {
	svc := NewSandboxQueryService(nil)
	defer svc.Stop()

	requestID := "complete-test"
	ch := svc.RegisterQuery(requestID)

	// Complete the query
	event := &runnerv1.SandboxesStatusEvent{
		RequestId: requestID,
		Sandboxes: []*runnerv1.SandboxStatus{
			{
				PodKey:       "pod-1",
				Exists:       true,
				CanResume:    true,
				SandboxPath:  "/path/to/sandbox",
				RepositoryUrl: "https://github.com/test/repo",
				BranchName:   "main",
				CurrentCommit: "abc12345",
				SizeBytes:    1024,
				LastModified: time.Now().Unix(),
				HasUncommittedChanges: true,
			},
		},
	}

	svc.CompleteQuery(requestID, 42, event)

	// Should receive result on channel
	select {
	case result := <-ch:
		if result == nil {
			t.Fatal("Expected non-nil result")
		}
		if result.RequestID != requestID {
			t.Errorf("RequestID = %s, want %s", result.RequestID, requestID)
		}
		if result.RunnerID != 42 {
			t.Errorf("RunnerID = %d, want 42", result.RunnerID)
		}
		if len(result.Sandboxes) != 1 {
			t.Fatalf("Sandboxes len = %d, want 1", len(result.Sandboxes))
		}
		sb := result.Sandboxes[0]
		if sb.PodKey != "pod-1" {
			t.Errorf("PodKey = %s, want pod-1", sb.PodKey)
		}
		if !sb.Exists {
			t.Error("Exists should be true")
		}
		if !sb.CanResume {
			t.Error("CanResume should be true")
		}
		if sb.SandboxPath != "/path/to/sandbox" {
			t.Errorf("SandboxPath = %s, want /path/to/sandbox", sb.SandboxPath)
		}
		if sb.RepositoryURL != "https://github.com/test/repo" {
			t.Errorf("RepositoryURL = %s, want https://github.com/test/repo", sb.RepositoryURL)
		}
		if sb.BranchName != "main" {
			t.Errorf("BranchName = %s, want main", sb.BranchName)
		}
		if sb.CurrentCommit != "abc12345" {
			t.Errorf("CurrentCommit = %s, want abc12345", sb.CurrentCommit)
		}
		if !sb.HasUncommittedChanges {
			t.Error("HasUncommittedChanges should be true")
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for result")
	}

	// Query should be removed from pending
	_, ok := svc.pendingQueries.Load(requestID)
	if ok {
		t.Error("Query should be removed after completion")
	}
}

func TestSandboxQueryService_CompleteQuery_NotFound(t *testing.T) {
	svc := NewSandboxQueryService(nil)
	defer svc.Stop()

	// Complete a query that was never registered - should not panic
	event := &runnerv1.SandboxesStatusEvent{
		RequestId: "nonexistent",
		Sandboxes: []*runnerv1.SandboxStatus{},
	}

	// Should not panic
	svc.CompleteQuery("nonexistent", 1, event)
}

func TestSandboxQueryService_QuerySandboxes_Success(t *testing.T) {
	svc := NewSandboxQueryService(nil)
	defer svc.Stop()

	ctx := context.Background()
	podKeys := []string{"pod-1", "pod-2"}

	// Mock send function that simulates async response
	sendFn := func(runnerID int64, requestID string, podKeys []string) error {
		// Simulate async response from runner
		go func() {
			time.Sleep(10 * time.Millisecond)
			event := &runnerv1.SandboxesStatusEvent{
				RequestId: requestID,
				Sandboxes: []*runnerv1.SandboxStatus{
					{PodKey: "pod-1", Exists: true},
					{PodKey: "pod-2", Exists: false},
				},
			}
			svc.CompleteQuery(requestID, runnerID, event)
		}()
		return nil
	}

	result, err := svc.QuerySandboxes(ctx, 123, podKeys, sendFn)
	if err != nil {
		t.Fatalf("QuerySandboxes error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected non-nil result")
	}
	if result.RunnerID != 123 {
		t.Errorf("RunnerID = %d, want 123", result.RunnerID)
	}
	if len(result.Sandboxes) != 2 {
		t.Errorf("Sandboxes len = %d, want 2", len(result.Sandboxes))
	}
}

func TestSandboxQueryService_QuerySandboxes_SendError(t *testing.T) {
	svc := NewSandboxQueryService(nil)
	defer svc.Stop()

	ctx := context.Background()
	expectedErr := errors.New("send failed")

	sendFn := func(runnerID int64, requestID string, podKeys []string) error {
		return expectedErr
	}

	_, err := svc.QuerySandboxes(ctx, 1, []string{"pod-1"}, sendFn)
	if err != expectedErr {
		t.Errorf("Error = %v, want %v", err, expectedErr)
	}
}

func TestSandboxQueryService_QuerySandboxes_ContextCanceled(t *testing.T) {
	svc := NewSandboxQueryService(nil)
	defer svc.Stop()

	ctx, cancel := context.WithCancel(context.Background())

	sendFn := func(runnerID int64, requestID string, podKeys []string) error {
		// Cancel context before response arrives
		cancel()
		return nil
	}

	_, err := svc.QuerySandboxes(ctx, 1, []string{"pod-1"}, sendFn)
	if err != context.Canceled {
		t.Errorf("Error = %v, want context.Canceled", err)
	}
}

func TestSandboxQueryService_QuerySandboxes_Timeout(t *testing.T) {
	// Create service with short timeout for testing
	svc := NewSandboxQueryService(nil)
	defer svc.Stop()

	// Override timeout for this test by using a context with shorter deadline
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	sendFn := func(runnerID int64, requestID string, podKeys []string) error {
		// Don't respond - let it timeout
		return nil
	}

	_, err := svc.QuerySandboxes(ctx, 1, []string{"pod-1"}, sendFn)
	if err != context.DeadlineExceeded {
		t.Errorf("Error = %v, want context.DeadlineExceeded", err)
	}
}

func TestSandboxQueryService_Stop(t *testing.T) {
	svc := NewSandboxQueryService(nil)

	// Stop should not block
	done := make(chan struct{})
	go func() {
		svc.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(time.Second):
		t.Error("Stop should not block")
	}
}

func TestSandboxStatus_Fields(t *testing.T) {
	status := SandboxStatus{
		PodKey:                "test-pod",
		Exists:                true,
		CanResume:             true,
		SandboxPath:           "/path/to/sandbox",
		RepositoryURL:         "https://github.com/test/repo",
		BranchName:            "feature-branch",
		CurrentCommit:         "abc12345",
		SizeBytes:             1024,
		LastModified:          1234567890,
		HasUncommittedChanges: true,
		Error:                 "",
	}

	if status.PodKey != "test-pod" {
		t.Errorf("PodKey = %s, want test-pod", status.PodKey)
	}
	if !status.Exists {
		t.Error("Exists should be true")
	}
	if !status.CanResume {
		t.Error("CanResume should be true")
	}
	if status.SandboxPath != "/path/to/sandbox" {
		t.Errorf("SandboxPath = %s, want /path/to/sandbox", status.SandboxPath)
	}
}

func TestSandboxQueryResult_Fields(t *testing.T) {
	result := SandboxQueryResult{
		RequestID: "req-123",
		RunnerID:  42,
		Sandboxes: []*SandboxStatus{
			{PodKey: "pod-1", Exists: true},
		},
		Error: "",
	}

	if result.RequestID != "req-123" {
		t.Errorf("RequestID = %s, want req-123", result.RequestID)
	}
	if result.RunnerID != 42 {
		t.Errorf("RunnerID = %d, want 42", result.RunnerID)
	}
	if len(result.Sandboxes) != 1 {
		t.Errorf("Sandboxes len = %d, want 1", len(result.Sandboxes))
	}
}

func TestSandboxQueryResult_WithError(t *testing.T) {
	result := SandboxQueryResult{
		RequestID: "req-456",
		RunnerID:  1,
		Sandboxes: nil,
		Error:     "runner offline",
	}

	if result.Error != "runner offline" {
		t.Errorf("Error = %s, want 'runner offline'", result.Error)
	}
}

func TestSandboxQueryTimeout_Constant(t *testing.T) {
	if SandboxQueryTimeout != 30*time.Second {
		t.Errorf("SandboxQueryTimeout = %v, want 30s", SandboxQueryTimeout)
	}
}

func TestSandboxQueryService_CleanupLoop(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping cleanup loop test in short mode")
	}

	svc := NewSandboxQueryService(nil)
	defer svc.Stop()

	// Register a query with a very short timeout (already expired)
	// Use RegisterQueryWithTimeout to avoid data race
	requestID := "expired-query"
	ch := svc.RegisterQueryWithTimeout(requestID, -time.Second) // Already expired

	// Wait for cleanup loop to run (it runs every 10 seconds)
	time.Sleep(11 * time.Second)

	// Check if we got a timeout error on the channel
	select {
	case result := <-ch:
		if result == nil {
			t.Error("Expected non-nil result")
		} else if result.Error != "query timeout" {
			t.Errorf("Expected 'query timeout' error, got: %s", result.Error)
		}
	default:
		t.Error("Expected cleanup loop to send timeout result")
	}

	// Verify query was removed
	_, exists := svc.pendingQueries.Load(requestID)
	if exists {
		t.Error("Query should be removed after cleanup")
	}
}

func TestSandboxQueryService_RegisterQueryWithTimeout(t *testing.T) {
	svc := NewSandboxQueryService(nil)
	defer svc.Stop()

	requestID := "custom-timeout"
	ch := svc.RegisterQueryWithTimeout(requestID, 5*time.Second)

	if ch == nil {
		t.Fatal("Expected non-nil channel")
	}

	// Verify query was stored
	_, ok := svc.pendingQueries.Load(requestID)
	if !ok {
		t.Error("Query should be stored in pending queries")
	}
}

func TestSandboxQueryService_NewWithConnectionManager(t *testing.T) {
	// Test creating service with nil connection manager
	svc := NewSandboxQueryService(nil)
	defer svc.Stop()

	if svc == nil {
		t.Fatal("Expected non-nil service")
	}
}

func TestSandboxQueryService_CompleteQuery_ChannelFull(t *testing.T) {
	svc := NewSandboxQueryService(nil)
	defer svc.Stop()

	requestID := "full-channel"
	ch := svc.RegisterQuery(requestID)

	// Fill the channel first
	ch <- &SandboxQueryResult{RequestID: "dummy"}

	// Now complete query - channel is full, should not panic
	event := &runnerv1.SandboxesStatusEvent{
		RequestId: requestID,
		Sandboxes: []*runnerv1.SandboxStatus{},
	}

	// Should not panic even with full channel
	svc.CompleteQuery(requestID, 1, event)
}

func TestSandboxQueryService_MultipleSandboxes(t *testing.T) {
	svc := NewSandboxQueryService(nil)
	defer svc.Stop()

	requestID := "multi-sandbox"
	ch := svc.RegisterQuery(requestID)

	// Complete with multiple sandboxes
	event := &runnerv1.SandboxesStatusEvent{
		RequestId: requestID,
		Sandboxes: []*runnerv1.SandboxStatus{
			{PodKey: "pod-1", Exists: true, CanResume: true},
			{PodKey: "pod-2", Exists: true, CanResume: false, Error: "session file missing"},
			{PodKey: "pod-3", Exists: false},
		},
	}

	svc.CompleteQuery(requestID, 99, event)

	select {
	case result := <-ch:
		if len(result.Sandboxes) != 3 {
			t.Errorf("Sandboxes len = %d, want 3", len(result.Sandboxes))
		}
		// Check each sandbox
		for i, sb := range result.Sandboxes {
			expectedKey := "pod-" + string(rune('1'+i))
			if sb.PodKey != expectedKey {
				t.Logf("Sandbox %d: PodKey = %s", i, sb.PodKey)
			}
		}
		// Check specific fields
		if result.Sandboxes[1].Error != "session file missing" {
			t.Errorf("Sandbox 2 error = %s, want 'session file missing'", result.Sandboxes[1].Error)
		}
		if result.Sandboxes[2].Exists {
			t.Error("Sandbox 3 should not exist")
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for result")
	}
}
