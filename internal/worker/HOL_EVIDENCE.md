# #148 evidence: RabbitMQ consumer head-of-line blocking

Repro: `internal/worker/hol_evidence_test.go` (`-tags=integration`), live
`rabbitmq:3-management-alpine` via `testutil.StartRabbitMQ` under colima
(`DOCKER_HOST=unix://~/.colima/default/docker.sock`,
`TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE=/var/run/docker.sock`).

Method note: the ticket suggested k6 with a slow handler, but k6 measures the
HTTP request path, which is async-decoupled from the consumer (dispatch is a
publish; nothing blocks HTTP). Consumer-side timing is the layer that answers
the question, so the repro publishes slow/fast/failing jobs and measures fast
completion latency from publish. Two consecutive runs, identical shape.

| Exp | Setup | Fast latency (run 1 / run 2) |
|---|---|---|
| A | conc=1, prefetch=10, slow(5s)→fast | 5.002s / 5.001s |
| B1 | conc=2, prefetch=10, slow+fast+fast | 5.002s / 5.002s (last fast) |
| B2 | conc=2, prefetch=1, slow+fast+fast | 1ms / 1ms (last fast) |
| C | conc=1, always-fail→fast (first hold) | 1s / 801ms |

Mechanism (code): `drainDeliveries` invokes the handler synchronously
(`internal/adapter/queue/rabbitmq.go`); each worker goroutine is one serial
consumer. Default `PrefetchCount` is 10, so the broker strands up to 10
unacked deliveries on an occupied consumer instead of routing to a free one
(B1 vs B2). Held retries occupy the consumer across the backoff
(`holdForRetry`, `worker.go`): first hold [0.5s,1s) measured (C, jitter
visible across runs); attempts ≥7 hold [30s,60s) by code reading
(`retryMaxDelay`).

Verdict for the decision ticket: practical, not theoretical, at starterkit
scale — at canonical concurrency=2 + prefetch=10, two simultaneous slow jobs
(or held retries) stall everything behind them. `prefetch=1` is a cheap
partial mitigation (B2) but does not fix all-consumers-busy.
