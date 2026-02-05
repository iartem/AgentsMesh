package mcp

import (
	"context"
	"net/http"

	"github.com/anthropics/agentsmesh/runner/internal/mcp/tools"
)

// Discovery Operations

// ListAvailablePods lists pods available for collaboration.
func (c *BackendClient) ListAvailablePods(ctx context.Context) ([]tools.AvailablePod, error) {
	var result struct {
		Pods []tools.AvailablePod `json:"pods"`
	}
	// Use pods endpoint with status filter
	err := c.request(ctx, http.MethodGet, c.podAPIPath()+"/pods?status=running", nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Pods, nil
}

// ListRunners returns simplified Runner list with nested Agent info.
func (c *BackendClient) ListRunners(ctx context.Context) ([]tools.RunnerSummary, error) {
	var result struct {
		Runners []tools.RunnerSummary `json:"runners"`
	}
	err := c.request(ctx, http.MethodGet, c.podAPIPath()+"/runners", nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Runners, nil
}

// ListRepositories lists repositories configured in the organization.
func (c *BackendClient) ListRepositories(ctx context.Context) ([]tools.Repository, error) {
	var result struct {
		Repositories []tools.Repository `json:"repositories"`
	}
	err := c.request(ctx, http.MethodGet, c.podAPIPath()+"/repositories", nil, &result)
	if err != nil {
		return nil, err
	}
	return result.Repositories, nil
}
