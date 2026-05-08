package casbin

import (
	"fmt"
	"testing"

	casbinlib "github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/stretchr/testify/require"
)

// newBenchAdapter creates an in-memory Casbin Adapter with cacheSize entries
// and a seed of numPolicies direct user permissions so the enforcer has real
// work to do on a cache miss.
func newBenchAdapter(b *testing.B, cacheSize, numPolicies int) *Adapter {
	b.Helper()
	m, err := model.NewModelFromString(defaultModel)
	require.NoError(b, err)
	enforcer, err := casbinlib.NewEnforcer(m)
	require.NoError(b, err)

	for i := range numPolicies {
		_, err := enforcer.AddPolicy(fmt.Sprintf("user%d", i), "resource", "read")
		require.NoError(b, err)
	}

	return &Adapter{
		enforcer: enforcer,
		cache:    newDecisionCache(cacheSize),
	}
}

// BenchmarkEnforce_NoCache measures raw Casbin Enforce cost with caching disabled.
func BenchmarkEnforce_NoCache(b *testing.B) {
	a := newBenchAdapter(b, 0, 100)
	b.ResetTimer()
	for i := range b.N {
		sub := fmt.Sprintf("user%d", i%100)
		_, _ = a.Enforce(sub, "resource", "read")
	}
}

// BenchmarkEnforce_Cached measures Enforce with all keys already in the LRU
// (warm cache, single key — best-case cached path).
func BenchmarkEnforce_Cached(b *testing.B) {
	a := newBenchAdapter(b, 10000, 100)
	// Warm the cache with the key we'll be benchmarking.
	_, _ = a.Enforce("user0", "resource", "read")
	b.ResetTimer()
	for range b.N {
		_, _ = a.Enforce("user0", "resource", "read")
	}
}

// BenchmarkEnforce_Cached_RotatingKeys measures the cached path over a set of
// ~100 distinct keys (realistic: different users hitting the hot path).
// All keys fit in the cache so every iteration is a hit after the warm-up.
func BenchmarkEnforce_Cached_RotatingKeys(b *testing.B) {
	const numKeys = 100
	a := newBenchAdapter(b, 10000, numKeys)
	// Warm all keys.
	for i := range numKeys {
		_, _ = a.Enforce(fmt.Sprintf("user%d", i), "resource", "read")
	}
	b.ResetTimer()
	for i := range b.N {
		sub := fmt.Sprintf("user%d", i%numKeys)
		_, _ = a.Enforce(sub, "resource", "read")
	}
}

// BenchmarkEnforce_NoCache_RotatingKeys is the no-cache baseline for the
// rotating-keys scenario above.
func BenchmarkEnforce_NoCache_RotatingKeys(b *testing.B) {
	const numKeys = 100
	a := newBenchAdapter(b, 0, numKeys)
	b.ResetTimer()
	for i := range b.N {
		sub := fmt.Sprintf("user%d", i%numKeys)
		_, _ = a.Enforce(sub, "resource", "read")
	}
}
