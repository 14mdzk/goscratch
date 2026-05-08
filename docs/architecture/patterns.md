# Architecture Patterns

Contributor reference for the four canonical idioms enforced across all modules.

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

## Shared Repo Injection

Auth and user modules share a single `*userrepo.Repository` instance created
once in `internal/platform/app/app.go`. `auth.NewModule` accepts a `usecase.UserRepo`
interface (not `*pgxpool.Pool`) so the same underlying connection pool is reused.

**Why**: avoids a second repository being created for the same pool (audit finding
auth/module.go:20).

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
