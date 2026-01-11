package handler

import (
	"github.com/14mdzk/goscratch/internal/module/user/dto"
	"github.com/14mdzk/goscratch/internal/module/user/usecase"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/platform/validator"
	"github.com/14mdzk/goscratch/pkg/response"
	"github.com/gofiber/fiber/v2"
)

// Handler handles user HTTP requests
type Handler struct {
	useCase *usecase.UseCase
}

// NewHandler creates a new user handler
func NewHandler(useCase *usecase.UseCase) *Handler {
	return &Handler{useCase: useCase}
}

// GetByID retrieves a user by ID
func (h *Handler) GetByID(c *fiber.Ctx) error {
	id := c.Params("id")
	user, err := h.useCase.GetByID(c.UserContext(), id)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.Success(c, user)
}

// List retrieves a paginated list of users
func (h *Handler) List(c *fiber.Ctx) error {
	var req dto.ListUsersRequest
	if err := validator.ValidateQuery(c, &req); err != nil {
		return validator.HandleValidationError(c, err)
	}

	result, err := h.useCase.List(c.UserContext(), req)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.Success(c, result)
}

// Create creates a new user
func (h *Handler) Create(c *fiber.Ctx) error {
	var req dto.CreateUserRequest
	if err := validator.ValidateAndBind(c, &req); err != nil {
		// Validation failed - return error response
		return validator.HandleValidationError(c, err)
	}

	// If we get here, validation passed
	user, err := h.useCase.Create(c.UserContext(), req)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.Created(c, user)
}

// Update updates a user
func (h *Handler) Update(c *fiber.Ctx) error {
	id := c.Params("id")
	var req dto.UpdateUserRequest
	if err := validator.ValidateAndBind(c, &req); err != nil {
		return validator.HandleValidationError(c, err)
	}

	user, err := h.useCase.Update(c.UserContext(), id, req)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.Success(c, user)
}

// ChangePassword changes the current user's password
func (h *Handler) ChangePassword(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return response.Unauthorized(c, "")
	}

	var req dto.ChangePasswordRequest
	if err := validator.ValidateAndBind(c, &req); err != nil {
		return validator.HandleValidationError(c, err)
	}

	if err := h.useCase.ChangePassword(c.UserContext(), userID, req); err != nil {
		return response.Fail(c, err)
	}
	return response.Message(c, "Password changed successfully")
}

// Delete soft-deletes a user
func (h *Handler) Delete(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.useCase.Delete(c.UserContext(), id); err != nil {
		return response.Fail(c, err)
	}
	return response.NoContent(c)
}

// GetMe retrieves the current user's profile
func (h *Handler) GetMe(c *fiber.Ctx) error {
	userID := middleware.GetUserID(c)
	if userID == "" {
		return response.Unauthorized(c, "")
	}

	user, err := h.useCase.GetByID(c.UserContext(), userID)
	if err != nil {
		return response.Fail(c, err)
	}
	return response.Success(c, user)
}

// Activate activates a user
func (h *Handler) Activate(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.useCase.Activate(c.UserContext(), id); err != nil {
		return response.Fail(c, err)
	}
	return response.Message(c, "User activated successfully")
}

// Deactivate deactivates a user
func (h *Handler) Deactivate(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.useCase.Deactivate(c.UserContext(), id); err != nil {
		return response.Fail(c, err)
	}
	return response.Message(c, "User deactivated successfully")
}
