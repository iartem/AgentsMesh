package mcp

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/anthropics/agentsmesh/runner/internal/mcp/tools"
)

// Ticket Operations

// SearchTickets searches for tickets.
func (c *BackendClient) SearchTickets(ctx context.Context, repositoryID *int, status *tools.TicketStatus, ticketType *tools.TicketType, priority *tools.TicketPriority, assigneeID, parentID *int, query string, limit, page int) ([]tools.Ticket, error) {
	params := url.Values{}
	params.Set("limit", strconv.Itoa(limit))
	params.Set("page", strconv.Itoa(page))

	if repositoryID != nil {
		params.Set("repository_id", strconv.Itoa(*repositoryID))
	}
	if status != nil {
		params.Set("status", string(*status))
	}
	if ticketType != nil {
		params.Set("type", string(*ticketType))
	}
	if priority != nil {
		params.Set("priority", string(*priority))
	}
	if assigneeID != nil {
		params.Set("assignee_id", strconv.Itoa(*assigneeID))
	}
	if parentID != nil {
		params.Set("parent_id", strconv.Itoa(*parentID))
	}
	if query != "" {
		params.Set("query", query)
	}

	path := c.podAPIPath() + "/tickets?" + params.Encode()

	var result struct {
		Tickets []tools.Ticket `json:"tickets"`
	}
	err := c.request(ctx, http.MethodGet, path, nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Tickets, nil
}

// GetTicket gets a ticket by ID or identifier (e.g., "AM-123").
func (c *BackendClient) GetTicket(ctx context.Context, ticketID string) (*tools.Ticket, error) {
	var result struct {
		Ticket tools.Ticket `json:"ticket"`
	}
	err := c.request(ctx, http.MethodGet, fmt.Sprintf("%s/tickets/%s", c.podAPIPath(), url.PathEscape(ticketID)), nil, &result)
	if err != nil {
		return nil, err
	}
	return &result.Ticket, nil
}

// CreateTicket creates a new ticket.
func (c *BackendClient) CreateTicket(ctx context.Context, repositoryID *int64, title, description string, ticketType tools.TicketType, priority tools.TicketPriority, parentTicketID *int64) (*tools.Ticket, error) {
	body := map[string]interface{}{
		"title":       title,
		"description": description,
		"type":        ticketType,
		"priority":    priority,
	}
	if repositoryID != nil {
		body["repository_id"] = *repositoryID
	}
	if parentTicketID != nil {
		body["parent_ticket_id"] = *parentTicketID
	}

	var result struct {
		Ticket tools.Ticket `json:"ticket"`
	}
	err := c.request(ctx, http.MethodPost, c.podAPIPath()+"/tickets", body, &result)
	if err != nil {
		return nil, err
	}
	return &result.Ticket, nil
}

// UpdateTicket updates a ticket.
func (c *BackendClient) UpdateTicket(ctx context.Context, ticketID string, title, description *string, status *tools.TicketStatus, priority *tools.TicketPriority, ticketType *tools.TicketType) (*tools.Ticket, error) {
	body := make(map[string]interface{})
	if title != nil {
		body["title"] = *title
	}
	if description != nil {
		body["description"] = *description
	}
	if status != nil {
		body["status"] = *status
	}
	if priority != nil {
		body["priority"] = *priority
	}
	if ticketType != nil {
		body["type"] = *ticketType
	}

	var result struct {
		Ticket tools.Ticket `json:"ticket"`
	}
	err := c.request(ctx, http.MethodPut, fmt.Sprintf("%s/tickets/%s", c.podAPIPath(), url.PathEscape(ticketID)), body, &result)
	if err != nil {
		return nil, err
	}
	return &result.Ticket, nil
}

// Pod Operations

// CreatePod creates a new AgentPod.
func (c *BackendClient) CreatePod(ctx context.Context, req *tools.PodCreateRequest) (*tools.PodCreateResponse, error) {
	var result struct {
		Pod struct {
			PodKey string `json:"pod_key"`
			Status string `json:"status"`
		} `json:"pod"`
	}
	err := c.request(ctx, http.MethodPost, c.podAPIPath()+"/pods", req, &result)
	if err != nil {
		return nil, err
	}
	return &tools.PodCreateResponse{
		PodKey: result.Pod.PodKey,
		Status: result.Pod.Status,
	}, nil
}
