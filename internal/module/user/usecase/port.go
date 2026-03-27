package usecase

import (
	"context"

	"github.com/14mdzk/goscratch/internal/module/user/dto"
	shareddomain "github.com/14mdzk/goscratch/internal/shared/domain"
)

// UseCase defines the interface for user business logic operations.
// Handlers and decorators depend on this interface rather than on the
// concrete *UseCase struct, enabling testability and extensibility.
type UseCase interface {
	GetByID(ctx context.Context, id string) (*dto.UserResponse, error)
	List(ctx context.Context, req dto.ListUsersRequest) (shareddomain.CursorPage[dto.UserResponse], error)
	Create(ctx context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error)
	Update(ctx context.Context, id string, req dto.UpdateUserRequest) (*dto.UserResponse, error)
	ChangePassword(ctx context.Context, id string, req dto.ChangePasswordRequest) error
	Delete(ctx context.Context, id string) error
	Activate(ctx context.Context, id string) error
	Deactivate(ctx context.Context, id string) error
}
