# Quick Start Guide

Get Goscratch running locally in 5 minutes.

## Prerequisites

- Go 1.25+
- Docker and Docker Compose
- make

## Steps

### 1. Clone the repository

```bash
git clone https://github.com/14mdzk/goscratch.git
cd goscratch
```

### 2. Install development tools

```bash
make install-tools
```

This installs: air (hot-reload), golangci-lint, sqlc, and golang-migrate.

### 3. Start PostgreSQL

```bash
make docker-up
```

Waits for PostgreSQL to be ready via `pg_isready`.

### 4. Run database migrations

```bash
make migrate-up
```

Creates the `users`, `audit_logs`, and `casbin_rules` tables.

### 5. Seed test data

```bash
make seed
```

Creates three test users with roles:

| Email | Password | Role |
|-------|----------|------|
| `superadmin@example.com` | `password123` | superadmin |
| `admin@example.com` | `password123` | admin |
| `user@example.com` | `password123` | viewer |

Also seeds 17 default permissions for RBAC.

### 6. Start the API server

```bash
make dev
```

The server starts at http://localhost:3000 with hot-reload enabled.

### 7. Verify it works

```bash
curl http://localhost:3000/health
```

Expected response:
```json
{"status":"ok"}
```

### 8. Browse the API docs

Open http://localhost:3000/docs in your browser. This loads the Scalar API reference with all 34 endpoints documented.

### 9. Try logging in

```bash
curl -s -X POST http://localhost:3000/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"password123"}' | jq .
```

Copy the `access_token` from the response.

### 10. Make an authenticated request

```bash
curl -s http://localhost:3000/users/me \
  -H "Authorization: Bearer <your-access-token>" | jq .
```

## Optional: Enable additional services

To start Redis and RabbitMQ alongside PostgreSQL:

```bash
make docker-full
```

Then set the feature flags in `.env`:

```bash
cp .env.example .env
# Edit .env:
# REDIS_ENABLED=true
# RABBITMQ_ENABLED=true
```

Restart the server to pick up the changes.

## Optional: Start the background worker

In a separate terminal:

```bash
make dev-worker
```

Requires RabbitMQ to be enabled and running.

## Secure-defaults checklist

Required for any non-local environment (staging, production) and for anyone upgrading from v1.0 → v1.1. The application **fails to boot** if items 1–4 are not satisfied. The remaining items prevent silent security regressions.

| # | Setting | Required value | Why |
|---|---------|---------------|-----|
| 1 | `JWT_SECRET` | Non-empty, **not** equal to the placeholder `your-super-secret-key-change-in-production`, and **≥ 32 bytes**. Generate with `openssl rand -base64 48`. | `app.New` hard-fails at startup otherwise. The committed placeholder is detected by exact match. |
| 2 | `JWT_ISSUER` and `JWT_AUDIENCE` | Both non-empty. The defaults in `config/config.default.json` are non-empty; do not override them with empty strings. | `Config.Validate` rejects empty values. Tokens are unconditionally validated against `iss` and `aud`. |
| 3 | `DB_SSL_MODE` | `require` (default) for production. Local dev with the bundled compose stack: set `DB_SSL_MODE=disable` explicitly. | Default flipped from `disable` → `require` in v1.1. |
| 4 | `REDIS_ENABLED` | `true` for any environment that issues real refresh tokens. | Auth (`/auth/login`, `/auth/refresh`) is **fail-closed** on the cache: with Redis disabled or unreachable, login returns 500. A `SECURITY WARNING` is logged at boot when Redis is disabled. |
| 5 | `SERVER_TRUSTED_PROXIES` | CSV of CIDRs for any reverse proxy in front of the API (Nginx, ALB, Cloudflare). Leave empty if the API is exposed directly. | Without this set, Fiber will not honour `X-Forwarded-For`, so per-IP rate limits and audit IPs reflect the proxy address, not the real client. With this set incorrectly, a hostile client can spoof their IP via the header. |
| 6 | `SERVER_PROXY_HEADER` | `X-Forwarded-For` (default when `SERVER_TRUSTED_PROXIES` is set). Override only if the proxy uses a different header. | Header is only honoured when the connecting peer is a trusted proxy. |
| 7 | `OBSERVABILITY_METRICS_PORT` | Any free port. The `/metrics` endpoint binds to `127.0.0.1:<port>` on a separate listener — **not** the public Fiber listener. | Operators must scrape from inside the host (Prometheus on the same machine, sidecar, or SSH tunnel). Exposing `/metrics` publicly is a known reconnaissance vector. |
| 8 | Refresh tokens | Existing v1.0 refresh tokens are invalidated by the v1.1 dual-key design. All users must re-login after the upgrade. | Communicate this to clients before deploying. |

After applying changes, restart the API and confirm:

```bash
curl -s http://localhost:3000/health
# {"status":"ok"}
```

Then verify Prometheus is reachable on the **internal** port only:

```bash
curl -s http://127.0.0.1:9090/metrics | head -1
# Should succeed locally; should fail from outside the host.
```

Full release notes for v1.1.0 are in [`CHANGELOG.md`](../CHANGELOG.md). Audit trail: [`docs/audit/`](audit/).

## Next steps

- Read the feature specs in `docs/features/`
- Review architecture decisions in `docs/adr/`
- Add your own modules following `internal/module/user/` as reference
- Run `make test` to verify all tests pass
