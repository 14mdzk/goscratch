package app

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAuthorizer counts Close invocations and satisfies the bare subset of
// port.Authorizer that App.Shutdown touches. The unused methods panic if hit
// so an unexpected call surfaces immediately.
type fakeAuthorizer struct {
	closes atomic.Int32
	err    error
}

func (f *fakeAuthorizer) Enforce(_, _, _ string) (bool, error) { panic("unused") }
func (f *fakeAuthorizer) EnforceWithContext(context.Context, string, string, string) (bool, error) {
	panic("unused")
}
func (f *fakeAuthorizer) AddRoleForUser(_, _ string) error             { panic("unused") }
func (f *fakeAuthorizer) RemoveRoleForUser(_, _ string) error          { panic("unused") }
func (f *fakeAuthorizer) GetRolesForUser(string) ([]string, error)     { panic("unused") }
func (f *fakeAuthorizer) GetUsersForRole(string) ([]string, error)     { panic("unused") }
func (f *fakeAuthorizer) HasRoleForUser(_, _ string) (bool, error)     { panic("unused") }
func (f *fakeAuthorizer) AddPermissionForRole(_, _, _ string) error    { panic("unused") }
func (f *fakeAuthorizer) RemovePermissionForRole(_, _, _ string) error { panic("unused") }
func (f *fakeAuthorizer) GetPermissionsForRole(string) ([][]string, error) {
	panic("unused")
}
func (f *fakeAuthorizer) AddPermissionForUser(_, _, _ string) error    { panic("unused") }
func (f *fakeAuthorizer) RemovePermissionForUser(_, _, _ string) error { panic("unused") }
func (f *fakeAuthorizer) GetPermissionsForUser(string) ([][]string, error) {
	panic("unused")
}
func (f *fakeAuthorizer) GetImplicitPermissionsForUser(string) ([][]string, error) {
	panic("unused")
}
func (f *fakeAuthorizer) LoadPolicy() error             { panic("unused") }
func (f *fakeAuthorizer) SavePolicy() error             { panic("unused") }
func (f *fakeAuthorizer) Start(_ context.Context) error { return nil }
func (f *fakeAuthorizer) Close() error {
	f.closes.Add(1)
	return f.err
}

// TestApp_Shutdown_ClosesAuthorizer verifies that App.Shutdown invokes
// Authorizer.Close exactly once. Other adapter ports are left nil; the
// Shutdown path is nil-safe by design (PR-04 task 1.5).
func TestApp_Shutdown_ClosesAuthorizer(t *testing.T) {
	fa := &fakeAuthorizer{}

	a := &App{
		Logger:     logger.New(logger.Config{Level: "error", Format: "json"}),
		Authorizer: fa,
	}

	require.NoError(t, a.Shutdown(context.Background()))
	assert.Equal(t, int32(1), fa.closes.Load(), "Authorizer.Close must be called exactly once")
}

// TestApp_Shutdown_NilAuthorizer_NoPanic verifies that Shutdown is safe when
// the Authorizer field is nil (e.g. early-failed boot, partial test fixtures).
func TestApp_Shutdown_NilAuthorizer_NoPanic(t *testing.T) {
	a := &App{
		Logger: logger.New(logger.Config{Level: "error", Format: "json"}),
	}
	require.NoError(t, a.Shutdown(context.Background()))
}

// =============================================================================
// Phase-ordering tests (PR-04 task 3)
// =============================================================================

// callRecorder is shared across the per-phase fakes so we can assert ordering.
type callRecorder struct {
	mu    sync.Mutex
	calls []string
}

func (r *callRecorder) record(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, name)
}

func (r *callRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]string, len(r.calls))
	copy(out, r.calls)
	return out
}

// fakeAuthorizerOrder records its Close into the recorder.
type fakeAuthorizerOrder struct {
	*fakeAuthorizer
	rec *callRecorder
}

func (f *fakeAuthorizerOrder) Close() error {
	f.rec.record("authorizer")
	return f.fakeAuthorizer.Close()
}

type fakeSSE struct{ rec *callRecorder }

func (f *fakeSSE) Subscribe(string, ...string) <-chan port.Event { panic("unused") }
func (f *fakeSSE) Unsubscribe(string)                            {}
func (f *fakeSSE) Broadcast(port.Event)                          {}
func (f *fakeSSE) BroadcastToTopic(string, port.Event)           {}
func (f *fakeSSE) SendTo(string, port.Event)                     {}
func (f *fakeSSE) ClientCount() int                              { return 0 }
func (f *fakeSSE) Close() error                                  { f.rec.record("sse"); return nil }

// TestApp_Shutdown_PhaseOrder asserts the canonical ordering: SSE before
// authorizer, both before tracer.
func TestApp_Shutdown_PhaseOrder(t *testing.T) {
	rec := &callRecorder{}

	a := &App{
		Logger:     logger.New(logger.Config{Level: "error", Format: "json"}),
		Authorizer: &fakeAuthorizerOrder{fakeAuthorizer: &fakeAuthorizer{}, rec: rec},
		SSE:        &fakeSSE{rec: rec},
		tracerShutdown: func(_ context.Context) error {
			rec.record("tracer")
			return nil
		},
	}

	require.NoError(t, a.Shutdown(context.Background()))

	calls := rec.snapshot()
	require.Equal(t, []string{"sse", "authorizer", "tracer"}, calls,
		"phase order must be: sse → authorizer → tracer")
}

// TestApp_Shutdown_TracerLast asserts that the tracer shutdown closure runs
// after the authorizer and DB phases. We use a pure stub tracer (no DB / pool
// required) and a closure that records into the same recorder.
func TestApp_Shutdown_TracerLast(t *testing.T) {
	rec := &callRecorder{}

	a := &App{
		Logger:     logger.New(logger.Config{Level: "error", Format: "json"}),
		Authorizer: &fakeAuthorizerOrder{fakeAuthorizer: &fakeAuthorizer{}, rec: rec},
		tracerShutdown: func(_ context.Context) error {
			rec.record("tracer")
			return nil
		},
	}

	require.NoError(t, a.Shutdown(context.Background()))

	calls := rec.snapshot()
	require.NotEmpty(t, calls)
	require.Equal(t, "tracer", calls[len(calls)-1], "tracer must be the last phase")
	require.Contains(t, calls, "authorizer")
	// authorizer must precede tracer.
	authIdx := indexOf(calls, "authorizer")
	traceIdx := indexOf(calls, "tracer")
	require.Less(t, authIdx, traceIdx, "authorizer must close before tracer")
}

// TestApp_Shutdown_RespectsParentBudget verifies the total Shutdown duration
// stays within a tight parent deadline (no phase blocks past its fraction).
func TestApp_Shutdown_RespectsParentBudget(t *testing.T) {
	rec := &callRecorder{}

	parent, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	a := &App{
		Logger:     logger.New(logger.Config{Level: "error", Format: "json"}),
		Authorizer: &fakeAuthorizerOrder{fakeAuthorizer: &fakeAuthorizer{}, rec: rec},
	}

	start := time.Now()
	require.NoError(t, a.Shutdown(parent))
	total := time.Since(start)

	require.LessOrEqual(t, total, 250*time.Millisecond,
		"shutdown overran the parent 100ms budget (got %s)", total)
}

func indexOf(ss []string, target string) int {
	for i, s := range ss {
		if s == target {
			return i
		}
	}
	return -1
}
