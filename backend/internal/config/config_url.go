package config

import "fmt"

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
