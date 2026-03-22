package validator

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCreateUserRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required,min=2,max=100"`
}

func TestValidate_EmptyFields(t *testing.T) {
	// Test that empty fields fail validation
	req := testCreateUserRequest{
		Email:    "",
		Password: "",
		Name:     "",
	}

	err := Validate(&req)

	require.NotNil(t, err, "Validation should fail for empty fields")

	ve, ok := IsValidationError(err)
	require.True(t, ok, "Error should be ValidationError")
	require.NotNil(t, ve)

	// Check that all 3 fields have errors
	assert.Contains(t, ve.Errors, "email", "email should have validation error")
	assert.Contains(t, ve.Errors, "password", "password should have validation error")
	assert.Contains(t, ve.Errors, "name", "name should have validation error")

	t.Logf("Validation errors: %v", ve.Errors)
}

func TestValidate_ValidFields(t *testing.T) {
	req := testCreateUserRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test User",
	}

	err := Validate(&req)
	assert.Nil(t, err, "Validation should pass for valid fields")
}

func TestValidate_InvalidEmail(t *testing.T) {
	req := testCreateUserRequest{
		Email:    "not-an-email",
		Password: "password123",
		Name:     "Test User",
	}

	err := Validate(&req)
	require.NotNil(t, err)

	ve, ok := IsValidationError(err)
	require.True(t, ok)
	assert.Contains(t, ve.Errors, "email")
}

func TestValidate_ShortPassword(t *testing.T) {
	req := testCreateUserRequest{
		Email:    "test@example.com",
		Password: "short",
		Name:     "Test User",
	}

	err := Validate(&req)
	require.NotNil(t, err)

	ve, ok := IsValidationError(err)
	require.True(t, ok)
	assert.Contains(t, ve.Errors, "password")
}

func TestValidate_ShortName(t *testing.T) {
	req := testCreateUserRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "A",
	}

	err := Validate(&req)
	require.NotNil(t, err)

	ve, ok := IsValidationError(err)
	require.True(t, ok)
	assert.Contains(t, ve.Errors, "name")
}

// --- ValidateAndBind Tests ---

func TestValidateAndBind(t *testing.T) {
	t.Run("valid_body", func(t *testing.T) {
		app := fiber.New()
		app.Post("/test", func(c *fiber.Ctx) error {
			var req testCreateUserRequest
			if err := ValidateAndBind(c, &req); err != nil {
				return c.Status(400).JSON(map[string]string{"error": err.Error()})
			}
			return c.JSON(map[string]string{"email": req.Email})
		})

		body := `{"email":"test@example.com","password":"password123","name":"Test User"}`
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid_json_body", func(t *testing.T) {
		app := fiber.New()
		app.Post("/test", func(c *fiber.Ctx) error {
			var req testCreateUserRequest
			if err := ValidateAndBind(c, &req); err != nil {
				ve, ok := IsValidationError(err)
				if ok {
					return c.Status(400).JSON(ve.Errors)
				}
				return c.Status(400).JSON(map[string]string{"error": err.Error()})
			}
			return c.SendStatus(200)
		})

		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader("not-json"))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]string
		_ = json.Unmarshal(respBody, &result)
		assert.Contains(t, result, "body")
	})

	t.Run("validation_failure", func(t *testing.T) {
		app := fiber.New()
		app.Post("/test", func(c *fiber.Ctx) error {
			var req testCreateUserRequest
			if err := ValidateAndBind(c, &req); err != nil {
				ve, ok := IsValidationError(err)
				if ok {
					return c.Status(400).JSON(ve.Errors)
				}
				return c.Status(400).JSON(map[string]string{"error": err.Error()})
			}
			return c.SendStatus(200)
		})

		body := `{"email":"bad","password":"short","name":"A"}`
		req := httptest.NewRequest(http.MethodPost, "/test", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")

		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

// --- ValidateQuery Tests ---

type testQueryParams struct {
	Page  int    `query:"page" validate:"required,gte=1"`
	Limit int    `query:"limit" validate:"required,gte=1,lte=100"`
	Sort  string `query:"sort" validate:"omitempty,oneof=asc desc"`
}

func TestValidateQuery(t *testing.T) {
	t.Run("valid_query", func(t *testing.T) {
		app := fiber.New()
		app.Get("/test", func(c *fiber.Ctx) error {
			var q testQueryParams
			if err := ValidateQuery(c, &q); err != nil {
				return c.Status(400).JSON(map[string]string{"error": err.Error()})
			}
			return c.JSON(map[string]int{"page": q.Page, "limit": q.Limit})
		})

		req := httptest.NewRequest(http.MethodGet, "/test?page=1&limit=20&sort=asc", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("invalid_query_params", func(t *testing.T) {
		app := fiber.New()
		app.Get("/test", func(c *fiber.Ctx) error {
			var q testQueryParams
			if err := ValidateQuery(c, &q); err != nil {
				ve, ok := IsValidationError(err)
				if ok {
					return c.Status(400).JSON(ve.Errors)
				}
				return c.Status(400).JSON(map[string]string{"error": err.Error()})
			}
			return c.SendStatus(200)
		})

		req := httptest.NewRequest(http.MethodGet, "/test?page=0&limit=200&sort=invalid", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("missing_required_params", func(t *testing.T) {
		app := fiber.New()
		app.Get("/test", func(c *fiber.Ctx) error {
			var q testQueryParams
			if err := ValidateQuery(c, &q); err != nil {
				ve, ok := IsValidationError(err)
				if ok {
					return c.Status(400).JSON(ve.Errors)
				}
				return c.Status(400).JSON(map[string]string{"error": err.Error()})
			}
			return c.SendStatus(200)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})
}

// --- HandleValidationError Tests ---

func TestHandleValidationError(t *testing.T) {
	t.Run("validation_error", func(t *testing.T) {
		app := fiber.New()
		app.Get("/test", func(c *fiber.Ctx) error {
			err := &ValidationError{Errors: map[string]string{
				"email": "email is required",
			}}
			return HandleValidationError(c, err)
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]any
		_ = json.Unmarshal(respBody, &result)
		assert.Equal(t, false, result["success"])
		errObj := result["error"].(map[string]any)
		assert.Equal(t, "Validation failed", errObj["message"])
		details := errObj["details"].(map[string]any)
		assert.Equal(t, "email is required", details["email"])
	})

	t.Run("non_validation_error", func(t *testing.T) {
		app := fiber.New()
		app.Get("/test", func(c *fiber.Ctx) error {
			return HandleValidationError(c, errors.New("something broke"))
		})

		req := httptest.NewRequest(http.MethodGet, "/test", nil)
		resp, err := app.Test(req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)

		respBody, _ := io.ReadAll(resp.Body)
		var result map[string]any
		_ = json.Unmarshal(respBody, &result)
		assert.Equal(t, false, result["success"])
	})
}
