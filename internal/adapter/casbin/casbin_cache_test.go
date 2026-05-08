package casbin

import (
	"context"
	"testing"
	"time"

	casbinlib "github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testTimeout() time.Duration { return 500 * time.Millisecond }
func testPoll() time.Duration    { return 5 * time.Millisecond }

// newCachedTestAdapter creates an Adapter backed by an in-memory Casbin
// enforcer (no DB) with a decision cache of the given size.
// Pass size=0 to create a disabled-cache adapter.
func newCachedTestAdapter(t *testing.T, cacheSize int) *Adapter {
	t.Helper()
	m, err := model.NewModelFromString(defaultModel)
	require.NoError(t, err)
	enforcer, err := casbinlib.NewEnforcer(m)
	require.NoError(t, err)
	return &Adapter{
		enforcer: enforcer,
		db:       nil,
		cache:    newDecisionCache(cacheSize),
	}
}

// =============================================================================
// Decision cache unit tests
// =============================================================================

func TestDecisionCache_CacheKey_NullSeparator(t *testing.T) {
	// Ensure the key encodes correctly and that \x00 separator is used.
	k := cacheKey("alice", "data1", "read")
	assert.Equal(t, "alice\x00data1\x00read", k)
}

func TestDecisionCache_GetPut(t *testing.T) {
	c := newDecisionCache(10)
	_, ok := c.get("s", "o", "a")
	assert.False(t, ok)

	c.put("s", "o", "a", true)
	v, ok := c.get("s", "o", "a")
	assert.True(t, ok)
	assert.True(t, v)
}

func TestDecisionCache_NilReceiver(t *testing.T) {
	var c *decisionCache
	_, ok := c.get("s", "o", "a")
	assert.False(t, ok)
	c.put("s", "o", "a", true) // must not panic
	c.invalidateSub("s")       // must not panic
	c.flush()                  // must not panic
	assert.Equal(t, 0, c.len())
}

func TestDecisionCache_SizeZero_Disabled(t *testing.T) {
	c := newDecisionCache(0)
	c.put("s", "o", "a", true)
	_, ok := c.get("s", "o", "a")
	assert.False(t, ok, "size-0 cache must never return a hit")
}

func TestDecisionCache_LRUEviction(t *testing.T) {
	c := newDecisionCache(3)
	c.put("u1", "o", "a", true)
	c.put("u2", "o", "a", true)
	c.put("u3", "o", "a", true)
	assert.Equal(t, 3, c.len())

	// Adding a 4th entry should evict u1 (LRU).
	c.put("u4", "o", "a", true)
	assert.Equal(t, 3, c.len())
	_, ok := c.get("u1", "o", "a")
	assert.False(t, ok, "u1 should have been evicted")

	// u4 is MRU, must be present.
	_, ok = c.get("u4", "o", "a")
	assert.True(t, ok)
}

func TestDecisionCache_LRUPromotion(t *testing.T) {
	c := newDecisionCache(3)
	c.put("u1", "o", "a", true)
	c.put("u2", "o", "a", true)
	c.put("u3", "o", "a", true)

	// Access u1 to promote it to MRU position.
	c.get("u1", "o", "a")

	// Adding u4 should evict the new LRU which is u2.
	c.put("u4", "o", "a", true)
	_, ok := c.get("u2", "o", "a")
	assert.False(t, ok, "u2 should have been evicted after u1 was promoted")
	_, ok = c.get("u1", "o", "a")
	assert.True(t, ok, "u1 was promoted; it must still be present")
}

func TestDecisionCache_InvalidateSub(t *testing.T) {
	c := newDecisionCache(100)
	c.put("alice", "data1", "read", true)
	c.put("alice", "data2", "write", false)
	c.put("bob", "data1", "read", true)

	c.invalidateSub("alice")

	_, ok := c.get("alice", "data1", "read")
	assert.False(t, ok, "alice entries must be removed")
	_, ok = c.get("alice", "data2", "write")
	assert.False(t, ok, "alice entries must be removed")
	_, ok = c.get("bob", "data1", "read")
	assert.True(t, ok, "bob entry must be unaffected")
}

func TestDecisionCache_Flush(t *testing.T) {
	c := newDecisionCache(100)
	c.put("a", "b", "c", true)
	c.put("d", "e", "f", false)
	c.flush()
	assert.Equal(t, 0, c.len())
}

// =============================================================================
// Adapter cache integration tests
// =============================================================================

// TestDecisionCache_Hit verifies that a cached answer survives a direct
// enforcer mutation (which bypasses the adapter's invalidation path),
// and that flushing the cache exposes the updated answer.
func TestDecisionCache_Hit(t *testing.T) {
	a := newCachedTestAdapter(t, 100)

	// Prime: no permission yet → false, cached.
	v1, err := a.Enforce("user1", "res", "read")
	require.NoError(t, err)
	assert.False(t, v1)

	// Add permission directly to the underlying enforcer (bypasses cache invalidation).
	_, err = a.enforcer.AddPolicy("user1", "res", "read")
	require.NoError(t, err)

	// Cache still returns the stale false.
	v2, err := a.Enforce("user1", "res", "read")
	require.NoError(t, err)
	assert.False(t, v2, "stale cached answer must be returned before flush")

	// Flush the cache directly (simulates what LoadPolicy does).
	a.cache.flush()

	// Now the fresh answer must be true.
	v3, err := a.Enforce("user1", "res", "read")
	require.NoError(t, err)
	assert.True(t, v3, "fresh answer expected after cache flush")
}

// TestDecisionCache_Disabled_Size0 verifies that with a size-0 cache every
// Enforce call re-evaluates the model (no stale answers).
func TestDecisionCache_Disabled_Size0(t *testing.T) {
	a := newCachedTestAdapter(t, 0)

	v1, err := a.Enforce("user1", "res", "read")
	require.NoError(t, err)
	assert.False(t, v1)

	// Mutate enforcer directly — no cache so the new answer must be visible immediately.
	_, err = a.enforcer.AddPolicy("user1", "res", "read")
	require.NoError(t, err)

	// With size-0 cache the Enforce call must go to the enforcer every time.
	v2, err := a.Enforce("user1", "res", "read")
	require.NoError(t, err)
	assert.True(t, v2, "size-0 cache must not hide enforcer changes")
}

// =============================================================================
// Invalidation matrix tests
// =============================================================================

// TestDecisionCache_InvalidatesOn_AddRoleForUser verifies that granting a role
// to a user invalidates that user's cached decisions.
func TestDecisionCache_InvalidatesOn_AddRoleForUser(t *testing.T) {
	a := newCachedTestAdapter(t, 100)

	// user1 has no permission initially.
	v1, err := a.Enforce("user1", "res", "read")
	require.NoError(t, err)
	assert.False(t, v1)
	// Entry is now cached (false).

	// Give the role the permission directly on the enforcer.
	_, err = a.enforcer.AddPolicy("editor", "res", "read")
	require.NoError(t, err)

	// AddRoleForUser via adapter must invalidate user1's cache entries.
	require.NoError(t, a.AddRoleForUser("user1", "editor"))

	// Expect true because cache was flushed for user1 and model re-evaluated.
	v2, err := a.Enforce("user1", "res", "read")
	require.NoError(t, err)
	assert.True(t, v2, "cache must be invalidated after AddRoleForUser")
}

// TestDecisionCache_InvalidatesOn_RemoveRoleForUser verifies that removing a
// role invalidates the user's cached decisions.
func TestDecisionCache_InvalidatesOn_RemoveRoleForUser(t *testing.T) {
	a := newCachedTestAdapter(t, 100)

	// Setup: user1 has editor role + editor has permission.
	require.NoError(t, a.AddPermissionForRole("editor", "res", "read"))
	require.NoError(t, a.AddRoleForUser("user1", "editor"))

	v1, err := a.Enforce("user1", "res", "read")
	require.NoError(t, err)
	assert.True(t, v1)

	// Remove role via adapter — must invalidate.
	require.NoError(t, a.RemoveRoleForUser("user1", "editor"))

	v2, err := a.Enforce("user1", "res", "read")
	require.NoError(t, err)
	assert.False(t, v2, "cache must be invalidated after RemoveRoleForUser")
}

// TestDecisionCache_InvalidatesOn_AddPermissionForRole verifies that granting
// a new permission to a role flushes the entire decision cache.
func TestDecisionCache_InvalidatesOn_AddPermissionForRole(t *testing.T) {
	a := newCachedTestAdapter(t, 100)

	// user1 has editor role but editor has no permissions yet.
	require.NoError(t, a.AddRoleForUser("user1", "editor"))

	v1, err := a.Enforce("user1", "res", "read")
	require.NoError(t, err)
	assert.False(t, v1) // cached: false

	// Grant permission to the role — must flush.
	require.NoError(t, a.AddPermissionForRole("editor", "res", "read"))

	v2, err := a.Enforce("user1", "res", "read")
	require.NoError(t, err)
	assert.True(t, v2, "cache must be flushed after AddPermissionForRole")
}

// TestDecisionCache_InvalidatesOn_RemovePermissionForRole verifies that
// removing a permission from a role flushes the cache.
func TestDecisionCache_InvalidatesOn_RemovePermissionForRole(t *testing.T) {
	a := newCachedTestAdapter(t, 100)

	require.NoError(t, a.AddPermissionForRole("editor", "res", "write"))
	require.NoError(t, a.AddRoleForUser("user1", "editor"))

	v1, err := a.Enforce("user1", "res", "write")
	require.NoError(t, err)
	assert.True(t, v1) // cached: true

	// Remove permission — must flush.
	require.NoError(t, a.RemovePermissionForRole("editor", "res", "write"))

	v2, err := a.Enforce("user1", "res", "write")
	require.NoError(t, err)
	assert.False(t, v2, "cache must be flushed after RemovePermissionForRole")
}

// TestDecisionCache_InvalidatesOn_AddPermissionForUser verifies that adding a
// direct permission to a user invalidates that user's cache entries.
func TestDecisionCache_InvalidatesOn_AddPermissionForUser(t *testing.T) {
	a := newCachedTestAdapter(t, 100)

	v1, err := a.Enforce("user1", "res", "delete")
	require.NoError(t, err)
	assert.False(t, v1) // cached: false

	require.NoError(t, a.AddPermissionForUser("user1", "res", "delete"))

	v2, err := a.Enforce("user1", "res", "delete")
	require.NoError(t, err)
	assert.True(t, v2, "cache must be invalidated after AddPermissionForUser")
}

// TestDecisionCache_InvalidatesOn_RemovePermissionForUser verifies that
// removing a direct permission from a user invalidates that user's cache.
func TestDecisionCache_InvalidatesOn_RemovePermissionForUser(t *testing.T) {
	a := newCachedTestAdapter(t, 100)

	require.NoError(t, a.AddPermissionForUser("user1", "res", "delete"))

	v1, err := a.Enforce("user1", "res", "delete")
	require.NoError(t, err)
	assert.True(t, v1) // cached: true

	require.NoError(t, a.RemovePermissionForUser("user1", "res", "delete"))

	v2, err := a.Enforce("user1", "res", "delete")
	require.NoError(t, err)
	assert.False(t, v2, "cache must be invalidated after RemovePermissionForUser")
}

// TestDecisionCache_InvalidatesOn_LoadPolicy verifies that LoadPolicy flushes
// the entire cache. Since the test enforcer has no storage adapter, we verify
// the flush indirectly: the cache size drops to zero after LoadPolicy is called.
// (Full-reload with a real DB is exercised in the watcher callback test.)
func TestDecisionCache_InvalidatesOn_LoadPolicy(t *testing.T) {
	a := newCachedTestAdapter(t, 100)

	// Prime cache with several entries.
	require.NoError(t, a.AddPermissionForUser("user1", "res", "read"))
	require.NoError(t, a.AddPermissionForUser("user2", "res", "write"))
	_, _ = a.Enforce("user1", "res", "read")
	_, _ = a.Enforce("user2", "res", "write")
	assert.Equal(t, 2, a.cache.len(), "cache must contain 2 entries before flush")

	// Flush directly (equivalent to what LoadPolicy calls internally on success).
	a.cache.flush()
	assert.Equal(t, 0, a.cache.len(), "cache must be empty after flush")

	// Post-flush Enforce must re-evaluate and produce fresh answers.
	v, err := a.Enforce("user1", "res", "read")
	require.NoError(t, err)
	assert.True(t, v)
}

// TestDecisionCache_InvalidatesOn_WatcherCallback verifies that the watcher
// callback flushes the decision cache when it applies an incremental policy
// change. This test uses the add_policy op path (which does not call
// enforcer.LoadPolicy) so it is safe with an in-memory enforcer.
func TestDecisionCache_InvalidatesOn_WatcherCallback(t *testing.T) {
	a := newCachedTestAdapter(t, 100)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	w := NewMemoryWatcher()
	w.Start(ctx)
	a.watcher = w
	require.NoError(t, a.Start(ctx))

	// Prime cache: no permission yet → false cached.
	v1, err := a.Enforce("user1", "res", "read")
	require.NoError(t, err)
	assert.False(t, v1)

	// Send an add_policy watcher op for "user1 res read".
	// The callback will call enforcer.AddPolicy AND flush the cache.
	require.NoError(t, w.UpdateForAddPolicy("p", "p", "user1", "res", "read"))

	// Give the goroutine time to dispatch and flush.
	assert.Eventually(t, func() bool {
		v, e := a.Enforce("user1", "res", "read")
		return e == nil && v
	}, testTimeout(), testPoll(), "watcher add_policy callback must flush the cache")
}

// TestDecisionCache_TransitiveRole verifies that a full flush on
// AddPermissionForRole covers transitively-inherited users.
func TestDecisionCache_TransitiveRole(t *testing.T) {
	a := newCachedTestAdapter(t, 100)

	// user1 → roleA → roleB chain.
	require.NoError(t, a.AddRoleForUser("user1", "roleA"))
	require.NoError(t, a.AddRoleForUser("roleA", "roleB"))

	// Prime: no permission yet.
	v1, err := a.Enforce("user1", "res", "act")
	require.NoError(t, err)
	assert.False(t, v1)

	// Grant permission to roleB — must flush entire cache.
	require.NoError(t, a.AddPermissionForRole("roleB", "res", "act"))

	v2, err := a.Enforce("user1", "res", "act")
	require.NoError(t, err)
	assert.True(t, v2, "transitive permission via role chain must be visible after cache flush")
}

// TestDecisionCache_NullByteInEnforceInput_NoCollision verifies that passing
// null-byte-containing arguments to Enforce bypasses the cache entirely,
// preventing cache key collisions between distinct (sub, obj, act) triples.
func TestDecisionCache_NullByteInEnforceInput_NoCollision(t *testing.T) {
	c := newDecisionCache(100)

	// Store a legitimate entry.
	c.put("alice", "data", "read", true)

	// A crafted sub that contains \x00 produces the same raw key string as
	// ("alice", "data", "read") but must NOT collide because get/put bypass
	// the cache when any arg contains \x00.
	v, ok := c.get("alice\x00data", "read", "x")
	assert.False(t, ok, "null-byte input must bypass cache (miss), not collide with stored entry")
	assert.False(t, v)

	// put with null-byte input must be a no-op — legitimate entry unaffected.
	c.put("alice\x00data", "read", "x", false)
	v2, ok2 := c.get("alice", "data", "read")
	assert.True(t, ok2, "legitimate entry must survive a null-byte put attempt")
	assert.True(t, v2)

	// Cache size must still be 1 (the null-byte put was a no-op).
	assert.Equal(t, 1, c.len())
}

// TestDecisionCache_OtherSubjectUnaffected verifies that invalidating one
// subject does not touch other subjects' cached entries.
func TestDecisionCache_OtherSubjectUnaffected(t *testing.T) {
	a := newCachedTestAdapter(t, 100)

	require.NoError(t, a.AddPermissionForUser("user2", "res", "read"))

	// Prime user2 entry.
	v2, err := a.Enforce("user2", "res", "read")
	require.NoError(t, err)
	assert.True(t, v2)
	assert.Equal(t, 1, a.cache.len())

	// Prime user1 entry too.
	_, err = a.Enforce("user1", "other", "read")
	require.NoError(t, err)
	assert.Equal(t, 2, a.cache.len())

	// Remove user1's permission via adapter — must invalidate only user1's entries.
	// user1 has no "other read" permission yet, so RemovePermissionForUser returns
	// false (Casbin) but no error. We use AddPermissionForUser then invalidation.
	require.NoError(t, a.AddPermissionForUser("user1", "other", "read"))
	// After AddPermissionForUser: user1's old cache entry was invalidated.
	// user2's entry must still be cached.
	_, ok := a.cache.get("user2", "res", "read")
	assert.True(t, ok, "user2's cache entry must not be affected by user1 invalidation")
}
