package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// DeploymentType represents the deployment environment
type DeploymentType string

const (
	DeploymentGlobal    DeploymentType = "global"    // International - Stripe
	DeploymentCN        DeploymentType = "cn"        // China - Alipay + WeChat Pay
	DeploymentOnPremise DeploymentType = "onpremise" // Self-hosted - License file
)

// Config holds all configuration for the application
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	JWT      JWTConfig
	OAuth    OAuthConfig
	Webhook  WebhookConfig
	Log      LogConfig
	Email    EmailConfig
	Storage  StorageConfig
	Payment  PaymentConfig
	PKI      PKIConfig
	GRPC     GRPCConfig
	Admin    AdminConfig
	Relay    RelayConfig

	// Unified domain configuration - all URLs are derived from these two values
	PrimaryDomain string // Primary domain (e.g., "localhost:10000" or "agentsmesh.com")
	UseHTTPS      bool   // Use HTTPS/WSS protocols
}

// =============================================================================
// URL Derivation Methods - All URLs are derived from PrimaryDomain + UseHTTPS
// =============================================================================

// BaseURL returns the base URL with protocol (http:// or https://)
func (c *Config) BaseURL() string {
	protocol := "http"
	if c.UseHTTPS {
		protocol = "https"
	}
	return fmt.Sprintf("%s://%s", protocol, c.PrimaryDomain)
}

// WebSocketBaseURL returns the WebSocket base URL (ws:// or wss://)
func (c *Config) WebSocketBaseURL() string {
	protocol := "ws"
	if c.UseHTTPS {
		protocol = "wss"
	}
	return fmt.Sprintf("%s://%s", protocol, c.PrimaryDomain)
}

// FrontendURL returns the frontend URL (same as BaseURL for unified domain)
func (c *Config) FrontendURL() string {
	return c.BaseURL()
}

// APIBaseURL returns the API base URL
func (c *Config) APIBaseURL() string {
	return c.BaseURL() + "/api"
}

// RelayURL returns the Relay WebSocket URL
func (c *Config) RelayURL() string {
	return c.WebSocketBaseURL() + "/relay"
}

// GitHubRedirectURL returns the GitHub OAuth callback URL
func (c *Config) GitHubRedirectURL() string {
	return c.BaseURL() + "/api/v1/auth/oauth/github/callback"
}

// GoogleRedirectURL returns the Google OAuth callback URL
func (c *Config) GoogleRedirectURL() string {
	return c.BaseURL() + "/api/v1/auth/oauth/google/callback"
}

// GitLabRedirectURL returns the GitLab OAuth callback URL
func (c *Config) GitLabRedirectURL() string {
	return c.BaseURL() + "/api/v1/auth/oauth/gitlab/callback"
}

// GiteeRedirectURL returns the Gitee OAuth callback URL
func (c *Config) GiteeRedirectURL() string {
	return c.BaseURL() + "/api/v1/auth/oauth/gitee/callback"
}

// AlipayNotifyURL returns the Alipay payment notification URL
func (c *Config) AlipayNotifyURL() string {
	return c.BaseURL() + "/api/v1/webhooks/alipay"
}

// LemonSqueezyWebhookURL returns the LemonSqueezy webhook URL
func (c *Config) LemonSqueezyWebhookURL() string {
	return c.BaseURL() + "/api/v1/webhooks/lemonsqueezy"
}

// AlipayReturnURL returns the Alipay payment return URL
func (c *Config) AlipayReturnURL() string {
	return c.BaseURL()
}

// WeChatNotifyURL returns the WeChat Pay notification URL
func (c *Config) WeChatNotifyURL() string {
	return c.BaseURL() + "/api/v1/webhooks/wechat"
}

// AdminFrontendURL returns the admin console URL
func (c *Config) AdminFrontendURL() string {
	return c.BaseURL() + "/admin"
}

// PaymentConfig holds payment and billing configuration
type PaymentConfig struct {
	DeploymentType DeploymentType
	MockEnabled    bool   // Enable mock payment provider for testing
	MockBaseURL    string // Base URL for mock checkout pages
	Stripe         StripeConfig
	LemonSqueezy   LemonSqueezyConfig
	Alipay         AlipayConfig
	WeChat         WeChatConfig
	License        LicenseConfig
}

// StripeConfig holds Stripe payment configuration
type StripeConfig struct {
	SecretKey      string
	PublishableKey string
	WebhookSecret  string
}

// AlipayConfig holds Alipay payment configuration
// Note: NotifyURL and ReturnURL are derived from Config.PrimaryDomain
type AlipayConfig struct {
	AppID           string
	PrivateKey      string
	AlipayPublicKey string
	IsSandbox       bool
}

// WeChatConfig holds WeChat Pay configuration
// Note: NotifyURL is derived from Config.PrimaryDomain
type WeChatConfig struct {
	AppID     string
	MchID     string
	APIKey    string
	APIv3Key  string
	CertPath  string
	KeyPath   string
	IsSandbox bool
}

// LicenseConfig holds OnPremise license configuration
type LicenseConfig struct {
	PublicKeyPath    string // Path to public key for license verification
	LicenseFilePath  string // Path to license file
	LicenseServerURL string // Optional: License server URL for online verification
}

// LemonSqueezyConfig holds LemonSqueezy payment configuration
type LemonSqueezyConfig struct {
	APIKey        string // LemonSqueezy API key
	StoreID       string // LemonSqueezy Store ID
	WebhookSecret string // Webhook signing secret for signature verification
}

// IsGlobal returns true if deployment is for international users (Stripe)
func (c PaymentConfig) IsGlobal() bool {
	return c.DeploymentType == DeploymentGlobal
}

// IsCN returns true if deployment is for China users (Alipay + WeChat)
func (c PaymentConfig) IsCN() bool {
	return c.DeploymentType == DeploymentCN
}

// IsOnPremise returns true if deployment is self-hosted (License)
func (c PaymentConfig) IsOnPremise() bool {
	return c.DeploymentType == DeploymentOnPremise
}

// StripeEnabled returns true if Stripe is configured and enabled
func (c PaymentConfig) StripeEnabled() bool {
	return c.IsGlobal() && c.Stripe.SecretKey != ""
}

// AlipayEnabled returns true if Alipay is configured and enabled
func (c PaymentConfig) AlipayEnabled() bool {
	return c.IsCN() && c.Alipay.AppID != ""
}

// WeChatEnabled returns true if WeChat Pay is configured and enabled
func (c PaymentConfig) WeChatEnabled() bool {
	return c.IsCN() && c.WeChat.AppID != "" && c.WeChat.MchID != ""
}

// LicenseEnabled returns true if license verification is enabled
func (c PaymentConfig) LicenseEnabled() bool {
	return c.IsOnPremise() && c.License.PublicKeyPath != ""
}

// LemonSqueezyEnabled returns true if LemonSqueezy is configured and enabled
func (c PaymentConfig) LemonSqueezyEnabled() bool {
	return c.IsGlobal() && c.LemonSqueezy.APIKey != ""
}

// LemonSqueezyFullyConfigured returns true if LemonSqueezy is fully configured
// including webhook secret (required for production webhook signature verification)
func (c PaymentConfig) LemonSqueezyFullyConfigured() bool {
	return c.LemonSqueezyEnabled() &&
		c.LemonSqueezy.StoreID != "" &&
		c.LemonSqueezy.WebhookSecret != ""
}

// IsMockEnabled returns true if mock payment provider is enabled (for testing)
func (c PaymentConfig) IsMockEnabled() bool {
	return c.MockEnabled
}

// GetAvailableProviders returns list of available payment providers
func (c PaymentConfig) GetAvailableProviders() []string {
	// If mock is enabled, only return mock provider
	if c.MockEnabled {
		return []string{"mock"}
	}

	var providers []string
	if c.LemonSqueezyEnabled() {
		providers = append(providers, "lemonsqueezy")
	}
	if c.StripeEnabled() {
		providers = append(providers, "stripe")
	}
	if c.AlipayEnabled() {
		providers = append(providers, "alipay")
	}
	if c.WeChatEnabled() {
		providers = append(providers, "wechat")
	}
	if c.LicenseEnabled() {
		providers = append(providers, "license")
	}
	return providers
}

// PKIConfig holds PKI (certificate) configuration for Runner mTLS authentication
// Required for Runner communication via gRPC + mTLS
type PKIConfig struct {
	CACertFile     string // Path to CA certificate file (required)
	CAKeyFile      string // Path to CA private key file (required)
	ServerCertFile string // Path to server certificate file (optional, generated if not set)
	ServerKeyFile  string // Path to server private key file (optional)
	ValidityDays   int    // Certificate validity period in days (default: 365)
}

// GRPCConfig holds gRPC server configuration for Runner connections
// gRPC server starts automatically when PKI CA files are configured
type GRPCConfig struct {
	Address  string // gRPC server listen address (default: :9090)
	Endpoint string // Public gRPC endpoint URL for Runners (e.g., grpcs://api.agentsmesh.cn:9443)
}

// AdminConfig holds admin console configuration
// Note: FrontendURL is derived from Config.PrimaryDomain + "/admin"
type AdminConfig struct {
	Enabled bool // Enable admin console
}

// IsEnabled returns true if admin console is enabled
func (c AdminConfig) IsEnabled() bool {
	return c.Enabled
}

// RelayConfig holds Relay server management configuration
type RelayConfig struct {
	// Domain configuration for auto-generated Relay URLs
	BaseDomain string // Base domain for relay subdomains (e.g., "relay.agentsmesh.cn")
	UseHTTPS   bool   // Use wss:// instead of ws://

	// DNS provider configuration
	DNS DNSConfig

	// ACME (Let's Encrypt) configuration for automatic TLS certificates
	ACME ACMEConfig
}

// DNSConfig holds DNS provider configuration
type DNSConfig struct {
	Provider string // "cloudflare" or "aliyun"

	// Cloudflare settings
	CloudflareAPIToken string // API token with DNS edit permissions
	CloudflareZoneID   string // Zone ID for the domain

	// Aliyun DNS settings
	AliyunAccessKeyID     string
	AliyunAccessKeySecret string
}

// ACMEConfig holds ACME (Let's Encrypt) configuration
type ACMEConfig struct {
	Enabled      bool   // Enable ACME certificate management
	Email        string // Email for Let's Encrypt registration
	DirectoryURL string // ACME directory URL (empty for production Let's Encrypt)
	StorageDir   string // Directory to store certificates (default: /var/lib/agentsmesh/acme)
	Staging      bool   // Use Let's Encrypt staging environment
}

// IsEnabled returns true if DNS management is configured
func (c RelayConfig) IsEnabled() bool {
	return c.BaseDomain != "" && c.DNS.Provider != ""
}

// DNSProviderType represents supported DNS providers
type DNSProviderType string

const (
	DNSProviderCloudflare DNSProviderType = "cloudflare"
	DNSProviderAliyun     DNSProviderType = "aliyun"
)

// StorageConfig holds object storage configuration (S3-compatible)
type StorageConfig struct {
	Endpoint       string   // S3 endpoint (empty for AWS, set for MinIO/OSS)
	PublicEndpoint string   // Public endpoint for browser access (if different from Endpoint)
	Region         string   // AWS region or equivalent
	Bucket         string   // Bucket name
	AccessKey      string   // Access key ID
	SecretKey      string   // Secret access key
	UseSSL         bool     // Use HTTPS
	UsePathStyle   bool     // Use path-style URLs (required for MinIO)
	MaxFileSize    int64    // Max file size in MB
	AllowedTypes   []string // Allowed MIME types
}

// EmailConfig holds email service configuration
// Note: BaseURL is derived from Config.PrimaryDomain
type EmailConfig struct {
	Provider    string // "resend" or "console"
	ResendKey   string
	FromAddress string
}

// LogConfig holds logging configuration
type LogConfig struct {
	Level      string // debug, info, warn, error
	Format     string // json, text
	FilePath   string // path to log file, empty means stdout only
	MaxSizeMB  int    // max size in MB before rotation
	MaxBackups int    // max number of backup files
}

// WebhookConfig holds webhook secret configurations
type WebhookConfig struct {
	GitLabSecret string
	GitHubSecret string
	GiteeSecret  string
}

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

// OAuthConfig holds OAuth provider configurations
// Note: RedirectURLs are derived from Config.PrimaryDomain
type OAuthConfig struct {
	DefaultRedirectURL string // Redirect path after OAuth (e.g., "/")
	GitHub             OAuthProviderConfig
	Google             OAuthProviderConfig
	GitLab             GitLabOAuthConfig
	Gitee              OAuthProviderConfig
}

// OAuthProviderConfig holds OAuth provider configuration
// Note: RedirectURL is derived from Config.PrimaryDomain, not stored here
type OAuthProviderConfig struct {
	ClientID     string
	ClientSecret string
}

// GitLabOAuthConfig holds GitLab OAuth provider configuration
// Note: RedirectURL is derived from Config.PrimaryDomain
type GitLabOAuthConfig struct {
	ClientID     string
	ClientSecret string
	BaseURL      string // GitLab server base URL (default: https://gitlab.com)
}

// Load loads configuration from environment variables
// All URLs are derived from PRIMARY_DOMAIN and USE_HTTPS
func Load() (*Config, error) {
	return &Config{
		// =============================================================================
		// Unified Domain Configuration - Single source of truth for all URLs
		// =============================================================================
		PrimaryDomain: getEnv("PRIMARY_DOMAIN", "localhost:10000"),
		UseHTTPS:      getEnvBool("USE_HTTPS", false),

		// =============================================================================
		// Server Configuration
		// =============================================================================
		Server: ServerConfig{
			Address:            getEnv("SERVER_ADDRESS", ":8080"),
			Debug:              getEnvBool("DEBUG", false),
			CORSAllowedOrigins: getEnvList("CORS_ALLOWED_ORIGINS", []string{"*"}),
			InternalAPISecret:  getEnv("INTERNAL_API_SECRET", "change-me-internal-secret"),
		},

		// =============================================================================
		// Database Configuration
		// =============================================================================
		Database: DatabaseConfig{
			Host:        getEnv("DB_HOST", "localhost"),
			Port:        getEnvInt("DB_PORT", 5432),
			User:        getEnv("DB_USER", "agentsmesh"),
			Password:    getEnv("DB_PASSWORD", ""),
			DBName:      getEnv("DB_NAME", "agentsmesh"),
			SSLMode:     getEnv("DB_SSLMODE", "disable"),
			ReplicaDSNs: getEnvList("DB_REPLICA_DSNS", nil),
		},

		// =============================================================================
		// Redis Configuration
		// =============================================================================
		Redis: RedisConfig{
			URL:      getEnv("REDIS_URL", ""),
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnvInt("REDIS_PORT", 6379),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnvInt("REDIS_DB", 0),
		},

		// =============================================================================
		// JWT Configuration
		// =============================================================================
		JWT: JWTConfig{
			Secret:          getEnv("JWT_SECRET", "change-me-in-production"),
			ExpirationHours: getEnvInt("JWT_EXPIRATION_HOURS", 24),
		},

		// =============================================================================
		// OAuth Configuration (RedirectURLs derived from PrimaryDomain)
		// =============================================================================
		OAuth: OAuthConfig{
			DefaultRedirectURL: getEnv("OAUTH_DEFAULT_REDIRECT_URL", "/"),
			GitHub: OAuthProviderConfig{
				ClientID:     getEnv("GITHUB_CLIENT_ID", ""),
				ClientSecret: getEnv("GITHUB_CLIENT_SECRET", ""),
			},
			Google: OAuthProviderConfig{
				ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
				ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
			},
			GitLab: GitLabOAuthConfig{
				ClientID:     getEnv("GITLAB_CLIENT_ID", ""),
				ClientSecret: getEnv("GITLAB_CLIENT_SECRET", ""),
				BaseURL:      getEnv("GITLAB_BASE_URL", "https://gitlab.com"),
			},
			Gitee: OAuthProviderConfig{
				ClientID:     getEnv("GITEE_CLIENT_ID", ""),
				ClientSecret: getEnv("GITEE_CLIENT_SECRET", ""),
			},
		},

		// =============================================================================
		// Webhook Configuration
		// =============================================================================
		Webhook: WebhookConfig{
			GitLabSecret: getEnv("GITLAB_WEBHOOK_SECRET", ""),
			GitHubSecret: getEnv("GITHUB_WEBHOOK_SECRET", ""),
			GiteeSecret:  getEnv("GITEE_WEBHOOK_SECRET", ""),
		},

		// =============================================================================
		// Logging Configuration
		// =============================================================================
		Log: LogConfig{
			Level:      getEnv("LOG_LEVEL", "info"),
			Format:     getEnv("LOG_FORMAT", "text"),
			FilePath:   getEnv("LOG_FILE", "logs/agentsmesh.log"),
			MaxSizeMB:  getEnvInt("LOG_MAX_SIZE_MB", 100),
			MaxBackups: getEnvInt("LOG_MAX_BACKUPS", 5),
		},

		// =============================================================================
		// Email Configuration (BaseURL derived from PrimaryDomain)
		// =============================================================================
		Email: EmailConfig{
			Provider:    getEnv("EMAIL_PROVIDER", "console"),
			ResendKey:   getEnv("RESEND_API_KEY", ""),
			FromAddress: getEnv("EMAIL_FROM_ADDRESS", "AgentsMesh <noreply@agentsmesh.ai>"),
		},

		// =============================================================================
		// Storage Configuration (S3/MinIO)
		// =============================================================================
		Storage: StorageConfig{
			Endpoint:       getEnv("STORAGE_ENDPOINT", ""),
			PublicEndpoint: getEnv("STORAGE_PUBLIC_ENDPOINT", ""),
			Region:         getEnv("STORAGE_REGION", "us-east-1"),
			Bucket:         getEnv("STORAGE_BUCKET", "agentsmesh"),
			AccessKey:      getEnv("STORAGE_ACCESS_KEY", ""),
			SecretKey:      getEnv("STORAGE_SECRET_KEY", ""),
			UseSSL:         getEnvBool("STORAGE_USE_SSL", true),
			UsePathStyle:   getEnvBool("STORAGE_USE_PATH_STYLE", false),
			MaxFileSize:    int64(getEnvInt("STORAGE_MAX_FILE_SIZE", 10)),
			AllowedTypes:   getEnvList("STORAGE_ALLOWED_TYPES", []string{"image/jpeg", "image/png", "image/gif", "image/webp", "application/pdf"}),
		},

		// =============================================================================
		// Payment Configuration (NotifyURLs derived from PrimaryDomain)
		// =============================================================================
		Payment: PaymentConfig{
			DeploymentType: DeploymentType(getEnv("DEPLOYMENT_TYPE", "global")),
			MockEnabled:    getEnvBool("PAYMENT_MOCK", false),
			MockBaseURL:    getEnv("PAYMENT_MOCK_BASE_URL", ""),
			Stripe: StripeConfig{
				SecretKey:      getEnv("STRIPE_SECRET_KEY", ""),
				PublishableKey: getEnv("STRIPE_PUBLISHABLE_KEY", ""),
				WebhookSecret:  getEnv("STRIPE_WEBHOOK_SECRET", ""),
			},
			LemonSqueezy: LemonSqueezyConfig{
				APIKey:        getEnv("LEMONSQUEEZY_API_KEY", ""),
				StoreID:       getEnv("LEMONSQUEEZY_STORE_ID", ""),
				WebhookSecret: getEnv("LEMONSQUEEZY_WEBHOOK_SECRET", ""),
			},
			Alipay: AlipayConfig{
				AppID:           getEnv("ALIPAY_APP_ID", ""),
				PrivateKey:      getEnv("ALIPAY_PRIVATE_KEY", ""),
				AlipayPublicKey: getEnv("ALIPAY_PUBLIC_KEY", ""),
				IsSandbox:       getEnvBool("ALIPAY_SANDBOX", false),
			},
			WeChat: WeChatConfig{
				AppID:     getEnv("WECHAT_APP_ID", ""),
				MchID:     getEnv("WECHAT_MCH_ID", ""),
				APIKey:    getEnv("WECHAT_API_KEY", ""),
				APIv3Key:  getEnv("WECHAT_APIV3_KEY", ""),
				CertPath:  getEnv("WECHAT_CERT_PATH", ""),
				KeyPath:   getEnv("WECHAT_KEY_PATH", ""),
				IsSandbox: getEnvBool("WECHAT_SANDBOX", false),
			},
			License: LicenseConfig{
				PublicKeyPath:    getEnv("LICENSE_PUBLIC_KEY_PATH", ""),
				LicenseFilePath:  getEnv("LICENSE_FILE_PATH", ""),
				LicenseServerURL: getEnv("LICENSE_SERVER_URL", ""),
			},
		},

		// =============================================================================
		// PKI Configuration (gRPC + mTLS)
		// =============================================================================
		PKI: PKIConfig{
			CACertFile:     getEnv("PKI_CA_CERT_FILE", ""),
			CAKeyFile:      getEnv("PKI_CA_KEY_FILE", ""),
			ServerCertFile: getEnv("PKI_SERVER_CERT_FILE", ""),
			ServerKeyFile:  getEnv("PKI_SERVER_KEY_FILE", ""),
			ValidityDays:   getEnvInt("PKI_VALIDITY_DAYS", 365),
		},

		// =============================================================================
		// gRPC Configuration
		// =============================================================================
		GRPC: GRPCConfig{
			Address:  getEnv("GRPC_ADDRESS", ":9090"),
			Endpoint: getEnv("GRPC_PUBLIC_ENDPOINT", ""), // Public gRPC endpoint for Runners
		},

		// =============================================================================
		// Admin Configuration (FrontendURL derived from PrimaryDomain + "/admin")
		// =============================================================================
		Admin: AdminConfig{
			Enabled: getEnvBool("ADMIN_ENABLED", true),
		},

		// =============================================================================
		// Relay Management Configuration (for multi-relay deployment)
		// =============================================================================
		Relay: RelayConfig{
			BaseDomain: getEnv("RELAY_BASE_DOMAIN", ""), // e.g., "relay.agentsmesh.cn"
			UseHTTPS:   getEnvBool("RELAY_USE_HTTPS", true),
			DNS: DNSConfig{
				Provider:              getEnv("DNS_PROVIDER", ""),
				CloudflareAPIToken:    getEnv("CLOUDFLARE_API_TOKEN", ""),
				CloudflareZoneID:      getEnv("CLOUDFLARE_ZONE_ID", ""),
				AliyunAccessKeyID:     getEnv("ALIYUN_ACCESS_KEY_ID", ""),
				AliyunAccessKeySecret: getEnv("ALIYUN_ACCESS_KEY_SECRET", ""),
			},
			ACME: ACMEConfig{
				Enabled:      getEnvBool("ACME_ENABLED", false),
				Email:        getEnv("ACME_EMAIL", ""),
				DirectoryURL: getEnv("ACME_DIRECTORY_URL", ""),
				StorageDir:   getEnv("ACME_STORAGE_DIR", "/var/lib/agentsmesh/acme"),
				Staging:      getEnvBool("ACME_STAGING", false),
			},
		},
	}, nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolVal, err := strconv.ParseBool(value); err == nil {
			return boolVal
		}
	}
	return defaultValue
}

func getEnvList(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		// Split by comma
		parts := []string{}
		for _, part := range splitAndTrim(value, ",") {
			if part != "" {
				parts = append(parts, part)
			}
		}
		if len(parts) > 0 {
			return parts
		}
	}
	return defaultValue
}

func splitAndTrim(s, sep string) []string {
	var result []string
	for _, part := range strings.Split(s, sep) {
		trimmed := strings.TrimSpace(part)
		result = append(result, trimmed)
	}
	return result
}
