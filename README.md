# Go Backend Boilerplate

A production-ready, modular Go backend boilerplate with clean architecture, SQLC, JWT authentication, and full observability stack (LGTM).

## ✨ Features

- **Modular Monolith Architecture** - Clean separation of modules, ports, adapters
- **SQLC** - Type-safe SQL with auto-generated Go code
- **JWT Authentication** - Login, token refresh, bcrypt password hashing
- **UUID v7** - Native PostgreSQL 18+ support via google/uuid
- **Observability (LGTM Stack)**
  - Prometheus metrics (`/metrics` endpoint)
  - OpenTelemetry tracing (Tempo integration)
  - Structured logging (Loki-compatible with trace correlation)
  - Grafana dashboards
- **Plug & Play Adapters**
  - Cache: Redis / NoOp
  - Queue: RabbitMQ / NoOp
  - Storage: Local / S3 / Composite
  - Audit: PostgreSQL / NoOp
- **Cursor-based Pagination** - Efficient pagination for large datasets
- **Docker Compose** - Full development environment

## 📁 Project Structure

```
.
├── cmd/
│   └── api/              # API entry point
├── config/               # Configuration files
├── internal/
│   ├── adapter/          # External service adapters (redis, rabbitmq, s3, etc.)
│   ├── module/           # Feature modules (user, auth, health)
│   ├── platform/         # Framework integrations (fiber, config, database)
│   ├── port/             # Interfaces for plug & play components
│   └── shared/           # Shared domain types (pagination, etc.)
├── migrations/           # PostgreSQL migrations
├── pkg/                  # Reusable packages
│   ├── apperr/           # Structured application errors
│   ├── logger/           # Structured logging
│   ├── pgutil/           # PostgreSQL utilities (UUID, errors)
│   ├── response/         # HTTP response helpers
│   └── types/            # Generic optional types (Opt, NOpt)
├── scripts/              # Database seeder
├── docker-compose.yml    # Development environment
├── Makefile              # Development commands
└── sqlc.yaml             # SQLC configuration
```

## 🚀 Quick Start

### Prerequisites

- Go 1.22+
- PostgreSQL 15+ (18+ for native UUID v7)
- Docker & Docker Compose (optional)

### 1. Clone and Setup

```bash
git clone https://github.com/14mdzk/goscratch.git
cd goscratch

# Copy and configure
cp config/config.example.json config/config.json
# Edit config/config.json with your settings
```

### 2. Start Services (Docker)

```bash
docker-compose up -d

# Wait for PostgreSQL to be ready
docker-compose logs -f postgres
```

### 3. Run Migrations

```bash
make migrate-up
```

### 4. Seed Database (Optional)

```bash
make seed
```

Creates test users:
| Email | Password | Role |
|-------|----------|------|
| `superadmin@example.com` | `password123` | Super Admin |
| `admin@example.com` | `password123` | Admin |
| `user@example.com` | `password123` | User |

### 5. Start Development Server

```bash
make dev
```

Server runs at `http://localhost:3000`

## 📡 API Endpoints

### Authentication

```bash
# Login
POST /auth/login
{"email": "admin@example.com", "password": "password123"}

# Refresh Token
POST /auth/refresh
{"refresh_token": "..."}

# Logout
POST /auth/logout
{"refresh_token": "..."}
```

### Users (Protected)

```bash
# List users (with pagination & filters)
GET /users?limit=10&cursor=...&search=john&is_active=true

# Get current user
GET /users/me

# Get user by ID
GET /users/:id

# Create user
POST /users
{"email": "new@example.com", "password": "password123", "name": "New User"}

# Update user
PUT /users/:id
{"name": "Updated Name"}

# Change password
POST /users/me/password
{"current_password": "...", "new_password": "..."}

# Activate/Deactivate user
POST /users/:id/activate
POST /users/:id/deactivate

# Soft delete user
DELETE /users/:id
```

### Health & Metrics

```bash
GET /health          # Health check
GET /metrics         # Prometheus metrics
```

## 🔧 Make Commands

```bash
make dev             # Start development server
make build           # Build production binary
make test            # Run all tests
make test-short      # Run unit tests only
make sqlc            # Generate SQLC code
make migrate-up      # Run migrations
make migrate-down    # Rollback migration
make migrate-create  # Create new migration
make seed            # Seed database
make lint            # Run linter
```

## ⚙️ Configuration

Configuration is loaded from `config/config.json` with environment variable overrides.

```json
{
  "app": {
    "name": "goscratch",
    "env": "development",
    "port": 3000
  },
  "database": {
    "host": "localhost",
    "port": 5432,
    "user": "postgres",
    "password": "postgres",
    "name": "goscratch"
  },
  "jwt": {
    "secret": "your-secret-key",
    "access_token_ttl": 15,
    "refresh_token_ttl": 10080
  },
  "cache": {
    "enabled": false,
    "host": "localhost",
    "port": 6379
  },
  "observability": {
    "metrics": {"enabled": true, "port": 9090},
    "tracing": {"enabled": false, "endpoint": "http://localhost:4317"}
  }
}
```

## 🏗️ Adding New Modules

1. Create module directory:
   ```
   internal/module/yourmodule/
   ├── domain/     # Domain entities
   ├── dto/        # Request/Response DTOs
   ├── handler/    # HTTP handlers
   ├── repository/ # Database access (with SQLC)
   │   └── queries/  # SQL queries for SQLC
   ├── usecase/    # Business logic
   └── module.go   # Module registration
   ```

2. Add SQLC queries in `internal/module/yourmodule/repository/queries/`

3. Update `sqlc.yaml` and run `make sqlc`

4. Register module in `internal/platform/app/app.go`

## 🧪 Testing

```bash
# Run all tests
make test

# Run unit tests only (skip integration)
make test-short

# Run specific package tests
go test -v ./internal/module/user/...
```

## 🐳 Docker

```bash
# Build image
docker build -t goscratch .

# Run with Docker Compose (includes PostgreSQL, Redis, Prometheus, etc.)
docker-compose up -d
```

## 📊 Observability

- **Prometheus**: http://localhost:9090
- **Grafana**: http://localhost:3001 (admin/admin)
- **Tempo** (Tracing): http://localhost:3200

## 📝 License

MIT

---

Built with ❤️ using Go, Fiber, SQLC, and PostgreSQL.
