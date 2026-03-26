# Background Jobs

## Overview

Background job processing via RabbitMQ. An HTTP API allows admins to dispatch jobs, and a separate worker process consumes and executes them. Jobs support configurable retry with exponential backoff.

## API Endpoints

| Method | Path | Auth | Role Required | Description |
|--------|------|------|---------------|-------------|
| POST | `/api/jobs/dispatch` | JWT | admin | Dispatch a new background job |
| GET | `/api/jobs/types` | JWT | admin | List available job types |

## Request/Response Examples

### POST /api/jobs/dispatch

**Request:**
```json
{
  "type": "email.send",
  "payload": {
    "to": "user@example.com",
    "subject": "Welcome",
    "body": "Hello!"
  },
  "max_retry": 5
}
```

`max_retry` is optional (default: 3).

**Response (201):**
```json
{
  "success": true,
  "data": {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "type": "email.send",
    "status": "queued",
    "created_at": "2025-01-15T10:30:00Z"
  }
}
```

### GET /api/jobs/types

**Response (200):**
```json
{
  "success": true,
  "data": {
    "types": [
      { "type": "email.send", "description": "Send an email to a recipient" },
      { "type": "audit.cleanup", "description": "Clean up old audit log entries" },
      { "type": "notification.send", "description": "Send a notification to a user" }
    ]
  }
}
```

## Available Job Types

| Type | Description |
|------|-------------|
| `email.send` | Send an email to a recipient |
| `audit.cleanup` | Clean up old audit log entries |
| `notification.send` | Send a notification to a user |

## Worker Processing

The worker runs as a separate process (or goroutine) that:

1. Declares the queue (durable) on startup
2. Spawns `concurrency` consumer goroutines
3. Each goroutine calls `queue.Consume` with a callback
4. On message receipt, decodes the `Job` JSON, finds the registered handler, and executes it
5. Each job handler gets a 5-minute context timeout

### Retry Logic

- On failure, if `attempts < max_retry`, the job is re-published to the queue after a delay
- Delay uses exponential backoff: `attempts^2` seconds (1s, 4s, 9s, ...)
- Malformed messages and unhandled job types are acknowledged without retry

### Job Struct

```json
{
  "id": "uuid",
  "type": "email.send",
  "payload": { ... },
  "attempts": 0,
  "max_retry": 3,
  "created_at": "2025-01-15T10:30:00Z"
}
```

## Configuration

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `worker.enabled` | `WORKER_ENABLED` | `false` | Enable the worker process |
| `worker.concurrency` | `WORKER_CONCURRENCY` | `1` | Number of consumer goroutines |
| `worker.queue_name` | `WORKER_QUEUE_NAME` | `jobs` | RabbitMQ queue name |
| `worker.exchange` | `WORKER_EXCHANGE` | `""` | RabbitMQ exchange name |
| `rabbitmq.enabled` | `RABBITMQ_ENABLED` | `false` | Enable RabbitMQ connection |
| `rabbitmq.url` | `RABBITMQ_URL` | (none) | RabbitMQ connection URL |

## Architecture

- `internal/module/job/` - HTTP handler and usecase for dispatching
- `internal/worker/` - Worker, Publisher, Job, and JobHandler interface
- `internal/worker/handlers/` - Concrete job handler implementations
- The API uses `worker.Publisher` to publish jobs
- The worker uses `worker.Worker` to consume and dispatch to registered `JobHandler` implementations

## Dependencies

| Port | Adapter | Purpose |
|------|---------|---------|
| `port.Queue` | RabbitMQ / NoOp | Job publishing and consuming |
| `port.Authorizer` | Casbin / NoOp | Admin role check on API routes |
