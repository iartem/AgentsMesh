package ws

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/anthropics/agentmesh/backend/internal/infra/websocket"
	"github.com/anthropics/agentmesh/backend/internal/middleware"
	"github.com/anthropics/agentmesh/backend/internal/service/runner"
	"github.com/anthropics/agentmesh/backend/internal/service/session"
	"github.com/gin-gonic/gin"
	gorillaws "github.com/gorilla/websocket"
)

var upgrader = gorillaws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// TODO: Implement proper origin checking
		return true
	},
}

// TerminalHandler handles terminal WebSocket connections
type TerminalHandler struct {
	hub            *websocket.Hub
	sessionService *session.Service
	terminalRouter *runner.TerminalRouter
}

// NewTerminalHandler creates a new terminal handler
func NewTerminalHandler(hub *websocket.Hub, sessionService *session.Service) *TerminalHandler {
	return &TerminalHandler{
		hub:            hub,
		sessionService: sessionService,
	}
}

// SetTerminalRouter sets the terminal router for routing terminal data
func (h *TerminalHandler) SetTerminalRouter(tr *runner.TerminalRouter) {
	h.terminalRouter = tr
}

// HandleTerminal handles WebSocket connection for terminal
func (h *TerminalHandler) HandleTerminal(c *gin.Context) {
	sessionKey := c.Param("session_key")

	log.Printf("[terminal_ws] HandleTerminal called for session: %s", sessionKey)

	// Get user from context
	claims, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	_ = claims.(*middleware.Claims)

	// Get tenant context
	tenant, exists := c.Get("tenant")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing organization context"})
		return
	}
	tenantCtx := tenant.(*middleware.TenantContext)

	// Verify session exists and belongs to the organization
	sess, err := h.sessionService.GetSession(c.Request.Context(), sessionKey)
	if err != nil {
		log.Printf("[terminal_ws] Session not found: %s, error: %v", sessionKey, err)
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	if sess.OrganizationID != tenantCtx.OrganizationID {
		c.JSON(http.StatusForbidden, gin.H{"error": "access denied"})
		return
	}

	// Check if terminal router is available
	if h.terminalRouter == nil {
		log.Printf("[terminal_ws] Terminal router not available")
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "terminal router not available"})
		return
	}

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[terminal_ws] Failed to upgrade connection: %v", err)
		return
	}

	log.Printf("[terminal_ws] WebSocket connection upgraded for session: %s", sessionKey)

	// Connect client to terminal router
	client, err := h.terminalRouter.ConnectClient(sessionKey, conn)
	if err != nil {
		log.Printf("[terminal_ws] Failed to connect client: %v", err)
		conn.Close()
		return
	}

	log.Printf("[terminal_ws] Client connected to terminal router for session: %s", sessionKey)

	// Start write pump to send data to client
	go h.writePump(client)

	// Start read loop to receive data from client
	h.readLoop(sessionKey, client, conn)

	// Disconnect client when done
	h.terminalRouter.DisconnectClient(client)
	log.Printf("[terminal_ws] Client disconnected from session: %s", sessionKey)
}

// writePump sends data from the terminal router to the WebSocket client
func (h *TerminalHandler) writePump(client *runner.TerminalClient) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case data, ok := <-client.Send:
			if !ok {
				// Channel closed
				client.Conn.WriteMessage(gorillaws.CloseMessage, []byte{})
				return
			}

			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(gorillaws.BinaryMessage, data); err != nil {
				log.Printf("[terminal_ws] Failed to write to client: %v", err)
				return
			}

		case <-ticker.C:
			client.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := client.Conn.WriteMessage(gorillaws.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// readLoop reads data from the WebSocket client and routes it to the runner
func (h *TerminalHandler) readLoop(sessionKey string, client *runner.TerminalClient, conn *gorillaws.Conn) {
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		msgType, data, err := conn.ReadMessage()
		if err != nil {
			if gorillaws.IsUnexpectedCloseError(err, gorillaws.CloseGoingAway, gorillaws.CloseNormalClosure) {
				log.Printf("[terminal_ws] Read error for session %s: %v", sessionKey, err)
			}
			return
		}

		// Reset read deadline
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		if msgType == gorillaws.TextMessage {
			// Parse JSON message
			var msg struct {
				Type string `json:"type"`
				Data string `json:"data"`
				Rows int    `json:"rows"`
				Cols int    `json:"cols"`
			}
			if err := json.Unmarshal(data, &msg); err != nil {
				log.Printf("[terminal_ws] Failed to parse message: %v", err)
				continue
			}

			switch msg.Type {
			case "input":
				// Route input to runner
				if err := h.terminalRouter.RouteInput(sessionKey, []byte(msg.Data)); err != nil {
					log.Printf("[terminal_ws] Failed to route input: %v", err)
				}
			case "resize":
				// Route resize to runner
				if err := h.terminalRouter.RouteResize(sessionKey, msg.Cols, msg.Rows); err != nil {
					log.Printf("[terminal_ws] Failed to route resize: %v", err)
				}
			}
		} else if msgType == gorillaws.BinaryMessage {
			// Binary data is terminal input
			if err := h.terminalRouter.RouteInput(sessionKey, data); err != nil {
				log.Printf("[terminal_ws] Failed to route binary input: %v", err)
			}
		}
	}
}

// EventsHandler handles WebSocket connection for real-time events
type EventsHandler struct {
	hub *websocket.Hub
}

// NewEventsHandler creates a new events handler
func NewEventsHandler(hub *websocket.Hub) *EventsHandler {
	return &EventsHandler{hub: hub}
}

// HandleEvents handles WebSocket connection for events
func (h *EventsHandler) HandleEvents(c *gin.Context) {
	// Get user from context
	claims, exists := c.Get("claims")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userClaims := claims.(*middleware.Claims)

	// Get tenant context
	tenant, exists := c.Get("tenant")
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing organization context"})
		return
	}
	tenantCtx := tenant.(*middleware.TenantContext)

	// Upgrade to WebSocket
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	// Create client
	client := websocket.NewClient(h.hub, conn, userClaims.UserID, tenantCtx.OrganizationID)

	// Register client
	h.hub.Register(client)

	// Start read/write pumps
	go client.WritePump()
	go client.ReadPump(h.onMessage)
}

func (h *EventsHandler) onMessage(client *websocket.Client, msg *websocket.Message) {
	switch msg.Type {
	case websocket.MessageTypePing:
		client.Send(&websocket.Message{
			Type:      websocket.MessageTypePong,
			Timestamp: time.Now().UnixMilli(),
		})

	default:
		// Handle subscription messages
		var subData struct {
			Action    string `json:"action"`
			SessionID string `json:"session_id,omitempty"`
			ChannelID int64  `json:"channel_id,omitempty"`
		}
		if err := json.Unmarshal(msg.Data, &subData); err == nil {
			switch subData.Action {
			case "subscribe_session":
				client.SetSession(subData.SessionID)
			case "subscribe_channel":
				client.SetChannel(subData.ChannelID)
			case "unsubscribe_session":
				client.SetSession("")
			case "unsubscribe_channel":
				client.SetChannel(0)
			}
		}
	}
}
