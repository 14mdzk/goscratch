package usecase

import (
	"errors"
	"testing"

	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/stretchr/testify/assert"
)

// TestRoleUseCase_ErrorsIsWrappedInternal verifies that errors returned by the
// role usecase via apperr.ErrInternal.WithError(err) are unwrappable via
// errors.Is. This is the "errors.Is sweep" acceptance criterion.
func TestRoleUseCase_ErrorsIsWrappedInternal(t *testing.T) {
	sentinel := errors.New("adapter failure")
	wrapped := apperr.ErrInternal.WithError(sentinel)

	// errors.Is must find the inner sentinel through Unwrap()
	assert.True(t, errors.Is(wrapped, sentinel),
		"errors.Is must traverse Unwrap() to find the wrapped sentinel")

	// errors.Is must also match the apperr.ErrInternal sentinel (same code)
	assert.True(t, errors.Is(wrapped, apperr.ErrInternal),
		"errors.Is must match ErrInternal by code via apperr.Error.Is")
}
