package config

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
