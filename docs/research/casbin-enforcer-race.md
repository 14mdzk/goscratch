# Casbin enforcer concurrency safety — race probe findings

Evidence for the decision ticket [Decide enforcer concurrency safety](https://github.com/14mdzk/goscratch/issues/101) on the [v1.3 Route Map](https://github.com/14mdzk/goscratch/issues/83).

Branch: `research/101-casbin-enforcer-race` (throwaway; the probe is intended to graduate into the fix PR as a regression test).

## What was measured

`internal/adapter/casbin/zzz_race_probe_test.go` — a white-box, in-memory probe (no database, no Docker) exercising the adapter's real paths:

| Probe | Access pattern |
|---|---|
| `TestRaceProbe_PlainEnforcer_EnforceVsMutation` | 8 goroutines `Adapter.Enforce` (request path) vs 1 goroutine `AddPermissionForRole` / `RemovePermissionForRole` / `AddRoleForUser` + `makeUpdateCallback` (watcher path) |
| `TestRaceProbe_PlainEnforcer_LoadPolicyVsEnforce` | 8 goroutines `Adapter.Enforce` vs the backstop/watcher `Adapter.LoadPolicy` |
| `TestRaceProbe_SyncedEnforcer_EnforceVsMutation` | control: same pattern on `casbin.NewSyncedEnforcer`, bypassing `Adapter` (field type blocks reuse) |

Run: `go test -race -run TestRaceProbe -count=1 ./internal/adapter/casbin/`

## Results

```
--- FAIL: TestRaceProbe_PlainEnforcer_EnforceVsMutation (0.20s)     # data races
--- FAIL: TestRaceProbe_PlainEnforcer_LoadPolicyVsEnforce (0.22s)   # 82 race reports
ok        TestRaceProbe_SyncedEnforcer_EnforceVsMutation            # control: clean
```

Representative race (mutation path vs request path):

```
WARNING: DATA RACE
Write at 0x00c0000d3230 by goroutine 17:
  github.com/casbin/casbin/v3/model.Model.AddPolicy()
      model/policy.go:202
  ... (*Enforcer).addPolicyWithoutNotify() internal_api.go:63
  ... (*Enforcer).AddPolicy() management_api.go:237
  github.com/14mdzk/goscratch/internal/adapter/casbin.(*Adapter).AddPermissionForRole()
      internal/adapter/casbin/casbin.go:327

Previous read at 0x00c0000d3230 by goroutine 9:
  ... (*Enforcer).enforce() enforcer.go:802
  ... (*Enforcer).Enforce() enforcer.go:928
  github.com/14mdzk/goscratch/internal/adapter/casbin.(*Adapter).Enforce()
      internal/adapter/casbin/casbin.go:248
```

Race sites observed, all in the model's policy storage:

- `model/policy.go:202-203` — `assertion.Policy = append(...)` **and** `assertion.PolicyMap[...] = ...` (a Go map write)
- `model/policy.go:274-277` — `RemovePolicy` swap-with-last: `ast.Policy[index] = ast.Policy[lastIdx]`, `PolicyMap` write, then `ast.Policy = ast.Policy[:lastIdx]`
- `model/policy.go:45` — incremental role-link build
- Reader side: `enforcer.go:802-804` — `len(...Policy)` + `range ...Policy` inside `enforce`

Severity note: the reported races are on the policy **slice** backing array (torn reads → wrong allow/deny, or panic if re-sliced mid-iteration). The same statements write the policy **index map**; a concurrent map read is a fatal `concurrent map read and map write` runtime error, reachable through Casbin's index fast paths rather than through this probe's reader.

## Hot-path cost (miss path; cache hits never touch the enforcer)

```
BenchmarkProbe_EnforceParallel_Plain-8            (3 runs)  1138 / 864.8 / 742.4 ns/op
BenchmarkProbe_EnforceParallel_Synced-8           (3 runs)   735.5 / 740.6 / 728.0 ns/op
BenchmarkProbe_EnforceParallel_AdapterRWMutex-8   (3 runs)   734.5 / 728.1 / 737.1 ns/op
```

1000 policies, `-cpu=8`, all cache misses (`-benchtime=200000x`). Locking cost is inside run-to-run noise; the first reported plain run is warm-up jitter.

## Library facts that bear on the options (casbin v3.10.0)

- Plain `*Enforcer` has **no mutex**; `SyncedEnforcer` wraps it with a `sync.RWMutex` (`enforcer_synced.go:28-34`), locking every override (`Enforce` = RLock; mutations and `LoadPolicy` = Lock).
- The watcher notification happens **inside** the mutating call (`internal_api.go:387-394`), i.e. inside `SyncedEnforcer`'s write-lock window — a Redis publish extends the lock hold.
- Our watchers deliver callbacks on **their own goroutine** (`MemoryWatcher.Start` dispatch loop; `RedisWatcher.listen`) and are `persist.WatcherEx`, so `SetWatcher` installs no default callback and there is **no lock re-entry by the mutating goroutine** — no SyncedEnforcer deadlock. (The library's default callback for a non-`WatcherEx` watcher calls the *plain* `Enforcer.LoadPolicy`, which would bypass the lock — latent hazard, not hit today.)
- `SyncedEnforcer.LoadPolicy` is two-phase (RLock for load, drop, then Lock for the model swap) and skips the plain enforcer's `RunDetections()`/log hooks — harmless here (no detectors configured).
- `*Enforcer` and `*SyncedEnforcer` are **not** interchangeable for a typed field; the adapter's field would change type and the ~20 call sites follow (mechanical).

## Status

Probe is temporary (failing repro on current code, by design). Trim to the regression test in the fix PR; keep the control/benchmarks only if wanted.
