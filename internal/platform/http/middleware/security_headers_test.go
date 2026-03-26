package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityHeaders_Development(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeaders(SecurityHeadersConfig{IsProduction: false}))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", resp.Header.Get("X-XSS-Protection"))
	assert.Equal(t, "default-src 'self'", resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "strict-origin-when-cross-origin", resp.Header.Get("Referrer-Policy"))
	assert.Equal(t, "camera=(), microphone=(), geolocation=()", resp.Header.Get("Permissions-Policy"))

	// HSTS should NOT be set in development
	assert.Empty(t, resp.Header.Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_Production(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeaders(SecurityHeadersConfig{IsProduction: true}))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	assert.Equal(t, "nosniff", resp.Header.Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", resp.Header.Get("X-Frame-Options"))
	assert.Equal(t, "1; mode=block", resp.Header.Get("X-XSS-Protection"))
	assert.Equal(t, "default-src 'self'", resp.Header.Get("Content-Security-Policy"))
	assert.Equal(t, "strict-origin-when-cross-origin", resp.Header.Get("Referrer-Policy"))
	assert.Equal(t, "camera=(), microphone=(), geolocation=()", resp.Header.Get("Permissions-Policy"))

	// HSTS SHOULD be set in production
	assert.Equal(t, "max-age=31536000; includeSubDomains", resp.Header.Get("Strict-Transport-Security"))
}

func TestSecurityHeaders_PassesThrough(t *testing.T) {
	app := fiber.New()
	app.Use(SecurityHeaders(SecurityHeadersConfig{IsProduction: false}))

	handlerCalled := false
	app.Get("/test", func(c *fiber.Ctx) error {
		handlerCalled = true
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)
	assert.True(t, handlerCalled)
}
