# Observability

## Overview

Built-in observability with three pillars: Prometheus metrics, OpenTelemetry distributed tracing, and structured logging with trace correlation. All three are optional and can be enabled independently.

## Prometheus Metrics

### Endpoint

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/metrics` | No | Prometheus metrics scrape endpoint |

Enabled when `observability.metrics.enabled` is `true`.

### Collected Metrics

**HTTP Metrics:**

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `http_requests_total` | Counter | method, path, status | Total HTTP requests |
| `http_request_duration_seconds` | Histogram | method, path, status | Request latency |
| `http_request_size_bytes` | Histogram | method, path | Request body size |
| `http_response_size_bytes` | Histogram | method, path | Response body size |
| `http_active_connections` | Gauge | (none) | Current active connections |

**Database Metrics:**

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `db_queries_total` | Counter | operation, table | Total DB queries |
| `db_query_duration_seconds` | Histogram | operation, table | Query latency |

**Cache Metrics:**

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `cache_hits_total` | Counter | cache | Cache hit count |
| `cache_misses_total` | Counter | cache | Cache miss count |

**Business Metrics:**

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `users_registered_total` | Counter | (none) | Total user registrations |
| `login_attempts_total` | Counter | status | Login attempts (success/failed) |

## OpenTelemetry Tracing

Distributed tracing via OTLP HTTP exporter. Each incoming HTTP request gets a span with method, route, URL, status, and user agent attributes.

### Features

- Automatic span creation for every HTTP request
- `X-Trace-ID` response header for debugging
- Context propagation (W3C TraceContext + Baggage)
- Helper functions: `WrapDBOperation`, `WrapCacheOperation` for downstream tracing
- `WithContext` logger method attaches `trace_id` and `span_id` to log entries

### Trace Attributes

Each HTTP span includes:
- `http.method`, `http.route`, `http.url`, `http.status_code`
- `http.user_agent`, `net.host.name`

DB operation spans include:
- `db.operation`, `db.table`, `db.system` ("postgresql")

Cache operation spans include:
- `cache.operation`, `cache.key`, `cache.system` ("redis")

## Structured Logging

Uses Go's `log/slog` with JSON output. The observability logger adds trace correlation (trace_id, span_id) to every log entry when tracing is active.

The primary application logger (`pkg/logger`) provides structured logging used throughout the codebase. The observability logger extends this with OpenTelemetry integration.

## Configuration

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `observability.metrics.enabled` | `METRICS_ENABLED` | `false` | Enable Prometheus metrics |
| `observability.metrics.port` | `METRICS_PORT` | (none) | Metrics port (currently served on main port) |
| `observability.tracing.enabled` | `TRACING_ENABLED` | `false` | Enable OpenTelemetry tracing |
| `observability.tracing.endpoint` | `TRACING_ENDPOINT` | (none) | OTLP HTTP endpoint (e.g., `localhost:4318`) |

## Architecture

- `internal/platform/observability/metrics.go` - Prometheus metrics and middleware
- `internal/platform/observability/tracer.go` - OpenTelemetry init and tracing middleware
- `internal/platform/observability/logger.go` - Trace-correlated structured logger
- `pkg/logger/` - Primary application logger
- Metrics and tracing middlewares are applied in `app.go` before route registration
