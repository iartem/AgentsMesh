package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/anthropics/agentsmesh/runner/internal/mcp/tools"
)

// Channel Operations

// SearchChannels searches for collaboration channels.
func (c *BackendClient) SearchChannels(ctx context.Context, name string, repositoryID, ticketID *int, isArchived *bool, offset, limit int) ([]tools.Channel, error) {
	params := url.Values{}
	if name != "" {
		params.Set("name", name)
	}
	if repositoryID != nil {
		params.Set("repository_id", strconv.Itoa(*repositoryID))
	}
	if ticketID != nil {
		params.Set("ticket_id", strconv.Itoa(*ticketID))
	}
	if isArchived != nil {
		params.Set("is_archived", strconv.FormatBool(*isArchived))
	}
	params.Set("offset", strconv.Itoa(offset))
	params.Set("limit", strconv.Itoa(limit))

	path := c.podAPIPath() + "/channels?" + params.Encode()

	var result struct {
		Channels []tools.Channel `json:"channels"`
	}
	err := c.request(ctx, http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Channels, nil
}

// CreateChannel creates a new collaboration channel.
func (c *BackendClient) CreateChannel(ctx context.Context, name, description string, repositoryID, ticketID *int) (*tools.Channel, error) {
	body := map[string]interface{}{
		"name":        name,
		"description": description,
	}
	if repositoryID != nil {
		body["repository_id"] = *repositoryID
	}
	if ticketID != nil {
		body["ticket_id"] = *ticketID
	}

	var result struct {
		Channel tools.Channel `json:"channel"`
	}
	err := c.request(ctx, http.MethodPost, c.podAPIPath()+"/channels", body, &result)
	if err != nil {
		return nil, err
	}
	return &result.Channel, nil
}

// GetChannel gets a channel by ID.
func (c *BackendClient) GetChannel(ctx context.Context, channelID int) (*tools.Channel, error) {
	var result struct {
		Channel tools.Channel `json:"channel"`
	}
	err := c.request(ctx, http.MethodGet, fmt.Sprintf("%s/channels/%d", c.podAPIPath(), channelID), nil, &result)
	if err != nil {
		return nil, err
	}
	return &result.Channel, nil
}

// SendMessage sends a message to a channel.
func (c *BackendClient) SendMessage(ctx context.Context, channelID int, content string, msgType tools.ChannelMessageType, mentions []string, replyTo *int) (*tools.ChannelMessage, error) {
	body := map[string]interface{}{
		"content":      content,
		"message_type": msgType,
	}
	if len(mentions) > 0 {
		body["mentions"] = mentions
	}
	if replyTo != nil {
		body["reply_to"] = *replyTo
	}

	var result struct {
		Message tools.ChannelMessage `json:"message"`
	}
	err := c.request(ctx, http.MethodPost, fmt.Sprintf("%s/channels/%d/messages", c.podAPIPath(), channelID), body, &result)
	if err != nil {
		return nil, err
	}
	return &result.Message, nil
}

// GetMessages gets messages from a channel.
func (c *BackendClient) GetMessages(ctx context.Context, channelID int, beforeTime, afterTime *string, mentionedPod *string, limit int) ([]tools.ChannelMessage, error) {
	params := url.Values{}
	if beforeTime != nil {
		params.Set("before_time", *beforeTime)
	}
	if afterTime != nil {
		params.Set("after_time", *afterTime)
	}
	if mentionedPod != nil {
		params.Set("mentioned_pod", *mentionedPod)
	}
	params.Set("limit", strconv.Itoa(limit))

	path := fmt.Sprintf("%s/channels/%d/messages?%s", c.podAPIPath(), channelID, params.Encode())

	var result struct {
		Messages []tools.ChannelMessage `json:"messages"`
	}
	err := c.request(ctx, http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Messages, nil
}

// GetDocument gets the shared document from a channel.
func (c *BackendClient) GetDocument(ctx context.Context, channelID int) (string, error) {
	var result struct {
		Document string `json:"document"`
	}
	err := c.request(ctx, http.MethodGet, fmt.Sprintf("%s/channels/%d/document", c.podAPIPath(), channelID), nil, &result)
	if err != nil {
		return "", err
	}
	return result.Document, nil
}

// UpdateDocument updates the shared document in a channel.
func (c *BackendClient) UpdateDocument(ctx context.Context, channelID int, document string) error {
	body := map[string]interface{}{
		"document": document,
	}
	return c.request(ctx, http.MethodPut, fmt.Sprintf("%s/channels/%d/document", c.podAPIPath(), channelID), body, nil)
}
