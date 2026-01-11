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

	// Close closes the cache connection
	Close() error
}

// ErrCacheMiss is returned when a key is not found in the cache
var ErrCacheMiss = CacheMissError{}

type CacheMissError struct{}

func (e CacheMissError) Error() string {
	return "cache: key not found"
}
