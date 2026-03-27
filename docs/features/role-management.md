# Role Management

## Overview

RBAC (Role-Based Access Control) management backed by Casbin. Provides endpoints to assign/revoke roles, manage per-role and direct user permissions, check permissions, and query the permission catalog. All endpoints require authentication and granular permission checks (`roles:read` or `roles:manage`).

## API Endpoints

| Method | Path | Auth | Permission | Description |
|--------|------|------|------------|-------------|
| GET | `/api/roles` | JWT | roles:read | List all predefined roles |
| GET | `/api/roles/permissions` | JWT | roles:read | List all permissions grouped by role (catalog) |
| POST | `/api/roles/assign` | JWT | roles:manage | Assign a role to a user |
| POST | `/api/roles/revoke` | JWT | roles:manage | Revoke a role from a user |
| GET | `/api/roles/:role/users` | JWT | roles:read | Get all users with a role |
| GET | `/api/roles/:role/permissions` | JWT | roles:read | Get permissions for a role |
| POST | `/api/roles/:role/permissions` | JWT | roles:manage | Add permission to a role |
| DELETE | `/api/roles/:role/permissions` | JWT | roles:manage | Remove permission from a role |
| GET | `/api/users/:id/roles` | JWT | roles:read | Get all roles for a user |
| GET | `/api/users/:id/permissions` | JWT | roles:read | Get all implicit permissions for a user |
| POST | `/api/users/:id/permissions` | JWT | roles:manage | Add direct permission to a user |
| DELETE | `/api/users/:id/permissions` | JWT | roles:manage | Remove direct permission from a user |
| GET | `/api/users/:id/permissions/check` | JWT | roles:read | Check if user has a specific permission |

## Predefined Roles

| Role | Description |
|------|-------------|
| `superadmin` | Full system access with all permissions |
| `admin` | Administrative access with most permissions |
| `editor` | Can create and edit content |
| `viewer` | Read-only access |

## Permission Model

Permissions follow an `object:action` pattern. For example:
- `users:read`, `users:create`, `users:update`, `users:delete`
- `sse:broadcast`, `sse:read`

Permissions can be assigned to roles (role-based) or directly to users (direct permissions). A user's effective permissions are the union of all permissions from their roles plus any direct permissions (implicit permissions).

## Request/Response Examples

### GET /api/roles

**Response (200):**
```json
{
  "success": true,
  "data": [
    { "name": "superadmin", "description": "Full system access with all permissions" },
    { "name": "admin", "description": "Administrative access with most permissions" },
    { "name": "editor", "description": "Can create and edit content" },
    { "name": "viewer", "description": "Read-only access" }
  ]
}
```

### POST /api/roles/assign

**Request:**
```json
{
  "user_id": "01912345-abcd-7def-8000-000000000001",
  "role": "editor"
}
```

**Response (200):**
```json
{
  "success": true,
  "message": "Role assigned successfully"
}
```

### POST /api/roles/revoke

**Request:**
```json
{
  "user_id": "01912345-abcd-7def-8000-000000000001",
  "role": "editor"
}
```

**Response (200):**
```json
{
  "success": true,
  "message": "Role revoked successfully"
}
```

### GET /api/roles/admin/users

**Response (200):**
```json
{
  "success": true,
  "data": {
    "role": "admin",
    "user_ids": [
      "01912345-abcd-7def-8000-000000000001",
      "01912345-abcd-7def-8000-000000000002"
    ]
  }
}
```

### GET /api/roles/admin/permissions

**Response (200):**
```json
{
  "success": true,
  "data": [
    { "object": "users", "action": "read" },
    { "object": "users", "action": "create" }
  ]
}
```

### POST /api/roles/editor/permissions

**Request:**
```json
{
  "object": "articles",
  "action": "write"
}
```

The `role` field in the body is overridden by the `:role` path parameter.

**Response (200):**
```json
{
  "success": true,
  "message": "Permission added successfully"
}
```

### GET /api/users/:id/permissions

**Response (200):**
```json
{
  "success": true,
  "data": {
    "user_id": "01912345-abcd-7def-8000-000000000001",
    "permissions": [
      { "object": "users", "action": "read" },
      { "object": "articles", "action": "write" }
    ]
  }
}
```

### GET /api/roles/permissions (Permission Catalog)

**Response (200):**
```json
{
  "success": true,
  "data": {
    "roles": [
      {
        "role": "superadmin",
        "permissions": [{ "object": "*", "action": "*" }]
      },
      {
        "role": "admin",
        "permissions": [
          { "object": "users", "action": "read" },
          { "object": "users", "action": "create" },
          { "object": "roles", "action": "read" },
          { "object": "roles", "action": "manage" }
        ]
      }
    ]
  }
}
```

### POST /api/users/:id/permissions (Direct Permission)

**Request:**
```json
{
  "object": "reports",
  "action": "export"
}
```

**Response (200):**
```json
{
  "success": true,
  "message": "Permission added successfully"
}
```

### GET /api/users/:id/permissions/check?object=users&action=read

**Response (200):**
```json
{
  "success": true,
  "data": {
    "user_id": "01912345-abcd-7def-8000-000000000001",
    "object": "users",
    "action": "read",
    "allowed": true
  }
}
```

## Configuration

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `authorization.enabled` | `AUTHORIZATION_ENABLED` | `false` | Enable Casbin authorization |

When disabled, a NoOp authorizer is used that permits all requests.

## Architecture

- `internal/module/role/` - Handler, usecase, DTO, domain
- `internal/adapter/casbin/` - Casbin adapter implementing `port.Authorizer`
- Casbin policies are stored in PostgreSQL via the Casbin adapter
- Granular `RequirePermission` middleware guards each route (`roles:read` for GET, `roles:manage` for mutations)

## Dependencies

| Port | Adapter | Purpose |
|------|---------|---------|
| `port.Authorizer` | Casbin / NoOp | All role and permission operations |
