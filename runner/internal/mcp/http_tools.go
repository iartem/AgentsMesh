package mcp

import (
	"context"
	"fmt"

	"github.com/anthropics/agentmesh/runner/internal/mcp/tools"
)

// Terminal Tools

func (s *HTTPServer) createObserveTerminalTool() *MCPTool {
	return &MCPTool{
		Name:        "observe_terminal",
		Description: "Observe the terminal output of another agent session. Requires terminal:read permission via binding.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_key": map[string]interface{}{
					"type":        "string",
					"description": "The session key of the target session to observe",
				},
				"lines": map[string]interface{}{
					"type":        "integer",
					"description": "Number of lines to retrieve (default: 50)",
				},
				"raw": map[string]interface{}{
					"type":        "boolean",
					"description": "Return raw output without ANSI processing (default: false)",
				},
				"include_screen": map[string]interface{}{
					"type":        "boolean",
					"description": "Include current screen content (default: false)",
				},
			},
			"required": []string{"session_key"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			sessionKey := getStringArg(args, "session_key")
			if sessionKey == "" {
				return nil, fmt.Errorf("session_key is required")
			}

			lines := getIntArg(args, "lines")
			if lines == 0 {
				lines = 50
			}
			raw := getBoolArg(args, "raw")
			includeScreen := getBoolArg(args, "include_screen")

			return client.ObserveTerminal(ctx, sessionKey, lines, raw, includeScreen)
		},
	}
}

func (s *HTTPServer) createSendTerminalTextTool() *MCPTool {
	return &MCPTool{
		Name:        "send_terminal_text",
		Description: "Send text input to another agent's terminal. Requires terminal:write permission via binding.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_key": map[string]interface{}{
					"type":        "string",
					"description": "The session key of the target session",
				},
				"text": map[string]interface{}{
					"type":        "string",
					"description": "The text to send to the terminal",
				},
			},
			"required": []string{"session_key", "text"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			sessionKey := getStringArg(args, "session_key")
			text := getStringArg(args, "text")

			if sessionKey == "" || text == "" {
				return nil, fmt.Errorf("session_key and text are required")
			}

			err := client.SendTerminalText(ctx, sessionKey, text)
			if err != nil {
				return nil, err
			}
			return "Text sent successfully", nil
		},
	}
}

func (s *HTTPServer) createSendTerminalKeyTool() *MCPTool {
	return &MCPTool{
		Name:        "send_terminal_key",
		Description: "Send special keys to another agent's terminal. Supports: enter, escape, tab, backspace, delete, ctrl+c, ctrl+d, ctrl+u, ctrl+l, ctrl+z, ctrl+a, ctrl+e, ctrl+k, ctrl+w, up, down, left, right, home, end, pageup, pagedown, shift+tab",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"session_key": map[string]interface{}{
					"type":        "string",
					"description": "The session key of the target session",
				},
				"keys": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Array of keys to send (e.g., ['ctrl+c', 'enter'])",
				},
			},
			"required": []string{"session_key", "keys"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			sessionKey := getStringArg(args, "session_key")
			keys := getStringSliceArg(args, "keys")

			if sessionKey == "" || len(keys) == 0 {
				return nil, fmt.Errorf("session_key and keys are required")
			}

			err := client.SendTerminalKey(ctx, sessionKey, keys)
			if err != nil {
				return nil, err
			}
			return "Keys sent successfully", nil
		},
	}
}

// Discovery Tools

func (s *HTTPServer) createListAvailableSessionsTool() *MCPTool {
	return &MCPTool{
		Name:        "list_available_sessions",
		Description: "List other agent sessions available for collaboration. Shows sessions that can be bound to.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			return client.ListAvailableSessions(ctx)
		},
	}
}

// Binding Tools

func (s *HTTPServer) createBindSessionTool() *MCPTool {
	return &MCPTool{
		Name:        "bind_session",
		Description: "Request to bind with another agent session. The target session must accept the binding request.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"target_session": map[string]interface{}{
					"type":        "string",
					"description": "The session key of the target session to bind with",
				},
				"scopes": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string", "enum": []string{"terminal:read", "terminal:write"}},
					"description": "Permission scopes to request (terminal:read, terminal:write)",
				},
			},
			"required": []string{"target_session", "scopes"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			targetSession := getStringArg(args, "target_session")
			scopeStrs := getStringSliceArg(args, "scopes")

			if targetSession == "" || len(scopeStrs) == 0 {
				return nil, fmt.Errorf("target_session and scopes are required")
			}

			scopes := make([]tools.BindingScope, len(scopeStrs))
			for i, s := range scopeStrs {
				scopes[i] = tools.BindingScope(s)
			}

			return client.RequestBinding(ctx, targetSession, scopes)
		},
	}
}

func (s *HTTPServer) createAcceptBindingTool() *MCPTool {
	return &MCPTool{
		Name:        "accept_binding",
		Description: "Accept a pending binding request from another agent session.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"binding_id": map[string]interface{}{
					"type":        "integer",
					"description": "The ID of the binding request to accept",
				},
			},
			"required": []string{"binding_id"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			bindingID := getIntArg(args, "binding_id")
			if bindingID == 0 {
				return nil, fmt.Errorf("binding_id is required")
			}
			return client.AcceptBinding(ctx, bindingID)
		},
	}
}

func (s *HTTPServer) createRejectBindingTool() *MCPTool {
	return &MCPTool{
		Name:        "reject_binding",
		Description: "Reject a pending binding request from another agent session.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"binding_id": map[string]interface{}{
					"type":        "integer",
					"description": "The ID of the binding request to reject",
				},
				"reason": map[string]interface{}{
					"type":        "string",
					"description": "Optional reason for rejection",
				},
			},
			"required": []string{"binding_id"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			bindingID := getIntArg(args, "binding_id")
			reason := getStringArg(args, "reason")

			if bindingID == 0 {
				return nil, fmt.Errorf("binding_id is required")
			}

			return client.RejectBinding(ctx, bindingID, reason)
		},
	}
}

func (s *HTTPServer) createUnbindSessionTool() *MCPTool {
	return &MCPTool{
		Name:        "unbind_session",
		Description: "Unbind from a previously bound session, revoking all permissions.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"target_session": map[string]interface{}{
					"type":        "string",
					"description": "The session key of the session to unbind from",
				},
			},
			"required": []string{"target_session"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			targetSession := getStringArg(args, "target_session")
			if targetSession == "" {
				return nil, fmt.Errorf("target_session is required")
			}

			err := client.UnbindSession(ctx, targetSession)
			if err != nil {
				return nil, err
			}
			return "Session unbound successfully", nil
		},
	}
}

func (s *HTTPServer) createGetBindingsTool() *MCPTool {
	return &MCPTool{
		Name:        "get_bindings",
		Description: "Get all bindings for this session, optionally filtered by status.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"status": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"pending", "active", "rejected", "inactive", "expired"},
					"description": "Filter by binding status (optional)",
				},
			},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			var status *tools.BindingStatus
			if s := getStringArg(args, "status"); s != "" {
				bs := tools.BindingStatus(s)
				status = &bs
			}
			return client.GetBindings(ctx, status)
		},
	}
}

func (s *HTTPServer) createGetBoundSessionsTool() *MCPTool {
	return &MCPTool{
		Name:        "get_bound_sessions",
		Description: "Get list of sessions that are currently bound to this session with active permissions.",
		InputSchema: map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			return client.GetBoundSessions(ctx)
		},
	}
}

// Channel Tools

func (s *HTTPServer) createSearchChannelsTool() *MCPTool {
	return &MCPTool{
		Name:        "search_channels",
		Description: "Search for collaboration channels. Channels are shared spaces for multi-agent communication.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Filter by channel name (partial match)",
				},
				"project_id": map[string]interface{}{
					"type":        "integer",
					"description": "Filter by project ID",
				},
				"ticket_id": map[string]interface{}{
					"type":        "integer",
					"description": "Filter by ticket ID",
				},
				"is_archived": map[string]interface{}{
					"type":        "boolean",
					"description": "Filter by archived status",
				},
				"offset": map[string]interface{}{
					"type":        "integer",
					"description": "Pagination offset (default: 0)",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum results to return (default: 20)",
				},
			},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			name := getStringArg(args, "name")
			projectID := getIntPtrArg(args, "project_id")
			ticketID := getIntPtrArg(args, "ticket_id")

			var isArchived *bool
			if v, ok := args["is_archived"].(bool); ok {
				isArchived = &v
			}

			offset := getIntArg(args, "offset")
			limit := getIntArg(args, "limit")
			if limit == 0 {
				limit = 20
			}

			return client.SearchChannels(ctx, name, projectID, ticketID, isArchived, offset, limit)
		},
	}
}

func (s *HTTPServer) createCreateChannelTool() *MCPTool {
	return &MCPTool{
		Name:        "create_channel",
		Description: "Create a new collaboration channel for multi-agent communication.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Unique name for the channel",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Description of the channel's purpose",
				},
				"project_id": map[string]interface{}{
					"type":        "integer",
					"description": "Associated project ID (optional)",
				},
				"ticket_id": map[string]interface{}{
					"type":        "integer",
					"description": "Associated ticket ID (optional)",
				},
			},
			"required": []string{"name"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			name := getStringArg(args, "name")
			description := getStringArg(args, "description")
			projectID := getIntPtrArg(args, "project_id")
			ticketID := getIntPtrArg(args, "ticket_id")

			if name == "" {
				return nil, fmt.Errorf("name is required")
			}

			return client.CreateChannel(ctx, name, description, projectID, ticketID)
		},
	}
}

func (s *HTTPServer) createGetChannelTool() *MCPTool {
	return &MCPTool{
		Name:        "get_channel",
		Description: "Get details of a specific collaboration channel.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"channel_id": map[string]interface{}{
					"type":        "integer",
					"description": "The ID of the channel to retrieve",
				},
			},
			"required": []string{"channel_id"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			channelID := getIntArg(args, "channel_id")
			if channelID == 0 {
				return nil, fmt.Errorf("channel_id is required")
			}
			return client.GetChannel(ctx, channelID)
		},
	}
}

func (s *HTTPServer) createSendChannelMessageTool() *MCPTool {
	return &MCPTool{
		Name:        "send_channel_message",
		Description: "Send a message to a collaboration channel.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"channel_id": map[string]interface{}{
					"type":        "integer",
					"description": "The ID of the channel to send to",
				},
				"content": map[string]interface{}{
					"type":        "string",
					"description": "The message content",
				},
				"message_type": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"text", "system"},
					"description": "Type of message (default: text)",
				},
				"mentions": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Session keys to mention in the message",
				},
				"reply_to": map[string]interface{}{
					"type":        "integer",
					"description": "Message ID to reply to (optional)",
				},
			},
			"required": []string{"channel_id", "content"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			channelID := getIntArg(args, "channel_id")
			content := getStringArg(args, "content")
			msgType := getStringArg(args, "message_type")
			mentions := getStringSliceArg(args, "mentions")
			replyTo := getIntPtrArg(args, "reply_to")

			if channelID == 0 || content == "" {
				return nil, fmt.Errorf("channel_id and content are required")
			}

			if msgType == "" {
				msgType = "text"
			}

			return client.SendMessage(ctx, channelID, content, tools.ChannelMessageType(msgType), mentions, replyTo)
		},
	}
}

func (s *HTTPServer) createGetChannelMessagesTool() *MCPTool {
	return &MCPTool{
		Name:        "get_channel_messages",
		Description: "Get messages from a collaboration channel.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"channel_id": map[string]interface{}{
					"type":        "integer",
					"description": "The ID of the channel",
				},
				"before_time": map[string]interface{}{
					"type":        "string",
					"description": "Get messages before this timestamp (ISO 8601)",
				},
				"after_time": map[string]interface{}{
					"type":        "string",
					"description": "Get messages after this timestamp (ISO 8601)",
				},
				"mentioned_session": map[string]interface{}{
					"type":        "string",
					"description": "Filter to messages mentioning this session",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum messages to return (default: 50)",
				},
			},
			"required": []string{"channel_id"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			channelID := getIntArg(args, "channel_id")
			if channelID == 0 {
				return nil, fmt.Errorf("channel_id is required")
			}

			var beforeTime, afterTime, mentionedSession *string
			if v := getStringArg(args, "before_time"); v != "" {
				beforeTime = &v
			}
			if v := getStringArg(args, "after_time"); v != "" {
				afterTime = &v
			}
			if v := getStringArg(args, "mentioned_session"); v != "" {
				mentionedSession = &v
			}

			limit := getIntArg(args, "limit")
			if limit == 0 {
				limit = 50
			}

			return client.GetMessages(ctx, channelID, beforeTime, afterTime, mentionedSession, limit)
		},
	}
}

func (s *HTTPServer) createGetChannelDocumentTool() *MCPTool {
	return &MCPTool{
		Name:        "get_channel_document",
		Description: "Get the shared document from a channel. Channels can have a collaborative document that all members can view and edit.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"channel_id": map[string]interface{}{
					"type":        "integer",
					"description": "The ID of the channel",
				},
			},
			"required": []string{"channel_id"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			channelID := getIntArg(args, "channel_id")
			if channelID == 0 {
				return nil, fmt.Errorf("channel_id is required")
			}
			return client.GetDocument(ctx, channelID)
		},
	}
}

func (s *HTTPServer) createUpdateChannelDocumentTool() *MCPTool {
	return &MCPTool{
		Name:        "update_channel_document",
		Description: "Update the shared document in a channel. This replaces the entire document content.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"channel_id": map[string]interface{}{
					"type":        "integer",
					"description": "The ID of the channel",
				},
				"document": map[string]interface{}{
					"type":        "string",
					"description": "The new document content",
				},
			},
			"required": []string{"channel_id", "document"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			channelID := getIntArg(args, "channel_id")
			document := getStringArg(args, "document")

			if channelID == 0 {
				return nil, fmt.Errorf("channel_id is required")
			}

			err := client.UpdateDocument(ctx, channelID, document)
			if err != nil {
				return nil, err
			}
			return "Document updated successfully", nil
		},
	}
}

// Ticket Tools

func (s *HTTPServer) createSearchTicketsTool() *MCPTool {
	return &MCPTool{
		Name:        "search_tickets",
		Description: "Search for tickets/tasks in the project management system.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"product_id": map[string]interface{}{
					"type":        "integer",
					"description": "Filter by product ID",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"backlog", "todo", "in_progress", "in_review", "done", "canceled"},
					"description": "Filter by ticket status",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"task", "bug", "feature", "improvement", "epic"},
					"description": "Filter by ticket type",
				},
				"priority": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"urgent", "high", "medium", "low", "none"},
					"description": "Filter by priority",
				},
				"assignee_id": map[string]interface{}{
					"type":        "integer",
					"description": "Filter by assignee user ID",
				},
				"parent_id": map[string]interface{}{
					"type":        "integer",
					"description": "Filter by parent ticket ID (for subtasks)",
				},
				"query": map[string]interface{}{
					"type":        "string",
					"description": "Search query for title/description",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum results (default: 20)",
				},
				"page": map[string]interface{}{
					"type":        "integer",
					"description": "Page number (default: 1)",
				},
			},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			productID := getIntPtrArg(args, "product_id")
			assigneeID := getIntPtrArg(args, "assignee_id")
			parentID := getIntPtrArg(args, "parent_id")
			query := getStringArg(args, "query")

			var status *tools.TicketStatus
			if s := getStringArg(args, "status"); s != "" {
				ts := tools.TicketStatus(s)
				status = &ts
			}

			var ticketType *tools.TicketType
			if t := getStringArg(args, "type"); t != "" {
				tt := tools.TicketType(t)
				ticketType = &tt
			}

			var priority *tools.TicketPriority
			if p := getStringArg(args, "priority"); p != "" {
				tp := tools.TicketPriority(p)
				priority = &tp
			}

			limit := getIntArg(args, "limit")
			if limit == 0 {
				limit = 20
			}

			page := getIntArg(args, "page")
			if page == 0 {
				page = 1
			}

			return client.SearchTickets(ctx, productID, status, ticketType, priority, assigneeID, parentID, query, limit, page)
		},
	}
}

func (s *HTTPServer) createGetTicketTool() *MCPTool {
	return &MCPTool{
		Name:        "get_ticket",
		Description: "Get details of a specific ticket by ID or identifier (e.g., 'AM-123').",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"ticket_id": map[string]interface{}{
					"type":        "string",
					"description": "Ticket ID (numeric) or identifier (e.g., 'AM-123')",
				},
			},
			"required": []string{"ticket_id"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			ticketID := getStringArg(args, "ticket_id")
			if ticketID == "" {
				return nil, fmt.Errorf("ticket_id is required")
			}
			return client.GetTicket(ctx, ticketID)
		},
	}
}

func (s *HTTPServer) createCreateTicketTool() *MCPTool {
	return &MCPTool{
		Name:        "create_ticket",
		Description: "Create a new ticket/task in the project management system.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"product_id": map[string]interface{}{
					"type":        "integer",
					"description": "The product ID to create the ticket in",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "Title of the ticket",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "Detailed description of the ticket",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"task", "bug", "feature", "improvement", "epic"},
					"description": "Type of ticket (default: task)",
				},
				"priority": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"urgent", "high", "medium", "low", "none"},
					"description": "Priority level (default: medium)",
				},
				"parent_ticket_id": map[string]interface{}{
					"type":        "integer",
					"description": "Parent ticket ID for creating subtasks",
				},
			},
			"required": []string{"product_id", "title"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			productID := getIntArg(args, "product_id")
			title := getStringArg(args, "title")
			description := getStringArg(args, "description")
			ticketType := getStringArg(args, "type")
			priority := getStringArg(args, "priority")
			parentTicketID := getIntPtrArg(args, "parent_ticket_id")

			if productID == 0 || title == "" {
				return nil, fmt.Errorf("product_id and title are required")
			}

			if ticketType == "" {
				ticketType = "task"
			}
			if priority == "" {
				priority = "medium"
			}

			return client.CreateTicket(ctx, productID, title, description, tools.TicketType(ticketType), tools.TicketPriority(priority), parentTicketID)
		},
	}
}

func (s *HTTPServer) createUpdateTicketTool() *MCPTool {
	return &MCPTool{
		Name:        "update_ticket",
		Description: "Update an existing ticket's fields.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"ticket_id": map[string]interface{}{
					"type":        "string",
					"description": "Ticket ID or identifier to update",
				},
				"title": map[string]interface{}{
					"type":        "string",
					"description": "New title (optional)",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "New description (optional)",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"backlog", "todo", "in_progress", "in_review", "done", "canceled"},
					"description": "New status (optional)",
				},
				"priority": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"urgent", "high", "medium", "low", "none"},
					"description": "New priority (optional)",
				},
				"type": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"task", "bug", "feature", "improvement", "epic"},
					"description": "New type (optional)",
				},
			},
			"required": []string{"ticket_id"},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			ticketID := getStringArg(args, "ticket_id")
			if ticketID == "" {
				return nil, fmt.Errorf("ticket_id is required")
			}

			var title, description *string
			if v := getStringArg(args, "title"); v != "" {
				title = &v
			}
			if v := getStringArg(args, "description"); v != "" {
				description = &v
			}

			var status *tools.TicketStatus
			if s := getStringArg(args, "status"); s != "" {
				ts := tools.TicketStatus(s)
				status = &ts
			}

			var priority *tools.TicketPriority
			if p := getStringArg(args, "priority"); p != "" {
				tp := tools.TicketPriority(p)
				priority = &tp
			}

			var ticketType *tools.TicketType
			if t := getStringArg(args, "type"); t != "" {
				tt := tools.TicketType(t)
				ticketType = &tt
			}

			return client.UpdateTicket(ctx, ticketID, title, description, status, priority, ticketType)
		},
	}
}

// Session Tools

func (s *HTTPServer) createCreateSessionTool() *MCPTool {
	return &MCPTool{
		Name:        "create_devpod_session",
		Description: "Create a new DevPod agent session. The new session will automatically have terminal:read and terminal:write permissions to the creator.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"runner_id": map[string]interface{}{
					"type":        "integer",
					"description": "ID of the runner to create the session on (optional, uses available runner)",
				},
				"ticket_id": map[string]interface{}{
					"type":        "integer",
					"description": "ID of the ticket to associate with the session",
				},
				"initial_prompt": map[string]interface{}{
					"type":        "string",
					"description": "Initial prompt to send to the new agent session",
				},
				"model": map[string]interface{}{
					"type":        "string",
					"description": "AI model to use for the session",
				},
			},
		},
		Handler: func(ctx context.Context, client tools.CollaborationClient, args map[string]interface{}) (interface{}, error) {
			req := &tools.SessionCreateRequest{
				InitialPrompt: getStringArg(args, "initial_prompt"),
				Model:         getStringArg(args, "model"),
			}

			if v := getIntArg(args, "runner_id"); v != 0 {
				req.RunnerID = v
			}
			if v := getIntPtrArg(args, "ticket_id"); v != nil {
				req.TicketID = v
			}

			return client.CreateSession(ctx, req)
		},
	}
}
