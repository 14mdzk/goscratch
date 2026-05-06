package handler

import (
	"github.com/14mdzk/goscratch/internal/module/auth/dto"
	"github.com/14mdzk/goscratch/internal/module/auth/usecase"
	"github.com/14mdzk/goscratch/internal/platform/http/middleware"
	"github.com/14mdzk/goscratch/internal/platform/validator"
	"github.com/14mdzk/goscratch/pkg/response"
	"github.com/gofiber/fiber/v2"
)

// Handler handles auth HTTP requests
type Handler struct {
	useCase usecase.UseCase
}

// NewHandler creates a new auth handler
func NewHandler(useCase usecase.UseCase) *Handler {
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

// Refresh refreshes an access token.
// The request body must include both user_id and refresh_token so the server
// can derive the per-user cache key.
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

// Logout invalidates the refresh token.
// This handler requires the Auth middleware (applied in module.go); the caller
// ID is taken from the JWT claims, not from the request body.
func (h *Handler) Logout(c *fiber.Ctx) error {
	callerID := middleware.GetUserID(c)
	if callerID == "" {
		return response.Unauthorized(c, "Missing or invalid token")
	}

	var req dto.LogoutRequest
	if err := validator.ValidateAndBind(c, &req); err != nil {
		return validator.HandleValidationError(c, err)
	}

	if err := h.useCase.Logout(c.UserContext(), callerID, req.RefreshToken); err != nil {
		return response.Fail(c, err)
	}

	return response.Message(c, "Logged out successfully")
}
