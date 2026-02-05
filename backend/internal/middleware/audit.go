package middleware

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

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
		action = "pods.terminated"
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
