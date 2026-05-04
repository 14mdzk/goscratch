package usecase

import (
	"context"
	"io"
	"mime/multipart"
	"time"

	"github.com/14mdzk/goscratch/internal/module/storage/dto"
	"github.com/14mdzk/goscratch/internal/port"
)

// AuditedUseCase wraps a UseCase and adds audit logging on every mutating
// operation. Read-only methods (Download, GetURL, List) are delegated as-is
// to keep the audit log signal-to-noise ratio high.
type AuditedUseCase struct {
	inner   UseCase
	auditor port.Auditor
}

// NewAuditedUseCase creates a new AuditedUseCase decorator.
func NewAuditedUseCase(inner UseCase, auditor port.Auditor) *AuditedUseCase {
	return &AuditedUseCase{inner: inner, auditor: auditor}
}

// Upload uploads a file and logs a CREATE audit entry on success.
func (d *AuditedUseCase) Upload(ctx context.Context, file multipart.File, header *multipart.FileHeader, directory string) (*dto.UploadResponse, error) {
	resp, err := d.inner.Upload(ctx, file, header, directory)
	if err != nil {
		return nil, err
	}

	entry := port.NewAuditEntry(ctx, port.AuditActionCreate, "file", resp.Path)
	entry.NewValue = map[string]any{
		"size":              resp.Size,
		"original_filename": header.Filename,
	}
	_ = d.auditor.Log(ctx, entry)

	return resp, nil
}

// Download delegates to inner without audit logging.
func (d *AuditedUseCase) Download(ctx context.Context, path string) (io.ReadCloser, string, error) {
	return d.inner.Download(ctx, path)
}

// Delete removes a file and logs a DELETE audit entry on success.
func (d *AuditedUseCase) Delete(ctx context.Context, path string) error {
	if err := d.inner.Delete(ctx, path); err != nil {
		return err
	}

	entry := port.NewAuditEntry(ctx, port.AuditActionDelete, "file", path)
	_ = d.auditor.Log(ctx, entry)

	return nil
}

// GetURL delegates to inner without audit logging.
func (d *AuditedUseCase) GetURL(ctx context.Context, path string, expires time.Duration) (*dto.FileResponse, error) {
	return d.inner.GetURL(ctx, path, expires)
}

// List delegates to inner without audit logging.
func (d *AuditedUseCase) List(ctx context.Context, prefix string) (*dto.ListFilesResponse, error) {
	return d.inner.List(ctx, prefix)
}
