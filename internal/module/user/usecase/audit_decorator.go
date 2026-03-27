package usecase

import (
	"context"

	"github.com/14mdzk/goscratch/internal/module/user/dto"
	"github.com/14mdzk/goscratch/internal/port"
	shareddomain "github.com/14mdzk/goscratch/internal/shared/domain"
)

// AuditedUseCase wraps a UseCase and adds audit logging on every mutating
// operation. Read-only methods (GetByID, List) are delegated as-is.
type AuditedUseCase struct {
	inner   UseCase
	auditor port.Auditor
}

// NewAuditedUseCase creates a new AuditedUseCase decorator.
func NewAuditedUseCase(inner UseCase, auditor port.Auditor) *AuditedUseCase {
	return &AuditedUseCase{inner: inner, auditor: auditor}
}

// GetByID delegates to inner without audit logging.
func (d *AuditedUseCase) GetByID(ctx context.Context, id string) (*dto.UserResponse, error) {
	return d.inner.GetByID(ctx, id)
}

// List delegates to inner without audit logging.
func (d *AuditedUseCase) List(ctx context.Context, req dto.ListUsersRequest) (shareddomain.CursorPage[dto.UserResponse], error) {
	return d.inner.List(ctx, req)
}

// Create creates a user and logs a CREATE audit entry on success.
func (d *AuditedUseCase) Create(ctx context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error) {
	resp, err := d.inner.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	entry := port.NewAuditEntry(ctx, port.AuditActionCreate, "user", resp.ID)
	entry.NewValue = map[string]any{
		"email": resp.Email,
		"name":  resp.Name,
	}
	_ = d.auditor.Log(ctx, entry)

	return resp, nil
}

// Update updates a user and logs an UPDATE audit entry on success.
func (d *AuditedUseCase) Update(ctx context.Context, id string, req dto.UpdateUserRequest) (*dto.UserResponse, error) {
	// Capture old state before mutation.
	oldUser, err := d.inner.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	resp, err := d.inner.Update(ctx, id, req)
	if err != nil {
		return nil, err
	}

	entry := port.NewAuditEntry(ctx, port.AuditActionUpdate, "user", resp.ID)
	entry.OldValue = map[string]any{"email": oldUser.Email, "name": oldUser.Name}
	entry.NewValue = map[string]any{"email": resp.Email, "name": resp.Name}
	_ = d.auditor.Log(ctx, entry)

	return resp, nil
}

// ChangePassword changes a user's password and logs an UPDATE audit entry on success.
func (d *AuditedUseCase) ChangePassword(ctx context.Context, id string, req dto.ChangePasswordRequest) error {
	if err := d.inner.ChangePassword(ctx, id, req); err != nil {
		return err
	}

	entry := port.NewAuditEntry(ctx, port.AuditActionUpdate, "user", id)
	entry.Metadata = map[string]any{"field": "password"}
	_ = d.auditor.Log(ctx, entry)

	return nil
}

// Delete soft-deletes a user and logs a DELETE audit entry on success.
func (d *AuditedUseCase) Delete(ctx context.Context, id string) error {
	// Capture old state before deletion.
	oldUser, err := d.inner.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := d.inner.Delete(ctx, id); err != nil {
		return err
	}

	entry := port.NewAuditEntry(ctx, port.AuditActionDelete, "user", id)
	entry.OldValue = map[string]any{
		"email": oldUser.Email,
		"name":  oldUser.Name,
	}
	_ = d.auditor.Log(ctx, entry)

	return nil
}

// Activate activates a user and logs an UPDATE audit entry on success.
// If the user is already active the inner usecase returns nil as a no-op;
// the decorator mirrors that behaviour and does not emit an audit entry.
func (d *AuditedUseCase) Activate(ctx context.Context, id string) error {
	// Capture old state to know whether the call actually mutated anything.
	oldUser, err := d.inner.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := d.inner.Activate(ctx, id); err != nil {
		return err
	}

	// Only audit when the state actually changed.
	if !oldUser.IsActive {
		entry := port.NewAuditEntry(ctx, port.AuditActionUpdate, "user", id)
		entry.OldValue = map[string]any{"is_active": false}
		entry.NewValue = map[string]any{"is_active": true}
		_ = d.auditor.Log(ctx, entry)
	}

	return nil
}

// Deactivate deactivates a user and logs an UPDATE audit entry on success.
// If the user is already inactive the inner usecase returns nil as a no-op;
// the decorator mirrors that behaviour and does not emit an audit entry.
func (d *AuditedUseCase) Deactivate(ctx context.Context, id string) error {
	// Capture old state to know whether the call actually mutated anything.
	oldUser, err := d.inner.GetByID(ctx, id)
	if err != nil {
		return err
	}

	if err := d.inner.Deactivate(ctx, id); err != nil {
		return err
	}

	// Only audit when the state actually changed.
	if oldUser.IsActive {
		entry := port.NewAuditEntry(ctx, port.AuditActionUpdate, "user", id)
		entry.OldValue = map[string]any{"is_active": true}
		entry.NewValue = map[string]any{"is_active": false}
		_ = d.auditor.Log(ctx, entry)
	}

	return nil
}
