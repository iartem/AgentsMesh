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
	case MsgTypeCreatePod:
		r.handleCreatePod(msg)

	case MsgTypeTerminatePod:
		r.handleTerminatePod(msg)

	case MsgTypeListPods:
		r.handleListPods()

	case MsgTypeTerminalInput:
		r.handleTerminalInput(msg)

	case MsgTypeTerminalResize:
		r.handleTerminalResize(msg)

	default:
		log.Printf("[router] Unknown message type: %s", msg.Type)
	}
}

func (r *MessageRouter) handleCreatePod(msg ProtocolMessage) {
	var req CreatePodRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("[router] Failed to parse create pod request: %v", err)
		return
	}

	log.Printf("[router] Received create pod request: pod_key=%s, command=%s",
		req.PodKey, req.LaunchCommand)

	if err := r.handler.OnCreatePod(req); err != nil {
		log.Printf("[router] Failed to create pod: %v", err)
	}
}

func (r *MessageRouter) handleTerminatePod(msg ProtocolMessage) {
	var req TerminatePodRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("[router] Failed to parse terminate pod request: %v", err)
		return
	}

	log.Printf("[router] Received terminate pod request: pod_key=%s", req.PodKey)
	if err := r.handler.OnTerminatePod(req); err != nil {
		log.Printf("[router] Failed to terminate pod: %v", err)
	}
}

func (r *MessageRouter) handleListPods() {
	pods := r.handler.OnListPods()
	if err := r.eventSender.SendEvent(MsgTypePodList, pods); err != nil {
		log.Printf("[router] Failed to send pod list: %v", err)
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
