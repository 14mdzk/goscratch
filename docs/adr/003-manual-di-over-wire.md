# ADR-003: Manual DI Over Wire/fx

## Status
Accepted

## Context
Dependency injection is needed to wire together repositories, usecases, handlers, adapters, and middleware. Options included Google Wire (compile-time codegen), Uber fx (runtime reflection), or manual wiring in a single composition root.

## Decision
We use manual dependency injection in `internal/platform/app/` (`app.go`, `infra.go`, `http.go`, `modules.go`). Every dependency is created explicitly with visible constructor calls and passed to the components that need it. Cross-cutting HTTP middleware (such as JWT authentication built via `middleware.AuthConfigFromJWT`) is constructed once in the composition root and injected into modules rather than rebuilt per module. There is no DI framework.

## Consequences
- **Pro:** Every dependency is traceable with a simple "find usages" -- no generated or reflected code
- **Pro:** No build-time codegen step or runtime container to understand
- **Pro:** Compilation errors immediately surface missing or mismatched dependencies
- **Pro:** Splitting the composition root by phase (`infra.go`, `http.go`, `modules.go`) keeps `app.go` concise while preserving top-to-bottom readability
- **Con:** Adding a new dependency requires manually threading it through constructors
