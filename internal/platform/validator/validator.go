package validator

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/14mdzk/goscratch/pkg/response"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
)

var (
	validate *validator.Validate
	once     sync.Once
)

// ValidationError is a custom error type for validation errors
type ValidationError struct {
	Errors map[string]string
}

func (e *ValidationError) Error() string {
	return "validation failed"
}

// IsValidationError checks if an error is a ValidationError
func IsValidationError(err error) (*ValidationError, bool) {
	var ve *ValidationError
	if errors.As(err, &ve) {
		return ve, true
	}
	return nil, false
}

// Get returns a singleton validator instance
func Get() *validator.Validate {
	once.Do(func() {
		validate = validator.New(validator.WithRequiredStructEnabled())

		// Register custom tag name function to use json tags
		validate.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})

		// Register custom validations here
		// Example:
		// validate.RegisterValidation("custom_rule", customRuleFunc)
	})

	return validate
}

// ValidateStruct validates a struct and returns validation errors
func ValidateStruct(s interface{}) map[string]string {
	errs := make(map[string]string)

	if err := Get().Struct(s); err != nil {
		for _, err := range err.(validator.ValidationErrors) {
			errs[err.Field()] = formatError(err)
		}
	}

	return errs
}

// Validate validates a struct and returns a ValidationError if invalid
func Validate(s interface{}) error {
	errs := ValidateStruct(s)
	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

// ValidateAndBind parses the request body and validates the struct
// Returns a ValidationError that the handler should convert to a response
func ValidateAndBind(c *fiber.Ctx, s interface{}) error {
	// Parse body
	if err := c.BodyParser(s); err != nil {
		return &ValidationError{
			Errors: map[string]string{"body": "Invalid request body"},
		}
	}

	// Validate
	return Validate(s)
}

// ValidateQuery parses query parameters and validates the struct
func ValidateQuery(c *fiber.Ctx, s interface{}) error {
	if err := c.QueryParser(s); err != nil {
		return &ValidationError{
			Errors: map[string]string{"query": "Invalid query parameters"},
		}
	}

	return Validate(s)
}

// HandleValidationError is a helper to convert ValidationError to HTTP response
func HandleValidationError(c *fiber.Ctx, err error) error {
	if ve, ok := IsValidationError(err); ok {
		return response.ValidationFailed(c, ve.Errors)
	}
	// Not a validation error, return as app error
	return response.Fail(c, apperr.BadRequestf("%s", err.Error()))
}

// formatError formats a validation error into a human-readable message
func formatError(err validator.FieldError) string {
	field := err.Field()

	switch err.Tag() {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email address", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, err.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, err.Param())
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", field, err.Param())
	case "gte":
		return fmt.Sprintf("%s must be greater than or equal to %s", field, err.Param())
	case "lte":
		return fmt.Sprintf("%s must be less than or equal to %s", field, err.Param())
	case "gt":
		return fmt.Sprintf("%s must be greater than %s", field, err.Param())
	case "lt":
		return fmt.Sprintf("%s must be less than %s", field, err.Param())
	case "oneof":
		return fmt.Sprintf("%s must be one of: %s", field, err.Param())
	case "uuid":
		return fmt.Sprintf("%s must be a valid UUID", field)
	case "url":
		return fmt.Sprintf("%s must be a valid URL", field)
	case "alphanum":
		return fmt.Sprintf("%s must contain only alphanumeric characters", field)
	case "numeric":
		return fmt.Sprintf("%s must be numeric", field)
	case "eqfield":
		return fmt.Sprintf("%s must be equal to %s", field, err.Param())
	default:
		return fmt.Sprintf("%s is invalid", field)
	}
}

// RegisterCustomValidation registers a custom validation function
func RegisterCustomValidation(tag string, fn validator.Func) error {
	return Get().RegisterValidation(tag, fn)
}
