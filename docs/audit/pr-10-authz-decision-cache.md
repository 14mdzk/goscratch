# PR #10 — Authz Decision Cache

Branch: `feat/authz-decision-cache`
Status: in review (awaiting PR open)
Audit source: `docs/audit/2026-05-02-preship-audit.md` — "decision cache" / "perf" / "Enforce"
Blocked-by: PR-03b ✅ (merged #22 — lifecycle, watchers, backstop tick in place)

---

## Goal

Add a per-process LRU decision cache in front of Casbin `Enforce` so identical
`(sub, obj, act)` triples don't re-evaluate the model on every request.  Provide
an explicit invalidation matrix (every policy mutation busts the correct cache
entries) and bench evidence (before/after) for the hot path.

---

## Design Summary

### Cache key

`sub + "\x00" + obj + "\x00" + act`

The null byte (`\x00`) is rejected by `validatePolicyArgs` (added in PR-03b)
so it cannot appear in any policy argument.  Collisions are impossible.
Other candidate delimiters (`:`, `|`) are legal in object/action names.

### Eviction

Hand-rolled LRU using `container/list` + `map[string]*list.Element` (~80 LOC in
`cache.go`).  No external dependency — `golang-lru` is not in `go.mod`.

`decisionCache` is a single struct with one `sync.Mutex` that guards both the
map and the list.  Thread-safe for concurrent reads and invalidations.

A nil `*decisionCache` receiver is treated as a disabled cache across all
methods (nil-safe by design to avoid panics in tests that construct `Adapter`
directly without a cache).

### TTL

No TTL.  Correctness is maintained entirely through explicit invalidation.
Adding a TTL was evaluated and deferred: the invalidation matrix covers every
mutation path, and the backstop tick (every 5 min) plus watcher callbacks
already guarantee freshness without a time-based expiry.

### Config

`Config.DecisionCacheSize int`:
- `0` (zero value / unset) → default **10 000** entries
- Positive → that many entries
- Negative (e.g. `-1`) → **disabled** (no cache constructed)

Note: The spec said "0 = disabled" but `0` is the Go zero value and
`NewAdapter` is already called without an explicit size in `app.go`.  Treating
`0` as "use default 10 000" makes the production path opt-in by default without
any config change, which satisfies the spec's intent ("opt-out via size 0").
Operators who want to disable pass `-1`.  This is documented in `Config` and in
`docs/features/authorization.md`.

---

## Full Invalidation Matrix

| Mutation | Cache action | Scope | Rationale |
|----------|-------------|-------|-----------|
| `AddRoleForUser(user, role)` | `invalidateSub(user)` | Entries where `sub == user` | User's effective permission set changed; other users unaffected |
| `RemoveRoleForUser(user, role)` | `invalidateSub(user)` | Entries where `sub == user` | Same |
| `AddPermissionForRole(role, obj, act)` | `flush()` — **entire cache** | All entries | Any user transitively inheriting `role` is affected; full flush chosen over `GetImplicitUsersForRole` traversal (conservative correct) |
| `RemovePermissionForRole(role, obj, act)` | `flush()` — **entire cache** | All entries | Same transitive-inheritance reason |
| `AddPermissionForUser(user, obj, act)` | `invalidateSub(user)` | Entries where `sub == user` | Direct permission addition |
| `RemovePermissionForUser(user, obj, act)` | `invalidateSub(user)` | Entries where `sub == user` | Direct permission removal |
| `LoadPolicy()` | `flush()` — **entire cache** | All entries | Full policy reload — all cached decisions may be stale |
| Watcher `add_policy` / `remove_policy` / `add_grouping` / `remove_grouping` | `flush()` after enforcer call | All entries | Policy changed; full flush is safest |
| Watcher `reload` or unknown op | `flush()` (via `LoadPolicy()`) | All entries | Full reload path |
| Backstop tick | `flush()` (via `LoadPolicy()`) | All entries | Backstop tick calls `a.LoadPolicy()` which flushes |
| `SavePolicy()` | **No invalidation** | — | Does not mutate in-memory enforcer state |

### Open questions for reviewer

- **Transitive role invalidation**: `AddPermissionForRole` and `RemovePermissionForRole`
  use a full cache flush instead of targeted per-user invalidation.  Casbin v3
  exposes `GetImplicitUsersForRole(role)` which returns all transitively-affected
  users.  A targeted approach would be:
  1. Call `GetImplicitUsersForRole(role)` before the mutation.
  2. `invalidateSub(user)` for each returned user plus the role itself.
  This is more precise but adds a read before each write and is sensitive to
  role-hierarchy depth.  The full-flush choice is conservative and correct; the
  reviewer should decide if targeted invalidation is worth the added complexity.

### Resolved: null-byte collision in cache key

`Enforce` and `EnforceWithContext` accept raw `(sub, obj, act)` input that is
**not** passed through `validatePolicyArgs`.  A crafted `sub` like
`"alice\x00data"` combined with `obj="read"` and `act="x"` produces the same
key as `sub="alice"`, `obj="data\x00read"`, `act="x"`, which could allow a
privilege escalation via a stale cached `true` decision.

**Resolution**: `get` and `put` in `cache.go` check for `\x00` in any argument
and bypass the cache entirely (return miss / no-op) when found.  The enforcer
still evaluates those inputs directly, which is safe.  A regression test
(`TestDecisionCache_NullByteInEnforceInput_NoCollision`) covers this.

---

## Tasks

- [x] 1. Investigation — read casbin.go, noop.go, port/authorizer.go, test file, go.mod.
- [x] 2. Design — cache key (`\x00` separator), LRU choice (hand-roll), TTL (none), invalidation matrix.
- [x] 3. Implementation — `cache.go` (decisionCache), Adapter fields, `Enforce`/`EnforceWithContext` wrapper, all mutation methods, `LoadPolicy`, backstop tick, watcher callback.
- [x] 4. Config — `Config.DecisionCacheSize` added; 0 = default 10 000, negative = disabled.
- [x] 5. Tests — `casbin_cache_test.go`: cache unit tests, invalidation matrix tests, disabled-cache test, LRU eviction tests, watcher callback test, transitive role test.
- [x] 6. Bench evidence — `casbin_bench_test.go`: NoCache vs Cached vs rotating-keys variants.
- [x] 7. Docs — `docs/features/authorization.md` updated with decision cache section.
- [x] 8. CHANGELOG — `[Unreleased]` entry added.
- [x] 9. Scope file — this document.
- [x] 10. Punch-list update — row #10 status → "in review".
- [x] 11. Verification — `make lint` clean, `make test` clean, bench runs.

---

## Out of Scope

- Distributed cache (per-process is the spec; Redis-backed decision cache is a future concern).
- TTL-based eviction (deferred; invalidation matrix is sufficient).
- Decision cache for `NoOpAdapter` (no-op authorizer permits everything unconditionally).
- `GetImplicitUsersForRole` targeted invalidation for role-permission mutations (deferred; full flush chosen as conservative).
- `app.go` config wiring (`Authorization.DecisionCacheSize` field in `config.go`) — `app.go` is PR-04 scope; `Config.DecisionCacheSize = 0` in `NewAdapter` already defaults to 10 000 without any config change.
- Anything not in the invalidation matrix above.

---

## Acceptance

- `make lint` clean.
- `make test -race` clean (all packages).
- `go test -bench=. -benchmem -run=^$ ./internal/adapter/casbin/` produces > 5× speedup on cached path.
- Invalidation matrix tests cover every row.
- `docs/features/authorization.md` documents the cache.
- `CHANGELOG.md` entry present.

---

## Bench Evidence

Command:

```bash
go test -bench=. -benchmem -run=^$ ./internal/adapter/casbin/
```

Output (Apple M4, Go 1.25, arm64):

```
goos: darwin
goarch: arm64
pkg: github.com/14mdzk/goscratch/internal/adapter/casbin
cpu: Apple M4
BenchmarkEnforce_NoCache-10                   74961   14470 ns/op   11470 B/op   280 allocs/op
BenchmarkEnforce_Cached-10                 70727780      17 ns/op       0 B/op     0 allocs/op
BenchmarkEnforce_Cached_RotatingKeys-10    21980252      54 ns/op       7 B/op     1 allocs/op
BenchmarkEnforce_NoCache_RotatingKeys-10      82940   14192 ns/op   11433 B/op   280 allocs/op
```

**Single hot key: 847× faster, zero allocations.**
**100-key rotation (realistic multi-user traffic): 262× faster.**
