package casbin

import (
	"fmt"
	"sync"
	"testing"

	casbinlib "github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/persist"
)

// memoryAdapter is a no-op persist.Adapter so LoadPolicy() has something to load
// from without a database.
type memoryAdapter struct {
	rules [][]string
}

func (a *memoryAdapter) LoadPolicy(m model.Model) error { return nil }
func (a *memoryAdapter) SavePolicy(m model.Model) error { return nil }
func (a *memoryAdapter) AddPolicy(sec, ptype string, rule []string) error {
	a.rules = append(a.rules, rule)
	return nil
}
func (a *memoryAdapter) RemovePolicy(sec, ptype string, rule []string) error { return nil }
func (a *memoryAdapter) RemoveFilteredPolicy(sec, ptype string, fieldIndex int, fieldValues ...string) error {
	return nil
}

func newPlainTestEnforcer(tb testing.TB) (*casbinlib.Enforcer, *Adapter) {
	tb.Helper()
	m, err := model.NewModelFromString(defaultModel)
	if err != nil {
		tb.Fatal(err)
	}
	e, err := casbinlib.NewEnforcer(m, &memoryAdapter{})
	if err != nil {
		tb.Fatal(err)
	}
	return e, &Adapter{enforcer: e, cache: newDecisionCache(1000)}
}

// TestRaceProbe_PlainEnforcer_EnforceVsMutation drives the Adapter's real read
// path (Enforce + decision cache) against its real write paths (use-case
// mutations + the watcher callback) on the plain *casbin.Enforcer it holds today.
func TestRaceProbe_PlainEnforcer_EnforceVsMutation(t *testing.T) {
	_, a := newPlainTestEnforcer(t)
	cb := a.makeUpdateCallback()

	const readers = 8
	const iters = 400

	var failOpen int64
	var mu sync.Mutex

	var wg sync.WaitGroup
	for i := range readers {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range iters {
				// A subject that is never granted anything must never be allowed.
				allowed, _ := a.Enforce("ghost", fmt.Sprintf("obj%d", j%20), "read")
				if allowed {
					mu.Lock()
					failOpen++
					mu.Unlock()
				}
				_, _ = a.Enforce(fmt.Sprintf("u%d", id), fmt.Sprintf("obj%d", j%20), "read")
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := range iters {
			_ = a.AddPermissionForRole("admin", fmt.Sprintf("obj%d", j%20), "read")
			_ = a.RemovePermissionForRole("admin", fmt.Sprintf("obj%d", j%20), "read")
			_ = a.AddRoleForUser(fmt.Sprintf("u%d", j%readers), "admin")
			cb(encodeOp("add_policy", "p", "p", []string{"admin", fmt.Sprintf("cb%d", j), "read"}))
		}
	}()

	wg.Wait()
	if failOpen > 0 {
		t.Errorf("fail-open decisions observed: %d", failOpen)
	}
}

// TestRaceProbe_PlainEnforcer_LoadPolicyVsEnforce races the backstop reload
// (model pointer swap + cache flush) against request-path readers.
func TestRaceProbe_PlainEnforcer_LoadPolicyVsEnforce(t *testing.T) {
	_, a := newPlainTestEnforcer(t)

	const readers = 8
	const iters = 400

	var wg sync.WaitGroup
	for i := range readers {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range iters {
				_, _ = a.Enforce(fmt.Sprintf("u%d", id), fmt.Sprintf("obj%d", j%20), "read")
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for range iters {
			_ = a.LoadPolicy()
		}
	}()

	wg.Wait()
}

// TestRaceProbe_SyncedEnforcer_EnforceVsMutation is the control: the same access
// pattern, with the lock-wrapped SyncedEnforcer instead of the plain one.
func TestRaceProbe_SyncedEnforcer_EnforceVsMutation(t *testing.T) {
	m, err := model.NewModelFromString(defaultModel)
	if err != nil {
		t.Fatal(err)
	}
	se, err := casbinlib.NewSyncedEnforcer(m, &memoryAdapter{})
	if err != nil {
		t.Fatal(err)
	}

	const readers = 8
	const iters = 400

	var wg sync.WaitGroup
	for i := range readers {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range iters {
				_, _ = se.Enforce("ghost", fmt.Sprintf("obj%d", j%20), "read")
				_, _ = se.Enforce(fmt.Sprintf("u%d", id), fmt.Sprintf("obj%d", j%20), "read")
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := range iters {
			_, _ = se.AddPolicy("admin", fmt.Sprintf("obj%d", j%20), "read")
			_, _ = se.RemovePolicy("admin", fmt.Sprintf("obj%d", j%20), "read")
			_, _ = se.AddGroupingPolicy(fmt.Sprintf("u%d", j%readers), "admin")
			_ = se.LoadPolicy()
		}
	}()

	wg.Wait()
}

// Benchmarks: parallel enforce cost, plain vs synced, all cache misses.
func benchEnforceParallel(b *testing.B, synced bool) {
	m, err := model.NewModelFromString(defaultModel)
	if err != nil {
		b.Fatal(err)
	}
	for i := range 1000 {
		m.AddPolicy("p", "p", []string{fmt.Sprintf("user%d", i), "resource", "read"})
	}

	var plain *casbinlib.Enforcer
	var sync_ *casbinlib.SyncedEnforcer
	if synced {
		sync_, err = casbinlib.NewSyncedEnforcer(m, &memoryAdapter{})
	} else {
		plain, err = casbinlib.NewEnforcer(m, &memoryAdapter{})
	}
	if err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sub := fmt.Sprintf("user%d", i%1000)
			if synced {
				_, _ = sync_.Enforce(sub, "resource", "read")
			} else {
				_, _ = plain.Enforce(sub, "resource", "read")
			}
			i++
		}
	})
}

func BenchmarkProbe_EnforceParallel_Plain(b *testing.B)  { benchEnforceParallel(b, false) }
func BenchmarkProbe_EnforceParallel_Synced(b *testing.B) { benchEnforceParallel(b, true) }

// benchEnforceParallelAdapterMutex measures option (b): the plain enforcer
// guarded by an adapter-level RWMutex on the read path.
func benchEnforceParallelAdapterMutex(b *testing.B) {
	m, err := model.NewModelFromString(defaultModel)
	if err != nil {
		b.Fatal(err)
	}
	for i := range 1000 {
		m.AddPolicy("p", "p", []string{fmt.Sprintf("user%d", i), "resource", "read"})
	}
	e, err := casbinlib.NewEnforcer(m, &memoryAdapter{})
	if err != nil {
		b.Fatal(err)
	}
	var mu sync.RWMutex

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sub := fmt.Sprintf("user%d", i%1000)
			mu.RLock()
			_, _ = e.Enforce(sub, "resource", "read")
			mu.RUnlock()
			i++
		}
	})
}

func BenchmarkProbe_EnforceParallel_AdapterRWMutex(b *testing.B) { benchEnforceParallelAdapterMutex(b) }

var _ persist.Adapter = (*memoryAdapter)(nil)
