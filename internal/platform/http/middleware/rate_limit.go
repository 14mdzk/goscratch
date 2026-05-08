package middleware

import (
	"context"
	"io"
	"log/slog"
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
	Max        int                     // Max requests per window (default: 100)
	Window     time.Duration           // Time window (default: 1 minute)
	KeyFunc    func(*fiber.Ctx) string // Custom key extraction
	UseRedis   bool                    // Use Redis backend
	FailClosed bool                    // On backend error: reject (true) or allow (false)
}

// rateLimitBackend defines the interface for rate limit storage backends
type rateLimitBackend interface {
	// Allow checks if a request is allowed and returns the current count
	Allow(ctx context.Context, key string, max int, window time.Duration) (allowed bool, remaining int, resetAt time.Time, err error)
	// Close releases any resources held by the backend (e.g. stops the janitor goroutine).
	Close() error
}

// RateLimit returns a rate limiting middleware and a Closer that must be called
// on application shutdown to release backend resources (stop janitor goroutines).
func RateLimit(cfg RateLimitConfig, cache port.Cache) (fiber.Handler, io.Closer) {
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

	handler := func(c *fiber.Ctx) error {
		key := cfg.KeyFunc(c)

		allowed, remaining, resetAt, err := backend.Allow(c.UserContext(), key, cfg.Max, cfg.Window)
		if err != nil {
			slog.Error("rate limit backend error", "path", c.Path(), "key", key, "error", err)
			if cfg.FailClosed {
				// Reject rather than allow — critical for auth endpoints.
				return response.Fail(c, apperr.New("RATE_LIMIT_ERROR", "Service temporarily unavailable, please try again later", fiber.StatusServiceUnavailable))
			}
			// Non-auth paths: fail open (legacy behaviour preserved)
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

	return handler, backend
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
	mu        sync.Mutex
	windows   map[string]*slidingWindow
	stop      chan struct{}
	closeOnce sync.Once
}

type slidingWindow struct {
	timestamps []time.Time
	expiresAt  time.Time
}

func newMemoryBackend() *memoryBackend {
	mb := &memoryBackend{
		windows: make(map[string]*slidingWindow),
		stop:    make(chan struct{}),
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

	for {
		select {
		case <-ticker.C:
			mb.mu.Lock()
			now := time.Now()
			for key, sw := range mb.windows {
				if now.After(sw.expiresAt) {
					delete(mb.windows, key)
				}
			}
			mb.mu.Unlock()
		case <-mb.stop:
			return
		}
	}
}

// Close stops the janitor goroutine. It is safe to call multiple times.
func (mb *memoryBackend) Close() error {
	mb.closeOnce.Do(func() {
		close(mb.stop)
	})
	return nil
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

	ok, rem, _, redisErr := rb.cache.SlidingWindowAllow(ctx, key, maxReqs, window)
	if redisErr != nil {
		return true, maxReqs, now.Add(window), redisErr
	}

	// resetAt approximation: now + window (exact oldest-entry expiry is inside Redis)
	resetAt = now.Add(window)
	return ok, rem, resetAt, nil
}

// Close is a no-op for the Redis backend; the cache connection is owned and
// closed by the App.
func (rb *redisBackend) Close() error {
	return nil
}
