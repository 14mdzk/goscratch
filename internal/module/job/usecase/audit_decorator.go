package usecase

import (
	"context"

	"github.com/14mdzk/goscratch/internal/module/job/dto"
	"github.com/14mdzk/goscratch/internal/port"
)

// AuditedUseCase wraps a UseCase and adds audit logging on Dispatch.
// ListJobTypes is a read-only operation and is delegated as-is.
type AuditedUseCase struct {
	inner   UseCase
	auditor port.Auditor
}

// NewAuditedUseCase creates a new AuditedUseCase decorator.
func NewAuditedUseCase(inner UseCase, auditor port.Auditor) *AuditedUseCase {
	return &AuditedUseCase{inner: inner, auditor: auditor}
}

// Dispatch validates and publishes a job, logging a CREATE audit entry on success.
func (d *AuditedUseCase) Dispatch(ctx context.Context, jobType string, payload any, maxRetry int) (*dto.JobResponse, error) {
	resp, err := d.inner.Dispatch(ctx, jobType, payload, maxRetry)
	if err != nil {
		return nil, err
	}

	entry := port.NewAuditEntry(ctx, port.AuditActionCreate, "job", resp.ID)
	entry.Metadata = map[string]any{
		"job_type":  jobType,
		"max_retry": maxRetry,
	}
	_ = d.auditor.Log(ctx, entry)

	return resp, nil
}

// ListJobTypes delegates to inner without audit logging.
func (d *AuditedUseCase) ListJobTypes(ctx context.Context) *dto.ListJobTypesResponse {
	return d.inner.ListJobTypes(ctx)
}
