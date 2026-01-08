package client

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// WebSocketConn defines the interface for a WebSocket connection.
// This abstraction allows testing without real network connections.
type WebSocketConn interface {
	// ReadMessage reads a message from the connection.
	ReadMessage() (messageType int, p []byte, err error)
	// WriteMessage writes a message to the connection.
	WriteMessage(messageType int, data []byte) error
	// Close closes the connection.
	Close() error
	// SetReadDeadline sets the read deadline on the connection.
	SetReadDeadline(t time.Time) error
}

// WebSocketDialer defines the interface for dialing WebSocket connections.
// This abstraction allows testing without real network connections.
type WebSocketDialer interface {
	// Dial creates a new WebSocket connection to the given URL.
	Dial(urlStr string, requestHeader http.Header) (WebSocketConn, *http.Response, error)
}

// GorillaWebSocketDialer is the default implementation using gorilla/websocket.
type GorillaWebSocketDialer struct {
	*websocket.Dialer
}

// NewGorillaDialer creates a new GorillaWebSocketDialer with default settings.
func NewGorillaDialer() *GorillaWebSocketDialer {
	return &GorillaWebSocketDialer{
		Dialer: websocket.DefaultDialer,
	}
}

// Dial implements WebSocketDialer using gorilla/websocket.
func (d *GorillaWebSocketDialer) Dial(urlStr string, requestHeader http.Header) (WebSocketConn, *http.Response, error) {
	conn, resp, err := d.Dialer.Dial(urlStr, requestHeader)
	if err != nil {
		return nil, resp, err
	}
	return conn, resp, nil
}

// EventSender is an interface for sending events back to the server.
type EventSender interface {
	SendEvent(msgType MessageType, data interface{}) error
}
