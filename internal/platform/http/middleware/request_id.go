package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/14mdzk/goscratch/pkg/logger"
)

const (
	// RequestIDHeader is the header name for request ID
	RequestIDHeader = "X-Request-ID"
	// RequestIDKey is the context key for request ID
	RequestIDKey = "request_id"
)

// RequestID adds a unique request ID to each request
func RequestID() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Check if request ID already exists in headers
		requestID := c.Get(RequestIDHeader)
		if requestID == "" {
			requestID = uuid.New().String()
		}

		// Set in response header
		c.Set(RequestIDHeader, requestID)

		// Store in locals for access in handlers
		c.Locals(RequestIDKey, requestID)

		// Add to context for logger
		ctx := c.UserContext()
		ctx = setContextValue(ctx, logger.RequestIDKey, requestID)
		c.SetUserContext(ctx)

		return c.Next()
	}
}

// GetRequestID retrieves the request ID from context
func GetRequestID(c *fiber.Ctx) string {
	if id, ok := c.Locals(RequestIDKey).(string); ok {
		return id
	}
	return ""
}
