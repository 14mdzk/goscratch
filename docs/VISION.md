# Vision

## What Is Goscratch?

Goscratch is a production-ready Go backend starterkit built with Clean Architecture principles. It provides a modular, well-tested foundation so developers can skip the repetitive boilerplate and jump straight into building business logic.

## Problem Statement

Starting a new Go backend project from scratch means re-implementing the same infrastructure every time: authentication, authorization, user management, config loading, middleware, observability, background jobs, file storage, caching. This takes weeks before any real business logic gets written.

Existing Go frameworks either do too much (opinionated full frameworks) or too little (just routing). There's a gap for a **structured starterkit** that provides production-grade infrastructure with clean boundaries, without locking you into a framework.

## Target Audience

Go developers (solo or team) building REST API backends who want:

- A clean starting point with sensible defaults
- Production patterns they can learn from and extend
- Modular components they can enable/disable as needed
- A codebase that's easy to understand and modify

## Design Principles

### 1. Modular by Default

Every external dependency (Redis, RabbitMQ, S3, etc.) has a NoOp fallback. Enable what you need, disable what you don't. A fresh clone works with just PostgreSQL.

### 2. Clean Architecture

Strict separation: `handler -> usecase -> port -> adapter/repository`. Business logic never depends on infrastructure. Swap implementations without touching business code.

### 3. No Magic

Manual dependency injection. No code generation required at runtime. No hidden globals. Every dependency is explicit and traceable in `app.go`.

### 4. Convention Over Configuration

Consistent patterns across all modules. Once you understand how the `user` module works, you know how to build any module.

### 5. Production-Ready from Day One

Structured logging, Prometheus metrics, OpenTelemetry tracing, health checks, graceful shutdown, audit logging. Not afterthoughts - built in from the start.

### 6. Test-Friendly

Ports/interfaces everywhere. Every adapter has a NoOp implementation. Mock-friendly architecture. No test should require an external service to run (integration tests opt-in).

## Non-Goals

- **Not a framework.** Goscratch is a starterkit you clone and own. There's no `goscratch` import in your code.
- **Not a microservice toolkit.** This is a modular monolith. Service mesh, gRPC, event sourcing are out of scope.
- **Not a frontend.** No templating, no SSR, no admin panel. Pure API backend.
- **Not opinionated about deployment.** Works on VPS with Docker, Kubernetes, or bare metal. Reverse proxy (Nginx, etc.) is your choice.

## Tech Stack

| Layer | Choice | Why |
|-------|--------|-----|
| Language | Go 1.25+ | Performance, simplicity, strong stdlib |
| Web Framework | Fiber v2 | Fast, Express-like API, good middleware ecosystem |
| Database | PostgreSQL 18+ | Reliable, feature-rich, native UUID v7 |
| SQL | SQLC | Type-safe, no ORM overhead, SQL-first |
| Auth | JWT (golang-jwt) | Stateless access tokens + cached refresh tokens |
| AuthZ | Casbin v3 | Flexible RBAC, DB-backed policies |
| Cache | Redis (go-redis) | Industry standard, optional via NoOp |
| Queue | RabbitMQ (amqp091) | Reliable messaging, optional via NoOp |
| Storage | S3 / Local filesystem | Pluggable via adapter pattern |
| Observability | Prometheus + OpenTelemetry + slog | LGTM stack compatible |
| Validation | go-playground/validator | Struct tag validation, widely adopted |
| Migration | golang-migrate | SQL-based, no ORM dependency |

## Success Criteria

Goscratch is considered complete when:

1. All planned features are implemented and documented
2. Test coverage is near 100% for all starterkit features
3. A developer can clone, run `make setup && make dev`, and have a working API in under 5 minutes
4. Every module follows the same consistent pattern
5. Documentation covers architecture decisions, feature specs, and API contracts
