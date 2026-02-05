package config

// DeploymentType represents the deployment environment
type DeploymentType string

const (
	DeploymentGlobal    DeploymentType = "global"    // International - Stripe
	DeploymentCN        DeploymentType = "cn"        // China - Alipay + WeChat Pay
	DeploymentOnPremise DeploymentType = "onpremise" // Self-hosted - License file
)

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
