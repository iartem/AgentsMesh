package mcp

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

// Tests for Server methods that don't require subprocess

func TestServerGetToolsWithData(t *testing.T) {
	server := NewServer(&Config{Name: "test", Command: "/bin/echo"})

	// Add tools manually
	server.tools["tool1"] = &Tool{Name: "tool1", Description: "Tool 1"}
	server.tools["tool2"] = &Tool{Name: "tool2", Description: "Tool 2"}

	tools := server.GetTools()
	if len(tools) != 2 {
		t.Errorf("tools count: got %v, want 2", len(tools))
	}
}

func TestServerGetResourcesWithData(t *testing.T) {
	server := NewServer(&Config{Name: "test", Command: "/bin/echo"})

	// Add resources manually
	server.resources["res1"] = &Resource{URI: "file://test1.txt", Name: "test1.txt"}
	server.resources["res2"] = &Resource{URI: "file://test2.txt", Name: "test2.txt"}

	resources := server.GetResources()
	if len(resources) != 2 {
		t.Errorf("resources count: got %v, want 2", len(resources))
	}
}

func TestServerStopWithPendingChannels(t *testing.T) {
	server := NewServer(&Config{Name: "test", Command: "/bin/echo"})

	// Add pending channels manually
	ch1 := make(chan *Response, 1)
	ch2 := make(chan *Response, 1)
	server.pending[1] = ch1
	server.pending[2] = ch2
	server.running = true

	// Stop should close all pending channels
	err := server.Stop()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify channels are closed
	select {
	case _, ok := <-ch1:
		if ok {
			t.Error("ch1 should be closed")
		}
	default:
		t.Error("ch1 should be closed and return immediately")
	}

	select {
	case _, ok := <-ch2:
		if ok {
			t.Error("ch2 should be closed")
		}
	default:
		t.Error("ch2 should be closed and return immediately")
	}

	// Pending should be cleared
	if len(server.pending) != 0 {
		t.Errorf("pending should be cleared, got %v", len(server.pending))
	}

	if server.running {
		t.Error("server should not be running")
	}
}

func TestServerSendWhenNotRunning(t *testing.T) {
	server := NewServer(&Config{Name: "test", Command: "/bin/echo"})

	req := &Request{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "test",
	}

	err := server.send(req)
	if err == nil {
		t.Error("expected error when server not running")
	}

	if err.Error() != "server not running" {
		t.Errorf("error message: got %v, want 'server not running'", err)
	}
}

func TestServerNotifyWhenNotRunning(t *testing.T) {
	server := NewServer(&Config{Name: "test", Command: "/bin/echo"})

	err := server.notify("test", nil)
	if err == nil {
		t.Error("expected error when server not running")
	}
}

func TestServerNotifyWithParams(t *testing.T) {
	server := NewServer(&Config{Name: "test", Command: "/bin/echo"})
	server.running = true

	// Will fail because stdin is nil
	params := map[string]interface{}{
		"key": "value",
	}

	err := server.notify("test", params)
	// Should fail because stdin is nil
	if err == nil {
		t.Error("expected error when stdin is nil")
	}
	if err.Error() != "stdin not available" {
		t.Errorf("error message: got %v, want 'stdin not available'", err)
	}
}

func TestServerCallContextCancelled(t *testing.T) {
	server := NewServer(&Config{Name: "test", Command: "/bin/echo"})
	server.running = true

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Use a mockable stdin
	r, w, _ := os.Pipe()
	server.stdin = w
	defer r.Close()
	defer w.Close()

	// call should return context cancelled error
	_, err := server.call(ctx, "test", nil)
	if err == nil {
		t.Error("expected error for cancelled context")
	}
}

func TestServerStartAlreadyRunning(t *testing.T) {
	server := NewServer(&Config{Name: "test", Command: "/bin/echo"})
	server.running = true

	err := server.Start(context.Background())
	if err == nil {
		t.Error("expected error when already running")
	}

	if err.Error() != "server already running" {
		t.Errorf("error message: got %v, want 'server already running'", err)
	}
}

func TestServerCallWithParamsMarshalError(t *testing.T) {
	server := NewServer(&Config{Name: "test", Command: "/bin/echo"})
	server.running = true

	r, w, _ := os.Pipe()
	server.stdin = w
	defer r.Close()
	defer w.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Use unmarshalable params (channel can't be JSON marshaled)
	params := make(chan int)

	_, err := server.call(ctx, "test", params)
	if err == nil {
		t.Error("expected error for unmarshalable params")
	}
}

// Test response routing through pending channels
func TestServerResponseRouting(t *testing.T) {
	server := NewServer(&Config{Name: "test", Command: "/bin/echo"})

	// Set up pending channel
	ch := make(chan *Response, 1)
	server.mu.Lock()
	server.pending[42] = ch
	server.mu.Unlock()

	// Simulate response routing
	resp := &Response{
		JSONRPC: "2.0",
		ID:      42,
		Result:  json.RawMessage(`{"success": true}`),
	}

	server.mu.Lock()
	if pendingCh, ok := server.pending[resp.ID]; ok {
		pendingCh <- resp
	}
	server.mu.Unlock()

	// Verify response was routed
	select {
	case got := <-ch:
		if got.ID != 42 {
			t.Errorf("response ID: got %v, want 42", got.ID)
		}
	case <-time.After(time.Second):
		t.Error("timeout waiting for response")
	}
}

func TestServerResponseRoutingUnknownID(t *testing.T) {
	server := NewServer(&Config{Name: "test", Command: "/bin/echo"})

	// No pending channel for ID 999
	resp := &Response{
		JSONRPC: "2.0",
		ID:      999,
		Result:  json.RawMessage(`{}`),
	}

	// Should not panic when routing to unknown ID
	server.mu.Lock()
	if pendingCh, ok := server.pending[resp.ID]; ok {
		pendingCh <- resp
	}
	server.mu.Unlock()
	// No assertion needed - just verifying no panic
}

// Test error parsing in CallTool
func TestCallToolErrorResponse(t *testing.T) {
	// This tests the error response handling code path
	// We can't easily test the actual CallTool without a real server,
	// but we can test the error struct
	errResp := &Response{
		JSONRPC: "2.0",
		ID:      1,
		Error: &Error{
			Code:    -32600,
			Message: "Invalid Request",
		},
	}

	if errResp.Error == nil {
		t.Error("error should not be nil")
	}

	if errResp.Error.Code != -32600 {
		t.Errorf("error code: got %v, want -32600", errResp.Error.Code)
	}
}

// Test ReadResource result parsing
func TestReadResourceResultParsing(t *testing.T) {
	// Test the result struct used in ReadResource
	jsonStr := `{
		"contents": [
			{
				"uri": "file:///test.txt",
				"mimeType": "text/plain",
				"text": "Hello, World!"
			}
		]
	}`

	var result struct {
		Contents []struct {
			URI      string `json:"uri"`
			MimeType string `json:"mimeType,omitempty"`
			Text     string `json:"text,omitempty"`
			Blob     string `json:"blob,omitempty"`
		} `json:"contents"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(result.Contents) != 1 {
		t.Errorf("contents count: got %v, want 1", len(result.Contents))
	}

	if result.Contents[0].Text != "Hello, World!" {
		t.Errorf("text: got %v, want 'Hello, World!'", result.Contents[0].Text)
	}
}

func TestReadResourceResultParsingBlob(t *testing.T) {
	// Test the result struct with blob content
	jsonStr := `{
		"contents": [
			{
				"uri": "file:///test.bin",
				"mimeType": "application/octet-stream",
				"blob": "SGVsbG8="
			}
		]
	}`

	var result struct {
		Contents []struct {
			URI      string `json:"uri"`
			MimeType string `json:"mimeType,omitempty"`
			Text     string `json:"text,omitempty"`
			Blob     string `json:"blob,omitempty"`
		} `json:"contents"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result.Contents[0].Blob != "SGVsbG8=" {
		t.Errorf("blob: got %v, want 'SGVsbG8='", result.Contents[0].Blob)
	}
}

func TestReadResourceEmptyContents(t *testing.T) {
	// Test empty contents parsing
	jsonStr := `{"contents": []}`

	var result struct {
		Contents []struct {
			URI      string `json:"uri"`
			MimeType string `json:"mimeType,omitempty"`
			Text     string `json:"text,omitempty"`
		} `json:"contents"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(result.Contents) != 0 {
		t.Errorf("contents should be empty, got %v", len(result.Contents))
	}
}

// Test CallTool result parsing
func TestCallToolResultParsing(t *testing.T) {
	jsonStr := `{
		"content": [
			{
				"type": "text",
				"text": "Result text"
			}
		],
		"isError": false
	}`

	var result struct {
		Content []struct {
			Type string          `json:"type"`
			Text string          `json:"text,omitempty"`
			Data json.RawMessage `json:"data,omitempty"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result.IsError {
		t.Error("isError should be false")
	}

	if len(result.Content) != 1 {
		t.Errorf("content count: got %v, want 1", len(result.Content))
	}

	if result.Content[0].Text != "Result text" {
		t.Errorf("text: got %v, want 'Result text'", result.Content[0].Text)
	}
}

func TestCallToolResultIsError(t *testing.T) {
	jsonStr := `{
		"content": [
			{
				"type": "text",
				"text": "Error message"
			}
		],
		"isError": true
	}`

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text,omitempty"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if !result.IsError {
		t.Error("isError should be true")
	}
}

// Test initialize response parsing
func TestInitializeResponseParsing(t *testing.T) {
	jsonStr := `{
		"protocolVersion": "2024-11-05",
		"capabilities": {
			"tools": {"listChanged": true},
			"resources": {"subscribe": true, "listChanged": true}
		},
		"serverInfo": {
			"name": "Test Server",
			"version": "1.0.0"
		}
	}`

	var result struct {
		ProtocolVersion string `json:"protocolVersion"`
		Capabilities    struct {
			Tools struct {
				ListChanged bool `json:"listChanged"`
			} `json:"tools"`
			Resources struct {
				Subscribe   bool `json:"subscribe"`
				ListChanged bool `json:"listChanged"`
			} `json:"resources"`
		} `json:"capabilities"`
		ServerInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"serverInfo"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if result.ProtocolVersion != "2024-11-05" {
		t.Errorf("protocolVersion: got %v, want '2024-11-05'", result.ProtocolVersion)
	}

	if !result.Capabilities.Tools.ListChanged {
		t.Error("tools.listChanged should be true")
	}

	if result.ServerInfo.Name != "Test Server" {
		t.Errorf("serverInfo.name: got %v, want 'Test Server'", result.ServerInfo.Name)
	}
}

// Test tools/list response parsing
func TestToolsListResponseParsing(t *testing.T) {
	jsonStr := `{
		"tools": [
			{
				"name": "read_file",
				"description": "Read a file from disk",
				"inputSchema": {"type": "object"}
			},
			{
				"name": "write_file",
				"description": "Write a file to disk",
				"inputSchema": {"type": "object"}
			}
		]
	}`

	var result struct {
		Tools []Tool `json:"tools"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(result.Tools) != 2 {
		t.Errorf("tools count: got %v, want 2", len(result.Tools))
	}

	if result.Tools[0].Name != "read_file" {
		t.Errorf("tool name: got %v, want 'read_file'", result.Tools[0].Name)
	}
}

// Test resources/list response parsing
func TestResourcesListResponseParsing(t *testing.T) {
	jsonStr := `{
		"resources": [
			{
				"uri": "file:///home/user/docs",
				"name": "Documents",
				"description": "User documents",
				"mimeType": "inode/directory"
			}
		]
	}`

	var result struct {
		Resources []Resource `json:"resources"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	if len(result.Resources) != 1 {
		t.Errorf("resources count: got %v, want 1", len(result.Resources))
	}

	if result.Resources[0].URI != "file:///home/user/docs" {
		t.Errorf("resource uri: got %v, want 'file:///home/user/docs'", result.Resources[0].URI)
	}
}

// Test concurrent access to tools and resources
func TestServerConcurrentAccess(t *testing.T) {
	server := NewServer(&Config{Name: "test", Command: "/bin/echo"})

	// Pre-populate
	server.tools["tool1"] = &Tool{Name: "tool1"}
	server.resources["res1"] = &Resource{URI: "file://res1"}

	done := make(chan bool, 4)

	// Concurrent GetTools
	go func() {
		for i := 0; i < 100; i++ {
			_ = server.GetTools()
		}
		done <- true
	}()

	// Concurrent GetResources
	go func() {
		for i := 0; i < 100; i++ {
			_ = server.GetResources()
		}
		done <- true
	}()

	// Concurrent IsRunning
	go func() {
		for i := 0; i < 100; i++ {
			_ = server.IsRunning()
		}
		done <- true
	}()

	// Concurrent Name
	go func() {
		for i := 0; i < 100; i++ {
			_ = server.Name()
		}
		done <- true
	}()

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}
}
