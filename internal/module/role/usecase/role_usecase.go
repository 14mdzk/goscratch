package usecase

import (
	"context"

	"github.com/14mdzk/goscratch/internal/module/role/domain"
	"github.com/14mdzk/goscratch/internal/module/role/dto"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
)

// roleUseCase handles role and permission business logic.
// Returned via the UseCase interface; the concrete type is unexported so
// callers depend on the interface (enables the audit decorator pattern).
type roleUseCase struct {
	authorizer port.Authorizer
}

// compile-time assertion that roleUseCase satisfies UseCase.
var _ UseCase = (*roleUseCase)(nil)

// NewUseCase creates a new role use case.
func NewUseCase(authorizer port.Authorizer) UseCase {
	return &roleUseCase{
		authorizer: authorizer,
	}
}

// AssignRole assigns a role to a user
func (uc *roleUseCase) AssignRole(ctx context.Context, userID, role string) error {
	if !domain.IsValidRole(role) {
		return apperr.BadRequestf("invalid role: %s", role)
	}

	// Check if user already has this role
	hasRole, err := uc.authorizer.HasRoleForUser(userID, role)
	if err != nil {
		return apperr.ErrInternal.WithError(err)
	}
	if hasRole {
		return apperr.Conflictf("user already has role %s", role)
	}

	if err := uc.authorizer.AddRoleForUser(userID, role); err != nil {
		return apperr.ErrInternal.WithError(err)
	}

	return nil
}

// RemoveRole removes a role from a user
func (uc *roleUseCase) RemoveRole(ctx context.Context, userID, role string) error {
	if !domain.IsValidRole(role) {
		return apperr.BadRequestf("invalid role: %s", role)
	}

	// Check if user has this role
	hasRole, err := uc.authorizer.HasRoleForUser(userID, role)
	if err != nil {
		return apperr.ErrInternal.WithError(err)
	}
	if !hasRole {
		return apperr.NotFoundf("user does not have role %s", role)
	}

	if err := uc.authorizer.RemoveRoleForUser(userID, role); err != nil {
		return apperr.ErrInternal.WithError(err)
	}

	return nil
}

// GetUserRoles returns all roles for a user
func (uc *roleUseCase) GetUserRoles(ctx context.Context, userID string) (*dto.UserRolesResponse, error) {
	roles, err := uc.authorizer.GetRolesForUser(userID)
	if err != nil {
		return nil, apperr.ErrInternal.WithError(err)
	}

	return &dto.UserRolesResponse{
		UserID: userID,
		Roles:  roles,
	}, nil
}

// GetRoleUsers returns all users with a specific role
func (uc *roleUseCase) GetRoleUsers(ctx context.Context, role string) (*dto.RoleUsersResponse, error) {
	if !domain.IsValidRole(role) {
		return nil, apperr.BadRequestf("invalid role: %s", role)
	}

	userIDs, err := uc.authorizer.GetUsersForRole(role)
	if err != nil {
		return nil, apperr.ErrInternal.WithError(err)
	}

	return &dto.RoleUsersResponse{
		Role:    role,
		UserIDs: userIDs,
	}, nil
}

// ListRoles returns all predefined roles
func (uc *roleUseCase) ListRoles(ctx context.Context) []dto.RoleResponse {
	roles := make([]dto.RoleResponse, 0, len(domain.PredefinedRoles))
	for _, r := range domain.PredefinedRoles {
		roles = append(roles, dto.RoleResponse{
			Name:        r.Name,
			Description: r.Description,
		})
	}
	return roles
}

// AddPermissionToRole adds a permission to a role
func (uc *roleUseCase) AddPermissionToRole(ctx context.Context, role, object, action string) error {
	if !domain.IsValidRole(role) {
		return apperr.BadRequestf("invalid role: %s", role)
	}

	if err := uc.authorizer.AddPermissionForRole(role, object, action); err != nil {
		return apperr.ErrInternal.WithError(err)
	}

	return nil
}

// RemovePermissionFromRole removes a permission from a role
func (uc *roleUseCase) RemovePermissionFromRole(ctx context.Context, role, object, action string) error {
	if !domain.IsValidRole(role) {
		return apperr.BadRequestf("invalid role: %s", role)
	}

	if err := uc.authorizer.RemovePermissionForRole(role, object, action); err != nil {
		return apperr.ErrInternal.WithError(err)
	}

	return nil
}

// GetRolePermissions returns all permissions for a role
func (uc *roleUseCase) GetRolePermissions(ctx context.Context, role string) ([]dto.PermissionResponse, error) {
	if !domain.IsValidRole(role) {
		return nil, apperr.BadRequestf("invalid role: %s", role)
	}

	perms, err := uc.authorizer.GetPermissionsForRole(role)
	if err != nil {
		return nil, apperr.ErrInternal.WithError(err)
	}

	return toPermissionResponses(perms), nil
}

// GetUserPermissions returns all implicit permissions for a user (including via roles)
func (uc *roleUseCase) GetUserPermissions(ctx context.Context, userID string) (*dto.UserPermissionsResponse, error) {
	perms, err := uc.authorizer.GetImplicitPermissionsForUser(userID)
	if err != nil {
		return nil, apperr.ErrInternal.WithError(err)
	}

	return &dto.UserPermissionsResponse{
		UserID:      userID,
		Permissions: toPermissionResponses(perms),
	}, nil
}

// CheckPermission checks if a user has a specific permission
func (uc *roleUseCase) CheckPermission(ctx context.Context, userID, object, action string) (bool, error) {
	allowed, err := uc.authorizer.EnforceWithContext(ctx, userID, object, action)
	if err != nil {
		return false, apperr.ErrInternal.WithError(err)
	}
	return allowed, nil
}

// ListAllPermissions returns all permissions grouped by predefined role
func (uc *roleUseCase) ListAllPermissions(ctx context.Context) (*dto.AllPermissionsResponse, error) {
	entries := make([]dto.RolePermissionsEntry, 0, len(domain.PredefinedRoles))
	for _, r := range domain.PredefinedRoles {
		perms, err := uc.authorizer.GetPermissionsForRole(r.Name)
		if err != nil {
			return nil, apperr.ErrInternal.WithError(err)
		}

		permEntries := make([]dto.PermissionEntry, 0, len(perms))
		for _, p := range perms {
			if len(p) >= 3 {
				permEntries = append(permEntries, dto.PermissionEntry{
					Object: p[1],
					Action: p[2],
				})
			}
		}

		entries = append(entries, dto.RolePermissionsEntry{
			Role:        r.Name,
			Permissions: permEntries,
		})
	}

	return &dto.AllPermissionsResponse{Roles: entries}, nil
}

// AddUserPermission adds a direct permission to a user (bypassing roles)
func (uc *roleUseCase) AddUserPermission(ctx context.Context, userID, object, action string) error {
	if err := uc.authorizer.AddPermissionForUser(userID, object, action); err != nil {
		return apperr.ErrInternal.WithError(err)
	}
	return nil
}

// RemoveUserPermission removes a direct permission from a user
func (uc *roleUseCase) RemoveUserPermission(ctx context.Context, userID, object, action string) error {
	if err := uc.authorizer.RemovePermissionForUser(userID, object, action); err != nil {
		return apperr.ErrInternal.WithError(err)
	}
	return nil
}

// toPermissionResponses converts raw permission slices to PermissionResponse DTOs
// Each permission slice is expected to be [subject, object, action]
func toPermissionResponses(perms [][]string) []dto.PermissionResponse {
	responses := make([]dto.PermissionResponse, 0, len(perms))
	for _, p := range perms {
		if len(p) >= 3 {
			responses = append(responses, dto.PermissionResponse{
				Object: p[1],
				Action: p[2],
			})
		}
	}
	return responses
}
