package usecase

import (
	"context"
	"time"

	userdomain "github.com/14mdzk/goscratch/internal/module/user/domain"
	"github.com/14mdzk/goscratch/internal/module/user/dto"
	"github.com/14mdzk/goscratch/internal/module/user/repository"
	"github.com/14mdzk/goscratch/internal/platform/database"
	shareddomain "github.com/14mdzk/goscratch/internal/shared/domain"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"golang.org/x/crypto/bcrypt"
)

// userUseCase handles user business logic
type userUseCase struct {
	repo       *repository.Repository
	transactor *database.Transactor
}

// NewUseCase creates a new user use case
func NewUseCase(repo *repository.Repository, transactor *database.Transactor) UseCase {
	return &userUseCase{
		repo:       repo,
		transactor: transactor,
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

// ChangePassword changes a user's password
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
