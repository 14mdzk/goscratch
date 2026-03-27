package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/14mdzk/goscratch/internal/module/auth/dto"
	"github.com/14mdzk/goscratch/internal/port"
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

func (m *mockAuthUseCase) Logout(ctx context.Context, refreshToken string) error {
	args := m.Called(ctx, refreshToken)
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

	t.Run("on success, logs LOGIN audit entry", func(t *testing.T) {
		inner := new(mockAuthUseCase)
		auditor := &mockAuditorAuth{}
		dec := NewAuditedUseCase(inner, auditor)

		resp := &dto.LoginResponse{
			AccessToken:  "access",
			RefreshToken: "refresh",
			ExpiresIn:    900,
			TokenType:    "Bearer",
		}
		inner.On("Login", ctx, req).Return(resp, nil)

		result, err := dec.Login(ctx, req)

		assert.NoError(t, err)
		assert.Equal(t, resp, result)

		assert.Len(t, auditor.Entries, 1)
		entry := auditor.Entries[0]
		assert.Equal(t, port.AuditActionLogin, entry.Action)
		assert.Equal(t, "user", entry.Resource)

		inner.AssertExpectations(t)
	})

	t.Run("on failure, does NOT log audit entry", func(t *testing.T) {
		inner := new(mockAuthUseCase)
		auditor := &mockAuditorAuth{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("Login", ctx, req).Return(nil, errors.New("invalid credentials"))

		_, err := dec.Login(ctx, req)

		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
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
	refreshToken := "some-refresh-token"

	t.Run("on success, logs LOGOUT audit entry", func(t *testing.T) {
		inner := new(mockAuthUseCase)
		auditor := &mockAuditorAuth{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("Logout", ctx, refreshToken).Return(nil)

		err := dec.Logout(ctx, refreshToken)

		assert.NoError(t, err)
		assert.Len(t, auditor.Entries, 1)
		entry := auditor.Entries[0]
		assert.Equal(t, port.AuditActionLogout, entry.Action)
		assert.Equal(t, "user", entry.Resource)

		inner.AssertExpectations(t)
	})

	t.Run("on failure, does NOT log audit entry", func(t *testing.T) {
		inner := new(mockAuthUseCase)
		auditor := &mockAuditorAuth{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("Logout", ctx, refreshToken).Return(errors.New("logout failed"))

		err := dec.Logout(ctx, refreshToken)

		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
		inner.AssertExpectations(t)
	})
}
