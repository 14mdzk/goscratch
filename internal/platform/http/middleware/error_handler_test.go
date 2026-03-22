package middleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLogger(buf *bytes.Buffer) *logger.Logger {
	return logger.New(logger.Config{
		Level:  "debug",
		Format: "json",
		Output: buf,
	})
}

func TestErrorHandler_AppError(t *testing.T) {
	var logBuf bytes.Buffer
	log := newTestLogger(&logBuf)

	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorHandler(log),
	})

	app.Get("/test", func(c *fiber.Ctx) error {
		return apperr.New(apperr.CodeNotFound, "user not found", http.StatusNotFound)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]any
	require.NoError(t, json.Unmarshal(body, &result))
	assert.Equal(t, false, result["success"])

	errObj := result["error"].(map[string]any)
	assert.Equal(t, apperr.CodeNotFound, errObj["code"])
	assert.Equal(t, "user not found", errObj["message"])
}

func TestErrorHandler_AppError500_LogsError(t *testing.T) {
	var logBuf bytes.Buffer
	log := newTestLogger(&logBuf)

	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorHandler(log),
	})

	app.Get("/test", func(c *fiber.Ctx) error {
		return apperr.New(apperr.CodeInternalError, "db connection failed", http.StatusInternalServerError)
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	// Verify error was logged
	assert.Contains(t, logBuf.String(), "Server error occurred")
}

func TestErrorHandler_FiberError(t *testing.T) {
	var logBuf bytes.Buffer
	log := newTestLogger(&logBuf)

	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorHandler(log),
	})

	app.Get("/test", func(c *fiber.Ctx) error {
		return fiber.NewError(fiber.StatusNotFound, "route not found")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusNotFound, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]any
	require.NoError(t, json.Unmarshal(body, &result))
	assert.Equal(t, false, result["success"])

	errObj := result["error"].(map[string]any)
	assert.Equal(t, apperr.CodeNotFound, errObj["code"])
	assert.Equal(t, "route not found", errObj["message"])
}

func TestErrorHandler_UnknownError(t *testing.T) {
	var logBuf bytes.Buffer
	log := newTestLogger(&logBuf)

	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorHandler(log),
	})

	app.Get("/test", func(c *fiber.Ctx) error {
		return errors.New("something unexpected")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	var result map[string]any
	require.NoError(t, json.Unmarshal(body, &result))
	assert.Equal(t, false, result["success"])

	errObj := result["error"].(map[string]any)
	assert.Equal(t, apperr.CodeInternalError, errObj["code"])

	// Verify the unexpected error was logged
	assert.Contains(t, logBuf.String(), "Unexpected error occurred")
}

func TestFiberStatusToCode(t *testing.T) {
	tests := []struct {
		status   int
		expected string
	}{
		{fiber.StatusBadRequest, apperr.CodeBadRequest},
		{fiber.StatusUnauthorized, apperr.CodeUnauthorized},
		{fiber.StatusForbidden, apperr.CodeForbidden},
		{fiber.StatusNotFound, apperr.CodeNotFound},
		{fiber.StatusConflict, apperr.CodeConflict},
		{fiber.StatusUnprocessableEntity, apperr.CodeUnprocessableEntity},
		{fiber.StatusInternalServerError, apperr.CodeInternalError},
		{999, apperr.CodeInternalError}, // unknown status
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, fiberStatusToCode(tt.status))
		})
	}
}
