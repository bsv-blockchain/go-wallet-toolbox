# Arcade-First Broadcast with Circuit-Breaker Failover

**Date:** 2026-06-10
**Status:** Approved
**Goal:** Broadcast stability. Default broadcast path is Arcade only; all other postBeef
services are demoted to explicit failover or verification roles.

## Problem

Today `services.PostFromBEEF` broadcasts every transaction to all enabled services in
parallel (ARC/TAAL + WhatsOnChain by default). These services are not mutually
idempotent — the response from one can be affected by the same tx arriving at another,
which produces false verdicts (see false double-spend fix, commit 6addd9e) and unstable
broadcast behavior.

## Decision Summary

| Decision | Choice |
|---|---|
| Client shape | New dedicated `pkg/services/internal/arcade` package; existing ARC client untouched (kept for TAAL/GorillaPool failover) |
| Failover trigger | Circuit breaker (N consecutive transport failures opens; `/health` probe; half-open trial; auto-close) |
| Callback token | Derived from wallet identity key: `hex(HMAC-SHA256(identityPubKey, "go-wallet-toolbox/arcade-sse/v1"))`; config override |
| SSE consumer | Long-lived goroutine in Monitor Daemon; `Last-Event-ID` persisted in `KeyValue` table |

## Arcade API (verified live against https://arcade-v2-us-1.bsvblockchain.tech)

Arcade is **not** classic-ARC path compatible (`/v1/tx` → 404). Live API:

- `POST /tx` — body: binary EF, `Content-Type: application/octet-stream` (also accepts
  hex `text/plain`, JSON `{"rawTx": hex}`). Headers: `X-CallbackToken` (recommended,
  scopes webhooks + SSE), `X-CallbackUrl` (optional), `X-FullStatusUpdates` (optional).
  Returns `202` with status JSON; idempotent resubmits return existing state.
  `400` validation failure. `503` + `Retry-After` under backpressure.
- `POST /txs` — batch, concatenated raw bytes, octet-stream only (256 MiB cap).
- `GET /tx/:txid` — `txid, txStatus, timestamp, blockHash, blockHeight, merklePath,
  extraInfo, competingTxs`. `404` if never submitted to this instance.
- `GET /events?callbackToken=<token>` — SSE. `id:` is nanosecond timestamp,
  `event: status`, JSON data `{txid, txStatus, timestamp}`. `Last-Event-ID` header on
  reconnect replays missed events. Keepalive comment every 15s.
- Status values: `RECEIVED, SEEN_ON_NETWORK, SEEN_ON_MULTIPLE_NODES, MINED, IMMUTABLE,
  REJECTED`.
- `GET /health` — liveness + datahub health JSON.

## Components

### 1. `pkg/services/internal/arcade` (new package)

- **Broadcaster**: `POST {URL}/tx`, binary EF bytes, octet-stream. Always sends
  `X-CallbackToken`; `X-FullStatusUpdates: true` by default; `X-CallbackUrl` only if
  configured. `503`+`Retry-After` is backpressure — wait/requeue, never counted as
  circuit-breaker failure. The existing PostEF queue interface passes `efHex` strings;
  the arcade adapter decodes hex → bytes at the boundary.
- **Query**: `GET /tx/{txid}` for monitor verification and SSE gap-fill.
- **SSE client**: reconnecting `text/event-stream` reader. Parses id/event/data frames,
  tolerates keepalive comments, exponential backoff on disconnect, sends `Last-Event-ID`
  on reconnect.
- **Status mapping** to existing `wdk` statuses:
  - `RECEIVED`, `SEEN_ON_NETWORK`, `SEEN_ON_MULTIPLE_NODES` → in-flight progress (unmined)
  - `MINED`, `IMMUTABLE` → mined (merkle path available)
  - `REJECTED` → candidate failure; must pass double-spend verification before terminal

### 2. Config (`pkg/defs`)

New `Arcade` struct: `Enabled` (default true on mainnet), `URL` (default
`https://arcade-v2-us-1.bsvblockchain.tech`), `EventsURL` (default = `URL`; covers
self-hosted split-port deployments), `CallbackToken` (optional override),
`CallbackURL`, `FullStatusUpdates`.

Failover chain defaults: TAAL ARC (existing config) → GorillaPool ARC
(`https://arc.gorillapool.io`) → WhatsOnChain → Bitails.

WhatsOnChain and Bitails are **removed from the default broadcast queue**. Their status,
merkle-path, and UTXO verification roles are unchanged.

### 3. Tiered broadcast router (`pkg/services`)

`PostFromBEEF` routes through a tiered broadcaster replacing broadcast-to-all:

- Happy path: Arcade only. No other service receives the tx.
- Circuit breaker: N consecutive transport failures (timeouts, connection refused, 5xx —
  excluding 503 backpressure) opens the circuit. Background `/health` probe; half-open
  trial request; auto-close on success.
- Circuit open → failover chain tried **sequentially** (OneByOne semantics, never
  parallel — services are not mutually idempotent). First acceptance wins.
- Monitor tasks (SendWaiting, BackgroundBroadcaster) flow through the same router; the
  failover logic exists in exactly one place.

### 4. Callback token derivation

Computed at wiring time in `pkg/infra/server.go` (key source available before
`services.New`): `hex(HMAC-SHA256(identityPubKey, "go-wallet-toolbox/arcade-sse/v1"))`.
Deterministic → stable SSE scope and replay across restarts. Config `CallbackToken`
overrides. Exported helper for library users who construct services manually.

### 5. SSE consumer (Monitor)

Long-lived goroutine started by Monitor `Daemon.Start` (persistent connection, not a
gocron job):

- `MINED`/`IMMUTABLE` → existing `UpdateKnownTxAsMined` path (merkle path from event
  payload or `GET /tx/:txid` fallback).
- `SEEN_*` → status progress update.
- `REJECTED` → routed through `confirmDoubleSpends`-style verification before any
  terminal failure (preserves false-double-spend guarantees; WhatsOnChain verifies,
  never broadcasts).
- `Last-Event-ID` persisted to `KeyValue` table, key `arcade_sse_last_event_id`; on
  restart the replay fills the gap.
- Existing polling tasks (CheckForProofs, SendWaiting, UnFail, FailAbandoned) remain
  unchanged as the safety net. SSE is the primary signal; polling is backup.

## Error Handling

- Arcade 400 → real validation failure, surfaced as broadcast error (no failover — the
  tx is bad, not the service).
- Arcade 503 + Retry-After → backpressure: honor delay, retry Arcade; not a failure.
- Transport errors / 5xx → circuit-breaker failure count; failover only when circuit
  open.
- SSE disconnect → reconnect with backoff + replay; never affects broadcast circuit.
- REJECTED via SSE → verified against network (rawTx presence) before terminal fail.

## Testing

- `httptest` fake Arcade: `POST /tx` (binary, content-type assertion, 202/400/503
  scenarios), `GET /tx/:txid`, SSE stream with keepalives, drops, and Last-Event-ID
  replay.
- Circuit breaker unit tests: open threshold, half-open trial, close on recovery, 503
  exclusion.
- Status mapping table tests.
- Existing provider broadcast tests updated for single-service happy path.
- False double-spend regression tests (`provider_broadcast_double_spend_test.go`) must
  stay green.

## Rejected Alternatives

- **Per-broadcast fallback** (try Arcade, on any error immediately try next): transient
  blips would spray txs to non-mutually-idempotent services — the instability this
  design removes.
- **Generalizing the existing ARC client** to speak both dialects: risks the working
  TAAL failover path; Arcade's API differs in paths, payloads, and status vocabulary.
- **Broadcast-to-all (status quo)**: causes cross-service verdict interference.
