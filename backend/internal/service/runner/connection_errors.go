package runner

import "errors"

// Connection-related errors
var (
	ErrRunnerNotConnected    = errors.New("runner not connected")
	ErrRunnerNotInitialized  = errors.New("runner not initialized")
	ErrConnectionClosed      = errors.New("connection closed")
)
