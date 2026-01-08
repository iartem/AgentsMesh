package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RegistrationRequest contains runner registration parameters.
type RegistrationRequest struct {
	ServerURL         string
	NodeID            string
	RegistrationToken string
	Description       string
	MaxSessions       int
}

// RegistrationResponse contains registration result.
type RegistrationResponse struct {
	AuthToken string `json:"auth_token"`
	RunnerID  int64  `json:"runner_id"`
}

// Register registers a runner with the server.
// This is an HTTP request, separate from the WebSocket connection.
func Register(ctx context.Context, req RegistrationRequest) (*RegistrationResponse, error) {
	// Build registration URL
	registerURL := fmt.Sprintf("%s/api/v1/runners/register", req.ServerURL)

	// Build request body
	body := map[string]interface{}{
		"node_id":                 req.NodeID,
		"description":             req.Description,
		"registration_token":      req.RegistrationToken,
		"max_concurrent_sessions": req.MaxSessions,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal registration body: %w", err)
	}

	// Create request
	httpReq, err := http.NewRequestWithContext(ctx, "POST", registerURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create registration request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	httpClient := &http.Client{Timeout: 30 * time.Second}
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("registration request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var result RegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode registration response: %w", err)
	}

	return &result, nil
}
