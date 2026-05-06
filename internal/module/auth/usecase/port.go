package usecase

import (
	"context"

	"github.com/14mdzk/goscratch/internal/module/auth/dto"
)

// UseCase defines the interface for authentication business logic operations.
// Handlers and decorators depend on this interface rather than on the
// concrete *UseCase struct, enabling testability and extensibility.
type UseCase interface {
	Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error)
	Refresh(ctx context.Context, req dto.RefreshRequest) (*dto.RefreshResponse, error)
	// Logout invalidates the refresh token for the authenticated caller.
	// callerID is the user ID extracted from the JWT by the auth middleware.
	Logout(ctx context.Context, callerID, refreshToken string) error
}
