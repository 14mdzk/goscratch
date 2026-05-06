package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/14mdzk/goscratch/internal/module/auth/dto"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockAuthUseCase is a testify mock that satisfies the UseCase interface.
type mockAuthUseCase struct {
	mock.Mock
}

func (m *mockAuthUseCase) Login(ctx context.Context, req dto.LoginRequest) (*dto.LoginResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.LoginResponse), args.Error(1)
}

func (m *mockAuthUseCase) Refresh(ctx context.Context, req dto.RefreshRequest) (*dto.RefreshResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.RefreshResponse), args.Error(1)
}

func (m *mockAuthUseCase) Logout(ctx context.Context, callerID, refreshToken string) error {
	args := m.Called(ctx, callerID, refreshToken)
	return args.Error(0)
}

// mockAuditorAuth is a simple in-memory auditor for decorator tests.
type mockAuditorAuth struct {
	Entries []port.AuditEntry
}

func (m *mockAuditorAuth) Log(_ context.Context, entry port.AuditEntry) error {
	m.Entries = append(m.Entries, entry)
	return nil
}

func (m *mockAuditorAuth) Query(_ context.Context, _ port.AuditFilter) ([]port.AuditEntry, error) {
	return m.Entries, nil
}

func (m *mockAuditorAuth) Close() error { return nil }

// ---------------------------------------------------------------------------
// Login
// ---------------------------------------------------------------------------

func TestAuthAuditDecorator_Login(t *testing.T) {
	ctx := context.Background()
	req := dto.LoginRequest{Email: "user@example.com", Password: "password123"}

	t.Run("on success, logs LOGIN audit entry with populated ResourceID", func(t *testing.T) {
		inner := new(mockAuthUseCase)
		auditor := &mockAuditorAuth{}
		dec := NewAuditedUseCase(inner, auditor)

		resp := &dto.LoginResponse{
			AccessToken:  "access",
			RefreshToken: "refresh",
			ExpiresIn:    900,
			TokenType:    "Bearer",
			UserID:       "u-1",
		}
		inner.On("Login", ctx, req).Return(resp, nil)

		result, err := dec.Login(ctx, req)

		assert.NoError(t, err)
		assert.Equal(t, resp, result)

		assert.Len(t, auditor.Entries, 1)
		entry := auditor.Entries[0]
		assert.Equal(t, port.AuditActionLogin, entry.Action)
		assert.Equal(t, "user", entry.Resource)
		assert.Equal(t, "u-1", entry.ResourceID)
		assert.Equal(t, "success", entry.Metadata["outcome"])

		inner.AssertExpectations(t)
	})

	t.Run("on invalid credentials, logs failed LOGIN entry with sanitized reason and email as ResourceID", func(t *testing.T) {
		inner := new(mockAuthUseCase)
		auditor := &mockAuditorAuth{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("Login", ctx, req).Return(nil, apperr.ErrUnauthorized.WithMessage("Invalid email or password"))

		_, err := dec.Login(ctx, req)

		assert.Error(t, err)
		assert.Len(t, auditor.Entries, 1)
		entry := auditor.Entries[0]
		assert.Equal(t, port.AuditActionLogin, entry.Action)
		assert.Equal(t, "user", entry.Resource)
		assert.Equal(t, req.Email, entry.ResourceID)
		assert.Equal(t, "failed", entry.Metadata["outcome"])
		assert.Equal(t, "invalid_credentials", entry.Metadata["reason"])
		inner.AssertExpectations(t)
	})

	t.Run("on user inactive (forbidden), classifies reason as user_inactive", func(t *testing.T) {
		inner := new(mockAuthUseCase)
		auditor := &mockAuditorAuth{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("Login", ctx, req).Return(nil, apperr.ErrForbidden.WithMessage("Account is disabled"))

		_, err := dec.Login(ctx, req)

		assert.Error(t, err)
		assert.Len(t, auditor.Entries, 1)
		assert.Equal(t, "user_inactive", auditor.Entries[0].Metadata["reason"])
		inner.AssertExpectations(t)
	})

	t.Run("on opaque error, classifies reason as unknown and never echoes raw message", func(t *testing.T) {
		inner := new(mockAuthUseCase)
		auditor := &mockAuditorAuth{}
		dec := NewAuditedUseCase(inner, auditor)

		raw := errors.New("connection refused: pq host=secret-internal.db.local")
		inner.On("Login", ctx, req).Return(nil, raw)

		_, err := dec.Login(ctx, req)

		assert.Error(t, err)
		assert.Len(t, auditor.Entries, 1)
		assert.Equal(t, "unknown", auditor.Entries[0].Metadata["reason"])
		// Raw error message must not leak into audit metadata.
		assert.NotContains(t, auditor.Entries[0].Metadata["reason"], "secret-internal")
		inner.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// Refresh — no audit expected
// ---------------------------------------------------------------------------

func TestAuthAuditDecorator_Refresh(t *testing.T) {
	ctx := context.Background()
	req := dto.RefreshRequest{RefreshToken: "some-refresh-token"}

	t.Run("delegates to inner and produces no audit entry", func(t *testing.T) {
		inner := new(mockAuthUseCase)
		auditor := &mockAuditorAuth{}
		dec := NewAuditedUseCase(inner, auditor)

		resp := &dto.RefreshResponse{
			AccessToken:  "new-access",
			RefreshToken: "new-refresh",
			ExpiresIn:    900,
			TokenType:    "Bearer",
		}
		inner.On("Refresh", ctx, req).Return(resp, nil)

		result, err := dec.Refresh(ctx, req)

		assert.NoError(t, err)
		assert.Equal(t, resp, result)
		assert.Empty(t, auditor.Entries)
		inner.AssertExpectations(t)
	})

	t.Run("on failure, propagates error without audit", func(t *testing.T) {
		inner := new(mockAuthUseCase)
		auditor := &mockAuditorAuth{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("Refresh", ctx, req).Return(nil, errors.New("expired token"))

		_, err := dec.Refresh(ctx, req)

		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
		inner.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// Logout
// ---------------------------------------------------------------------------

func TestAuthAuditDecorator_Logout(t *testing.T) {
	ctx := context.Background()
	callerID := "user-42"
	refreshToken := "some-refresh-token"

	t.Run("on success, logs LOGOUT audit entry with callerID as ResourceID", func(t *testing.T) {
		inner := new(mockAuthUseCase)
		auditor := &mockAuditorAuth{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("Logout", ctx, callerID, refreshToken).Return(nil)

		err := dec.Logout(ctx, callerID, refreshToken)

		assert.NoError(t, err)
		assert.Len(t, auditor.Entries, 1)
		entry := auditor.Entries[0]
		assert.Equal(t, port.AuditActionLogout, entry.Action)
		assert.Equal(t, "user", entry.Resource)
		assert.Equal(t, callerID, entry.ResourceID)

		inner.AssertExpectations(t)
	})

	t.Run("on failure, does NOT log audit entry", func(t *testing.T) {
		inner := new(mockAuthUseCase)
		auditor := &mockAuditorAuth{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("Logout", ctx, callerID, refreshToken).Return(errors.New("logout failed"))

		err := dec.Logout(ctx, callerID, refreshToken)

		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
		inner.AssertExpectations(t)
	})
}
