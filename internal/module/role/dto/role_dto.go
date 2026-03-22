package dto

// AssignRoleRequest represents the request to assign a role to a user
type AssignRoleRequest struct {
	UserID string `json:"user_id" validate:"required,uuid"`
	Role   string `json:"role" validate:"required"`
}

// RemoveRoleRequest represents the request to remove a role from a user
type RemoveRoleRequest struct {
	UserID string `json:"user_id" validate:"required,uuid"`
	Role   string `json:"role" validate:"required"`
}

// AddPermissionRequest represents the request to add a permission to a role
type AddPermissionRequest struct {
	Role   string `json:"role" validate:"required"`
	Object string `json:"object" validate:"required"`
	Action string `json:"action" validate:"required"`
}

// RemovePermissionRequest represents the request to remove a permission from a role
type RemovePermissionRequest struct {
	Role   string `json:"role" validate:"required"`
	Object string `json:"object" validate:"required"`
	Action string `json:"action" validate:"required"`
}

// RoleResponse represents a role in the response
type RoleResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// PermissionResponse represents a permission in the response
type PermissionResponse struct {
	Object string `json:"object"`
	Action string `json:"action"`
}

// UserRolesResponse represents a user's roles in the response
type UserRolesResponse struct {
	UserID string   `json:"user_id"`
	Roles  []string `json:"roles"`
}

// RoleUsersResponse represents users assigned to a role
type RoleUsersResponse struct {
	Role    string   `json:"role"`
	UserIDs []string `json:"user_ids"`
}

// UserPermissionsResponse represents a user's permissions in the response
type UserPermissionsResponse struct {
	UserID      string               `json:"user_id"`
	Permissions []PermissionResponse `json:"permissions"`
}
