package client

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ServerConnection manages the WebSocket connection to the server.
// Responsibilities: Connection lifecycle, read/write loops.
// Message routing is delegated to MessageRouter.
// Reconnection timing is delegated to ReconnectStrategy.
type ServerConnection struct {
	serverURL string
	nodeID    string
	authToken string
	orgSlug   string // Organization slug for org-scoped API paths

	conn    WebSocketConn
	dialer  WebSocketDialer
	mu      sync.Mutex
	handler MessageHandler

	// Message routing (SRP: delegated)
	router *MessageRouter

	// Reconnection (SRP: delegated)
	reconnectStrategy *ReconnectStrategy

	// Heartbeat
	heartbeatInterval time.Duration

	// Channels
	sendCh   chan ProtocolMessage
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewServerConnection creates a new server connection.
// serverURL should be the WebSocket base URL (e.g., ws://localhost:8080)
// The actual connection URL will be: {serverURL}/api/v1/orgs/{orgSlug}/ws/runners?node_id=xxx&token=xxx
func NewServerConnection(serverURL, nodeID, authToken, orgSlug string) *ServerConnection {
	conn := &ServerConnection{
		serverURL:         serverURL,
		nodeID:            nodeID,
		authToken:         authToken,
		orgSlug:           orgSlug,
		dialer:            NewGorillaDialer(), // Default dialer
		heartbeatInterval: 30 * time.Second,
		reconnectStrategy: NewReconnectStrategy(5*time.Second, 5*time.Minute),
		sendCh:            make(chan ProtocolMessage, 100),
		stopCh:            make(chan struct{}),
	}
	return conn
}

// WithDialer sets a custom WebSocket dialer (for testing).
func (c *ServerConnection) WithDialer(dialer WebSocketDialer) *ServerConnection {
	c.dialer = dialer
	return c
}

// SetHandler sets the message handler.
func (c *ServerConnection) SetHandler(handler MessageHandler) {
	c.handler = handler
	// Create router with this connection as EventSender
	c.router = NewMessageRouter(handler, c)
}

// SetHeartbeatInterval sets the heartbeat interval.
func (c *ServerConnection) SetHeartbeatInterval(interval time.Duration) {
	c.heartbeatInterval = interval
}

// SetAuthToken sets the authentication token.
// This should be called after registration to update the token before connecting.
func (c *ServerConnection) SetAuthToken(token string) {
	c.mu.Lock()
	c.authToken = token
	c.mu.Unlock()
}

// SetOrgSlug sets the organization slug.
// This should be called after registration to update the org slug before connecting.
func (c *ServerConnection) SetOrgSlug(orgSlug string) {
	c.mu.Lock()
	c.orgSlug = orgSlug
	c.mu.Unlock()
}

// GetOrgSlug returns the organization slug.
func (c *ServerConnection) GetOrgSlug() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.orgSlug
}

// Connect establishes a connection to the server.
func (c *ServerConnection) Connect() error {
	// Build org-scoped URL with query parameters for authentication
	// New format: {serverURL}/api/v1/orgs/{orgSlug}/ws/runners?node_id=xxx&token=xxx
	connectURL := fmt.Sprintf("%s/api/v1/orgs/%s/ws/runners?node_id=%s&token=%s",
		c.serverURL, c.orgSlug, c.nodeID, c.authToken)

	conn, _, err := c.dialer.Dial(connectURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()

	log.Printf("[connection] Connected to server: %s (org: %s)", c.serverURL, c.orgSlug)
	return nil
}

// Start starts the connection management (connect, heartbeat, reconnect).
func (c *ServerConnection) Start() {
	go c.connectionLoop()
}

// Stop stops the connection.
func (c *ServerConnection) Stop() {
	c.stopOnce.Do(func() {
		close(c.stopCh)
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.mu.Unlock()
	})
}

// Send sends a message to the server (non-blocking).
// Used for non-critical messages like heartbeat.
// For terminal output, use SendWithBackpressure instead.
func (c *ServerConnection) Send(msg ProtocolMessage) {
	msg.Timestamp = time.Now().UnixMilli()
	select {
	case c.sendCh <- msg:
	default:
		log.Printf("[connection] Send buffer full (queue_len=%d, queue_cap=%d), dropping message type=%s",
			len(c.sendCh), cap(c.sendCh), msg.Type)
	}
}

// SendWithBackpressure sends a message with backpressure (blocking).
// Used for terminal output to ensure no data loss.
// Returns false if the connection is stopped.
func (c *ServerConnection) SendWithBackpressure(msg ProtocolMessage) bool {
	msg.Timestamp = time.Now().UnixMilli()
	select {
	case c.sendCh <- msg:
		return true
	case <-c.stopCh:
		return false
	}
}

// QueueLength returns the current send queue length.
func (c *ServerConnection) QueueLength() int {
	return len(c.sendCh)
}

// QueueCapacity returns the send queue capacity.
func (c *ServerConnection) QueueCapacity() int {
	return cap(c.sendCh)
}

// SendEvent sends an event message to the server.
// Implements EventSender interface.
func (c *ServerConnection) SendEvent(msgType MessageType, data interface{}) error {
	dataBytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	c.Send(ProtocolMessage{
		Type: msgType,
		Data: dataBytes,
	})
	return nil
}

// connectionLoop manages the connection lifecycle.
func (c *ServerConnection) connectionLoop() {
	for {
		select {
		case <-c.stopCh:
			log.Println("[connection] Connection loop stopped")
			return
		default:
		}

		// Try to connect
		if err := c.Connect(); err != nil {
			delay := c.reconnectStrategy.NextDelay()
			log.Printf("[connection] Failed to connect (attempt=%d, error=%v), will retry in %v",
				c.reconnectStrategy.AttemptCount(), err, delay)

			// Wait with stop signal check
			select {
			case <-c.stopCh:
				log.Println("[connection] Connection loop stopped during reconnect wait")
				return
			case <-time.After(delay):
			}
			continue
		}

		// Reset reconnect strategy on successful connection
		c.reconnectStrategy.Reset()

		// Run read/write loops
		c.runConnection()

		// Check if we should stop before attempting reconnect
		select {
		case <-c.stopCh:
			log.Println("[connection] Connection loop stopped after connection closed")
			return
		default:
		}

		// Connection closed, log and wait before reconnecting
		log.Println("[connection] Connection closed, will attempt to reconnect")
		select {
		case <-c.stopCh:
			log.Println("[connection] Connection loop stopped during post-close wait")
			return
		case <-time.After(c.reconnectStrategy.CurrentInterval()):
		}
	}
}

// runConnection runs the read and write loops for an established connection.
func (c *ServerConnection) runConnection() {
	done := make(chan struct{})

	// Start write loop
	go c.writeLoop(done)

	// Start heartbeat
	go c.heartbeatLoop(done)

	// Run read loop (blocking)
	c.readLoop()

	// Signal other goroutines to stop
	close(done)
}

// readLoop reads messages from the server.
func (c *ServerConnection) readLoop() {
	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.mu.Unlock()
	}()

	// Set read deadline to detect stale connections
	// The heartbeat from server or pong will reset this
	readTimeout := 90 * time.Second // Should be > heartbeat interval

	for {
		c.mu.Lock()
		if c.conn != nil {
			c.conn.SetReadDeadline(time.Now().Add(readTimeout))
		}
		c.mu.Unlock()

		_, data, err := c.conn.ReadMessage()
		if err != nil {
			// Log all read errors for debugging
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				log.Printf("[connection] WebSocket read error: %v", err)
			} else {
				log.Printf("[connection] WebSocket connection closed: %v", err)
			}
			return
		}

		var msg ProtocolMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			log.Printf("[connection] Failed to parse message: %v", err)
			continue
		}

		// Delegate to message router (SRP)
		if c.router != nil {
			c.router.Route(msg)
		}
	}
}

// writeLoop writes messages to the server.
func (c *ServerConnection) writeLoop(done <-chan struct{}) {
	for {
		select {
		case <-c.stopCh:
			return
		case <-done:
			return
		case msg := <-c.sendCh:
			c.mu.Lock()
			if c.conn == nil {
				c.mu.Unlock()
				continue
			}

			data, err := json.Marshal(msg)
			if err != nil {
				c.mu.Unlock()
				log.Printf("[connection] Failed to marshal message: %v", err)
				continue
			}

			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				// Close the connection to trigger readLoop exit and reconnection
				c.conn.Close()
				c.mu.Unlock()
				log.Printf("[connection] Failed to send message, closing connection for reconnect: %v", err)
				return
			}
			c.mu.Unlock()
		}
	}
}

// heartbeatLoop sends periodic heartbeats.
func (c *ServerConnection) heartbeatLoop(done <-chan struct{}) {
	ticker := time.NewTicker(c.heartbeatInterval)
	defer ticker.Stop()

	// Send initial heartbeat
	c.sendHeartbeat()

	for {
		select {
		case <-c.stopCh:
			return
		case <-done:
			return
		case <-ticker.C:
			c.sendHeartbeat()
		}
	}
}

// sendHeartbeat sends a heartbeat message.
func (c *ServerConnection) sendHeartbeat() {
	var pods []PodInfo
	if c.handler != nil {
		pods = c.handler.OnListPods()
	}

	data := HeartbeatData{
		NodeID: c.nodeID,
		Pods:   pods,
	}

	if err := c.SendEvent(MsgTypeHeartbeat, data); err != nil {
		log.Printf("[connection] Failed to send heartbeat: %v", err)
	}
}
