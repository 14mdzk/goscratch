package pgutil

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUUIDToPgtype(t *testing.T) {
	id := uuid.New()
	pg := UUIDToPgtype(id)

	assert.True(t, pg.Valid)
	assert.Equal(t, [16]byte(id), pg.Bytes)
}

func TestPgtypeToUUID(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		id := uuid.New()
		pg := pgtype.UUID{Bytes: id, Valid: true}

		result := PgtypeToUUID(pg)
		assert.Equal(t, id, result)
	})

	t.Run("invalid_returns_nil_uuid", func(t *testing.T) {
		pg := pgtype.UUID{Valid: false}
		result := PgtypeToUUID(pg)
		assert.Equal(t, uuid.Nil, result)
	})
}

func TestNullableUUID(t *testing.T) {
	t.Run("empty_string", func(t *testing.T) {
		pg := NullableUUID("")
		assert.False(t, pg.Valid)
	})

	t.Run("invalid_uuid_string", func(t *testing.T) {
		pg := NullableUUID("not-a-uuid")
		assert.False(t, pg.Valid)
	})

	t.Run("valid_uuid_string", func(t *testing.T) {
		id := uuid.New()
		pg := NullableUUID(id.String())
		assert.True(t, pg.Valid)
		assert.Equal(t, [16]byte(id), pg.Bytes)
	})
}

func TestParseUUID(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		input := "123e4567-e89b-12d3-a456-426614174000"
		id, err := ParseUUID(input)
		require.NoError(t, err)
		assert.Equal(t, input, id.String())
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := ParseUUID("not-a-uuid")
		assert.Error(t, err)
	})

	t.Run("empty", func(t *testing.T) {
		_, err := ParseUUID("")
		assert.Error(t, err)
	})
}

func TestIsDuplicateKeyError(t *testing.T) {
	t.Run("nil_error", func(t *testing.T) {
		assert.False(t, IsDuplicateKeyError(nil))
	})

	t.Run("pg_error_23505", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23505"}
		assert.True(t, IsDuplicateKeyError(err))
	})

	t.Run("pg_error_other_code", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23503"}
		assert.False(t, IsDuplicateKeyError(err))
	})

	t.Run("string_contains_23505", func(t *testing.T) {
		err := errors.New("ERROR: duplicate key value violates unique constraint (SQLSTATE 23505)")
		assert.True(t, IsDuplicateKeyError(err))
	})

	t.Run("string_contains_duplicate_key", func(t *testing.T) {
		err := errors.New("duplicate key error")
		assert.True(t, IsDuplicateKeyError(err))
	})

	t.Run("unrelated_error", func(t *testing.T) {
		err := errors.New("connection refused")
		assert.False(t, IsDuplicateKeyError(err))
	})
}

func TestIsForeignKeyViolation(t *testing.T) {
	t.Run("nil_error", func(t *testing.T) {
		assert.False(t, IsForeignKeyViolation(nil))
	})

	t.Run("pg_error_23503", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23503"}
		assert.True(t, IsForeignKeyViolation(err))
	})

	t.Run("pg_error_other_code", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23505"}
		assert.False(t, IsForeignKeyViolation(err))
	})

	t.Run("string_contains_23503", func(t *testing.T) {
		err := errors.New("foreign key violation (SQLSTATE 23503)")
		assert.True(t, IsForeignKeyViolation(err))
	})

	t.Run("unrelated_error", func(t *testing.T) {
		err := errors.New("something else")
		assert.False(t, IsForeignKeyViolation(err))
	})
}

func TestIsNotNullViolation(t *testing.T) {
	t.Run("nil_error", func(t *testing.T) {
		assert.False(t, IsNotNullViolation(nil))
	})

	t.Run("pg_error_23502", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23502"}
		assert.True(t, IsNotNullViolation(err))
	})

	t.Run("pg_error_other_code", func(t *testing.T) {
		err := &pgconn.PgError{Code: "23505"}
		assert.False(t, IsNotNullViolation(err))
	})

	t.Run("string_contains_23502", func(t *testing.T) {
		err := errors.New("not null violation (SQLSTATE 23502)")
		assert.True(t, IsNotNullViolation(err))
	})

	t.Run("unrelated_error", func(t *testing.T) {
		err := errors.New("timeout")
		assert.False(t, IsNotNullViolation(err))
	})
}

func TestPgError(t *testing.T) {
	t.Run("pg_error", func(t *testing.T) {
		pgErr := &pgconn.PgError{Code: "23505", Message: "duplicate"}
		result := PgError(pgErr)
		assert.NotNil(t, result)
		assert.Equal(t, "23505", result.Code)
		assert.Equal(t, "duplicate", result.Message)
	})

	t.Run("non_pg_error", func(t *testing.T) {
		err := errors.New("regular error")
		result := PgError(err)
		assert.Nil(t, result)
	})
}
