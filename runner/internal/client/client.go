package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// MessageType constants matching backend's RunnerMessage types
const (
	MessageTypeHeartbeat         = "heartbeat"
	MessageTypeSessionCreated    = "session_created"
	MessageTypeSessionTerminated = "session_terminated"
	MessageTypeSessionStatus     = "agent_status" // Agent status update
	MessageTypeTerminalOutput    = "terminal_output"
	MessageTypePtyResized        = "pty_resized"
	MessageTypeError             = "error"

	// From server (to runner)
	MessageTypeCreateSession    = "create_session"
	MessageTypeTerminateSession = "terminate_session"
	MessageTypeTerminalInput    = "terminal_input"
	MessageTypeTerminalResize   = "terminal_resize"
	MessageTypeSendPrompt       = "send_prompt"

	// Legacy aliases for backward compatibility
	MessageTypeSessionStart = "create_session"
	MessageTypeSessionStop  = "terminate_session"
	MessageTypeSessionList  = "session_list"
)

// Message represents a WebSocket message
// Note: Field names and types must match backend's RunnerMessage struct
type Message struct {
	Type      string          `json:"type"`
	SessionID string          `json:"session_id,omitempty"`
	Payload   json.RawMessage `json:"payload,omitempty"` // Legacy field for backward compatibility
	Data      json.RawMessage `json:"data,omitempty"`    // New field matching backend
	Timestamp int64           `json:"timestamp"`
}

// Client is the WebSocket client for connecting to the server
type Client struct {
	serverURL string
	nodeID    string
	authToken string

	conn     *websocket.Conn
	messages chan *Message
	mu       sync.Mutex

	reconnecting     bool
	stopChan         chan struct{}
	messagesCloseOnce sync.Once
}

// New creates a new client
func New(serverURL, nodeID, authToken string) *Client {
	return &Client{
		serverURL: serverURL,
		nodeID:    nodeID,
		authToken: authToken,
		messages:  make(chan *Message, 100),
		stopChan:  make(chan struct{}),
	}
}

// SetAuthToken sets the auth token
func (c *Client) SetAuthToken(token string) {
	c.mu.Lock()
	c.authToken = token
	c.mu.Unlock()
}

// Register registers the runner with the server
func (c *Client) Register(ctx context.Context, registrationToken, description string, maxSessions int) (string, error) {
	// Build registration URL
	registerURL := fmt.Sprintf("%s/api/v1/runners/register", c.serverURL)

	// Build request body
	body := map[string]interface{}{
		"node_id":                 c.nodeID,
		"description":             description,
		"registration_token":      registrationToken,
		"max_concurrent_sessions": maxSessions,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal registration body: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", registerURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create registration request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("registration request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var result struct {
		AuthToken string `json:"auth_token"`
		RunnerID  int64  `json:"runner_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode registration response: %w", err)
	}

	c.mu.Lock()
	c.authToken = result.AuthToken
	c.mu.Unlock()

	return result.AuthToken, nil
}

// Connect establishes a WebSocket connection to the server
func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Parse server URL and convert to WebSocket URL
	parsedURL, err := url.Parse(c.serverURL)
	if err != nil {
		return fmt.Errorf("failed to parse server URL: %w", err)
	}

	// Convert HTTP(S) to WS(S)
	scheme := "ws"
	if parsedURL.Scheme == "https" {
		scheme = "wss"
	}

	wsURL := fmt.Sprintf("%s://%s/api/v1/runners/ws", scheme, parsedURL.Host)

	// Build headers
	headers := http.Header{}
	headers.Set("Authorization", fmt.Sprintf("Bearer %s", c.authToken))
	headers.Set("X-Runner-ID", c.nodeID)

	// Connect with context
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.DialContext(ctx, wsURL, headers)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	c.conn = conn

	// Start reader
	go c.readLoop()

	// Start ping/pong handler
	go c.pingLoop(ctx)

	return nil
}

// Close closes the connection
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	close(c.stopChan)

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

// Messages returns the channel for receiving messages
func (c *Client) Messages() <-chan *Message {
	return c.messages
}

// Send sends a message to the server
func (c *Client) Send(msg *Message) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("not connected")
	}

	msg.Timestamp = time.Now().UnixMilli()

	return c.conn.WriteJSON(msg)
}

// SendHeartbeat sends a heartbeat message
func (c *Client) SendHeartbeat(currentSessions int) error {
	data, _ := json.Marshal(map[string]interface{}{
		"current_sessions": currentSessions,
	})

	return c.Send(&Message{
		Type: MessageTypeHeartbeat,
		Data: data,
	})
}

// SendTerminalOutput sends terminal output to the server
func (c *Client) SendTerminalOutput(sessionKey string, data []byte) error {
	msgData, _ := json.Marshal(map[string]interface{}{
		"session_id": sessionKey, // Use session_id to match backend
		"data":       data,
	})

	return c.Send(&Message{
		Type:      MessageTypeTerminalOutput,
		SessionID: sessionKey,
		Data:      msgData,
	})
}

// SendSessionStatus sends session status to the server
func (c *Client) SendSessionStatus(sessionKey, status string, details map[string]interface{}) error {
	dataMap := map[string]interface{}{
		"session_id": sessionKey, // Use session_id to match backend
		"status":     status,
	}

	for k, v := range details {
		dataMap[k] = v
	}

	data, _ := json.Marshal(dataMap)

	return c.Send(&Message{
		Type:      MessageTypeSessionStatus,
		SessionID: sessionKey,
		Data:      data,
	})
}

// readLoop reads messages from the WebSocket
func (c *Client) readLoop() {
	defer c.messagesCloseOnce.Do(func() {
		close(c.messages)
	})

	for {
		select {
		case <-c.stopChan:
			return
		default:
		}

		var msg Message
		if err := c.conn.ReadJSON(&msg); err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				fmt.Printf("WebSocket error: %v\n", err)
			}

			// Attempt reconnection
			c.reconnect()
			return
		}

		c.messages <- &msg
	}
}

// pingLoop sends periodic pings to keep the connection alive
func (c *Client) pingLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.mu.Lock()
			if c.conn != nil {
				c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					fmt.Printf("Ping error: %v\n", err)
				}
			}
			c.mu.Unlock()
		}
	}
}

// reconnect attempts to reconnect to the server
func (c *Client) reconnect() {
	c.mu.Lock()
	if c.reconnecting {
		c.mu.Unlock()
		return
	}
	c.reconnecting = true
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.reconnecting = false
		c.mu.Unlock()
	}()

	backoff := time.Second
	maxBackoff := 60 * time.Second

	for {
		select {
		case <-c.stopChan:
			return
		default:
		}

		fmt.Printf("Attempting to reconnect in %v...\n", backoff)
		time.Sleep(backoff)

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err := c.Connect(ctx)
		cancel()

		if err == nil {
			fmt.Println("Reconnected successfully")
			return
		}

		fmt.Printf("Reconnection failed: %v\n", err)

		// Exponential backoff
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// IsConnected returns whether the client is connected
func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.conn != nil
}
