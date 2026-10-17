package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/14mdzk/goscratch/internal/worker"
	"github.com/14mdzk/goscratch/pkg/logger"
)

const (
	auditCleanupBatchSize = 1000
	// auditCleanupMaxBatches caps progress per run (60k rows at the batch
	// size above). Cleanup runs on operator cron, so a larger backlog drains
	// over successive runs; the 5-minute handler timeout stays the backstop
	// for a stuck batch.
	auditCleanupMaxBatches = 60
)

// batchDeleter is the subset of *pgxpool.Pool the cleanup handler needs.
// *pgxpool.Pool satisfies it; tests substitute a fake.
type batchDeleter interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// AuditCleanupPayload represents the data for audit log cleanup
type AuditCleanupPayload struct {
	RetentionDays int `json:"retention_days"`
}

// AuditCleanupHandler handles audit log cleanup jobs
type AuditCleanupHandler struct {
	db     batchDeleter
	logger *logger.Logger
}

// NewAuditCleanupHandler creates a new audit cleanup handler
func NewAuditCleanupHandler(db batchDeleter, log *logger.Logger) *AuditCleanupHandler {
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
	start := time.Now()

	// Delete old audit logs in batches to prevent memory issues. The
	// per-run batch cap bounds the loop; the remainder drains on the next
	// scheduled run. Partial progress is fine — the job is idempotent.
	totalDeleted := int64(0)
	batches := 0
	capped := false

	for {
		result, err := h.db.Exec(ctx,
			`DELETE FROM audit_logs
			WHERE id IN (
				SELECT id
				FROM audit_logs
				WHERE created_at < $1
				LIMIT $2
			)`,
			cutoff,
			auditCleanupBatchSize,
		)
		if err != nil {
			return fmt.Errorf("failed to delete old audit logs: %w", err)
		}

		deletedInBatch := result.RowsAffected()
		totalDeleted += deletedInBatch
		batches++

		if deletedInBatch == 0 {
			break // No more rows to delete
		}

		h.logger.Info("Deleted batch of audit logs",
			"deleted_in_batch", deletedInBatch,
			"total_deleted", totalDeleted,
			"cutoff_date", cutoff.Format(time.RFC3339),
			"job_id", job.ID,
		)

		if batches >= auditCleanupMaxBatches {
			capped = true
			h.logger.Warn("Audit cleanup batch cap reached; remainder continues on next scheduled run",
				"rows_deleted", totalDeleted,
				"batches", batches,
				"cutoff_date", cutoff.Format(time.RFC3339),
				"job_id", job.ID,
			)
			break
		}
	}

	h.logger.Info("Audit log cleanup completed",
		"rows_deleted", totalDeleted,
		"batches", batches,
		"capped", capped,
		"duration_ms", time.Since(start).Milliseconds(),
		"cutoff_date", cutoff.Format(time.RFC3339),
		"job_id", job.ID,
	)

	return nil
}
