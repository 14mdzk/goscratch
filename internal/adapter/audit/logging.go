package audit

import (
	"context"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/logger"
)

// LoggingAuditor wraps a port.Auditor and emits a structured error log every
// time an audit INSERT fails. Every usecase audit decorator calls Log
// best-effort (`_ = d.auditor.Log(...)`) so a failed INSERT never fails the
// user-facing operation — without this wrapper those failures vanished
// silently and gaps in the audit trail were undetectable. The error is still
// returned so callers that do check it keep working.
type LoggingAuditor struct {
	inner  port.Auditor
	logger *logger.Logger
}

// NewLoggingAuditor creates a LoggingAuditor.
func NewLoggingAuditor(inner port.Auditor, log *logger.Logger) *LoggingAuditor {
	return &LoggingAuditor{inner: inner, logger: log}
}

// Log records the entry via the inner auditor, logging failures.
func (a *LoggingAuditor) Log(ctx context.Context, entry port.AuditEntry) error {
	if err := a.inner.Log(ctx, entry); err != nil {
		a.logger.Error("audit insert failed",
			"error", err,
			"action", entry.Action,
			"resource", entry.Resource,
			"resource_id", entry.ResourceID,
		)
		return err
	}
	return nil
}

// Query delegates to the inner auditor.
func (a *LoggingAuditor) Query(ctx context.Context, filter port.AuditFilter) ([]port.AuditEntry, error) {
	return a.inner.Query(ctx, filter)
}

// Close delegates to the inner auditor.
func (a *LoggingAuditor) Close() error {
	return a.inner.Close()
}
