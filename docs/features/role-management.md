# Role Management

## Overview

RBAC (Role-Based Access Control) management backed by Casbin. Provides endpoints to assign/revoke roles, manage per-role permissions, and query user roles and permissions. All endpoints require authentication and admin or superadmin role.

## API Endpoints

| Method | Path | Auth | Role Required | Description |
|--------|------|------|---------------|-------------|
| GET | `/api/roles` | JWT | admin/superadmin | List all predefined roles |
| POST | `/api/roles/assign` | JWT | admin/superadmin | Assign a role to a user |
| POST | `/api/roles/revoke` | JWT | admin/superadmin | Revoke a role from a user |
| GET | `/api/roles/:role/users` | JWT | admin/superadmin | Get all users with a role |
| GET | `/api/roles/:role/permissions` | JWT | admin/superadmin | Get permissions for a role |
| POST | `/api/roles/:role/permissions` | JWT | admin/superadmin | Add permission to a role |
| DELETE | `/api/roles/:role/permissions` | JWT | admin/superadmin | Remove permission from a role |
| GET | `/api/users/:id/roles` | JWT | admin/superadmin | Get all roles for a user |
| GET | `/api/users/:id/permissions` | JWT | admin/superadmin | Get all implicit permissions for a user |

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

Permissions are assigned to roles, and roles are assigned to users. A user's effective permissions are the union of all permissions from all their roles (implicit permissions).

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

## Configuration

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `authorization.enabled` | `AUTHORIZATION_ENABLED` | `false` | Enable Casbin authorization |

When disabled, a NoOp authorizer is used that permits all requests.

## Architecture

- `internal/module/role/` - Handler, usecase, DTO, domain
- `internal/adapter/casbin/` - Casbin adapter implementing `port.Authorizer`
- Casbin policies are stored in PostgreSQL via the Casbin adapter
- The `RequireAnyRole` middleware guards all role management routes

## Dependencies

| Port | Adapter | Purpose |
|------|---------|---------|
| `port.Authorizer` | Casbin / NoOp | All role and permission operations |
