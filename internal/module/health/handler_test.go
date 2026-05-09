package health

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseBody reads the raw response body into a generic map.
func parseBody(t *testing.T, resp *http.Response) map[string]interface{} {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	defer resp.Body.Close()
	var result map[string]interface{}
	require.NoError(t, json.Unmarshal(body, &result))
	return result
}

// stubChecker is a test double for HealthChecker.
type stubChecker struct {
	name string
	err  error
	// delay simulates a slow check for timeout tests.
	delay time.Duration
}

func (s *stubChecker) Name() string { return s.name }
func (s *stubChecker) Check(ctx context.Context) error {
	if s.delay > 0 {
		select {
		case <-time.After(s.delay):
		case <-ctx.Done():
			return errors.New("context deadline exceeded")
		}
	}
	return s.err
}

// setupApp builds a Fiber app with the given readiness timeout and checkers.
func setupApp(timeout time.Duration, checkers ...HealthChecker) *fiber.App {
	app := fiber.New()
	module := NewModule(timeout, checkers...)
	module.RegisterRoutes(app)
	return app
}

// TestLivenessNoCheckers: GET /healthz/live returns 200 with status "alive" even
// when no checkers are wired.
func TestLivenessNoCheckers(t *testing.T) {
	app := setupApp(0)

	req := httptest.NewRequest(http.MethodGet, "/healthz/live", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseBody(t, resp)
	data := result
	assert.Equal(t, "alive", data["status"])
	assert.NotEmpty(t, data["timestamp"])
}

// TestReadinessAllPass: GET /healthz/ready returns 200 when all checkers pass.
func TestReadinessAllPass(t *testing.T) {
	app := setupApp(2*time.Second,
		&stubChecker{name: "db"},
		&stubChecker{name: "cache"},
	)

	req := httptest.NewRequest(http.MethodGet, "/healthz/ready", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseBody(t, resp)
	assert.Equal(t, "ready", result["status"])
	assert.NotEmpty(t, result["timestamp"])
	checks := result["checks"].(map[string]interface{})
	assert.Equal(t, "ok", checks["db"])
	assert.Equal(t, "ok", checks["cache"])
}

// TestReadinessOneFails: GET /healthz/ready returns 503 with the failing check
// name in the body when one checker errors.
func TestReadinessOneFails(t *testing.T) {
	app := setupApp(2*time.Second,
		&stubChecker{name: "database", err: errors.New("ping failed")},
		&stubChecker{name: "cache"},
	)

	req := httptest.NewRequest(http.MethodGet, "/healthz/ready", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	result := parseBody(t, resp)
	assert.Equal(t, "degraded", result["status"])
	checks := result["checks"].(map[string]interface{})
	assert.Equal(t, "ping failed", checks["database"])
	assert.Equal(t, "ok", checks["cache"])
}

// TestReadinessTimeout: GET /healthz/ready returns 503 when a checker exceeds
// the configured deadline.
func TestReadinessTimeout(t *testing.T) {
	// Checker takes 500ms but timeout budget is 50ms.
	app := setupApp(50*time.Millisecond,
		&stubChecker{name: "slow", delay: 500 * time.Millisecond},
	)

	req := httptest.NewRequest(http.MethodGet, "/healthz/ready", nil)
	resp, err := app.Test(req, 2000) // 2s test timeout
	require.NoError(t, err)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

	result := parseBody(t, resp)
	assert.Equal(t, "degraded", result["status"])
	checks := result["checks"].(map[string]interface{})
	assert.NotEmpty(t, checks["slow"])
}

// TestHealthAliasLiveness: GET /health (back-compat alias) returns 200 with
// the liveness payload.
func TestHealthAliasLiveness(t *testing.T) {
	app := setupApp(0)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseBody(t, resp)
	assert.Equal(t, "alive", result["status"])
	assert.NotEmpty(t, result["timestamp"])
}

// TestNewHandler verifies the constructor is non-nil.
func TestNewHandler(t *testing.T) {
	h := NewHandler(0)
	assert.NotNil(t, h)
}

// TestNewModule verifies the module constructor is non-nil.
func TestNewModule(t *testing.T) {
	m := NewModule(0)
	assert.NotNil(t, m)
}
