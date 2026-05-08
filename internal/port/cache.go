package port

import (
	"context"
	"time"
)

// Cache defines the interface for cache operations
type Cache interface {
	// Get retrieves a value from the cache
	Get(ctx context.Context, key string) ([]byte, error)

	// Set stores a value in the cache with a TTL
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error

	// Delete removes a value from the cache
	Delete(ctx context.Context, key string) error

	// DeleteByPrefix removes all keys with the given prefix.
	// NoOpCache returns ErrCacheUnavailable; Redis uses SCAN + DEL.
	DeleteByPrefix(ctx context.Context, prefix string) error

	// Exists checks if a key exists in the cache
	Exists(ctx context.Context, key string) (bool, error)

	// SetJSON stores a JSON-serializable value
	SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error

	// GetJSON retrieves and unmarshals a JSON value
	GetJSON(ctx context.Context, key string, dest any) error

	// Increment increments an integer value
	Increment(ctx context.Context, key string) (int64, error)

	// Decrement decrements an integer value
	Decrement(ctx context.Context, key string) (int64, error)

	// Expire sets a TTL on an existing key
	Expire(ctx context.Context, key string, ttl time.Duration) error

	// SlidingWindowAllow checks a sliding-window rate limit using a sorted-set
	// of timestamps.  The operation is atomic (single Lua script on Redis).
	// Returns (allowed, remaining, retryAfterSeconds, error).
	// NoOpCache always returns (true, maxReqs, 0, nil) — use the in-memory backend
	// for single-instance rate limiting without Redis.
	SlidingWindowAllow(ctx context.Context, key string, maxReqs int, window time.Duration) (bool, int, int, error)

	// Close closes the cache connection
	Close() error
}

// ErrCacheMiss is returned when a key is not found in the cache
var ErrCacheMiss = CacheMissError{}

type CacheMissError struct{}

func (e CacheMissError) Error() string {
	return "cache: key not found"
}

// ErrCacheUnavailable is returned by security-critical operations (refresh-token
// gating, DeleteByPrefix on NoOp) to signal that the cache backend is not
// functional. Callers on auth paths must treat this as a hard failure rather
// than a miss.
var ErrCacheUnavailable = CacheUnavailableError{}

// CacheUnavailableError is the concrete type for ErrCacheUnavailable.
type CacheUnavailableError struct{}

func (e CacheUnavailableError) Error() string {
	return "cache: backend unavailable"
}
