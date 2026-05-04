package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/14mdzk/goscratch/internal/module/job/dto"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockJobUseCase is a testify mock satisfying the UseCase interface.
type mockJobUseCase struct {
	mock.Mock
}

func (m *mockJobUseCase) Dispatch(ctx context.Context, jobType string, payload any, maxRetry int) (*dto.JobResponse, error) {
	args := m.Called(ctx, jobType, payload, maxRetry)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*dto.JobResponse), args.Error(1)
}

func (m *mockJobUseCase) ListJobTypes(ctx context.Context) *dto.ListJobTypesResponse {
	args := m.Called(ctx)
	return args.Get(0).(*dto.ListJobTypesResponse)
}

type mockJobAuditor struct {
	Entries []port.AuditEntry
}

func (m *mockJobAuditor) Log(_ context.Context, entry port.AuditEntry) error {
	m.Entries = append(m.Entries, entry)
	return nil
}

func (m *mockJobAuditor) Query(_ context.Context, _ port.AuditFilter) ([]port.AuditEntry, error) {
	return m.Entries, nil
}

func (m *mockJobAuditor) Close() error { return nil }

func TestJobAuditDecorator_Dispatch(t *testing.T) {
	ctx := context.Background()

	t.Run("on success, logs CREATE audit entry with job metadata", func(t *testing.T) {
		inner := new(mockJobUseCase)
		auditor := &mockJobAuditor{}
		dec := NewAuditedUseCase(inner, auditor)

		resp := &dto.JobResponse{ID: "job-1", Type: "email.send", Status: "queued"}
		inner.On("Dispatch", ctx, "email.send", "payload", 3).Return(resp, nil)

		got, err := dec.Dispatch(ctx, "email.send", "payload", 3)
		assert.NoError(t, err)
		assert.Equal(t, resp, got)
		assert.Len(t, auditor.Entries, 1)
		entry := auditor.Entries[0]
		assert.Equal(t, port.AuditActionCreate, entry.Action)
		assert.Equal(t, "job", entry.Resource)
		assert.Equal(t, "job-1", entry.ResourceID)
		assert.Equal(t, "email.send", entry.Metadata["job_type"])
		assert.Equal(t, 3, entry.Metadata["max_retry"])
	})

	t.Run("on failure, does NOT log audit entry", func(t *testing.T) {
		inner := new(mockJobUseCase)
		auditor := &mockJobAuditor{}
		dec := NewAuditedUseCase(inner, auditor)

		inner.On("Dispatch", ctx, "bad", nil, 0).Return(nil, errors.New("invalid"))

		_, err := dec.Dispatch(ctx, "bad", nil, 0)
		assert.Error(t, err)
		assert.Empty(t, auditor.Entries)
	})
}

func TestJobAuditDecorator_ListJobTypes_NoAudit(t *testing.T) {
	ctx := context.Background()
	inner := new(mockJobUseCase)
	auditor := &mockJobAuditor{}
	dec := NewAuditedUseCase(inner, auditor)

	lr := &dto.ListJobTypesResponse{}
	inner.On("ListJobTypes", ctx).Return(lr)

	_ = dec.ListJobTypes(ctx)
	assert.Empty(t, auditor.Entries)
}
