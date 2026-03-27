package grpc

import (
	"context"
	"strings"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/middleware"
	"github.com/anthropics/agentsmesh/backend/internal/service/channel"
)

// mcpSendMessage handles the "send_message" MCP method.
func (a *GRPCRunnerAdapter) mcpSendMessage(ctx context.Context, tc *middleware.TenantContext, podKey string, payload []byte) (interface{}, *mcpError) {
	var params struct {
		ChannelID   int64    `json:"channel_id"`
		Content     string   `json:"content"`
		MessageType string   `json:"message_type"`
		Mentions    []string `json:"mentions"`
	}
	if err := unmarshalPayload(payload, &params); err != nil {
		return nil, err
	}
	if params.ChannelID == 0 {
		return nil, newMcpError(400, "channel_id is required")
	}
	if params.Content == "" {
		return nil, newMcpError(400, "content is required")
	}

	ch, mcpErr := a.validateChannelAccess(ctx, tc, params.ChannelID)
	if mcpErr != nil {
		return nil, mcpErr
	}
	if ch.IsArchived {
		return nil, newMcpError(400, "cannot send messages to archived channel")
	}

	msgType := params.MessageType
	if msgType == "" {
		msgType = "text"
	}

	var mentions []channel.MentionInput
	for _, m := range params.Mentions {
		parts := strings.SplitN(m, ":", 2)
		if len(parts) == 2 {
			mentions = append(mentions, channel.MentionInput{Type: parts[0], ID: parts[1]})
		}
	}

	msg, err := a.channelService.SendMessage(ctx, params.ChannelID, &podKey, &tc.UserID, msgType, params.Content, nil, mentions)
	if err != nil {
		return nil, newMcpError(500, "failed to send message")
	}
	return map[string]interface{}{"message": msg}, nil
}

// mcpGetMessages handles the "get_messages" MCP method.
func (a *GRPCRunnerAdapter) mcpGetMessages(ctx context.Context, tc *middleware.TenantContext, payload []byte) (interface{}, *mcpError) {
	var params struct {
		ChannelID    int64   `json:"channel_id"`
		BeforeTime   *string `json:"before_time"`
		AfterTime    *string `json:"after_time"`
		MentionedPod *string `json:"mentioned_pod"`
		Limit        int     `json:"limit"`
	}
	if err := unmarshalPayload(payload, &params); err != nil {
		return nil, err
	}
	if params.ChannelID == 0 {
		return nil, newMcpError(400, "channel_id is required")
	}
	if _, mcpErr := a.validateChannelAccess(ctx, tc, params.ChannelID); mcpErr != nil {
		return nil, mcpErr
	}

	limit := params.Limit
	if limit <= 0 {
		limit = 50
	} else if limit > 100 {
		limit = 100
	}

	// If filtering by mentioned pod, use dedicated method
	if params.MentionedPod != nil && *params.MentionedPod != "" {
		messages, hasMore, err := a.channelService.GetMessagesMentioning(ctx, params.ChannelID, *params.MentionedPod, limit)
		if err != nil {
			return nil, newMcpError(500, "failed to get messages")
		}
		return map[string]interface{}{"messages": messages, "has_more": hasMore}, nil
	}

	// Parse time filters
	var before, after *time.Time
	if params.BeforeTime != nil && *params.BeforeTime != "" {
		if t, err := time.Parse(time.RFC3339, *params.BeforeTime); err == nil {
			before = &t
		} else {
			return nil, newMcpError(400, "invalid before_time format, expected RFC3339")
		}
	}
	if params.AfterTime != nil && *params.AfterTime != "" {
		if t, err := time.Parse(time.RFC3339, *params.AfterTime); err == nil {
			after = &t
		} else {
			return nil, newMcpError(400, "invalid after_time format, expected RFC3339")
		}
	}

	messages, hasMore, err := a.channelService.GetMessages(ctx, params.ChannelID, before, after, limit)
	if err != nil {
		return nil, newMcpError(500, "failed to get messages")
	}
	return map[string]interface{}{"messages": messages, "has_more": hasMore}, nil
}

// mcpGetDocument handles the "get_document" MCP method.
func (a *GRPCRunnerAdapter) mcpGetDocument(ctx context.Context, tc *middleware.TenantContext, payload []byte) (interface{}, *mcpError) {
	var params struct {
		ChannelID int64 `json:"channel_id"`
	}
	if err := unmarshalPayload(payload, &params); err != nil {
		return nil, err
	}
	if params.ChannelID == 0 {
		return nil, newMcpError(400, "channel_id is required")
	}
	ch, mcpErr := a.validateChannelAccess(ctx, tc, params.ChannelID)
	if mcpErr != nil {
		return nil, mcpErr
	}

	document := ""
	if ch.Document != nil {
		document = *ch.Document
	}
	return map[string]interface{}{"document": document}, nil
}

// mcpUpdateDocument handles the "update_document" MCP method.
func (a *GRPCRunnerAdapter) mcpUpdateDocument(ctx context.Context, tc *middleware.TenantContext, payload []byte) (interface{}, *mcpError) {
	var params struct {
		ChannelID int64  `json:"channel_id"`
		Document  string `json:"document"`
	}
	if err := unmarshalPayload(payload, &params); err != nil {
		return nil, err
	}
	if params.ChannelID == 0 {
		return nil, newMcpError(400, "channel_id is required")
	}
	if _, mcpErr := a.validateChannelAccess(ctx, tc, params.ChannelID); mcpErr != nil {
		return nil, mcpErr
	}

	_, err := a.channelService.UpdateChannel(ctx, params.ChannelID, nil, nil, &params.Document)
	if err != nil {
		return nil, newMcpError(500, "failed to update document")
	}
	return map[string]interface{}{"document": params.Document}, nil
}
