package casbin

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestUpdateCallback_FlushesDecisionCache confirms the D3 fog item: every
// watcher event path flushes the decision cache, so a policy change that
// arrives over Redis (or the in-memory watcher) can never serve a stale
// allow/deny afterwards. Unknown payloads fall back to LoadPolicy, which
// flushes too — except with no database handle there is nothing to reload,
// in which case the explicit flush paths below are what matter.
func TestUpdateCallback_FlushesDecisionCache(t *testing.T) {
	for _, msg := range []string{
		`{"op":"add_policy","sec":"p","ptype":"p","params":["carol","data9","read"]}`,
		`{"op":"remove_policy","sec":"p","ptype":"p","params":["carol","data9","read"]}`,
		`{"op":"add_grouping","sec":"g","ptype":"g","params":["carol","admin"]}`,
		`{"op":"remove_grouping","sec":"g","ptype":"g","params":["carol","admin"]}`,
	} {
		a := newTestAdapter(t)
		a.cache = newDecisionCache(100)

		// Seed the cache through the public path.
		_, err := a.enforcer.AddPolicy("alice", "data1", "read")
		require.NoError(t, err)
		allowed, err := a.Enforce("alice", "data1", "read")
		require.NoError(t, err)
		require.True(t, allowed, "fixture policy must allow alice/data1/read")
		assert.Equal(t, 1, a.cache.len(), "Enforce must populate the cache")

		a.makeUpdateCallback()(msg)

		assert.Equal(t, 0, a.cache.len(), "watcher event %q must flush the cache", msg)
	}
}

// TestUpdateCallback_AppliesIncrementalPolicy proves the event path is not
// flush-only: a remote add_policy changes subsequent decisions.
func TestUpdateCallback_AppliesIncrementalPolicy(t *testing.T) {
	a := newTestAdapter(t)
	a.cache = newDecisionCache(100)

	allowed, err := a.Enforce("dave", "data9", "read")
	require.NoError(t, err)
	require.False(t, allowed, "dave must be denied before the remote grant")

	a.makeUpdateCallback()(`{"op":"add_policy","sec":"p","ptype":"p","params":["dave","data9","read"]}`)

	allowed, err = a.Enforce("dave", "data9", "read")
	require.NoError(t, err)
	assert.True(t, allowed, "remote add_policy must take effect without a restart")
}
