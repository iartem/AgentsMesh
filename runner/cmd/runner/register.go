package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// registrationClient handles runner registration with the server
type registrationClient struct {
	serverURL string
	nodeID    string
}

// register registers the runner with the server and returns the auth token
func (c *registrationClient) register(ctx context.Context, registrationToken, description string, maxSessions int) (string, error) {
	// Build registration URL
	registerURL := fmt.Sprintf("%s/api/v1/runners/register", c.serverURL)

	// Build request body
	body := map[string]interface{}{
		"node_id":                 c.nodeID,
		"description":             description,
		"registration_token":      registrationToken,
		"max_concurrent_sessions": maxSessions,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("failed to marshal registration body: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", registerURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("failed to create registration request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("registration request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("registration failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	// Parse response
	var result struct {
		AuthToken string `json:"auth_token"`
		RunnerID  int64  `json:"runner_id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode registration response: %w", err)
	}

	if result.AuthToken == "" {
		return "", fmt.Errorf("server returned empty auth token")
	}

	return result.AuthToken, nil
}

// savedConfig represents the configuration saved to ~/.agentmesh/config.yaml
type savedConfig struct {
	ServerURL             string `yaml:"server_url"`
	NodeID                string `yaml:"node_id"`
	Description           string `yaml:"description"`
	MaxConcurrentSessions int    `yaml:"max_concurrent_sessions"`
	WorkspaceRoot         string `yaml:"workspace_root"`
	DefaultAgent          string `yaml:"default_agent"`
	DefaultShell          string `yaml:"default_shell"`
	HealthCheckPort       int    `yaml:"health_check_port"`
	LogLevel              string `yaml:"log_level"`
}

// saveConfig saves the registration result to ~/.agentmesh/
func saveConfig(nodeID, serverURL, authToken, description string, maxSessions int) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	configDir := filepath.Join(home, ".agentmesh")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Save auth token
	tokenFile := filepath.Join(configDir, "auth_token")
	if err := os.WriteFile(tokenFile, []byte(authToken), 0600); err != nil {
		return fmt.Errorf("failed to save auth token: %w", err)
	}

	// Save config
	cfg := savedConfig{
		ServerURL:             serverURL,
		NodeID:                nodeID,
		Description:           description,
		MaxConcurrentSessions: maxSessions,
		WorkspaceRoot:         "/tmp/agentmesh-workspace",
		DefaultAgent:          "claude-code",
		DefaultShell:          getDefaultShell(),
		HealthCheckPort:       9090,
		LogLevel:              "info",
	}

	configData, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	configFile := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configFile, configData, 0600); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}

// getDefaultShell returns the default shell for the current platform
func getDefaultShell() string {
	shell := os.Getenv("SHELL")
	if shell != "" {
		return shell
	}
	// Default to /bin/sh if SHELL is not set
	return "/bin/sh"
}
