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

## Next steps

- Read the feature specs in `docs/features/`
- Review architecture decisions in `docs/adr/`
- Add your own modules following `internal/module/user/` as reference
- Run `make test` to verify all tests pass
