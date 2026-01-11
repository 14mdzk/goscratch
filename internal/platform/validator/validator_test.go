package validator

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testCreateUserRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
	Name     string `json:"name" validate:"required,min=2,max=100"`
}

func TestValidate_EmptyFields(t *testing.T) {
	// Test that empty fields fail validation
	req := testCreateUserRequest{
		Email:    "",
		Password: "",
		Name:     "",
	}

	err := Validate(&req)

	require.NotNil(t, err, "Validation should fail for empty fields")

	ve, ok := IsValidationError(err)
	require.True(t, ok, "Error should be ValidationError")
	require.NotNil(t, ve)

	// Check that all 3 fields have errors
	assert.Contains(t, ve.Errors, "email", "email should have validation error")
	assert.Contains(t, ve.Errors, "password", "password should have validation error")
	assert.Contains(t, ve.Errors, "name", "name should have validation error")

	t.Logf("Validation errors: %v", ve.Errors)
}

func TestValidate_ValidFields(t *testing.T) {
	req := testCreateUserRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "Test User",
	}

	err := Validate(&req)
	assert.Nil(t, err, "Validation should pass for valid fields")
}

func TestValidate_InvalidEmail(t *testing.T) {
	req := testCreateUserRequest{
		Email:    "not-an-email",
		Password: "password123",
		Name:     "Test User",
	}

	err := Validate(&req)
	require.NotNil(t, err)

	ve, ok := IsValidationError(err)
	require.True(t, ok)
	assert.Contains(t, ve.Errors, "email")
}

func TestValidate_ShortPassword(t *testing.T) {
	req := testCreateUserRequest{
		Email:    "test@example.com",
		Password: "short",
		Name:     "Test User",
	}

	err := Validate(&req)
	require.NotNil(t, err)

	ve, ok := IsValidationError(err)
	require.True(t, ok)
	assert.Contains(t, ve.Errors, "password")
}

func TestValidate_ShortName(t *testing.T) {
	req := testCreateUserRequest{
		Email:    "test@example.com",
		Password: "password123",
		Name:     "A",
	}

	err := Validate(&req)
	require.NotNil(t, err)

	ve, ok := IsValidationError(err)
	require.True(t, ok)
	assert.Contains(t, ve.Errors, "name")
}
