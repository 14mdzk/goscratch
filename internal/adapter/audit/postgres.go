package audit

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/14mdzk/goscratch/internal/port"
)

// PostgresAuditor implements port.Auditor using PostgreSQL
type PostgresAuditor struct {
	pool *pgxpool.Pool
}

// NewPostgresAuditor creates a new PostgreSQL auditor
func NewPostgresAuditor(pool *pgxpool.Pool) *PostgresAuditor {
	return &PostgresAuditor{pool: pool}
}

func (a *PostgresAuditor) Log(ctx context.Context, entry port.AuditEntry) error {
	var oldValueJSON, newValueJSON []byte
	var err error

	if entry.OldValue != nil {
		oldValueJSON, err = json.Marshal(entry.OldValue)
		if err != nil {
			return fmt.Errorf("failed to marshal old value: %w", err)
		}
	}

	if entry.NewValue != nil {
		newValueJSON, err = json.Marshal(entry.NewValue)
		if err != nil {
			return fmt.Errorf("failed to marshal new value: %w", err)
		}
	}

	query := `
		INSERT INTO audit_logs (user_id, action, resource, resource_id, old_value, new_value, ip_address, user_agent, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`

	var userID any
	if entry.UserID != "" {
		userID = entry.UserID
	}

	_, err = a.pool.Exec(ctx, query,
		userID,
		entry.Action,
		entry.Resource,
		entry.ResourceID,
		oldValueJSON,
		newValueJSON,
		nullString(entry.IPAddress),
		nullString(entry.UserAgent),
		entry.Timestamp,
	)

	if err != nil {
		return fmt.Errorf("failed to insert audit log: %w", err)
	}

	return nil
}

func (a *PostgresAuditor) Query(ctx context.Context, filter port.AuditFilter) ([]port.AuditEntry, error) {
	query := `
		SELECT id, user_id, action, resource, resource_id, old_value, new_value, ip_address, user_agent, created_at
		FROM audit_logs
		WHERE 1=1
	`
	args := []any{}
	argIndex := 1

	if filter.UserID != "" {
		query += fmt.Sprintf(" AND user_id = $%d", argIndex)
		args = append(args, filter.UserID)
		argIndex++
	}

	if filter.Action != "" {
		query += fmt.Sprintf(" AND action = $%d", argIndex)
		args = append(args, filter.Action)
		argIndex++
	}

	if filter.Resource != "" {
		query += fmt.Sprintf(" AND resource = $%d", argIndex)
		args = append(args, filter.Resource)
		argIndex++
	}

	if filter.ResourceID != "" {
		query += fmt.Sprintf(" AND resource_id = $%d", argIndex)
		args = append(args, filter.ResourceID)
		argIndex++
	}

	if filter.StartTime != nil {
		query += fmt.Sprintf(" AND created_at >= $%d", argIndex)
		args = append(args, filter.StartTime)
		argIndex++
	}

	if filter.EndTime != nil {
		query += fmt.Sprintf(" AND created_at <= $%d", argIndex)
		args = append(args, filter.EndTime)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	limit := filter.Limit
	if limit <= 0 {
		limit = 50
	}
	query += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, limit)

	rows, err := a.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query audit logs: %w", err)
	}
	defer rows.Close()

	var entries []port.AuditEntry
	for rows.Next() {
		var entry port.AuditEntry
		var id string
		var userID *string
		var oldValue, newValue []byte
		var ipAddress, userAgent *string
		var createdAt time.Time

		err := rows.Scan(
			&id,
			&userID,
			&entry.Action,
			&entry.Resource,
			&entry.ResourceID,
			&oldValue,
			&newValue,
			&ipAddress,
			&userAgent,
			&createdAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan audit log: %w", err)
		}

		entry.ID = id
		if userID != nil {
			entry.UserID = *userID
		}
		if oldValue != nil {
			json.Unmarshal(oldValue, &entry.OldValue)
		}
		if newValue != nil {
			json.Unmarshal(newValue, &entry.NewValue)
		}
		if ipAddress != nil {
			entry.IPAddress = *ipAddress
		}
		if userAgent != nil {
			entry.UserAgent = *userAgent
		}
		entry.Timestamp = createdAt

		entries = append(entries, entry)
	}

	return entries, nil
}

func (a *PostgresAuditor) Close() error {
	return nil // Pool is managed externally
}

func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// Ensure PostgresAuditor implements the interface
var _ port.Auditor = (*PostgresAuditor)(nil)

// Compile-time check for pgx.Row interface
var _ pgx.Row = (pgx.Row)(nil)
