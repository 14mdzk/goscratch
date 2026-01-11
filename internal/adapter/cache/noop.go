package cache

import (
	"context"
	"time"

	"github.com/14mdzk/goscratch/internal/port"
)

// NoOpCache implements port.Cache as a no-op (does nothing)
// Used when Redis is disabled
type NoOpCache struct{}

// NewNoOpCache creates a new no-op cache
func NewNoOpCache() *NoOpCache {
	return &NoOpCache{}
}

func (c *NoOpCache) Get(ctx context.Context, key string) ([]byte, error) {
	return nil, port.ErrCacheMiss
}

func (c *NoOpCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return nil
}

func (c *NoOpCache) Delete(ctx context.Context, key string) error {
	return nil
}

func (c *NoOpCache) Exists(ctx context.Context, key string) (bool, error) {
	return false, nil
}

func (c *NoOpCache) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	return nil
}

func (c *NoOpCache) GetJSON(ctx context.Context, key string, dest any) error {
	return port.ErrCacheMiss
}

func (c *NoOpCache) Increment(ctx context.Context, key string) (int64, error) {
	return 0, nil
}

func (c *NoOpCache) Decrement(ctx context.Context, key string) (int64, error) {
	return 0, nil
}

func (c *NoOpCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return nil
}

func (c *NoOpCache) Close() error {
	return nil
}
