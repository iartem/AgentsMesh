package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/anthropics/agentmesh/runner/internal/mcp/tools"
)

func TestNewBackendClient(t *testing.T) {
	client := NewBackendClient("http://localhost:8080", "test-session")

	if client == nil {
		t.Fatal("NewBackendClient returned nil")
	}

	if client.baseURL != "http://localhost:8080" {
		t.Errorf("baseURL: got %v, want %v", client.baseURL, "http://localhost:8080")
	}

	if client.sessionKey != "test-session" {
		t.Errorf("sessionKey: got %v, want %v", client.sessionKey, "test-session")
	}

	if client.httpClient == nil {
		t.Error("httpClient should not be nil")
	}
}

func TestSetSessionKey(t *testing.T) {
	client := NewBackendClient("http://localhost:8080", "old-session")
	client.SetSessionKey("new-session")

	if client.sessionKey != "new-session" {
		t.Errorf("sessionKey: got %v, want %v", client.sessionKey, "new-session")
	}
}

func TestObserveTerminal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method: got %v, want GET", r.Method)
		}

		if r.Header.Get("X-Session-Key") != "test-session" {
			t.Errorf("X-Session-Key: got %v, want test-session", r.Header.Get("X-Session-Key"))
		}

		resp := tools.TerminalOutput{
			SessionKey: "target-session",
			Output:     "test output",
			CursorX:    10,
			CursorY:    5,
			TotalLines: 100,
			HasMore:    true,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	result, err := client.ObserveTerminal(context.Background(), "target-session", 50, false, true)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Output != "test output" {
		t.Errorf("Output: got %v, want test output", result.Output)
	}

	if result.CursorX != 10 {
		t.Errorf("CursorX: got %v, want 10", result.CursorX)
	}
}

func TestSendTerminalText(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %v, want POST", r.Method)
		}

		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		if body["text"] != "hello world" {
			t.Errorf("text: got %v, want hello world", body["text"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	err := client.SendTerminalText(context.Background(), "target-session", "hello world")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSendTerminalKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		keys, ok := body["keys"].([]interface{})
		if !ok || len(keys) != 2 {
			t.Errorf("keys: got %v, want 2 keys", body["keys"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	err := client.SendTerminalKey(context.Background(), "target-session", []string{"ctrl+c", "enter"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListAvailableSessions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"sessions": []tools.AvailableSession{
				{SessionKey: "session-1", Status: tools.SessionStatusRunning},
				{SessionKey: "session-2", Status: tools.SessionStatusRunning},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	sessions, err := client.ListAvailableSessions(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 2 {
		t.Errorf("sessions count: got %v, want 2", len(sessions))
	}
}

func TestRequestBinding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		if body["target_session"] != "target-session" {
			t.Errorf("target_session: got %v, want target-session", body["target_session"])
		}

		resp := tools.Binding{
			ID:               1,
			InitiatorSession: "test-session",
			TargetSession:    "target-session",
			Status:           tools.BindingStatusPending,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	binding, err := client.RequestBinding(context.Background(), "target-session", []tools.BindingScope{tools.ScopeTerminalRead})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if binding.Status != tools.BindingStatusPending {
		t.Errorf("status: got %v, want pending", binding.Status)
	}
}

func TestAcceptBinding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		if body["binding_id"].(float64) != 1 {
			t.Errorf("binding_id: got %v, want 1", body["binding_id"])
		}

		resp := tools.Binding{
			ID:     1,
			Status: tools.BindingStatusActive,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	binding, err := client.AcceptBinding(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if binding.Status != tools.BindingStatusActive {
		t.Errorf("status: got %v, want active", binding.Status)
	}
}

func TestRejectBinding(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		if body["reason"] != "not allowed" {
			t.Errorf("reason: got %v, want not allowed", body["reason"])
		}

		resp := tools.Binding{
			ID:     1,
			Status: tools.BindingStatusRejected,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	binding, err := client.RejectBinding(context.Background(), 1, "not allowed")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if binding.Status != tools.BindingStatusRejected {
		t.Errorf("status: got %v, want rejected", binding.Status)
	}
}

func TestUnbindSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	err := client.UnbindSession(context.Background(), "target-session")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetBindings(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"bindings": []tools.Binding{
				{ID: 1, Status: tools.BindingStatusActive},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	bindings, err := client.GetBindings(context.Background(), nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(bindings) != 1 {
		t.Errorf("bindings count: got %v, want 1", len(bindings))
	}
}

func TestGetBindingsWithStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("status") != "active" {
			t.Errorf("status param: got %v, want active", r.URL.Query().Get("status"))
		}

		resp := map[string]interface{}{
			"bindings": []tools.Binding{},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	status := tools.BindingStatusActive
	_, err := client.GetBindings(context.Background(), &status)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGetBoundSessions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"sessions": []tools.AvailableSession{
				{SessionKey: "bound-session"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	sessions, err := client.GetBoundSessions(context.Background())

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(sessions) != 1 {
		t.Errorf("sessions count: got %v, want 1", len(sessions))
	}
}

func TestSearchChannels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("name") != "test" {
			t.Errorf("name param: got %v, want test", r.URL.Query().Get("name"))
		}

		resp := map[string]interface{}{
			"channels": []tools.Channel{
				{ID: 1, Name: "test-channel"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	channels, err := client.SearchChannels(context.Background(), "test", nil, nil, nil, 0, 20)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(channels) != 1 {
		t.Errorf("channels count: got %v, want 1", len(channels))
	}
}

func TestCreateChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		if body["name"] != "new-channel" {
			t.Errorf("name: got %v, want new-channel", body["name"])
		}

		resp := tools.Channel{
			ID:   1,
			Name: "new-channel",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	channel, err := client.CreateChannel(context.Background(), "new-channel", "description", nil, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if channel.Name != "new-channel" {
		t.Errorf("name: got %v, want new-channel", channel.Name)
	}
}

func TestGetChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := tools.Channel{
			ID:   1,
			Name: "test-channel",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	channel, err := client.GetChannel(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if channel.ID != 1 {
		t.Errorf("ID: got %v, want 1", channel.ID)
	}
}

func TestSendMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		if body["content"] != "Hello" {
			t.Errorf("content: got %v, want Hello", body["content"])
		}

		resp := tools.ChannelMessage{
			ID:      1,
			Content: "Hello",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	msg, err := client.SendMessage(context.Background(), 1, "Hello", tools.ChannelMessageTypeText, nil, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if msg.Content != "Hello" {
		t.Errorf("content: got %v, want Hello", msg.Content)
	}
}

func TestGetMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"messages": []tools.ChannelMessage{
				{ID: 1, Content: "Message 1"},
				{ID: 2, Content: "Message 2"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	messages, err := client.GetMessages(context.Background(), 1, nil, nil, nil, 50)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(messages) != 2 {
		t.Errorf("messages count: got %v, want 2", len(messages))
	}
}

func TestGetDocument(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"document": "test document content",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	doc, err := client.GetDocument(context.Background(), 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if doc != "test document content" {
		t.Errorf("document: got %v, want test document content", doc)
	}
}

func TestUpdateDocument(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		if body["document"] != "updated content" {
			t.Errorf("document: got %v, want updated content", body["document"])
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	err := client.UpdateDocument(context.Background(), 1, "updated content")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSearchTickets(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"tickets": []tools.Ticket{
				{ID: 1, Identifier: "AM-1", Title: "Test Ticket"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	tickets, err := client.SearchTickets(context.Background(), nil, nil, nil, nil, nil, nil, "", 20, 1)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(tickets) != 1 {
		t.Errorf("tickets count: got %v, want 1", len(tickets))
	}
}

func TestGetTicket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := tools.Ticket{
			ID:         1,
			Identifier: "AM-123",
			Title:      "Test Ticket",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	ticket, err := client.GetTicket(context.Background(), "AM-123")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ticket.Identifier != "AM-123" {
		t.Errorf("identifier: got %v, want AM-123", ticket.Identifier)
	}
}

func TestCreateTicket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		if body["title"] != "New Ticket" {
			t.Errorf("title: got %v, want New Ticket", body["title"])
		}

		resp := tools.Ticket{
			ID:         1,
			Identifier: "AM-1",
			Title:      "New Ticket",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	ticket, err := client.CreateTicket(context.Background(), 1, "New Ticket", "Description", tools.TicketTypeTask, tools.TicketPriorityMedium, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ticket.Title != "New Ticket" {
		t.Errorf("title: got %v, want New Ticket", ticket.Title)
	}
}

func TestUpdateTicket(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		json.NewDecoder(r.Body).Decode(&body)

		if body["title"] != "Updated Title" {
			t.Errorf("title: got %v, want Updated Title", body["title"])
		}

		resp := tools.Ticket{
			ID:    1,
			Title: "Updated Title",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	title := "Updated Title"
	ticket, err := client.UpdateTicket(context.Background(), "AM-1", &title, nil, nil, nil, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ticket.Title != "Updated Title" {
		t.Errorf("title: got %v, want Updated Title", ticket.Title)
	}
}

func TestCreateSession(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body tools.SessionCreateRequest
		json.NewDecoder(r.Body).Decode(&body)

		if body.InitialPrompt != "Hello" {
			t.Errorf("initial_prompt: got %v, want Hello", body.InitialPrompt)
		}

		resp := tools.SessionCreateResponse{
			SessionKey: "new-session",
			Status:     "created",
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	resp, err := client.CreateSession(context.Background(), &tools.SessionCreateRequest{
		InitialPrompt: "Hello",
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.SessionKey != "new-session" {
		t.Errorf("session_key: got %v, want new-session", resp.SessionKey)
	}
}

func TestRequestError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	client := NewBackendClient(server.URL, "test-session")
	_, err := client.GetTicket(context.Background(), "AM-1")

	if err == nil {
		t.Error("expected error for 500 response")
	}
}

func TestBackendClientImplementsInterface(t *testing.T) {
	var _ tools.CollaborationClient = (*BackendClient)(nil)
}
