# Webhook Delivery Hardening Prior Art

Research artifact for #142 (v1.4 Route Map, charted in #141).
Question: survey retry policies, payload signing, idempotency, and
dead-lettering prior art so the webhook-scope decision has facts, and compare
against the current `notification.send` webhook backend.

All claims below come from the primary source cited inline. Local baseline
facts were read from `internal/adapter/notification/webhook.go`,
`internal/adapter/notification/noop.go`,
`internal/worker/handlers/notification_handler.go`,
`internal/worker/worker.go`, `internal/worker/job.go`,
`internal/platform/config/config.go`, and `docs/features/notifications.md`
at v1.3.0 (`7f1f831`).

## 1. Retry policies

### 1a. Shape: exponential backoff + jitter, multi-day horizon

- Standard Webhooks spec (v1.0.0) recommends retrying on a schedule spanning
  multiple days with exponential backoff, plus random jitter so attempts do
  not fire in lockstep after a shared outage. Success = `2xx`; anything else
  (non-`2xx`, timeout, connection reset) is failure. If an endpoint fails
  consistently over a long period, notify consumers out-of-band (e.g. email)
  and disable future delivery to the endpoint. Source:
  [Standard Webhooks spec](https://raw.githubusercontent.com/standard-webhooks/standard-webhooks/main/spec/standard-webhooks.md)
  ("Operational considerations" / "Deliverability and reliability").
- Svix publishes its exact schedule — 8 attempts: immediately, 5s, 5min,
  30min, 2h, 5h, 10h, 10h (each interval starts at the previous failure), ~27h
  total — then marks the message `Failed`. Source:
  [Svix Retry Schedule](https://docs.svix.com/retries) ("The schedule").
- Stripe retries live-mode event deliveries for up to 3 days with
  exponential backoff (sandbox events: 3 retries over a few hours); the next
  retry time is visible per event. Source:
  [Stripe webhooks doc](https://docs.stripe.com/webhooks) ("Automatic
  retries"). Svix's own retry best-practices note frames the rationale: retry
  fast twice to ride out a blip, then spread out so a long outage costs a
  handful of attempts. Source:
  [Svix best practices for sending](https://www.svix.com/resources/webhook-best-practices/sending/).
- AWS SNS structures HTTP/S retries as four phases (immediate, pre-backoff,
  backoff, post-backoff), applies jitter to retries, and lets only HTTP/S
  policies be customized (capped at 3,600s total). Sources:
  [SNS message delivery retries](https://docs.aws.amazon.com/sns/latest/dg/sns-message-delivery-retries.html),
  [AWS Architecture Blog on backoff + jitter](https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/).

### 1b. Status-code etiquette and Retry-After

- Standard Webhooks recommends: `2xx` success; `3xx` failure (update the URL
  instead of following redirects); `410 Gone` means stop sending to the
  endpoint; `429`/`502`/`504` mean throttle; honor `Retry-After` (e.g. on
  `503`) when scheduling the next attempt. Source: Standard Webhooks spec
  ("Delivery success and failure").
- Stripe treats `3xx` redirect responses as failures and tells operators to
  set the endpoint to the resolved URL. Source: Stripe webhooks doc
  (deliveries table, `302` row).

### 1c. Per-attempt timeouts

- Standard Webhooks recommends 15–30s per-attempt timeouts so consumers have
  time to process and acknowledge. Source: Standard Webhooks spec ("Request
  timeouts").
- Svix's sending best practices put the bound at 5–30s: a hung endpoint must
  not hold a delivery worker. Source: Svix best practices for sending.

## 2. Payload signing and receiver verification ergonomics

### 2a. Standard Webhooks scheme (Svix-compatible)

- Signed content is `msg_id.timestamp.payload` (dot-delimited), HMAC-SHA256
  for symmetric (`v1`) or ed25519 for asymmetric (`v1a`). Secrets are base64
  with `whsec_` / `whsk_` / `whpk_` prefixes. Metadata travels in
  `webhook-id`, `webhook-timestamp` (unix seconds), `webhook-signature`
  headers. Source: Standard Webhooks spec ("Signature scheme", "Webhook
  headers").
- Rotation without downtime: the sender signs with the current key AND the
  old key for a transition period and sends both signatures space-delimited;
  the receiver tries each until one verifies. Source: Standard Webhooks spec
  ("Webhook headers"). Keys must be unique per endpoint; asymmetric is
  preferred where the producer does not control both ends. Source: same,
  signature-scheme guidance list.
- Receiver rules: constant-time comparison for HMAC, battle-tested lib for
  asymmetric, reject timestamps outside a tolerance (replay defense), and use
  `webhook-id` as an idempotency key. Never re-serialize parsed JSON before
  verifying — byte-level changes break the signature. Source: Standard
  Webhooks spec ("Verifying signatures").

### 2b. Stripe scheme

- Per-endpoint `whsec_...` secret; `Stripe-Signature` header carries `t=`
  timestamp plus one or more `vN=` signatures, HMAC-SHA256 over
  `timestamp.payload`; only scheme `v1` is live (ignore others to block
  downgrade attacks); verification requires the raw, unmodified request body.
  Sources: Stripe webhooks doc ("Verify webhook signatures manually", "create
  a handler" callouts).
- Rotation: dashboard "Roll secret" can keep the old secret valid up to 24h;
  during that window Stripe emits one signature per active secret. Source:
  Stripe webhooks doc ("Roll endpoint signing secrets periodically").
- Ergonomics: official libraries verify from (payload, header, secret) and
  raise on failure; server SDKs keep per-endpoint secrets (test vs live
  differ). Sources: Stripe webhooks doc ("Verify with official libraries").

### 2c. Reference implementation (Go)

- `svix-webhooks` Go library (`go/webhook.go`): accepts `svix-*` or
  `webhook-*` headers, enforces a 5-minute timestamp tolerance both ways
  (too-old AND too-new), skips unknown signature versions, compares with
  `hmac.Equal` (constant-time), and offers an explicit
  `VerifyIgnoringTimestamp` escape hatch. Source:
  [svix-webhooks go/webhook.go](https://raw.githubusercontent.com/svix/svix-webhooks/main/go/webhook.go).
- Receiver-side ergonomics takeaway shared by all three: verification must be
  a 3-argument library call (body bytes, headers, secret) with tolerance
  built in, because hand-rolled timestamp checks and non-constant-time
  compares are the recurring failure modes.

## 3. Idempotency and delivery semantics

### 3a. Everyone redelivers: at-least-once is the norm

- Stripe: endpoints may receive the same event more than once; receivers must
  log processed event IDs and skip repeats; order is not guaranteed
  (`created` timestamps can tie), so do not use them for ordering or
  dedupe — use event IDs, and for double-generated events use
  `data.object` ID + `type`. Sources: Stripe webhooks doc ("Handle duplicate
  events", event-ordering section).
- Standard Webhooks: the message ID is stable across retries precisely so it
  can serve as the idempotency key (e.g. stash IDs for ~5 minutes). Source:
  Standard Webhooks spec ("Verifying signatures", unique-identifier section).
- Consequence: no surveyed provider promises at-most-once. Retries +
  redelivery make duplicates possible by design; correctness comes from
  receiver-side dedupe keys, not from the sender avoiding resends.

### 3b. Send-side idempotency keys are a separate mechanism

- Stripe's API accepts an `Idempotency-Key` header on POSTs: same key replays
  the stored response instead of re-executing (v1 stores status + body ≥24h
  and errors on parameter mismatch; v2 retries failed work to completion).
  Keys should be random (UUIDv4), ≤255 chars, never sensitive data. Source:
  [Stripe idempotent requests](https://docs.stripe.com/api/idempotent_requests).
- This is producer/API-side dedupe (safe client retries), distinct from the
  webhook receiver-side dedupe above. A webhook design needs the latter
  (stable message IDs); the former only matters if our dispatch API grows
  client retries.

## 4. Dead-lettering, parking, and operator replay

- AWS SNS: when the retry policy is exhausted the message is discarded
  *unless* a DLQ is attached; the DLQ is an ordinary SQS queue attached per
  subscription (so the failing endpoint is identifiable); client-side errors
  (deleted endpoint, bad policy) are NOT retried at all. Sources:
  [SNS dead-letter queues](https://docs.aws.amazon.com/sns/latest/dg/sns-dead-letter-queues.html),
  SNS retries doc.
- Svix: after the schedule exhausts, the message is marked `Failed` and the
  *sender* gets a `message.attempt.exhausted` operational webhook; endpoints
  failing long enough (failures ≥12h apart within 24h, then 5 more days of
  nothing but failures) are auto-disabled with an `endpoint.disabled` event.
  Operators recover via per-message resend API, "Recover Failed" (all
  failures since a date), and "Replay Missing" (never-attempted). Sources:
  Svix retries doc ("The schedule", "Disabling failing endpoints", "Manual
  retries").
- Stripe: per-event delivery state (`Delivered`/`Pending`/`Failed` + HTTP
  status + next-retry time) in the dashboard; manual resend from dashboard
  (≤15 days) or CLI (≤30 days); a manual resend does NOT cancel automatic
  retries, so receivers must still dedupe. Sources: Stripe webhooks doc
  ("Event deliveries" tab, "Manual retries").
- Shape of prior art: park verbatim payload + inspectable attempt history +
  explicit replay path + endpoint auto-disable as three separate features.

## 5. Baseline comparison: real gaps vs speculative ones

Local baseline (v1.3.0): `WebhookSender` does one unauthenticated POST with a
10s client timeout and fails on HTTP ≥400; `NotificationHandler` maps any
`Send` error to a job failure; the worker retries with exponential backoff +
jitter (`1s·2^(n-1)`, 60s cap, jitter [0.5,1)) up to `max_retry` (default 3),
parks exhausted/undecodable/unhandled jobs verbatim in `<queue>.dlq`, and
gives each handler run a 5-minute context. Config surface is two keys
(`notification.enabled`, `notification.webhook_url`).

| # | Candidate gap | Verdict | Why |
|---|---|---|---|
| 1 | No payload signing / auth | REAL | No signature, timestamp, or ID headers are sent; no secret config exists. Any receiver needing authenticity (the Standard/Stripe/Svix case) cannot verify us. |
| 2 | No stable message ID for receiver dedupe | REAL | Payload has no ID; job ID exists but is never transmitted. System is at-least-once (retry + broker requeue on shutdown) with no dedupe aid — §3a says that combination requires a transmitted ID. |
| 3 | Retry horizon is minutes, not days | REAL | Default 3 attempts with 60s cap ≈ seconds of coverage vs multi-day (Standard), ~27h/8 attempts (Svix), 3 days (Stripe). Multi-hour outages always become DLQ parks. |
| 4 | No `Retry-After` / status-code etiquette | REAL | All HTTP ≥400 (incl. 429/503/410) is one undifferentiated error; no throttle, no 410-disable, no redirect policy beyond Go-default follows. |
| 5 | No replay/redrive path for the DLQ | REAL | Park is verbatim (good) but the code comments confirm "no redrive tooling in v1.3"; Stripe/Svix both pair parking with resend/recover APIs. |
| 6 | No per-attempt timeout tuning | PARTLY REAL | 10s client timeout is slightly under the 15–30s (Standard) / 5–30s (Svix) guidance, and is a code constant, not config. Minor vs #1–#5. |
| 7 | No endpoint auto-disable | SPECULATIVE for v1.4 | Single static URL, not a registry of endpoints — there is nothing to disable yet. Real only if scope adds multi-endpoint subscriptions. |
| 8 | No jitter / no DLQ / no backoff | SPECULATIVE (already exists) | Worker already has jittered exponential backoff and a verbatim-park DLQ. Do not re-scope these. |
| 9 | Receiver verification ergonomics | OUT OF SCOPE (sender-side) | We are the producer; ergonomics work here means emitting a standard scheme + docs, not building a verifier. Note: for Slack/Discord incoming-webhook targets, signing is moot (they authenticate by secret URL) — scheme matters for custom-receiver targets. |
| 10 | Send-side idempotency keys on dispatch API | SPECULATIVE | Only needed if the dispatch API grows client retries; §3b mechanism, not required for webhook hardening itself. |

## 6. Recommendation for the scope decision

Scope v1.4 webhook work as **signed, deduplicable, longer-horizon delivery
with replay** — four items, in this order:

1. **Standard Webhooks–compatible signing** (highest value). Emit `webhook-id`
   (stable per message — reuse the job ID), `webhook-timestamp`, and
   `webhook-signature` (`v1,HMAC-SHA256` over `id.timestamp.body`), with a
   `whsec_`-prefixed per-deployment secret in config and dual-signature
   rotation. This single change closes gaps #1 and #2 together and lets any
   Standard/Svix receiver library verify us unmodified — the cheapest
   interoperability in the survey.
2. **Lengthen the retry horizon** (gap #3) before adding cleverness: raise the
   default `max_retry` / cap so coverage spans hours, keep the existing
   jittered exponential shape (it already matches prior art), and add
   `Retry-After` + 410-disable handling (gap #4). Do not rebuild backoff or
   DLQ parking — they exist.
3. **Minimal replay path** (gap #5): a documented/CLI republish of verbatim
   DLQ bytes. Stripe/Svix both treat parking-without-replay as incomplete;
   our park format is already replay-ready, so this is small.
4. **Explicitly defer**: endpoint registry + auto-disable (#7), dispatch-API
   idempotency keys (#10), timeout re-tuning (#6, fold into #2's config work
   if touched). Do not scope jitter/backoff/DLQ work (#8) — verifying they
   exist is this ticket's answer, not new work.
