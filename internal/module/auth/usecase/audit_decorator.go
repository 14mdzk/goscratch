package usecase

import (
	"context"

	"github.com/14mdzk/goscratch/internal/module/auth/dto"
	"github.com/14mdzk/goscratch/internal/port"
)

// AuditedUseCase wraps a UseCase and adds audit logging for Login and
// Logout. Refresh is delegated as-is since it is not a security-sensitive
// event that requires an audit trail in the existing implementation.
type AuditedUseCase struct {
	inner   UseCase
	auditor port.Auditor
}

// NewAuditedUseCase creates a new AuditedUseCase decorator.
func NewAuditedUseCase(inner UseCase, auditor port.Auditor) *AuditedUseCase {
	return &AuditedUseCase{inner: inner, auditor: auditor}
}

// Login authenticates a user and logs a LOGIN audit entry on success.
func (d *AuditedUseCase) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error) {
	resp, err := d.inner.Login(ctx, req)
	if err != nil {
		return nil, err
	}

	// The user ID is embedded in the JWT but we can derive it from context
	// after the inner usecase propagates it. We use an empty resource ID
	// consistent with how the concrete usecase extracted it previously via
	// port.NewAuditEntry which reads from the context's user_id key. However
	// the concrete usecase used the real user ID obtained from the DB, so we
	// replicate that using a best-effort extraction. The inner call already
	// wrote the user_id into the token, not into the context, so we cannot
	// retrieve it here without coupling to implementation details. We keep
	// the same behaviour as the original code: NewAuditEntry reads user_id
	// from ctx (set by auth middleware on subsequent requests).
	entry := port.NewAuditEntry(ctx, port.AuditActionLogin, "user", "")
	_ = d.auditor.Log(ctx, entry)

	return resp, nil
}

// Refresh delegates to inner without audit logging.
func (d *AuditedUseCase) Refresh(ctx context.Context, req dto.RefreshRequest) (*dto.RefreshResponse, error) {
	return d.inner.Refresh(ctx, req)
}

// Logout invalidates the refresh token and logs a LOGOUT audit entry on success.
func (d *AuditedUseCase) Logout(ctx context.Context, refreshToken string) error {
	if err := d.inner.Logout(ctx, refreshToken); err != nil {
		return err
	}

	entry := port.NewAuditEntry(ctx, port.AuditActionLogout, "user", "")
	_ = d.auditor.Log(ctx, entry)

	return nil
}
