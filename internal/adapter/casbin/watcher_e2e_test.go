package casbin

// End-to-end watcher tests: two parallel Adapter instances share a watcher.
// Mutate policy on instance A; assert instance B sees the change within 1s
// via the incremental-apply path (not the full-reload backstop).
//
// MemoryWatcher: single-instance, in-proc broadcast.
//   Instance A owns the MemoryWatcher (A.Start wires A's callback and sets the
//   enforcer watcher). B is a second enforcer whose callback is registered
//   directly on the watcher WITHOUT calling enforcer.SetWatcher — this prevents
//   the "re-publish on callback" cascade that would arise if B's enforcer also
//   has the watcher set.
//
// RedisWatcher: multi-instance, pub/sub.
//   A is a plain in-memory enforcer (NoopWatcher, no subscription goroutine).
//   A manually publishes the watcher op to Redis after mutating its enforcer.
//   B has a RedisWatcher subscribed to the same channel; B's callback is
//   registered directly (no enforcer.SetWatcher on B) to prevent re-publish.
//
// Thread-safety notes:
//   Both designs avoid the "B's enforcer calls watcher.UpdateForAddPolicy →
//   re-publish → B's callback fires again" cascade. The notifyWatcher shim lets
//   tests wait for each callback to complete before reading B's enforcer,
//   eliminating concurrent enforcer access.

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	casbinlib "github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/persist"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Verify notifyWatcher satisfies persist.WatcherEx at compile time.
// This is required so that casbin's enforcer.SetWatcher detects it as WatcherEx
// and does NOT replace the adapter's incremental callback with a generic
// LoadPolicy call.
var _ persist.WatcherEx = (*notifyWatcher)(nil)

const (
	e2eTimeout = 1 * time.Second
	// backstopSafe is long enough that the backstop tick cannot fire during
	// the test, ensuring the only propagation path is the watcher.
	backstopSafe = 24 * time.Hour
)

// notifyWatcher wraps a real WatcherEx and signals a channel each time the
// registered callback finishes. This lets tests wait for the callback to
// complete before querying the enforcer, avoiding concurrent enforcer access
// that would trigger the race detector.
//
// notifyWatcher itself implements WatcherEx so that casbin's enforcer.SetWatcher
// detects it as WatcherEx and does NOT replace the callback with a generic
// LoadPolicy call.
type notifyWatcher struct {
	inner persist.WatcherEx
	done  chan struct{}
}

func newNotifyWatcher(inner persist.WatcherEx) *notifyWatcher {
	return &notifyWatcher{
		inner: inner,
		done:  make(chan struct{}, 64),
	}
}

func (nw *notifyWatcher) SetUpdateCallback(f func(string)) error {
	wrapped := func(msg string) {
		f(msg)
		select {
		case nw.done <- struct{}{}:
		default:
		}
	}
	return nw.inner.SetUpdateCallback(wrapped)
}

func (nw *notifyWatcher) Update() error { return nw.inner.Update() }
func (nw *notifyWatcher) Close()        { nw.inner.Close() }

// WatcherEx delegation — these methods are called by the enforcer after a
// mutation (publish side). They just delegate to the inner watcher.
func (nw *notifyWatcher) UpdateForAddPolicy(sec, ptype string, params ...string) error {
	return nw.inner.UpdateForAddPolicy(sec, ptype, params...)
}

func (nw *notifyWatcher) UpdateForRemovePolicy(sec, ptype string, params ...string) error {
	return nw.inner.UpdateForRemovePolicy(sec, ptype, params...)
}

func (nw *notifyWatcher) UpdateForRemoveFilteredPolicy(sec, ptype string, fieldIndex int, fieldValues ...string) error {
	return nw.inner.UpdateForRemoveFilteredPolicy(sec, ptype, fieldIndex, fieldValues...)
}

func (nw *notifyWatcher) UpdateForSavePolicy(m model.Model) error {
	return nw.inner.UpdateForSavePolicy(m)
}

func (nw *notifyWatcher) UpdateForAddPolicies(sec, ptype string, rules ...[]string) error {
	return nw.inner.UpdateForAddPolicies(sec, ptype, rules...)
}

func (nw *notifyWatcher) UpdateForRemovePolicies(sec, ptype string, rules ...[]string) error {
	return nw.inner.UpdateForRemovePolicies(sec, ptype, rules...)
}

// waitForUpdate blocks until the watcher callback fires or the deadline expires.
func (nw *notifyWatcher) waitForUpdate(t *testing.T) {
	t.Helper()
	select {
	case <-nw.done:
	case <-time.After(e2eTimeout):
		t.Fatal("timeout waiting for watcher callback to fire")
	}
}

// newBareEnforcer creates a fresh Casbin Enforcer with no storage adapter.
func newBareEnforcer(t *testing.T) *casbinlib.Enforcer {
	t.Helper()
	m, err := model.NewModelFromString(defaultModel)
	require.NoError(t, err)
	e, err := casbinlib.NewEnforcer(m)
	require.NoError(t, err)
	return e
}

// newPublisherAdapter builds a full Adapter with the given watcher (A's role:
// mutate policy and publish via the watcher).
func newPublisherAdapter(t *testing.T, w persist.Watcher) *Adapter {
	t.Helper()
	return &Adapter{
		enforcer:       newBareEnforcer(t),
		db:             nil,
		reloadInterval: backstopSafe,
		watcher:        w,
		cache:          newDecisionCache(10_000),
	}
}

// applyOp is the incremental callback logic (mirrors makeUpdateCallback) used
// by subscriber enforcers. It is implemented in-package (same package as casbin)
// so it can access the unexported watcherOp type and stringsToIfaces helper.
// Unlike makeUpdateCallback, applyOp does NOT call enforcer.SetWatcher, so the
// enforcer never re-publishes when the callback fires.
func applyOp(enforcer *casbinlib.Enforcer, msg string) {
	var op watcherOp
	if err := json.Unmarshal([]byte(msg), &op); err != nil {
		_ = enforcer.LoadPolicy()
		return
	}
	ifaces := stringsToIfaces(op.Params)
	switch op.Op {
	case "add_policy":
		_, _ = enforcer.AddPolicy(ifaces...)
	case "remove_policy":
		_, _ = enforcer.RemovePolicy(ifaces...)
	case "add_grouping":
		_, _ = enforcer.AddGroupingPolicy(ifaces...)
	case "remove_grouping":
		_, _ = enforcer.RemoveGroupingPolicy(ifaces...)
	default:
		_ = enforcer.LoadPolicy()
	}
}

// publishOp publishes a watcher op directly to a Redis channel, simulating
// what a RedisWatcher would publish when an Adapter mutates policy.
func publishOp(t *testing.T, ctx context.Context, client *redis.Client, channel, op, ptype string, params []string) {
	t.Helper()
	msg := encodeOp(op, "p", ptype, params)
	require.NoError(t, client.Publish(ctx, channel, msg).Err())
}

// =============================================================================
// MemoryWatcher e2e
// =============================================================================

// TestWatcherE2E_Memory verifies that two in-process Adapter instances sharing
// a single MemoryWatcher propagate an incremental add-policy from A to B
// without requiring a full LoadPolicy call.
//
// Design: A owns the MemoryWatcher (A.Start wires the watcher and sets it on
// A's enforcer). B's callback is registered directly on the watcher via
// notifyWatcher.SetUpdateCallback — this bypasses enforcer.SetWatcher so B's
// enforcer does not re-publish when the callback fires.
//
// Proof that the watcher path (not the backstop) is responsible:
//   - backstopSafe interval prevents the backstop tick from firing.
//   - B's enforcer starts with no policy and never calls LoadPolicy.
//   - The only way B can see the rule is via the watcher callback.
func TestWatcherE2E_Memory(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// Shared watcher — in-proc broadcast.
	inner := NewMemoryWatcher()
	inner.Start(ctx)

	// A: full adapter with watcher (A.Start sets watcher on A's enforcer).
	adapterA := newPublisherAdapter(t, inner)
	require.NoError(t, adapterA.Start(ctx))

	// B: bare enforcer. Register B's callback directly on the watcher without
	// calling enforcer.SetWatcher — prevents re-publish cascade.
	enforcerB := newBareEnforcer(t)
	nw := newNotifyWatcher(inner)
	require.NoError(t, nw.SetUpdateCallback(func(msg string) { applyOp(enforcerB, msg) }))

	t.Cleanup(func() {
		cancel()
		if adapterA.cancel != nil {
			adapterA.cancel()
		}
		inner.Close()
	})

	// Pre-condition: B has no policy.
	allowed, err := enforcerB.Enforce("e2e-role", "orders", "delete")
	require.NoError(t, err)
	assert.False(t, allowed, "pre-condition: B must not have the rule before A mutates")

	// Action: A adds a permission via the adapter.
	// This causes A's enforcer to call watcher.UpdateForAddPolicy, which
	// enqueues the encoded op in the MemoryWatcher channel.
	require.NoError(t, adapterA.AddPermissionForRole("e2e-role", "orders", "delete"))

	// Wait for B's callback to complete before reading B's enforcer.
	nw.waitForUpdate(t)

	// Assertion: B must see the rule — exclusively via the watcher callback
	// (incremental add_policy path), not a full reload.
	allowed, err = enforcerB.Enforce("e2e-role", "orders", "delete")
	require.NoError(t, err)
	assert.True(t, allowed,
		"MemoryWatcher e2e: B must enforce the rule via the watcher path")
}

// TestWatcherE2E_Memory_Remove verifies that a remove-policy op propagates from
// A to B via the MemoryWatcher incremental path.
func TestWatcherE2E_Memory_Remove(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	inner := NewMemoryWatcher()
	inner.Start(ctx)

	adapterA := newPublisherAdapter(t, inner)
	require.NoError(t, adapterA.Start(ctx))

	enforcerB := newBareEnforcer(t)
	nw := newNotifyWatcher(inner)
	require.NoError(t, nw.SetUpdateCallback(func(msg string) { applyOp(enforcerB, msg) }))

	t.Cleanup(func() {
		cancel()
		if adapterA.cancel != nil {
			adapterA.cancel()
		}
		inner.Close()
	})

	// Seed: A adds a rule, B receives via watcher.
	require.NoError(t, adapterA.AddPermissionForRole("e2e-role", "invoices", "read"))
	nw.waitForUpdate(t)

	// Verify B has the rule.
	allowed, err := enforcerB.Enforce("e2e-role", "invoices", "read")
	require.NoError(t, err)
	require.True(t, allowed, "setup: B must see initial rule before remove test")

	// Action: A removes the permission.
	require.NoError(t, adapterA.RemovePermissionForRole("e2e-role", "invoices", "read"))
	nw.waitForUpdate(t)

	// Assertion: B must lose the rule via the incremental remove path.
	allowed, err = enforcerB.Enforce("e2e-role", "invoices", "read")
	require.NoError(t, err)
	assert.False(t, allowed,
		"MemoryWatcher e2e remove: B must lose the rule via the watcher path")
}

// =============================================================================
// RedisWatcher e2e
// =============================================================================

// newMiniredisClient starts a miniredis server and returns a go-redis client.
func newMiniredisClient(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return client, mr
}

// TestWatcherE2E_Redis verifies that when a watcher op is published to a Redis
// channel, a subscribed RedisWatcher applies the incremental add-policy to B's
// enforcer within the deadline.
//
// Design: A is a plain adapter with NoopWatcher. A mutates its own enforcer
// and manually publishes the op to Redis (no self-subscription race). B's
// RedisWatcher subscribes and applies the callback — B's callback is registered
// directly (no enforcer.SetWatcher on B) to prevent re-publish.
//
// Proof that the watcher path (not the backstop) is responsible:
//   - backstopSafe prevents the backstop tick from firing.
//   - B's enforcer starts with no policy and never calls LoadPolicy.
//   - The only way B can see the rule is via the Redis subscription callback.
func TestWatcherE2E_Redis(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client, _ := newMiniredisClient(t)

	const channel = "casbin:policy:update:v1"

	// A: plain adapter with NoopWatcher (no subscription goroutine).
	adapterA := newPublisherAdapter(t, NewNoopWatcher())
	require.NoError(t, adapterA.Start(ctx))

	// B: subscriber. Register B's callback on the RedisWatcher directly —
	// no enforcer.SetWatcher on B to avoid the re-publish cascade.
	innerB, err := NewRedisWatcher(ctx, client, channel)
	require.NoError(t, err)
	enforcerB := newBareEnforcer(t)
	watcherB := newNotifyWatcher(innerB)
	require.NoError(t, watcherB.SetUpdateCallback(func(msg string) { applyOp(enforcerB, msg) }))

	t.Cleanup(func() {
		_ = adapterA.Close()
		watcherB.Close()
	})

	// Pre-condition: B has no policy.
	allowed, err := enforcerB.Enforce("e2e-redis-role", "shipments", "create")
	require.NoError(t, err)
	assert.False(t, allowed, "pre-condition: B must not have the rule before A mutates")

	// Action: A mutates its enforcer and publishes the watcher op to Redis.
	require.NoError(t, adapterA.AddPermissionForRole("e2e-redis-role", "shipments", "create"))
	publishOp(t, ctx, client, channel, "add_policy", "p", []string{"e2e-redis-role", "shipments", "create"})

	// Wait for B's callback to complete before reading B's enforcer.
	watcherB.waitForUpdate(t)

	// Assertion: B's enforcer must see the rule via the pub/sub path.
	allowed, err = enforcerB.Enforce("e2e-redis-role", "shipments", "create")
	require.NoError(t, err)
	assert.True(t, allowed,
		"RedisWatcher e2e: B must enforce the rule via pub/sub path")
}

// TestWatcherE2E_Redis_Remove verifies that a remove-policy op propagates to
// B via the RedisWatcher incremental path.
func TestWatcherE2E_Redis_Remove(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client, _ := newMiniredisClient(t)

	const channel = "casbin:policy:update:v1"

	adapterA := newPublisherAdapter(t, NewNoopWatcher())
	require.NoError(t, adapterA.Start(ctx))

	innerB, err := NewRedisWatcher(ctx, client, channel)
	require.NoError(t, err)
	enforcerB := newBareEnforcer(t)
	watcherB := newNotifyWatcher(innerB)
	require.NoError(t, watcherB.SetUpdateCallback(func(msg string) { applyOp(enforcerB, msg) }))

	t.Cleanup(func() {
		_ = adapterA.Close()
		watcherB.Close()
	})

	// Seed: add rule on A and propagate to B via pub/sub.
	require.NoError(t, adapterA.AddPermissionForRole("e2e-redis-role", "returns", "approve"))
	publishOp(t, ctx, client, channel, "add_policy", "p", []string{"e2e-redis-role", "returns", "approve"})
	watcherB.waitForUpdate(t)

	// Verify B has the rule.
	allowed, err := enforcerB.Enforce("e2e-redis-role", "returns", "approve")
	require.NoError(t, err)
	require.True(t, allowed, "setup: B must see initial rule before remove test")

	// Action: A removes and publishes the remove op.
	require.NoError(t, adapterA.RemovePermissionForRole("e2e-redis-role", "returns", "approve"))
	publishOp(t, ctx, client, channel, "remove_policy", "p", []string{"e2e-redis-role", "returns", "approve"})
	watcherB.waitForUpdate(t)

	// Assertion: B must lose the rule via the incremental remove path.
	allowed, err = enforcerB.Enforce("e2e-redis-role", "returns", "approve")
	require.NoError(t, err)
	assert.False(t, allowed,
		"RedisWatcher e2e remove: B must lose the rule via pub/sub path")
}

// TestWatcherE2E_Redis_IsolatedChannels verifies that two subscriber instances
// on different Redis channels do not cross-contaminate each other's policy state.
func TestWatcherE2E_Redis_IsolatedChannels(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	client, _ := newMiniredisClient(t)

	const (
		chPair1 = "casbin:policy:update:v1:pair1"
		chPair2 = "casbin:policy:update:v1:pair2"
	)

	// B1: subscribes to pair1 channel.
	innerB1, err := NewRedisWatcher(ctx, client, chPair1)
	require.NoError(t, err)
	enforcerB1 := newBareEnforcer(t)
	watcherB1 := newNotifyWatcher(innerB1)
	require.NoError(t, watcherB1.SetUpdateCallback(func(msg string) { applyOp(enforcerB1, msg) }))

	// B2: subscribes to pair2 channel.
	innerB2, err := NewRedisWatcher(ctx, client, chPair2)
	require.NoError(t, err)
	enforcerB2 := newBareEnforcer(t)
	require.NoError(t, innerB2.SetUpdateCallback(func(msg string) { applyOp(enforcerB2, msg) }))

	t.Cleanup(func() {
		watcherB1.Close()
		innerB2.Close()
	})

	// Publish to pair1 channel only.
	publishOp(t, ctx, client, chPair1, "add_policy", "p", []string{"pair1-role", "widget", "read"})
	watcherB1.waitForUpdate(t)

	// B1 must see the change.
	allowed, err := enforcerB1.Enforce("pair1-role", "widget", "read")
	require.NoError(t, err)
	assert.True(t, allowed, "pair1 B must see the rule")

	// B2 must NOT be affected (different channel).
	// Allow a brief window in case any message leaked.
	time.Sleep(50 * time.Millisecond)
	allowed, err = enforcerB2.Enforce("pair1-role", "widget", "read")
	require.NoError(t, err)
	assert.False(t, allowed, "pair2 B must not receive pair1 messages (different channel)")
}
