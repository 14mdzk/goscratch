# ADR-010: Queue Backend Spectrum

Date: 2026-10-17
Status: accepted (decides [Decide the queue backend spectrum](https://github.com/14mdzk/goscratch/issues/84) and [Decide in-process worker semantics](https://github.com/14mdzk/goscratch/issues/90))

## Context

The starterkit needs a queue story that works with zero infrastructure for
local development and templates, while supporting a durable broker in
production. Previously the code knew only two states: RabbitMQ when
`rabbitmq.enabled=true`, and a silent NoOp otherwise — jobs published with
RabbitMQ disabled vanished without a trace, and a broker outage at boot
quietly degraded to the same black hole.

## Decision

A three-tier spectrum behind `port.Queue`, selected by `queue.backend`
(`goroutine` | `redis` | `rabbitmq`):

- `goroutine` is the default: an in-memory channel backend with best-effort
  durability (jobs die with the process). It is always consumed in-process
  (`worker.in_process=true`, enforced at boot); the worker starts before the
  HTTP listener and a start failure aborts boot.
- `rabbitmq` is the durable tier: first-class, unreachable broker fails
  boot (fail fast, no silent fallback). It is consumed by the external
  `cmd/worker` process; the API never joins consumption as a competing
  consumer (`worker.in_process=true` with rabbitmq fails boot).
- `redis` is a reserved enum value that fails boot until implemented.

Compatibility: when `queue.backend` is unset, the deprecated
`rabbitmq.enabled` alias applies (true → rabbitmq, false → goroutine).
Unknown values fail boot. The dead `worker.enabled` key is deleted;
`worker.concurrency` is canonical (default 2).

## Alternatives considered

- Keep RabbitMQ-or-NoOp: rejected — the NoOp black hole hides outages and
  the zero-infra default needs real (if ephemeral) delivery.
- Adapter-level in-process flag without backend split: rejected — durability
  expectations differ per tier and must be explicit in config.
- Build the Redis tier now: rejected — reserved; no consumer asks for it.

## Consequences

- New deployments run `goroutine` + in-process with no extra services.
- Existing deployments: at rest, `rabbitmq.enabled=false` now resolves to
  `goroutine` (jobs are consumed in-process instead of dropped) and
  requires `worker.in_process=true` (shipped as the new default in
  `config.default.json`); `rabbitmq.enabled=true` resolves to `rabbitmq`
  unchanged. Removing a deployment's `worker.enabled` key is required only
  if strict JSON is validated — unknown keys are otherwise ignored.
- Operator action on upgrade: set `queue.backend` explicitly (recommended)
  and ensure `worker.in_process` matches the matrix; see
  `docs/features/background-jobs.md`.
