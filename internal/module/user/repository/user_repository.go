package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/14mdzk/goscratch/internal/module/user/domain"
	"github.com/14mdzk/goscratch/internal/module/user/repository/sqlc"
	"github.com/14mdzk/goscratch/internal/platform/observability"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/14mdzk/goscratch/pkg/pgutil"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles user data access using SQLC-generated queries
type Repository struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

// NewRepository creates a new user repository
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{
		pool:    pool,
		queries: sqlc.New(pool),
	}
}

// GetByID retrieves a user by ID
func (r *Repository) GetByID(ctx context.Context, id string) (*domain.User, error) {
	start := time.Now()
	defer func() {
		observability.RecordDBQuery("select", "users", time.Since(start))
	}()

	ctx, span := observability.WrapDBOperation(ctx, "GetUserByID", "users")
	defer span.End()

	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, apperr.NotFoundf("user %s not found", id)
	}

	pgUUID := pgutil.UUIDToPgtype(uid)
	user, err := r.queries.GetUserByID(ctx, pgUUID)
	if err == pgx.ErrNoRows {
		return nil, apperr.NotFoundf("user %s not found", id)
	}
	if err != nil {
		observability.RecordSpanError(ctx, err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return sqlcUserToDomain(&user), nil
}

// GetByEmail retrieves a user by email
func (r *Repository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	start := time.Now()
	defer func() {
		observability.RecordDBQuery("select", "users", time.Since(start))
	}()

	ctx, span := observability.WrapDBOperation(ctx, "GetUserByEmail", "users")
	defer span.End()

	user, err := r.queries.GetUserByEmail(ctx, email)
	if err == pgx.ErrNoRows {
		return nil, apperr.NotFoundf("user with email %s not found", email)
	}
	if err != nil {
		observability.RecordSpanError(ctx, err)
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	return sqlcUserToDomain(&user), nil
}

// List retrieves a list of users with cursor pagination and optional filtering
func (r *Repository) List(ctx context.Context, filter domain.UserFilter) ([]domain.User, error) {
	start := time.Now()
	defer func() {
		observability.RecordDBQuery("select", "users", time.Since(start))
	}()

	ctx, span := observability.WrapDBOperation(ctx, "ListUsers", "users")
	defer span.End()

	// Normalize filter
	filter.NormalizeFilter()

	// Fetch one extra to determine if there are more
	limit := filter.Limit + 1

	// Build query params with nullable types
	params := sqlc.ListUsersParams{
		Limit:  int32(limit),
		Cursor: pgutil.NullableUUID(filter.Cursor),
	}

	// Apply optional filters
	if filter.Search.Set && filter.Search.Val != "" {
		params.Search = pgtype.Text{String: filter.Search.Val, Valid: true}
	}
	if filter.Email.Set && filter.Email.Val != "" {
		params.EmailFilter = pgtype.Text{String: filter.Email.Val, Valid: true}
	}
	if filter.IsActive.Set {
		params.IsActive = pgtype.Bool{Bool: filter.IsActive.Val, Valid: true}
	}

	users, err := r.queries.ListUsers(ctx, params)
	if err != nil {
		observability.RecordSpanError(ctx, err)
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	result := make([]domain.User, 0, len(users))
	for _, u := range users {
		result = append(result, *sqlcUserToDomain(&u))
	}

	return result, nil
}

// Create creates a new user
func (r *Repository) Create(ctx context.Context, email, passwordHash, name string) (*domain.User, error) {
	start := time.Now()
	defer func() {
		observability.RecordDBQuery("insert", "users", time.Since(start))
	}()

	ctx, span := observability.WrapDBOperation(ctx, "CreateUser", "users")
	defer span.End()

	user, err := r.queries.CreateUser(ctx, sqlc.CreateUserParams{
		Email:        email,
		PasswordHash: passwordHash,
		Name:         name,
	})
	if err != nil {
		observability.RecordSpanError(ctx, err)
		if pgutil.IsDuplicateKeyError(err) {
			return nil, apperr.Conflictf("user with email %s already exists", email)
		}
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	return sqlcUserToDomain(&user), nil
}

// Update updates a user
func (r *Repository) Update(ctx context.Context, id, name, email string) (*domain.User, error) {
	start := time.Now()
	defer func() {
		observability.RecordDBQuery("update", "users", time.Since(start))
	}()

	ctx, span := observability.WrapDBOperation(ctx, "UpdateUser", "users")
	defer span.End()

	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, apperr.NotFoundf("user %s not found", id)
	}

	user, err := r.queries.UpdateUser(ctx, sqlc.UpdateUserParams{
		ID:      pgutil.UUIDToPgtype(uid),
		Column2: name,
		Column3: email,
	})
	if err == pgx.ErrNoRows {
		return nil, apperr.NotFoundf("user %s not found", id)
	}
	if err != nil {
		observability.RecordSpanError(ctx, err)
		if pgutil.IsDuplicateKeyError(err) {
			return nil, apperr.Conflictf("user with email %s already exists", email)
		}
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	return sqlcUserToDomain(&user), nil
}

// UpdatePassword updates a user's password
func (r *Repository) UpdatePassword(ctx context.Context, id, passwordHash string) error {
	start := time.Now()
	defer func() {
		observability.RecordDBQuery("update", "users", time.Since(start))
	}()

	ctx, span := observability.WrapDBOperation(ctx, "UpdatePassword", "users")
	defer span.End()

	uid, err := uuid.Parse(id)
	if err != nil {
		return apperr.NotFoundf("user %s not found", id)
	}

	err = r.queries.UpdatePassword(ctx, sqlc.UpdatePasswordParams{
		ID:           pgutil.UUIDToPgtype(uid),
		PasswordHash: passwordHash,
	})
	if err != nil {
		observability.RecordSpanError(ctx, err)
		return fmt.Errorf("failed to update password: %w", err)
	}

	return nil
}

// Delete soft-deletes a user (sets is_active = false)
func (r *Repository) Delete(ctx context.Context, id string) error {
	start := time.Now()
	defer func() {
		observability.RecordDBQuery("update", "users", time.Since(start))
	}()

	ctx, span := observability.WrapDBOperation(ctx, "DeleteUser", "users")
	defer span.End()

	uid, err := uuid.Parse(id)
	if err != nil {
		return apperr.NotFoundf("user %s not found", id)
	}

	err = r.queries.DeleteUser(ctx, pgutil.UUIDToPgtype(uid))
	if err != nil {
		observability.RecordSpanError(ctx, err)
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// Activate activates a user (sets is_active = true)
func (r *Repository) Activate(ctx context.Context, id string) error {
	start := time.Now()
	defer func() {
		observability.RecordDBQuery("update", "users", time.Since(start))
	}()

	ctx, span := observability.WrapDBOperation(ctx, "ActivateUser", "users")
	defer span.End()

	uid, err := uuid.Parse(id)
	if err != nil {
		return apperr.NotFoundf("user %s not found", id)
	}

	err = r.queries.ActivateUser(ctx, pgutil.UUIDToPgtype(uid))
	if err != nil {
		observability.RecordSpanError(ctx, err)
		return fmt.Errorf("failed to activate user: %w", err)
	}

	return nil
}

// Deactivate deactivates a user (sets is_active = false)
func (r *Repository) Deactivate(ctx context.Context, id string) error {
	start := time.Now()
	defer func() {
		observability.RecordDBQuery("update", "users", time.Since(start))
	}()

	ctx, span := observability.WrapDBOperation(ctx, "DeactivateUser", "users")
	defer span.End()

	uid, err := uuid.Parse(id)
	if err != nil {
		return apperr.NotFoundf("user %s not found", id)
	}

	err = r.queries.DeactivateUser(ctx, pgutil.UUIDToPgtype(uid))
	if err != nil {
		observability.RecordSpanError(ctx, err)
		return fmt.Errorf("failed to deactivate user: %w", err)
	}

	return nil
}

// ExistsByEmail checks if a user with the given email exists
func (r *Repository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	start := time.Now()
	defer func() {
		observability.RecordDBQuery("select", "users", time.Since(start))
	}()

	ctx, span := observability.WrapDBOperation(ctx, "UserExistsByEmail", "users")
	defer span.End()

	exists, err := r.queries.UserExistsByEmail(ctx, email)
	if err != nil {
		observability.RecordSpanError(ctx, err)
		return false, fmt.Errorf("failed to check email existence: %w", err)
	}

	return exists, nil
}

// Count returns the total count of users with optional is_active filter
func (r *Repository) Count(ctx context.Context, isActive *bool) (int64, error) {
	start := time.Now()
	defer func() {
		observability.RecordDBQuery("select", "users", time.Since(start))
	}()

	ctx, span := observability.WrapDBOperation(ctx, "CountUsers", "users")
	defer span.End()

	var isActiveParam pgtype.Bool
	if isActive != nil {
		isActiveParam = pgtype.Bool{Bool: *isActive, Valid: true}
	}

	count, err := r.queries.CountUsers(ctx, isActiveParam)
	if err != nil {
		observability.RecordSpanError(ctx, err)
		return 0, fmt.Errorf("failed to count users: %w", err)
	}

	return count, nil
}

// sqlcUserToDomain converts SQLC User to domain User
func sqlcUserToDomain(u *sqlc.User) *domain.User {
	var createdAt, updatedAt time.Time
	if u.CreatedAt.Valid {
		createdAt = u.CreatedAt.Time
	}
	if u.UpdatedAt.Valid {
		updatedAt = u.UpdatedAt.Time
	}

	isActive := false
	if u.IsActive.Valid {
		isActive = u.IsActive.Bool
	}

	return &domain.User{
		ID:           pgutil.PgtypeToUUID(u.ID),
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		Name:         u.Name,
		IsActive:     isActive,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
}
