package middleware

import (
	"github.com/gofiber/fiber/v2"
)

// SecurityHeadersConfig holds configuration for security headers middleware
type SecurityHeadersConfig struct {
	// IsProduction enables production-only headers like HSTS
	IsProduction bool
}

// SecurityHeaders returns a middleware that sets security-related HTTP headers
func SecurityHeaders(cfg SecurityHeadersConfig) fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Set("X-Content-Type-Options", "nosniff")
		c.Set("X-Frame-Options", "DENY")
		c.Set("X-XSS-Protection", "1; mode=block")
		c.Set("Content-Security-Policy", "default-src 'self'")
		c.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		c.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")

		if cfg.IsProduction {
			c.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}

		return c.Next()
	}
}
