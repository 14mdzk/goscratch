package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AuditCleanupPayload represents the data for audit log cleanup
type AuditCleanupPayload struct {
	RetentionDays int `json:"retention_days"`
}

// AuditCleanupHandler handles audit log cleanup jobs
type AuditCleanupHandler struct {
	db     *pgxpool.Pool
	logger *logger.Logger
}

// NewAuditCleanupHandler creates a new audit cleanup handler
func NewAuditCleanupHandler(db *pgxpool.Pool, log *logger.Logger) *AuditCleanupHandler {
	return &AuditCleanupHandler{
		db:     db,
		logger: log,
	}
}

// Type returns the job type this handler processes
func (h *AuditCleanupHandler) Type() string {
	return worker.JobTypeAuditCleanup
}

// Handle processes an audit cleanup job
func (h *AuditCleanupHandler) Handle(ctx context.Context, job *worker.Job) error {
	var payload AuditCleanupPayload
	if err := job.UnmarshalPayload(&payload); err != nil {
		return fmt.Errorf("failed to unmarshal audit cleanup payload: %w", err)
	}

	// Default retention: 90 days
	retentionDays := payload.RetentionDays
	if retentionDays <= 0 {
		retentionDays = 90
	}

	h.logger.Info("Starting audit log cleanup",
		"retention_days", retentionDays,
		"job_id", job.ID,
	)

	// Calculate cutoff date
	cutoff := time.Now().AddDate(0, 0, -retentionDays)

	// Delete old audit logs
	result, err := h.db.Exec(ctx,
		"DELETE FROM audit_logs WHERE created_at < $1",
		cutoff,
	)
	if err != nil {
		return fmt.Errorf("failed to delete old audit logs: %w", err)
	}

	rowsDeleted := result.RowsAffected()

	h.logger.Info("Audit log cleanup completed",
		"rows_deleted", rowsDeleted,
		"cutoff_date", cutoff.Format(time.RFC3339),
		"job_id", job.ID,
	)

	return nil
}
