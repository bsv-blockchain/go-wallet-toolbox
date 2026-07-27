# Throughput Dashboard — Where the Bottleneck Actually Lies

**Date:** 2026-07-24 · **Branch:** `feat/network-ttn-tstn-support` · **Stack:** local Docker TSTN (Postgres + `infra_throughput` + dashboard), private Arcade

**Goal:** 1000 createAction/s sustained in the demo dashboard, UI responsive.
**Where we landed today:** ~100/s sustained cleanly, **282/s peak** at the 1000 TPS setting while mature (mined) fuel lasted, no lockups, UI live at every load, stop bounded. The remaining gap to 1000 is *not* one bug — it is a stack of four ceilings, quantified below.

---

## 1. Executive summary

The original 1000 TPS lockup was client-side and is fixed. Peeling it away exposed each next layer:

| # | Ceiling | Measured | Status |
|---|---------|----------|--------|
| 1 | One serialized wallet RPC at a time (go-sdk AuthFetch bugs) | ~12 actions/s | **fixed** (go-sdk PR [#338](https://github.com/bsv-blockchain/go-sdk/pull/338), vendored in `third_party/go-sdk`) |
| 2 | Client-side shared BEEF graph (`BeefParty`) — unbounded growth, one mutex, merkle hashing per action | 81→13/s decay, then total stall | **fixed** (locking, graph cap, `ReturnTXIDOnly` on stream) |
| 3 | Server-side per-action cost (funding tx + input-BEEF assembly, 2 RPCs/action) | **~280/s** with mined fuel; ~10–30/s when fuel ancestry is unproven/deep | **the current wall** |
| 4 | Fuel maturation economics (TSTN block cadence ~10 min + funding) | needs `TPS × 600s` mined fuel standing + 30 sats/action burn | **capital, not code** |

**UI liveness at 1000 TPS setting:** `/api/status` stayed 3–36 ms during every run (live controller overlay), start/stop never hung (bounded stop returns 202 + `draining` when in-flight actions outlive the wait window).

---

## 2. What was measured

### RPC-layer capacity (ListOutputs probe, local infra)

| Configuration | Aggregate RPC/s |
|---|---|
| 1 client, serial (pre-fix behavior) | 256 |
| 1 client, 8 concurrent (patched SDK) | **1128** (0 failures) |
| 1 client, 16 concurrent | 941 (0 failures) |
| 8 distinct-identity clients | 956 |
| 32 concurrent connections | connection EOFs — infra HTTP limit |

### End-to-end createAction (0-sat OP_RETURN, `AcceptDelayedBroadcast`, `ReturnTXIDOnly`)

| Scenario | Sustained rate |
|---|---|
| Before any fixes (serial mutex) | 10–20/s, decaying; 1000 TPS setting → full UI lockup, stop hang |
| After concurrency fixes, 100 TPS setting | **~100/s** (on-target, 0 failures, 45 s) |
| 1000 TPS setting, spending **mined** fuel | **268–282/s** until mined stock ran out |
| 1000 TPS setting, spending fresh **unproven** fuel | 10–30/s, infra at ~600% CPU |

The mined-vs-unproven split is the signature of ceiling #3: for every createAction the server assembles the input BEEF for the fuel being spent. A mined fuel UTXO terminates at its leaf fan-out tx + merkle path (~4 KB, cheap). An unproven fuel UTXO drags its whole unproven ancestry chain (leaf → chunk → the session's change chain), and both sides then hash/merge it per action.

---

## 3. Root causes found and fixed

1. **go-sdk `AuthFetch` response listener** deregistered itself *before* the nonce check; the first response killed every other in-flight request's listener → 30 s hangs → the global serialization mutex existed at all. Fixed + regression tests; PR upstream, vendored locally via `replace` in `go.mod`.
2. **go-sdk `DefaultSessionManager.UpdateSession`** was remove-then-add — a removal window that made concurrent requests fail `session-not-found` server-side ("missing version header in response" client-side). Fixed (idempotent re-add). Also fixed data races on `Peer.lastInteractedWithPeer` and `PeerSession.LastUpdate`.
3. **`wdk.BeefParty`** — embedded `transaction.Beef` (map-backed, no locking) mutated by every concurrent action → `fatal error: concurrent map iteration and map write`. All graph access now goes through the party lock (`WithLock` for verify+serialize sequences).
4. **O(n²) client work per run** — every action merged its full returned BEEF into the shared party and advertised the entire known-txid list on the next request. Fixed twice over: graph capped at 256 txs (reset before merge), and the stream now sends `ReturnTXIDOnly` so the response BEEF (and the merge, and the mutex) vanish from the hot path entirely.
5. **FuelKeeper monopolizing the wallet** — 300-leaf catch-up rounds back-to-back starved stream/sampler/stop (the original handover issue). Now: fair-share mode driven by the stream controller's state-transition listener (race-free vs. HTTP handler racing), leaf cap + proportional yields while streaming, parallel leaf minting (`MintConcurrency=6`) so refill can track burn, full-rate catch-up when idle, and interval (not 10 ms) retry after failed/zero-progress rounds.
6. **Fuel denomination 20 sats vs. real ~21 sat fee** → funder claimed 2 UTXOs per action (`multi_claim`): double burn, double contention. Denomination is now explicit 30 sats (single claim) in both server yaml and dashboard config.
7. **MINED/status application was serial** — one SSE event at a time plus a cursor write per event (~7/s; an 18k backlog = 45 min of maturation lag). Now applied in batches through an 8-worker pool with one cursor write per batch and deadlock retry (~2.5×+ measured; per-event DB cost is the next limit).
8. **Ops/UI hardening** — ctx-aware bounded wallet gate (FIFO, cancelable waits — sampler ticks can no longer freeze for minutes), live stats overlay on `/api/status`, bounded stop with `draining` state end-to-end, Postgres demo-mode durability relaxation, infra log level warn.

---

## 4. The remaining wall, quantified

**Ceiling #3 — server per-action cost.** With mature fuel and 16–32 concurrent RPCs the stack sustains ~280 actions/s (2 storage RPCs each: `createAction` funding + `processAction`), with infra+db consuming ~5–7 cores. That is ~15–20 ms of CPU per action across the pipeline. Reaching 1000/s needs roughly 4× — from cheaper per-action work (single round trip, leaner funding claim, no per-action BEEF work server-side), more cores, or horizontal infra instances.

**Ceiling #4 — maturation economics.** This TSTN mines roughly every ~10 minutes. Fuel only becomes cheap to spend once MINED, so sustained X TPS needs a standing **mined** pool ≥ X × 600 s:

| Sustained target | Standing mined fuel | Standing value @30 sats | Burn rate |
|---|---|---|---|
| 100 TPS | 60,000 UTXOs | 1.8M sats | 3,000 sats/s (~10.8M/h) |
| 1000 TPS | 600,000 UTXOs | 18M sats | 30,000 sats/s (~108M sats ≈ 1.08 tBSV per hour) |

The wallet currently holds ~190k sats total (≈ 6,300 actions of runway). **Any sustained demo above ~1 minute needs a deposit**; a 10-minute 1000 TPS demo needs ~36M sats (~0.36 tBSV) including the standing pool. Alternatively, ask the TSTN operator for faster blocks — pool size scales linearly with block interval.

**Pipeline corollary:** at 1000 TPS the status pipeline must also apply ~1000 MINED events/s (each currently a DB transaction; fan-out txs update 100 output rows). Batching status updates per block is the structural fix.

---

## 5. Ranked path to 1000 TPS sustained

1. **Fund the wallet** (blocker for anything sustained; see table above) and pre-fill the pool at idle full-rate mint, then let it mature ≥ 1 block before streaming.
2. **Shrink block interval** on the private TSTN (or accept `SpendPolicyMinedOnly` + huge pool). Pool capital scales 1:1 with block time.
3. **Cut server cost per action**: skip input-BEEF assembly for exact-fee fuel spends (the client signs against known 30-sat P2PKH leaves), or add a "fuel spend" fast path; consider merging create+process into one RPC for this shape.
4. **Batch MINED application per block** instead of per tx.
5. **Horizontal scale**: N infra instances / DB partitions, or multiple dashboards with distinct identities (safe post-SDK-fix; verified by probe).
6. **Raise infra HTTP concurrency** past ~32 in-flight (connection EOFs observed via host proxy; likely tunable server-side).

## 6. Runbook deltas

- Rebuild: `docker compose -f docker-compose.throughput-dashboard.yaml -f docker-compose.throughput-dashboard.tstn.override.yaml --env-file .env.tstn up -d --build db infra dashboard`
- New env knob: `WALLET_MAX_IN_FLIGHT` (default 16) — concurrent storage RPC bound.
- `third_party/go-sdk` + `replace` in `go.mod` carry the auth fixes until go-sdk ships PR #338; then delete the directory and the replace line, and bump the dependency.
- Postgres now runs with `fsync=off` etc. in this compose file — demo only, never production.
