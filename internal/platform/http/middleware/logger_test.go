package middleware

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogger_LogsRequestDetails(t *testing.T) {
	var logBuf bytes.Buffer
	log := logger.New(logger.Config{
		Level:  "debug",
		Format: "json",
		Output: &logBuf,
	})

	app := fiber.New()
	app.Use(Logger(log))
	app.Get("/hello", func(c *fiber.Ctx) error {
		return c.SendString("world")
	})

	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.Header.Set("User-Agent", "test-agent")

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	output := logBuf.String()
	assert.Contains(t, output, "GET")
	assert.Contains(t, output, "/hello")
	assert.Contains(t, output, "200")
	assert.Contains(t, output, "latency")
	assert.Contains(t, output, "Request completed")
}

func TestLogger_IncludesRequestID(t *testing.T) {
	var logBuf bytes.Buffer
	log := logger.New(logger.Config{
		Level:  "debug",
		Format: "json",
		Output: &logBuf,
	})

	app := fiber.New()
	app.Use(RequestID()) // add request ID first
	app.Use(Logger(log))
	app.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	output := logBuf.String()
	assert.Contains(t, output, "request_id")
}

func TestLogger_ServerError_LogsError(t *testing.T) {
	var logBuf bytes.Buffer
	log := logger.New(logger.Config{
		Level:  "debug",
		Format: "json",
		Output: &logBuf,
	})

	app := fiber.New()
	app.Use(Logger(log))
	app.Get("/fail", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusInternalServerError).SendString("error")
	})

	req := httptest.NewRequest(http.MethodGet, "/fail", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)

	output := logBuf.String()
	assert.Contains(t, output, "Server error")
}

func TestLogger_ClientError_LogsWarn(t *testing.T) {
	var logBuf bytes.Buffer
	log := logger.New(logger.Config{
		Level:  "debug",
		Format: "json",
		Output: &logBuf,
	})

	app := fiber.New()
	app.Use(Logger(log))
	app.Get("/notfound", func(c *fiber.Ctx) error {
		return c.Status(fiber.StatusNotFound).SendString("not found")
	})

	req := httptest.NewRequest(http.MethodGet, "/notfound", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	output := logBuf.String()
	assert.Contains(t, output, "Client error")
}
