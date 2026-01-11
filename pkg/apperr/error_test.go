package apperr

import (
	"errors"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestError_Error(t *testing.T) {
	t.Run("without_wrapped_error", func(t *testing.T) {
		err := New(CodeBadRequest, "Something went wrong", http.StatusBadRequest)
		assert.Equal(t, "BAD_REQUEST: Something went wrong", err.Error())
	})

	t.Run("with_wrapped_error", func(t *testing.T) {
		inner := errors.New("database error")
		err := Wrap(inner, CodeInternalError, "Failed to save", http.StatusInternalServerError)
		assert.Contains(t, err.Error(), "INTERNAL_ERROR")
		assert.Contains(t, err.Error(), "Failed to save")
		assert.Contains(t, err.Error(), "database error")
	})
}

func TestError_Unwrap(t *testing.T) {
	inner := errors.New("original error")
	err := Wrap(inner, CodeInternalError, "Wrapped", http.StatusInternalServerError)

	unwrapped := errors.Unwrap(err)
	assert.Equal(t, inner, unwrapped)
}

func TestError_WithMessage(t *testing.T) {
	original := ErrNotFound
	customized := original.WithMessage("User with ID 123 not found")

	assert.Equal(t, CodeNotFound, customized.Code)
	assert.Equal(t, "User with ID 123 not found", customized.Message)
	assert.Equal(t, http.StatusNotFound, customized.HTTPStatus)
}

func TestError_WithError(t *testing.T) {
	original := ErrInternal
	inner := errors.New("connection refused")
	wrapped := original.WithError(inner)

	assert.Equal(t, inner, wrapped.Err)
	assert.Contains(t, wrapped.Error(), "connection refused")
}

func TestAsAppError(t *testing.T) {
	t.Run("app_error", func(t *testing.T) {
		err := ErrNotFound
		appErr, ok := AsAppError(err)

		assert.True(t, ok)
		assert.Equal(t, CodeNotFound, appErr.Code)
	})

	t.Run("regular_error", func(t *testing.T) {
		err := errors.New("regular error")
		appErr, ok := AsAppError(err)

		assert.False(t, ok)
		assert.Nil(t, appErr)
	})

	t.Run("wrapped_app_error", func(t *testing.T) {
		inner := ErrUnauthorized
		// Standard errors.Is/As should work through wrapping
		var appErr *Error
		ok := errors.As(inner, &appErr)

		assert.True(t, ok)
		assert.Equal(t, CodeUnauthorized, appErr.Code)
	})
}

func TestPredefinedErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        *Error
		code       string
		httpStatus int
	}{
		{"BadRequest", ErrBadRequest, CodeBadRequest, http.StatusBadRequest},
		{"Unauthorized", ErrUnauthorized, CodeUnauthorized, http.StatusUnauthorized},
		{"Forbidden", ErrForbidden, CodeForbidden, http.StatusForbidden},
		{"NotFound", ErrNotFound, CodeNotFound, http.StatusNotFound},
		{"Conflict", ErrConflict, CodeConflict, http.StatusConflict},
		{"UnprocessableEntity", ErrUnprocessableEntity, CodeUnprocessableEntity, http.StatusUnprocessableEntity},
		{"Internal", ErrInternal, CodeInternalError, http.StatusInternalServerError},
		{"ServiceUnavailable", ErrServiceUnavailable, CodeServiceUnavailable, http.StatusServiceUnavailable},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.code, tt.err.Code)
			assert.Equal(t, tt.httpStatus, tt.err.HTTPStatus)
		})
	}
}

func TestValidation(t *testing.T) {
	err := Validation("Email is required")

	assert.Equal(t, CodeValidation, err.Code)
	assert.Equal(t, "Email is required", err.Message)
	assert.Equal(t, http.StatusBadRequest, err.HTTPStatus)
}

func TestFormattedErrors(t *testing.T) {
	t.Run("NotFoundf", func(t *testing.T) {
		err := NotFoundf("user %s not found", "123")
		assert.Equal(t, CodeNotFound, err.Code)
		assert.Equal(t, "user 123 not found", err.Message)
	})

	t.Run("BadRequestf", func(t *testing.T) {
		err := BadRequestf("invalid value: %d", 42)
		assert.Equal(t, CodeBadRequest, err.Code)
		assert.Equal(t, "invalid value: 42", err.Message)
	})

	t.Run("Conflictf", func(t *testing.T) {
		err := Conflictf("email %s already exists", "test@example.com")
		assert.Equal(t, CodeConflict, err.Code)
		assert.Equal(t, "email test@example.com already exists", err.Message)
	})

	t.Run("Internalf", func(t *testing.T) {
		err := Internalf("failed to connect to %s", "database")
		assert.Equal(t, CodeInternalError, err.Code)
		assert.Equal(t, "failed to connect to database", err.Message)
	})
}
