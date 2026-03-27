package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
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

func (m *MockAuthorizer) Close() error {
	args := m.Called()
	return args.Error(0)
}

// Ensure MockAuthorizer implements port.Authorizer
var _ port.Authorizer = (*MockAuthorizer)(nil)

func TestAssignRole_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("HasRoleForUser", "user-123", "admin").Return(false, nil)
	mockAuth.On("AddRoleForUser", "user-123", "admin").Return(nil)

	err := uc.AssignRole(ctx, "user-123", "admin")
	assert.NoError(t, err)
	mockAuth.AssertExpectations(t)
}

func TestAssignRole_InvalidRole(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	err := uc.AssignRole(ctx, "user-123", "invalid_role")
	assert.Error(t, err)

	appErr, ok := apperr.AsAppError(err)
	assert.True(t, ok)
	assert.Equal(t, apperr.CodeBadRequest, appErr.Code)
}

func TestAssignRole_AlreadyHasRole(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("HasRoleForUser", "user-123", "admin").Return(true, nil)

	err := uc.AssignRole(ctx, "user-123", "admin")
	assert.Error(t, err)

	appErr, ok := apperr.AsAppError(err)
	assert.True(t, ok)
	assert.Equal(t, apperr.CodeConflict, appErr.Code)
}

func TestRemoveRole_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("HasRoleForUser", "user-123", "editor").Return(true, nil)
	mockAuth.On("RemoveRoleForUser", "user-123", "editor").Return(nil)

	err := uc.RemoveRole(ctx, "user-123", "editor")
	assert.NoError(t, err)
	mockAuth.AssertExpectations(t)
}

func TestRemoveRole_InvalidRole(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	err := uc.RemoveRole(ctx, "user-123", "nonexistent")
	assert.Error(t, err)

	appErr, ok := apperr.AsAppError(err)
	assert.True(t, ok)
	assert.Equal(t, apperr.CodeBadRequest, appErr.Code)
}

func TestRemoveRole_UserDoesNotHaveRole(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("HasRoleForUser", "user-123", "viewer").Return(false, nil)

	err := uc.RemoveRole(ctx, "user-123", "viewer")
	assert.Error(t, err)

	appErr, ok := apperr.AsAppError(err)
	assert.True(t, ok)
	assert.Equal(t, apperr.CodeNotFound, appErr.Code)
}

func TestGetUserRoles_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("GetRolesForUser", "user-123").Return([]string{"admin", "editor"}, nil)

	result, err := uc.GetUserRoles(ctx, "user-123")
	assert.NoError(t, err)
	assert.Equal(t, "user-123", result.UserID)
	assert.Equal(t, []string{"admin", "editor"}, result.Roles)
}

func TestGetUserRoles_Error(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("GetRolesForUser", "user-123").Return([]string{}, errors.New("db error"))

	_, err := uc.GetUserRoles(ctx, "user-123")
	assert.Error(t, err)
}

func TestGetRoleUsers_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("GetUsersForRole", "admin").Return([]string{"user-1", "user-2"}, nil)

	result, err := uc.GetRoleUsers(ctx, "admin")
	assert.NoError(t, err)
	assert.Equal(t, "admin", result.Role)
	assert.Equal(t, []string{"user-1", "user-2"}, result.UserIDs)
}

func TestGetRoleUsers_InvalidRole(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	_, err := uc.GetRoleUsers(ctx, "fake_role")
	assert.Error(t, err)

	appErr, ok := apperr.AsAppError(err)
	assert.True(t, ok)
	assert.Equal(t, apperr.CodeBadRequest, appErr.Code)
}

func TestListRoles(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	roles := uc.ListRoles(ctx)
	assert.Len(t, roles, 4)
	assert.Equal(t, "superadmin", roles[0].Name)
	assert.Equal(t, "admin", roles[1].Name)
	assert.Equal(t, "editor", roles[2].Name)
	assert.Equal(t, "viewer", roles[3].Name)
}

func TestAddPermissionToRole_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("AddPermissionForRole", "admin", "users", "read").Return(nil)

	err := uc.AddPermissionToRole(ctx, "admin", "users", "read")
	assert.NoError(t, err)
	mockAuth.AssertExpectations(t)
}

func TestAddPermissionToRole_InvalidRole(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	err := uc.AddPermissionToRole(ctx, "bogus", "users", "read")
	assert.Error(t, err)

	appErr, ok := apperr.AsAppError(err)
	assert.True(t, ok)
	assert.Equal(t, apperr.CodeBadRequest, appErr.Code)
}

func TestRemovePermissionFromRole_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("RemovePermissionForRole", "editor", "posts", "write").Return(nil)

	err := uc.RemovePermissionFromRole(ctx, "editor", "posts", "write")
	assert.NoError(t, err)
	mockAuth.AssertExpectations(t)
}

func TestGetRolePermissions_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("GetPermissionsForRole", "admin").Return([][]string{
		{"admin", "users", "read"},
		{"admin", "users", "create"},
	}, nil)

	perms, err := uc.GetRolePermissions(ctx, "admin")
	assert.NoError(t, err)
	assert.Len(t, perms, 2)
	assert.Equal(t, "users", perms[0].Object)
	assert.Equal(t, "read", perms[0].Action)
	assert.Equal(t, "users", perms[1].Object)
	assert.Equal(t, "create", perms[1].Action)
}

func TestGetUserPermissions_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("GetImplicitPermissionsForUser", "user-123").Return([][]string{
		{"user-123", "users", "read"},
		{"admin", "posts", "write"},
	}, nil)

	result, err := uc.GetUserPermissions(ctx, "user-123")
	assert.NoError(t, err)
	assert.Equal(t, "user-123", result.UserID)
	assert.Len(t, result.Permissions, 2)
	assert.Equal(t, "users", result.Permissions[0].Object)
	assert.Equal(t, "read", result.Permissions[0].Action)
}

func TestCheckPermission_Allowed(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("EnforceWithContext", ctx, "user-123", "users", "read").Return(true, nil)

	allowed, err := uc.CheckPermission(ctx, "user-123", "users", "read")
	assert.NoError(t, err)
	assert.True(t, allowed)
}

func TestCheckPermission_Denied(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("EnforceWithContext", ctx, "user-123", "users", "delete").Return(false, nil)

	allowed, err := uc.CheckPermission(ctx, "user-123", "users", "delete")
	assert.NoError(t, err)
	assert.False(t, allowed)
}

func TestCheckPermission_Error(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("EnforceWithContext", ctx, "user-123", "users", "read").Return(false, errors.New("enforcer error"))

	_, err := uc.CheckPermission(ctx, "user-123", "users", "read")
	assert.Error(t, err)
}

func TestListAllPermissions_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("GetPermissionsForRole", "superadmin").Return([][]string{
		{"superadmin", "*", "*"},
	}, nil)
	mockAuth.On("GetPermissionsForRole", "admin").Return([][]string{
		{"admin", "users", "read"},
		{"admin", "roles", "read"},
	}, nil)
	mockAuth.On("GetPermissionsForRole", "editor").Return([][]string{
		{"editor", "users", "read"},
	}, nil)
	mockAuth.On("GetPermissionsForRole", "viewer").Return([][]string{}, nil)

	result, err := uc.ListAllPermissions(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Roles, 4)
	assert.Equal(t, "superadmin", result.Roles[0].Role)
	assert.Len(t, result.Roles[0].Permissions, 1)
	assert.Equal(t, "admin", result.Roles[1].Role)
	assert.Len(t, result.Roles[1].Permissions, 2)
	assert.Equal(t, "users", result.Roles[1].Permissions[0].Object)
	assert.Equal(t, "read", result.Roles[1].Permissions[0].Action)
	mockAuth.AssertExpectations(t)
}

func TestListAllPermissions_Error(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("GetPermissionsForRole", "superadmin").Return([][]string{}, errors.New("db error"))

	_, err := uc.ListAllPermissions(ctx)
	assert.Error(t, err)

	appErr, ok := apperr.AsAppError(err)
	assert.True(t, ok)
	assert.Equal(t, apperr.CodeInternalError, appErr.Code)
	mockAuth.AssertExpectations(t)
}

func TestAddUserPermission_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("AddPermissionForUser", "user-123", "posts", "write").Return(nil)

	err := uc.AddUserPermission(ctx, "user-123", "posts", "write")
	assert.NoError(t, err)
	mockAuth.AssertExpectations(t)
}

func TestAddUserPermission_Error(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("AddPermissionForUser", "user-123", "posts", "write").Return(errors.New("adapter error"))

	err := uc.AddUserPermission(ctx, "user-123", "posts", "write")
	assert.Error(t, err)

	appErr, ok := apperr.AsAppError(err)
	assert.True(t, ok)
	assert.Equal(t, apperr.CodeInternalError, appErr.Code)
	mockAuth.AssertExpectations(t)
}

func TestRemoveUserPermission_Success(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("RemovePermissionForUser", "user-123", "posts", "write").Return(nil)

	err := uc.RemoveUserPermission(ctx, "user-123", "posts", "write")
	assert.NoError(t, err)
	mockAuth.AssertExpectations(t)
}

func TestRemoveUserPermission_Error(t *testing.T) {
	mockAuth := new(MockAuthorizer)
	uc := NewUseCase(mockAuth)
	ctx := context.Background()

	mockAuth.On("RemovePermissionForUser", "user-123", "posts", "write").Return(errors.New("adapter error"))

	err := uc.RemoveUserPermission(ctx, "user-123", "posts", "write")
	assert.Error(t, err)

	appErr, ok := apperr.AsAppError(err)
	assert.True(t, ok)
	assert.Equal(t, apperr.CodeInternalError, appErr.Code)
	mockAuth.AssertExpectations(t)
}

func TestToPermissionResponses_EmptySlice(t *testing.T) {
	result := toPermissionResponses([][]string{})
	assert.Empty(t, result)
}

func TestToPermissionResponses_ShortSlice(t *testing.T) {
	// Slices with fewer than 3 elements should be skipped
	result := toPermissionResponses([][]string{
		{"only_one"},
		{"two", "elements"},
	})
	assert.Empty(t, result)
}
