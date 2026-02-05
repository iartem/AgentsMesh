package v1

import (
	"net/http"
	"net/url"

	"github.com/anthropics/agentsmesh/backend/internal/service/auth"
	"github.com/gin-gonic/gin"
)

// OAuthRedirect returns a handler for OAuth redirect
func (h *AuthHandler) OAuthRedirect(provider string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get redirect URL from query params (for post-auth redirect)
		redirectTo := c.Query("redirect")
		if redirectTo == "" {
			redirectTo = h.config.OAuth.DefaultRedirectURL
		}

		// Get OAuth provider configuration
		oauthCfg := h.getOAuthConfig(provider)
		if oauthCfg == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "OAuth provider not configured"})
			return
		}

		// Generate state with redirect info
		state, err := h.authService.GenerateOAuthState(c.Request.Context(), provider, redirectTo)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate OAuth state"})
			return
		}

		// Build authorization URL
		authURL := oauthCfg.AuthURL(state)
		c.Redirect(http.StatusTemporaryRedirect, authURL)
	}
}

// OAuthCallback returns a handler for OAuth callback
func (h *AuthHandler) OAuthCallback(provider string) gin.HandlerFunc {
	return func(c *gin.Context) {
		code := c.Query("code")
		state := c.Query("state")

		if code == "" {
			errorMsg := c.Query("error")
			errorDesc := c.Query("error_description")
			c.JSON(http.StatusBadRequest, gin.H{
				"error":       errorMsg,
				"description": errorDesc,
			})
			return
		}

		// Validate state and get redirect URL
		redirectTo, err := h.authService.ValidateOAuthState(c.Request.Context(), state)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid or expired state"})
			return
		}

		// Get OAuth provider configuration
		oauthCfg := h.getOAuthConfig(provider)
		if oauthCfg == nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "OAuth provider not configured"})
			return
		}

		// Exchange code for token
		token, err := oauthCfg.Exchange(c.Request.Context(), code)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange OAuth code"})
			return
		}

		// Get user info from provider
		userInfo, err := oauthCfg.GetUserInfo(c.Request.Context(), token.AccessToken)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info"})
			return
		}

		// Authenticate or create user
		result, err := h.authService.OAuthLogin(c.Request.Context(), &auth.OAuthLoginRequest{
			Provider:       provider,
			ProviderUserID: userInfo.ID,
			Email:          userInfo.Email,
			Username:       userInfo.Username,
			Name:           userInfo.Name,
			AvatarURL:      userInfo.AvatarURL,
			AccessToken:    token.AccessToken,
			RefreshToken:   token.RefreshToken,
			ExpiresAt:      &token.ExpiresAt,
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "OAuth authentication failed"})
			return
		}

		// Redirect with token (for SPA to capture)
		redirectURL, _ := url.Parse(redirectTo)
		q := redirectURL.Query()
		q.Set("token", result.Token)
		q.Set("refresh_token", result.RefreshToken)
		redirectURL.RawQuery = q.Encode()

		c.Redirect(http.StatusTemporaryRedirect, redirectURL.String())
	}
}
