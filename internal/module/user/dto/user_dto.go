package dto

import "github.com/14mdzk/goscratch/pkg/types"

// CreateUserRequest represents the request to create a user
type CreateUserRequest struct {
	Email    string `json:"email" validate:"required,email,min=1"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required,min=2,max=100"`
}

// UpdateUserRequest represents the request to update a user
type UpdateUserRequest struct {
	Name  string `json:"name" validate:"omitempty,min=2,max=100"`
	Email string `json:"email" validate:"omitempty,email"`
}

// ChangePasswordRequest represents the request to change password
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required,min=1"`
	NewPassword     string `json:"new_password" validate:"required,min=8"`
}

// UserResponse represents the user response
type UserResponse struct {
	ID        string `json:"id"`
	Email     string `json:"email"`
	Name      string `json:"name"`
	IsActive  bool   `json:"is_active"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

// ListUsersRequest represents the request to list users with optional filters
type ListUsersRequest struct {
	// Pagination
	Cursor string `query:"cursor"`
	Limit  int    `query:"limit" validate:"omitempty,min=1,max=100"`

	// Filters (optional) - uses custom Fiber decoder
	Search   types.Opt[string] `query:"search"`    // Search by name or email
	Email    types.Opt[string] `query:"email"`     // Exact email match
	IsActive types.Opt[bool]   `query:"is_active"` // Filter by active status
}
