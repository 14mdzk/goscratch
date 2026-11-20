package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockAuthorizer implements port.Enforcer for testing (no policy-management
// or lifecycle stubs required).
type mockAuthorizer struct {
	enforceFunc        func(sub, obj, act string) (bool, error)
	enforceCtxFunc     func(ctx context.Context, sub, obj, act string) (bool, error)
	hasRoleForUserFunc func(userID, role string) (bool, error)
}

func (m *mockAuthorizer) Enforce(sub, obj, act string) (bool, error) {
	if m.enforceFunc != nil {
		return m.enforceFunc(sub, obj, act)
	}
	return false, nil
}

func (m *mockAuthorizer) EnforceWithContext(ctx context.Context, sub, obj, act string) (bool, error) {
	if m.enforceCtxFunc != nil {
		return m.enforceCtxFunc(ctx, sub, obj, act)
	}
	return m.Enforce(sub, obj, act)
}

func (m *mockAuthorizer) HasRoleForUser(userID, role string) (bool, error) {
	if m.hasRoleForUserFunc != nil {
		return m.hasRoleForUserFunc(userID, role)
	}
	return false, nil
}

// setupAuthzApp creates a fiber app with user_id pre-set in locals
func setupAuthzApp(handler fiber.Handler, userID string) *fiber.App {
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		if userID != "" {
			c.Locals("user_id", userID)
		}
		return c.Next()
	})
	app.Get("/test", handler, func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})
	return app
}

func TestRequirePermission_Allowed(t *testing.T) {
	mock := &mockAuthorizer{
		enforceFunc: func(sub, obj, act string) (bool, error) {
			return sub == "user-1" && obj == "users" && act == "read", nil
		},
	}

	app := setupAuthzApp(RequirePermission(mock, "users", "read"), "user-1")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestRequirePermission_Denied(t *testing.T) {
	mock := &mockAuthorizer{
		enforceFunc: func(_, _, _ string) (bool, error) {
			return false, nil
		},
	}

	app := setupAuthzApp(RequirePermission(mock, "users", "delete"), "user-1")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestRequirePermission_NoUserID(t *testing.T) {
	mock := &mockAuthorizer{}
	app := setupAuthzApp(RequirePermission(mock, "users", "read"), "")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusUnauthorized, resp.StatusCode)
}

func TestRequirePermission_EnforceError(t *testing.T) {
	mock := &mockAuthorizer{
		enforceFunc: func(_, _, _ string) (bool, error) {
			return false, errors.New("db error")
		},
	}

	app := setupAuthzApp(RequirePermission(mock, "users", "read"), "user-1")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusInternalServerError, resp.StatusCode)
}

func TestRequireRole_HasRole(t *testing.T) {
	mock := &mockAuthorizer{
		hasRoleForUserFunc: func(userID, role string) (bool, error) {
			return userID == "user-1" && role == "admin", nil
		},
	}

	app := setupAuthzApp(RequireRole(mock, "admin"), "user-1")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestRequireRole_MissingRole(t *testing.T) {
	mock := &mockAuthorizer{
		hasRoleForUserFunc: func(_, _ string) (bool, error) {
			return false, nil
		},
	}

	app := setupAuthzApp(RequireRole(mock, "admin"), "user-1")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestRequireAnyPermission_OneMatches(t *testing.T) {
	mock := &mockAuthorizer{
		enforceFunc: func(_, obj, act string) (bool, error) {
			return obj == "orders" && act == "read", nil
		},
	}

	app := setupAuthzApp(RequireAnyPermission(mock, "users:write", "orders:read"), "user-1")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestRequireAnyPermission_NoneMatch(t *testing.T) {
	mock := &mockAuthorizer{
		enforceFunc: func(_, _, _ string) (bool, error) {
			return false, nil
		},
	}

	app := setupAuthzApp(RequireAnyPermission(mock, "users:write", "orders:delete"), "user-1")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestRequireAllPermissions_AllMatch(t *testing.T) {
	mock := &mockAuthorizer{
		enforceFunc: func(_, _, _ string) (bool, error) {
			return true, nil
		},
	}

	app := setupAuthzApp(RequireAllPermissions(mock, "users:read", "orders:read"), "user-1")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
}

func TestRequireAllPermissions_OneMissing(t *testing.T) {
	mock := &mockAuthorizer{
		enforceFunc: func(_, obj, _ string) (bool, error) {
			return obj == "users", nil // only users permission granted
		},
	}

	app := setupAuthzApp(RequireAllPermissions(mock, "users:read", "orders:read"), "user-1")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)

	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusForbidden, resp.StatusCode)
}

func TestParsePermission(t *testing.T) {
	tests := []struct {
		perm    string
		wantObj string
		wantAct string
	}{
		{"users:read", "users", "read"},
		{"orders:delete", "orders", "delete"},
		{"nocolon", "nocolon", "*"},
	}

	for _, tt := range tests {
		t.Run(tt.perm, func(t *testing.T) {
			obj, act := parsePermission(tt.perm)
			assert.Equal(t, tt.wantObj, obj)
			assert.Equal(t, tt.wantAct, act)
		})
	}
}

func TestRequirePermission_PropagatesUserContext(t *testing.T) {
	type ctxKey struct{}

	var seenVal any
	mock := &mockAuthorizer{
		enforceCtxFunc: func(ctx context.Context, sub, obj, act string) (bool, error) {
			seenVal = ctx.Value(ctxKey{})
			return sub == "user-1" && obj == "users" && act == "read", nil
		},
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user_id", "user-1")
		c.SetUserContext(context.WithValue(c.UserContext(), ctxKey{}, "req-trace-42"))
		return c.Next()
	})
	app.Get("/test", RequirePermission(mock, "users", "read"), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/test", nil))
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)
	assert.Equal(t, "req-trace-42", seenVal)
}
