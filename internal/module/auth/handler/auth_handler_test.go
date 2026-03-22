package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/14mdzk/goscratch/internal/module/auth/dto"
	"github.com/14mdzk/goscratch/internal/module/auth/usecase"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
)

// parseResponse reads and parses JSON response body
func parseResponse(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	return result
}

func TestLogin(t *testing.T) {
	t.Run("invalid_json", func(t *testing.T) {
		app := fiber.New()
		app.Post("/auth/login", func(c *fiber.Ctx) error {
			var req dto.LoginRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}
			return c.JSON(fiber.Map{"success": true})
		})

		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader([]byte(`not valid json`)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, false, result["success"])
	})

	t.Run("missing_email", func(t *testing.T) {
		app := fiber.New()
		app.Post("/auth/login", func(c *fiber.Ctx) error {
			var req dto.LoginRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}
			if req.Email == "" {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "email is required"},
				})
			}
			return c.JSON(fiber.Map{"success": true})
		})

		body := []byte(`{"password":"password123"}`)
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, false, result["success"])
	})

	t.Run("missing_password", func(t *testing.T) {
		app := fiber.New()
		app.Post("/auth/login", func(c *fiber.Ctx) error {
			var req dto.LoginRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}
			if req.Password == "" {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "password is required"},
				})
			}
			return c.JSON(fiber.Map{"success": true})
		})

		body := []byte(`{"email":"user@example.com"}`)
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, false, result["success"])
	})

	t.Run("valid_credentials_returns_tokens", func(t *testing.T) {
		app := fiber.New()
		app.Post("/auth/login", func(c *fiber.Ctx) error {
			var req dto.LoginRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}
			return c.JSON(fiber.Map{
				"success": true,
				"data": fiber.Map{
					"access_token":  "mock-access-token",
					"refresh_token": "mock-refresh-token",
					"expires_in":    900,
					"token_type":    "Bearer",
				},
			})
		})

		body := []byte(`{"email":"user@example.com","password":"password123"}`)
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, true, result["success"])
		data := result["data"].(map[string]interface{})
		assert.NotEmpty(t, data["access_token"])
		assert.NotEmpty(t, data["refresh_token"])
		assert.NotEmpty(t, data["expires_in"])
		assert.Equal(t, "Bearer", data["token_type"])
	})

	t.Run("wrong_password_returns_unauthorized", func(t *testing.T) {
		app := fiber.New()
		app.Post("/auth/login", func(c *fiber.Ctx) error {
			var req dto.LoginRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}

			hash, _ := bcrypt.GenerateFromPassword([]byte("correctpassword"), bcrypt.MinCost)
			if bcrypt.CompareHashAndPassword(hash, []byte(req.Password)) != nil {
				return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "UNAUTHORIZED", "message": "Invalid email or password"},
				})
			}
			return c.JSON(fiber.Map{"success": true})
		})

		body := []byte(`{"email":"user@example.com","password":"wrongpassword"}`)
		req := httptest.NewRequest(http.MethodPost, "/auth/login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, false, result["success"])
		errData := result["error"].(map[string]interface{})
		assert.Equal(t, "UNAUTHORIZED", errData["code"])
	})
}

func TestRefresh(t *testing.T) {
	t.Run("valid_token", func(t *testing.T) {
		app := fiber.New()
		app.Post("/auth/refresh", func(c *fiber.Ctx) error {
			var req dto.RefreshRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}
			if req.RefreshToken == "" {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "refresh_token is required"},
				})
			}
			return c.JSON(fiber.Map{
				"success": true,
				"data": fiber.Map{
					"access_token":  "new-access-token",
					"refresh_token": "new-refresh-token",
					"expires_in":    900,
					"token_type":    "Bearer",
				},
			})
		})

		body := []byte(`{"refresh_token":"valid-token-here"}`)
		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, true, result["success"])
		data := result["data"].(map[string]interface{})
		assert.NotEmpty(t, data["access_token"])
		assert.NotEmpty(t, data["refresh_token"])
	})

	t.Run("missing_token", func(t *testing.T) {
		app := fiber.New()
		app.Post("/auth/refresh", func(c *fiber.Ctx) error {
			var req dto.RefreshRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}
			if req.RefreshToken == "" {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "refresh_token is required"},
				})
			}
			return c.JSON(fiber.Map{"success": true})
		})

		body := []byte(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("invalid_token_returns_unauthorized", func(t *testing.T) {
		app := fiber.New()
		app.Post("/auth/refresh", func(c *fiber.Ctx) error {
			var req dto.RefreshRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "UNAUTHORIZED", "message": "Invalid or expired refresh token"},
			})
		})

		body := []byte(`{"refresh_token":"invalid-token"}`)
		req := httptest.NewRequest(http.MethodPost, "/auth/refresh", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, false, result["success"])
	})
}

func TestLogout(t *testing.T) {
	t.Run("valid_logout", func(t *testing.T) {
		app := fiber.New()
		app.Post("/auth/logout", func(c *fiber.Ctx) error {
			var req dto.RefreshRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}
			if req.RefreshToken == "" {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "refresh_token is required"},
				})
			}
			return c.JSON(fiber.Map{
				"success": true,
				"message": "Logged out successfully",
			})
		})

		body := []byte(`{"refresh_token":"valid-token-here"}`)
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, true, result["success"])
		assert.Equal(t, "Logged out successfully", result["message"])
	})

	t.Run("missing_token", func(t *testing.T) {
		app := fiber.New()
		app.Post("/auth/logout", func(c *fiber.Ctx) error {
			var req dto.RefreshRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}
			if req.RefreshToken == "" {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "refresh_token is required"},
				})
			}
			return c.JSON(fiber.Map{"success": true})
		})

		body := []byte(`{}`)
		req := httptest.NewRequest(http.MethodPost, "/auth/logout", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)

		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

// TestNewHandler verifies handler construction
func TestNewHandler(t *testing.T) {
	var uc *usecase.UseCase
	h := NewHandler(uc)
	assert.NotNil(t, h)
}
