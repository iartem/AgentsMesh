package client

// Connection defines the interface for server communication.
// This interface abstracts ServerConnection for testing and decoupling.
type Connection interface {
	// SetHandler sets the message handler for incoming messages.
	SetHandler(handler MessageHandler)

	// Connect establishes a connection to the server.
	Connect() error

	// Start starts the connection management loop (heartbeat, reconnect).
	Start()

	// Stop stops the connection and releases resources.
	Stop()

	// Send sends a message to the server (non-blocking, may drop if buffer full).
	Send(msg ProtocolMessage)

	// SendWithBackpressure sends a message with backpressure (blocking).
	// Returns false if the connection is stopped.
	SendWithBackpressure(msg ProtocolMessage) bool

	// SendEvent sends an event message to the server.
	// This is a convenience method that marshals data and creates ProtocolMessage.
	SendEvent(msgType MessageType, data interface{}) error

	// QueueLength returns the current send queue length.
	QueueLength() int

	// QueueCapacity returns the send queue capacity.
	QueueCapacity() int

	// SetAuthToken sets the authentication token.
	// This should be called after registration to update the token before connecting.
	SetAuthToken(token string)

	// SetOrgSlug sets the organization slug.
	// This should be called after registration to update the org slug before connecting.
	SetOrgSlug(orgSlug string)

	// GetOrgSlug returns the organization slug.
	GetOrgSlug() string
}

// Ensure ServerConnection implements Connection interface.
var _ Connection = (*ServerConnection)(nil)
