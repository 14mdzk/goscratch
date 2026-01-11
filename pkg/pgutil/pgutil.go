package pgutil

import (
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// UUIDToPgtype converts google/uuid to pgtype.UUID
func UUIDToPgtype(id uuid.UUID) pgtype.UUID {
	return pgtype.UUID{
		Bytes: id,
		Valid: true,
	}
}

// PgtypeToUUID converts pgtype.UUID to google/uuid
func PgtypeToUUID(id pgtype.UUID) uuid.UUID {
	if !id.Valid {
		return uuid.Nil
	}
	return id.Bytes
}

// NullableUUID converts a string to pgtype.UUID, returning invalid if empty
func NullableUUID(s string) pgtype.UUID {
	if s == "" {
		return pgtype.UUID{Valid: false}
	}
	uid, err := uuid.Parse(s)
	if err != nil {
		return pgtype.UUID{Valid: false}
	}
	return UUIDToPgtype(uid)
}

// ParseUUID parses a string to uuid.UUID with error handling
func ParseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

// IsDuplicateKeyError checks if the error is a PostgreSQL unique constraint violation
func IsDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	// Check for pgx error type
	if pgErr, ok := err.(*pgconn.PgError); ok {
		return pgErr.Code == "23505" // unique_violation
	}
	// Fallback to string matching
	errStr := err.Error()
	return strings.Contains(errStr, "23505") || strings.Contains(errStr, "duplicate key")
}

// IsForeignKeyViolation checks if the error is a PostgreSQL foreign key violation
func IsForeignKeyViolation(err error) bool {
	if err == nil {
		return false
	}
	if pgErr, ok := err.(*pgconn.PgError); ok {
		return pgErr.Code == "23503" // foreign_key_violation
	}
	return strings.Contains(err.Error(), "23503")
}

// IsNotNullViolation checks if the error is a PostgreSQL not null violation
func IsNotNullViolation(err error) bool {
	if err == nil {
		return false
	}
	if pgErr, ok := err.(*pgconn.PgError); ok {
		return pgErr.Code == "23502" // not_null_violation
	}
	return strings.Contains(err.Error(), "23502")
}

// PgError extracts the underlying PostgreSQL error if present
func PgError(err error) *pgconn.PgError {
	if pgErr, ok := err.(*pgconn.PgError); ok {
		return pgErr
	}
	return nil
}
