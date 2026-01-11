package repository

import (
	"context"
	"testing"

	"github.com/14mdzk/goscratch/internal/module/user/domain"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRepository runs integration tests against the database
// These tests require a running PostgreSQL database
// Run with: go test -tags=integration ./...

type testDB struct {
	pool *pgxpool.Pool
}

func setupTestDB(t *testing.T) *testDB {
	// Skip if not running integration tests
	t.Helper()

	// Use environment variable for test database URL
	dsn := "postgres://postgres:postgres@localhost:5432/goscratch_test?sslmode=disable"

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Skipf("Skipping integration test: could not connect to test database: %v", err)
	}

	// Clean up users table before each test
	_, err = pool.Exec(ctx, "DELETE FROM users WHERE email LIKE 'test_%'")
	if err != nil {
		t.Logf("Warning: could not clean up test users: %v", err)
	}

	return &testDB{pool: pool}
}

func (db *testDB) cleanup(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	_, _ = db.pool.Exec(ctx, "DELETE FROM users WHERE email LIKE 'test_%'")
	db.pool.Close()
}

func TestRepository_Create(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	defer db.cleanup(t)

	repo := NewRepository(db.pool)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		user, err := repo.Create(ctx, "test_create@example.com", "hashedpassword123", "Test User")
		require.NoError(t, err)
		assert.NotEmpty(t, user.ID)
		assert.Equal(t, "test_create@example.com", user.Email)
		assert.Equal(t, "Test User", user.Name)
		assert.True(t, user.IsActive)
	})

	t.Run("duplicate_email", func(t *testing.T) {
		// Create first user
		_, err := repo.Create(ctx, "test_dup@example.com", "hash1", "User 1")
		require.NoError(t, err)

		// Try to create duplicate
		_, err = repo.Create(ctx, "test_dup@example.com", "hash2", "User 2")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestRepository_GetByID(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	defer db.cleanup(t)

	repo := NewRepository(db.pool)
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		created, err := repo.Create(ctx, "test_getbyid@example.com", "hash", "Test User")
		require.NoError(t, err)

		user, err := repo.GetByID(ctx, created.ID.String())
		require.NoError(t, err)
		assert.Equal(t, created.ID, user.ID)
		assert.Equal(t, created.Email, user.Email)
	})

	t.Run("not_found", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "00000000-0000-0000-0000-000000000000")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})

	t.Run("invalid_uuid", func(t *testing.T) {
		_, err := repo.GetByID(ctx, "invalid-uuid")
		assert.Error(t, err)
	})
}

func TestRepository_GetByEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	defer db.cleanup(t)

	repo := NewRepository(db.pool)
	ctx := context.Background()

	t.Run("found", func(t *testing.T) {
		_, err := repo.Create(ctx, "test_getbyemail@example.com", "hash", "Test User")
		require.NoError(t, err)

		user, err := repo.GetByEmail(ctx, "test_getbyemail@example.com")
		require.NoError(t, err)
		assert.Equal(t, "test_getbyemail@example.com", user.Email)
	})

	t.Run("not_found", func(t *testing.T) {
		_, err := repo.GetByEmail(ctx, "nonexistent@example.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not found")
	})
}

func TestRepository_List(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	defer db.cleanup(t)

	repo := NewRepository(db.pool)
	ctx := context.Background()

	// Create test users
	for i := 1; i <= 5; i++ {
		_, err := repo.Create(ctx, "test_list_"+string(rune('a'+i-1))+"@example.com", "hash", "User "+string(rune('0'+i)))
		require.NoError(t, err)
	}

	t.Run("list_all", func(t *testing.T) {
		users, err := repo.List(ctx, domain.UserFilter{Limit: 10})
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(users), 5)
	})

	t.Run("with_limit", func(t *testing.T) {
		users, err := repo.List(ctx, domain.UserFilter{Limit: 2})
		require.NoError(t, err)
		// Returns limit + 1 for hasMore check
		assert.LessOrEqual(t, len(users), 3)
	})
}

func TestRepository_Update(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	defer db.cleanup(t)

	repo := NewRepository(db.pool)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		created, err := repo.Create(ctx, "test_update@example.com", "hash", "Original Name")
		require.NoError(t, err)

		updated, err := repo.Update(ctx, created.ID.String(), "Updated Name", "")
		require.NoError(t, err)
		assert.Equal(t, "Updated Name", updated.Name)
		assert.Equal(t, "test_update@example.com", updated.Email)
	})

	t.Run("update_email", func(t *testing.T) {
		created, err := repo.Create(ctx, "test_update2@example.com", "hash", "User")
		require.NoError(t, err)

		updated, err := repo.Update(ctx, created.ID.String(), "", "test_updated2@example.com")
		require.NoError(t, err)
		assert.Equal(t, "test_updated2@example.com", updated.Email)
	})
}

func TestRepository_Delete(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	defer db.cleanup(t)

	repo := NewRepository(db.pool)
	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		created, err := repo.Create(ctx, "test_delete@example.com", "hash", "To Delete")
		require.NoError(t, err)

		err = repo.Delete(ctx, created.ID.String())
		require.NoError(t, err)

		// Should not find deleted user
		_, err = repo.GetByID(ctx, created.ID.String())
		assert.Error(t, err)
	})
}

func TestRepository_ExistsByEmail(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	db := setupTestDB(t)
	defer db.cleanup(t)

	repo := NewRepository(db.pool)
	ctx := context.Background()

	t.Run("exists", func(t *testing.T) {
		_, err := repo.Create(ctx, "test_exists@example.com", "hash", "User")
		require.NoError(t, err)

		exists, err := repo.ExistsByEmail(ctx, "test_exists@example.com")
		require.NoError(t, err)
		assert.True(t, exists)
	})

	t.Run("not_exists", func(t *testing.T) {
		exists, err := repo.ExistsByEmail(ctx, "test_notexists@example.com")
		require.NoError(t, err)
		assert.False(t, exists)
	})
}
