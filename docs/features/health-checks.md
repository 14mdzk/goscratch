# Health Checks

## Overview

Three health check endpoints for different monitoring purposes: general health, readiness (can serve traffic), and liveness (process is alive). These endpoints are unauthenticated and intended for load balancers, orchestrators (Kubernetes), and monitoring systems.

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/api/health` | No | General health status |
| GET | `/api/health/ready` | No | Readiness probe (dependencies OK) |
| GET | `/api/health/live` | No | Liveness probe (process alive) |

## Request/Response Examples

### GET /api/health

**Response (200):**
```json
{
  "success": true,
  "data": {
    "status": "ok",
    "timestamp": "2025-01-15T10:30:00Z"
  }
}
```

### GET /api/health/ready

**Response (200):**
```json
{
  "success": true,
  "data": {
    "status": "ready",
    "timestamp": "2025-01-15T10:30:00Z",
    "checks": {
      "database": "ok",
      "cache": "ok"
    }
  }
}
```

### GET /api/health/live

**Response (200):**
```json
{
  "success": true,
  "data": {
    "status": "alive",
    "timestamp": "2025-01-15T10:30:00Z"
  }
}
```

## Endpoint Purposes

| Endpoint | Use Case | Kubernetes Probe |
|----------|----------|-----------------|
| `/health` | General monitoring, uptime checks | (none) |
| `/health/ready` | Can the app handle requests? Check after startup, during graceful shutdown | `readinessProbe` |
| `/health/live` | Is the process stuck or deadlocked? | `livenessProbe` |

## Configuration

No configuration. Health check endpoints are always registered.

## Architecture

- `internal/module/health/handler.go` - Three handler methods
- `internal/module/health/module.go` - Route registration
- The readiness check currently returns hardcoded `"ok"` for database and cache; it is designed to be extended with actual dependency health checks.

## Dependencies

None. Health checks are intentionally dependency-free to ensure they always respond.
