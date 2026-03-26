# Rate Limiting

## Overview

Sliding window rate limiting middleware applied globally to all HTTP requests. Supports two backends: in-memory (default) and Redis (when Redis is enabled). Identifies clients by authenticated user ID or IP address. Fails open on backend errors to avoid blocking legitimate traffic.

## Configuration

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `rate_limit.enabled` | `RATE_LIMIT_ENABLED` | `false` | Enable rate limiting |
| `rate_limit.max` | `RATE_LIMIT_MAX` | `100` | Max requests per window |
| `rate_limit.window_sec` | `RATE_LIMIT_WINDOW_SEC` | `60` | Window duration in seconds |

The Redis backend is used automatically when `redis.enabled` is `true`; otherwise the in-memory backend is used.

## Response Headers

Every response includes rate limit headers:

| Header | Description |
|--------|-------------|
| `X-RateLimit-Limit` | Maximum requests allowed per window |
| `X-RateLimit-Remaining` | Requests remaining in current window |
| `X-RateLimit-Reset` | Unix timestamp when the window resets |

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
2. Client IP address (`ip:<ip>`) otherwise

A custom `KeyFunc` can be provided in `RateLimitConfig` to override this behavior.

## Backends

### In-Memory Backend

- Uses a sliding window algorithm with per-key timestamp arrays
- A background goroutine runs every minute to clean up expired windows
- Suitable for single-instance deployments

### Redis Backend

- Uses a fixed window counter per time bucket
- Key format: `ratelimit:<client_key>:<bucket>`
- Bucket calculated as `unix_timestamp / window_seconds`
- TTL set on first request in each bucket
- Suitable for multi-instance deployments with shared Redis

## Architecture

- `internal/platform/http/middleware/rate_limit.go` - Middleware, both backends
- Applied globally in `app.go` after the logger middleware and before route registration
- Uses `port.Cache` for the Redis backend (same cache adapter used elsewhere)
