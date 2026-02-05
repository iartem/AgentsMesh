package v1

// RequestAuthURLRequest represents request for authorization URL.
type RequestAuthURLRequest struct {
	MachineKey string            `json:"machine_key" binding:"required"`
	NodeID     string            `json:"node_id"`
	Labels     map[string]string `json:"labels"`
}

// AuthorizeRunnerRequest represents authorization request from Web UI.
type AuthorizeRunnerRequest struct {
	AuthKey string `json:"auth_key" binding:"required"`
	NodeID  string `json:"node_id"`
}

// GenerateGRPCTokenRequest represents request to generate registration token.
type GenerateGRPCTokenRequest struct {
	Name      string            `json:"name"`
	Labels    map[string]string `json:"labels"`
	SingleUse bool              `json:"single_use"`
	MaxUses   int               `json:"max_uses"`
	ExpiresIn int               `json:"expires_in"` // seconds
}

// RegisterWithTokenRequest represents request to register with pre-generated token.
type RegisterWithTokenRequest struct {
	Token  string `json:"token" binding:"required"`
	NodeID string `json:"node_id"`
}

// ReactivateRequest represents request to reactivate a runner.
type ReactivateRequest struct {
	Token string `json:"token" binding:"required"`
}
