package health

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

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

func setupApp() *fiber.App {
	app := fiber.New()
	module := NewModule()
	module.RegisterRoutes(app)
	return app
}

func TestHealthCheck(t *testing.T) {
	app := setupApp()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponse(t, resp)
	assert.Equal(t, true, result["success"])

	data := result["data"].(map[string]interface{})
	assert.Equal(t, "ok", data["status"])
	assert.NotEmpty(t, data["timestamp"])
}

func TestReadinessCheck(t *testing.T) {
	app := setupApp()

	req := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponse(t, resp)
	assert.Equal(t, true, result["success"])

	data := result["data"].(map[string]interface{})
	assert.Equal(t, "ready", data["status"])
	assert.NotEmpty(t, data["timestamp"])

	// Verify checks are present
	checks := data["checks"].(map[string]interface{})
	assert.Equal(t, "ok", checks["database"])
	assert.Equal(t, "ok", checks["cache"])
}

func TestLivenessCheck(t *testing.T) {
	app := setupApp()

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponse(t, resp)
	assert.Equal(t, true, result["success"])

	data := result["data"].(map[string]interface{})
	assert.Equal(t, "alive", data["status"])
	assert.NotEmpty(t, data["timestamp"])
}

func TestNewHandler(t *testing.T) {
	h := NewHandler()
	assert.NotNil(t, h)
}

func TestNewModule(t *testing.T) {
	m := NewModule()
	assert.NotNil(t, m)
}
