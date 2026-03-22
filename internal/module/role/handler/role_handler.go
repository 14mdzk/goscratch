package handler

import (
	"github.com/14mdzk/goscratch/internal/module/role/dto"
	"github.com/14mdzk/goscratch/internal/module/role/usecase"
	"github.com/14mdzk/goscratch/internal/platform/validator"
	"github.com/14mdzk/goscratch/pkg/response"
	"github.com/gofiber/fiber/v2"
)

// Handler handles role and permission HTTP requests
type Handler struct {
	useCase *usecase.UseCase
}

// NewHandler creates a new role handler
func NewHandler(useCase *usecase.UseCase) *Handler {
	return &Handler{useCase: useCase}
}

// ListRoles returns all available roles
func (h *Handler) ListRoles(c *fiber.Ctx) error {
	roles := h.useCase.ListRoles(c.UserContext())
	return response.Success(c, roles)
}

// AssignRole assigns a role to a user
func (h *Handler) AssignRole(c *fiber.Ctx) error {
	var req dto.AssignRoleRequest
	if err := validator.ValidateAndBind(c, &req); err != nil {
		return validator.HandleValidationError(c, err)
	}

	if err := h.useCase.AssignRole(c.UserContext(), req.UserID, req.Role); err != nil {
		return response.Fail(c, err)
	}
	return response.Message(c, "Role assigned successfully")
}

// RevokeRole removes a role from a user
func (h *Handler) RevokeRole(c *fiber.Ctx) error {
	var req dto.RemoveRoleRequest
	if err := validator.ValidateAndBind(c, &req); err != nil {
		return validator.HandleValidationError(c, err)
	}

	if err := h.useCase.RemoveRole(c.UserContext(), req.UserID, req.Role); err != nil {
		return response.Fail(c, err)
	}
	return response.Message(c, "Role revoked successfully")
}

// GetRoleUsers returns all users with a specific role
func (h *Handler) GetRoleUsers(c *fiber.Ctx) error {
	role := c.Params("role")
	result, err := h.useCase.GetRoleUsers(c.UserContext(), role)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.Success(c, result)
}

// GetRolePermissions returns all permissions for a role
func (h *Handler) GetRolePermissions(c *fiber.Ctx) error {
	role := c.Params("role")
	perms, err := h.useCase.GetRolePermissions(c.UserContext(), role)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.Success(c, perms)
}

// AddRolePermission adds a permission to a role
func (h *Handler) AddRolePermission(c *fiber.Ctx) error {
	role := c.Params("role")
	var req dto.AddPermissionRequest
	if err := validator.ValidateAndBind(c, &req); err != nil {
		return validator.HandleValidationError(c, err)
	}

	// Override role from path param
	req.Role = role

	if err := h.useCase.AddPermissionToRole(c.UserContext(), req.Role, req.Object, req.Action); err != nil {
		return response.Fail(c, err)
	}
	return response.Message(c, "Permission added successfully")
}

// RemoveRolePermission removes a permission from a role
func (h *Handler) RemoveRolePermission(c *fiber.Ctx) error {
	role := c.Params("role")
	var req dto.RemovePermissionRequest
	if err := validator.ValidateAndBind(c, &req); err != nil {
		return validator.HandleValidationError(c, err)
	}

	// Override role from path param
	req.Role = role

	if err := h.useCase.RemovePermissionFromRole(c.UserContext(), req.Role, req.Object, req.Action); err != nil {
		return response.Fail(c, err)
	}
	return response.Message(c, "Permission removed successfully")
}

// GetUserRoles returns all roles for a user
func (h *Handler) GetUserRoles(c *fiber.Ctx) error {
	userID := c.Params("id")
	result, err := h.useCase.GetUserRoles(c.UserContext(), userID)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.Success(c, result)
}

// GetUserPermissions returns all implicit permissions for a user
func (h *Handler) GetUserPermissions(c *fiber.Ctx) error {
	userID := c.Params("id")
	result, err := h.useCase.GetUserPermissions(c.UserContext(), userID)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.Success(c, result)
}
