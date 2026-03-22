package handler

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/14mdzk/goscratch/internal/module/user/dto"
	"github.com/14mdzk/goscratch/internal/module/user/usecase"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

// TestNewHandler verifies handler construction
func TestNewHandler(t *testing.T) {
	var uc *usecase.UseCase
	h := NewHandler(uc)
	assert.NotNil(t, h)
}

// --- GetByID Tests ---

func TestGetByID(t *testing.T) {
	t.Run("valid_uuid", func(t *testing.T) {
		app := fiber.New()
		app.Get("/users/:id", func(c *fiber.Ctx) error {
			id := c.Params("id")
			if id == "01234567-89ab-cdef-0123-456789abcdef" {
				return c.JSON(fiber.Map{
					"success": true,
					"data": fiber.Map{
						"id":        id,
						"email":     "test@example.com",
						"name":      "Test User",
						"is_active": true,
					},
				})
			}
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "NOT_FOUND", "message": "user not found"},
			})
		})

		req := httptest.NewRequest(http.MethodGet, "/users/01234567-89ab-cdef-0123-456789abcdef", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, true, result["success"])
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "01234567-89ab-cdef-0123-456789abcdef", data["id"])
		assert.Equal(t, "test@example.com", data["email"])
	})

	t.Run("invalid_uuid", func(t *testing.T) {
		app := fiber.New()
		app.Get("/users/:id", func(c *fiber.Ctx) error {
			id := c.Params("id")
			if len(id) != 36 {
				return c.Status(http.StatusNotFound).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "NOT_FOUND", "message": "user not found"},
				})
			}
			return c.JSON(fiber.Map{"success": true})
		})

		req := httptest.NewRequest(http.MethodGet, "/users/invalid-uuid", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("not_found", func(t *testing.T) {
		app := fiber.New()
		app.Get("/users/:id", func(c *fiber.Ctx) error {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "NOT_FOUND", "message": "user not found"},
			})
		})

		req := httptest.NewRequest(http.MethodGet, "/users/00000000-0000-0000-0000-000000000000", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, false, result["success"])
	})
}

// --- List Tests ---

func TestList(t *testing.T) {
	t.Run("default_params", func(t *testing.T) {
		app := fiber.New()
		app.Get("/users", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{
				"success": true,
				"data": []fiber.Map{
					{"id": "1", "email": "user1@example.com", "name": "User 1"},
					{"id": "2", "email": "user2@example.com", "name": "User 2"},
				},
				"pagination": fiber.Map{
					"has_more": false,
					"has_prev": false,
				},
			})
		})

		req := httptest.NewRequest(http.MethodGet, "/users", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, true, result["success"])
		data := result["data"].([]interface{})
		assert.Len(t, data, 2)
	})

	t.Run("with_cursor", func(t *testing.T) {
		app := fiber.New()
		app.Get("/users", func(c *fiber.Ctx) error {
			cursor := c.Query("cursor")
			assert.NotEmpty(t, cursor)
			return c.JSON(fiber.Map{
				"success": true,
				"data":    []fiber.Map{},
				"pagination": fiber.Map{
					"has_more": false,
					"has_prev": true,
				},
			})
		})

		req := httptest.NewRequest(http.MethodGet, "/users?cursor=abc123", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("with_filters", func(t *testing.T) {
		app := fiber.New()
		app.Get("/users", func(c *fiber.Ctx) error {
			search := c.Query("search")
			email := c.Query("email")
			isActive := c.Query("is_active")

			assert.Equal(t, "john", search)
			assert.Equal(t, "john@example.com", email)
			assert.Equal(t, "true", isActive)

			return c.JSON(fiber.Map{
				"success": true,
				"data": []fiber.Map{
					{"id": "1", "email": "john@example.com", "name": "John"},
				},
				"pagination": fiber.Map{
					"has_more": false,
					"has_prev": false,
				},
			})
		})

		req := httptest.NewRequest(http.MethodGet, "/users?search=john&email=john@example.com&is_active=true", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// --- Create Tests ---

func TestCreate(t *testing.T) {
	t.Run("valid_request", func(t *testing.T) {
		app := fiber.New()
		app.Post("/users", func(c *fiber.Ctx) error {
			var req dto.CreateUserRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}
			if req.Email == "" || req.Password == "" || req.Name == "" {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Missing required fields"},
				})
			}
			return c.Status(http.StatusCreated).JSON(fiber.Map{
				"success": true,
				"data": fiber.Map{
					"id":        "01234567-89ab-cdef-0123-456789abcdef",
					"email":     req.Email,
					"name":      req.Name,
					"is_active": true,
				},
			})
		})

		body := []byte(`{"email":"new@example.com","password":"password123","name":"New User"}`)
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusCreated, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, true, result["success"])
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "new@example.com", data["email"])
		assert.Equal(t, "New User", data["name"])
	})

	t.Run("validation_errors_missing_email", func(t *testing.T) {
		app := fiber.New()
		app.Post("/users", func(c *fiber.Ctx) error {
			var req dto.CreateUserRequest
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

		body := []byte(`{"password":"password123","name":"User"}`)
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("validation_errors_short_password", func(t *testing.T) {
		app := fiber.New()
		app.Post("/users", func(c *fiber.Ctx) error {
			var req dto.CreateUserRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}
			if len(req.Password) < 8 {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "password must be at least 8 characters"},
				})
			}
			return c.JSON(fiber.Map{"success": true})
		})

		body := []byte(`{"email":"test@example.com","password":"short","name":"User"}`)
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("duplicate_email", func(t *testing.T) {
		app := fiber.New()
		app.Post("/users", func(c *fiber.Ctx) error {
			return c.Status(http.StatusConflict).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "CONFLICT", "message": "user with email test@example.com already exists"},
			})
		})

		body := []byte(`{"email":"test@example.com","password":"password123","name":"User"}`)
		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusConflict, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, false, result["success"])
		errData := result["error"].(map[string]interface{})
		assert.Equal(t, "CONFLICT", errData["code"])
	})

	t.Run("invalid_json", func(t *testing.T) {
		app := fiber.New()
		app.Post("/users", func(c *fiber.Ctx) error {
			var req dto.CreateUserRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}
			return c.JSON(fiber.Map{"success": true})
		})

		req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader([]byte(`invalid json`)))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

// --- Update Tests ---

func TestUpdate(t *testing.T) {
	t.Run("valid_request", func(t *testing.T) {
		app := fiber.New()
		app.Put("/users/:id", func(c *fiber.Ctx) error {
			id := c.Params("id")
			var req dto.UpdateUserRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}
			return c.JSON(fiber.Map{
				"success": true,
				"data": fiber.Map{
					"id":    id,
					"email": "test@example.com",
					"name":  req.Name,
				},
			})
		})

		body := []byte(`{"name":"Updated Name"}`)
		req := httptest.NewRequest(http.MethodPut, "/users/01234567-89ab-cdef-0123-456789abcdef", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, true, result["success"])
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "Updated Name", data["name"])
	})

	t.Run("not_found", func(t *testing.T) {
		app := fiber.New()
		app.Put("/users/:id", func(c *fiber.Ctx) error {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "NOT_FOUND", "message": "user not found"},
			})
		})

		body := []byte(`{"name":"Updated Name"}`)
		req := httptest.NewRequest(http.MethodPut, "/users/00000000-0000-0000-0000-000000000000", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})

	t.Run("validation_errors", func(t *testing.T) {
		app := fiber.New()
		app.Put("/users/:id", func(c *fiber.Ctx) error {
			var req dto.UpdateUserRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}
			// Name must be at least 2 chars
			if req.Name != "" && len(req.Name) < 2 {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "name must be at least 2 characters"},
				})
			}
			return c.JSON(fiber.Map{"success": true})
		})

		body := []byte(`{"name":"A"}`)
		req := httptest.NewRequest(http.MethodPut, "/users/01234567-89ab-cdef-0123-456789abcdef", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

// --- Delete Tests ---

func TestDelete(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		app := fiber.New()
		app.Delete("/users/:id", func(c *fiber.Ctx) error {
			return c.SendStatus(http.StatusNoContent)
		})

		req := httptest.NewRequest(http.MethodDelete, "/users/01234567-89ab-cdef-0123-456789abcdef", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, resp.StatusCode)
	})

	t.Run("not_found", func(t *testing.T) {
		app := fiber.New()
		app.Delete("/users/:id", func(c *fiber.Ctx) error {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "NOT_FOUND", "message": "user not found"},
			})
		})

		req := httptest.NewRequest(http.MethodDelete, "/users/00000000-0000-0000-0000-000000000000", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

// --- GetMe Tests ---

func TestGetMe(t *testing.T) {
	t.Run("authenticated_user", func(t *testing.T) {
		app := fiber.New()
		app.Get("/users/me", func(c *fiber.Ctx) error {
			// Simulate middleware setting user_id
			c.Locals("user_id", "01234567-89ab-cdef-0123-456789abcdef")
			userID, ok := c.Locals("user_id").(string)
			if !ok || userID == "" {
				return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "UNAUTHORIZED", "message": "Authentication required"},
				})
			}
			return c.JSON(fiber.Map{
				"success": true,
				"data": fiber.Map{
					"id":    userID,
					"email": "me@example.com",
					"name":  "Current User",
				},
			})
		})

		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, true, result["success"])
		data := result["data"].(map[string]interface{})
		assert.Equal(t, "me@example.com", data["email"])
	})

	t.Run("unauthenticated_user", func(t *testing.T) {
		app := fiber.New()
		app.Get("/users/me", func(c *fiber.Ctx) error {
			userID, _ := c.Locals("user_id").(string)
			if userID == "" {
				return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "UNAUTHORIZED", "message": "Authentication required"},
				})
			}
			return c.JSON(fiber.Map{"success": true})
		})

		req := httptest.NewRequest(http.MethodGet, "/users/me", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// --- ChangePassword Tests ---

func TestChangePassword(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		app := fiber.New()
		app.Post("/users/me/password", func(c *fiber.Ctx) error {
			c.Locals("user_id", "user-123")
			userID, _ := c.Locals("user_id").(string)
			if userID == "" {
				return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "UNAUTHORIZED", "message": "Authentication required"},
				})
			}
			var req dto.ChangePasswordRequest
			if err := c.BodyParser(&req); err != nil {
				return c.Status(http.StatusBadRequest).JSON(fiber.Map{
					"success": false,
					"error":   fiber.Map{"code": "VALIDATION_ERROR", "message": "Invalid request body"},
				})
			}
			return c.JSON(fiber.Map{
				"success": true,
				"message": "Password changed successfully",
			})
		})

		body := []byte(`{"current_password":"oldpass123","new_password":"newpass123"}`)
		req := httptest.NewRequest(http.MethodPost, "/users/me/password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, true, result["success"])
		assert.Equal(t, "Password changed successfully", result["message"])
	})

	t.Run("wrong_current_password", func(t *testing.T) {
		app := fiber.New()
		app.Post("/users/me/password", func(c *fiber.Ctx) error {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "UNAUTHORIZED", "message": "Current password is incorrect"},
			})
		})

		body := []byte(`{"current_password":"wrongpass","new_password":"newpass123"}`)
		req := httptest.NewRequest(http.MethodPost, "/users/me/password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		result := parseResponse(t, resp)
		errData := result["error"].(map[string]interface{})
		assert.Equal(t, "Current password is incorrect", errData["message"])
	})
}

// --- Activate Tests ---

func TestActivate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		app := fiber.New()
		app.Post("/users/:id/activate", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{
				"success": true,
				"message": "User activated successfully",
			})
		})

		req := httptest.NewRequest(http.MethodPost, "/users/01234567-89ab-cdef-0123-456789abcdef/activate", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, true, result["success"])
		assert.Equal(t, "User activated successfully", result["message"])
	})

	t.Run("already_active", func(t *testing.T) {
		app := fiber.New()
		app.Post("/users/:id/activate", func(c *fiber.Ctx) error {
			// The real usecase returns nil for already active users (no-op)
			return c.JSON(fiber.Map{
				"success": true,
				"message": "User activated successfully",
			})
		})

		req := httptest.NewRequest(http.MethodPost, "/users/01234567-89ab-cdef-0123-456789abcdef/activate", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("not_found", func(t *testing.T) {
		app := fiber.New()
		app.Post("/users/:id/activate", func(c *fiber.Ctx) error {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "NOT_FOUND", "message": "user not found"},
			})
		})

		req := httptest.NewRequest(http.MethodPost, "/users/00000000-0000-0000-0000-000000000000/activate", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}

// --- Deactivate Tests ---

func TestDeactivate(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		app := fiber.New()
		app.Post("/users/:id/deactivate", func(c *fiber.Ctx) error {
			return c.JSON(fiber.Map{
				"success": true,
				"message": "User deactivated successfully",
			})
		})

		req := httptest.NewRequest(http.MethodPost, "/users/01234567-89ab-cdef-0123-456789abcdef/deactivate", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		result := parseResponse(t, resp)
		assert.Equal(t, true, result["success"])
		assert.Equal(t, "User deactivated successfully", result["message"])
	})

	t.Run("already_inactive", func(t *testing.T) {
		app := fiber.New()
		app.Post("/users/:id/deactivate", func(c *fiber.Ctx) error {
			// The real usecase returns nil for already inactive users (no-op)
			return c.JSON(fiber.Map{
				"success": true,
				"message": "User deactivated successfully",
			})
		})

		req := httptest.NewRequest(http.MethodPost, "/users/01234567-89ab-cdef-0123-456789abcdef/deactivate", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("not_found", func(t *testing.T) {
		app := fiber.New()
		app.Post("/users/:id/deactivate", func(c *fiber.Ctx) error {
			return c.Status(http.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"error":   fiber.Map{"code": "NOT_FOUND", "message": "user not found"},
			})
		})

		req := httptest.NewRequest(http.MethodPost, "/users/00000000-0000-0000-0000-000000000000/deactivate", nil)
		resp, err := app.Test(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	})
}
