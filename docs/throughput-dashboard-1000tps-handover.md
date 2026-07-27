# Handover: Throughput Dashboard 1000 TPS / FuelKeeper Balance

> **STATUS UPDATE (2026-07-24, later session): §3's lockup is FIXED and the
> architecture changed substantially.** The serial-wallet constraint itself was
> two go-sdk auth bugs — fixed upstream in
> [go-sdk PR #338](https://github.com/bsv-blockchain/go-sdk/pull/338) and
> vendored via `third_party/go-sdk` + a `replace` in `go.mod` (delete both once
> the PR ships in a release). `syncwallet` is now a bounded CONCURRENT gate
> (default 16 in flight, `WALLET_MAX_IN_FLIGHT`), the keeper fair-shares via a
> controller state listener and mints leaves in parallel, stream actions use
> `ReturnTXIDOnly`, `wdk.BeefParty` is thread-safe and growth-capped, SSE
> status application is parallel/batched, and stop/status are bounded/live.
> Measured: ~100/s sustained at the 100 TPS setting; 282/s peak at 1000;
> remaining ceilings are server per-action cost (~280/s on this machine) and
> fuel maturation economics (TSTN blocks ~10 min; wallet needs funding).
> **Read `docs/throughput-bottleneck-report.md` first** — the sections below
> describe the OLD serialized architecture and are kept for history.

**Audience:** another LLM (or engineer) continuing this work<br>
**Repo:** `go-wallet-toolbox`<br>
**Branch context:** `feat/network-ttn-tstn-support` (local TSTN demo stack)<br>
**Date context:** 2026-07-24<br>
**Primary goal:** run createAction load at high TPS (target **1000**) **indefinitely**, with FuelKeeper still minting fuel **during** the stream (not paused), without locking up the UI.

---

## 1. What this system is

Local Docker demo:

| Service | Role | Host port |
|---------|------|-----------|
| `db` | Postgres | `127.0.0.1:5433` |
| `infra` | `cmd/infra_throughput` storage server (throughput UTXO mode) | `127.0.0.1:8101` |
| `dashboard` | `cmd/throughput_dashboard` — UI, stream, FuelKeeper, metrics | `127.0.0.1:8200` |

Compose:

```bash
docker compose \
  -f docker-compose.throughput-dashboard.yaml \
  -f docker-compose.throughput-dashboard.tstn.override.yaml \
  --env-file .env.tstn \
  up -d --build
```

- **Network:** private **TSTN** (testnet-like keys/addresses; private Arcade/ChainTracks via `.env.tstn`).
- **Stream:** rate-limited `createAction` with 0-sat OP_RETURN; `AcceptDelayedBroadcast: true`.
- **FuelKeeper (client-side):** mints reserve chunks then exact-denomination **fuel** basket UTXOs; stream is meant to spend fuel.
- **UI:** start/stop stream, TPS, fuel gauges, runway, and **network accept** stats (separate from createAction OK).

Critical design constraint:

> The dashboard wraps the operator wallet in **`syncwallet.Serial`**: **one mutex for all storage RPCs** (CreateAction, FanOutFuel, ListOutputs, ListActions, Balance, Internalize).<br>
> Reason: BRC-104 AuthFetch is not safe for concurrent session handshakes on one peer.<br>
> File: `cmd/throughput_dashboard/internal/syncwallet/serial.go`

So **stream, FuelKeeper, and metrics sampler all serialize on the same lock**.

---

## 2. Problems already solved (do not re-litigate unless regressing)

### 2.1 Arcade fee floor (was “100% network fail”)

- Wallet had been configured for **1 sat/kb** and **2-sat** fuel on TSTN.
- Arcade (v0.10.x, GoBDK) default **`DefaultMinFeePerKB = 100`**.
- Broadcasts failed with ARC **465** / `insufficient-fee` / `transaction fee is too low`.
- createAction still returned OK (delayed broadcast) → UI looked healthy while almost nothing was accepted on-network.

**Fix in place:**

- `infra-config-docker-throughput-tstn.yaml`: `fee_model` **100 sat/kb**, denomination **derived 20** sats (200 B OP_RETURN shape).
- `config.DemoFeeModel` / `DemoDenomination`: same 100 sat/kb → **20** sats for all demo networks.
- After fee fix, sampled stream txs showed **network accept_rate ≈ 1** when the system was healthy.

### 2.2 SSE status names (empty `utxo_status`)

- Arcade emits `ACCEPTED_BY_NETWORK`, `SEEN_MULTIPLE_NODES` (canonical), not only older doc names.
- Empty `utxo_status` is not fundable; funder only selects `mined` / `unproven` / `sending` (with `spend_policy: any`).
- Fixed mapping in `pkg/storage/internal/actions/process_external_status.go` + arcade status constants.
- MINED frames can carry **BUMP `merklePath`** (see Arcade `docs/sse.md`).

### 2.3 Dynamic FuelKeeper target from UI TPS

- On stream start: `target_pool = tps × 300s × 1.5` via `DemoTargetPoolForTPS`.
- Examples: 100 TPS → 45_000; **1000 TPS → 450_000**.

### 2.4 FuelKeeper mint rate knobs

- Raised `FanoutMaxTxsPerRound` **50 → 300** (30k fuel outputs/round at 100 outs/tx).
- Catch-up: while below low water, rounds run back-to-back (not only every TopUp interval).
- Stream start kicks `RunOnce` after resizing target.

### 2.5 Auto workers formula (partial)

Was: `workers = min(512, tps)` → **1000 TPS ⇒ 512 workers**.

Now:

```text
workers = min(32, max(1, ceil(2 × √tps)))
```

| TPS | Workers |
|-----|---------|
| 10 | 7 |
| 100 | 20 |
| 1000 | **32** |

- Go: `stream.WorkersForTPS` (`AutoWorkersCap=32`, `MaxWorkers=64`)
- UI must stay in sync: `web/app.js` `workersForTPS`
- **32 workers is still not the root lockup** (see §3); serial RPC is.

### 2.6 Network accept metric (UI)

- Stream createActions labeled `throughput-dashboard-stream`.
- Sampler lists recent labeled actions; tick field `tick.network` with accept_rate, sending, failed, etc.
- UI distinguishes **createAction OK** vs **network accept**.

---

## 3. The open problem (handover focus)

### 3.1 Symptom

- **100 TPS / 20 workers:** createAction works (~hundreds of successes); app usable.
- **1000 TPS / 32 workers:** app **appears locked**:
  - UI/status ticks freeze (timestamp stops updating).
  - Network health sample: `context deadline exceeded` on ListActions.
  - `POST /api/stream/stop` **hangs** (waits for in-flight createActions).
  - `POST /api/stream/start` returns **`stream already running`** while status tick still shows old `running: false` (stale LastTick).
  - Infra may show little/no new `stream #N` completions while keeper is minting.

### 3.2 Root cause (confirmed)

Not “32 workers is magically too many vs 20” as the sole issue. The lethal combo is:

1. **Stream start at 1000 TPS** sets FuelKeeper `target_pool = 450_000`.
2. Inventory ~25k ≪ low water (270k) → keeper starts a **huge catch-up**: up to **300 leaf FanOutFuel** (+ chunk fan-outs) in a tight loop.
3. All of that holds **`syncwallet` mutex** for long stretches.
4. **Go’s `sync.Mutex` is unfair**: after each `FanOutFuel` unlock, the same keeper goroutine often re-locks immediately → **starves**:
   - stream `CreateAction` workers
   - metrics sampler (`ListOutputs` / `ListActions`)
   - funding / status-adjacent RPCs
5. UI freezes because **status/events feed `Sampler.LastTick()`**, which only updates when `sample()` completes — and sample needs the same mutex.
6. **Stop** cancels production then `<-done` waits for all workers to finish current `CreateAction`; those are stuck behind the keeper → **stop deadlocks**.

So: **FuelKeeper catch-up monopolizes the serial wallet during the stream**, which is the opposite of “run indefinitely with balanced mint + spend.”

### 3.3 What the user does **not** want

- **Do not pause FuelKeeper entirely while the stream runs.**<br>
  Minting must continue so the pool can track burn rate for indefinite load.

### 3.4 What “success” looks like

- Stream createAction and FuelKeeper mint **interleave** on the serial wallet so neither starves for long.
- UI stays responsive (status ticks update; stop works in seconds).
- At steady state, fuel mint rate ≈ stream fuel consume rate (within serial RPC capacity).
- Network accept remains healthy (fees already aligned).
- Honest about **serial ceiling**: true 1000 createAction/s may be impossible with one serialized AuthFetch client if each RPC is tens of ms; balancing should maximize sustainable rate and avoid lockup even if createAction/s is below 1000.

---

## 4. Architecture details the next agent needs

### 4.1 Data flow

```text
UI POST /api/stream/start {tps}
  → stream.Controller.Start (workers = WorkersForTPS(tps))
  → Fuel.SetTargetPoolSize(DemoTargetPoolForTPS(tps))  // 1000 → 450_000
  → go Fuel.RunOnce()   // currently kicks catch-up immediately
  → Sampler.SetTargetPool(...)

FuelKeeper.Run loop:
  while below low water: runOnce (up to FanoutMaxTxsPerRound leaves)
  → wallet.FanOutFuel (serial mutex)
  → back-to-back catch-up (10ms yield) when still low

Stream workers:
  rate.Limiter(tps) → jobs chan → CreateAction (serial mutex)

Sampler (1s):
  Balance, ListOutputs×2, ListActions (network health) — all serial mutex
```

### 4.2 Key files

| Path | Role |
|------|------|
| `cmd/throughput_dashboard/main.go` | Wires wallet→syncwallet, keeper, stream, API |
| `cmd/throughput_dashboard/internal/syncwallet/serial.go` | **Global RPC mutex** |
| `cmd/throughput_dashboard/internal/stream/controller.go` | Rate limit, workers, createAction |
| `pkg/wallet/fuelkeeper/keeper.go` | Mint loop, catch-up, caps |
| `cmd/throughput_dashboard/internal/api/server.go` | start/stop/status; fuel target resize + RunOnce kick |
| `cmd/throughput_dashboard/internal/config/config.go` | DemoThroughput, DemoTargetPoolForTPS, fees |
| `cmd/throughput_dashboard/internal/metrics/sampler.go` | Ticks + network health |
| `infra-config-docker-throughput-tstn.yaml` | Server fee 100, throughput profile |
| `docs/throughput-dashboard.md` | Runbook |

### 4.3 Important numbers (current)

| Knob | Value |
|------|--------|
| Fee | 100 sat/kb |
| Fuel denomination | 20 sats |
| Fanout outputs/tx | 100 |
| Fanout max txs/round | 300 → up to **30_000** fuel outs/round |
| TopUp interval (idle) | 2s (catch-up ignores this when below low water) |
| Target pool @ 1000 TPS | **450_000** |
| Auto workers @ 1000 TPS | **32** |
| Stream label | `throughput-dashboard-stream` |

---

## 5. Recommended direction (not yet implemented)

User rejected “pause keeper while stream runs.” Need **shared, fair access** to the serial RPC path.

### 5.1 Preferred approach: cooperative time-slicing / fair interleaving

While stream is active (`SetStreamActive(true)` on start, `false` on stop):

1. **Do not** run 300 leaves in a tight loop with zero yield for stream waiters.
2. After **each** keeper wallet RPC (`FanOutFuel` leaf and chunk), **yield wall-clock** so stream workers can acquire the mutex:
   - e.g. sleep so stream gets a target share of time (e.g. 60–80% stream / 20–40% mint), **or**
   - quota: after 1 keeper op, wait until N stream CreateActions complete or timeout.
3. Optionally **cap leaves per round while stream active** (e.g. 1–10) but **keep rounds continuous** so mint never fully stops—steady drip matching burn.
4. When stream **stops**, restore full catch-up (300 leaves, back-to-back) to refill toward high water.

### 5.2 Status / stop robustness (should ship with balance work)

- **`GET /api/status`**: always overlay **live** `Ctrl.Stats()` onto the response (do not rely only on frozen `LastTick.stream`).
- **`Stop`**: must not hang forever if workers are blocked; timeout + clear generation, and/or cancel in-flight work carefully.
- Sampler timeouts already exist; still fails when mutex held for >15s continuously—interleaving fixes the root.

### 5.3 Optional harder paths (only if slicing is not enough)

- Real concurrent storage clients with fixed AuthFetch (high risk; original deadlock reason).
- Separate processes: loadgen stream vs keeper process (two AuthFetch identities).
- Server-side mint (not current architecture; keys stay client-side).

### 5.4 What not to do

- Do not “fix” high TPS only by raising workers again (512 was harmful; 32 is not the main bug).
- Do not pause keeper for the whole stream as the permanent solution (user requirement).
- Do not lower fees back to 1 sat/kb on this Arcade host without changing Arcade policy.

---

## 6. How to reproduce the lockup

1. Stack up with TSTN compose + `.env.tstn`.
2. Fund operator; wait until some fuel exists.
3. Start stream at **1000** TPS (workers auto → 32).
4. Observe:
   - Log: `targetPoolSize=450000`, `leafTxs=300`, minting starts.
   - Shortly: UI freezes / status tick timestamp stops.
   - `curl -m 10 -X POST .../api/stream/stop` → **timeout**.
   - `curl -X POST .../api/stream/start` → **`stream already running`**.
   - Live `Ctrl` is running; **LastTick is stale**.

Recovery today: recreate/restart **dashboard** container (or kill process); stop may never return.

---

## 7. Suggested implementation checklist for the next agent

1. Add `FuelKeeper.SetStreamActive(bool)` (atomic).
2. In mint loops (leaves + chunks), when stream active: **fair yield** after each wallet RPC (time-slice or quota).
3. When stream active: lower per-round leaf cap but keep minting every loop iteration (indefinite refill).
4. Wire API start/stop to SetStreamActive; reconsider or soften immediate `RunOnce` kick under stream (must not monopolize).
5. Status: return live stream stats independently of LastTick.
6. Stop: bounded wait; document force-restart if needed.
7. Tests: stream-active yield / leaf cap; status live stats; stop does not hang when mint is slow (fake wallet with delays).
8. Rebuild dashboard; retest 100 then 1000 TPS for UI liveness + stop + fuel not monotonically draining to zero.

---

## 8. One-paragraph summary for the next model

The TSTN throughput dashboard can create and (after fee alignment) broadcast stream txs, but at 1000 TPS the client **serializes all wallet RPCs**. Starting 1000 TPS resizes the fuel target to **450k** and FuelKeeper begins a **massive back-to-back mint**; unfair mutex re-acquisition starves stream CreateAction, metrics, and Stop—so the app looks locked. The user wants **FuelKeeper to keep minting during the stream**, not pause; implement **fair interleaving / time-slicing** (and live status/stop hardening) so mint and stream share the serial wallet and can run indefinitely within the serial throughput ceiling.
