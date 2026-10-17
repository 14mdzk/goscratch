package cache

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/14mdzk/goscratch/internal/port"
)

// RedisCache implements port.Cache using Redis
type RedisCache struct {
	client *redis.Client
}

// NewRedisCache creates a new Redis cache instance
func NewRedisCache(addr, password string, db int) (*RedisCache, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &RedisCache{client: client}, nil
}

func (c *RedisCache) Get(ctx context.Context, key string) ([]byte, error) {
	val, err := c.client.Get(ctx, key).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, port.ErrCacheMiss
	}
	if err != nil {
		return nil, fmt.Errorf("redis get error: %w", err)
	}
	return val, nil
}

func (c *RedisCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if err := c.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis set error: %w", err)
	}
	return nil
}

func (c *RedisCache) Delete(ctx context.Context, key string) error {
	if err := c.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("redis delete error: %w", err)
	}
	return nil
}

// deleteByPrefixScript sweeps keys matching ARGV[1] in a single server-side
// execution: SCAN loops inside the script, each batch is DELeted, and the
// total deleted count is returned. Server-side execution is atomic — no other
// client interleaves — so a transport failure can no longer leave a partial
// revocation behind: the call either fully applied or fully failed, and the
// caller retries idempotently. Used by ChangePassword to revoke all active
// refresh tokens for a user without knowing each individual token hash.
const deleteByPrefixScript = `
local cursor = "0"
local deleted = 0
local pattern = ARGV[1]
repeat
  local res = redis.call("SCAN", cursor, "MATCH", pattern, "COUNT", 100)
  cursor = res[1]
  local keys = res[2]
  if #keys > 0 then
    deleted = deleted + redis.call("DEL", unpack(keys))
  end
until cursor == "0"
return deleted
`

// DeleteByPrefix removes all keys whose name starts with prefix using the
// atomic Lua sweep above.
func (c *RedisCache) DeleteByPrefix(ctx context.Context, prefix string) error {
	if err := c.client.Eval(ctx, deleteByPrefixScript, nil, prefix+"*").Err(); err != nil {
		return fmt.Errorf("redis delete-by-prefix error: %w", err)
	}
	return nil
}

func (c *RedisCache) Exists(ctx context.Context, key string) (bool, error) {
	n, err := c.client.Exists(ctx, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists error: %w", err)
	}
	return n > 0, nil
}

func (c *RedisCache) SetJSON(ctx context.Context, key string, value any, ttl time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}
	return c.Set(ctx, key, data, ttl)
}

func (c *RedisCache) GetJSON(ctx context.Context, key string, dest any) error {
	data, err := c.Get(ctx, key)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, dest); err != nil {
		return fmt.Errorf("failed to unmarshal json: %w", err)
	}
	return nil
}

func (c *RedisCache) Increment(ctx context.Context, key string) (int64, error) {
	val, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("redis incr error: %w", err)
	}
	return val, nil
}

func (c *RedisCache) Decrement(ctx context.Context, key string) (int64, error) {
	val, err := c.client.Decr(ctx, key).Result()
	if err != nil {
		return 0, fmt.Errorf("redis decr error: %w", err)
	}
	return val, nil
}

func (c *RedisCache) Expire(ctx context.Context, key string, ttl time.Duration) error {
	if err := c.client.Expire(ctx, key, ttl).Err(); err != nil {
		return fmt.Errorf("redis expire error: %w", err)
	}
	return nil
}

// slidingWindowScript is a Lua script that implements an atomic sliding-window
// rate limiter using a Redis sorted set.
//
// KEYS[1]  — the rate-limit key (e.g. "ratelimit:ip:1.2.3.4")
// ARGV[1]  — current time as Unix nanoseconds (string)
// ARGV[2]  — window size in nanoseconds (string)
// ARGV[3]  — max allowed requests in the window
//
// Returns an array: {allowed (0|1), remaining, retry_after_seconds}
var slidingWindowScript = redis.NewScript(`
local key      = KEYS[1]
local now      = tonumber(ARGV[1])
local window   = tonumber(ARGV[2])
local max      = tonumber(ARGV[3])
local cutoff   = now - window

redis.call('ZREMRANGEBYSCORE', key, '-inf', cutoff)
local count = redis.call('ZCARD', key)

if count >= max then
  -- Oldest entry tells us when a slot frees up.
  local oldest = redis.call('ZRANGE', key, 0, 0, 'WITHSCORES')
  local retry_ns = 0
  if oldest and #oldest >= 2 then
    retry_ns = tonumber(oldest[2]) + window - now
  end
  local retry_sec = math.ceil(retry_ns / 1e9)
  if retry_sec < 1 then retry_sec = 1 end
  return {0, 0, retry_sec}
end

redis.call('ZADD', key, now, now)
redis.call('PEXPIRE', key, math.ceil(window / 1e6))
local remaining = max - count - 1
return {1, remaining, 0}
`)

// SlidingWindowAllow implements port.Cache.SlidingWindowAllow using a Lua
// script so the ZREMRANGEBYSCORE → ZCARD → ZADD sequence is atomic.
func (c *RedisCache) SlidingWindowAllow(ctx context.Context, key string, maxReqs int, window time.Duration) (allowed bool, remaining, retryAfter int, err error) {
	nowNs := time.Now().UnixNano()
	windowNs := window.Nanoseconds()

	var res []int64
	res, err = slidingWindowScript.Run(ctx, c.client,
		[]string{key},
		nowNs, windowNs, maxReqs,
	).Int64Slice()
	if err != nil {
		err = fmt.Errorf("redis sliding window error: %w", err)
		return
	}
	if len(res) < 3 {
		err = fmt.Errorf("redis sliding window: unexpected result length %d", len(res))
		return
	}

	allowed = res[0] == 1
	remaining = int(res[1])
	retryAfter = int(res[2])
	return
}

func (c *RedisCache) Close() error {
	return c.client.Close()
}
