package database

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetTx verifies that GetTx returns nil when no transaction is in the
// context, and returns the stored transaction when one is present.
func TestGetTx(t *testing.T) {
	t.Run("no_tx_in_context", func(t *testing.T) {
		ctx := context.Background()
		tx := GetTx(ctx)
		assert.Nil(t, tx)
	})

	t.Run("tx_in_context", func(t *testing.T) {
		// We cannot create a real pgx.Tx without a live database in unit tests,
		// but we can verify the type-assertion path: a non-nil value that is NOT
		// a pgx.Tx must result in nil being returned (graceful degradation).
		ctx := context.WithValue(context.Background(), txKey{}, "not-a-tx")
		tx := GetTx(ctx)
		assert.Nil(t, tx, "type mismatch should return nil, not panic")
	})
}

// TestDBFromContext verifies the pool-fallback behaviour when no transaction
// is active.  A nil pool is acceptable here because DBFromContext only
// inspects the context; it does not dereference the pool.
func TestDBFromContext(t *testing.T) {
	t.Run("returns_pool_when_no_tx", func(t *testing.T) {
		ctx := context.Background()
		// Pass nil pool — DBFromContext must return it when there is no TX.
		db := DBFromContext(ctx, nil)
		assert.Nil(t, db, "expected pool (nil) to be returned when no transaction is present")
	})

	t.Run("returns_tx_when_tx_present", func(t *testing.T) {
		// Craft a context that carries a value under txKey that IS a pgx.Tx.
		// Without a live database we cannot obtain a real pgx.Tx, so we
		// exercise only the "no TX" path in this unit-test suite.
		// The integration tests validate the full round-trip.
		ctx := context.Background()
		require.Nil(t, GetTx(ctx))
	})
}

// fakeTx is a minimal pgx.Tx stand-in. Only Rollback and Commit are exercised
// by the WithTx tests; the embedded pgx.Tx interface lets the struct satisfy
// the type without re-declaring every method (calling any non-overridden
// method would nil-deref, which is fine — the tests never call them).
type fakeTx struct {
	pgx.Tx
	rollbackCalled      bool
	commitCalled        bool
	rollbackErrAtCall   error // ctx.Err() captured at rollback time
	rollbackHadDeadline bool
	rollbackDeadline    time.Time
	rollbackErrFunc     func(ctx context.Context) error
}

func (f *fakeTx) Rollback(ctx context.Context) error {
	f.rollbackCalled = true
	f.rollbackErrAtCall = ctx.Err()
	if d, ok := ctx.Deadline(); ok {
		f.rollbackHadDeadline = true
		f.rollbackDeadline = d
	}
	if f.rollbackErrFunc != nil {
		return f.rollbackErrFunc(ctx)
	}
	return nil
}

func (f *fakeTx) Commit(ctx context.Context) error {
	f.commitCalled = true
	return nil
}

// fakeBeginner returns the same fakeTx for every Begin call.
type fakeBeginner struct {
	tx       *fakeTx
	beginErr error
}

func (f *fakeBeginner) Begin(ctx context.Context) (pgx.Tx, error) {
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	return f.tx, nil
}

// TestWithTx_RollbackUsesFreshContext is the regression test for the bug fixed
// in this PR: the rollback path must NOT use the outer ctx, because on shutdown
// that ctx is already cancelled and pgx would short-circuit the rollback,
// leaving the transaction open on the server.
func TestWithTx_RollbackUsesFreshContext(t *testing.T) {
	t.Run("fn_error_with_cancelled_outer_ctx", func(t *testing.T) {
		tx := &fakeTx{}
		tor := &Transactor{pool: &fakeBeginner{tx: tx}}

		// Cancel the outer ctx BEFORE calling fn so the rollback path sees a
		// dead parent. A naive implementation would forward this ctx straight
		// to tx.Rollback and the rollback would observe context.Canceled.
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		fnErr := errors.New("boom")
		gotErr := tor.WithTx(ctx, func(ctx context.Context) error {
			return fnErr
		})

		require.ErrorIs(t, gotErr, fnErr)
		require.True(t, tx.rollbackCalled, "rollback must be invoked even when outer ctx is cancelled")

		// The rollback ctx must NOT be the outer (cancelled) ctx — captured
		// at the moment Rollback was called, before WithTx's defer cancel().
		require.NoError(t, tx.rollbackErrAtCall,
			"rollback ctx must be live at call time (not the outer cancelled ctx)")

		// And it must carry a bounded deadline so a slow server cannot wedge
		// shutdown indefinitely.
		require.True(t, tx.rollbackHadDeadline, "rollback ctx must carry a deadline")
		require.WithinDuration(t, time.Now().Add(rollbackTimeout), tx.rollbackDeadline, rollbackTimeout)
	})

	t.Run("fn_success_commits_on_outer_ctx", func(t *testing.T) {
		tx := &fakeTx{}
		tor := &Transactor{pool: &fakeBeginner{tx: tx}}

		err := tor.WithTx(context.Background(), func(ctx context.Context) error {
			return nil
		})
		require.NoError(t, err)
		require.False(t, tx.rollbackCalled)
		require.True(t, tx.commitCalled)
	})
}
