# Authentication

## Overview

JWT-based authentication with stateless access tokens and cached refresh tokens. Users authenticate with email/password and receive a token pair. Access tokens are short-lived JWTs; refresh tokens are opaque strings stored in Redis (or NoOp cache).

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/auth/login` | No | Authenticate and receive token pair |
| POST | `/api/auth/refresh` | No | Exchange refresh token for new token pair |
| POST | `/api/auth/logout` | No | Invalidate a refresh token |

## Request/Response Examples

### POST /api/auth/login

**Request:**
```json
{
  "email": "user@example.com",
  "password": "secret123"
}
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "dGhpcyBpcyBhIHJhbmRvbQ...",
    "expires_in": 900,
    "token_type": "Bearer"
  }
}
```

**Error (401):**
```json
{
  "success": false,
  "error": {
    "code": "UNAUTHORIZED",
    "message": "Invalid email or password"
  }
}
```

### POST /api/auth/refresh

**Request:**
```json
{
  "refresh_token": "dGhpcyBpcyBhIHJhbmRvbQ..."
}
```

**Response (200):**
```json
{
  "success": true,
  "data": {
    "access_token": "eyJhbGciOiJIUzI1NiIs...",
    "refresh_token": "bmV3IHJlZnJlc2ggdG9rZW4...",
    "expires_in": 900,
    "token_type": "Bearer"
  }
}
```

### POST /api/auth/logout

**Request:**
```json
{
  "refresh_token": "dGhpcyBpcyBhIHJhbmRvbQ..."
}
```

**Response (200):**
```json
{
  "success": true,
  "message": "Logged out successfully"
}
```

## Configuration

| Key | Env | Default | Description |
|-----|-----|---------|-------------|
| `jwt.secret` | `JWT_SECRET` | (none — startup fails) | HMAC-SHA256 signing secret. Must be ≥ 32 bytes and must not equal the committed placeholder `your-super-secret-key-change-in-production`. |
| `jwt.access_token_ttl` | `JWT_ACCESS_TOKEN_TTL` | (none) | Access token lifetime in minutes |
| `jwt.refresh_token_ttl` | `JWT_REFRESH_TOKEN_TTL` | (none) | Refresh token lifetime in minutes |

> **Operator note.** `app.New` calls `cfg.Validate()` before any adapter is constructed. If `JWT_SECRET` is unset, equals the committed placeholder, or is shorter than 32 bytes, the process refuses to start with a message naming the env var. This guard exists because the boilerplate previously shipped a forgeable token if the operator forgot the override.

## Architecture

### JWT Access Token Claims

```
sub:     user UUID
user_id: user UUID
email:   user email
name:    user display name
iat:     issued at
exp:     expiry (now + access_token_ttl)
nbf:     not before (now)
```

Signed with `HS256` using the configured secret.

### Refresh Token

- 32 random bytes, base64url-encoded
- Stored in cache with key `refresh:<token>` and value `<user_id>`
- TTL set to `refresh_token_ttl` duration
- On refresh, old token is deleted and a new one is issued (rotation)

### Data Flow

1. `handler.Login` -> validates body -> `usecase.Login`
2. Usecase looks up user by email via `user.Repository`
3. Verifies password with bcrypt
4. Generates JWT access token and random refresh token
5. Stores refresh token in `port.Cache`
6. Logs login event via `port.Auditor`

### Audit logging

Both successful and failed login attempts are recorded in `audit_logs`:

| Outcome | `action` | `resource_id` | `metadata.outcome` | `metadata.reason` |
|---------|----------|---------------|--------------------|---------------------|
| Success | `LOGIN` | authenticated user ID | `success` | — |
| Failed (bad password / unknown user) | `LOGIN` | attempted email | `failed` | `invalid_credentials` |
| Failed (inactive account) | `LOGIN` | attempted email | `failed` | `user_inactive` |
| Failed (other) | `LOGIN` | attempted email | `failed` | `unknown` |

Logging the attempted email on failure makes brute-force activity against a single email address detectable. The `reason` is sanitized to a fixed category — raw error strings are never echoed into the audit log.

## Dependencies

| Port | Adapter | Purpose |
|------|---------|---------|
| `port.Cache` | Redis / NoOp | Refresh token storage |
| `port.Auditor` | PostgreSQL / NoOp | Login/logout audit logging |
| `user.Repository` | PostgreSQL (SQLC) | User lookup by email/ID |
