package usecase

import (
	"context"

	"github.com/14mdzk/goscratch/internal/module/role/dto"
)

// UseCase defines the interface for role and permission business logic operations.
// Handlers and decorators depend on this interface rather than on the concrete
// *roleUseCase struct, enabling testability and extensibility.
type UseCase interface {
	ListRoles(ctx context.Context) []dto.RoleResponse
	AssignRole(ctx context.Context, userID, role string) error
	RemoveRole(ctx context.Context, userID, role string) error
	GetRoleUsers(ctx context.Context, role string) (*dto.RoleUsersResponse, error)
	GetRolePermissions(ctx context.Context, role string) ([]dto.PermissionResponse, error)
	AddPermissionToRole(ctx context.Context, role, object, action string) error
	RemovePermissionFromRole(ctx context.Context, role, object, action string) error
	GetUserRoles(ctx context.Context, userID string) (*dto.UserRolesResponse, error)
	GetUserPermissions(ctx context.Context, userID string) (*dto.UserPermissionsResponse, error)
	ListAllPermissions(ctx context.Context) (*dto.AllPermissionsResponse, error)
	AddUserPermission(ctx context.Context, userID, object, action string) error
	RemoveUserPermission(ctx context.Context, userID, object, action string) error
	CheckPermission(ctx context.Context, userID, object, action string) (bool, error)
}
