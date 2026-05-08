package cache

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/alicebob/miniredis/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// NoOpCache Tests
// =============================================================================

func TestNoOpCache_ImplementsInterface(t *testing.T) {
	var _ port.Cache = (*NoOpCache)(nil)
}

func TestNoOpCache_Get_ReturnsCacheMiss(t *testing.T) {
	c := NewNoOpCache()
	val, err := c.Get(context.Background(), "any-key")
	assert.Nil(t, val)
	assert.ErrorIs(t, err, port.ErrCacheMiss)
}

func TestNoOpCache_Set_ReturnsNil(t *testing.T) {
	c := NewNoOpCache()
	err := c.Set(context.Background(), "key", []byte("value"), time.Minute)
	assert.NoError(t, err)
}

func TestNoOpCache_Delete_ReturnsNil(t *testing.T) {
	c := NewNoOpCache()
	err := c.Delete(context.Background(), "key")
	assert.NoError(t, err)
}

func TestNoOpCache_Exists_ReturnsFalse(t *testing.T) {
	c := NewNoOpCache()
	exists, err := c.Exists(context.Background(), "key")
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestNoOpCache_SetJSON_ReturnsNil(t *testing.T) {
	c := NewNoOpCache()
	err := c.SetJSON(context.Background(), "key", map[string]string{"a": "b"}, time.Minute)
	assert.NoError(t, err)
}

func TestNoOpCache_GetJSON_ReturnsCacheMiss(t *testing.T) {
	c := NewNoOpCache()
	var dest map[string]string
	err := c.GetJSON(context.Background(), "key", &dest)
	assert.ErrorIs(t, err, port.ErrCacheMiss)
}

func TestNoOpCache_Increment_ReturnsZero(t *testing.T) {
	c := NewNoOpCache()
	val, err := c.Increment(context.Background(), "counter")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), val)
}

func TestNoOpCache_Decrement_ReturnsZero(t *testing.T) {
	c := NewNoOpCache()
	val, err := c.Decrement(context.Background(), "counter")
	assert.NoError(t, err)
	assert.Equal(t, int64(0), val)
}

func TestNoOpCache_Expire_ReturnsNil(t *testing.T) {
	c := NewNoOpCache()
	err := c.Expire(context.Background(), "key", time.Minute)
	assert.NoError(t, err)
}

func TestNoOpCache_Close_ReturnsNil(t *testing.T) {
	c := NewNoOpCache()
	err := c.Close()
	assert.NoError(t, err)
}

func TestNoOpCache_DeleteByPrefix_ReturnsErrCacheUnavailable(t *testing.T) {
	c := NewNoOpCache()
	err := c.DeleteByPrefix(context.Background(), "refresh:user:abc:")
	assert.ErrorIs(t, err, port.ErrCacheUnavailable)
}

// =============================================================================
// RedisCache Tests (using miniredis)
// =============================================================================

func newTestRedisCache(t *testing.T) (*RedisCache, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	t.Cleanup(mr.Close)

	rc, err := NewRedisCache(mr.Addr(), "", 0)
	require.NoError(t, err)
	t.Cleanup(func() { rc.Close() })

	return rc, mr
}

func TestRedisCache_ImplementsInterface(t *testing.T) {
	var _ port.Cache = (*RedisCache)(nil)
}

func TestRedisCache_SetAndGet(t *testing.T) {
	rc, _ := newTestRedisCache(t)
	ctx := context.Background()

	err := rc.Set(ctx, "hello", []byte("world"), time.Minute)
	require.NoError(t, err)

	val, err := rc.Get(ctx, "hello")
	require.NoError(t, err)
	assert.Equal(t, []byte("world"), val)
}

func TestRedisCache_Get_CacheMiss(t *testing.T) {
	rc, _ := newTestRedisCache(t)
	ctx := context.Background()

	val, err := rc.Get(ctx, "nonexistent")
	assert.Nil(t, val)
	assert.ErrorIs(t, err, port.ErrCacheMiss)
}

func TestRedisCache_Delete(t *testing.T) {
	rc, _ := newTestRedisCache(t)
	ctx := context.Background()

	err := rc.Set(ctx, "to-delete", []byte("value"), time.Minute)
	require.NoError(t, err)

	err = rc.Delete(ctx, "to-delete")
	require.NoError(t, err)

	_, err = rc.Get(ctx, "to-delete")
	assert.ErrorIs(t, err, port.ErrCacheMiss)
}

func TestRedisCache_Exists(t *testing.T) {
	rc, _ := newTestRedisCache(t)
	ctx := context.Background()

	exists, err := rc.Exists(ctx, "nope")
	require.NoError(t, err)
	assert.False(t, exists)

	err = rc.Set(ctx, "yes", []byte("here"), time.Minute)
	require.NoError(t, err)

	exists, err = rc.Exists(ctx, "yes")
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestRedisCache_SetJSON_GetJSON(t *testing.T) {
	rc, _ := newTestRedisCache(t)
	ctx := context.Background()

	type testStruct struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	input := testStruct{Name: "test", Count: 42}
	err := rc.SetJSON(ctx, "json-key", input, time.Minute)
	require.NoError(t, err)

	var output testStruct
	err = rc.GetJSON(ctx, "json-key", &output)
	require.NoError(t, err)
	assert.Equal(t, input, output)
}

func TestRedisCache_GetJSON_CacheMiss(t *testing.T) {
	rc, _ := newTestRedisCache(t)
	ctx := context.Background()

	var dest map[string]any
	err := rc.GetJSON(ctx, "missing-json", &dest)
	assert.ErrorIs(t, err, port.ErrCacheMiss)
}

func TestRedisCache_GetJSON_InvalidJSON(t *testing.T) {
	rc, _ := newTestRedisCache(t)
	ctx := context.Background()

	// Store raw bytes that are not valid JSON
	err := rc.Set(ctx, "bad-json", []byte("not-json{{{"), time.Minute)
	require.NoError(t, err)

	var dest map[string]any
	err = rc.GetJSON(ctx, "bad-json", &dest)
	assert.Error(t, err)
	// Should be a JSON unmarshal error, not a cache miss
	assert.NotErrorIs(t, err, port.ErrCacheMiss)
}

func TestRedisCache_Increment(t *testing.T) {
	rc, _ := newTestRedisCache(t)
	ctx := context.Background()

	val, err := rc.Increment(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(1), val)

	val, err = rc.Increment(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(2), val)
}

func TestRedisCache_Decrement(t *testing.T) {
	rc, _ := newTestRedisCache(t)
	ctx := context.Background()

	// Set initial value
	err := rc.Set(ctx, "counter", []byte("10"), time.Minute)
	require.NoError(t, err)

	val, err := rc.Decrement(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(9), val)

	val, err = rc.Decrement(ctx, "counter")
	require.NoError(t, err)
	assert.Equal(t, int64(8), val)
}

func TestRedisCache_Expire(t *testing.T) {
	rc, mr := newTestRedisCache(t)
	ctx := context.Background()

	err := rc.Set(ctx, "expiring", []byte("value"), 0) // no TTL initially
	require.NoError(t, err)

	err = rc.Expire(ctx, "expiring", 2*time.Second)
	require.NoError(t, err)

	// Key should still exist
	exists, err := rc.Exists(ctx, "expiring")
	require.NoError(t, err)
	assert.True(t, exists)

	// Fast-forward time in miniredis
	mr.FastForward(3 * time.Second)

	exists, err = rc.Exists(ctx, "expiring")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestRedisCache_Set_WithTTL(t *testing.T) {
	rc, mr := newTestRedisCache(t)
	ctx := context.Background()

	err := rc.Set(ctx, "ttl-key", []byte("value"), 1*time.Second)
	require.NoError(t, err)

	// Key should exist
	val, err := rc.Get(ctx, "ttl-key")
	require.NoError(t, err)
	assert.Equal(t, []byte("value"), val)

	// Fast-forward past TTL
	mr.FastForward(2 * time.Second)

	_, err = rc.Get(ctx, "ttl-key")
	assert.ErrorIs(t, err, port.ErrCacheMiss)
}

func TestRedisCache_SetJSON_MarshalError(t *testing.T) {
	rc, _ := newTestRedisCache(t)
	ctx := context.Background()

	// Channels cannot be marshalled to JSON
	err := rc.SetJSON(ctx, "bad", make(chan int), time.Minute)
	assert.Error(t, err)

	// Verify key was not set
	_, getErr := rc.Get(ctx, "bad")
	assert.ErrorIs(t, getErr, port.ErrCacheMiss)
}

func TestRedisCache_DeleteByPrefix(t *testing.T) {
	rc, _ := newTestRedisCache(t)
	ctx := context.Background()

	// Store three keys with the same prefix
	require.NoError(t, rc.Set(ctx, "refresh:user:u1:aaa", []byte("v1"), time.Minute))
	require.NoError(t, rc.Set(ctx, "refresh:user:u1:bbb", []byte("v2"), time.Minute))
	require.NoError(t, rc.Set(ctx, "refresh:user:u2:ccc", []byte("v3"), time.Minute)) // different user

	// Delete by prefix for u1 only
	err := rc.DeleteByPrefix(ctx, "refresh:user:u1:")
	require.NoError(t, err)

	// u1 keys are gone
	_, err = rc.Get(ctx, "refresh:user:u1:aaa")
	assert.ErrorIs(t, err, port.ErrCacheMiss)
	_, err = rc.Get(ctx, "refresh:user:u1:bbb")
	assert.ErrorIs(t, err, port.ErrCacheMiss)

	// u2 key is untouched
	val, err := rc.Get(ctx, "refresh:user:u2:ccc")
	require.NoError(t, err)
	assert.Equal(t, []byte("v3"), val)
}

func TestRedisCache_DeleteByPrefix_NoMatchingKeys(t *testing.T) {
	rc, _ := newTestRedisCache(t)
	ctx := context.Background()

	// Should not error even if no keys match
	err := rc.DeleteByPrefix(ctx, "refresh:user:nonexistent:")
	assert.NoError(t, err)
}

func TestRedisCache_SetAndGet_BinaryData(t *testing.T) {
	rc, _ := newTestRedisCache(t)
	ctx := context.Background()

	data := []byte{0x00, 0x01, 0xFF, 0xFE, 0x80}
	err := rc.Set(ctx, "binary", data, time.Minute)
	require.NoError(t, err)

	val, err := rc.Get(ctx, "binary")
	require.NoError(t, err)
	assert.Equal(t, data, val)
}

func TestRedisCache_SetJSON_GetJSON_ComplexStruct(t *testing.T) {
	rc, _ := newTestRedisCache(t)
	ctx := context.Background()

	input := map[string]any{
		"nested": map[string]any{
			"key": "value",
		},
		"list": []any{1.0, 2.0, 3.0},
	}

	err := rc.SetJSON(ctx, "complex", input, time.Minute)
	require.NoError(t, err)

	var output map[string]any
	err = rc.GetJSON(ctx, "complex", &output)
	require.NoError(t, err)

	// Re-marshal both for comparison since json.Number vs float64 can differ
	expectedJSON, _ := json.Marshal(input)
	actualJSON, _ := json.Marshal(output)
	assert.JSONEq(t, string(expectedJSON), string(actualJSON))
}

// =============================================================================
// SlidingWindowAllow Tests
// =============================================================================

func TestRedisSlidingWindow_AllowsUnderLimit(t *testing.T) {
	rc, _ := newTestRedisCache(t)
	ctx := context.Background()

	const max = 5
	window := 10 * time.Second
	key := "ratelimit:test:under"

	// N-1 requests — all must be allowed.
	// The Lua script computes remaining = max - count_before_add - 1.
	// Before request 1: count=0 → remaining = max-1.
	// Before request 2: count=1 → remaining = max-2. etc.
	for i := 0; i < max-1; i++ {
		allowed, remaining, retryAfter, err := rc.SlidingWindowAllow(ctx, key, max, window)
		require.NoError(t, err)
		assert.True(t, allowed, "request %d should be allowed", i+1)
		assert.Equal(t, 0, retryAfter)
		assert.Equal(t, max-i-1, remaining, "remaining after request %d", i+1)
	}
}

func TestRedisSlidingWindow_DeniesAtLimit(t *testing.T) {
	rc, mr := newTestRedisCache(t)
	ctx := context.Background()

	const max = 3
	window := 10 * time.Second
	key := "ratelimit:test:deny"

	// Exhaust the limit
	for i := 0; i < max; i++ {
		allowed, _, _, err := rc.SlidingWindowAllow(ctx, key, max, window)
		require.NoError(t, err)
		assert.True(t, allowed, "request %d should be allowed before limit", i+1)
	}

	// Next request must be denied
	allowed, remaining, retryAfter, err := rc.SlidingWindowAllow(ctx, key, max, window)
	require.NoError(t, err)
	assert.False(t, allowed, "request at limit should be denied")
	assert.Equal(t, 0, remaining)
	assert.GreaterOrEqual(t, retryAfter, 1, "retry-after must be >= 1s")

	// miniredis is used just to ensure the key exists; no fast-forward needed here.
	_ = mr
}

func TestRedisSlidingWindow_RecoversAfterWindowSlides(t *testing.T) {
	rc, mr := newTestRedisCache(t)
	ctx := context.Background()

	const max = 2
	window := 5 * time.Second
	key := "ratelimit:test:recover"

	// Exhaust the limit
	for i := 0; i < max; i++ {
		allowed, _, _, err := rc.SlidingWindowAllow(ctx, key, max, window)
		require.NoError(t, err)
		assert.True(t, allowed)
	}

	// Denied at limit
	allowed, _, _, err := rc.SlidingWindowAllow(ctx, key, max, window)
	require.NoError(t, err)
	assert.False(t, allowed)

	// Fast-forward past the window so all old entries fall out
	mr.FastForward(window + time.Second)

	// Should be allowed again
	allowed, remaining, _, err := rc.SlidingWindowAllow(ctx, key, max, window)
	require.NoError(t, err)
	assert.True(t, allowed, "request after window slide should be allowed")
	assert.Equal(t, max-1, remaining)
}

func TestNoOpCache_SlidingWindowAllow_AlwaysAllows(t *testing.T) {
	c := NewNoOpCache()
	ctx := context.Background()

	allowed, remaining, retryAfter, err := c.SlidingWindowAllow(ctx, "any-key", 10, time.Minute)
	assert.NoError(t, err)
	assert.True(t, allowed)
	assert.Equal(t, 10, remaining)
	assert.Equal(t, 0, retryAfter)
}
