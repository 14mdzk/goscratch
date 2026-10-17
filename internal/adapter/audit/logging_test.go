package audit

import (
	"bytes"
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/logger"
)

// stubAuditor is a scriptable port.Auditor.
type stubAuditor struct {
	logErr error
	logged []port.AuditEntry
	closed bool
}

func (s *stubAuditor) Log(_ context.Context, entry port.AuditEntry) error {
	s.logged = append(s.logged, entry)
	return s.logErr
}

func (s *stubAuditor) Query(_ context.Context, _ port.AuditFilter) ([]port.AuditEntry, error) {
	return []port.AuditEntry{}, nil
}

func (s *stubAuditor) Close() error {
	s.closed = true
	return nil
}

func newTestLoggingAuditor(inner port.Auditor) (*LoggingAuditor, *bytes.Buffer) {
	var buf bytes.Buffer
	log := logger.New(logger.Config{Level: "debug", Format: "json", Output: &buf})
	return NewLoggingAuditor(inner, log), &buf
}

func TestLoggingAuditor_ImplementsInterface(t *testing.T) {
	var _ port.Auditor = (*LoggingAuditor)(nil)
}

func TestLoggingAuditor_Log_SuccessPassthrough(t *testing.T) {
	inner := &stubAuditor{}
	a, buf := newTestLoggingAuditor(inner)

	entry := port.AuditEntry{Action: "CREATE", Resource: "job", ResourceID: "job-1"}
	require.NoError(t, a.Log(context.Background(), entry))
	require.Len(t, inner.logged, 1)
	assert.NotContains(t, buf.String(), "audit insert failed")
}

func TestLoggingAuditor_Log_FailureIsLoggedAndReturned(t *testing.T) {
	inner := &stubAuditor{logErr: errors.New("connection refused")}
	a, buf := newTestLoggingAuditor(inner)

	entry := port.AuditEntry{Action: "CREATE", Resource: "job", ResourceID: "job-1"}
	err := a.Log(context.Background(), entry)
	require.ErrorIs(t, err, inner.logErr)

	out := buf.String()
	assert.Contains(t, out, "audit insert failed")
	assert.Contains(t, out, "job-1")
}

func TestLoggingAuditor_QueryAndClose_Delegate(t *testing.T) {
	inner := &stubAuditor{}
	a, _ := newTestLoggingAuditor(inner)

	entries, err := a.Query(context.Background(), port.AuditFilter{})
	require.NoError(t, err)
	assert.Empty(t, entries)
	require.NoError(t, a.Close())
	assert.True(t, inner.closed)
}
