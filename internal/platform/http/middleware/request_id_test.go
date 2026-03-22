package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestID_GeneratesUniqueID(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req1 := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp1, err := app.Test(req1)
	require.NoError(t, err)
	id1 := resp1.Header.Get(RequestIDHeader)

	req2 := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp2, err := app.Test(req2)
	require.NoError(t, err)
	id2 := resp2.Header.Get(RequestIDHeader)

	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2, "each request should get a unique ID")
}

func TestRequestID_AddsToResponseHeader(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	id := resp.Header.Get(RequestIDHeader)
	assert.NotEmpty(t, id)
	assert.Len(t, id, 36, "should be a UUID format")
}

func TestRequestID_AvailableInContext(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())

	var capturedID string

	app.Get("/test", func(c *fiber.Ctx) error {
		capturedID = GetRequestID(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	assert.NotEmpty(t, capturedID)
	assert.Equal(t, resp.Header.Get(RequestIDHeader), capturedID)
}

func TestRequestID_PreservesExistingHeader(t *testing.T) {
	app := fiber.New()
	app.Use(RequestID())
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	existingID := "my-custom-request-id"
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set(RequestIDHeader, existingID)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, existingID, resp.Header.Get(RequestIDHeader))
}

func TestGetRequestID_NoMiddleware(t *testing.T) {
	app := fiber.New()

	var result string
	app.Get("/test", func(c *fiber.Ctx) error {
		result = GetRequestID(c)
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	_, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}
