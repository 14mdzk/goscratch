# Notifications

## Overview

A pluggable notification system that delivers messages to users via configurable backends. The default backend is a webhook sender that POSTs JSON payloads to an operator-specified URL. A NoOp sender logs notifications without delivering them, making the system zero-dependency in development.

Notifications are dispatched via the background job system (`notification.send` job type), so they never block the API response.

## API Endpoints

Notifications are dispatched through the existing job API:

| Method | Path | Auth | Role Required | Description |
|--------|------|------|---------------|-------------|
| POST | `/api/jobs/dispatch` | JWT | admin | Dispatch a `notification.send` job |

## Request/Response Examples

### POST /api/jobs/dispatch (notification)

**Request:**
```json
{
  "type": "notification.send",
  "payload": {
    "user_id": "usr_abc123",
    "title": "New message received",
    "body": "You have a new message from Alice",
    "channel": "in_app"
  },
  "max_retry": 3
}
```

`channel` is optional — the adapter may use it to route to different delivery targets.

**Response (201):**
```json
{
  "success": true,
  "data": {
    "id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "type": "notification.send",
    "status": "queued",
    "created_at": "2025-01-15T10:30:00Z"
  }
}
```

## Configuration

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `notification.enabled` | `NOTIFICATION_ENABLED` | `false` | Enable the notification sender |
| `notification.webhook_url` | `NOTIFICATION_WEBHOOK_URL` | `""` | Webhook URL for the webhook backend |

## Notification Backends

| Backend | Adapter | When to use |
|---------|---------|-------------|
| NoOp | `notification.NoOpSender` | Development — logs without delivering |
| Webhook | `notification.WebhookSender` | Production — POSTs JSON to a configurable URL |

### Webhook payload format

The webhook sender POSTs the following JSON:

```json
{
  "user_id": "usr_abc123",
  "title": "New message received",
  "body": "You have a new message from Alice",
  "channel": "in_app",
  "sent_at": "2025-01-15T10:30:00Z"
}
```

Operators wire this to Slack webhooks, Discord webhooks, a custom notification service, or any HTTP endpoint that accepts JSON.

## Architecture

```
POST /api/jobs/dispatch
  → job.UseCase.Dispatch("notification.send", payload)
    → worker.Publisher.PublishWithRetry(...)
      → RabbitMQ (or in-process queue, future)

Worker:
  → worker.Worker.Consume("jobs")
    → NotificationHandler.Handle(job)
      → port.NotificationSender.Send(msg)
        → WebhookSender.Send(msg)   [production]
        → NoOpSender.Send(msg)      [development]
```

- `internal/port/notification.go` — `NotificationSender` interface + `NotificationMessage` type
- `internal/adapter/notification/noop.go` — NoOp implementation (dev default)
- `internal/adapter/notification/webhook.go` — Webhook implementation (production)
- `internal/worker/handlers/notification_handler.go` — Background job handler
- `internal/module/job/usecase/` — `notification.send` is a valid job type

## Dependencies

| Port | Adapter | Purpose |
|------|---------|---------|
| `port.NotificationSender` | Webhook / NoOp | Deliver notification to user |
| `port.Queue` | RabbitMQ / NoOp | Job publishing (via job module) |
| `port.Authorizer` | Casbin / NoOp | Admin role check on dispatch API |

## Wire-up

In `app.go`: if `notification.enabled=true`, creates `WebhookSender`; otherwise `NoOpSender`.
In `cmd/worker/main.go`: registers `NotificationHandler` with the worker so `notification.send` jobs are consumed.
If the notification sender is a NoOp, registered jobs are still consumed and logged — no dispatch is silently dropped.
