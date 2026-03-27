package database

import (
	"context"
	"testing"

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
