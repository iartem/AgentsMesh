package client

import (
	"encoding/json"
	"log"
)

// MessageRouter routes incoming messages to the appropriate handler method.
// Single Responsibility: Message parsing and routing only.
type MessageRouter struct {
	handler     MessageHandler
	eventSender EventSender
}

// NewMessageRouter creates a new MessageRouter.
func NewMessageRouter(handler MessageHandler, eventSender EventSender) *MessageRouter {
	return &MessageRouter{
		handler:     handler,
		eventSender: eventSender,
	}
}

// Route routes an incoming message to the appropriate handler.
func (r *MessageRouter) Route(msg ProtocolMessage) {
	if r.handler == nil {
		log.Println("[router] No handler set for incoming messages")
		return
	}

	switch msg.Type {
	case MsgTypeCreateSession:
		r.handleCreateSession(msg)

	case MsgTypeTerminateSession:
		r.handleTerminateSession(msg)

	case MsgTypeListSessions:
		r.handleListSessions()

	case MsgTypeTerminalInput:
		r.handleTerminalInput(msg)

	case MsgTypeTerminalResize:
		r.handleTerminalResize(msg)

	default:
		log.Printf("[router] Unknown message type: %s", msg.Type)
	}
}

func (r *MessageRouter) handleCreateSession(msg ProtocolMessage) {
	var req CreateSessionRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("[router] Failed to parse create session request: %v", err)
		return
	}

	log.Printf("[router] Received create session request: session_id=%s, initial_command=%s, permission_mode=%s",
		req.SessionID, req.InitialCommand, req.PermissionMode)

	if err := r.handler.OnCreateSession(req); err != nil {
		log.Printf("[router] Failed to create session: %v", err)
	}
}

func (r *MessageRouter) handleTerminateSession(msg ProtocolMessage) {
	var req TerminateSessionRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("[router] Failed to parse terminate session request: %v", err)
		return
	}

	log.Printf("[router] Received terminate session request: session_id=%s", req.SessionID)
	if err := r.handler.OnTerminateSession(req); err != nil {
		log.Printf("[router] Failed to terminate session: %v", err)
	}
}

func (r *MessageRouter) handleListSessions() {
	sessions := r.handler.OnListSessions()
	if err := r.eventSender.SendEvent(MsgTypeSessionList, sessions); err != nil {
		log.Printf("[router] Failed to send session list: %v", err)
	}
}

func (r *MessageRouter) handleTerminalInput(msg ProtocolMessage) {
	var req TerminalInputRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("[router] Failed to parse terminal input request: %v", err)
		return
	}
	if err := r.handler.OnTerminalInput(req); err != nil {
		log.Printf("[router] Failed to handle terminal input: %v", err)
	}
}

func (r *MessageRouter) handleTerminalResize(msg ProtocolMessage) {
	var req TerminalResizeRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("[router] Failed to parse terminal resize request: %v", err)
		return
	}
	if err := r.handler.OnTerminalResize(req); err != nil {
		log.Printf("[router] Failed to handle terminal resize: %v", err)
	}
}
