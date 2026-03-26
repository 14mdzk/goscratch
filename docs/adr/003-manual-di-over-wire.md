# ADR-003: Manual DI Over Wire/fx

## Status
Accepted

## Context
Dependency injection is needed to wire together repositories, usecases, handlers, adapters, and middleware. Options included Google Wire (compile-time codegen), Uber fx (runtime reflection), or manual wiring in a single composition root.

## Decision
We use manual dependency injection in `internal/platform/app/app.go`. Every dependency is created explicitly with visible constructor calls and passed to the components that need it. There is no DI framework.

## Consequences
- **Pro:** Every dependency is traceable with a simple "find usages" -- no generated or reflected code
- **Pro:** No build-time codegen step or runtime container to understand
- **Pro:** Compilation errors immediately surface missing or mismatched dependencies
- **Pro:** New developers can read `app.go` top-to-bottom to understand the full application graph
- **Con:** `app.go` grows linearly with the number of modules
- **Con:** Adding a new dependency requires manually threading it through constructors
