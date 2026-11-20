package usecase

import (
	"context"

	userdomain "github.com/14mdzk/goscratch/internal/module/user/domain"
	"github.com/14mdzk/goscratch/internal/module/user/dto"
	shareddomain "github.com/14mdzk/goscratch/internal/shared/domain"
)

// UserRepo is the narrow repository port that userUseCase depends on.
// *repository.Repository satisfies it; unit tests inject a mock.
type UserRepo interface {
	GetByID(ctx context.Context, id string) (*userdomain.User, error)
	GetByEmail(ctx context.Context, email string) (*userdomain.User, error)
	List(ctx context.Context, filter userdomain.UserFilter) ([]userdomain.User, error)
	Create(ctx context.Context, email, passwordHash, name string) (*userdomain.User, error)
	Update(ctx context.Context, id, name, email string) (*userdomain.User, error)
	UpdatePassword(ctx context.Context, id, passwordHash string) error
	Delete(ctx context.Context, id string) error
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	Activate(ctx context.Context, id string) error
	Deactivate(ctx context.Context, id string) error
}

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

// AuthRevoker is a narrow port for revoking auth sessions. The user module
// depends on this interface rather than on the concrete auth usecase, avoiding
// a circular import. The auth module satisfies this interface via its Revoker()
// accessor.
type AuthRevoker interface {
	// RevokeAllForUser terminates all active refresh tokens for the given user.
	RevokeAllForUser(ctx context.Context, userID string) error
}
