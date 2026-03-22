package middleware

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestRateLimit_UnderLimit(t *testing.T) {
	app := fiber.New()

	app.Use(RateLimit(RateLimitConfig{
		Max:    5,
		Window: 1 * time.Minute,
	}, nil))

	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	})

	// Make 3 requests (under limit of 5)
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		// Verify headers are set
		assert.Equal(t, "5", resp.Header.Get("X-RateLimit-Limit"))
		assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Remaining"))
		assert.NotEmpty(t, resp.Header.Get("X-RateLimit-Reset"))
	}
}

func TestRateLimit_AtLimit(t *testing.T) {
	app := fiber.New()

	app.Use(RateLimit(RateLimitConfig{
		Max:    3,
		Window: 1 * time.Minute,
	}, nil))

	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	})

	// Make requests up to and past the limit
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}

	// 4th request should be rejected
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(body, &result)

	assert.Equal(t, false, result["success"])
	assert.Equal(t, "0", resp.Header.Get("X-RateLimit-Remaining"))
}

func TestRateLimit_WindowReset(t *testing.T) {
	app := fiber.New()

	app.Use(RateLimit(RateLimitConfig{
		Max:    2,
		Window: 100 * time.Millisecond, // Very short window for testing
	}, nil))

	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	})

	// Exhaust the limit
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Should be allowed again
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRateLimit_CustomKeyFunc(t *testing.T) {
	app := fiber.New()

	app.Use(RateLimit(RateLimitConfig{
		Max:    2,
		Window: 1 * time.Minute,
		KeyFunc: func(c *fiber.Ctx) string {
			return "custom:" + c.Get("X-API-Key")
		},
	}, nil))

	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	})

	// Two requests with same key
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-API-Key", "key-a")
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}

	// Third request with same key should be blocked
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "key-a")
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	// Request with different key should still be allowed
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-API-Key", "key-b")
	resp, err = app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRateLimit_DifferentClients(t *testing.T) {
	app := fiber.New()

	app.Use(RateLimit(RateLimitConfig{
		Max:    2,
		Window: 1 * time.Minute,
		KeyFunc: func(c *fiber.Ctx) string {
			return "client:" + c.Get("X-Client-ID")
		},
	}, nil))

	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	})

	// Client A makes 2 requests (reaches limit)
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Client-ID", "client-a")
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}

	// Client A should be blocked
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("X-Client-ID", "client-a")
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusTooManyRequests, resp.StatusCode)

	// Client B should still be allowed
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		req.Header.Set("X-Client-ID", "client-b")
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	}
}

func TestRateLimit_DefaultConfig(t *testing.T) {
	app := fiber.New()

	// Use zero values to test defaults
	app.Use(RateLimit(RateLimitConfig{}, nil))

	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "100", resp.Header.Get("X-RateLimit-Limit"))
}

func TestRateLimit_RemainingHeaderDecreases(t *testing.T) {
	app := fiber.New()

	app.Use(RateLimit(RateLimitConfig{
		Max:    5,
		Window: 1 * time.Minute,
	}, nil))

	app.Get("/test", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"success": true})
	})

	// First request: remaining should be 4
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, "4", resp.Header.Get("X-RateLimit-Remaining"))

	// Second request: remaining should be 3
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err = app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, "3", resp.Header.Get("X-RateLimit-Remaining"))

	// Third request: remaining should be 2
	req = httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err = app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, "2", resp.Header.Get("X-RateLimit-Remaining"))
}
