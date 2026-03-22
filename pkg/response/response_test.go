package response

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupApp(handler fiber.Handler) *fiber.App {
	app := fiber.New()
	app.Get("/test", handler)
	return app
}

func doRequest(t *testing.T, app *fiber.App) (*http.Response, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close()

	if len(body) == 0 {
		return resp, nil
	}

	var result map[string]any
	err = json.Unmarshal(body, &result)
	require.NoError(t, err)
	return resp, result
}

func TestSuccess(t *testing.T) {
	app := setupApp(func(c *fiber.Ctx) error {
		return Success(c, map[string]string{"name": "test"})
	})

	resp, body := doRequest(t, app)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, true, body["success"])
	data := body["data"].(map[string]any)
	assert.Equal(t, "test", data["name"])
}

func TestPaginated(t *testing.T) {
	app := setupApp(func(c *fiber.Ctx) error {
		items := []string{"a", "b"}
		pagination := map[string]any{"has_more": true, "next_cursor": "abc"}
		return Paginated(c, items, pagination)
	})

	resp, body := doRequest(t, app)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, true, body["success"])
	assert.NotNil(t, body["data"])
	assert.NotNil(t, body["pagination"])
}

func TestCreated(t *testing.T) {
	app := setupApp(func(c *fiber.Ctx) error {
		return Created(c, map[string]string{"id": "123"})
	})

	resp, body := doRequest(t, app)
	assert.Equal(t, http.StatusCreated, resp.StatusCode)
	assert.Equal(t, true, body["success"])
}

func TestNoContent(t *testing.T) {
	app := setupApp(func(c *fiber.Ctx) error {
		return NoContent(c)
	})

	resp, _ := doRequest(t, app)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)
}

func TestMessage(t *testing.T) {
	app := setupApp(func(c *fiber.Ctx) error {
		return Message(c, "operation successful")
	})

	resp, body := doRequest(t, app)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, true, body["success"])
	assert.Equal(t, "operation successful", body["message"])
}

func TestFail(t *testing.T) {
	t.Run("app_error", func(t *testing.T) {
		app := setupApp(func(c *fiber.Ctx) error {
			return Fail(c, apperr.ErrNotFound.WithMessage("user not found"))
		})

		resp, body := doRequest(t, app)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		assert.Equal(t, false, body["success"])
		errObj := body["error"].(map[string]any)
		assert.Equal(t, apperr.CodeNotFound, errObj["code"])
		assert.Equal(t, "user not found", errObj["message"])
	})

	t.Run("generic_error", func(t *testing.T) {
		app := setupApp(func(c *fiber.Ctx) error {
			return Fail(c, errors.New("something broke"))
		})

		resp, body := doRequest(t, app)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		assert.Equal(t, false, body["success"])
		errObj := body["error"].(map[string]any)
		assert.Equal(t, apperr.CodeInternalError, errObj["code"])
	})
}

func TestFailWithDetails(t *testing.T) {
	t.Run("app_error_with_details", func(t *testing.T) {
		app := setupApp(func(c *fiber.Ctx) error {
			return FailWithDetails(c, apperr.ErrBadRequest, map[string]any{"field": "email"})
		})

		resp, body := doRequest(t, app)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
		errObj := body["error"].(map[string]any)
		details := errObj["details"].(map[string]any)
		assert.Equal(t, "email", details["field"])
	})

	t.Run("generic_error_with_details", func(t *testing.T) {
		app := setupApp(func(c *fiber.Ctx) error {
			return FailWithDetails(c, errors.New("fail"), map[string]any{"info": "extra"})
		})

		resp, body := doRequest(t, app)
		assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
		errObj := body["error"].(map[string]any)
		details := errObj["details"].(map[string]any)
		assert.Equal(t, "extra", details["info"])
	})
}

func TestValidationFailed(t *testing.T) {
	app := setupApp(func(c *fiber.Ctx) error {
		return ValidationFailed(c, map[string]string{
			"email":    "email is required",
			"password": "password must be at least 8 characters",
		})
	})

	resp, body := doRequest(t, app)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, false, body["success"])
	errObj := body["error"].(map[string]any)
	assert.Equal(t, apperr.CodeValidation, errObj["code"])
	assert.Equal(t, "Validation failed", errObj["message"])
	details := errObj["details"].(map[string]any)
	assert.Equal(t, "email is required", details["email"])
	assert.Equal(t, "password must be at least 8 characters", details["password"])
}

func TestUnauthorized(t *testing.T) {
	t.Run("custom_message", func(t *testing.T) {
		app := setupApp(func(c *fiber.Ctx) error {
			return Unauthorized(c, "token expired")
		})

		resp, body := doRequest(t, app)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		errObj := body["error"].(map[string]any)
		assert.Equal(t, "token expired", errObj["message"])
	})

	t.Run("default_message", func(t *testing.T) {
		app := setupApp(func(c *fiber.Ctx) error {
			return Unauthorized(c, "")
		})

		resp, body := doRequest(t, app)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
		errObj := body["error"].(map[string]any)
		assert.Equal(t, "Authentication required", errObj["message"])
	})
}

func TestForbidden(t *testing.T) {
	t.Run("custom_message", func(t *testing.T) {
		app := setupApp(func(c *fiber.Ctx) error {
			return Forbidden(c, "insufficient permissions")
		})

		resp, body := doRequest(t, app)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		errObj := body["error"].(map[string]any)
		assert.Equal(t, "insufficient permissions", errObj["message"])
	})

	t.Run("default_message", func(t *testing.T) {
		app := setupApp(func(c *fiber.Ctx) error {
			return Forbidden(c, "")
		})

		resp, body := doRequest(t, app)
		assert.Equal(t, http.StatusForbidden, resp.StatusCode)
		errObj := body["error"].(map[string]any)
		assert.Equal(t, "Access denied", errObj["message"])
	})
}

func TestNotFound(t *testing.T) {
	t.Run("custom_message", func(t *testing.T) {
		app := setupApp(func(c *fiber.Ctx) error {
			return NotFound(c, "user not found")
		})

		resp, body := doRequest(t, app)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		errObj := body["error"].(map[string]any)
		assert.Equal(t, "user not found", errObj["message"])
	})

	t.Run("default_message", func(t *testing.T) {
		app := setupApp(func(c *fiber.Ctx) error {
			return NotFound(c, "")
		})

		resp, body := doRequest(t, app)
		assert.Equal(t, http.StatusNotFound, resp.StatusCode)
		errObj := body["error"].(map[string]any)
		assert.Equal(t, "Resource not found", errObj["message"])
	})
}
