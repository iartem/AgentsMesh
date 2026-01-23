package server

import (
	"testing"
	"time"

	"github.com/anthropics/agentsmesh/relay/internal/config"
)

func TestNew(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:         "0.0.0.0",
			Port:         8090,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		JWT: config.JWTConfig{
			Secret: "test-secret",
			Issuer: "test-issuer",
		},
		Backend: config.BackendConfig{
			URL:               "http://localhost:8080",
			InternalAPISecret: "internal-secret",
			HeartbeatInterval: 10 * time.Second,
		},
		Session: config.SessionConfig{
			KeepAliveDuration: 30 * time.Second,
			MaxBrowsersPerPod: 10,
		},
		Relay: config.RelayConfig{
			ID:          "relay-1",
			URL:         "ws://localhost:8090",
			InternalURL: "ws://relay:8090",
			Region:      "us-west",
			Capacity:    1000,
		},
	}

	server := New(cfg)

	if server == nil {
		t.Fatal("New returned nil")
	}
	if server.cfg != cfg {
		t.Error("cfg not set correctly")
	}
	if server.sessionManager == nil {
		t.Error("sessionManager should not be nil")
	}
	if server.backendClient == nil {
		t.Error("backendClient should not be nil")
	}
	if server.handler == nil {
		t.Error("handler should not be nil")
	}
	if server.logger == nil {
		t.Error("logger should not be nil")
	}
}

func TestServer_Stats(t *testing.T) {
	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:         "0.0.0.0",
			Port:         8090,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		JWT: config.JWTConfig{
			Secret: "test-secret",
			Issuer: "test-issuer",
		},
		Backend: config.BackendConfig{
			URL:               "http://localhost:8080",
			InternalAPISecret: "internal-secret",
			HeartbeatInterval: 10 * time.Second,
		},
		Session: config.SessionConfig{
			KeepAliveDuration: 30 * time.Second,
			MaxBrowsersPerPod: 10,
		},
		Relay: config.RelayConfig{
			ID:          "relay-1",
			URL:         "ws://localhost:8090",
			InternalURL: "ws://relay:8090",
			Region:      "us-west",
			Capacity:    1000,
		},
	}

	server := New(cfg)
	stats := server.Stats()

	// Initial stats should be zero
	if stats.ActiveSessions != 0 {
		t.Errorf("ActiveSessions: expected 0, got %d", stats.ActiveSessions)
	}
	if stats.TotalBrowsers != 0 {
		t.Errorf("TotalBrowsers: expected 0, got %d", stats.TotalBrowsers)
	}
	if stats.PendingRunners != 0 {
		t.Errorf("PendingRunners: expected 0, got %d", stats.PendingRunners)
	}
	if stats.PendingBrowsers != 0 {
		t.Errorf("PendingBrowsers: expected 0, got %d", stats.PendingBrowsers)
	}
}
