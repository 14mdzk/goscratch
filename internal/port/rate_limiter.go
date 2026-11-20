package port

import (
	"context"
	"time"
)

// RateLimiter checks whether a request key is within a sliding-window limit.
// RedisCache implements this via an atomic Lua sorted-set script; cache
// backends that only provide key-value storage do not need to implement it.
type RateLimiter interface {
	SlidingWindowAllow(ctx context.Context, key string, maxReqs int, window time.Duration) (allowed bool, remaining, retryAfterSeconds int, err error)
}
