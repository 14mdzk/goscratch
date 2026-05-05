package usecase

import (
	"context"
	"errors"

	"github.com/14mdzk/goscratch/internal/module/auth/dto"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
)

// AuditedUseCase wraps a UseCase and adds audit logging for Login (success
// and failure), Logout. Refresh is delegated as-is.
type AuditedUseCase struct {
	inner   UseCase
	auditor port.Auditor
}

// NewAuditedUseCase creates a new AuditedUseCase decorator.
func NewAuditedUseCase(inner UseCase, auditor port.Auditor) *AuditedUseCase {
	return &AuditedUseCase{inner: inner, auditor: auditor}
}

// Login authenticates a user and logs a LOGIN audit entry on both success
// and failure. On failure, ResourceID is the attempted email so brute-force
// activity against a single email is detectable; the failure reason is
// sanitized to a fixed category to avoid echoing raw error strings into the
// audit log.
func (d *AuditedUseCase) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error) {
	resp, err := d.inner.Login(ctx, req)
	if err != nil {
		entry := port.NewAuditEntry(ctx, port.AuditActionLogin, "user", req.Email)
		entry.Metadata = map[string]any{
			"outcome": "failed",
			"reason":  classifyLoginFailure(err),
		}
		_ = d.auditor.Log(ctx, entry)
		return nil, err
	}

	entry := port.NewAuditEntry(ctx, port.AuditActionLogin, "user", resp.UserID)
	entry.Metadata = map[string]any{"outcome": "success"}
	_ = d.auditor.Log(ctx, entry)

	return resp, nil
}

// Refresh delegates to inner without audit logging.
func (d *AuditedUseCase) Refresh(ctx context.Context, req dto.RefreshRequest) (*dto.RefreshResponse, error) {
	return d.inner.Refresh(ctx, req)
}

// Logout invalidates the refresh token and logs a LOGOUT audit entry on success.
func (d *AuditedUseCase) Logout(ctx context.Context, callerID, refreshToken string) error {
	if err := d.inner.Logout(ctx, callerID, refreshToken); err != nil {
		return err
	}

	entry := port.NewAuditEntry(ctx, port.AuditActionLogout, "user", callerID)
	_ = d.auditor.Log(ctx, entry)

	return nil
}

// classifyLoginFailure maps a login error to a fixed sanitized category so
// the audit log never echoes raw error strings (which could leak details or
// vary across releases). Inner usecase wraps both bad-password and
// no-such-user as ErrUnauthorized with the same generic message.
func classifyLoginFailure(err error) string {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		switch ae.Code {
		case apperr.CodeUnauthorized:
			return "invalid_credentials"
		case apperr.CodeForbidden:
			return "user_inactive"
		}
	}
	return "unknown"
}
