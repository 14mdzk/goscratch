# User Management

## Overview

Full CRUD operations for users with cursor-based pagination, soft deletion, activation/deactivation, and password management. All management endpoints require authentication and authorization; self-service endpoints (get profile, change password) require only authentication.

## API Endpoints

| Method | Path | Auth | Permission | Description |
|--------|------|------|------------|-------------|
| GET | `/api/users/me` | JWT | (none) | Get current user profile |
| POST | `/api/users/me/password` | JWT | (none) | Change own password |
| GET | `/api/users` | JWT | `users:read` | List users (paginated) |
| GET | `/api/users/:id` | JWT | `users:read` | Get user by ID |
| POST | `/api/users` | JWT | `users:create` | Create a new user |
| PUT | `/api/users/:id` | JWT | `users:update` | Update a user |
| DELETE | `/api/users/:id` | JWT | `users:delete` | Soft-delete a user |
| POST | `/api/users/:id/activate` | JWT | `users:update` | Activate a user |
| POST | `/api/users/:id/deactivate` | JWT | `users:update` | Deactivate a user |

## Request/Response Examples

### GET /api/users/me

**Response (200):**
```json
{
  "success": true,
  "data": {
    "id": "01912345-abcd-7def-8000-000000000001",
    "email": "user@example.com",
    "name": "Jane Doe",
    "is_active": true,
    "created_at": "2025-01-15T10:30:00Z",
    "updated_at": "2025-01-15T10:30:00Z"
  }
}
```

### POST /api/users

**Request:**
```json
{
  "email": "newuser@example.com",
  "password": "securepass8",
  "name": "John Smith"
}
```

Validation: `email` required + valid email, `password` required + min 8 chars, `name` required + 2-100 chars.

**Response (201):**
```json
{
  "success": true,
  "data": {
    "id": "01912345-abcd-7def-8000-000000000002",
    "email": "newuser@example.com",
    "name": "John Smith",
    "is_active": true,
    "created_at": "2025-01-15T11:00:00Z",
    "updated_at": "2025-01-15T11:00:00Z"
  }
}
```

### PUT /api/users/:id

**Request:**
```json
{
  "name": "Jane Updated",
  "email": "newemail@example.com"
}
```

Both fields are optional. Validation: `name` 2-100 chars, `email` valid email.

**Response (200):**
```json
{
  "success": true,
  "data": {
    "id": "01912345-abcd-7def-8000-000000000001",
    "email": "newemail@example.com",
    "name": "Jane Updated",
    "is_active": true,
    "created_at": "2025-01-15T10:30:00Z",
    "updated_at": "2025-01-16T09:00:00Z"
  }
}
```

### GET /api/users?limit=10&search=jane

**Query parameters:**

| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `cursor` | string | (none) | Base64-encoded cursor from previous page |
| `limit` | int | 20 | Items per page (1-100) |
| `search` | string | (none) | Search by name or email (partial match) |
| `email` | string | (none) | Exact email match |
| `is_active` | bool | (none) | Filter by active status |

**Response (200):**
```json
{
  "success": true,
  "data": [
    {
      "id": "01912345-abcd-7def-8000-000000000001",
      "email": "jane@example.com",
      "name": "Jane Doe",
      "is_active": true,
      "created_at": "2025-01-15T10:30:00Z",
      "updated_at": "2025-01-15T10:30:00Z"
    }
  ],
  "pagination": {
    "next_cursor": "eyJsYXN0X2lkIjoiMDE5MT...",
    "prev_cursor": "eyJsYXN0X2lkIjoiMDE5MT...",
    "has_more": true,
    "has_prev": false
  }
}
```

### POST /api/users/me/password

**Request:**
```json
{
  "current_password": "oldpassword",
  "new_password": "newpassword8"
}
```

**Response (200):**
```json
{
  "success": true,
  "message": "Password changed successfully"
}
```

### POST /api/users/:id/activate

**Response (200):**
```json
{
  "success": true,
  "message": "User activated successfully"
}
```

### DELETE /api/users/:id

**Response:** `204 No Content`

## Configuration

No module-specific configuration. Uses the JWT secret from the auth config for route protection.

## Architecture

### Cursor Pagination

Cursors are base64-encoded JSON containing `last_id` and `direction` (`next`/`prev`). The system uses bidirectional cursor pagination: it fetches `limit + 1` rows to determine whether more pages exist. When navigating backward, the extra item is trimmed from the beginning; when forward, from the end.

### Packages

- `internal/module/user/handler` - HTTP handlers
- `internal/module/user/usecase` - Business logic, audit logging
- `internal/module/user/repository` - PostgreSQL via SQLC
- `internal/module/user/dto` - Request/response DTOs
- `internal/module/user/domain` - User entity, filter, constants

## Dependencies

| Port | Adapter | Purpose |
|------|---------|---------|
| `port.Auditor` | PostgreSQL / NoOp | CRUD audit logging |
| `port.Authorizer` | Casbin / NoOp | Permission checks on routes |
