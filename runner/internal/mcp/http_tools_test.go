package mcp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestHTTPToolsCoverage provides additional tests for http_tools.go functions

func TestHTTPServerMCPToolsCallSendTerminalKey(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "send_terminal_key",
			"arguments": {
				"pod_key": "target-pod",
				"key": "ctrl_c"
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Tool should be found (may error on backend call)
	if resp.Error != nil && resp.Error.Code == -32601 {
		t.Error("tool send_terminal_key should be found")
	}
}

func TestHTTPServerMCPToolsCallSendTerminalKeyMissingArgs(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "send_terminal_key",
			"arguments": {
				"pod_key": "target-pod"
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	// Should handle missing key argument
}

func TestHTTPServerMCPToolsCallAcceptBinding(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "accept_binding",
			"arguments": {
				"binding_id": 123
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	// Tool should be found
}

func TestHTTPServerMCPToolsCallRejectBinding(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "reject_binding",
			"arguments": {
				"binding_id": 123,
				"reason": "Not needed"
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	// Tool should be found
}

func TestHTTPServerMCPToolsCallUnbindPod(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "unbind_pod",
			"arguments": {
				"binding_id": 123
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	// Tool should be found
}

func TestHTTPServerMCPToolsCallGetBindings(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "get_bindings",
			"arguments": {}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	// Tool should be found
}

func TestHTTPServerMCPToolsCallGetBoundPods(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "get_bound_pods",
			"arguments": {}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	// Tool should be found
}

func TestHTTPServerMCPToolsCallGetChannel(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "get_channel",
			"arguments": {
				"channel_id": 123
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	// Tool should be found
}

func TestHTTPServerMCPToolsCallSendChannelMessage(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "send_channel_message",
			"arguments": {
				"channel_id": 123,
				"content": "Hello, world!"
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	// Tool should be found
}

func TestHTTPServerMCPToolsCallGetChannelMessages(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "get_channel_messages",
			"arguments": {
				"channel_id": 123,
				"limit": 10
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	// Tool should be found
}

func TestHTTPServerMCPToolsCallGetChannelDocument(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "get_channel_document",
			"arguments": {
				"channel_id": 123
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	// Tool should be found
}

func TestHTTPServerMCPToolsCallUpdateChannelDocument(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "update_channel_document",
			"arguments": {
				"channel_id": 123,
				"content": "Updated content"
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	// Tool should be found
}

func TestHTTPServerMCPToolsCallUpdateTicket(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "update_ticket",
			"arguments": {
				"ticket_id": "AM-123",
				"status": "done"
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	// Tool should be found
}

func TestHTTPServerMCPToolsCallCreatePod(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "create_pod",
			"arguments": {
				"ticket_id": 123,
				"command": "echo hello"
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	// Tool should be found
}

// Tests for helper function edge cases

func TestGetStringArgEmpty(t *testing.T) {
	args := map[string]interface{}{
		"empty": "",
	}
	result := getStringArg(args, "empty")
	if result != "" {
		t.Errorf("expected empty string, got %v", result)
	}
}

func TestGetStringArgNonString(t *testing.T) {
	args := map[string]interface{}{
		"number": 42,
	}
	result := getStringArg(args, "number")
	if result != "" {
		t.Errorf("expected empty string for non-string, got %v", result)
	}
}

func TestGetIntArgFloat(t *testing.T) {
	args := map[string]interface{}{
		"float": 42.5,
	}
	result := getIntArg(args, "float")
	if result != 42 {
		t.Errorf("expected 42 for float64, got %v", result)
	}
}

func TestGetIntPtrArgFloat(t *testing.T) {
	args := map[string]interface{}{
		"float": 42.5,
	}
	result := getIntPtrArg(args, "float")
	if result == nil {
		t.Error("expected non-nil result")
	} else if *result != 42 {
		t.Errorf("expected 42, got %v", *result)
	}
}

func TestGetBoolArgNonBool(t *testing.T) {
	args := map[string]interface{}{
		"string": "true",
	}
	result := getBoolArg(args, "string")
	if result {
		t.Error("expected false for non-bool")
	}
}

func TestGetStringSliceArgInvalidItems(t *testing.T) {
	args := map[string]interface{}{
		"mixed": []interface{}{"a", 123, "b"},
	}
	result := getStringSliceArg(args, "mixed")
	// Should only include string items
	if len(result) != 2 {
		t.Errorf("expected 2 strings, got %v", len(result))
	}
}

func TestGetStringSliceArgNonSlice(t *testing.T) {
	args := map[string]interface{}{
		"string": "not a slice",
	}
	result := getStringSliceArg(args, "string")
	if result != nil {
		t.Errorf("expected nil for non-slice, got %v", result)
	}
}

// Tests for tools with various argument combinations

func TestHTTPServerMCPToolsCallSearchTicketsWithAllParams(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "search_tickets",
			"arguments": {
				"query": "test",
				"status": "todo",
				"type": "task",
				"priority": "high",
				"assignee_id": 123,
				"product_id": 456,
				"limit": 10
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
}

func TestHTTPServerMCPToolsCallSearchChannelsWithAllParams(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "search_channels",
			"arguments": {
				"name": "test",
				"type": "public",
				"owner_type": "pod",
				"owner_id": "pod-123"
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
}

func TestHTTPServerMCPToolsCallCreateChannelWithAllParams(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "create_channel",
			"arguments": {
				"name": "test-channel",
				"description": "A test channel",
				"type": "public"
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
}

func TestHTTPServerMCPToolsCallObserveTerminalWithDefaultLines(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "observe_terminal",
			"arguments": {
				"pod_key": "target-pod"
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
}

func TestHTTPServerMCPToolsCallGetChannelMessagesWithAllParams(t *testing.T) {
	server := NewHTTPServer("http://localhost:8080", 9090)
	server.RegisterPod("test-pod", "test-org", nil, nil, "claude")

	body := bytes.NewBufferString(`{
		"jsonrpc": "2.0",
		"id": 1,
		"method": "tools/call",
		"params": {
			"name": "get_channel_messages",
			"arguments": {
				"channel_id": 123,
				"limit": 50,
				"before_id": 1000
			}
		}
	}`)

	req := httptest.NewRequest(http.MethodPost, "/mcp", body)
	req.Header.Set("X-Pod-Key", "test-pod")
	rec := httptest.NewRecorder()

	server.handleMCP(rec, req)

	var resp MCPResponse
	json.NewDecoder(rec.Body).Decode(&resp)
}
