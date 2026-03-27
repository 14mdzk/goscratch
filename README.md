# Goscratch

Production-ready Go backend starterkit with clean architecture, modular design, and built-in observability.

[![Go Version](https://img.shields.io/github/go-mod/go-version/14mdzk/goscratch)](https://github.com/14mdzk/goscratch)
[![CI](https://github.com/14mdzk/goscratch/actions/workflows/ci.yml/badge.svg)](https://github.com/14mdzk/goscratch/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## What Is Goscratch?

Goscratch is a **starterkit you clone and own** -- not a framework or package you import. It provides production-grade infrastructure (authentication, authorization, background jobs, file storage, observability) so you skip weeks of boilerplate and jump straight into building business logic.

Every external dependency (Redis, RabbitMQ, S3) has a NoOp fallback. Start with just PostgreSQL, enable services as you need them.

## Quick Start

Get a running API in 5 minutes:

```bash
git clone https://github.com/14mdzk/goscratch.git
cd goscratch
make install-tools
make docker-up
make migrate-up
make seed
make dev
```

Then visit:
- Health check: http://localhost:3000/health
- API docs: http://localhost:3000/docs

Test credentials (from seeded data):

| Email | Password | Role |
|-------|----------|------|
| `superadmin@example.com` | `password123` | Super Admin |
| `admin@example.com` | `password123` | Admin |
| `user@example.com` | `password123` | Viewer |

## Features

| Feature | Description |
|---------|-------------|
| **Authentication** | JWT access + refresh tokens, bcrypt hashing, login/logout/refresh |
| **User Management** | CRUD, activate/deactivate, password change, cursor-based pagination |
| **Role & Permission** | Casbin v3 RBAC, database-backed policies, 9 management endpoints |
| **File Storage** | Upload/download via S3 or local filesystem, path traversal protection |
| **Background Jobs** | RabbitMQ with exponential backoff retry, dispatch + list APIs |
| **Server-Sent Events** | Real-time push, subscribe/broadcast/client-count endpoints |
| **Email** | SMTP adapter + NoOp fallback for development |
| **Rate Limiting** | Sliding window (in-memory or Redis), X-RateLimit headers, 429 responses |
| **API Documentation** | OpenAPI 3.0 spec, Scalar interactive explorer at `/docs` |
| **Observability** | Prometheus metrics, OpenTelemetry tracing, structured JSON logging |
| **Security** | Security headers, config-driven CORS, JWT iss/aud claims, XSS sanitization |
| **Health Checks** | Database, cache, queue, storage readiness at `/health` |

## Architecture

Goscratch follows **hexagonal (clean) architecture**:

```
HTTP Handler -> UseCase (business logic) -> Port (interface) -> Adapter (implementation)
```

Key principles:
- **No magic** -- manual DI, explicit dependencies wired in `internal/platform/app/app.go`
- **Port-driven** -- swap implementations without touching business logic
- **NoOp fallbacks** -- every adapter has a no-op variant, start minimal
- **Decorator Pattern** -- audit logging is decoupled from usecases via decorators
- **TX-aware repositories** -- transactions propagate through context automatically

Design decisions documented in `docs/adr/`.

## Project Structure

```
.
├── cmd/
│   ├── api/                     # API server entry point
│   └── worker/                  # Background job worker entry point
├── config/
│   └── config.default.json      # Default configuration (JSON layer)
├── internal/
│   ├── adapter/                 # External service implementations
│   │   ├── audit/               #   PostgreSQL audit logging
│   │   ├── cache/               #   Redis + NoOp
│   │   ├── casbin/              #   RBAC authorization
│   │   ├── email/               #   SMTP + NoOp
│   │   ├── queue/               #   RabbitMQ + NoOp
│   │   ├── sse/                 #   In-memory SSE broker
│   │   └── storage/             #   S3 + local filesystem
│   ├── module/                  # Feature modules
│   │   ├── auth/                #   Authentication
│   │   ├── docs/                #   OpenAPI / Scalar endpoint
│   │   ├── health/              #   Health checks
│   │   ├── job/                 #   Job dispatch API
│   │   ├── role/                #   Role & permission management
│   │   ├── sse/                 #   SSE HTTP endpoints
│   │   ├── storage/             #   File upload/download API
│   │   └── user/                #   User CRUD & profile
│   ├── platform/                # Framework integrations
│   │   ├── app/                 #   DI container, module wiring
│   │   ├── config/              #   3-layer config loader
│   │   ├── database/            #   PostgreSQL pool, transactor
│   │   ├── http/                #   Fiber server, middleware
│   │   ├── sanitizer/           #   XSS input sanitization
│   │   ├── testutil/            #   Integration test helpers
│   │   └── validator/           #   Request validation
│   ├── port/                    # Interfaces (cache, queue, storage, audit, etc.)
│   ├── shared/                  # Shared domain types (pagination)
│   └── worker/                  # Background job processing
├── pkg/                         # Reusable packages
│   ├── apperr/                  #   Structured application errors
│   ├── logger/                  #   Structured logging
│   ├── pgutil/                  #   PostgreSQL utilities
│   ├── response/                #   HTTP response helpers
│   └── types/                   #   Optional types (Opt, NOpt)
├── migrations/                  # SQL migration files
├── scripts/seed/                # Database seeding
├── deploy/                      # Production deployment configs
│   ├── docker/                  #   Docker Compose (production)
│   ├── systemd/                 #   Systemd unit files
│   └── nginx/                   #   Reverse proxy config
├── docs/                        # Documentation
│   ├── features/                #   Feature specifications
│   ├── adr/                     #   Architecture decision records
│   └── openapi.yaml             #   OpenAPI 3.0 spec
├── .github/workflows/ci.yml    # CI pipeline (lint, test, build)
├── docker-compose.yml           # Development environment
├── Dockerfile                   # Multi-stage production image
├── Makefile                     # Development commands
└── sqlc.yaml                    # SQLC configuration
```

## Configuration

Goscratch uses a **3-layer configuration system** (each layer overrides the previous):

1. **JSON defaults** -- `config/config.default.json`
2. **.env file** -- loaded via godotenv (optional, `cp .env.example .env`)
3. **Environment variables** -- highest priority, for production

| Setting | Default | Environment Variable |
|---------|---------|---------------------|
| App port | `3000` | `PORT` |
| Database host | `localhost` | `DB_HOST` |
| JWT secret | *(insecure)* | `JWT_SECRET` |
| Redis | disabled | `REDIS_ENABLED=true` |
| RabbitMQ | disabled | `RABBITMQ_ENABLED=true` |
| Email | disabled | `EMAIL_ENABLED=true` |

See `.env.example` and `config/config.default.json` for all options.

## Development

```bash
make dev                # API server with hot-reload (air)
make dev-worker         # Background worker with hot-reload
make test               # Unit tests
make test-integration   # Integration tests (requires Docker)
make test-ci            # Tests with coverage report
make lint               # golangci-lint
make build              # Production binaries (bin/goscratch + bin/worker)
make migrate-up         # Run migrations
make migrate-down       # Rollback last migration
make migrate-create NAME=xxx  # Create new migration
make sqlc               # Regenerate SQLC code
make docker-up          # Start PostgreSQL
make docker-full        # Start all services (Redis, RabbitMQ, etc.)
make seed               # Seed test data
make install-tools      # Install air, golangci-lint, sqlc, migrate
```

### Adding a New Module

1. Create `internal/module/yourmodule/` with domain, dto, handler, usecase, module.go
2. Add SQL queries and run `make sqlc`
3. Register in `internal/platform/app/app.go`

See `internal/module/user/` as reference.

## Deployment

Production configs in `deploy/`:

- **Docker Compose** -- `deploy/docker/docker-compose.prod.yml` (all services, healthchecks, resource limits)
- **Systemd** -- `deploy/systemd/` (API + Worker unit files)
- **Nginx** -- `deploy/nginx/nginx.conf` (reverse proxy, TLS, rate limiting)

## Documentation

- `docs/VISION.md` -- Project vision and design principles
- `docs/ROADMAP.md` -- Version history and feature tracking
- `docs/features/` -- Feature specifications (10 specs)
- `docs/adr/` -- Architecture decision records (7 ADRs)
- `/docs` -- Interactive API explorer (Scalar, runtime)
- `/health` -- Health check endpoint
- `/metrics` -- Prometheus metrics

## Tech Stack

| Component | Technology |
|-----------|-----------|
| Language | Go 1.25+ |
| Web framework | Fiber v2 |
| Database | PostgreSQL 18+ |
| SQL generation | SQLC |
| Authentication | JWT (golang-jwt) + bcrypt |
| Authorization | Casbin v3 |
| Cache | Redis (optional, NoOp fallback) |
| Queue | RabbitMQ (optional, NoOp fallback) |
| Storage | S3 / Local filesystem |
| Metrics | Prometheus |
| Tracing | OpenTelemetry |
| Logging | Structured JSON (slog) |
| Linting | golangci-lint |
| Hot-reload | Air |

## License

MIT
