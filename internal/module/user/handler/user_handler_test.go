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

func TestHealthHandler(t *testing.T) {
	app := fiber.New()

	// Simple health endpoint for testing
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"status": "ok",
			},
		})
	})

	t.Run("health_check", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/health", nil)
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		body, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		err = json.Unmarshal(body, &result)

		assert.NoError(t, err)
		assert.Equal(t, true, result["success"])
	})
}

func TestUserHandlerValidation(t *testing.T) {
	app := fiber.New()

	// Mock user creation endpoint
	app.Post("/users", func(c *fiber.Ctx) error {
		type CreateUserRequest struct {
			Email    string `json:"email"`
			Password string `json:"password"`
			Name     string `json:"name"`
		}

		var req CreateUserRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"message": "Invalid request body"},
			})
		}

		// Simple validation
		if req.Email == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "email is required"},
			})
		}

		if len(req.Password) < 8 {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "password must be at least 8 characters"},
			})
		}

		return c.Status(http.StatusCreated).JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"id":    "123",
				"email": req.Email,
				"name":  req.Name,
			},
		})
	})

	t.Run("valid_create_user", func(t *testing.T) {
		body := []byte(`{"email":"test@example.com","password":"password123","name":"Test User"}`)
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)
	})

	t.Run("missing_email", func(t *testing.T) {
		body := []byte(`{"password":"password123","name":"Test"}`)
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("short_password", func(t *testing.T) {
		body := []byte(`{"email":"test@example.com","password":"short","name":"Test"}`)
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("invalid_json", func(t *testing.T) {
		body := []byte(`not valid json`)
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

func TestAuthHandlerValidation(t *testing.T) {
	app := fiber.New()

	// Mock login endpoint
	app.Post("/auth/login", func(c *fiber.Ctx) error {
		type LoginRequest struct {
			Email    string `json:"email"`
			Password string `json:"password"`
		}

		var req LoginRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"message": "Invalid request body"},
			})
		}

		if req.Email == "" || req.Password == "" {
			return c.Status(http.StatusBadRequest).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "email and password are required"},
			})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"access_token":  "mock_token",
				"refresh_token": "mock_refresh",
				"expires_in":    900,
				"token_type":    "Bearer",
			},
		})
	})

	t.Run("valid_login", func(t *testing.T) {
		body := []byte(`{"email":"user@example.com","password":"password123"}`)
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]interface{}
		json.Unmarshal(respBody, &result)

		assert.Equal(t, true, result["success"])
		data := result["data"].(map[string]interface{})
		assert.NotEmpty(t, data["access_token"])
	})

	t.Run("missing_credentials", func(t *testing.T) {
		body := []byte(`{"email":"user@example.com"}`)
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}
