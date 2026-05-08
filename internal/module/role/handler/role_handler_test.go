package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	roledto "github.com/14mdzk/goscratch/internal/module/role/dto"
	"github.com/14mdzk/goscratch/internal/module/role/usecase"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAuthorizer is a mock implementation of port.Authorizer
type MockAuthorizer struct {
	mock.Mock
}

func (m *MockAuthorizer) Enforce(sub, obj, act string) (bool, error) {
	args := m.Called(sub, obj, act)
	return args.Bool(0), args.Error(1)
}

func (m *MockAuthorizer) EnforceWithContext(ctx context.Context, sub, obj, act string) (bool, error) {
	args := m.Called(ctx, sub, obj, act)
	return args.Bool(0), args.Error(1)
}

func (m *MockAuthorizer) AddRoleForUser(userID, role string) error {
	args := m.Called(userID, role)
	return args.Error(0)
}

func (m *MockAuthorizer) RemoveRoleForUser(userID, role string) error {
	args := m.Called(userID, role)
	return args.Error(0)
}

func (m *MockAuthorizer) GetRolesForUser(userID string) ([]string, error) {
	args := m.Called(userID)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockAuthorizer) GetUsersForRole(role string) ([]string, error) {
	args := m.Called(role)
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockAuthorizer) HasRoleForUser(userID, role string) (bool, error) {
	args := m.Called(userID, role)
	return args.Bool(0), args.Error(1)
}

func (m *MockAuthorizer) AddPermissionForRole(role, obj, act string) error {
	args := m.Called(role, obj, act)
	return args.Error(0)
}

func (m *MockAuthorizer) RemovePermissionForRole(role, obj, act string) error {
	args := m.Called(role, obj, act)
	return args.Error(0)
}

func (m *MockAuthorizer) GetPermissionsForRole(role string) ([][]string, error) {
	args := m.Called(role)
	return args.Get(0).([][]string), args.Error(1)
}

func (m *MockAuthorizer) AddPermissionForUser(userID, obj, act string) error {
	args := m.Called(userID, obj, act)
	return args.Error(0)
}

func (m *MockAuthorizer) RemovePermissionForUser(userID, obj, act string) error {
	args := m.Called(userID, obj, act)
	return args.Error(0)
}

func (m *MockAuthorizer) GetPermissionsForUser(userID string) ([][]string, error) {
	args := m.Called(userID)
	return args.Get(0).([][]string), args.Error(1)
}

func (m *MockAuthorizer) GetImplicitPermissionsForUser(userID string) ([][]string, error) {
	args := m.Called(userID)
	return args.Get(0).([][]string), args.Error(1)
}

func (m *MockAuthorizer) LoadPolicy() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockAuthorizer) SavePolicy() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockAuthorizer) Start(_ context.Context) error {
	return nil
}

func (m *MockAuthorizer) Close() error {
	args := m.Called()
	return args.Error(0)
}

var _ port.Authorizer = (*MockAuthorizer)(nil)

// setupTestApp creates a Fiber app with the handler for testing
func setupTestApp(mockAuth *MockAuthorizer) (*fiber.App, *Handler) {
	app := fiber.New()
	uc := usecase.NewUseCase(mockAuth)
	h := NewHandler(uc)
	return app, h
}

func parseResponseBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	defer resp.Body.Close()

	var result map[string]any
	err = json.Unmarshal(body, &result)
	assert.NoError(t, err)
	return result
}

func TestListRoles(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Get("/roles", h.ListRoles)

	req := httptest.NewRequest(http.MethodGet, "/roles", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponseBody(t, resp)
	assert.True(t, result["success"].(bool))
	data := result["data"].([]any)
	assert.Len(t, data, 4)
}

func TestAssignRole_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Post("/roles/assign", h.AssignRole)

	mockAuth.On("HasRoleForUser", "550e8400-e29b-41d4-a716-446655440000", "admin").Return(false, nil)
	mockAuth.On("AddRoleForUser", "550e8400-e29b-41d4-a716-446655440000", "admin").Return(nil)

	body, _ := json.Marshal(map[string]string{
		"user_id": "550e8400-e29b-41d4-a716-446655440000",
		"role":    "admin",
	})
	req := httptest.NewRequest(http.MethodPost, "/roles/assign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponseBody(t, resp)
	assert.True(t, result["success"].(bool))
	assert.Equal(t, "Role assigned successfully", result["message"])
	mockAuth.AssertExpectations(t)
}

func TestAssignRole_ValidationError(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Post("/roles/assign", h.AssignRole)

	// Missing required fields
	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/roles/assign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestAssignRole_InvalidUUID(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Post("/roles/assign", h.AssignRole)

	body, _ := json.Marshal(map[string]string{
		"user_id": "not-a-uuid",
		"role":    "admin",
	})
	req := httptest.NewRequest(http.MethodPost, "/roles/assign", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRevokeRole_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Post("/roles/revoke", h.RevokeRole)

	mockAuth.On("HasRoleForUser", "550e8400-e29b-41d4-a716-446655440000", "editor").Return(true, nil)
	mockAuth.On("RemoveRoleForUser", "550e8400-e29b-41d4-a716-446655440000", "editor").Return(nil)

	body, _ := json.Marshal(map[string]string{
		"user_id": "550e8400-e29b-41d4-a716-446655440000",
		"role":    "editor",
	})
	req := httptest.NewRequest(http.MethodPost, "/roles/revoke", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponseBody(t, resp)
	assert.True(t, result["success"].(bool))
	assert.Equal(t, "Role revoked successfully", result["message"])
	mockAuth.AssertExpectations(t)
}

func TestGetRoleUsers_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Get("/roles/:role/users", h.GetRoleUsers)

	mockAuth.On("GetUsersForRole", "admin").Return([]string{"user-1", "user-2"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/roles/admin/users", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponseBody(t, resp)
	assert.True(t, result["success"].(bool))
	data := result["data"].(map[string]any)
	assert.Equal(t, "admin", data["role"])
	userIDs := data["user_ids"].([]any)
	assert.Len(t, userIDs, 2)
}

func TestGetRoleUsers_InvalidRole(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Get("/roles/:role/users", h.GetRoleUsers)

	req := httptest.NewRequest(http.MethodGet, "/roles/invalid_role/users", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestGetRolePermissions_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Get("/roles/:role/permissions", h.GetRolePermissions)

	mockAuth.On("GetPermissionsForRole", "admin").Return([][]string{
		{"admin", "users", "read"},
		{"admin", "users", "create"},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/roles/admin/permissions", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponseBody(t, resp)
	assert.True(t, result["success"].(bool))
	data := result["data"].([]any)
	assert.Len(t, data, 2)

	perm0 := data[0].(map[string]any)
	assert.Equal(t, "users", perm0["object"])
	assert.Equal(t, "read", perm0["action"])
}

func TestAddRolePermission_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Post("/roles/:role/permissions", h.AddRolePermission)

	mockAuth.On("AddPermissionForRole", "editor", "posts", "write").Return(nil)

	body, _ := json.Marshal(map[string]string{
		"role":   "editor",
		"object": "posts",
		"action": "write",
	})
	req := httptest.NewRequest(http.MethodPost, "/roles/editor/permissions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponseBody(t, resp)
	assert.True(t, result["success"].(bool))
	assert.Equal(t, "Permission added successfully", result["message"])
	mockAuth.AssertExpectations(t)
}

func TestRemoveRolePermission_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Delete("/roles/:role/permissions", h.RemoveRolePermission)

	mockAuth.On("RemovePermissionForRole", "editor", "posts", "write").Return(nil)

	body, _ := json.Marshal(map[string]string{
		"role":   "editor",
		"object": "posts",
		"action": "write",
	})
	req := httptest.NewRequest(http.MethodDelete, "/roles/editor/permissions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponseBody(t, resp)
	assert.True(t, result["success"].(bool))
	assert.Equal(t, "Permission removed successfully", result["message"])
	mockAuth.AssertExpectations(t)
}

func TestGetUserRoles_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Get("/users/:id/roles", h.GetUserRoles)

	mockAuth.On("GetRolesForUser", "user-123").Return([]string{"admin", "editor"}, nil)

	req := httptest.NewRequest(http.MethodGet, "/users/user-123/roles", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponseBody(t, resp)
	assert.True(t, result["success"].(bool))
	data := result["data"].(map[string]any)
	assert.Equal(t, "user-123", data["user_id"])
	roles := data["roles"].([]any)
	assert.Len(t, roles, 2)
}

func TestGetUserPermissions_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Get("/users/:id/permissions", h.GetUserPermissions)

	mockAuth.On("GetImplicitPermissionsForUser", "user-123").Return([][]string{
		{"user-123", "users", "read"},
		{"admin", "posts", "write"},
	}, nil)

	req := httptest.NewRequest(http.MethodGet, "/users/user-123/permissions", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponseBody(t, resp)
	assert.True(t, result["success"].(bool))
	data := result["data"].(map[string]any)
	assert.Equal(t, "user-123", data["user_id"])
	perms := data["permissions"].([]any)
	assert.Len(t, perms, 2)
}

func TestAddRolePermission_ValidationError(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Post("/roles/:role/permissions", h.AddRolePermission)

	// Missing required fields
	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/roles/admin/permissions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestListAllPermissions_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Get("/roles/permissions", h.ListAllPermissions)

	mockAuth.On("GetPermissionsForRole", "superadmin").Return([][]string{
		{"superadmin", "*", "*"},
	}, nil)
	mockAuth.On("GetPermissionsForRole", "admin").Return([][]string{
		{"admin", "users", "read"},
	}, nil)
	mockAuth.On("GetPermissionsForRole", "editor").Return([][]string{}, nil)
	mockAuth.On("GetPermissionsForRole", "viewer").Return([][]string{}, nil)

	req := httptest.NewRequest(http.MethodGet, "/roles/permissions", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponseBody(t, resp)
	assert.True(t, result["success"].(bool))
	data := result["data"].(map[string]any)
	roles := data["roles"].([]any)
	assert.Len(t, roles, 4)
	mockAuth.AssertExpectations(t)
}

func TestAddUserPermission_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Post("/users/:id/permissions", h.AddUserPermission)

	mockAuth.On("AddPermissionForUser", "user-123", "posts", "write").Return(nil)

	body, _ := json.Marshal(map[string]string{
		"object": "posts",
		"action": "write",
	})
	req := httptest.NewRequest(http.MethodPost, "/users/user-123/permissions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponseBody(t, resp)
	assert.True(t, result["success"].(bool))
	assert.Equal(t, "Permission added successfully", result["message"])
	mockAuth.AssertExpectations(t)
}

func TestAddUserPermission_ValidationError(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Post("/users/:id/permissions", h.AddUserPermission)

	// Missing required fields
	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/users/user-123/permissions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestRemoveUserPermission_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Delete("/users/:id/permissions", h.RemoveUserPermission)

	mockAuth.On("RemovePermissionForUser", "user-123", "posts", "write").Return(nil)

	body, _ := json.Marshal(map[string]string{
		"object": "posts",
		"action": "write",
	})
	req := httptest.NewRequest(http.MethodDelete, "/users/user-123/permissions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponseBody(t, resp)
	assert.True(t, result["success"].(bool))
	assert.Equal(t, "Permission removed successfully", result["message"])
	mockAuth.AssertExpectations(t)
}

func TestCheckUserPermission_Allowed(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Get("/users/:id/permissions/check", h.CheckUserPermission)

	mockAuth.On("EnforceWithContext", mock.Anything, "user-123", "users", "read").Return(true, nil)

	req := httptest.NewRequest(http.MethodGet, "/users/user-123/permissions/check?object=users&action=read", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponseBody(t, resp)
	assert.True(t, result["success"].(bool))
	data := result["data"].(map[string]any)
	assert.Equal(t, "user-123", data["user_id"])
	assert.Equal(t, "users", data["object"])
	assert.Equal(t, "read", data["action"])
	assert.True(t, data["allowed"].(bool))
	mockAuth.AssertExpectations(t)
}

func TestCheckUserPermission_Denied(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Get("/users/:id/permissions/check", h.CheckUserPermission)

	mockAuth.On("EnforceWithContext", mock.Anything, "user-123", "users", "delete").Return(false, nil)

	req := httptest.NewRequest(http.MethodGet, "/users/user-123/permissions/check?object=users&action=delete", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	result := parseResponseBody(t, resp)
	assert.True(t, result["success"].(bool))
	data := result["data"].(map[string]any)
	assert.False(t, data["allowed"].(bool))
	mockAuth.AssertExpectations(t)
}

func TestCheckUserPermission_MissingParams(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	app, h := setupTestApp(mockAuth)
	app.Get("/users/:id/permissions/check", h.CheckUserPermission)

	// Missing query params
	req := httptest.NewRequest(http.MethodGet, "/users/user-123/permissions/check", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

// TestRoleHandler_UsesPort verifies that Handler depends on the usecase.UseCase
// interface, not the concrete *roleUseCase. A hand-rolled fake satisfying the
// interface is wired directly — no NewUseCase constructor involved.
func TestRoleHandler_UsesPort(t *testing.T) {
	fake := &stubRoleUseCase{}
	h := NewHandler(fake)
	app := fiber.New()
	app.Get("/roles", h.ListRoles)

	req := httptest.NewRequest(http.MethodGet, "/roles", nil)
	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.True(t, fake.listRolesCalled, "handler must delegate to the UseCase interface")
}

// stubRoleUseCase is a minimal hand-rolled implementation of usecase.UseCase
// used to verify the handler accepts the interface, not the concrete type.
type stubRoleUseCase struct {
	listRolesCalled bool
}

var _ usecase.UseCase = (*stubRoleUseCase)(nil)

func (s *stubRoleUseCase) ListRoles(_ context.Context) []roledto.RoleResponse {
	s.listRolesCalled = true
	return []roledto.RoleResponse{}
}
func (s *stubRoleUseCase) AssignRole(_ context.Context, _, _ string) error { return nil }
func (s *stubRoleUseCase) RemoveRole(_ context.Context, _, _ string) error { return nil }
func (s *stubRoleUseCase) GetRoleUsers(_ context.Context, _ string) (*roledto.RoleUsersResponse, error) {
	return nil, nil
}
func (s *stubRoleUseCase) GetRolePermissions(_ context.Context, _ string) ([]roledto.PermissionResponse, error) {
	return nil, nil
}
func (s *stubRoleUseCase) AddPermissionToRole(_ context.Context, _, _, _ string) error { return nil }
func (s *stubRoleUseCase) RemovePermissionFromRole(_ context.Context, _, _, _ string) error {
	return nil
}
func (s *stubRoleUseCase) GetUserRoles(_ context.Context, _ string) (*roledto.UserRolesResponse, error) {
	return nil, nil
}
func (s *stubRoleUseCase) GetUserPermissions(_ context.Context, _ string) (*roledto.UserPermissionsResponse, error) {
	return nil, nil
}
func (s *stubRoleUseCase) ListAllPermissions(_ context.Context) (*roledto.AllPermissionsResponse, error) {
	return nil, nil
}
func (s *stubRoleUseCase) AddUserPermission(_ context.Context, _, _, _ string) error    { return nil }
func (s *stubRoleUseCase) RemoveUserPermission(_ context.Context, _, _, _ string) error { return nil }
func (s *stubRoleUseCase) CheckPermission(_ context.Context, _, _, _ string) (bool, error) {
	return false, nil
}
