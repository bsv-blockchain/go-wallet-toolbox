# High-throughput UTXO management — configuration & design proposal

**Status:** Draft for discussion
**Baseline:** [PR #935 — DR v1 Track P](https://github.com/bsv-blockchain/go-wallet-toolbox/pull/935) (bounded target-aware funder). That branch merges before this work goes ahead; everything below assumes its funder, indexes, and contention-retry machinery as the starting point.
**Scope:** UTXO management configuration options for operators with sustained, high-volume, uniform workloads (payment rails, data anchoring) where **throughput and operational simplicity outrank privacy**.

---

## 1. Motivation & target workload

Some operators run this software as infrastructure for a single application that emits a
firehose of near-identical transactions — the motivating example:

- `createAction` called at up to **100,000 TPS**, indefinitely.
- Every transaction has a **known, fixed shape** (e.g. ~240 bytes ⇒ 24 sat fee at the
  default 100 sat/kb fee model).
- Privacy is a non-goal: all funds belong to one operator; linkability of the tx graph
  is acceptable.
- The wallet must **never stall waiting for funds**: a pre-fanned-out pool of
  correctly-sized UTXOs (ideally matured to block inclusion, ~5 min average) must always
  be available.
- The operator wants **sensible top-up automation** and **out-of-band alerting**
  (e.g. SMS via Twilio) when funds run low.

**Primary profile — repo market settlement.** The concrete deployment is a
repurchase-agreement market: on average **~10 createActions/s**, with two sharp
diurnal peaks around traditional market open and close reaching **~160,000
outputs/s**. Peaks arrive as many-output actions (e.g. ~160 actions × 1,000
outputs), and the typical action defines ~15 outputs — so the denomination is
sized to the **typical action** (e.g. 240-sat fuel), and *output* throughput, not
*action* throughput, is the scaling axis. The 100k-TPS single-output figures used
elsewhere in this doc remain the stress envelope the design must not preclude.

The current model is privacy-oriented: randomized change values (when change exceeds
`count × minimum value`), output shuffling, per-request change-count heuristics, and a
funder that treats every request as bespoke. That model
is correct for consumer wallets but leaves both **wasted work** and **missing
operational machinery** for the workload above.

### 1.1 Relationship to PR #935 (DR v1 Track P)

Track P already removed the biggest algorithmic ceiling: the funder used to load — and
FOR-UPDATE-lock — the entire eligible pool per `createAction`; it now issues bounded,
target-aware micro-queries (exact/smallest-sufficient via `ORDER BY satoshis ASC LIMIT
1`, largest-insufficient via `DESC LIMIT 16`, walked tier by tier), with cost **flat in
pool size** (~0.16 ms/op at 10k-row pools in `BenchmarkSQLFund`). It also added:

- composite index `idx_user_utxos_selection (user_id, basket_name, reserved_by_id,
  utxo_status, satoshis)` — which is precisely the claim-path index a denominated pool
  needs;
- CAS-guarded reservation (`WHERE reserved_by_id IS NULL` + RowsAffected) and the
  public `wdk.ErrUTXOContention` sentinel with bounded, jittered retry around the
  funding transaction;
- SQLite WAL defaults and sweep-paging dedup.

This proposal is therefore **not** a new funding engine. It is: (a) a *pool shape* that
lets the Track P funder hit its best case ~100% of the time, (b) the *replenishment
automation* that keeps that pool full at a rated throughput, and (c) the *policy,
observability, and alerting knobs* around it. A uniform, exact-denomination pool turns
every fund into: one `FindSmallestSufficientUTXOForUpdate` micro-query → index-served
exact match → single row locked → done. No stage-3 fallback, no multi-round
accumulation, no cross-request lock overlap beyond `SKIP LOCKED` skips.

---

## 2. Current state — the knobs we have today

### 2.1 Configuration surface

| Knob | Default | Where | Effect |
|------|---------|-------|--------|
| `change_basket.number_of_desired_utxos` | 32 | `pkg/defs/change_basket.go` | Desired pool size; funder mints extra change outputs until the basket holds this many |
| `change_basket.minimum_desired_utxo_value` | 1000 | `pkg/defs/change_basket.go` | Baseline value for change outputs; also drives `minimumChange = value/4` |
| `change_basket.max_change_outputs_per_tx` | 8 | `pkg/defs/change_basket.go` | Caps change outputs per tx (runtime-tunable via `Provider.SetMaxChangeOutputsPerTx`) |
| `fee_model` | 100 sat/kb | `pkg/defs/fee_model.go` | Fee rate used by the funder's fee calculator |
| `monitor.tasks.check_for_proofs.interval_seconds` | 60 | `pkg/defs/monitor.go` | How fast UTXOs transition to `mined` (maturation latency floor) |
| `monitor.tasks.send_waiting / fail_abandoned / un_fail` | 300/300/600 | `pkg/defs/monitor.go` | Broadcast fallback & failure hygiene |

`number_of_desired_utxos` and `minimum_desired_utxo_value` are materialized per-user
into `output_baskets` rows (`wdk.BasketConfiguration`) at user creation;
`max_change_outputs_per_tx` is process-wide funder state, not per-user.

### 2.2 Runtime behavior (per `createAction`, post-#935)

From `pkg/storage/internal/actions/create.go` and `pkg/internal/storage/funder/` on the
Track P branch:

1. `CountUTXOs` — a `COUNT(*)` over the user's not-reserved UTXOs (outside the DB tx),
   feeding the `numberOfDesiredUTXOs − existing` change-minting heuristic.
2. Bounded tiered allocation (`allocateBounded`): per allocation round, per status tier
   (`mined → unproven → sending` when included), a smallest-sufficient micro-query, then
   a largest-insufficient `DESC LIMIT 16` batch — only rows actually considered get
   locked (`FOR UPDATE SKIP LOCKED`).
3. Change count computed from `minimum_desired_utxo_value`, capped by
   `max_change_outputs_per_tx` and remaining `number_of_desired_utxos`.
4. Change values **randomized** (`txutils.ChangeDistribution` noise), outputs
   optionally shuffled, fresh random derivation suffix per output.
5. On reservation contention: `wdk.ErrUTXOContention` → up to 2 jittered retries of the
   whole funding transaction.

Pool replenishment happens **only** as a side effect of step 3 — there is no proactive
fan-out, no reserve concept, no pool-level monitoring, and no alerting.

---

## 3. Gap analysis — what still breaks at high sustained TPS

1. ~~O(pool) funding~~ — **resolved by #935** for selection. What remains on the hot
   path is `CountUTXOs` (item 2) and privacy work (item 7).
2. **`COUNT(*)` per call.** Step 1 above is an index scan whose cost grows with pool
   size — at the pool sizes this workload needs (§6: tens of millions of rows) it
   becomes the new dominant per-request cost. It only exists to drive change-minting
   heuristics that throughput mode doesn't use.
3. **Replenishment is reactive and capped.** With `max_change_outputs_per_tx = 8`, pool
   growth is coupled to spend traffic and can never outrun a drain of 1 UTXO per tx +
   fees. A drained pool has no recovery path other than manual intervention.
4. **Variable-value UTXOs push the funder off its fast path.** Randomized change values
   mean the smallest-sufficient probe often over-allocates (minting change and growing
   the tx), or falls to the stage-3 insufficient batch and multi-round accumulation.
   A uniform pool makes round one an exact match by construction.
5. **No confirmed-only / maturation policy.** The tier ordering *prefers* mined UTXOs
   but silently falls back to `unproven`; `sending` inclusion is decided by
   `IsDelayed`, not by the operator. An operator who wants "only block-included
   inputs" (or explicitly: "zero-conf is fine, never wait") has no knob.
6. **No observability or alerting.** Pool depth, maturation pipeline depth, reserve
   balance, and fee-burn runway are not tracked; the first symptom of trouble is
   `ErrNotEnoughFunds` at the API surface.
7. **Wasted privacy work.** Randomized distribution, shuffling, and change-count
   heuristics burn CPU per request for a workload that explicitly doesn't want them.

---

## 4. Proposed configuration surface

A new top-level `utxo_management` section in the infra config selects a **strategy**.
`privacy` is today's behavior and stays the default; `throughput` activates the
denominated-pool machinery described in §5.

```yaml
utxo_management:
  # privacy  – current behavior: randomized change, best-fit funding (default)
  # throughput – denominated pool, exact-match claims, proactive fan-out top-up
  strategy: throughput

  throughput:
    # --- Denomination sizing -------------------------------------------------
    # The operator's known transaction shape. Used to derive the denomination:
    #   denomination = ceil(expected_tx_size_bytes / 1000 * fee_model.value)
    #                  + expected_output_satoshis
    #                  + commission.satoshis        (when commission is enabled)
    # When commission.satoshis > 0, the commission output's bytes are also added
    # to expected_tx_size_bytes automatically, so a fuel UTXO still covers the
    # whole per-tx spend and change stays zero by construction.
    # MUST be the final signed size INCLUDING the fuel input the funder adds
    # (148 bytes for one P2PKH input) — payload-only sizes derive a denomination
    # below the real fee and push every request off the exact-match fast path.
    expected_tx_size_bytes: 240
    # Satoshis carried by the app's own outputs per tx (0 for pure-fee/data txs).
    expected_output_satoshis: 0
    # Explicit override; when > 0 it wins over the derivation above. (=> 24 here)
    denomination_satoshis: 0

    # --- Pool sizing ----------------------------------------------------------
    # Sustained createAction rate the pool must absorb.
    target_tps: 100000
    # Average block-inclusion wait for minted pool UTXOs.
    expected_confirmation_seconds: 300
    # Safety multiplier over (target_tps × expected_confirmation_seconds)
    # to absorb block-time variance (blocks are Poisson; 30–60 min gaps happen).
    pool_headroom_factor: 1.5
    # Explicit override for the total pool target; when > 0 it wins.
    # (derived here: 100000 × 300 × 1.5 = 45,000,000)
    target_pool_size: 0
    # Top-up trigger / fill target as fractions of target_pool_size:
    # top-up starts when the pool drops below low_water and refills toward
    # high_water (deficit = high_water_percent/100 × target_pool_size − inventory).
    low_water_percent: 60
    high_water_percent: 100

    # --- Selection policy -------------------------------------------------
    # mined_only        – only block-included UTXOs are spendable fuel
    # prefer_mined      – mined first, fall back to unproven (default; matches
    #                     today's tiering but without the 'sending' tail)
    # any               – include 'sending' (today's includeSending semantics)
    spend_policy: prefer_mined

    # --- Baskets ------------------------------------------------------------
    # Basket holding the denominated fuel UTXOs (claimed by the funder).
    pool_basket: fuel
    # Basket the operator deposits large funding UTXOs into (via internalizeAction).
    # The top-up task draws exclusively from here.
    reserve_basket: reserve

    # --- Fan-out shape --------------------------------------------------------
    # Outputs per fan-out transaction (the "100 outputs" in the target shape).
    fanout_outputs_per_tx: 100
    # Max LEAF fan-out transactions (the ones minting fuel outputs) built per
    # top-up round; interior chunk-split txs are not counted against this cap.
    # Must satisfy the sustained-throughput identity (validated at startup):
    #   fanout_outputs_per_tx × fanout_max_txs_per_round
    #     ≥ target_tps × top_up.interval_seconds × recovery_margin (≥ 1.2)
    # (here: 100 × 12000 = 1.2M ≥ 100000 × 10 × 1.2)
    fanout_max_txs_per_round: 12000
    # Depth of the intermediate split tree (see §5.2). 1 = reserve → fuel directly;
    # 2 = reserve → chunks → fuel (needed at very high mint rates).
    fanout_tree_depth: 2
    # Stale-denomination recoup (§10 Q2): max stale fuel inputs consumed per
    # consolidation transaction when the denomination changes.
    consolidation_inputs_per_tx: 1000

    # --- Top-up task ----------------------------------------------------------
    # Authoritative home for this task's schedule: wired into the monitor's task
    # registry at startup (there is no monitor.tasks.pool_top_up key).
    top_up:
      enabled: true
      interval_seconds: 10
      start_immediately: true

  # --- Alerting (strategy-independent, but designed for this workload) --------
  alerting:
    enabled: true
    # Conditions are a closed, built-in set evaluated by the pool accounting
    # loop (§5.3); each has a fixed comparison direction (< for runways,
    # ≥ for failure/error counters) and a default severity.
    rules:
      # runway = spendable_pool / target_tps  (seconds until exhaustion at rated load)
      - name: pool_runway_low
        condition: pool_runway_seconds
        threshold: 900          # alert when < 15 min of runway
      - name: reserve_runway_low
        condition: reserve_runway_seconds
        threshold: 86400        # alert when reserve funds < 1 day of fee burn
      - name: top_up_failing
        condition: consecutive_top_up_failures
        threshold: 3
      - name: not_enough_funds
        condition: create_action_funding_errors_per_minute
        threshold: 1
    # Alerts fan out to every configured notifier. Repeated alerts are
    # rate-limited per rule via cooldown.
    cooldown_seconds: 300
    notifiers:
      - type: webhook
        url: https://ops.example.com/hooks/wallet
      - type: twilio
        # Secrets are supplied via the loader's viper AutomaticEnv override
        # (as with db passwords), NOT written into the YAML. NOTE: env override
        # of keys inside a list needs the notifier config restructured into a
        # map (e.g. notifiers.twilio.*) or interpolation support added to
        # internal/config/loader.go — an implementation decision for Phase 3.
        account_sid: <set-via-env-override>
        auth_token: <set-via-env-override>
        from: "+15550001111"
        to: ["+15552223333"]
```

### Derivation & validation rules

- **Denomination floor.** The generic funder's dust floor
  (`funder.newCollector`: `max(1, ceil(192/1000 × feeRate) × 2)` — **40 sat at
  100 sat/kb**) deliberately does **not** apply to fuel: that floor prices a
  *future* spend of a change output, whereas a fuel UTXO's spend fee is priced into
  the denomination by construction (the denomination *is* the fee of the tx it
  funds). The throughput-mode floor is instead: `denomination` > the marginal fee of
  including one fuel input (`ceil(148/1000 × feeRate)` ≈ 15 sat at 100 sat/kb), so a
  fuel UTXO always contributes more than it costs to spend. Reject config below
  that; the 24-sat worked example passes (24 > 15) though it sits below the generic
  40-sat dust floor — which is exactly why the exemption must be explicit.
  **Decided:** the throughput-mode dust exemption is blessed by the operator and is
  no longer an open question.
- **Sustained-throughput identity** (reject config that cannot keep up):
  `fanout_outputs_per_tx × fanout_max_txs_per_round ≥ target_tps ×
  top_up.interval_seconds × recovery_margin`, margin ≥ 1.2 so the pool can climb
  back from low water while absorbing rated load.
- Warn (don't fail) when `strategy: throughput` and the legacy
  `change_basket.*` knobs are also customized — they only govern the `privacy`
  path and any overflow change (§5.1).
- `fanout_outputs_per_tx × denomination + fanout_fee` defines the minimum useful
  reserve UTXO size; validate the reserve isn't configured below it.
- Everything under `throughput` should be runtime-tunable following the existing
  `Provider.SetMaxChangeOutputsPerTx` pattern in `pkg/storage/provider.go`, so an
  operator can raise `target_pool_size` or `target_tps` without a restart.

---

## 5. Runtime design

### 5.1 Denominated fast path — a thin specialization of the #935 funder

No new funding engine. A `throughput` strategy flag on `funder.SQL` (or a small
wrapper) changes five things on the non-sweep path:

1. **Claim from `pool_basket`, expect exact match.** The collector loop is unchanged
   (allocate → recompute fee → `IsFunded`), so fee growth per added input is handled
   exactly as today — no closed-form claim count is assumed. For the designed
   workload (`denomination` = expected per-tx spend) the **first**
   `FindSmallestSufficientUTXOForUpdate` probe against the #935 composite index
   `(user_id, basket_name, reserved_by_id, utxo_status, satoshis)` returns an
   exact-denomination row and funds the tx in one round trip. If a request is still
   unfunded after a small bounded number of claims (off-model shape), it falls
   through to item 5 rather than draining the pool. Many-output actions multi-claim
   by design: a 1,000-output peak action claims ~64 × 240-sat fuel UTXOs (each
   claim nets ~225 sat after its own input fee) via the unchanged collector loop —
   bounded, and cheap at peak action rates of ~160/s. Funder outcome metrics (§5.3)
   track how often we deviate from the one-query happy path.
2. **Skip `CountUTXOs`.** Change-minting heuristics don't apply; the pool gauge (§5.3)
   replaces the count for observability. This deletes the remaining
   pool-size-dependent cost from the hot path.
3. **Deterministic change, no randomization, no shuffle.** Change is normally **zero by
   construction** (denomination ≈ per-tx spend). When a request overshoots and the
   overshoot is ≥ the generic dust floor, mint a single deterministic change output
   into the legacy change basket for the privacy-path funder to consume. Overshoot
   **below** the dust floor is given to the miner as extra fee (existing
   `prepareResult` behavior) — bounded by one denomination per request and counted in
   the §5.3 metrics so operators can see the burn.
4. **`spend_policy` decides the tier walk** (`mined_only` | `prefer_mined` | `any`)
   instead of the `IsDelayed`-driven `includeSending`.
5. **Fallback:** requests that don't fit the model (sweeps, huge outputs, drained
   pool with funds still in reserve/legacy baskets) fall through to the standard #935
   bounded path against the legacy basket, so correctness never depends on the pool.
   `wdk.ErrUTXOContention` retry machinery from #935 applies unchanged to both paths.

### 5.2 Reserve basket & fan-out top-up task

A new monitor task (`pool_top_up` alongside `check_for_proofs` etc. in
`defs.TasksConfig`), driven by the pool gauge:

1. Compute `deficit = (high_water_percent/100) × target_pool_size − (spendable +
   immature_in_flight)`. Counting minted-but-unproven fuel as pipeline inventory
   prevents over-minting during the ~5 min maturation window.
2. If pool level ≥ `low_water_percent`, do nothing (hysteresis: once triggered, top
   up toward `high_water_percent`).
3. Build fan-out transactions as a split tree. **Interior** layers split reserve
   UTXOs into chunk outputs (`fanout_outputs_per_tx × denomination + fee` each);
   only **leaf** transactions create `fanout_outputs_per_tx` outputs of exactly
   `denomination_satoshis` into `pool_basket`. `fanout_max_txs_per_round` caps the
   leaf transactions per round (interior txs are comparatively few:
   1 per `fanout_outputs_per_tx` leaves). This keeps every tx small (~100 outputs ≈
   3.5 KB) while letting one round mint `fanout_outputs_per_tx²` UTXOs per reserve
   input, and the sustained-throughput identity in §4 guarantees a round can mint at
   least `interval × target_tps × 1.2` fuel outputs.
4. Broadcast through the existing background broadcaster / `send_waiting` machinery;
   maturation is tracked by `check_for_proofs` exactly like any other tx. Fan-out
   outputs are ordinary change-purpose P2PKH outputs with fresh derivations — key
   hygiene is preserved even though values are uniform.
5. Self-throttle: never let `immature_in_flight` exceed
   `target_tps × expected_confirmation_seconds × pool_headroom_factor` — minting
   faster than the chain confirms just bloats the DB.

The reserve is filled by the operator with ordinary `internalizeAction` deposits into
`reserve_basket` (large UTXOs, e.g. whole coins). Reserve depth is what the
`reserve_runway_low` alert watches.

### 5.2a Basket separation — why `fuel` is its own basket (resolves §10 Q1)

Fuel lives in a dedicated `fuel` basket, not in `default`. Baskets are already
per-user rows keyed by name, the #935 selection index leads with
`(user_id, basket_name)`, and the privacy-path funder is hardcoded to `default` —
so a dedicated basket gives the two strategies **disjoint data by construction**,
with no schema change.

Why reuse of `default` fails:

- **Poisoned heuristics.** `CountUTXOs` would count millions of denomination rows
  as "existing", permanently suppressing normal change minting.
- **Degenerate funding.** With large change exhausted, a normal payment falls to
  the largest-insufficient path over 24-sat rows: each nets ~9 sat after its
  ~15-sat marginal input fee, so a 100k-sat shortfall means ~11,000 inputs pulled
  16 per micro-query — ~700 round trips and a ~1.6 MB tx.
- **Lock collisions.** Exact-match claims and privacy best-fit contend over the
  same index range, inflating `ErrUTXOContention` retries on both paths.
- **No cheap accounting.** A mixed basket loses the `count × denomination =
  balance` gauge property.

Lifecycle:

1. **Bootstrap.** Provider start with `strategy: throughput` find-or-creates the
   `fuel` and `reserve` baskets for the operator user. The basket row reuses
   existing columns (`minimum_desired_utxo_value := denomination`,
   `number_of_desired_utxos := target_pool_size`) — informational, since the
   heuristics are skipped. Both names become reserved (`wdk.BasketNameForFuel`,
   `wdk.BasketNameForReserve`, alongside `BasketNameForChange`); validation
   rejects colliding user baskets.
2. **Deposit.** Operator internalizes large UTXOs into `reserve`; nothing but the
   top-up task ever spends it.
3. **Mint.** Leaf fan-out txs create standard storage-provided P2PKH outputs into
   `fuel`: `change = true` (the existing `user_utxos` materialization applies
   unchanged), fresh derivation per output, `purpose: "fuel"`.
4. **Claim.** `create.go`'s hardcoded `FindBasketByName(user, "default")` becomes
   strategy-aware: throughput claims from `fuel`, holding `default` as the
   fallback handle for off-model requests.
5. **Overflow.** Any non-zero change routes to `default`, never to `fuel` — fuel
   only ever contains exact-denomination rows (active or tracked-stale).

Migration/adoption/rollback are all **relabels, never sweeps**: adopting existing
fuel-shaped change in `default` is one UPDATE (`satoshis = denomination AND
change = true` → `basket_name = 'fuel'`); rollback relabels `fuel` rows back into
`default`, where the generic funder spends them as small change. Draining
on-chain instead would burn ~62% of a 24-sat pool (each 148-byte input costs
~15 sat of its 24-sat value at 100 sat/kb).

### 5.3 Pool accounting, gauges & metrics

`COUNT(*)`-per-request is replaced by a **pool gauge**: in-memory atomic counters per
`(user, basket, status)` maintained on claim / mint / mature / unreserve events,
reconciled against a real `COUNT(*)` periodically (e.g. each top-up round) to correct
drift and to survive restarts. Multi-instance deployments reconcile from the DB, which
remains the source of truth.

Exposed (via the existing `logging`/`tracing` infrastructure, and as fields on a new
admin/stats endpoint):

- `pool_spendable` (per status tier), `pool_reserved_in_flight`, `pool_immature`
- `reserve_balance_satoshis`
- `pool_runway_seconds = pool_spendable / target_tps`
- `reserve_runway_seconds = reserve_balance / (denomination × (1 +
  fanout_fee_overhead) × target_tps)` — the overhead factor matters: at a 24-sat
  denomination, fan-out fees add ~15% to burn (§6)
- top-up round stats: minted count, per-round failures, **consecutive failed
  rounds** (feeds the `top_up_failing` alert), round duration
- funder outcomes: exact-match hits, fallback hits, contention retries
  (`ErrUTXOContention`), sub-dust overshoot burned (satoshis), and a windowed
  funding-error rate (`ErrNotEnoughFunds`/min, feeds the `not_enough_funds` alert)

### 5.4 Alerting / notifiers

A small `Notifier` interface in a new `pkg/alerting` package:

```go
type Alert struct {
    Rule      string            // e.g. "pool_runway_low"
    Severity  Severity
    Message   string            // human-readable, includes current values
    Values    map[string]string
    Timestamp time.Time
}

type Notifier interface {
    Notify(ctx context.Context, alert Alert) error
}
```

- Built-in drivers: `webhook` (generic JSON POST — covers Slack, PagerDuty, ops
  bridges) and `twilio` (SMS via Twilio's REST API; plain `net/http`, no SDK
  dependency). The interface keeps other channels (email, opsgenie) as drop-ins.
- Rules from §4 are evaluated by the pool accounting loop; firing is rate-limited by
  `cooldown_seconds` per rule so a sustained low-water condition doesn't page every
  tick.
- Alerting is deliberately **decoupled from the funder hot path** — evaluation happens
  in the monitor loop, never per-request.

---

## 6. Worked example — the motivating numbers

Config: 240-byte txs, 100 sat/kb fee model, 100k TPS, 300 s confirmation, headroom 1.5,
fan-out 100 outputs/tx.

| Quantity | Derivation | Value |
|---|---|---|
| Denomination | ceil(240/1000 × 100) | **24 sat** (passes the 15-sat fuel floor; below the generic 40-sat dust floor — see §4 exemption) |
| Consumption rate | target_tps | 100,000 UTXO/s |
| Pipeline inventory (minted, immature) | 100k × 300 s | 30 M UTXOs |
| Target pool | 100k × 300 × 1.5 | **45 M UTXOs** (~10.8 BSV of fuel) |
| Required mint rate | = consumption | 100,000 outputs/s |
| Leaf fan-out txs needed | 100k / 100 outputs | 1,000 tx/s sustained (3,558 bytes each ⇒ 10,000 per 10-s round; the 12,000 default gives 1.2× recovery margin) |
| Fan-out fee overhead | 356 sat fee / (100 × 24 sat fuel) | **~14.8%** of fuel value (~15.0% incl. the depth-2 chunk tier) — at a 24-sat denomination fan-out fees are a first-order cost, not a rounding error |
| Fee burn at rated load | 24 sat × 100k/s | 2.4 M sat/s ≈ **2,074 BSV/day** |
| Reserve for 1 day | 2,074 fee burn + ~311 fan-out fees | **~2,385 BSV** |

### 6.1 Primary profile: repo market

The table above is the single-output **stress envelope**. The actual deployment
profile is gentler on every axis because applications pack outputs into actions
(D5):

| Quantity | Derivation | Value |
|---|---|---|
| Average action rate | — | ~10 /s; a 15-output action (≈668 B, 67-sat fee + ~170 sat of output value) funds with **one** 240-sat claim |
| Peak output rate | market open/close | ~160,000 outputs/s |
| Peak shape | ~160 actions × 1,000 outputs | ~64 fuel claims per action |
| Peak fuel consumption | 160 × ~64 | **~10,000 claims/s** — an order of magnitude below the envelope; a well-provisioned single Postgres becomes plausible |
| Fee per output when packed | ~4,350 sat / 1,000 outputs | ~4.4 sat/output vs 24 sat single-output — packing is a ~5× fee win |
| Pool sizing for diurnal peaks | peak window × consumption × headroom | a 15-min peak at ~10k fuel/s ≈ 9 M fuel; target ~15–20 M covers both daily peaks, refilled between them (5-min maturation ≪ inter-peak hours) |
| Average fee burn | 10 × 240 sat/s | ~2.1 BSV/day, plus burst burn during the two peaks |

Honest observations the config must surface rather than hide:

- **A 100k-UTXO pool (100 outputs × 1000 txs) is ~1 second of runway at 100k TPS.**
  The steady-state pool target has to be tied to `target_tps × confirmation latency`
  (hence §4 derives `target_pool_size` from those inputs, with explicit override) —
  and the *per-round fan-out batch* must equal `target_tps × top_up interval`
  (1.2 M outputs at the defaults), not the pool figure. At lower rated loads
  (e.g. 1k TPS) the same config derives a 450k pool and everything scales down
  linearly.
- **Strict `mined_only` at high TPS needs deep buffers** because block intervals are
  Poisson — a 45-minute block gap at 100k TPS means 270 M UTXOs of buffer or a stall.
  `prefer_mined` (default) degrades gracefully to spending unproven fuel — which is
  self-created, first-seen-safe change, the standard zero-conf posture on BSV.
  `mined_only` remains available for operators who insist and size for it.
- **Wallet-level design ≠ whole-system throughput.** #935 makes selection flat in pool
  size (~0.16 ms/op benchmarked); this proposal removes the remaining per-request
  pool-size-dependent work (`COUNT(*)`, randomization) and guarantees the one-query
  happy path. From there, throughput is a database-provisioning problem (connection
  pools, partitioning `bsv_user_utxos`, read replicas, sharding users across
  instances) rather than a wallet-protocol problem. Single-node Postgres will saturate
  well below 100k TPS; the claim path is already multi-instance-safe (`SKIP LOCKED` +
  CAS reservation), so operators can scale horizontally.

---

## 7. Schema & index changes

- **None required for the claim path** — #935's `idx_user_utxos_selection (user_id,
  basket_name, reserved_by_id, utxo_status, satoshis)` serves the exact-denomination
  probe as-is. The follow-up already noted on #935 (appending `output_id` as a 6th
  column for a pure ordered index walk) benefits this workload too and could ride
  along with Phase 1.
- `output_baskets`: no schema change — `fuel` and `reserve` are ordinary baskets. The
  throughput knobs live in infra config, not per-user rows: scope is global per
  deployment by decision (§10 Q4) — no per-user strategy override.
- Optional `pool_gauge` table `(user_id, basket_name, status, count)` if we prefer
  DB-backed counters over reconcile-on-start in multi-instance deployments.
- SQLite is not the target engine for this strategy (single-writer WAL tops out far
  below these rates even with #935's WAL defaults); config validation should warn when
  `strategy: throughput` is combined with the SQLite engine.

## 8. Phasing

0. **Phase 0 — land #935.** Rebase this work on `feat/dr-v1-track-p` once merged; the
   funder split points (`allocateBounded`, `BoundedUTXOQuery`, repo micro-queries) are
   the extension seams Phase 1 builds on.
1. **Phase 1 — config + fast path.** `utxo_management` config section, strategy
   switch, denominated claim specialization (§5.1: pool-basket targeting, skip
   `CountUTXOs`, deterministic change, `spend_policy`), funder-outcome metrics.
   Immediately useful even without automation (operator can fan out manually).
2. **Phase 2 — top-up automation.** `reserve` basket convention, `pool_top_up`
   monitor task, tree fan-out builder, self-throttling, pool gauge + reconciliation.
3. **Phase 3 — observability & alerting.** Stats surface, `pkg/alerting`, webhook +
   Twilio notifiers, rules & cooldowns.
4. **Phase 4 — scale hardening.** Internal claim batching (amortize N concurrent
   `createAction`s into one claim query — an internal optimization, NOT an API
   change; per §10 Q5 there is no batch endpoint, and at repo-market rates this is
   likely unnecessary), gauge-backed admission control (fail fast with a typed
   "pool exhausted, retry-after" error), partitioning guidance, load-test harness
   pinned at rated peak load in CI (nightly, reduced scale — extending #935's
   `BenchmarkSQLFund` baseline).

## 9. Privacy & security trade-offs (explicit)

- Uniform denominations, deterministic change, and a shared fan-out ancestry make the
  operator's tx graph **fully linkable by design**. Documentation must state that
  `strategy: throughput` is for single-operator infrastructure funds, never for
  custodial end-user funds. The strategy is selected per deployment, and the privacy
  path remains the default.
- Key hygiene is *not* sacrificed: fan-out and change outputs keep fresh derivation
  suffixes (type-42 style), so key reuse does not increase even though values are
  uniform.
- Twilio credentials are supplied the same way as DB passwords: via the config
  loader's viper `AutomaticEnv` override of config keys (`internal/config/loader.go`),
  never written into the YAML. Alert payloads must not include keys or derivation
  material.

## 10. Open questions

1. **Basket naming vs. flag — RESOLVED: dedicated `fuel` basket.** Fully specified
   in §5.2a: disjoint data for the two funding paths, count-based gauges, zero-cost
   relabel migration in both directions.
2. **Fee-model changes mid-flight — RESOLVED: consolidate.** A fee-rate change
   alters the derived denomination; the old pool stays claimable-by-value inside
   `fuel` (the `satoshis` key partitions it naturally) but is stale. The top-up
   task gains a **consolidation** phase — the inverse of fan-out: spend up to
   `consolidation_inputs_per_tx` (default 1000) stale fuel UTXOs per transaction
   and mint new-denomination outputs from the recouped value. One or many rounds
   run until the stale partition is drained. Mechanics and economics:
   - Yield per stale input is `stale_denomination − marginal_input_fee(new rate)`
     (148-byte P2PKH input). Rate **down** (24 → 12 sat at 50 sat/kb): each old
     24-sat input nets ~16.6 sat — recoup is cheap and productive. Rate **up**
     (24 → 36 sat at 150 sat/kb): each nets ~1.8 sat — consolidation recovers
     little but still cleans the pool; the alternative (leaving stale rows) wastes
     nothing less and keeps the gauge polluted, so consolidate regardless.
   - Per-tx remainder below one new denomination routes to `default`
     (sub-dust → miner, counted in the §5.3 burn metric).
   - Consolidation txs spend only self-created, first-seen-safe outputs; they run
     at lower priority than fan-out, count toward pipeline inventory for
     throttling, and reuse the same broadcast/maturation machinery.
   - The stale-inventory gauge alerts (`stale_fuel` threshold) if consolidation
     falls behind or is disabled.
   (Strategy *rollback* remains a pure relabel into `default` — §5.2a;
   consolidation is specifically for denomination changes while the strategy
   stays on.)
3. **Commission interaction — RESOLVED: folded in when non-zero.** The denomination
   derivation adds `commission.satoshis` (and the commission output's bytes to
   `expected_tx_size_bytes`) whenever commission is enabled, so a fuel UTXO covers
   the whole per-tx spend and change stays zero by construction; at commission 0
   nothing changes (see §4).
4. **Per-user scope — RESOLVED: global.** The strategy is a property of the
   deployment, not of individual users: one `utxo_management` config per server,
   applied uniformly. No per-user strategy columns; a multi-tenant operator who
   needs both models runs separate deployments. This keeps the funder's strategy
   dispatch branch-free on the hot path and the pool gauges single-scope.
5. **Batch `createAction` — RESOLVED: no batch API.** Actions are inherently atomic
   per txid; a batch endpoint adds nothing the transaction format doesn't already
   provide. Applications batch at the application layer by defining many outputs
   per action (repo profile: typical ~15 outputs; peaks as ~160 actions × 1,000
   outputs ≈ 160k outputs/s). Packing also amortizes fees (~4.4 sat/output at
   1,000 outputs vs 24 sat single-output) and drops peak claim load to ~10k
   fuel/s. The funder needs no changes beyond §5.1's multi-claim behavior.
6. **Exclusion-list growth.** #935 threads allocated rows through `NOT IN` exclusion
   lists (~32k-input theoretical ceiling). Denominated funding allocates 1–few rows per
   tx so this is a non-issue on the happy path, but Phase 4 batch claiming should keep
   it in view.
7. **Hash-puzzle fuel locking — EXPLORATION.** Lock fuel with a SHA-256 hash puzzle
   (`OP_SHA256 <hash> OP_EQUAL`, unlock = push the 32-byte preimage) instead of
   P2PKH. A Go `HashPuzzle` script template (port of the truth-machine demo's TS
   template, with interpreter-verified tests) is ready for
   `bsv-blockchain/go-script-templates` on branch `claude/hashpuzzle-template`.
   Why it's attractive for fuel:
   - **No ECDSA on the hot path**: claims push a stored secret instead of signing —
     at peak that saves ~10k signature computations/s, and minting needs no per-output
     key derivation.
   - **Half the input weight**: a hash-puzzle input is ~74 bytes vs ~148 for P2PKH →
     marginal input fee ~7.4 sat vs ~14.8 at 100 sat/kb; the fuel floor drops
     accordingly and packed-action fees shrink further.
   - Wallet integration sketch: fan-out mints fuel with per-output random secrets
     (secret stored with the output row), a new output type carries the smaller
     estimated input size, and the claim path emits the preimage push.
   Why it needs a security review before adoption:
   - The preimage does not commit to the spending transaction (no signature), so a
     mempool observer or miner can rewrite an in-flight spend and redirect its
     value. Exposure is bounded per output by the denomination (240 sat) but at
     peak ~10k claims/s the in-flight aggregate is ~2.4M sat/s of
     redirectable value — acceptability rests entirely on first-seen policy.
   - Secrets are bearer instruments: a DB leak makes fuel directly spendable
     (P2PKH fuel requires the root key). Store encrypted at minimum.
   Status: template PR ready; wallet adoption is a Phase-2+ option gated on that
   review.
