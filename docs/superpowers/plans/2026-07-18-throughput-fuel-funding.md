# Throughput Fuel Funding Implementation Plan

> **For agentic workers:** implement tasks strictly in order; each task ends with
> its own commit and a green targeted test run. Spec:
> `docs/superpowers/specs/2026-07-18-throughput-fuel-funding-design.md`.

**Goal:** Ship the `throughput` UTXO-management strategy (fuel pool, denominated
fast path, shaped-change fan-out, client keeper, OTel metrics) on PR #936.

**Architecture:** strategy dispatch in `create.Create`; funder gains a
non-breaking `FundWithConstraints` seam; fuel outputs are ordinary change rows in
the `fuel` basket; minting is client-driven via a `FuelShape` create-action
option; metrics are greenfield OTel on the existing OTLP endpoint.

**Tech Stack:** Go 1.25, gorm, viper, otel SDK (`sdk/metric`,
`otlpmetricgrpc` — promote from indirect), gocron not required (keeper uses a
plain ticker).

## Global Constraints

- Base: `claude/friendly-wozniak-azql5k` @ `4ada637` (main + docs commits).
- Task order LOCKED: F1 → F2 → F3 → F4 → F5 → F6 → F7 → final gate.
- Per-task gate: `go build ./... && go test ./<touched packages>/...`;
  full gate at the end: `go test ./... ` + `golangci-lint run` (v2.12.2,
  `.golangci.json`) + `go generate ./pkg/infra` (example YAML in sync).
- House rules: wrap errors `fmt.Errorf("…: %w", err)`; gofumpt + gci (local
  prefix `github.com/bsv-blockchain/go-wallet-toolbox`); exhaustive switches
  over new enums; no `//nolint` without justification.
- `strategy=privacy` behavior must be provably unchanged (existing suites are
  the regression net — they run with the default config).
- Commit trailer: `Co-Authored-By: Claude Fable 5 <noreply@anthropic.com>` +
  `Claude-Session: https://claude.ai/code/session_01H3WQjBJkXrUzPJouK98Zy4`.

---

### Task F1: config layer — defs.UTXOManagement + defs.Observability

**Files:**
- Create: `pkg/defs/utxo_management.go`, `pkg/defs/utxo_management_test.go`,
  `pkg/defs/observability.go`
- Modify: `pkg/infra/config.go` (fields ~:16-32, defaults ~:69-94, validate
  ~:113-154), `pkg/storage/provider_options.go` (field + `WithUTXOManagement` +
  default), `pkg/infra/gorm_provider_options.go`, `pkg/infra/config_test.go`
- Regenerate: `infra-config.example.yaml`

**Steps:**
- [ ] defs types per spec (strategy + spend-policy enums with
  `Parse…Str` helpers mirroring `parse_enum.go`; `TaskConfig` reused for
  `top_up`); `DefaultUTXOManagement()` = privacy strategy + repo-market-shaped
  throughput defaults; `Denomination(feeModel, commission)` derivation;
  `Validate(feeModel, commission)` per spec rules.
- [ ] Thread through infra + provider options; cross-validate observability ×
  tracing in `Config.Validate`.
- [ ] Table tests: derivation (explicit override, commission fold-in),
  validation rejections (denomination ≤ marginal input fee, identity violation,
  reserved basket names), YAML round-trip via loader.
- [ ] Run: `go test ./pkg/defs/... ./pkg/infra/...` ·
  Commit: `feat(defs): add utxo_management and observability configuration`

### Task F2: wdk basket constants + seeding

**Files:**
- Modify: `pkg/wdk/constants.go`, `pkg/storage/provider.go` (`FindOrInsertUser`
  ~:394; store throughput config), `pkg/internal/storage/repo/users.go` (no
  change expected — varargs already), `pkg/internal/validate/validate_basket_config.go`
- Tests: `pkg/storage/provider_find_or_insert_user_test.go` (+ new cases)

**Steps:**
- [ ] `BasketNameForFuel`/`BasketNameForReserve`; seed both baskets on new-user
  creation when strategy=throughput (fuel: min=denomination,
  desired=target_pool_size; reserve: 0/0); lazy `FindOrCreateBasket` helper on
  the provider for existing users.
- [ ] Run: `go test ./pkg/storage/... ./pkg/wdk/...` ·
  Commit: `feat(storage): seed fuel and reserve baskets for throughput strategy`

### Task F3: funder constraints seam + denominated fast path

**Files:**
- Modify: `pkg/internal/storage/funder/sql.go` (Constraints struct,
  `FundWithConstraints`, tier builder), `pkg/storage/internal/actions/create.go`
  (strategy dispatch, skip CountUTXOs, fuel→default fallback, change-basket
  parameter on `newOutputs`), `pkg/storage/internal/actions/actions.go` (thread
  throughput config), `pkg/storage/provider.go` (pass config into actions)
- Tests: `pkg/internal/storage/funder/sql_constraints_test.go`,
  `pkg/storage/internal/integrationtests/throughput_funding_test.go`,
  extend `sql_bench_test.go` with `fuel_pool` sub-benchmark

**Steps:**
- [ ] `FundWithConstraints` (nil-safe delegation from `Fund`); `SpendTiers`.
- [ ] Dispatch in `Create`: throughput → fuel basket + policy tiers + change cap
  1 + no CountUTXOs; `ErrNotEnoughFunds` → fallback to default basket (same
  constraints); outcome classification (exact_match/multi_claim/fallback)
  returned for F6.
- [ ] Change outputs to default basket always (basket param on `newOutputs`).
- [ ] Tests: exact-match single claim; multi-claim packed action; policy tiers
  (mined_only refuses unproven); fallback drains default; privacy-mode
  regression (existing suites untouched); invariants assertion after each.
- [ ] Run: `go test ./pkg/internal/storage/funder/... ./pkg/storage/...` ·
  Commit: `feat(funder): denominated fuel fast path with spend policy and fallback`

### Task F4: shaped-change fan-out option

**Files:**
- Modify: `pkg/wdk/storage_create_action_args.go` (`ShapedChange`, options
  field), `pkg/internal/validate/valid_create_action_args.go`,
  `pkg/storage/internal/actions/create.go` (pseudo-change outputs, targetSat/txSize
  accounting, remainder to default)
- Tests: `pkg/storage/provider_create_action_test.go` (+shape cases),
  `pkg/storage/internal/integrationtests/fanout_test.go`

**Steps:**
- [ ] Types + validation per spec (leaf=denomination into fuel; chunk ≥
  outputs×denomination into reserve; count bounds; strategy required).
- [ ] Create-path shaping; ensure shaped rows satisfy `isChangeDaoScope` and
  round-trip through `resultOutputs` with derivation suffixes.
- [ ] Integration: fan-out create → process → outputs become mined fuel
  UserUTXOs claimable by F3 path (SQLite + Postgres-gated).
- [ ] Run: `go test ./pkg/storage/... ./pkg/wdk/... ./pkg/internal/validate/...` ·
  Commit: `feat(storage): shaped-change fan-out option for fuel minting`

### Task F5: FuelKeeper (client-side)

**Files:**
- Create: `pkg/wallet/fuelkeeper/keeper.go`, `keeper_test.go`
- Modify: `pkg/wallet/wallet.go` (no structural change; keeper consumes the
  public Wallet API + `ListOutputs`)

**Steps:**
- [ ] Ticker loop per spec (pool level via ListOutputs totals, low/high water
  hysteresis, immature counted as inventory, per-round leaf-tx cap, reserve
  input selection, CreateAction with `FuelShape` + SignAction).
- [ ] Unit tests with mocked wallet interface; end-to-end test via wallet
  testabilities: seeded reserve → keeper round → fuel pool grows → funder
  claims from it.
- [ ] Run: `go test ./pkg/wallet/...` ·
  Commit: `feat(wallet): fuel keeper for automated pool top-up`

### Task F6: OTel metrics

**Files:**
- Create: `pkg/tracing/metrics.go`, `pkg/storage/internal/metrics/metrics.go`
- Modify: `pkg/infra/server.go` (enable beside tracing block),
  `pkg/storage/provider.go` + `create.go` (counter increments, gauge callback
  registration), `go.mod` (promote `sdk/metric`, `otlpmetricgrpc`)

**Steps:**
- [ ] `tracing.EnableMetrics` per spec; instruments catalog (proposal §5.4
  names); observable-gauge callback = one GROUP BY count + reserve sum per
  export interval; runway derived from config; all no-op without provider.
- [ ] Smoke tests with `sdkmetric` manual reader.
- [ ] Run: `go test ./pkg/tracing/... ./pkg/storage/... ./pkg/infra/...` ·
  Commit: `feat(observability): OTel metrics for fuel pool, funder, and runway`

### Task F7: example + docs

**Files:**
- Modify: `examples/throughput_mode/throughput-mode.go` (drop build tag, wire
  FuelKeeper, STATUS header), `examples/throughput_mode/throughput-mode.md`,
  `plans/high-throughput-utxo-management.md` (mark §8 P1–P3 delivered)

**Steps:**
- [ ] Untag; `go vet ./examples/...` green; runbook aligned with real config.
- [ ] Run: `go build ./... && go vet ./examples/...` ·
  Commit: `docs(examples): wire throughput-mode example to the shipped feature`

## Final gate

- [ ] `go test ./...` (SQLite) green; `TEST_DB_MODE=postgres` suite green if a
  local PG is available (CI runs it regardless).
- [ ] `golangci-lint run` clean; `go generate ./pkg/infra` no diff.
- [ ] Adversarial review pass over the full diff; push to
  `claude/friendly-wozniak-azql5k`; update PR #936 body.
