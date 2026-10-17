# k6 Load Baseline (Tier F2)

Local-dev load harness. It baselines request rate + p99 for the login →
upload → download → delete loop, paced to stay under the default
sliding-window rate limiters. Not a CI gate — k6 needs a live seeded stack,
which CI does not run (same reason pg integration is local-gated).

## Prerequisites

1. Dependencies: `docker-compose up -d postgres redis`
   (or `make docker-up` if it covers both).
2. Schema + seed: `make migrate-up seed`
   (seeds `admin@example.com` / `admin123`; re-runnable).
3. API running with the **redis-backed** limiter (dev `.env` ships
   `REDIS_ENABLED=false`, which falls back to an in-memory limiter):
   `REDIS_ENABLED=true go run ./cmd/api/main.go`
   (default port `8080` when `SERVER_PORT` is unset).
4. k6 installed (`k6 version`).

## Run

```sh
make k6-baseline
# or: k6 run k6/baseline.js
# custom target: BASE_URL=http://host:port K6_EMAIL=... K6_PASSWORD=... k6 run k6/baseline.js
```

What it does: `setup()` logs in once (the auth limiter allows 20 req /
5 min / IP, so per-iteration login is impossible); 4 VUs then loop
upload → download → delete with a 10 s pace (≈72 req/min aggregate,
under the 100 req/min global limiter). The loop deletes what it uploads,
so a green run leaves no residue. Thresholds: failed rate < 1%,
p99 < 1500 ms.

Between runs leave ≥ 70 s quiet: the sliding window still holds the
previous run's requests, and even `setup()`'s single login 429s on a hot
window (observed while developing this harness). Runs fit the 15-minute
access-token TTL with wide margin.

## Baseline (recorded 2026-10-17)

MacBook colima Docker, `postgis` + `redis` from compose, API with
`REDIS_ENABLED=true`, default limiters (100 req/min global, 20 req/5 min
auth), goroutine in-process queue, `AUDIT_ENABLED=false`. Two consecutive
runs, identical shape:

| metric | run 1 | run 2 |
|---|---|---|
| http_reqs total (rate) | 127 (1.06/s, ≈63/min) | 127 (1.06/s) |
| iterations | 42 | 42 |
| checks succeeded | 127/127 | 127/127 |
| http_req_failed | 0.00% | 0.00% |
| http_req_duration avg / med | 3.72 ms / 2.60 ms | 3.74 ms / 2.38 ms |
| http_req_duration p90 / p95 / p99 | 5.47 / 5.78 / — ms | 5.34 / 5.85 / 7.00 ms |
| login_duration (1 sample) | 87.91 ms | 121.64 ms |
| thresholds | (output truncated, same shape) | p99<1500 ✓, rate<0.01 ✓ |

Floor reading: at ~1 req/s paced under the limiter, the files CRUD loop
serves p99 ≈ 7 ms on local dev hardware. Tighten the script thresholds
from new data when the hardware/profile changes — not from ambition.

## Failure modes seen

- `upload 201` failing with 415: payload must sniff as an allowlisted
  type (`http.DetectContentType` on magic bytes, not the multipart
  header). `k6/payload-10k.png` carries a real PNG signature + zero
  padding; verified with Go's sniffer before use.
- 429 storm + thousands of it/s: pacing `sleep` skipped on early return.
  The script has no early returns — every path ends at the paced sleep,
  so a failing endpoint slows down instead of hot-spinning.
- Setup 429 on a hot window (see above): wait, don't raise limits to
  force it — the point is baselining *under* the limiters.
