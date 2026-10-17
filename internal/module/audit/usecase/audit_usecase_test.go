package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/14mdzk/goscratch/internal/module/audit/dto"
	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
)

// stubAuditor is a scriptable port.Auditor.
type stubAuditor struct {
	entries []port.AuditEntry
	err     error
	last    port.AuditFilter
}

func (s *stubAuditor) Log(_ context.Context, _ port.AuditEntry) error { return nil }

func (s *stubAuditor) Query(_ context.Context, filter port.AuditFilter) ([]port.AuditEntry, error) {
	s.last = filter
	if s.err != nil {
		return nil, s.err
	}
	return s.entries, nil
}

func (s *stubAuditor) Close() error { return nil }

func testEntry(id string, ts time.Time) port.AuditEntry {
	return port.AuditEntry{
		ID:         id,
		UserID:     "user-1",
		Action:     port.AuditActionLogin,
		Resource:   "user",
		ResourceID: "user-1",
		Timestamp:  ts,
	}
}

func TestList_MapsFiltersAndDefaultsLimit(t *testing.T) {
	auditor := &stubAuditor{}
	uc := NewUseCase(auditor)

	_, err := uc.List(context.Background(), dto.ListAuditLogsRequest{
		Action:    "LOGIN",
		UserID:    "user-1",
		StartTime: "2026-01-01T00:00:00Z",
		EndTime:   "2026-12-31T00:00:00Z",
	})
	require.NoError(t, err)
	assert.Equal(t, "user-1", auditor.last.UserID)
	assert.Equal(t, port.AuditAction("LOGIN"), auditor.last.Action)
	assert.Equal(t, 51, auditor.last.Limit, "limit+1 for has_more detection")
	require.NotNil(t, auditor.last.StartTime)
	require.NotNil(t, auditor.last.EndTime)
}

func TestList_TrimsExtraRowAndMintsCursor(t *testing.T) {
	now := time.Now().UTC()
	auditor := &stubAuditor{entries: []port.AuditEntry{
		testEntry("11111111-1111-1111-1111-111111111111", now),
		testEntry("22222222-2222-2222-2222-222222222222", now.Add(-time.Minute)),
		testEntry("33333333-3333-3333-3333-333333333333", now.Add(-2*time.Minute)),
	}}
	uc := NewUseCase(auditor)

	page, err := uc.List(context.Background(), dto.ListAuditLogsRequest{Limit: 2})
	require.NoError(t, err)
	require.Len(t, page.Items, 2)
	assert.Equal(t, "11111111-1111-1111-1111-111111111111", page.Items[0].ID)
	assert.True(t, page.HasMore)
	require.NotNil(t, page.NextCursor)

	// The cursor must decode to the last returned entry.
	ts, id, err := port.DecodeAuditCursor(*page.NextCursor)
	require.NoError(t, err)
	assert.Equal(t, "22222222-2222-2222-2222-222222222222", id)
	assert.True(t, ts.Equal(auditor.entries[1].Timestamp))
}

func TestList_NoMoreWhenExactPage(t *testing.T) {
	auditor := &stubAuditor{entries: []port.AuditEntry{testEntry("11111111-1111-1111-1111-111111111111", time.Now().UTC())}}
	uc := NewUseCase(auditor)

	page, err := uc.List(context.Background(), dto.ListAuditLogsRequest{Limit: 5})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.False(t, page.HasMore)
	assert.Nil(t, page.NextCursor)
}

func TestList_InvalidCursorIsBadRequest(t *testing.T) {
	auditor := &stubAuditor{err: port.ErrInvalidAuditCursor}
	uc := NewUseCase(auditor)

	_, err := uc.List(context.Background(), dto.ListAuditLogsRequest{Cursor: "bogus"})
	require.Error(t, err)
	appErr, ok := apperr.AsAppError(err)
	require.True(t, ok)
	assert.Equal(t, apperr.CodeBadRequest, appErr.Code)
}

func TestList_AuditorErrorIsInternal(t *testing.T) {
	auditor := &stubAuditor{err: errors.New("connection refused")}
	uc := NewUseCase(auditor)

	_, err := uc.List(context.Background(), dto.ListAuditLogsRequest{})
	require.Error(t, err)
	appErr, ok := apperr.AsAppError(err)
	require.True(t, ok)
	assert.Equal(t, apperr.CodeInternalError, appErr.Code)
}

func TestList_InvalidTimeIsBadRequest(t *testing.T) {
	uc := NewUseCase(&stubAuditor{})

	_, err := uc.List(context.Background(), dto.ListAuditLogsRequest{StartTime: "not-a-time"})
	require.Error(t, err)
	appErr, ok := apperr.AsAppError(err)
	require.True(t, ok)
	assert.Equal(t, apperr.CodeBadRequest, appErr.Code)
}
