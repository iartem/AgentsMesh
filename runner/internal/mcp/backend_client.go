package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/anthropics/agentsmesh/runner/internal/mcp/tools"
)

// BackendClient calls the AgentsMesh Backend API for collaboration operations.
type BackendClient struct {
	baseURL    string
	orgSlug    string // Organization slug for org-scoped API paths
	podKey     string
	httpClient *http.Client
}

// NewBackendClient creates a new backend API client.
// orgSlug is required for org-scoped API paths (/api/v1/orgs/:slug/pod/*)
func NewBackendClient(baseURL, orgSlug, podKey string) *BackendClient {
	return &BackendClient{
		baseURL: baseURL,
		orgSlug: orgSlug,
		podKey:  podKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// SetPodKey updates the pod key for the client.
func (c *BackendClient) SetPodKey(podKey string) {
	c.podKey = podKey
}

// GetPodKey returns the current pod key.
func (c *BackendClient) GetPodKey() string {
	return c.podKey
}

// SetOrgSlug updates the organization slug for the client.
func (c *BackendClient) SetOrgSlug(orgSlug string) {
	c.orgSlug = orgSlug
}

// GetOrgSlug returns the current organization slug.
func (c *BackendClient) GetOrgSlug() string {
	return c.orgSlug
}

// podAPIPath returns the org-scoped pod API path prefix for MCP tools.
// MCP tools use /api/v1/orgs/:slug/pod/* with X-Pod-Key authentication.
func (c *BackendClient) podAPIPath() string {
	return fmt.Sprintf("/api/v1/orgs/%s/pod", c.orgSlug)
}

// request makes an HTTP request to the backend.
func (c *BackendClient) request(ctx context.Context, method, path string, body interface{}, result interface{}) error {
	fullURL := c.baseURL + path

	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Pod-Key", c.podKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	if result != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, result); err != nil {
			return fmt.Errorf("failed to unmarshal response: %w", err)
		}
	}

	return nil
}

// Verify BackendClient implements CollaborationClient interface.
var _ tools.CollaborationClient = (*BackendClient)(nil)
