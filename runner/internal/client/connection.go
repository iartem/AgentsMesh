package client

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// 协议版本
const CurrentProtocolVersion = 2

// ServerConnection manages the WebSocket connection to the server.
// Responsibilities: Connection lifecycle, read/write loops, initialization.
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

	// Initialization timeout
	initTimeout time.Duration

	// Runner info
	runnerVersion string
	mcpPort       int

	// Initialization state
	initialized     bool
	availableAgents []string
	initCh          chan InitializeResult // Channel to receive initialize_result

	// Channels
	sendCh   chan ProtocolMessage
	stopCh   chan struct{}
	stopOnce sync.Once
}

// NewServerConnection creates a new server connection.
// serverURL should be the WebSocket base URL (e.g., ws://localhost:8080)
// The actual connection URL will be: {serverURL}/api/v1/orgs/{orgSlug}/ws/runners?node_id=xxx
// DefaultInitTimeout is the default timeout for initialization handshake
const DefaultInitTimeout = 30 * time.Second

func NewServerConnection(serverURL, nodeID, authToken, orgSlug string) *ServerConnection {
	conn := &ServerConnection{
		serverURL:         serverURL,
		nodeID:            nodeID,
		authToken:         authToken,
		orgSlug:           orgSlug,
		dialer:            NewGorillaDialer(), // Default dialer
		heartbeatInterval: 30 * time.Second,
		initTimeout:       DefaultInitTimeout,
		reconnectStrategy: NewReconnectStrategy(5*time.Second, 5*time.Minute),
		sendCh:            make(chan ProtocolMessage, 100),
		stopCh:            make(chan struct{}),
		initCh:            make(chan InitializeResult, 1),
		runnerVersion:     "1.0.0", // Default version
		mcpPort:           19000,   // Default MCP port
	}
	return conn
}

// SetInitTimeout sets the initialization handshake timeout.
func (c *ServerConnection) SetInitTimeout(timeout time.Duration) {
	c.initTimeout = timeout
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

// SetRunnerVersion sets the runner version for initialization.
func (c *ServerConnection) SetRunnerVersion(version string) {
	c.runnerVersion = version
}

// SetMCPPort sets the MCP port for initialization.
func (c *ServerConnection) SetMCPPort(port int) {
	c.mcpPort = port
}

// SetAuthToken sets the authentication token.
func (c *ServerConnection) SetAuthToken(token string) {
	c.mu.Lock()
	c.authToken = token
	c.mu.Unlock()
}

// SetOrgSlug sets the organization slug.
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

// IsInitialized returns whether the connection has completed initialization.
func (c *ServerConnection) IsInitialized() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.initialized
}

// GetAvailableAgents returns the list of available agents on this runner.
func (c *ServerConnection) GetAvailableAgents() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.availableAgents
}

// Connect establishes a connection to the server.
func (c *ServerConnection) Connect() error {
	// Build org-scoped URL with only node_id in query (non-sensitive)
	// Authentication token is passed via Authorization header
	connectURL := fmt.Sprintf("%s/api/v1/orgs/%s/ws/runners?node_id=%s",
		c.serverURL, c.orgSlug, c.nodeID)

	// Create headers with Authorization token (more secure than query string)
	headers := map[string][]string{
		"Authorization": {fmt.Sprintf("Bearer %s", c.authToken)},
	}

	conn, _, err := c.dialer.Dial(connectURL, headers)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}

	c.mu.Lock()
	c.conn = conn
	c.initialized = false
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

// HandleInitializeResult handles the initialize_result message from server.
// Called by MessageRouter when receiving initialize_result.
func (c *ServerConnection) HandleInitializeResult(result InitializeResult) {
	select {
	case c.initCh <- result:
	default:
		log.Printf("[connection] Initialize result channel full, dropping")
	}
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

		// Run read/write loops and initialization
		c.runConnection()

		// Check if we should stop before attempting reconnect
		select {
		case <-c.stopCh:
			log.Println("[connection] Connection loop stopped after connection closed")
			return
		default:
		}

		// Connection closed, wait before reconnecting
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

	// Start read loop in goroutine so we can do initialization
	readDone := make(chan struct{})
	go func() {
		c.readLoop()
		close(readDone)
	}()

	// Perform initialization (three-phase handshake)
	if err := c.performInitialization(); err != nil {
		log.Printf("[connection] Initialization failed: %v", err)
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.mu.Unlock()
		close(done)
		<-readDone
		return
	}

	// Start heartbeat after successful initialization
	go c.heartbeatLoop(done)

	// Wait for read loop to finish
	<-readDone

	// Signal other goroutines to stop
	close(done)
}

// performInitialization performs the three-phase initialization handshake.
func (c *ServerConnection) performInitialization() error {
	log.Println("[connection] Starting initialization handshake...")

	// Phase 1: Send initialize request
	hostname, _ := os.Hostname()
	initParams := InitializeParams{
		ProtocolVersion: CurrentProtocolVersion,
		RunnerInfo: RunnerInfo{
			Version:  c.runnerVersion,
			NodeID:   c.nodeID,
			MCPPort:  c.mcpPort,
			OS:       runtime.GOOS,
			Arch:     runtime.GOARCH,
			Hostname: hostname,
		},
	}

	if err := c.SendEvent(MsgTypeInitialize, initParams); err != nil {
		return fmt.Errorf("failed to send initialize: %w", err)
	}
	log.Printf("[connection] Sent initialize request: version=%s, mcp_port=%d", c.runnerVersion, c.mcpPort)

	// Phase 2: Wait for initialize_result
	select {
	case result := <-c.initCh:
		log.Printf("[connection] Received initialize_result: server_version=%s, agent_types=%d, features=%v",
			result.ServerInfo.Version, len(result.AgentTypes), result.Features)

		// Phase 3: Check available agents and send initialized
		availableAgents := c.checkAvailableAgents(result.AgentTypes)
		c.mu.Lock()
		c.availableAgents = availableAgents
		c.mu.Unlock()

		initializedParams := InitializedParams{
			AvailableAgents: availableAgents,
		}

		if err := c.SendEvent(MsgTypeInitialized, initializedParams); err != nil {
			return fmt.Errorf("failed to send initialized: %w", err)
		}
		log.Printf("[connection] Sent initialized: available_agents=%v", availableAgents)

		c.mu.Lock()
		c.initialized = true
		c.mu.Unlock()

		log.Println("[connection] Initialization completed successfully")
		return nil

	case <-time.After(c.initTimeout):
		return fmt.Errorf("timeout waiting for initialize_result after %v", c.initTimeout)

	case <-c.stopCh:
		return fmt.Errorf("connection stopped during initialization")
	}
}

// checkAvailableAgents checks which agents are available on this runner.
func (c *ServerConnection) checkAvailableAgents(agentTypes []AgentTypeInfo) []string {
	var available []string

	for _, agent := range agentTypes {
		if agent.Executable == "" {
			log.Printf("[connection] Agent %s has no executable defined, skipping", agent.Slug)
			continue
		}

		// Check if executable exists in PATH
		path, err := exec.LookPath(agent.Executable)
		if err != nil {
			log.Printf("[connection] Agent %s: executable '%s' not found in PATH",
				agent.Slug, agent.Executable)
			continue
		}

		log.Printf("[connection] Agent %s: found executable at %s", agent.Slug, path)
		available = append(available, agent.Slug)
	}

	return available
}

// readLoop reads messages from the server.
func (c *ServerConnection) readLoop() {
	// Get connection reference under lock for this read loop
	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()

	if conn == nil {
		return
	}

	defer func() {
		c.mu.Lock()
		if c.conn != nil {
			c.conn.Close()
		}
		c.mu.Unlock()
	}()

	readTimeout := 90 * time.Second

	conn.SetPingHandler(func(appData string) error {
		log.Printf("[connection] Received ping from server, resetting read deadline")
		conn.SetReadDeadline(time.Now().Add(readTimeout))
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(10*time.Second))
	})

	for {
		conn.SetReadDeadline(time.Now().Add(readTimeout))

		_, data, err := conn.ReadMessage()
		if err != nil {
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

		log.Printf("[connection] Received message: type=%s, pod_key=%s", msg.Type, msg.PodKey)

		// Handle initialize_result specially (before full initialization)
		if msg.Type == MsgTypeInitializeResult {
			var result InitializeResult
			if err := json.Unmarshal(msg.Data, &result); err != nil {
				log.Printf("[connection] Failed to parse initialize_result: %v", err)
				continue
			}
			c.HandleInitializeResult(result)
			continue
		}

		// Delegate other messages to router (only after initialization)
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

	log.Printf("[connection] Sending heartbeat with %d pods", len(pods))

	if err := c.SendEvent(MsgTypeHeartbeat, data); err != nil {
		log.Printf("[connection] Failed to send heartbeat: %v", err)
	}
}
