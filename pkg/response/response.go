package response

import (
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/gofiber/fiber/v2"
)

// Response represents a standard API response
type Response struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

// Error represents an error in the response
type Error struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Success sends a successful response with data
func Success(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusOK).JSON(Response{
		Success: true,
		Data:    data,
	})
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse struct {
	Success    bool `json:"success"`
	Data       any  `json:"data"`
	Pagination any  `json:"pagination"`
}

// Paginated sends a successful response with data and pagination metadata
// Usage: response.Paginated(c, page.GetItems(), page.GetMeta())
func Paginated(c *fiber.Ctx, data any, pagination any) error {
	return c.Status(fiber.StatusOK).JSON(PaginatedResponse{
		Success:    true,
		Data:       data,
		Pagination: pagination,
	})
}

// Created sends a 201 response with data
func Created(c *fiber.Ctx, data any) error {
	return c.Status(fiber.StatusCreated).JSON(Response{
		Success: true,
		Data:    data,
	})
}

// NoContent sends a 204 response
func NoContent(c *fiber.Ctx) error {
	return c.SendStatus(fiber.StatusNoContent)
}

// Message sends a successful response with a message
func Message(c *fiber.Ctx, message string) error {
	return c.Status(fiber.StatusOK).JSON(Response{
		Success: true,
		Message: message,
	})
}

// Fail sends an error response
func Fail(c *fiber.Ctx, err error) error {
	// Check if it's an application error
	if appErr, ok := apperr.AsAppError(err); ok {
		return c.Status(appErr.HTTPStatus).JSON(Response{
			Success: false,
			Error: &Error{
				Code:    appErr.Code,
				Message: appErr.Message,
			},
		})
	}

	// Default to internal server error
	return c.Status(fiber.StatusInternalServerError).JSON(Response{
		Success: false,
		Error: &Error{
			Code:    apperr.CodeInternalError,
			Message: "An unexpected error occurred",
		},
	})
}

// FailWithDetails sends an error response with additional details
func FailWithDetails(c *fiber.Ctx, err error, details map[string]any) error {
	if appErr, ok := apperr.AsAppError(err); ok {
		return c.Status(appErr.HTTPStatus).JSON(Response{
			Success: false,
			Error: &Error{
				Code:    appErr.Code,
				Message: appErr.Message,
				Details: details,
			},
		})
	}

	return c.Status(fiber.StatusInternalServerError).JSON(Response{
		Success: false,
		Error: &Error{
			Code:    apperr.CodeInternalError,
			Message: "An unexpected error occurred",
			Details: details,
		},
	})
}

// ValidationFailed sends a validation error response
func ValidationFailed(c *fiber.Ctx, errors map[string]string) error {
	details := make(map[string]any, len(errors))
	for k, v := range errors {
		details[k] = v
	}

	return c.Status(fiber.StatusBadRequest).JSON(Response{
		Success: false,
		Error: &Error{
			Code:    apperr.CodeValidation,
			Message: "Validation failed",
			Details: details,
		},
	})
}

// Unauthorized sends a 401 response
func Unauthorized(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Authentication required"
	}
	return c.Status(fiber.StatusUnauthorized).JSON(Response{
		Success: false,
		Error: &Error{
			Code:    apperr.CodeUnauthorized,
			Message: message,
		},
	})
}

// Forbidden sends a 403 response
func Forbidden(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Access denied"
	}
	return c.Status(fiber.StatusForbidden).JSON(Response{
		Success: false,
		Error: &Error{
			Code:    apperr.CodeForbidden,
			Message: message,
		},
	})
}

// NotFound sends a 404 response
func NotFound(c *fiber.Ctx, message string) error {
	if message == "" {
		message = "Resource not found"
	}
	return c.Status(fiber.StatusNotFound).JSON(Response{
		Success: false,
		Error: &Error{
			Code:    apperr.CodeNotFound,
			Message: message,
		},
	})
}
