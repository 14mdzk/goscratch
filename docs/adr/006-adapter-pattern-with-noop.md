# ADR-006: Adapter Pattern with NoOp Fallbacks

## Status
Accepted

## Context
The starterkit uses multiple external services (Redis, RabbitMQ, S3, SMTP) that may not be available in every environment. Developers should be able to run the application with just PostgreSQL during development, and production deployments should enable only the services they need.

## Decision
Every external dependency is accessed through a port interface, and every port has both a real adapter and a NoOp adapter. When a service is disabled in config or fails to initialize, the NoOp adapter is used automatically. NoOp adapters either silently succeed or log the operation.

Adapters with NoOp fallbacks:
- Cache: `RedisCache` / `NoOpCache`
- Queue: `RabbitMQ` / `NoOpQueue`
- Storage: `S3Storage`, `LocalStorage` (S3 falls back to local on init failure)
- SSE: `Broker` / `NoOpBroker`
- Audit: `PostgresAuditor` / `NoOpAuditor`
- Authorization: `CasbinAdapter` / `NoOpAdapter`
- Email: `SMTPSender` / `NoOpSender`

## Consequences
- **Pro:** A fresh clone works with just PostgreSQL -- no Redis, RabbitMQ, or S3 required
- **Pro:** Tests can use NoOp adapters without spinning up external services
- **Pro:** Graceful degradation -- if Redis connection fails, the app still starts with NoOp cache
- **Con:** NoOp adapters may silently mask issues (e.g., refresh tokens not persisting)
- **Con:** Each new external dependency requires implementing two adapters
