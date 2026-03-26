# ADR-002: SQLC Over ORM

## Status
Accepted

## Context
We needed a database access strategy for PostgreSQL. The main options were: an ORM (GORM, Ent), a query builder (squirrel, goqu), or a code generator from SQL (SQLC). ORMs hide SQL behind abstractions that often produce suboptimal queries and make debugging harder. They also introduce large dependency trees.

## Decision
We chose SQLC, which generates type-safe Go code from hand-written SQL queries. SQL files live alongside the repository code, and `sqlc generate` produces the query functions and model structs.

## Consequences
- **Pro:** Full control over SQL -- no magic query generation or N+1 surprises
- **Pro:** Compile-time type safety for queries without runtime reflection
- **Pro:** Zero runtime overhead; generated code uses standard `pgx` directly
- **Pro:** Developers who know SQL are immediately productive
- **Con:** Every new query requires writing SQL and re-running `sqlc generate`
- **Con:** Dynamic filters (optional WHERE clauses) require manual handling in the repository layer
