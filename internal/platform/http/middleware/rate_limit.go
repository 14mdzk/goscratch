package middleware

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/14mdzk/goscratch/internal/port"
	"github.com/14mdzk/goscratch/pkg/apperr"
	"github.com/14mdzk/goscratch/pkg/response"
	"github.com/gofiber/fiber/v2"
)

// RateLimitConfig holds rate limiting configuration
type RateLimitConfig struct {
	Max      int                     // Max requests per window (default: 100)
	Window   time.Duration           // Time window (default: 1 minute)
	KeyFunc  func(*fiber.Ctx) string // Custom key extraction
	UseRedis bool                    // Use Redis backend
}

// rateLimitBackend defines the interface for rate limit storage backends
type rateLimitBackend interface {
	// Allow checks if a request is allowed and returns the current count
	Allow(ctx context.Context, key string, max int, window time.Duration) (allowed bool, remaining int, resetAt time.Time, err error)
}

// RateLimit returns a rate limiting middleware
func RateLimit(cfg RateLimitConfig, cache port.Cache) fiber.Handler {
	// Apply defaults
	if cfg.Max <= 0 {
		cfg.Max = 100
	}
	if cfg.Window <= 0 {
		cfg.Window = 1 * time.Minute
	}
	if cfg.KeyFunc == nil {
		cfg.KeyFunc = defaultKeyFunc
	}

	var backend rateLimitBackend
	if cfg.UseRedis && cache != nil {
		backend = newRedisBackend(cache)
	} else {
		backend = newMemoryBackend()
	}

	return func(c *fiber.Ctx) error {
		key := cfg.KeyFunc(c)

		allowed, remaining, resetAt, err := backend.Allow(c.UserContext(), key, cfg.Max, cfg.Window)
		if err != nil {
			// On error, allow the request through (fail open)
			return c.Next()
		}

		// Set rate limit headers
		c.Set("X-RateLimit-Limit", strconv.Itoa(cfg.Max))
		c.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		c.Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))

		if !allowed {
			return response.Fail(c, apperr.New("RATE_LIMIT_EXCEEDED", "Too many requests, please try again later", fiber.StatusTooManyRequests))
		}

		return c.Next()
	}
}

// defaultKeyFunc extracts the client IP as the rate limit key
func defaultKeyFunc(c *fiber.Ctx) string {
	// Use user ID if authenticated
	if userID, ok := c.Locals("user_id").(string); ok && userID != "" {
		return "user:" + userID
	}
	return "ip:" + c.IP()
}

// --- In-memory backend ---

type memoryBackend struct {
	mu      sync.Mutex
	windows map[string]*slidingWindow
}

type slidingWindow struct {
	timestamps []time.Time
	expiresAt  time.Time
}

func newMemoryBackend() *memoryBackend {
	mb := &memoryBackend{
		windows: make(map[string]*slidingWindow),
	}
	// Start periodic cleanup
	go mb.cleanup()
	return mb
}

func (mb *memoryBackend) Allow(_ context.Context, key string, maxReqs int, window time.Duration) (allowed bool, remaining int, resetAt time.Time, err error) {
	mb.mu.Lock()
	defer mb.mu.Unlock()

	now := time.Now()
	windowStart := now.Add(-window)
	resetAt = now.Add(window)

	sw, exists := mb.windows[key]
	if !exists {
		sw = &slidingWindow{}
		mb.windows[key] = sw
	}

	// Remove expired timestamps
	valid := sw.timestamps[:0]
	for _, ts := range sw.timestamps {
		if ts.After(windowStart) {
			valid = append(valid, ts)
		}
	}
	sw.timestamps = valid
	sw.expiresAt = resetAt

	count := len(sw.timestamps)
	if count >= maxReqs {
		return false, 0, resetAt, nil
	}

	// Add current request
	sw.timestamps = append(sw.timestamps, now)
	remaining = maxReqs - count - 1

	return true, remaining, resetAt, nil
}

func (mb *memoryBackend) cleanup() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		mb.mu.Lock()
		now := time.Now()
		for key, sw := range mb.windows {
			if now.After(sw.expiresAt) {
				delete(mb.windows, key)
			}
		}
		mb.mu.Unlock()
	}
}

// --- Redis backend ---

type redisBackend struct {
	cache port.Cache
}

func newRedisBackend(cache port.Cache) *redisBackend {
	return &redisBackend{cache: cache}
}

func (rb *redisBackend) Allow(ctx context.Context, key string, maxReqs int, window time.Duration) (allowed bool, remaining int, resetAt time.Time, err error) {
	now := time.Now()
	windowSec := int(window.Seconds())
	windowKey := fmt.Sprintf("ratelimit:%s:%d", key, now.Unix()/int64(windowSec))
	resetAt = time.Unix(((now.Unix()/int64(windowSec))+1)*int64(windowSec), 0)

	// Increment counter
	count, err := rb.cache.Increment(ctx, windowKey)
	if err != nil {
		return true, maxReqs, resetAt, err
	}

	// Set expiry on first request
	if count == 1 {
		_ = rb.cache.Expire(ctx, windowKey, window)
	}

	if int(count) > maxReqs {
		return false, 0, resetAt, nil
	}

	remaining = maxReqs - int(count)
	return true, remaining, resetAt, nil
}
