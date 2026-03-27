package channel

import (
	"context"
	"log/slog"
	"slices"
	"strconv"
	"time"

	"github.com/anthropics/agentsmesh/backend/internal/domain/channel"
)

// SendMessage sends a message to a channel.
// mentions is an optional list of structured mention declarations from the caller.
func (s *Service) SendMessage(ctx context.Context, channelID int64, senderPod *string, senderUserID *int64, messageType, content string, metadata channel.MessageMetadata, mentions []MentionInput) (*channel.Message, error) {
	ch, err := s.GetChannel(ctx, channelID)
	if err != nil {
		return nil, err
	}

	if ch.IsArchived {
		return nil, ErrChannelArchived
	}

	// Pre-process structured mentions into metadata before persistence
	var mentionResult *MentionResult
	if len(mentions) > 0 {
		if metadata == nil {
			metadata = make(channel.MessageMetadata)
		}
		var userIDs []int64
		var podKeys []string
		for _, m := range mentions {
			switch m.Type {
			case "user":
				if id, err := strconv.ParseInt(m.ID, 10, 64); err == nil {
					userIDs = append(userIDs, id)
				}
			case "pod":
				podKeys = append(podKeys, m.ID)
			}
		}
		if len(userIDs) > 0 {
			metadata[MetaMentionedUsers] = userIDs
		}
		if len(podKeys) > 0 {
			metadata[MetaMentionedPods] = podKeys
		}
		mentionResult = &MentionResult{UserIDs: userIDs, PodKeys: podKeys}
	}

	msg := &channel.Message{
		ChannelID:    channelID,
		SenderPod:    senderPod,
		SenderUserID: senderUserID,
		MessageType:  messageType,
		Content:      content,
		Metadata:     metadata,
	}

	if err := s.repo.CreateMessage(ctx, msg); err != nil {
		return nil, err
	}

	// Auto-join human sender as channel member (idempotent)
	if senderUserID != nil {
		_ = s.repo.UpsertMember(ctx, channelID, *senderUserID)
	}

	// Update channel updated_at
	_ = s.repo.TouchChannel(ctx, channelID)

	// Run PostSendHooks (mention validation, event publish, notifications, etc.)
	if len(s.postSendHooks) > 0 {
		mc := &MessageContext{Channel: ch, Message: msg, Mentions: mentionResult}
		for _, hook := range s.postSendHooks {
			if err := hook(ctx, mc); err != nil {
				slog.Error("post-send hook failed", "error", err)
			}
		}
	}

	return msg, nil
}

// GetMessages returns messages for a channel.
// Returns (messages, hasMore, error).
// When after is set without before: hasMore means newer messages exist beyond the limit.
// Otherwise: hasMore means older messages exist (scroll-up / load-more).
func (s *Service) GetMessages(ctx context.Context, channelID int64, before *time.Time, after *time.Time, limit int) ([]*channel.Message, bool, error) {
	// Fetch limit+1 to determine if more messages exist
	messages, err := s.repo.GetMessages(ctx, channelID, before, after, limit+1)
	if err != nil {
		return nil, false, err
	}

	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}

	// After-only queries return ASC from the repo (oldest-first); no reverse needed.
	// All other cases return DESC and must be reversed to chronological order.
	if after == nil || before != nil {
		slices.Reverse(messages)
	}

	return messages, hasMore, nil
}

// SendSystemMessage sends a system message to a channel
func (s *Service) SendSystemMessage(ctx context.Context, channelID int64, content string) (*channel.Message, error) {
	return s.SendMessage(ctx, channelID, nil, nil, channel.MessageTypeSystem, content, channel.MessageMetadata{}, nil)
}

// SendMessageAsUser sends a message as a user (human) to a channel
func (s *Service) SendMessageAsUser(ctx context.Context, channelID int64, userID int64, content string, metadata channel.MessageMetadata, mentions []MentionInput) (*channel.Message, error) {
	return s.SendMessage(ctx, channelID, nil, &userID, channel.MessageTypeText, content, metadata, mentions)
}

// SendMessageAsPod sends a message as a pod (agent) to a channel
func (s *Service) SendMessageAsPod(ctx context.Context, channelID int64, podKey string, content string, metadata channel.MessageMetadata, mentions []MentionInput) (*channel.Message, error) {
	return s.SendMessage(ctx, channelID, &podKey, nil, channel.MessageTypeText, content, metadata, mentions)
}

// GetMessagesMentioning returns messages mentioning a specific pod.
// Uses JSONB query on structured metadata with text LIKE fallback for legacy messages.
// Returns (messages, hasMore, error) where hasMore indicates if more results exist beyond the limit.
func (s *Service) GetMessagesMentioning(ctx context.Context, channelID int64, podKey string, limit int) ([]*channel.Message, bool, error) {
	messages, hasMore, err := s.repo.GetMessagesMentioning(ctx, channelID, podKey, limit)
	if err != nil {
		return nil, false, err
	}
	slices.Reverse(messages)
	return messages, hasMore, nil
}

// GetMessagesByCursor returns messages before a given message ID (cursor-based pagination).
// Returns (messages, hasMore, error) where hasMore indicates if older messages exist.
func (s *Service) GetMessagesByCursor(ctx context.Context, channelID int64, beforeID int64, limit int) ([]*channel.Message, bool, error) {
	// Fetch limit+1 to determine if more messages exist
	messages, err := s.repo.GetMessagesBefore(ctx, channelID, beforeID, limit+1)
	if err != nil {
		return nil, false, err
	}

	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}

	slices.Reverse(messages)

	return messages, hasMore, nil
}

// GetRecentMessages returns the most recent messages from a channel
func (s *Service) GetRecentMessages(ctx context.Context, channelID int64, limit int) ([]*channel.Message, error) {
	messages, err := s.repo.GetRecentMessages(ctx, channelID, limit)
	if err != nil {
		return nil, err
	}

	slices.Reverse(messages)
	return messages, nil
}
