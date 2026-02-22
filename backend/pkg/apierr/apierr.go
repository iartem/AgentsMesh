package apierr

import (
	"github.com/gin-gonic/gin"
)

// ErrorResponse is the standard error response structure.
type ErrorResponse struct {
	Error string `json:"error"`
	Code  string `json:"code"`
}

// Respond sends a structured error response with a given HTTP status, code, and message.
func Respond(c *gin.Context, status int, code, message string) {
	c.JSON(status, ErrorResponse{
		Error: message,
		Code:  code,
	})
}

// RespondWithExtra sends a structured error response with additional fields merged in.
func RespondWithExtra(c *gin.Context, status int, code, message string, extra gin.H) {
	resp := gin.H{
		"error": message,
		"code":  code,
	}
	for k, v := range extra {
		resp[k] = v
	}
	c.JSON(status, resp)
}
