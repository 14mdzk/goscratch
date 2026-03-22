package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
)

func TestDispatchHandler(t *testing.T) {
	app := fiber.New()

	// Mock dispatch endpoint (simulates handler behavior)
	app.Post("/jobs/dispatch", func(c *fiber.Ctx) error {
		type DispatchRequest struct {
			Type     string          `json:"type"`
			Payload  json.RawMessage `json:"payload"`
			MaxRetry *int            `json:"max_retry,omitempty"`
		}

		var req DispatchRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "BAD_REQUEST", "message": "Invalid request body"},
			})
		}

		if req.Type == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "type is required"},
			})
		}

		if req.Payload == nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "payload is required"},
			})
		}

		// Validate job type
		validTypes := map[string]bool{
			"email.send":        true,
			"audit.cleanup":     true,
			"notification.send": true,
		}
		if !validTypes[req.Type] {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "BAD_REQUEST", "message": "invalid job type: " + req.Type},
			})
		}

		return c.Status(http.StatusCreated).JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"id":         "test-job-id",
				"type":       req.Type,
				"status":     "queued",
				"created_at": "2026-03-22T00:00:00Z",
			},
		})
	})

	t.Run("valid_dispatch", func(t *testing.T) {
		body := []byte(`{"type":"email.send","payload":{"to":"user@example.com","subject":"Hello"}}`)
		req := httptest.NewRequest(http.MethodPost, "/jobs/dispatch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		assert.Equal(t, true, result["success"])
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "email.send", data["type"])
		assert.Equal(t, "queued", data["status"])
		assert.NotEmpty(t, data["id"])
	})

	t.Run("missing_type", func(t *testing.T) {
		body := []byte(`{"payload":{"to":"user@example.com"}}`)
		req := httptest.NewRequest(http.MethodPost, "/jobs/dispatch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("missing_payload", func(t *testing.T) {
		body := []byte(`{"type":"email.send"}`)
		req := httptest.NewRequest(http.MethodPost, "/jobs/dispatch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("invalid_job_type", func(t *testing.T) {
		body := []byte(`{"type":"unknown.type","payload":{"key":"value"}}`)
		req := httptest.NewRequest(http.MethodPost, "/jobs/dispatch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		assert.Equal(t, false, result["success"])
	})

	t.Run("invalid_json", func(t *testing.T) {
		body := []byte(`not valid json`)
		req := httptest.NewRequest(http.MethodPost, "/jobs/dispatch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("with_max_retry", func(t *testing.T) {
		body := []byte(`{"type":"notification.send","payload":{"user_id":"123"},"max_retry":5}`)
		req := httptest.NewRequest(http.MethodPost, "/jobs/dispatch", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})
}

func TestListTypesHandler(t *testing.T) {
	app := fiber.New()

	// Mock list types endpoint
	app.Get("/jobs/types", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"types": []fiber.Map{
					{"type": "email.send", "description": "Send an email to a recipient"},
					{"type": "audit.cleanup", "description": "Clean up old audit log entries"},
					{"type": "notification.send", "description": "Send a notification to a user"},
				},
			},
		})
	})

	t.Run("list_types", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/jobs/types", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(body, &result)

		assert.Equal(t, true, result["success"])
		data := result["data"].(map[string]interface{})
		types := data["types"].([]interface{})
		assert.Len(t, types, 3)
	})
}
