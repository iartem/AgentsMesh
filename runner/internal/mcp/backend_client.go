package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/anthropics/agentmesh/runner/internal/mcp/tools"
)

// BackendClient calls the AgentMesh Backend API for collaboration operations.
type BackendClient struct {
	baseURL    string
	sessionKey string
	httpClient *http.Client
}

// NewBackendClient creates a new backend API client.
func NewBackendClient(baseURL, sessionKey string) *BackendClient {
	return &BackendClient{
		baseURL:    baseURL,
		sessionKey: sessionKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetSessionKey updates the session key for the client.
func (c *BackendClient) SetSessionKey(sessionKey string) {
	c.sessionKey = sessionKey
}

// request makes an HTTP request to the backend.
func (c *BackendClient) request(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	fullURL := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Session-Key", c.sessionKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// Terminal Operations

// ObserveTerminal gets terminal output from another session.
func (c *BackendClient) ObserveTerminal(ctx context.Context, sessionKey string, lines int, raw bool, includeScreen bool) (*tools.TerminalOutput, error) {
	params := url.Values{}
	params.Set("lines", strconv.Itoa(lines))
	params.Set("raw", strconv.FormatBool(raw))
	params.Set("include_screen", strconv.FormatBool(includeScreen))

	path := fmt.Sprintf("/api/v1/sessions/%s/terminal/observe?%s", url.PathEscape(sessionKey), params.Encode())

	var result tools.TerminalOutput
	err := c.request(ctx, http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SendTerminalText sends text input to a terminal.
func (c *BackendClient) SendTerminalText(ctx context.Context, sessionKey string, text string) error {
	body := map[string]interface{}{
		"text": text,
	}
	return c.request(ctx, http.MethodPost, fmt.Sprintf("/api/v1/sessions/%s/terminal/input", url.PathEscape(sessionKey)), body, nil)
}

// SendTerminalKey sends special keys to a terminal.
func (c *BackendClient) SendTerminalKey(ctx context.Context, sessionKey string, keys []string) error {
	body := map[string]interface{}{
		"keys": keys,
	}
	return c.request(ctx, http.MethodPost, fmt.Sprintf("/api/v1/sessions/%s/terminal/input", url.PathEscape(sessionKey)), body, nil)
}

// Discovery Operations

// ListAvailableSessions lists sessions available for collaboration.
func (c *BackendClient) ListAvailableSessions(ctx context.Context) ([]tools.AvailableSession, error) {
	var result struct {
		Sessions []tools.AvailableSession `json:"sessions"`
	}
	// Use sessions endpoint with status filter
	err := c.request(ctx, http.MethodGet, "/api/v1/sessions?status=running", nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Sessions, nil
}

// Binding Operations

// RequestBinding requests a binding with another session.
func (c *BackendClient) RequestBinding(ctx context.Context, targetSession string, scopes []tools.BindingScope) (*tools.Binding, error) {
	body := map[string]interface{}{
		"target_session": targetSession,
		"scopes":         scopes,
	}

	var result tools.Binding
	err := c.request(ctx, http.MethodPost, "/api/v1/bindings", body, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// AcceptBinding accepts a binding request.
func (c *BackendClient) AcceptBinding(ctx context.Context, bindingID int) (*tools.Binding, error) {
	body := map[string]interface{}{
		"binding_id": bindingID,
	}

	var result tools.Binding
	err := c.request(ctx, http.MethodPost, "/api/v1/bindings/accept", body, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// RejectBinding rejects a binding request.
func (c *BackendClient) RejectBinding(ctx context.Context, bindingID int, reason string) (*tools.Binding, error) {
	body := map[string]interface{}{
		"binding_id": bindingID,
		"reason":     reason,
	}

	var result tools.Binding
	err := c.request(ctx, http.MethodPost, "/api/v1/bindings/reject", body, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// UnbindSession unbinds from another session.
func (c *BackendClient) UnbindSession(ctx context.Context, targetSession string) error {
	body := map[string]interface{}{
		"target_session": targetSession,
	}
	return c.request(ctx, http.MethodPost, "/api/v1/bindings/unbind", body, nil)
}

// GetBindings gets all bindings for the current session.
func (c *BackendClient) GetBindings(ctx context.Context, status *tools.BindingStatus) ([]tools.Binding, error) {
	path := "/api/v1/bindings"
	if status != nil {
		path += "?status=" + url.QueryEscape(string(*status))
	}

	var result struct {
		Bindings []tools.Binding `json:"bindings"`
	}
	err := c.request(ctx, http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Bindings, nil
}

// GetBoundSessions gets sessions that are bound to the current session.
func (c *BackendClient) GetBoundSessions(ctx context.Context) ([]tools.AvailableSession, error) {
	var result struct {
		Sessions []tools.AvailableSession `json:"sessions"`
	}
	err := c.request(ctx, http.MethodGet, "/api/v1/bindings/sessions", nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Sessions, nil
}

// Channel Operations

// SearchChannels searches for collaboration channels.
func (c *BackendClient) SearchChannels(ctx context.Context, name string, projectID, ticketID *int, isArchived *bool, offset, limit int) ([]tools.Channel, error) {
	params := url.Values{}
	if name != "" {
		params.Set("name", name)
	}
	if projectID != nil {
		params.Set("project_id", strconv.Itoa(*projectID))
	}
	if ticketID != nil {
		params.Set("ticket_id", strconv.Itoa(*ticketID))
	}
	if isArchived != nil {
		params.Set("is_archived", strconv.FormatBool(*isArchived))
	}
	params.Set("offset", strconv.Itoa(offset))
	params.Set("limit", strconv.Itoa(limit))

	path := "/api/v1/channels?" + params.Encode()

	var result struct {
		Channels []tools.Channel `json:"channels"`
	}
	err := c.request(ctx, http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Channels, nil
}

// CreateChannel creates a new collaboration channel.
func (c *BackendClient) CreateChannel(ctx context.Context, name, description string, projectID, ticketID *int) (*tools.Channel, error) {
	body := map[string]interface{}{
		"name":        name,
		"description": description,
	}
	if projectID != nil {
		body["project_id"] = *projectID
	}
	if ticketID != nil {
		body["ticket_id"] = *ticketID
	}

	var result tools.Channel
	err := c.request(ctx, http.MethodPost, "/api/v1/channels", body, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetChannel gets a channel by ID.
func (c *BackendClient) GetChannel(ctx context.Context, channelID int) (*tools.Channel, error) {
	var result tools.Channel
	err := c.request(ctx, http.MethodGet, fmt.Sprintf("/api/v1/channels/%d", channelID), nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// SendMessage sends a message to a channel.
func (c *BackendClient) SendMessage(ctx context.Context, channelID int, content string, msgType tools.ChannelMessageType, mentions []string, replyTo *int) (*tools.ChannelMessage, error) {
	body := map[string]interface{}{
		"content":      content,
		"message_type": msgType,
	}
	if len(mentions) > 0 {
		body["mentions"] = mentions
	}
	if replyTo != nil {
		body["reply_to"] = *replyTo
	}

	var result tools.ChannelMessage
	err := c.request(ctx, http.MethodPost, fmt.Sprintf("/api/v1/channels/%d/messages", channelID), body, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// GetMessages gets messages from a channel.
func (c *BackendClient) GetMessages(ctx context.Context, channelID int, beforeTime, afterTime *string, mentionedSession *string, limit int) ([]tools.ChannelMessage, error) {
	params := url.Values{}
	if beforeTime != nil {
		params.Set("before_time", *beforeTime)
	}
	if afterTime != nil {
		params.Set("after_time", *afterTime)
	}
	if mentionedSession != nil {
		params.Set("mentioned_session", *mentionedSession)
	}
	params.Set("limit", strconv.Itoa(limit))

	path := fmt.Sprintf("/api/v1/channels/%d/messages?%s", channelID, params.Encode())

	var result struct {
		Messages []tools.ChannelMessage `json:"messages"`
	}
	err := c.request(ctx, http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Messages, nil
}

// GetDocument gets the shared document from a channel.
func (c *BackendClient) GetDocument(ctx context.Context, channelID int) (string, error) {
	var result struct {
		Document string `json:"document"`
	}
	err := c.request(ctx, http.MethodGet, fmt.Sprintf("/api/v1/channels/%d/document", channelID), nil, &result)
	if err != nil {
		return "", err
	}
	return result.Document, nil
}

// UpdateDocument updates the shared document in a channel.
func (c *BackendClient) UpdateDocument(ctx context.Context, channelID int, document string) error {
	body := map[string]interface{}{
		"document": document,
	}
	return c.request(ctx, http.MethodPut, fmt.Sprintf("/api/v1/channels/%d/document", channelID), body, nil)
}

// Ticket Operations

// SearchTickets searches for tickets.
func (c *BackendClient) SearchTickets(ctx context.Context, productID *int, status *tools.TicketStatus, ticketType *tools.TicketType, priority *tools.TicketPriority, assigneeID, parentID *int, query string, limit, page int) ([]tools.Ticket, error) {
	params := url.Values{}
	params.Set("limit", strconv.Itoa(limit))
	params.Set("page", strconv.Itoa(page))

	if productID != nil {
		params.Set("product_id", strconv.Itoa(*productID))
	}
	if status != nil {
		params.Set("status", string(*status))
	}
	if ticketType != nil {
		params.Set("type", string(*ticketType))
	}
	if priority != nil {
		params.Set("priority", string(*priority))
	}
	if assigneeID != nil {
		params.Set("assignee_id", strconv.Itoa(*assigneeID))
	}
	if parentID != nil {
		params.Set("parent_id", strconv.Itoa(*parentID))
	}
	if query != "" {
		params.Set("query", query)
	}

	path := "/api/v1/tickets?" + params.Encode()

	var result struct {
		Tickets []tools.Ticket `json:"tickets"`
	}
	err := c.request(ctx, http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Tickets, nil
}

// GetTicket gets a ticket by ID or identifier (e.g., "AM-123").
func (c *BackendClient) GetTicket(ctx context.Context, ticketID string) (*tools.Ticket, error) {
	var result tools.Ticket
	err := c.request(ctx, http.MethodGet, fmt.Sprintf("/api/v1/tickets/%s", url.PathEscape(ticketID)), nil, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// CreateTicket creates a new ticket.
func (c *BackendClient) CreateTicket(ctx context.Context, productID int, title, description string, ticketType tools.TicketType, priority tools.TicketPriority, parentTicketID *int) (*tools.Ticket, error) {
	body := map[string]interface{}{
		"product_id":  productID,
		"title":       title,
		"description": description,
		"type":        ticketType,
		"priority":    priority,
	}
	if parentTicketID != nil {
		body["parent_ticket_id"] = *parentTicketID
	}

	var result tools.Ticket
	err := c.request(ctx, http.MethodPost, "/api/v1/tickets", body, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// UpdateTicket updates a ticket.
func (c *BackendClient) UpdateTicket(ctx context.Context, ticketID string, title, description *string, status *tools.TicketStatus, priority *tools.TicketPriority, ticketType *tools.TicketType) (*tools.Ticket, error) {
	body := make(map[string]interface{})
	if title != nil {
		body["title"] = *title
	}
	if description != nil {
		body["description"] = *description
	}
	if status != nil {
		body["status"] = *status
	}
	if priority != nil {
		body["priority"] = *priority
	}
	if ticketType != nil {
		body["type"] = *ticketType
	}

	var result tools.Ticket
	err := c.request(ctx, http.MethodPut, fmt.Sprintf("/api/v1/tickets/%s", url.PathEscape(ticketID)), body, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Session Operations

// CreateSession creates a new DevPod session.
func (c *BackendClient) CreateSession(ctx context.Context, req *tools.SessionCreateRequest) (*tools.SessionCreateResponse, error) {
	var result tools.SessionCreateResponse
	err := c.request(ctx, http.MethodPost, "/api/v1/sessions", req, &result)
	if err != nil {
		return nil, err
	}
	return &result, nil
}

// Verify BackendClient implements CollaborationClient interface.
var _ tools.CollaborationClient = (*BackendClient)(nil)
