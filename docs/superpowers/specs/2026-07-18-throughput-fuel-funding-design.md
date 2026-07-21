# Throughput Fuel Funding — Design

**Date:** 2026-07-18
**Branch:** `claude/friendly-wozniak-azql5k` (base: `main` @ `b7e05a2`, post-#935 Track P funder)
**Sources:** `plans/high-throughput-utxo-management.md` (approved proposal, decisions D1–D6 settled), PR #936 discussion, operator profile: repo-market settlement (~10 actions/s avg, ~160k outputs/s peaks via packed actions, 240-sat typical denomination).

## Goal

Implement the `throughput` UTXO-management strategy: a dedicated `fuel` basket of
exact-denomination, storage-managed change UTXOs claimed by the Track P bounded
funder in one exact-match micro-query, replenished from a `reserve` basket via
client-driven fan-out transactions, observable end-to-end through OpenTelemetry
metrics. `privacy` (today's behavior) stays the default; nothing changes when the
strategy is not enabled.

## Scope (proposal sections addressed)

| Proposal § | Requirement | Task |
|---|---|---|
| §4 | `utxo_management` config: strategy, denomination/pool derivation, validation | F1 |
| §4 | `observability.metrics` config (OTLP, reuses `tracing.dialAddr`) | F1, F6 |
| §5.2a | `fuel`/`reserve` reserved baskets, per-user seeding, no schema change | F2 |
| §5.1 | Denominated fast path: fuel-first, spend policy, no `CountUTXOs`, deterministic change, fallback to `default` | F3 |
| §5.2 | Fan-out minting (change shaping); tree fan-out via chunk outputs | F4 |
| §5.2 | Top-up automation (client-side keeper — storage holds no user keys) | F5 |
| §5.3–5.4 | Pool gauges, funder outcome counters, OTel MeterProvider | F6 |
| §8 P1 | `examples/throughput_mode` untagged & wired | F7 |

**Out of scope:** consolidation task (D2 — follows once denomination changes are
possible in practice), hash-puzzle fuel (D7 — gated on security review), batch
claiming, admission control, per-user strategy scope (D4 settled that away).

## Core design — F3 denominated fast path

### Key insight: fuel outputs are ordinary storage change in a different basket

`isChangeDaoScope` (`repo/outputs.go:698`) qualifies outputs for `user_utxos`
materialization by `Change=true AND basket NOT NULL AND satoshis>0` — basket name
is not consulted. The client assembler derives change locking scripts on
`ProvidedBy=storage && Purpose=="change"` (`create_action_tx_assembler.go:207`),
and unlocks spends from stored derivation prefix/suffix. The bounded funder
micro-queries are already basket-parameterized and index-covered
(`idx_user_utxos_selection`). Therefore a fuel UTXO is exactly a change output
with `BasketName="fuel"` — **no schema, signing, or materialization changes**.

### Strategy dispatch in `create.Create`

`create.go:229` currently hardcodes `FindBasketByName(userID, BasketNameForChange)`.
With `strategy=throughput`:

- Resolve **both** the fuel basket (funding source) and the default basket
  (change destination). Lazy-seed them if missing (F2).
- **Skip `CountUTXOs` entirely** (`create.go:345`). The desired-count change
  heuristic does not apply in throughput mode: pass `existing` such that the
  collector's change budget clamps to **1** (`NumberOfDesiredUTXOs - existing = 0
  → clamped 1` at `sql.go:428`); with change count 1, `ChangeDistribution`'s
  `count==1` branch is already deterministic (`change_distribution.go:31`) — no
  new distribution code, no randomization.
- Fund against the fuel basket with policy tiers (below). On
  `wdk.ErrNotEnoughFunds` → **fallback**: fund against the default basket with
  today's bounded path (still without `CountUTXOs`; change budget stays 1).
  Fallback keeps correctness independent of pool health.
- All change outputs (overshoot) route to the **default** basket
  (`newOutputs` basket parameter — currently hardcoded at `create.go:733`),
  preserving the invariant that `fuel` holds only exact-denomination rows.
- Funder outcome recorded per request: `exact_match` (1 claim, satoshis ==
  denomination), `multi_claim`, `fallback` (F6 counters).

### Spend policy → tier walk

`allocateBounded` builds tiers `[Mined, Unproven] (+Sending iff includeSending)`
(`sql.go:170-173`). A new `funder.SpendTiers(policy)` maps:
`mined_only → [Mined]`, `prefer_mined → [Mined, Unproven]` (default),
`any → [Mined, Unproven, Sending]`. `Fund` gains tier control via a new
`FundParams`-lite seam: rather than growing the 12-arg signature, add

```go
// funder/sql.go
type Constraints struct {
    Tiers           []wdk.UTXOStatus // nil → legacy: mined,unproven(+sending if includeSending)
    MaxChangeOutputs uint64          // 0 → legacy: maxChangeOutputsPerTx atomic
}
func (f *SQL) FundWithConstraints(ctx ..., c Constraints, tx *gorm.DB) (*Result, error)
```

`Fund` delegates to `FundWithConstraints` with zero-value constraints — existing
call sites, tests, and the benchmark are untouched.

## F1 — configuration

`pkg/defs/utxo_management.go`:

```go
type UTXOManagementStrategy string // "privacy" | "throughput"
type SpendPolicy string            // "mined_only" | "prefer_mined" | "any"

type Throughput struct {
    ExpectedTxSizeBytes     uint64  `mapstructure:"expected_tx_size_bytes"`
    ExpectedOutputSatoshis  uint64  `mapstructure:"expected_output_satoshis"`
    DenominationSatoshis    uint64  `mapstructure:"denomination_satoshis"` // 0 → derive
    TargetTPS               uint64  `mapstructure:"target_tps"`
    ExpectedConfirmationSeconds uint64 `mapstructure:"expected_confirmation_seconds"`
    PoolHeadroomFactor      float64 `mapstructure:"pool_headroom_factor"`
    TargetPoolSize          uint64  `mapstructure:"target_pool_size"` // 0 → derive
    LowWaterPercent         uint64  `mapstructure:"low_water_percent"`
    HighWaterPercent        uint64  `mapstructure:"high_water_percent"`
    SpendPolicy             SpendPolicy `mapstructure:"spend_policy"`
    PoolBasket              string  `mapstructure:"pool_basket"`    // default "fuel"
    ReserveBasket           string  `mapstructure:"reserve_basket"` // default "reserve"
    FanoutOutputsPerTx      uint64  `mapstructure:"fanout_outputs_per_tx"`
    FanoutMaxTxsPerRound    uint64  `mapstructure:"fanout_max_txs_per_round"`
    FanoutTreeDepth         uint64  `mapstructure:"fanout_tree_depth"`
    ConsolidationInputsPerTx uint64 `mapstructure:"consolidation_inputs_per_tx"`
}

type UTXOManagement struct {
    Strategy   UTXOManagementStrategy `mapstructure:"strategy"`
    Throughput Throughput             `mapstructure:"throughput"`
}
```

- `Denomination(feeModel)` derivation: `ceil(expected_tx_size_bytes/1000 ×
  fee.value) + expected_output_satoshis + commission.satoshis(when enabled)`;
  explicit `denomination_satoshis > 0` wins.
- `Validate(feeModel, commission)`: denomination > marginal fuel-input fee
  (`ceil(148/1000 × rate)`, D6 exemption — the generic ×2 dust floor deliberately
  does not apply); `fanout_outputs_per_tx × fanout_max_txs_per_round ≥ target_tps
  × top_up.interval × 1.2`; basket names non-empty, distinct, and ≠ `default`;
  percent bounds; warn (log, not fail) when engine is SQLite.
- `pkg/defs/observability.go`: `Observability{ Metrics{ Enabled bool,
  ExportIntervalSeconds uint } }`; cross-validated in `infra.Config.Validate`:
  metrics enabled requires non-empty `tracing.dialAddr` (tracing.enabled may be
  false — it gates spans only).
- Threading: fields on `infra.Config` (+`Defaults`, `Validate`), `storage.WithUTXOManagement`
  provider option, appended in `GORMProviderOptionsFromConfig`. `top_up` schedule
  lives under `utxo_management.throughput.top_up` (client keeper consumes it; the
  infra monitor does NOT register it — F5).
- Regenerate `infra-config.example.yaml`.

## F2 — baskets

- `wdk.BasketNameForFuel = "fuel"`, `wdk.BasketNameForReserve = "reserve"`.
- Seeding: `FindOrInsertUser` (`provider.go:394`) passes two extra
  `BasketConfiguration`s to `CreateUser` when strategy=throughput
  (fuel: `MinimumDesiredUTXOValue = denomination`, `NumberOfDesiredUTXOs =
  target_pool_size` — informational; reserve: 0/0 like non-change baskets).
  Existing users: lazy `FindOrCreateBasket` in the throughput funding path.
- Reserved names: `validate.ValidBasketConfiguration` and output-basket
  validation reject caller-supplied outputs targeting `fuel` unless the request
  is a fan-out shape (F4) — reserve accepts basket-insertion deposits by design.

## F4 — fan-out via shaped change

The storage server holds no user keys, so minting rides the normal
createAction→signAction flow. New optional field on `wdk.ValidCreateActionOptions`
(toolbox extension; absent for BRC-100 clients, `omitempty`):

```go
type ShapedChange struct {
    Count    uint64                   `json:"count"`
    Satoshis primitives.SatoshiValue  `json:"satoshis"`
    Basket   primitives.StringUnder300 `json:"basket"` // fuel | reserve
}
// ValidCreateActionOptions gains: FuelShape *ShapedChange `json:"fuelShape,omitempty"`
```

In `create.Create`, when `FuelShape` is set (requires strategy=throughput):

- Append `Count` pseudo-change outputs (Change=true, ProvidedBy=storage,
  Purpose=change, Basket=`FuelShape.Basket`, fresh derivation suffixes, exact
  `Satoshis`) — same construction as today's change rows (`create.go:727`).
- `targetSat += Count × Satoshis`, `txSize += Count × P2PKHOutputSize` before
  funding; funding then proceeds normally (typically fully covered by the
  explicit reserve inputs the keeper provides — the funder allocates nothing).
- Remainder change (≥ dust floor) → single deterministic output to `default`.
- Validation: `Count ∈ [1, fanout_outputs_per_tx]`; `Basket ∈ {fuel, reserve}`;
  leaf shapes (`Basket=fuel`) require `Satoshis == active denomination`; interior
  chunk shapes (`Basket=reserve`) require `Satoshis ≥ fanout_outputs_per_tx ×
  denomination` (tree depth 2).
- Because shaped outputs are ordinary change rows, maturation
  (`CreateUTXOForSpendableOutputsByTxID`), reorg handling, and client signing
  need zero changes.

## F5 — FuelKeeper (client-side top-up)

`pkg/wallet/fuelkeeper`: a goroutine-per-wallet helper owned by the operator's
process (which holds the keys), torn down via context.

```go
type Config struct { // mirrors utxo_management.throughput + top_up interval
    Denomination, TargetPoolSize, LowWaterPercent, HighWaterPercent uint64
    FanoutOutputsPerTx, FanoutMaxTxsPerRound uint64
    Interval time.Duration
}
func New(w *wallet.Wallet, cfg Config, logger *slog.Logger) *Keeper
func (k *Keeper) Run(ctx context.Context) // ticker loop; Stop via ctx cancel
```

Round algorithm: pool level via `ListOutputs(basket=fuel)` `totalOutputs`; if
below low-water, mint toward high-water: list spendable reserve outputs
(`ListOutputs(basket=reserve, include=locking scripts)`), build
createAction(inputs=reserve outpoints, `FuelShape{count, denomination, fuel}`)
→ sign → broadcast, up to `FanoutMaxTxsPerRound` per round. Reserve inputs that
are storage-known outputs (wallet-payment internalize or interior chunk change)
are unlocked by the standard assembler (`ProvidedByYouAndStorage` path); MVP
requires that and documents it. Immature fuel (unproven) counts toward inventory
to prevent over-minting.

## F6 — observability

- `pkg/tracing`: `EnableMetrics(logger, serviceName, dialAddr string,
  interval time.Duration) (func(), error)` — `otlpmetricgrpc` exporter +
  `sdkmetric.MeterProvider` (periodic reader), `otel.SetMeterProvider`; enabled
  from `infra.NewServer` beside the tracing block, gated by
  `observability.metrics.enabled`, cleanup chained.
- `pkg/storage/internal/metrics` (greenfield): funder outcome counters
  (`wallet.funder.claims{result}`, `wallet.funder.not_enough_funds`,
  `wallet.funder.contention_retries`) incremented in `create.go`; pool
  observable gauges (`wallet.utxo.pool.spendable{basket,status}`,
  `wallet.utxo.pool.runway_seconds`, `wallet.utxo.reserve.balance_satoshis`,
  `wallet.utxo.reserve.runway_seconds`) via a registered callback running one
  `GROUP BY basket_name, utxo_status` count (+ reserve satoshi sum) per export
  interval — never on the hot path. Runway computed from configured
  `target_tps × denomination` (with fan-out overhead factor per proposal §5.3).
  All instruments no-op when no MeterProvider is registered (otel default),
  so `privacy` deployments pay nothing.

## Error handling & compatibility

- `strategy=privacy` (default): zero behavior change — dispatch happens before
  any new code path; `Fund` signature untouched (constraints seam delegates).
- Fuel exhausted → transparent fallback to `default` + `fallback` counter; both
  drained → `wdk.ErrNotEnoughFunds` exactly as today.
- `FuelShape` without throughput strategy → `wdk` validation error.
- Contention retry (`ErrUTXOContention`, 3 attempts) applies unchanged to both
  paths.
- BRC-100 conformance: new option is additive + omitempty; conformance vectors
  never set it; `FromValidCreateActionArgs` passes it through explicitly.

## Testing strategy

Funder: constraints/tier-walk unit tests + exact-match fast-path tests via the
existing funder testabilities (fuel basket + denomination fixtures); fallback and
multi-claim packed-action integration tests via storage testabilities (SQLite +
Postgres harness, `TEST_DB_MODE=postgres`); shaped-change create/sign round-trip
via wallet testabilities (fan-out tx signs and its outputs materialize as fuel
UserUTXOs); config validation table tests; metrics smoke test with the SDK
manual reader. `BenchmarkSQLFund` gains a `fuel_pool` sub-benchmark
(exact-denomination pool, expect ≤ existing best case).
