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
| `jwt.secret` | `JWT_SECRET` | (none) | HMAC-SHA256 signing secret |
| `jwt.access_token_ttl` | `JWT_ACCESS_TOKEN_TTL` | (none) | Access token lifetime in minutes |
| `jwt.refresh_token_ttl` | `JWT_REFRESH_TOKEN_TTL` | (none) | Refresh token lifetime in minutes |

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

## Dependencies

| Port | Adapter | Purpose |
|------|---------|---------|
| `port.Cache` | Redis / NoOp | Refresh token storage |
| `port.Auditor` | PostgreSQL / NoOp | Login/logout audit logging |
| `user.Repository` | PostgreSQL (SQLC) | User lookup by email/ID |
