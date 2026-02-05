package config

import "fmt"

// ServerConfig holds server configuration
type ServerConfig struct {
	Address            string
	Debug              bool
	CORSAllowedOrigins []string
	InternalAPISecret  string // Secret for internal API authentication (Relay communication)
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host        string
	Port        int
	User        string
	Password    string
	DBName      string
	SSLMode     string
	ReplicaDSNs []string // Read replica DSNs for read-write separation
}

// DSN returns the PostgreSQL connection string for the master database
func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.DBName, c.SSLMode,
	)
}

// HasReplicas returns true if replica DSNs are configured
func (c DatabaseConfig) HasReplicas() bool {
	return len(c.ReplicaDSNs) > 0
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	URL      string
	Host     string
	Port     int
	Password string
	DB       int
}

// IsConfigured returns true if Redis is configured
func (c RedisConfig) IsConfigured() bool {
	return c.URL != "" || c.Host != ""
}

// JWTConfig holds JWT configuration
type JWTConfig struct {
	Secret          string
	ExpirationHours int
}

// WebhookConfig holds webhook secret configurations
type WebhookConfig struct {
	GitLabSecret string
	GitHubSecret string
	GiteeSecret  string
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level      string // debug, info, warn, error
	Format     string // json, text
	FilePath   string // path to log file, empty means stdout only
	MaxSizeMB  int    // max size in MB before rotation
	MaxBackups int    // max number of backup files
}
