package usecase

import (
	"context"
	"time"

	userdomain "github.com/14mdzk/goscratch/internal/module/user/domain"
	"github.com/14mdzk/goscratch/internal/module/user/dto"
	"github.com/14mdzk/goscratch/internal/module/user/repository"
	"github.com/14mdzk/goscratch/internal/port"
	shareddomain "github.com/14mdzk/goscratch/internal/shared/domain"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"golang.org/x/crypto/bcrypt"
)

// UseCase handles user business logic
type UseCase struct {
	repo    *repository.Repository
	auditor port.Auditor
}

// NewUseCase creates a new user use case
func NewUseCase(repo *repository.Repository, auditor port.Auditor) *UseCase {
	return &UseCase{
		repo:    repo,
		auditor: auditor,
	}
}

// GetByID retrieves a user by ID
func (uc *UseCase) GetByID(ctx context.Context, id string) (*dto.UserResponse, error) {
	user, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return toUserResponse(user), nil
}

// List retrieves a paginated list of users
func (uc *UseCase) List(ctx context.Context, req dto.ListUsersRequest) (shareddomain.CursorPage[dto.UserResponse], error) {
	limit := shareddomain.NormalizeLimit(req.Limit)

	// Decode cursor if provided
	var cursorID string
	if req.Cursor != "" {
		cursor, err := shareddomain.DecodeCursor(req.Cursor)
		if err != nil {
			return shareddomain.CursorPage[dto.UserResponse]{}, apperr.BadRequestf("invalid cursor")
		}
		if cursor != nil {
			cursorID = cursor.LastID
		}
	}

	// Build filter with all optional parameters
	filter := userdomain.UserFilter{
		Cursor:   cursorID,
		Limit:    limit,
		Search:   req.Search,
		Email:    req.Email,
		IsActive: req.IsActive,
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

	// Create cursor page
	return shareddomain.NewCursorPage(responses, limit, func(u dto.UserResponse) *shareddomain.Cursor {
		return &shareddomain.Cursor{LastID: u.ID}
	}), nil
}

// Create creates a new user
func (uc *UseCase) Create(ctx context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error) {
	// Check if email already exists
	exists, err := uc.repo.ExistsByEmail(ctx, req.Email)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, apperr.Conflictf("user with email %s already exists", req.Email)
	}

	// Hash password
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, apperr.Internalf("failed to hash password")
	}

	// Create user
	user, err := uc.repo.Create(ctx, req.Email, string(passwordHash), req.Name)
	if err != nil {
		return nil, err
	}

	// Audit log
	entry := port.NewAuditEntry(ctx, port.AuditActionCreate, "user", user.ID.String())
	entry.NewValue = map[string]any{
		"email": user.Email,
		"name":  user.Name,
	}
	uc.auditor.Log(ctx, entry)

	return toUserResponse(user), nil
}

// Update updates a user
func (uc *UseCase) Update(ctx context.Context, id string, req dto.UpdateUserRequest) (*dto.UserResponse, error) {
	// Get current user for audit
	oldUser, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Update user
	user, err := uc.repo.Update(ctx, id, req.Name, req.Email)
	if err != nil {
		return nil, err
	}

	// Audit log
	entry := port.NewAuditEntry(ctx, port.AuditActionUpdate, "user", user.ID.String())
	entry.OldValue = map[string]any{"email": oldUser.Email, "name": oldUser.Name}
	entry.NewValue = map[string]any{"email": user.Email, "name": user.Name}
	uc.auditor.Log(ctx, entry)

	return toUserResponse(user), nil
}

// ChangePassword changes a user's password
func (uc *UseCase) ChangePassword(ctx context.Context, id string, req dto.ChangePasswordRequest) error {
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

	// Audit log
	entry := port.NewAuditEntry(ctx, port.AuditActionUpdate, "user", id)
	entry.Metadata = map[string]any{"field": "password"}
	uc.auditor.Log(ctx, entry)

	return nil
}

// Delete soft-deletes a user
func (uc *UseCase) Delete(ctx context.Context, id string) error {
	// Get user for audit
	user, err := uc.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Delete user
	if err := uc.repo.Delete(ctx, id); err != nil {
		return err
	}

	// Audit log
	entry := port.NewAuditEntry(ctx, port.AuditActionDelete, "user", id)
	entry.OldValue = map[string]any{
		"email": user.Email,
		"name":  user.Name,
	}
	uc.auditor.Log(ctx, entry)

	return nil
}

// Activate activates a user
func (uc *UseCase) Activate(ctx context.Context, id string) error {
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

	// Audit log
	entry := port.NewAuditEntry(ctx, port.AuditActionUpdate, "user", id)
	entry.OldValue = map[string]any{"is_active": false}
	entry.NewValue = map[string]any{"is_active": true}
	uc.auditor.Log(ctx, entry)

	return nil
}

// Deactivate deactivates a user
func (uc *UseCase) Deactivate(ctx context.Context, id string) error {
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

	// Audit log
	entry := port.NewAuditEntry(ctx, port.AuditActionUpdate, "user", id)
	entry.OldValue = map[string]any{"is_active": true}
	entry.NewValue = map[string]any{"is_active": false}
	uc.auditor.Log(ctx, entry)

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
