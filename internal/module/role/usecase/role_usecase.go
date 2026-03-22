package usecase

import (
	"context"

	"github.com/14mdzk/goscratch/internal/module/role/domain"
	"github.com/14mdzk/goscratch/internal/module/role/dto"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
)

// UseCase handles role and permission business logic
type UseCase struct {
	authorizer port.Authorizer
}

// NewUseCase creates a new role use case
func NewUseCase(authorizer port.Authorizer) *UseCase {
	return &UseCase{
		authorizer: authorizer,
	}
}

// AssignRole assigns a role to a user
func (uc *UseCase) AssignRole(ctx context.Context, userID, role string) error {
	if !domain.IsValidRole(role) {
		return apperr.BadRequestf("invalid role: %s", role)
	}

	// Check if user already has this role
	hasRole, err := uc.authorizer.HasRoleForUser(userID, role)
	if err != nil {
		return apperr.Internalf("failed to check user role: %s", err.Error())
	}
	if hasRole {
		return apperr.Conflictf("user already has role %s", role)
	}

	if err := uc.authorizer.AddRoleForUser(userID, role); err != nil {
		return apperr.Internalf("failed to assign role: %s", err.Error())
	}

	return nil
}

// RemoveRole removes a role from a user
func (uc *UseCase) RemoveRole(ctx context.Context, userID, role string) error {
	if !domain.IsValidRole(role) {
		return apperr.BadRequestf("invalid role: %s", role)
	}

	// Check if user has this role
	hasRole, err := uc.authorizer.HasRoleForUser(userID, role)
	if err != nil {
		return apperr.Internalf("failed to check user role: %s", err.Error())
	}
	if !hasRole {
		return apperr.NotFoundf("user does not have role %s", role)
	}

	if err := uc.authorizer.RemoveRoleForUser(userID, role); err != nil {
		return apperr.Internalf("failed to remove role: %s", err.Error())
	}

	return nil
}

// GetUserRoles returns all roles for a user
func (uc *UseCase) GetUserRoles(ctx context.Context, userID string) (*dto.UserRolesResponse, error) {
	roles, err := uc.authorizer.GetRolesForUser(userID)
	if err != nil {
		return nil, apperr.Internalf("failed to get user roles: %s", err.Error())
	}

	return &dto.UserRolesResponse{
		UserID: userID,
		Roles:  roles,
	}, nil
}

// GetRoleUsers returns all users with a specific role
func (uc *UseCase) GetRoleUsers(ctx context.Context, role string) (*dto.RoleUsersResponse, error) {
	if !domain.IsValidRole(role) {
		return nil, apperr.BadRequestf("invalid role: %s", role)
	}

	userIDs, err := uc.authorizer.GetUsersForRole(role)
	if err != nil {
		return nil, apperr.Internalf("failed to get role users: %s", err.Error())
	}

	return &dto.RoleUsersResponse{
		Role:    role,
		UserIDs: userIDs,
	}, nil
}

// ListRoles returns all predefined roles
func (uc *UseCase) ListRoles(ctx context.Context) []dto.RoleResponse {
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
func (uc *UseCase) AddPermissionToRole(ctx context.Context, role, object, action string) error {
	if !domain.IsValidRole(role) {
		return apperr.BadRequestf("invalid role: %s", role)
	}

	if err := uc.authorizer.AddPermissionForRole(role, object, action); err != nil {
		return apperr.Internalf("failed to add permission: %s", err.Error())
	}

	return nil
}

// RemovePermissionFromRole removes a permission from a role
func (uc *UseCase) RemovePermissionFromRole(ctx context.Context, role, object, action string) error {
	if !domain.IsValidRole(role) {
		return apperr.BadRequestf("invalid role: %s", role)
	}

	if err := uc.authorizer.RemovePermissionForRole(role, object, action); err != nil {
		return apperr.Internalf("failed to remove permission: %s", err.Error())
	}

	return nil
}

// GetRolePermissions returns all permissions for a role
func (uc *UseCase) GetRolePermissions(ctx context.Context, role string) ([]dto.PermissionResponse, error) {
	if !domain.IsValidRole(role) {
		return nil, apperr.BadRequestf("invalid role: %s", role)
	}

	perms, err := uc.authorizer.GetPermissionsForRole(role)
	if err != nil {
		return nil, apperr.Internalf("failed to get role permissions: %s", err.Error())
	}

	return toPermissionResponses(perms), nil
}

// GetUserPermissions returns all implicit permissions for a user (including via roles)
func (uc *UseCase) GetUserPermissions(ctx context.Context, userID string) (*dto.UserPermissionsResponse, error) {
	perms, err := uc.authorizer.GetImplicitPermissionsForUser(userID)
	if err != nil {
		return nil, apperr.Internalf("failed to get user permissions: %s", err.Error())
	}

	return &dto.UserPermissionsResponse{
		UserID:      userID,
		Permissions: toPermissionResponses(perms),
	}, nil
}

// CheckPermission checks if a user has a specific permission
func (uc *UseCase) CheckPermission(ctx context.Context, userID, object, action string) (bool, error) {
	allowed, err := uc.authorizer.EnforceWithContext(ctx, userID, object, action)
	if err != nil {
		return false, apperr.Internalf("failed to check permission: %s", err.Error())
	}
	return allowed, nil
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
