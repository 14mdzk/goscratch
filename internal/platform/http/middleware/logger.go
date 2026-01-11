package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/14mdzk/goscratch/pkg/logger"
)

// Logger returns a middleware that logs HTTP requests
func Logger(log *logger.Logger) fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process request
		err := c.Next()

		// Calculate latency
		latency := time.Since(start)

		// Build log fields
		fields := map[string]any{
			"method":     c.Method(),
			"path":       c.Path(),
			"status":     c.Response().StatusCode(),
			"latency":    latency.String(),
			"latency_ms": latency.Milliseconds(),
			"ip":         c.IP(),
			"user_agent": c.Get("User-Agent"),
		}

		// Add request ID if present
		if requestID := GetRequestID(c); requestID != "" {
			fields["request_id"] = requestID
		}

		// Add user ID if present
		if userID, ok := c.Locals("user_id").(string); ok && userID != "" {
			fields["user_id"] = userID
		}

		// Add error if present
		if err != nil {
			fields["error"] = err.Error()
		}

		// Log based on status code
		status := c.Response().StatusCode()
		logEntry := log.WithFields(fields)

		switch {
		case status >= 500:
			logEntry.Error("Server error")
		case status >= 400:
			logEntry.Warn("Client error")
		default:
			logEntry.Info("Request completed")
		}

		return err
	}
}
