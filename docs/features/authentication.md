# Authentication

## Overview

JWT-based authentication with stateless access tokens and cached refresh tokens. Users authenticate with email/password and receive a token pair. Access tokens are short-lived JWTs; refresh tokens are opaque strings stored in Redis (or NoOp cache).

## API Endpoints

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/api/auth/login` | No | Authenticate and receive token pair |
| POST | `/api/auth/refresh` | No | Exchange refresh token for new token pair |
| POST | `/api/auth/logout` | **Yes** | Invalidate a refresh token (requires Bearer token) |

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

**Error (500) — cache unavailable:**

Login is **fail-closed** on the cache: if Redis is unavailable or not enabled, the server cannot issue a revocable refresh token and returns a 500 error. Operators must enable Redis (`redis.enabled=true`) for `/auth/login` to work.

### POST /api/auth/refresh

**Request:**
```json
{
  "refresh_token": "dGhpcyBpcyBhIHJhbmRvbQ..."
}
```

> Only the opaque `refresh_token` is required. The server resolves the user ID
> from its own lookup key — no `user_id` field is accepted or needed in the
> request body.

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

> **Auth required.** The `Authorization: Bearer <access_token>` header must be present.

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
| `jwt.issuer` | `JWT_ISSUER` | `goscratch` | Token issuer claim (`iss`). **Required — startup fails if empty.** |
| `jwt.audience` | `JWT_AUDIENCE` | `goscratch-api` | Token audience claim (`aud`). **Required — startup fails if empty.** |
| `jwt.access_token_ttl` | `JWT_ACCESS_TOKEN_TTL` | (none) | Access token lifetime in minutes |
| `jwt.refresh_token_ttl` | `JWT_REFRESH_TOKEN_TTL` | (none) | Refresh token lifetime in minutes |

> **Operator notes.**
>
> - `app.New` calls `cfg.Validate()` before any adapter is constructed. If `JWT_SECRET` is unset, equals the committed placeholder, or is shorter than 32 bytes, the process refuses to start. If `JWT_ISSUER` or `JWT_AUDIENCE` are empty, the process also refuses to start.
> - Login is **fail-closed**: if the Redis cache is unavailable (not enabled or connection failure), `/auth/login` returns a 500 error. Enable Redis for any environment that issues JWTs.
> - A `SECURITY WARNING` is logged at startup when Redis is disabled.

## Architecture

### JWT Access Token Claims

```
sub:     user UUID
iss:     jwt.issuer config value (required, enforced at parse time)
aud:     jwt.audience config value (required, enforced at parse time)
user_id: user UUID
email:   user email
name:    user display name
iat:     issued at
exp:     expiry (now + access_token_ttl)
nbf:     not before (now)
```

Signed with `HS256` using the configured secret.

Both `iss` and `aud` are **strictly validated** on every request: a token with an empty or mismatched issuer or audience is rejected with 401, even if the signature is valid.

### Refresh Token — Dual-Key Cache Design

Each issued refresh token is stored under **two independent keys** that share the same TTL (`refresh_token_ttl`):

| Key | Value | Purpose |
|-----|-------|---------|
| `refresh:tok:<sha256-hex(token)>` | `<userID>` | **Lookup key** — used by `Refresh` to translate an opaque token into a userID without any client-supplied hint. |
| `refresh:user:<userID>:<sha256-hex(token)>` | `1` | **Per-user index key** — used by `RevokeAllForUser` (called by `ChangePassword`) to iterate and delete every active session for a user via prefix scan. |

The hash is the full 64-character SHA-256 hex string for collision resistance. Storage cost is trivial.

**Why two keys?**

A single `refresh:user:<id>:<hash>` key (previous design) forced the client to supply `user_id` so the server could derive the key. Client-supplied `user_id` is bad API ergonomics: operators reading the API assumed it was part of the authentication credential rather than merely a key-derivation hint, and an erroneous `user_id` would silently return "invalid token" even for a valid token. The dual-key design removes `user_id` from the `/auth/refresh` request body entirely.

### Key lifecycle

**Login:**
1. Write lookup key, then per-user index key, both with the same TTL.
2. If either write fails, the partner key is deleted best-effort and the login is rejected (fail-closed).

**Refresh:**
1. Hash the supplied token, look up `refresh:tok:<hash>` → userID. Miss → 401.
2. Look up `refresh:user:<userID>:<hash>`. Miss → 401 (same message — no existence oracle). **Both keys must exist.**
3. No client-supplied `user_id` is used or accepted.
4. Delete both old keys (lookup + index).
5. Issue new token; write both new keys (fail-closed).

> **Why the index key is the revocation gate.** `RevokeAllForUser` (called by `ChangePassword`) deletes only the per-user index keys via prefix scan. The corresponding lookup keys remain in the cache until their TTL expires — they are orphaned. By requiring the index key at step 2, `Refresh` treats a password change as an immediate revocation even though the lookup key is still cached. An orphaned lookup key is harmless.

**Logout:**
1. Caller is authenticated (JWT required on `/auth/logout`).
2. Hash token, read lookup key → stored userID.
3. If lookup miss or stored userID ≠ callerID → return success silently (avoids token-existence oracle: an attacker with another user's token cannot confirm liveness by logging out with their own JWT).
4. Otherwise delete both keys.

**ChangePassword (`POST /api/users/me/password`):**
1. The auth module exposes a `Revoker` interface with `RevokeAllForUser(ctx, userID)`.
2. The user usecase calls `Revoker.RevokeAllForUser` after updating the password.
3. `RevokeAllForUser` uses `Cache.DeleteByPrefix("refresh:user:<id>:")` to delete all per-user index keys. The corresponding lookup keys (`refresh:tok:<hash>`) are left to expire naturally — they become orphans.
4. Orphaned lookup keys are **harmless**: `Refresh` requires BOTH the lookup key AND the per-user index key. Because the index key is gone, any refresh attempt with a pre-change token is rejected with 401 immediately, before the lookup-key TTL expires.
5. If the cache is unavailable, the error is propagated — the password is still updated but the caller learns revocation did not occur.

### Rate Limiting

`/auth/login` and `/auth/refresh` are protected by a per-IP tight rate limit (20 requests / 5 minutes) applied **before** the global rate limiter. The auth rate limiter is **fail-closed**: on Redis backend failure the request is rejected rather than allowed through.

### Logout

`/auth/logout` requires a valid JWT (`Authorization: Bearer <access_token>`). The caller ID is extracted from the JWT claims by the auth middleware and passed to the usecase. Unauthenticated callers receive 401.

### Data Flow

1. `handler.Login` -> validates body -> `usecase.Login`
2. Usecase looks up user by email via `user.Repository`
3. Verifies password with bcrypt
4. Generates JWT access token and random refresh token
5. Stores refresh token in `port.Cache` via dual-key write (fail-closed: returns error if cache unavailable)
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

### Password Change & Session Revocation

`POST /api/users/me/password` (ChangePassword) revokes all active refresh tokens for the user by calling the auth module's `Revoker.RevokeAllForUser`. If the cache is unavailable, the password is still updated but the error is propagated so the handler can inform the caller that session revocation did not occur.

## Dependencies

| Port | Adapter | Purpose |
|------|---------|---------|
| `port.Cache` | Redis (**required for login**) / NoOp (login disabled) | Refresh token storage and revocation |
| `port.Auditor` | PostgreSQL / NoOp | Login/logout audit logging |
| `user.Repository` | PostgreSQL (SQLC) | User lookup by email/ID |
