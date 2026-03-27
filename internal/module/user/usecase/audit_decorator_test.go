package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/14mdzk/goscratch/internal/module/user/dto"
	"github.com/14mdzk/goscratch/internal/port"
	shareddomain "github.com/14mdzk/goscratch/internal/shared/domain"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockUseCase is a testify mock that satisfies the UseCase interface.
type mockUseCase struct {
	mock.Mock
}

func (m *mockUseCase) GetByID(ctx context.Context, id string) (*dto.UserResponse, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.UserResponse), args.Error(1)
}

func (m *mockUseCase) List(ctx context.Context, req dto.ListUsersRequest) (shareddomain.CursorPage[dto.UserResponse], error) {
	args := m.Called(ctx, req)
	return args.Get(0).(shareddomain.CursorPage[dto.UserResponse]), args.Error(1)
}

func (m *mockUseCase) Create(ctx context.Context, req dto.CreateUserRequest) (*dto.UserResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.UserResponse), args.Error(1)
}

func (m *mockUseCase) Update(ctx context.Context, id string, req dto.UpdateUserRequest) (*dto.UserResponse, error) {
	args := m.Called(ctx, id, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.UserResponse), args.Error(1)
}

func (m *mockUseCase) ChangePassword(ctx context.Context, id string, req dto.ChangePasswordRequest) error {
	args := m.Called(ctx, id, req)
	return args.Error(0)
}

func (m *mockUseCase) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUseCase) Activate(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockUseCase) Deactivate(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// mockAuditorDecorator is a simple in-memory auditor for decorator tests.
type mockAuditorDecorator struct {
	Entries []port.AuditEntry
	err     error
}

func (m *mockAuditorDecorator) Log(_ context.Context, entry port.AuditEntry) error {
	if m.err != nil {
		return m.err
	}
	m.Entries = append(m.Entries, entry)
	return nil
}

func (m *mockAuditorDecorator) Query(_ context.Context, _ port.AuditFilter) ([]port.AuditEntry, error) {
	return m.Entries, nil
}

func (m *mockAuditorDecorator) Close() error { return nil }

// helper to build a UserResponse for a given UUID.
func buildUserResp(id uuid.UUID, email, name string, isActive bool) *dto.UserResponse {
	return &dto.UserResponse{
		ID:        id.String(),
		Email:     email,
		Name:      name,
		IsActive:  isActive,
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
}

// ---------------------------------------------------------------------------
// GetByID — no audit expected
// ---------------------------------------------------------------------------

func TestAuditDecorator_GetByID(t *testing.T) {
	ctx := context.Background()
	testID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")

	t.Run("delegates to inner and produces no audit entry", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		expected := buildUserResp(testID, "u@example.com", "User", true)
		inner.On("GetByID", ctx, testID.String()).Return(expected, nil)

		resp, err := dec.GetByID(ctx, testID.String())

		assert.NoError(t, err)
		assert.Equal(t, expected, resp)
		assert.Empty(t, auditor.Entries)
		inner.AssertExpectations(t)
	})

	t.Run("propagates error without audit", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("GetByID", ctx, "bad-id").Return(nil, errors.New("not found"))

		_, err := dec.GetByID(ctx, "bad-id")

		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
		inner.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// List — no audit expected
// ---------------------------------------------------------------------------

func TestAuditDecorator_List(t *testing.T) {
	ctx := context.Background()

	t.Run("delegates to inner and produces no audit entry", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		req := dto.ListUsersRequest{Limit: 10}
		page := shareddomain.CursorPage[dto.UserResponse]{}
		inner.On("List", ctx, req).Return(page, nil)

		result, err := dec.List(ctx, req)

		assert.NoError(t, err)
		assert.Equal(t, page, result)
		assert.Empty(t, auditor.Entries)
		inner.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// Create
// ---------------------------------------------------------------------------

func TestAuditDecorator_Create(t *testing.T) {
	ctx := context.Background()
	testID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")

	t.Run("on success, logs CREATE audit entry with new values", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		req := dto.CreateUserRequest{Email: "new@example.com", Password: "password123", Name: "New User"}
		resp := buildUserResp(testID, "new@example.com", "New User", true)
		inner.On("Create", ctx, req).Return(resp, nil)

		result, err := dec.Create(ctx, req)

		assert.NoError(t, err)
		assert.Equal(t, resp, result)

		assert.Len(t, auditor.Entries, 1)
		entry := auditor.Entries[0]
		assert.Equal(t, port.AuditActionCreate, entry.Action)
		assert.Equal(t, "user", entry.Resource)
		assert.Equal(t, testID.String(), entry.ResourceID)
		newVal := entry.NewValue.(map[string]any)
		assert.Equal(t, "new@example.com", newVal["email"])
		assert.Equal(t, "New User", newVal["name"])

		inner.AssertExpectations(t)
	})

	t.Run("on failure, does NOT log audit entry", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		req := dto.CreateUserRequest{Email: "dup@example.com", Password: "password123", Name: "User"}
		inner.On("Create", ctx, req).Return(nil, errors.New("conflict"))

		_, err := dec.Create(ctx, req)

		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
		inner.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// Update
// ---------------------------------------------------------------------------

func TestAuditDecorator_Update(t *testing.T) {
	ctx := context.Background()
	testID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")

	t.Run("on success, logs UPDATE audit entry with old and new values", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		oldResp := buildUserResp(testID, "old@example.com", "Old Name", true)
		req := dto.UpdateUserRequest{Name: "New Name", Email: "new@example.com"}
		newResp := buildUserResp(testID, "new@example.com", "New Name", true)

		inner.On("GetByID", ctx, testID.String()).Return(oldResp, nil)
		inner.On("Update", ctx, testID.String(), req).Return(newResp, nil)

		result, err := dec.Update(ctx, testID.String(), req)

		assert.NoError(t, err)
		assert.Equal(t, newResp, result)

		assert.Len(t, auditor.Entries, 1)
		entry := auditor.Entries[0]
		assert.Equal(t, port.AuditActionUpdate, entry.Action)
		assert.Equal(t, "user", entry.Resource)
		assert.Equal(t, testID.String(), entry.ResourceID)
		oldVal := entry.OldValue.(map[string]any)
		assert.Equal(t, "old@example.com", oldVal["email"])
		assert.Equal(t, "Old Name", oldVal["name"])
		newVal := entry.NewValue.(map[string]any)
		assert.Equal(t, "new@example.com", newVal["email"])
		assert.Equal(t, "New Name", newVal["name"])

		inner.AssertExpectations(t)
	})

	t.Run("on GetByID failure, does NOT call Update and does NOT log", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("GetByID", ctx, "bad-id").Return(nil, errors.New("not found"))

		_, err := dec.Update(ctx, "bad-id", dto.UpdateUserRequest{})

		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
		inner.AssertNotCalled(t, "Update")
		inner.AssertExpectations(t)
	})

	t.Run("on Update failure, does NOT log audit entry", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		oldResp := buildUserResp(testID, "old@example.com", "Old Name", true)
		req := dto.UpdateUserRequest{Name: "New Name", Email: "dup@example.com"}

		inner.On("GetByID", ctx, testID.String()).Return(oldResp, nil)
		inner.On("Update", ctx, testID.String(), req).Return(nil, errors.New("conflict"))

		_, err := dec.Update(ctx, testID.String(), req)

		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
		inner.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// ChangePassword
// ---------------------------------------------------------------------------

func TestAuditDecorator_ChangePassword(t *testing.T) {
	ctx := context.Background()
	testID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")
	req := dto.ChangePasswordRequest{CurrentPassword: "old", NewPassword: "newpass123"}

	t.Run("on success, logs UPDATE audit entry with password metadata", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("ChangePassword", ctx, testID.String(), req).Return(nil)

		err := dec.ChangePassword(ctx, testID.String(), req)

		assert.NoError(t, err)
		assert.Len(t, auditor.Entries, 1)
		entry := auditor.Entries[0]
		assert.Equal(t, port.AuditActionUpdate, entry.Action)
		assert.Equal(t, "user", entry.Resource)
		assert.Equal(t, testID.String(), entry.ResourceID)
		assert.Equal(t, map[string]any{"field": "password"}, entry.Metadata)

		inner.AssertExpectations(t)
	})

	t.Run("on failure, does NOT log audit entry", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("ChangePassword", ctx, testID.String(), req).Return(errors.New("wrong password"))

		err := dec.ChangePassword(ctx, testID.String(), req)

		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
		inner.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// Delete
// ---------------------------------------------------------------------------

func TestAuditDecorator_Delete(t *testing.T) {
	ctx := context.Background()
	testID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")

	t.Run("on success, logs DELETE audit entry with old values", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		oldResp := buildUserResp(testID, "del@example.com", "Del User", true)
		inner.On("GetByID", ctx, testID.String()).Return(oldResp, nil)
		inner.On("Delete", ctx, testID.String()).Return(nil)

		err := dec.Delete(ctx, testID.String())

		assert.NoError(t, err)
		assert.Len(t, auditor.Entries, 1)
		entry := auditor.Entries[0]
		assert.Equal(t, port.AuditActionDelete, entry.Action)
		assert.Equal(t, "user", entry.Resource)
		assert.Equal(t, testID.String(), entry.ResourceID)
		oldVal := entry.OldValue.(map[string]any)
		assert.Equal(t, "del@example.com", oldVal["email"])
		assert.Equal(t, "Del User", oldVal["name"])

		inner.AssertExpectations(t)
	})

	t.Run("on GetByID failure, does NOT call Delete and does NOT log", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("GetByID", ctx, "bad-id").Return(nil, errors.New("not found"))

		err := dec.Delete(ctx, "bad-id")

		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
		inner.AssertNotCalled(t, "Delete")
		inner.AssertExpectations(t)
	})

	t.Run("on Delete failure, does NOT log audit entry", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		oldResp := buildUserResp(testID, "del@example.com", "Del User", true)
		inner.On("GetByID", ctx, testID.String()).Return(oldResp, nil)
		inner.On("Delete", ctx, testID.String()).Return(errors.New("delete failed"))

		err := dec.Delete(ctx, testID.String())

		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
		inner.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// Activate
// ---------------------------------------------------------------------------

func TestAuditDecorator_Activate(t *testing.T) {
	ctx := context.Background()
	testID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")

	t.Run("on success for inactive user, logs UPDATE audit entry", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		oldResp := buildUserResp(testID, "u@example.com", "User", false)
		inner.On("GetByID", ctx, testID.String()).Return(oldResp, nil)
		inner.On("Activate", ctx, testID.String()).Return(nil)

		err := dec.Activate(ctx, testID.String())

		assert.NoError(t, err)
		assert.Len(t, auditor.Entries, 1)
		entry := auditor.Entries[0]
		assert.Equal(t, port.AuditActionUpdate, entry.Action)
		assert.Equal(t, "user", entry.Resource)
		assert.Equal(t, map[string]any{"is_active": false}, entry.OldValue)
		assert.Equal(t, map[string]any{"is_active": true}, entry.NewValue)

		inner.AssertExpectations(t)
	})

	t.Run("for already-active user, delegates no-op and does NOT log", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		// inner.Activate is a no-op when already active (returns nil immediately)
		alreadyActive := buildUserResp(testID, "u@example.com", "User", true)
		inner.On("GetByID", ctx, testID.String()).Return(alreadyActive, nil)
		inner.On("Activate", ctx, testID.String()).Return(nil)

		err := dec.Activate(ctx, testID.String())

		assert.NoError(t, err)
		// No audit because IsActive was already true
		assert.Empty(t, auditor.Entries)

		inner.AssertExpectations(t)
	})

	t.Run("on GetByID failure, does NOT call Activate and does NOT log", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("GetByID", ctx, "bad-id").Return(nil, errors.New("not found"))

		err := dec.Activate(ctx, "bad-id")

		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
		inner.AssertNotCalled(t, "Activate")
		inner.AssertExpectations(t)
	})

	t.Run("on Activate failure, does NOT log audit entry", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		oldResp := buildUserResp(testID, "u@example.com", "User", false)
		inner.On("GetByID", ctx, testID.String()).Return(oldResp, nil)
		inner.On("Activate", ctx, testID.String()).Return(errors.New("db error"))

		err := dec.Activate(ctx, testID.String())

		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
		inner.AssertExpectations(t)
	})
}

// ---------------------------------------------------------------------------
// Deactivate
// ---------------------------------------------------------------------------

func TestAuditDecorator_Deactivate(t *testing.T) {
	ctx := context.Background()
	testID := uuid.MustParse("01234567-89ab-cdef-0123-456789abcdef")

	t.Run("on success for active user, logs UPDATE audit entry", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		oldResp := buildUserResp(testID, "u@example.com", "User", true)
		inner.On("GetByID", ctx, testID.String()).Return(oldResp, nil)
		inner.On("Deactivate", ctx, testID.String()).Return(nil)

		err := dec.Deactivate(ctx, testID.String())

		assert.NoError(t, err)
		assert.Len(t, auditor.Entries, 1)
		entry := auditor.Entries[0]
		assert.Equal(t, port.AuditActionUpdate, entry.Action)
		assert.Equal(t, "user", entry.Resource)
		assert.Equal(t, map[string]any{"is_active": true}, entry.OldValue)
		assert.Equal(t, map[string]any{"is_active": false}, entry.NewValue)

		inner.AssertExpectations(t)
	})

	t.Run("for already-inactive user, delegates no-op and does NOT log", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		alreadyInactive := buildUserResp(testID, "u@example.com", "User", false)
		inner.On("GetByID", ctx, testID.String()).Return(alreadyInactive, nil)
		inner.On("Deactivate", ctx, testID.String()).Return(nil)

		err := dec.Deactivate(ctx, testID.String())

		assert.NoError(t, err)
		// No audit because IsActive was already false
		assert.Empty(t, auditor.Entries)

		inner.AssertExpectations(t)
	})

	t.Run("on GetByID failure, does NOT call Deactivate and does NOT log", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("GetByID", ctx, "bad-id").Return(nil, errors.New("not found"))

		err := dec.Deactivate(ctx, "bad-id")

		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
		inner.AssertNotCalled(t, "Deactivate")
		inner.AssertExpectations(t)
	})

	t.Run("on Deactivate failure, does NOT log audit entry", func(t *testing.T) {
		inner := new(mockUseCase)
		auditor := &mockAuditorDecorator{}
		dec := NewAuditedUseCase(inner, auditor)

		oldResp := buildUserResp(testID, "u@example.com", "User", true)
		inner.On("GetByID", ctx, testID.String()).Return(oldResp, nil)
		inner.On("Deactivate", ctx, testID.String()).Return(errors.New("db error"))

		err := dec.Deactivate(ctx, testID.String())

		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
		inner.AssertExpectations(t)
	})
}
