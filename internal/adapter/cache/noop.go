package cache

import (
	"context"
	"time"

	"github.com/14mdzk/goscratch/internal/port"
)

// NoOpCache implements port.Cache as a no-op (does nothing).
// It is intended for development and testing only — it must NOT be used in
// production for security-critical adapters (see ADR-006 carve-out).
// Methods that would silently weaken auth controls return ErrCacheUnavailable
// so callers fail closed rather than open.
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

// DeleteByPrefix returns ErrCacheUnavailable on the NoOp implementation.
// ChangePassword calls this to revoke all refresh tokens; the NoOp cannot
// honour that guarantee, so callers must treat this as a hard failure.
func (c *NoOpCache) DeleteByPrefix(ctx context.Context, prefix string) error {
	return port.ErrCacheUnavailable
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
