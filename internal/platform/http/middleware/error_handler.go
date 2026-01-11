package middleware

import (
	"github.com/gofiber/fiber/v2"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/14mdzk/goscratch/pkg/response"
)

// ErrorHandler returns a centralized error handling middleware
func ErrorHandler(log *logger.Logger) fiber.ErrorHandler {
	return func(c *fiber.Ctx, err error) error {
		// Check if it's an application error
		if appErr, ok := apperr.AsAppError(err); ok {
			// Log server errors
			if appErr.HTTPStatus >= 500 {
				log.WithContext(c.UserContext()).WithError(err).Error("Server error occurred")
			}
			return response.Fail(c, appErr)
		}

		// Check if it's a Fiber error
		if fiberErr, ok := err.(*fiber.Error); ok {
			return c.Status(fiberErr.Code).JSON(fiber.Map{
				"success": false,
				"error": fiber.Map{
					"code":    fiberStatusToCode(fiberErr.Code),
					"message": fiberErr.Message,
				},
			})
		}

		// Unknown error - log and return generic error
		log.WithContext(c.UserContext()).WithError(err).Error("Unexpected error occurred")
		return response.Fail(c, apperr.ErrInternal)
	}
}

// fiberStatusToCode maps HTTP status codes to error codes
func fiberStatusToCode(status int) string {
	switch status {
	case fiber.StatusBadRequest:
		return apperr.CodeBadRequest
	case fiber.StatusUnauthorized:
		return apperr.CodeUnauthorized
	case fiber.StatusForbidden:
		return apperr.CodeForbidden
	case fiber.StatusNotFound:
		return apperr.CodeNotFound
	case fiber.StatusConflict:
		return apperr.CodeConflict
	case fiber.StatusUnprocessableEntity:
		return apperr.CodeUnprocessableEntity
	default:
		return apperr.CodeInternalError
	}
}
