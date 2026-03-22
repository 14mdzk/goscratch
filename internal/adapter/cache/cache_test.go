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
