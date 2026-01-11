package handler

import (
	"github.com/14mdzk/goscratch/internal/module/auth/dto"
	"github.com/14mdzk/goscratch/internal/module/auth/usecase"
	"github.com/14mdzk/goscratch/internal/platform/validator"
	"github.com/14mdzk/goscratch/pkg/response"
	"github.com/gofiber/fiber/v2"
)

// Handler handles auth HTTP requests
type Handler struct {
	useCase *usecase.UseCase
}

// NewHandler creates a new auth handler
func NewHandler(useCase *usecase.UseCase) *Handler {
	return &Handler{useCase: useCase}
}

// Login authenticates a user
func (h *Handler) Login(c *fiber.Ctx) error {
	var req dto.LoginRequest
	if err := validator.ValidateAndBind(c, &req); err != nil {
		return validator.HandleValidationError(c, err)
	}

	result, err := h.useCase.Login(c.UserContext(), req)
	if err != nil {
		return response.Fail(c, err)
	}

	return response.Success(c, result)
}

// Refresh refreshes an access token
func (h *Handler) Refresh(c *fiber.Ctx) error {
	var req dto.RefreshRequest
	if err := validator.ValidateAndBind(c, &req); err != nil {
		return validator.HandleValidationError(c, err)
	}

	result, err := h.useCase.Refresh(c.UserContext(), req)
	if err != nil {
		return response.Fail(c, err)
	}

	return response.Success(c, result)
}

// Logout invalidates the refresh token
func (h *Handler) Logout(c *fiber.Ctx) error {
	var req dto.RefreshRequest
	if err := validator.ValidateAndBind(c, &req); err != nil {
		return validator.HandleValidationError(c, err)
	}

	if err := h.useCase.Logout(c.UserContext(), req.RefreshToken); err != nil {
		return response.Fail(c, err)
	}

	return response.Message(c, "Logged out successfully")
}
