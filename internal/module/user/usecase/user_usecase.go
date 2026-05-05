package usecase

import (
	"context"
	"fmt"
	"time"

	userdomain "github.com/14mdzk/goscratch/internal/module/user/domain"
	"github.com/14mdzk/goscratch/internal/module/user/dto"
	"github.com/14mdzk/goscratch/internal/module/user/repository"
	"github.com/14mdzk/goscratch/internal/platform/database"
	"github.com/14mdzk/goscratch/internal/port"
	shareddomain "github.com/14mdzk/goscratch/internal/shared/domain"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"golang.org/x/crypto/bcrypt"
)

// userRepo is the narrow repository interface that userUseCase depends on.
// The concrete *repository.Repository satisfies it; tests use a mock.
type userRepo interface {
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

// userUseCase handles user business logic
type userUseCase struct {
	repo        userRepo
	transactor  *database.Transactor
	cache       port.Cache
	authRevoker AuthRevoker
}

// NewUseCase creates a new user use case.
// authRevoker is the auth module's session-revocation interface; it may be nil
// in tests that do not exercise ChangePassword revocation.
func NewUseCase(repo *repository.Repository, transactor *database.Transactor, cache port.Cache, authRevoker AuthRevoker) UseCase {
	return newUseCase(repo, transactor, cache, authRevoker)
}

// newUseCase is the internal constructor that accepts the userRepo interface,
// enabling unit tests (same package) to inject mock repositories.
func newUseCase(repo userRepo, transactor *database.Transactor, cache port.Cache, authRevoker AuthRevoker) UseCase {
	return &userUseCase{
		repo:        repo,
		transactor:  transactor,
		cache:       cache,
		authRevoker: authRevoker,
	}
}

// GetByID retrieves a user by ID
func (uc *userUseCase) GetByID(ctx context.Context, id string) (*dto.UserResponse, error) {
	user, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toUserResponse(user), nil
}

// List retrieves a paginated list of users
func (uc *userUseCase) List(ctx context.Context, req dto.ListUsersRequest) (shareddomain.CursorPage[dto.UserResponse], error) {
	limit := shareddomain.NormalizeLimit(req.Limit)

	// Decode cursor if provided
	var cursorID string
	var direction string
	hasCursor := false

	if req.Cursor != "" {
		cursor, err := shareddomain.DecodeCursor(req.Cursor)
		if err != nil {
			return shareddomain.CursorPage[dto.UserResponse]{}, apperr.BadRequestf("invalid cursor")
		}
		if cursor != nil {
			cursorID = cursor.LastID
			direction = string(cursor.Direction)
			hasCursor = true
		}
	}

	// Build filter with all optional parameters
	filter := userdomain.UserFilter{
		Cursor:    cursorID,
		Limit:     limit,
		Direction: direction,
		Search:    req.Search,
		Email:     req.Email,
		IsActive:  req.IsActive,
	}

	users, err := uc.repo.List(ctx, filter)
	if err != nil {
		return shareddomain.CursorPage[dto.UserResponse]{}, err
	}

	// Convert to responses
	responses := make([]dto.UserResponse, 0, len(users))
	for _, u := range users {
		responses = append(responses, *toUserResponse(&u))
	}

	// Create bidirectional cursor page
	return shareddomain.NewBidirectionalCursorPage(responses, limit, direction, hasCursor, func(u dto.UserResponse) *shareddomain.Cursor {
		return &shareddomain.Cursor{LastID: u.ID}
	}), nil
}

// Create creates a new user. The email-existence check and the INSERT are
// executed inside a single transaction so that concurrent requests cannot
// both pass the check and then both insert the same email address.
func (uc *userUseCase) Create(ctx context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error) {
	// Hash the password before entering the transaction — bcrypt is CPU-bound
	// and does not need to hold a DB connection.
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperr.Internalf("failed to hash password")
	}

	var user *userdomain.User

	if err := uc.transactor.WithTx(ctx, func(ctx context.Context) error {
		// Check if email already exists (within the transaction)
		exists, err := uc.repo.ExistsByEmail(ctx, req.Email)
		if err != nil {
			return err
		}
		if exists {
			return apperr.Conflictf("user with email %s already exists", req.Email)
		}

		// Create user (within the same transaction)
		user, err = uc.repo.Create(ctx, req.Email, string(passwordHash), req.Name)
		return err
	}); err != nil {
		return nil, err
	}

	return toUserResponse(user), nil
}

// Update updates a user
func (uc *userUseCase) Update(ctx context.Context, id string, req dto.UpdateUserRequest) (*dto.UserResponse, error) {
	// Update user
	user, err := uc.repo.Update(ctx, id, req.Name, req.Email)
	if err != nil {
		return nil, err
	}

	return toUserResponse(user), nil
}

// ChangePassword changes a user's password and revokes all active refresh
// tokens for that user. Revocation is mandatory: if the cache backend is
// unavailable (ErrCacheUnavailable from NoOpCache or a Redis error) the
// password update is still applied but the error is returned so the caller
// knows sessions were not revoked.
func (uc *userUseCase) ChangePassword(ctx context.Context, id string, req dto.ChangePasswordRequest) error {
	// Get user to verify current password
	user, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Verify current password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		return apperr.ErrUnauthorized.WithMessage("Current password is incorrect")
	}

	// Hash new password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return apperr.Internalf("failed to hash password")
	}

	// Update password
	if err := uc.repo.UpdatePassword(ctx, id, string(passwordHash)); err != nil {
		return err
	}

	// Revoke all active refresh tokens for this user via the auth module's
	// Revoker. The Revoker knows the dual-key cache shape and deletes both the
	// lookup key and the per-user index key for every active session.
	// On NoOpCache this returns ErrCacheUnavailable — propagate it so the
	// caller (audit decorator, handler) knows revocation did not happen.
	if uc.authRevoker != nil {
		if err := uc.authRevoker.RevokeAllForUser(ctx, id); err != nil {
			return fmt.Errorf("password updated but refresh token revocation failed: %w", err)
		}
	}

	return nil
}

// Delete soft-deletes a user
func (uc *userUseCase) Delete(ctx context.Context, id string) error {
	// Delete user
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}

	return nil
}

// Activate activates a user
func (uc *userUseCase) Activate(ctx context.Context, id string) error {
	// Verify user exists
	user, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Already active
	if user.IsActive {
		return nil
	}

	// Activate user
	if err := uc.repo.Activate(ctx, id); err != nil {
		return err
	}

	return nil
}

// Deactivate deactivates a user
func (uc *userUseCase) Deactivate(ctx context.Context, id string) error {
	// Verify user exists
	user, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Already inactive
	if !user.IsActive {
		return nil
	}

	// Deactivate user
	if err := uc.repo.Deactivate(ctx, id); err != nil {
		return err
	}

	return nil
}

// toUserResponse converts a domain user to a response DTO
func toUserResponse(user *userdomain.User) *dto.UserResponse {
	return &dto.UserResponse{
		ID:        user.ID.String(),
		Email:     user.Email,
		Name:      user.Name,
		IsActive:  user.IsActive,
		CreatedAt: user.CreatedAt.Format(time.RFC3339),
		UpdatedAt: user.UpdatedAt.Format(time.RFC3339),
	}
}
