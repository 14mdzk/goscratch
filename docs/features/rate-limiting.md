# Rate Limiting

## Overview

Sliding window rate limiting middleware applied globally to all HTTP requests. Supports two backends: in-memory (default) and Redis (when Redis is enabled). Identifies clients by authenticated user ID or IP address. Fails open on backend errors to avoid blocking legitimate traffic (configurable per-use-site via `FailClosed`).

## Configuration

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `rate_limit.enabled` | `RATE_LIMIT_ENABLED` | `false` | Enable rate limiting |
| `rate_limit.max` | `RATE_LIMIT_MAX` | `100` | Max requests per window |
| `rate_limit.window_sec` | `RATE_LIMIT_WINDOW_SEC` | `60` | Window duration in seconds |

The Redis backend is used automatically when `redis.enabled` is `true`; otherwise the in-memory backend is used.

### Trusted-Proxy Configuration

The default key function uses `c.IP()` to get the client IP. When the server is behind a reverse proxy (nginx, Cloudflare, load balancer), `c.IP()` would return the proxy's IP instead of the real client IP unless trusted-proxy checking is configured.

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `server.trusted_proxies` | `SERVER_TRUSTED_PROXIES` | `[]` | CSV of trusted upstream CIDRs (e.g. `"10.0.0.1/32,10.0.0.2/32"`) |
| `server.proxy_header` | `SERVER_PROXY_HEADER` | `""` | Header to read real client IP from when request comes from a trusted proxy (default when `trusted_proxies` is set: `X-Forwarded-For`) |

**Security model:** `c.IP()` only reads the proxy header when the socket remote address is in `trusted_proxies`. If the CIDR is not matched, `c.IP()` returns the socket address (spoofing-safe). If `proxy_header` is set without `trusted_proxies`, a warning is logged at boot and the header is ignored.

**Operator note:** If you run behind a proxy and set `SERVER_TRUSTED_PROXIES` to `0.0.0.0/0` (all IPs), any client can spoof their IP via the proxy header and bypass per-IP rate limits. Always restrict the CIDR to the actual upstream proxy addresses.

## Response Headers

Every response includes rate limit headers:

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Maximum requests allowed per window |
| `X-RateLimit-Remaining` | Requests remaining in current window |
| `X-RateLimit-Reset` | Unix timestamp when the current window resets (approximate) |

## 429 Response

When the limit is exceeded:

**Response (429):**
```json
{
  "success": false,
  "error": {
    "code": "RATE_LIMIT_EXCEEDED",
    "message": "Too many requests, please try again later"
  }
}
```

## Client Identification

The default key function identifies clients by:
1. Authenticated user ID (`user:<user_id>`) if the request has a valid JWT
2. Client IP address (`ip:<ip>`) otherwise — subject to trusted-proxy config above

A custom `KeyFunc` can be provided in `RateLimitConfig` to override this behavior.

## Backends

### In-Memory Backend

- Uses a sliding window algorithm with per-key timestamp arrays
- A background goroutine runs every minute to clean up expired windows
- The goroutine is stopped on application shutdown via a stop channel (`Close()`)
- Suitable for single-instance deployments

### Redis Backend

- Uses a Lua script for an atomic sliding window algorithm (ZSET of nanosecond timestamps)
- Operations: `ZREMRANGEBYSCORE` (drop old entries), `ZCARD` (count), `ZADD` (record new entry) — all in one round trip
- Key TTL set via `PEXPIRE` after each write
- Suitable for multi-instance deployments with shared Redis

**Algorithm change note:** Prior to this version the Redis backend used a fixed-window counter (`INCR` + `EXPIRE`). The new sliding window prevents the 2× burst that was possible at window boundaries. Existing fixed-window counter keys in Redis are orphaned; they will TTL out naturally. Rate-limit decisions during the rollout window may differ slightly from the old behavior near window boundaries.

## Shutdown / Lifecycle

`RateLimit(cfg, cache)` returns `(fiber.Handler, io.Closer)`. The `io.Closer` must be called on shutdown to stop the in-memory janitor goroutine. `app.Shutdown` calls it automatically. Callers that instantiate a `RateLimit` outside `app.go` (e.g. module-level per-route limiters with `UseRedis: true`) may discard the closer — the Redis backend's `Close()` is a no-op.

## Architecture

- `internal/platform/http/middleware/rate_limit.go` — Middleware, both backends, `Close()` on memory backend
- `internal/adapter/cache/redis.go` — `SlidingWindowAllow` Lua implementation
- `internal/platform/config/config.go` — `ServerConfig.TrustedProxies`, `ServerConfig.ProxyHeader`, extended env-override for `[]string`
- `internal/platform/http/server.go` — Fiber `EnableTrustedProxyCheck` / `TrustedProxies` / `ProxyHeader` wiring
- Applied globally in `app.go` after the logger middleware and before route registration
