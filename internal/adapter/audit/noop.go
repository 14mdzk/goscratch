package audit

import (
	"context"

	"github.com/14mdzk/goscratch/internal/port"
)

// NoOpAuditor implements port.Auditor as a no-op
// Used when audit logging is disabled
type NoOpAuditor struct{}

// NewNoOpAuditor creates a new no-op auditor
func NewNoOpAuditor() *NoOpAuditor {
	return &NoOpAuditor{}
}

func (a *NoOpAuditor) Log(ctx context.Context, entry port.AuditEntry) error {
	return nil
}

func (a *NoOpAuditor) Query(ctx context.Context, filter port.AuditFilter) ([]port.AuditEntry, error) {
	return []port.AuditEntry{}, nil
}

func (a *NoOpAuditor) Close() error {
	return nil
}
