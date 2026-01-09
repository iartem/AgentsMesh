package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

// Config holds all runner configuration
type Config struct {
	// Server connection
	ServerURL string `mapstructure:"server_url"`

	// Runner identification
	NodeID      string `mapstructure:"node_id"`
	Description string `mapstructure:"description"`

	// Authentication
	AuthToken         string `mapstructure:"auth_token"`
	RegistrationToken string `mapstructure:"registration_token"`

	// Capacity
	MaxConcurrentSessions int `mapstructure:"max_concurrent_sessions"`

	// Workspace settings
	WorkspaceRoot string `mapstructure:"workspace_root"`
	GitConfigPath string `mapstructure:"git_config_path"`

	// Worktree settings (for ticket-based development)
	RepositoryPath string `mapstructure:"repository_path"` // Path to the main git repository
	WorktreesDir   string `mapstructure:"worktrees_dir"`   // Directory for worktrees
	BaseBranch     string `mapstructure:"base_branch"`     // Base branch for new worktrees (default: main)

	// MCP settings
	MCPConfigPath string `mapstructure:"mcp_config_path"` // Path to MCP servers config file
	MCPPort       int    `mapstructure:"mcp_port"`        // MCP HTTP Server port (default: 19000)

	// Sandbox settings
	Workspace string `mapstructure:"workspace"` // Workspace root for sandboxes and repos cache

	// Agent settings
	DefaultAgent string            `mapstructure:"default_agent"`
	DefaultShell string            `mapstructure:"default_shell"` // Default shell for sessions
	AgentEnvVars map[string]string `mapstructure:"agent_env_vars"`

	// Health check
	HealthCheckPort int `mapstructure:"health_check_port"`

	// Logging
	LogLevel string `mapstructure:"log_level"`
	LogFile  string `mapstructure:"log_file"`
}

// Load loads configuration from file and environment
func Load(configFile string) (*Config, error) {
	v := viper.New()

	// Set defaults
	v.SetDefault("server_url", "https://api.agentmesh.dev")
	v.SetDefault("max_concurrent_sessions", 5)
	v.SetDefault("workspace_root", "/workspace")
	v.SetDefault("mcp_port", 19000)
	v.SetDefault("health_check_port", 9090)
	v.SetDefault("log_level", "info")
	v.SetDefault("default_agent", "claude-code")

	// Read from environment
	v.SetEnvPrefix("AGENTMESH")
	v.AutomaticEnv()

	// Read from config file if specified
	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		// Search for config in common locations
		v.SetConfigName("runner")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.agentmesh")
		v.AddConfigPath("/etc/agentmesh")
	}

	if err := v.ReadInConfig(); err != nil {
		// Config file not found is okay if we have env vars
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, err
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}

	// Generate node ID if not set
	if cfg.NodeID == "" {
		hostname, _ := os.Hostname()
		if hostname == "" {
			hostname = "runner"
		}
		cfg.NodeID = hostname
	}

	// Expand workspace root
	if cfg.WorkspaceRoot != "" {
		cfg.WorkspaceRoot = os.ExpandEnv(cfg.WorkspaceRoot)
	}

	return &cfg, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.ServerURL == "" {
		return errors.New("server_url is required")
	}

	if c.AuthToken == "" && c.RegistrationToken == "" {
		return errors.New("either auth_token or registration_token is required")
	}

	if c.MaxConcurrentSessions < 1 {
		return errors.New("max_concurrent_sessions must be at least 1")
	}

	// Ensure workspace root exists
	if c.WorkspaceRoot != "" {
		if err := os.MkdirAll(c.WorkspaceRoot, 0755); err != nil {
			return errors.New("failed to create workspace root: " + err.Error())
		}
	}

	return nil
}

// SaveAuthToken saves the auth token to a file for persistence
func (c *Config) SaveAuthToken(token string) error {
	c.AuthToken = token

	// Save to config file in home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	configDir := filepath.Join(home, ".agentmesh")
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return err
	}

	tokenFile := filepath.Join(configDir, "auth_token")
	return os.WriteFile(tokenFile, []byte(token), 0600)
}

// LoadAuthToken loads the auth token from file if not in config
func (c *Config) LoadAuthToken() error {
	if c.AuthToken != "" {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil // Not an error, token might be in env
	}

	tokenFile := filepath.Join(home, ".agentmesh", "auth_token")
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		return nil // Not an error
	}

	c.AuthToken = string(data)
	return nil
}

// GetWorkspace returns the workspace directory path.
// Falls back to WorkspaceRoot if Workspace is not set.
func (c *Config) GetWorkspace() string {
	if c.Workspace != "" {
		return c.Workspace
	}
	if c.WorkspaceRoot != "" {
		return c.WorkspaceRoot
	}
	// Default to user's home directory
	home, err := os.UserHomeDir()
	if err != nil {
		return "/tmp/agentmesh"
	}
	return filepath.Join(home, ".agentmesh")
}

// GetSandboxesDir returns the sandboxes directory path.
func (c *Config) GetSandboxesDir() string {
	return filepath.Join(c.GetWorkspace(), "sandboxes")
}

// GetReposDir returns the repository cache directory path.
func (c *Config) GetReposDir() string {
	return filepath.Join(c.GetWorkspace(), "repos")
}

// GetMCPPort returns the MCP HTTP Server port.
func (c *Config) GetMCPPort() int {
	if c.MCPPort > 0 {
		return c.MCPPort
	}
	return 19000 // Default port
}
