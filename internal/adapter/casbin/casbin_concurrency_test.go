package casbin

import (
	"context"
	"fmt"
	"sync"
	"testing"

	casbinlib "github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/stretchr/testify/require"
)

// newConcurrencyTestAdapter builds an in-memory Adapter (no DB, no watcher)
// backed by a SyncedEnforcer, mirroring the production constructor in
// NewAdapter. cacheSize 0 disables the decision cache.
func newConcurrencyTestAdapter(t *testing.T, cacheSize int) *Adapter {
	t.Helper()
	m, err := model.NewModelFromString(defaultModel)
	require.NoError(t, err)
	enforcer, err := casbinlib.NewSyncedEnforcer(m)
	require.NoError(t, err)
	return &Adapter{
		enforcer: enforcer,
		db:       nil,
		cache:    newDecisionCache(cacheSize),
	}
}

// TestAdapter_ConcurrentEnforceAndPolicyMutation is a regression test for the
// enforcer-concurrency fix (Adapter.enforcer is a *casbin.SyncedEnforcer,
// which serialises every adapter-used method with an RWMutex). It hammers
// Enforce concurrently with policy mutations through the public Adapter
// methods — the same paths the watcher callback and backstop reload hit.
// Run with -race: with a plain *casbin.Enforcer the detector reports
// concurrent map access inside the model/policy.
func TestAdapter_ConcurrentEnforceAndPolicyMutation(t *testing.T) {
	for _, cacheSize := range []int{0, 64} {
		t.Run(fmt.Sprintf("cache=%d", cacheSize), func(t *testing.T) {
			a := newConcurrencyTestAdapter(t, cacheSize)

			// Seed a baseline policy so readers have real work to do.
			require.NoError(t, a.AddPermissionForRole("reader", "articles", "read"))
			require.NoError(t, a.AddRoleForUser("alice", "reader"))

			var wg sync.WaitGroup

			// Readers: Enforce + EnforceWithContext in a tight loop.
			for r := range 8 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := range 200 {
						sub := fmt.Sprintf("user%d", (r+i)%16)
						_, _ = a.Enforce(sub, "articles", "read")
						_, _ = a.EnforceWithContext(context.Background(), sub, "articles", "write")
					}
				}()
			}

			// Writers: interleaved add/remove of policies and groupings.
			// Keys overlap across writers on purpose; duplicate adds and
			// missing removes are benign (false, nil) by Casbin semantics.
			for w := range 4 {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := range 100 {
						role := fmt.Sprintf("role-%d-%d", w, i%8)
						user := fmt.Sprintf("user-%d-%d", w, i%8)
						_ = a.AddPermissionForRole(role, "articles", "read")
						_ = a.AddRoleForUser(user, role)
						_, _ = a.GetRolesForUser(user)
						_, _ = a.GetImplicitPermissionsForUser(user)
						_ = a.RemoveRoleForUser(user, role)
						_ = a.RemovePermissionForRole(role, "articles", "read")
					}
				}()
			}

			wg.Wait()

			// Deterministic functional tail: after all goroutines join, the
			// adapter must still serve correct decisions.
			require.NoError(t, a.AddPermissionForUser("bob", "reports", "read"))
			allowed, err := a.Enforce("bob", "reports", "read")
			require.NoError(t, err)
			require.True(t, allowed)
		})
	}
}
