package apperr

import (
	"errors"
	"fmt"
	"net/http"
)

// Error represents an application error with code and message
type Error struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	HTTPStatus int    `json:"-"`
	Err        error  `json:"-"`
}

// Error implements the error interface
func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// Unwrap returns the wrapped error
func (e *Error) Unwrap() error {
	return e.Err
}

// New creates a new application error
func New(code, message string, httpStatus int) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
	}
}

// Wrap wraps an existing error with an application error
func Wrap(err error, code, message string, httpStatus int) *Error {
	return &Error{
		Code:       code,
		Message:    message,
		HTTPStatus: httpStatus,
		Err:        err,
	}
}

// WithMessage returns a copy of the error with a new message
func (e *Error) WithMessage(message string) *Error {
	return &Error{
		Code:       e.Code,
		Message:    message,
		HTTPStatus: e.HTTPStatus,
		Err:        e.Err,
	}
}

// WithError returns a copy of the error with a wrapped error
func (e *Error) WithError(err error) *Error {
	return &Error{
		Code:       e.Code,
		Message:    e.Message,
		HTTPStatus: e.HTTPStatus,
		Err:        err,
	}
}

// Is checks if the target error is an application error with the same code
func (e *Error) Is(target error) bool {
	var appErr *Error
	if errors.As(target, &appErr) {
		return e.Code == appErr.Code
	}
	return false
}

// AsAppError attempts to convert an error to an application error
func AsAppError(err error) (*Error, bool) {
	var appErr *Error
	if errors.As(err, &appErr) {
		return appErr, true
	}
	return nil, false
}

// Common error codes
const (
	CodeBadRequest          = "BAD_REQUEST"
	CodeUnauthorized        = "UNAUTHORIZED"
	CodeForbidden           = "FORBIDDEN"
	CodeNotFound            = "NOT_FOUND"
	CodeConflict            = "CONFLICT"
	CodeUnprocessableEntity = "UNPROCESSABLE_ENTITY"
	CodeInternalError       = "INTERNAL_ERROR"
	CodeServiceUnavailable  = "SERVICE_UNAVAILABLE"
	CodeValidation          = "VALIDATION_ERROR"
)

// Predefined errors
var (
	ErrBadRequest = New(
		CodeBadRequest,
		"The request was invalid or malformed",
		http.StatusBadRequest,
	)

	ErrUnauthorized = New(
		CodeUnauthorized,
		"Authentication is required",
		http.StatusUnauthorized,
	)

	ErrForbidden = New(
		CodeForbidden,
		"You don't have permission to access this resource",
		http.StatusForbidden,
	)

	ErrNotFound = New(
		CodeNotFound,
		"The requested resource was not found",
		http.StatusNotFound,
	)

	ErrConflict = New(
		CodeConflict,
		"The request conflicts with the current state",
		http.StatusConflict,
	)

	ErrUnprocessableEntity = New(
		CodeUnprocessableEntity,
		"The request was well-formed but contains invalid data",
		http.StatusUnprocessableEntity,
	)

	ErrInternal = New(
		CodeInternalError,
		"An internal error occurred",
		http.StatusInternalServerError,
	)

	ErrServiceUnavailable = New(
		CodeServiceUnavailable,
		"The service is temporarily unavailable",
		http.StatusServiceUnavailable,
	)
)

// Validation creates a validation error with the given message
func Validation(message string) *Error {
	return New(CodeValidation, message, http.StatusBadRequest)
}

// NotFoundf creates a not found error with a formatted message
func NotFoundf(format string, args ...any) *Error {
	return New(CodeNotFound, fmt.Sprintf(format, args...), http.StatusNotFound)
}

// BadRequestf creates a bad request error with a formatted message
func BadRequestf(format string, args ...any) *Error {
	return New(CodeBadRequest, fmt.Sprintf(format, args...), http.StatusBadRequest)
}

// Conflictf creates a conflict error with a formatted message
func Conflictf(format string, args ...any) *Error {
	return New(CodeConflict, fmt.Sprintf(format, args...), http.StatusConflict)
}

// Internalf creates an internal error with a formatted message
func Internalf(format string, args ...any) *Error {
	return New(CodeInternalError, fmt.Sprintf(format, args...), http.StatusInternalServerError)
}
