# Server-Sent Events (SSE)

## Overview

Real-time event streaming via Server-Sent Events. Clients subscribe to an SSE stream and receive events pushed by the server. Supports topic-based routing so clients only receive events for topics they care about. An admin endpoint allows broadcasting events to all clients or to a specific topic.

## API Endpoints

| Method | Path | Auth | Permission | Description |
|--------|------|------|------------|-------------|
| GET | `/api/sse/subscribe` | JWT | (none) | Subscribe to SSE event stream |
| POST | `/api/sse/broadcast` | JWT | `sse:broadcast` | Broadcast an event |
| GET | `/api/sse/clients` | JWT | `sse:read` | Get connected client count |

## Request/Response Examples

### GET /api/sse/subscribe?topics=orders,notifications

**Query parameters:**

| Param | Type | Required | Description |
|-------|------|----------|-------------|
| `topics` | string | no | Comma-separated list of topics to subscribe to |

**Response:** `text/event-stream`

```
event: order.created
data: {"order_id": "123", "total": 99.99}

id: evt-456
event: notification
data: You have a new message

```

The SSE stream follows the standard format:
- `id:` - Optional event ID
- `event:` - Event type name
- `data:` - Event payload
- `retry:` - Optional reconnection time in milliseconds
- Each event is terminated by a blank line

### POST /api/sse/broadcast

**Request:**
```json
{
  "event": "system.announcement",
  "data": "Scheduled maintenance at 2am UTC",
  "topic": "notifications"
}
```

If `topic` is omitted, the event is broadcast to all connected clients.

**Response (200):**
```json
{
  "success": true,
  "message": "Event broadcast successfully"
}
```

### GET /api/sse/clients

**Response (200):**
```json
{
  "success": true,
  "data": {
    "count": 42
  }
}
```

## Configuration

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `sse.enabled` | `SSE_ENABLED` | `false` | Enable the SSE broker |

When disabled, a NoOp broker is used that silently discards all events.

## Architecture

- `internal/module/sse/` - HTTP handler and module routing
- `internal/adapter/sse/` - In-memory broker implementing `port.SSEBroker`
- The broker maintains a map of client subscriptions with buffered channels (buffer size: 100)
- On subscribe, the handler sets SSE headers (`Content-Type: text/event-stream`, `Cache-Control: no-cache`, `Connection: keep-alive`) and streams events via `SetBodyStreamWriter`
- On client disconnect, the client is unsubscribed from the broker
- `BroadcastToTopic` only delivers to clients subscribed to that specific topic

## Dependencies

| Port | Adapter | Purpose |
|------|---------|---------|
| `port.SSEBroker` | In-memory / NoOp | Event subscription and broadcasting |
| `port.Authorizer` | Casbin / NoOp | Permission checks on broadcast/clients endpoints |
