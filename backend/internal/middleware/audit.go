package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AuditLog represents an audit log entry
type AuditLog struct {
	ID             int64           `gorm:"primaryKey" json:"id"`
	OrganizationID *int64          `gorm:"index" json:"organization_id,omitempty"`
	ActorID        *int64          `gorm:"index" json:"actor_id,omitempty"`
	ActorType      string          `gorm:"size:50;not null" json:"actor_type"` // user, system, runner
	Action         string          `gorm:"size:100;not null;index" json:"action"`
	ResourceType   string          `gorm:"size:50;not null" json:"resource_type"`
	ResourceID     *int64          `json:"resource_id,omitempty"`
	Details        json.RawMessage `gorm:"type:jsonb" json:"details,omitempty"`
	IPAddress      *string         `gorm:"type:inet" json:"ip_address,omitempty"`
	UserAgent      *string         `gorm:"type:text" json:"user_agent,omitempty"`
	StatusCode     int             `json:"status_code"`
	Duration       int64           `json:"duration_ms"`
	CreatedAt      time.Time       `gorm:"not null;default:now();index" json:"created_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}

// AuditConfig holds configuration for the audit middleware
type AuditConfig struct {
	DB             *gorm.DB
	SkipPaths      []string
	SkipMethods    []string
	CaptureBody    bool
	MaxBodySize    int64
	SensitiveFields []string
}

// DefaultAuditConfig returns a default audit configuration
func DefaultAuditConfig(db *gorm.DB) *AuditConfig {
	return &AuditConfig{
		DB:          db,
		SkipPaths:   []string{"/health", "/metrics", "/api/v1/ws"},
		SkipMethods: []string{"GET", "HEAD", "OPTIONS"},
		CaptureBody: true,
		MaxBodySize: 10 * 1024, // 10KB
		SensitiveFields: []string{
			"password", "token", "secret", "api_key", "access_token",
			"refresh_token", "client_secret", "private_key",
		},
	}
}

// AuditMiddleware creates an audit logging middleware
func AuditMiddleware(config *AuditConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Skip certain paths
		for _, path := range config.SkipPaths {
			if strings.HasPrefix(c.Request.URL.Path, path) {
				c.Next()
				return
			}
		}

		// Skip certain methods (usually read operations)
		for _, method := range config.SkipMethods {
			if c.Request.Method == method {
				c.Next()
				return
			}
		}

		startTime := time.Now()

		// Capture request body if enabled
		var requestBody []byte
		if config.CaptureBody && c.Request.Body != nil {
			requestBody, _ = io.ReadAll(io.LimitReader(c.Request.Body, config.MaxBodySize))
			c.Request.Body = io.NopCloser(bytes.NewBuffer(requestBody))
		}

		// Use response writer to capture status code
		rw := &responseWriter{ResponseWriter: c.Writer, statusCode: http.StatusOK}
		c.Writer = rw

		// Process request
		c.Next()

		// Calculate duration
		duration := time.Since(startTime).Milliseconds()

		// Build audit log entry
		log := buildAuditLog(c, config, requestBody, rw.statusCode, duration)
		if log != nil {
			// Save asynchronously to avoid blocking the response
			go func() {
				if err := config.DB.Create(log).Error; err != nil {
					// Log error but don't fail the request
					// In production, use a proper logger
				}
			}()
		}
	}
}

// responseWriter wraps gin.ResponseWriter to capture status code
type responseWriter struct {
	gin.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// buildAuditLog creates an audit log entry from the request
func buildAuditLog(c *gin.Context, config *AuditConfig, body []byte, statusCode int, duration int64) *AuditLog {
	// Parse action from path and method
	action, resourceType, resourceID := parseAction(c.Request.Method, c.Request.URL.Path)
	if action == "" {
		return nil
	}

	// Get tenant context
	tc := GetTenant(c.Request.Context())

	log := &AuditLog{
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		ActorType:    "user",
		StatusCode:   statusCode,
		Duration:     duration,
		CreatedAt:    time.Now(),
	}

	// Set organization ID if available
	if tc != nil && tc.OrganizationID > 0 {
		log.OrganizationID = &tc.OrganizationID
		log.ActorID = &tc.UserID
	}

	// Set IP address
	ip := c.ClientIP()
	if ip != "" {
		log.IPAddress = &ip
	}

	// Set user agent
	ua := c.Request.UserAgent()
	if ua != "" {
		log.UserAgent = &ua
	}

	// Build details
	details := buildDetails(c, config, body)
	if len(details) > 0 {
		detailsJSON, _ := json.Marshal(details)
		log.Details = detailsJSON
	}

	return log
}

// parseAction extracts action, resource type, and resource ID from path
func parseAction(method, path string) (action string, resourceType string, resourceID *int64) {
	// Remove /api/v1 prefix
	path = strings.TrimPrefix(path, "/api/v1")
	path = strings.TrimPrefix(path, "/api")

	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return "", "", nil
	}

	resourceType = parts[0]

	// Map HTTP methods to actions
	switch method {
	case "POST":
		action = resourceType + ".created"
	case "PUT", "PATCH":
		action = resourceType + ".updated"
	case "DELETE":
		action = resourceType + ".deleted"
	default:
		return "", "", nil
	}

	// Extract resource ID if present
	if len(parts) > 1 {
		// Try to parse as integer
		var id int64
		if _, err := json.Number(parts[1]).Int64(); err == nil {
			id, _ = json.Number(parts[1]).Int64()
			resourceID = &id
		}
	}

	// Handle special cases
	switch {
	case strings.Contains(path, "/terminate"):
		action = "sessions.terminated"
	case strings.Contains(path, "/archive"):
		action = "channels.archived"
	case strings.Contains(path, "/unarchive"):
		action = "channels.unarchived"
	case strings.Contains(path, "/join"):
		action = "channels.joined"
	case strings.Contains(path, "/leave"):
		action = "channels.left"
	case strings.Contains(path, "/register"):
		action = "users.registered"
		resourceType = "users"
	case strings.Contains(path, "/login"):
		action = "users.logged_in"
		resourceType = "users"
	case strings.Contains(path, "/oauth"):
		action = "users.oauth_login"
		resourceType = "users"
	}

	return action, resourceType, resourceID
}

// buildDetails creates the details object for the audit log
func buildDetails(c *gin.Context, config *AuditConfig, body []byte) map[string]interface{} {
	details := make(map[string]interface{})

	// Add query parameters
	if len(c.Request.URL.Query()) > 0 {
		query := make(map[string]string)
		for key, values := range c.Request.URL.Query() {
			if len(values) > 0 {
				query[key] = values[0]
			}
		}
		details["query"] = query
	}

	// Add sanitized request body
	if len(body) > 0 && config.CaptureBody {
		var bodyData map[string]interface{}
		if err := json.Unmarshal(body, &bodyData); err == nil {
			sanitizedBody := sanitizeBody(bodyData, config.SensitiveFields)
			details["body"] = sanitizedBody
		}
	}

	return details
}

// sanitizeBody removes sensitive fields from the request body
func sanitizeBody(body map[string]interface{}, sensitiveFields []string) map[string]interface{} {
	result := make(map[string]interface{})
	for key, value := range body {
		// Check if this is a sensitive field
		isSensitive := false
		keyLower := strings.ToLower(key)
		for _, field := range sensitiveFields {
			if strings.Contains(keyLower, field) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			result[key] = "[REDACTED]"
		} else if nested, ok := value.(map[string]interface{}); ok {
			result[key] = sanitizeBody(nested, sensitiveFields)
		} else {
			result[key] = value
		}
	}
	return result
}

// AuditAction is a helper type for logging specific actions
type AuditAction string

const (
	// User actions
	AuditUserCreated     AuditAction = "users.created"
	AuditUserUpdated     AuditAction = "users.updated"
	AuditUserDeleted     AuditAction = "users.deleted"
	AuditUserLoggedIn    AuditAction = "users.logged_in"
	AuditUserLoggedOut   AuditAction = "users.logged_out"
	AuditUserRegistered  AuditAction = "users.registered"

	// Organization actions
	AuditOrgCreated      AuditAction = "organizations.created"
	AuditOrgUpdated      AuditAction = "organizations.updated"
	AuditOrgDeleted      AuditAction = "organizations.deleted"
	AuditOrgMemberAdded  AuditAction = "organizations.member_added"
	AuditOrgMemberRemoved AuditAction = "organizations.member_removed"

	// Team actions
	AuditTeamCreated      AuditAction = "teams.created"
	AuditTeamUpdated      AuditAction = "teams.updated"
	AuditTeamDeleted      AuditAction = "teams.deleted"
	AuditTeamMemberAdded  AuditAction = "teams.member_added"
	AuditTeamMemberRemoved AuditAction = "teams.member_removed"

	// Runner actions
	AuditRunnerRegistered AuditAction = "runners.registered"
	AuditRunnerDeleted    AuditAction = "runners.deleted"
	AuditRunnerOnline     AuditAction = "runners.online"
	AuditRunnerOffline    AuditAction = "runners.offline"

	// Session actions
	AuditSessionCreated   AuditAction = "sessions.created"
	AuditSessionStarted   AuditAction = "sessions.started"
	AuditSessionTerminated AuditAction = "sessions.terminated"
	AuditSessionFailed    AuditAction = "sessions.failed"

	// Channel actions
	AuditChannelCreated  AuditAction = "channels.created"
	AuditChannelArchived AuditAction = "channels.archived"
	AuditChannelJoined   AuditAction = "channels.joined"
	AuditChannelLeft     AuditAction = "channels.left"

	// Ticket actions
	AuditTicketCreated     AuditAction = "tickets.created"
	AuditTicketUpdated     AuditAction = "tickets.updated"
	AuditTicketDeleted     AuditAction = "tickets.deleted"
	AuditTicketStatusChanged AuditAction = "tickets.status_changed"

	// Git Provider actions
	AuditGitProviderCreated AuditAction = "git_providers.created"
	AuditGitProviderUpdated AuditAction = "git_providers.updated"
	AuditGitProviderDeleted AuditAction = "git_providers.deleted"

	// Repository actions
	AuditRepositoryCreated AuditAction = "repositories.created"
	AuditRepositoryDeleted AuditAction = "repositories.deleted"

	// Billing actions
	AuditSubscriptionCreated AuditAction = "subscriptions.created"
	AuditSubscriptionUpdated AuditAction = "subscriptions.updated"
	AuditSubscriptionCanceled AuditAction = "subscriptions.canceled"
)

// LogAction logs a specific action programmatically
func LogAction(db *gorm.DB, action AuditAction, opts *LogActionOptions) error {
	log := &AuditLog{
		Action:       string(action),
		ResourceType: opts.ResourceType,
		ActorType:    opts.ActorType,
		StatusCode:   opts.StatusCode,
		CreatedAt:    time.Now(),
	}

	if opts.OrganizationID > 0 {
		log.OrganizationID = &opts.OrganizationID
	}
	if opts.ActorID > 0 {
		log.ActorID = &opts.ActorID
	}
	if opts.ResourceID > 0 {
		log.ResourceID = &opts.ResourceID
	}
	if opts.IPAddress != "" {
		log.IPAddress = &opts.IPAddress
	}
	if opts.UserAgent != "" {
		log.UserAgent = &opts.UserAgent
	}
	if opts.Details != nil {
		detailsJSON, _ := json.Marshal(opts.Details)
		log.Details = detailsJSON
	}

	return db.Create(log).Error
}

// LogActionOptions holds options for logging an action
type LogActionOptions struct {
	OrganizationID int64
	ActorID        int64
	ActorType      string // user, system, runner
	ResourceType   string
	ResourceID     int64
	StatusCode     int
	Details        map[string]interface{}
	IPAddress      string
	UserAgent      string
}

// QueryAuditLogs queries audit logs with filters
func QueryAuditLogs(db *gorm.DB, filter *AuditLogFilter) ([]AuditLog, int64, error) {
	var logs []AuditLog
	var total int64

	query := db.Model(&AuditLog{})

	if filter.OrganizationID > 0 {
		query = query.Where("organization_id = ?", filter.OrganizationID)
	}
	if filter.ActorID > 0 {
		query = query.Where("actor_id = ?", filter.ActorID)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.ResourceType != "" {
		query = query.Where("resource_type = ?", filter.ResourceType)
	}
	if filter.ResourceID > 0 {
		query = query.Where("resource_id = ?", filter.ResourceID)
	}
	if !filter.StartTime.IsZero() {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if !filter.EndTime.IsZero() {
		query = query.Where("created_at <= ?", filter.EndTime)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Apply pagination
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	// Order by created_at desc
	query = query.Order("created_at DESC")

	if err := query.Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// AuditLogFilter holds filters for querying audit logs
type AuditLogFilter struct {
	OrganizationID int64
	ActorID        int64
	Action         string
	ResourceType   string
	ResourceID     int64
	StartTime      time.Time
	EndTime        time.Time
	Limit          int
	Offset         int
}
