# Authorization

The authorization subsystem is built on [Casbin v3](https://casbin.org/) with a
PostgreSQL-backed policy store.  It exposes the `port.Authorizer` interface consumed
by HTTP middleware and use-case layers.

---

## Overview

| Component | Package | Purpose |
|-----------|---------|---------|
| `Adapter` | `internal/adapter/casbin` | Production Casbin adapter backed by PostgreSQL |
| `NoOpAdapter` | `internal/adapter/casbin` | Test/dev adapter — permits every call |
| `NoopWatcher` | `internal/adapter/casbin` | Watcher stub (no-op, single-instance) |
| `MemoryWatcher` | `internal/adapter/casbin` | In-process watcher via buffered channel |
| `RedisWatcher` | `internal/adapter/casbin` | Multi-instance watcher via Redis Pub/Sub |

---

## Configuration

```go
cfg := casbin.Config{
    DatabaseURL:    "postgres://...",
    ModelText:      "",            // empty = built-in RBAC model
    ReloadInterval: 5 * time.Minute, // 0 = default 5 minutes
    Watcher:        watcher,       // nil = backstop tick only
}
adapter, err := casbin.NewAdapter(cfg)
```

`NewAdapter` opens the database, creates the SQL adapter, loads policies, and
returns the adapter.  **No goroutines are started in `NewAdapter`.**  Call `Start`
to begin the lifecycle.

---

## Lifecycle — `Start(ctx context.Context) error`

`Start` must be called before the adapter is used in production.  It:

1. If a `Watcher` is configured:
   - Calls `watcher.SetUpdateCallback` with the incremental-apply callback.
   - Calls `enforcer.SetWatcher(watcher)` so Casbin notifies the watcher on every
     policy mutation.
2. Launches the **backstop tick goroutine** (see below).
3. Returns `nil` on success.

The goroutine exits cleanly when the supplied `ctx` is cancelled (typically on
application shutdown).

Wiring `Start` into application startup is handled by **PR-04**; do not call it
outside that path in production code.

---

## Backstop Tick

Even when a watcher is configured, the adapter runs a periodic `LoadPolicy()` as a
safety net to recover from missed messages or watcher failures.

The interval is controlled by `Config.ReloadInterval` (default: **5 minutes**).
Setting a shorter interval is appropriate for high-churn policies; longer intervals
reduce database load.

---

## Watcher Options

### `NoopWatcher`

Satisfies `persist.WatcherEx` with no-ops.  Use this in single-instance deployments
or when you want only the backstop tick for policy synchronisation.

```go
casbin.Config{Watcher: casbin.NewNoopWatcher()}
```

### `MemoryWatcher`

Routes policy-change signals through a buffered in-process channel (size 64).
All `UpdateFor*` mutations send a JSON-encoded operation; the adapter's callback
applies the delta without a full `LoadPolicy`.  Messages are dropped (with a warning
log) when the channel is full.

```go
w := casbin.NewMemoryWatcher()
w.Start(ctx) // start the dispatch goroutine
casbin.Config{Watcher: w}
```

Suitable for: tests, single-binary deployments where all enforcers share the same
process.

### `RedisWatcher`

Distributes policy-change signals across multiple instances via Redis Pub/Sub on the
`casbin:policy:update` channel (configurable).  Every instance that shares the same
channel will have its callback invoked when any instance mutates the policy.

```go
w, err := casbin.NewRedisWatcher(ctx, redisClient, "") // "" = default channel
casbin.Config{Watcher: w}
```

The subscriber goroutine starts inside `NewRedisWatcher`.  Call `w.Close()` during
shutdown to release the Pub/Sub subscription.

---

## Incremental Policy Load

Rather than calling `enforcer.LoadPolicy()` (a full round-trip to the database) on
every mutation, the watcher callback decodes the operation encoded in the message
and applies only the specific change:

| `op` field | Enforcer call |
|------------|---------------|
| `add_policy` | `enforcer.AddPolicy(params...)` |
| `remove_policy` | `enforcer.RemovePolicy(params...)` |
| `add_grouping` | `enforcer.AddGroupingPolicy(params...)` |
| `remove_grouping` | `enforcer.RemoveGroupingPolicy(params...)` |
| `reload` (or unknown) | `enforcer.LoadPolicy()` — full reload |

Filtered removes (`UpdateForRemoveFilteredPolicy`) and save operations
(`UpdateForSavePolicy`) always trigger a full reload because the result set is
non-trivial to replicate incrementally.

---

## Input Validation

All policy-mutation methods (`AddRoleForUser`, `RemoveRoleForUser`,
`AddPermissionForRole`, `RemovePermissionForRole`, `AddPermissionForUser`,
`RemovePermissionForUser`) reject arguments that contain null bytes (`\x00`).

A rejected call returns an error wrapping `casbin.ErrInvalidPolicyArg`.

---

## RBAC Model

The default built-in model is:

```ini
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && (p.obj == "*" || r.obj == p.obj) && (p.act == "*" || r.act == p.act)
```

Wildcard `*` is supported for both `obj` and `act`.  A custom model may be provided
via `Config.ModelText`.

---

## Decision Cache

### What It Is

`Adapter` keeps a per-process LRU cache that maps `(sub, obj, act)` triples to
`bool` authorization decisions.  When the same triple is evaluated again, the
cached answer is returned without re-running the Casbin model against the
in-memory policy table — eliminating allocations and lock contention on the hot
path.

Cache key encoding: `sub + "\x00" + obj + "\x00" + act`.  If any of the three
arguments contains `\x00`, the cache lookup and store are bypassed entirely
(the enforcer is called directly).  This prevents cache key collisions from
untrusted input passed to `Enforce`/`EnforceWithContext`, which do not
validate arguments through `validatePolicyArgs`.

Errors from `enforcer.Enforce` are **never cached**.  A transient error on one
call does not poison subsequent calls.

### Defaults

| Setting | Default | Effect |
|---------|---------|--------|
| `Config.DecisionCacheSize` | 0 (→ 10 000) | LRU capacity; 0 means "use default"; negative disables |

### Eviction

Entries are evicted by the LRU algorithm once the cache reaches capacity.  The
oldest-accessed entry is dropped.  There is no TTL — correctness is maintained
entirely through explicit invalidation (see below).

### Invalidation Matrix

Every policy-mutation method invalidates the affected cache entries immediately
after the mutation succeeds:

| Mutation | Invalidation scope | Rationale |
|----------|--------------------|-----------|
| `AddRoleForUser(user, role)` | All entries where `sub == user` | User's effective permission set changed |
| `RemoveRoleForUser(user, role)` | All entries where `sub == user` | User's effective permission set changed |
| `AddPermissionForRole(role, obj, act)` | **Entire cache flush** | Any user inheriting `role` transitively is affected; full flush is the conservative correct choice |
| `RemovePermissionForRole(role, obj, act)` | **Entire cache flush** | Same transitive-inheritance reason |
| `AddPermissionForUser(user, obj, act)` | All entries where `sub == user` | Direct permission addition |
| `RemovePermissionForUser(user, obj, act)` | All entries where `sub == user` | Direct permission removal |
| `LoadPolicy()` | **Entire cache flush** | Full policy reload — all cached decisions may be stale |
| Watcher callback — `add_policy` / `remove_policy` / `add_grouping` / `remove_grouping` | **Entire cache flush** | Policy changed; safest to start clean |
| Watcher callback — `reload` (or unknown op) | **Entire cache flush** (via `LoadPolicy`) | Full reload |
| `SavePolicy()` | No invalidation | Does not change the in-memory enforcer state |

Design note on role-permission mutations: precise invalidation would require
traversing the full role hierarchy with `GetImplicitUsersForRole` to find every
transitively-affected user.  A full flush is chosen as the conservative safe
alternative.  Role-permission mutations are infrequent in practice.

### Operator Guidance

```go
// Default (10 000 entries) — recommended for production.
casbin.Config{DatabaseURL: "..."}

// Explicit capacity — reduce if RAM is a concern.
casbin.Config{DatabaseURL: "...", DecisionCacheSize: 500}

// Disabled — every Enforce call re-evaluates the model.
casbin.Config{DatabaseURL: "...", DecisionCacheSize: -1}
```

Disable the cache (`DecisionCacheSize: -1`) only if you need sub-second
policy visibility without a watcher — the backstop tick runs every 5 minutes
and a full flush on `LoadPolicy` already guarantees correctness; the cache
does not introduce stale windows under normal operation.

### Bench Evidence

Measured on Apple M4, Go 1.25, in-memory enforcer with 100 seeded policies:

```
BenchmarkEnforce_NoCache-10                   74961   14470 ns/op   11470 B/op   280 allocs/op
BenchmarkEnforce_Cached-10                 70727780      17 ns/op       0 B/op     0 allocs/op
BenchmarkEnforce_Cached_RotatingKeys-10    21980252      54 ns/op       7 B/op     1 allocs/op
BenchmarkEnforce_NoCache_RotatingKeys-10      82940   14192 ns/op   11433 B/op   280 allocs/op
```

Single hot key: **847× faster, zero allocations**.
100-key rotation (realistic multi-user traffic): **262× faster**.

Run locally:

```bash
go test -bench=. -benchmem -run=^$ ./internal/adapter/casbin/
```
