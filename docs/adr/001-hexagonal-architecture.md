# ADR-001: Hexagonal Architecture

## Status
Accepted

## Context
We needed an architecture pattern for a Go backend starterkit that supports swappable infrastructure (databases, caches, queues) without touching business logic. Traditional MVC or layered architectures couple handlers directly to infrastructure, making testing and adapter swaps difficult.

## Decision
We adopted Hexagonal (Ports & Adapters) Architecture with the flow: `handler -> usecase -> port -> adapter/repository`. Business logic lives in usecases and depends only on port interfaces. Infrastructure implementations (Redis, RabbitMQ, S3, PostgreSQL) live in adapters that satisfy those ports.

Key conventions:
- `internal/port/` defines all interfaces (Cache, Queue, Storage, Auditor, Authorizer, EmailSender, SSEBroker)
- `internal/adapter/` contains implementations
- `internal/module/<name>/usecase/` contains business logic that depends only on ports
- `internal/module/<name>/handler/` handles HTTP concerns only

## Consequences
- **Pro:** Any adapter can be swapped without changing business logic
- **Pro:** Every dependency is mockable via its port interface, enabling thorough unit testing
- **Pro:** NoOp adapters allow running the app with minimal infrastructure (just PostgreSQL)
- **Con:** More files and indirection compared to a flat MVC structure
- **Con:** Developers must learn the convention before contributing
