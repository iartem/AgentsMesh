package v1

import (
	"net/http"

	"github.com/anthropics/agentsmesh/backend/internal/service/auth"
	"github.com/gin-gonic/gin"
)

// LoginRequest represents a login request
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login handles email/password login
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Authenticate user
	result, err := h.authService.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if err == auth.ErrInvalidCredentials {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
			return
		}
		if err == auth.ErrUserDisabled {
			c.JSON(http.StatusForbidden, gin.H{"error": "Account is disabled"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Authentication failed"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":         result.Token,
		"refresh_token": result.RefreshToken,
		"expires_in":    result.ExpiresIn,
		"user": gin.H{
			"id":         result.User.ID,
			"email":      result.User.Email,
			"username":   result.User.Username,
			"name":       result.User.Name,
			"avatar_url": result.User.AvatarURL,
		},
	})
}

// RegisterRequest represents a registration request
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=8"`
	Name     string `json:"name"`
}

// Register handles user registration
func (h *AuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Register user
	result, err := h.authService.Register(c.Request.Context(), &auth.RegisterRequest{
		Email:    req.Email,
		Username: req.Username,
		Password: req.Password,
		Name:     req.Name,
	})
	if err != nil {
		if err == auth.ErrEmailExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
			return
		}
		if err == auth.ErrUsernameExists {
			c.JSON(http.StatusConflict, gin.H{"error": "Username already taken"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Registration failed"})
		return
	}

	// Generate email verification token and send verification email
	verificationToken, err := h.userService.SetEmailVerificationToken(c.Request.Context(), result.User.ID)
	if err != nil {
		// Log error but don't fail registration
		c.JSON(http.StatusCreated, gin.H{
			"token":         result.Token,
			"refresh_token": result.RefreshToken,
			"expires_in":    result.ExpiresIn,
			"user": gin.H{
				"id":                result.User.ID,
				"email":             result.User.Email,
				"username":          result.User.Username,
				"name":              result.User.Name,
				"is_email_verified": false,
			},
			"message": "Registration successful. Please verify your email.",
		})
		return
	}

	// Send verification email (don't fail registration if email fails)
	_ = h.emailService.SendVerificationEmail(c.Request.Context(), result.User.Email, verificationToken)

	c.JSON(http.StatusCreated, gin.H{
		"token":         result.Token,
		"refresh_token": result.RefreshToken,
		"expires_in":    result.ExpiresIn,
		"user": gin.H{
			"id":                result.User.ID,
			"email":             result.User.Email,
			"username":          result.User.Username,
			"name":              result.User.Name,
			"is_email_verified": false,
		},
		"message": "Registration successful. Please check your email to verify your account.",
	})
}

// RefreshToken handles token refresh
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	var req struct {
		RefreshToken string `json:"refresh_token" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := h.authService.RefreshToken(c.Request.Context(), req.RefreshToken)
	if err != nil {
		if err == auth.ErrInvalidToken || err == auth.ErrInvalidRefreshToken {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid refresh token"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to refresh token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":         result.Token,
		"refresh_token": result.RefreshToken,
		"expires_in":    result.ExpiresIn,
	})
}

// Logout handles user logout
func (h *AuthHandler) Logout(c *gin.Context) {
	// Get token from header
	token := c.GetHeader("Authorization")
	if token != "" && len(token) > 7 {
		token = token[7:] // Remove "Bearer " prefix
		// Optionally blacklist the token
		h.authService.RevokeToken(c.Request.Context(), token)
	}

	c.JSON(http.StatusOK, gin.H{"message": "Logged out successfully"})
}
