# Architecture Patterns

Contributor reference for the canonical idioms enforced across all modules and the composition root.

---

## Composition Root Layout (`internal/platform/app/`)

Manual dependency wiring (see [ADR-003](../adr/003-manual-di-over-wire.md)) is split by startup phase across four files in `internal/platform/app/`:

- `app.go` — `App` struct, thin `New` orchestrator (`cfg.Validate()` → `initInfra` → `initHTTP` → `initModules` → `authorizer.Start`), `Start`, and phased `Shutdown`.
- `infra.go` — `initInfra` and per-adapter constructors (`initTracer`, `initCache`, `initQueue`, `initStorage`, `initSSE`, `initAuditor`, `initAuthorizer`, `initEmail`, `initNotification`), following the fail-fast contract in [ADR-012](../adr/012-fail-fast-boot-and-explicit-noop.md).
- `http.go` — `initHTTP`, CORS config, rate-limit middleware, and localhost-only `/metrics` listener.
- `modules.go` — `initModules` (module construction and route registration) and `initInProcessWorker`.

Built-in worker handlers are registered through a single shared function — `handlers.RegisterDefault(w, handlers.Dependencies{...})` in `internal/worker/handlers/register.go` — called by both `cmd/worker/main.go` and `internal/platform/app/modules.go` so standalone and in-process workers never drift.

Build version metadata lives in `internal/platform/version.Version` (defaults to `"dev"`, injected via `-ldflags` in `Makefile` and `Dockerfile`).

---

## Centralized Auth Middleware Injection

The JWT authentication middleware is constructed **once** in the composition root from `cfg.JWT`:

```go
authMiddleware := middleware.Auth(middleware.AuthConfigFromJWT(cfg.JWT))
```

Every module with protected routes (`auth`, `user`, `role`, `storage`, `sse`, `job`, `audit`) accepts `authMiddleware fiber.Handler` in `NewModule(...)` instead of receiving raw JWT secrets or calling `middleware.DefaultAuthConfig(...)`.

**Why:**
- Eliminates repeating secret / rotation / issuer / audience plumbing across every module constructor.
- Guarantees custom `JWT_ISSUER` and `JWT_AUDIENCE` values match between token issuance (`auth/usecase`) and token verification (`middleware.Auth`).
- Lets `cmd/openapi-drift` register all routes with a trivial no-op `fiber.Handler`.

---

## UseCase Port Idiom

Every module exposes a `UseCase` interface in `internal/module/<x>/usecase/port.go`.
Handlers accept the interface, not the concrete struct. The concrete type is
unexported (e.g. `type roleUseCase struct`) and returned by `NewUseCase() UseCase`.

**Why**: enables audit-decorator injection, mock-based unit testing, and respects
ADR-001 (handler must not depend on concrete usecase).

**Files**: `internal/module/{auth,user,role,storage,job}/usecase/port.go`

**Compile-time check** in concrete file:
```go
var _ UseCase = (*roleUseCase)(nil)
```

---

## Claims Domain Boundary

`internal/module/auth/domain/claims.go` defines `Claims` — the domain
representation of an authenticated caller.

The JWT layer (`internal/platform/http/middleware/auth.go`) owns the
`Claims` struct (which embeds `jwt.RegisteredClaims`) for signing/parsing,
then maps it to `authdomain.Claims` via `toDomainClaims`. The Auth middleware
stores `*authdomain.Claims` in `c.Locals("user")`, so handlers and usecases
depend on the domain type, not on a JWT library type.

**Import rule**: only `middleware/auth.go` and `auth/usecase/auth_usecase.go`
may import `github.com/golang-jwt/jwt/v5`. All other code uses `authdomain.Claims`.

---

## Shared Repo & Port Transactor Injection

Auth and user modules share a single `*userrepo.Repository` instance created
once in `internal/platform/app/modules.go`:

- `auth.NewModule` accepts `auth/usecase.UserRepo` (read-only lookup subset).
- `user.NewModule` accepts `user/usecase.UserRepo` and `port.Transactor` (satisfied by `*database.Transactor`).

**Why:** neither `module/user` nor `module/user/usecase` imports `pgxpool`, `internal/platform/database`, or `user/repository` directly — enforcing ADR-001's strict `handler -> usecase -> port -> adapter/repository` boundary and avoiding duplicate repository wrappers.

## Compile-Time Service Composition (`auth/usecase.Service`)

When a usecase satisfies a secondary cross-module port in addition to its handler `UseCase` (e.g., `auth/usecase` exposing `Revoker` to `module/user`), combine them into a named `Service` interface returned by `NewUseCase` (`type Service interface { UseCase; Revoker }`) rather than type-asserting at runtime in `module.go`.

---

## `errors.Is` Convention

All sentinel comparisons must use `errors.Is`, not `==`:

```go
// correct
if errors.Is(err, pgx.ErrNoRows) { ... }
if errors.Is(err, redis.Nil) { ... }

// wrong
if err == pgx.ErrNoRows { ... }
```

When wrapping and re-returning errors from adapters, use `%w` or `WithError`:

```go
// preserve chain for errors.Is / errors.As
return apperr.ErrInternal.WithError(err)
return fmt.Errorf("context: %w", err)

// severs chain — avoid
return apperr.Internalf("failed: %s", err.Error())
```
