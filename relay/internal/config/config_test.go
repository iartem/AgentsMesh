package config

import (
	"os"
	"testing"
	"time"
)

func clearEnv() {
	for _, env := range []string{
		"JWT_SECRET", "INTERNAL_API_SECRET", "SERVER_HOST", "SERVER_PORT", "JWT_ISSUER",
		"BACKEND_URL", "HEARTBEAT_INTERVAL", "KEEP_ALIVE_DURATION", "MAX_BROWSERS_PER_POD",
		"RELAY_ID", "RELAY_URL", "RELAY_INTERNAL_URL", "RELAY_REGION", "RELAY_CAPACITY",
		"RELAY_SERVER_HOST", "RELAY_SERVER_PORT", "RELAY_JWT_SECRET", "RELAY_JWT_ISSUER",
		"RELAY_BACKEND_URL", "RELAY_INTERNAL_API_SECRET",
	} {
		os.Unsetenv(env)
	}
}

func TestServerConfig_Address(t *testing.T) {
	tests := []struct{ host string; port int; expected string }{
		{"0.0.0.0", 8090, "0.0.0.0:8090"}, {"127.0.0.1", 8080, "127.0.0.1:8080"},
		{"localhost", 3000, "localhost:3000"}, {"", 80, ":80"},
	}
	for _, tt := range tests {
		cfg := &ServerConfig{Host: tt.host, Port: tt.port}
		if got := cfg.Address(); got != tt.expected {
			t.Errorf("Address() = %q, want %q", got, tt.expected)
		}
	}
}

func TestLoad_MissingSecrets(t *testing.T) {
	clearEnv()
	defer clearEnv()
	os.Setenv("INTERNAL_API_SECRET", "test")
	if _, err := Load(); err == nil {
		t.Error("expected error for missing JWT_SECRET")
	}
	clearEnv()
	os.Setenv("JWT_SECRET", "test")
	if _, err := Load(); err == nil {
		t.Error("expected error for missing INTERNAL_API_SECRET")
	}
}

func TestLoad_WithRequiredEnvVars(t *testing.T) {
	clearEnv()
	defer clearEnv()
	os.Setenv("JWT_SECRET", "test-jwt")
	os.Setenv("INTERNAL_API_SECRET", "test-internal")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	// Verify defaults
	if cfg.Server.Host != "0.0.0.0" || cfg.Server.Port != 8090 {
		t.Error("Server defaults wrong")
	}
	if cfg.Server.ReadTimeout != 30*time.Second || cfg.Server.WriteTimeout != 30*time.Second {
		t.Error("Server timeout defaults wrong")
	}
	if cfg.JWT.Issuer != "agentsmesh-relay" {
		t.Error("JWT issuer default wrong")
	}
	if cfg.Backend.URL != "http://backend:8080" || cfg.Backend.HeartbeatInterval != 10*time.Second {
		t.Error("Backend defaults wrong")
	}
	if cfg.Session.KeepAliveDuration != 30*time.Second || cfg.Session.MaxBrowsersPerPod != 10 {
		t.Error("Session defaults wrong")
	}
	if cfg.Relay.Capacity != 1000 || cfg.Relay.Region != "default" {
		t.Error("Relay defaults wrong")
	}
}

func TestLoad_EnvironmentOverrides(t *testing.T) {
	clearEnv()
	defer clearEnv()
	os.Setenv("JWT_SECRET", "test-jwt")
	os.Setenv("INTERNAL_API_SECRET", "test-internal")
	os.Setenv("SERVER_HOST", "192.168.1.1")
	os.Setenv("JWT_ISSUER", "custom-issuer")
	os.Setenv("BACKEND_URL", "http://custom:8080")
	os.Setenv("RELAY_ID", "relay-custom")
	os.Setenv("RELAY_URL", "ws://custom:8090")
	os.Setenv("RELAY_REGION", "us-west")
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Server.Host != "192.168.1.1" {
		t.Error("Server.Host override failed")
	}
	if cfg.JWT.Issuer != "custom-issuer" {
		t.Error("JWT.Issuer override failed")
	}
	if cfg.Backend.URL != "http://custom:8080" {
		t.Error("Backend.URL override failed")
	}
	if cfg.Relay.ID != "relay-custom" || cfg.Relay.URL != "ws://custom:8090" || cfg.Relay.Region != "us-west" {
		t.Error("Relay overrides failed")
	}
}

func TestLoad_RelayIDDefault(t *testing.T) {
	clearEnv()
	defer clearEnv()
	os.Setenv("JWT_SECRET", "test-jwt")
	os.Setenv("INTERNAL_API_SECRET", "test-internal")
	cfg, _ := Load()
	if cfg.Relay.ID == "" || len(cfg.Relay.ID) < 6 {
		t.Error("Relay.ID should be auto-generated")
	}
}

func TestLoad_RelayURLDefault(t *testing.T) {
	clearEnv()
	defer clearEnv()
	os.Setenv("JWT_SECRET", "test-jwt")
	os.Setenv("INTERNAL_API_SECRET", "test-internal")
	cfg, _ := Load()
	if cfg.Relay.URL != "ws://localhost:8090" {
		t.Errorf("Relay.URL: expected ws://localhost:8090, got %q", cfg.Relay.URL)
	}
}

func TestLoad_WithRelayPrefix(t *testing.T) {
	clearEnv()
	defer clearEnv()
	os.Setenv("JWT_SECRET", "test-jwt")
	os.Setenv("INTERNAL_API_SECRET", "test-internal")
	os.Setenv("RELAY_SERVER_HOST", "10.0.0.1")
	os.Setenv("RELAY_JWT_ISSUER", "prefixed-issuer")
	cfg, _ := Load()
	if cfg.Server.Host != "10.0.0.1" || cfg.JWT.Issuer != "prefixed-issuer" {
		t.Error("RELAY_ prefixed env vars not working")
	}
}

func TestLoad_InternalURL(t *testing.T) {
	clearEnv()
	defer clearEnv()
	os.Setenv("JWT_SECRET", "test-jwt")
	os.Setenv("INTERNAL_API_SECRET", "test-internal")
	os.Setenv("RELAY_INTERNAL_URL", "ws://relay:8090")
	cfg, _ := Load()
	if cfg.Relay.InternalURL != "ws://relay:8090" {
		t.Errorf("Relay.InternalURL: expected ws://relay:8090, got %q", cfg.Relay.InternalURL)
	}
}
