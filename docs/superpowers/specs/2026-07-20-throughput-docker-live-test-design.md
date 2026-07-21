# Throughput Docker Live-Test Stack — Design

**Date:** 2026-07-20
**Branch:** to be created as `feat/throughput-docker-live-test` (worktree)
**Status:** Approved for implementation
**Depends on:** throughput UTXO strategy (`pkg/defs/utxo_management.go`, FuelKeeper, PR #936 fuel funding)

## Goal

Ship two first-class run modes for the infra server and Docker:

1. **Standard (privacy)** — today’s path: privacy-mode UTXO management for normal operations.
2. **Throughput (live test)** — throughput-mode UTXO management sized for ~1000 TPS OP_RETURN createActions, plus a tiny wallet-client load generator that drives that load and keeps the fuel pool topped up.

## Non-goals

- Changing funder / FuelKeeper core algorithms.
- Batch createAction APIs.
- Production multi-tenant throughput deployments.
- Guaranteeing 1000 TPS against real network/ARC limits in CI (live tests remain operator-run).

## Decisions (locked)

| Decision | Choice |
|---|---|
| Cmd shape | Two server binaries under `cmd/` |
| Standard strategy | Explicit `utxo_management.strategy: privacy` |
| Throughput profile | `expected_tx_size_bytes: 200`, fee `100 sat/kb`, `target_tps: 1000`, pure OP_RETURN outputs (`expected_output_satoshis: 0`) |
| Load generator | Wallet client at 1000 createAction/s; one OP_RETURN output with payload `"the proof is in the pudding"` |
| Funding | Loadgen runs FuelKeeper + optional faucet bootstrap (env-driven) |
| Docker packaging | Separate Dockerfile + compose for each mode (Approach A) |

## Architecture

```
Standard stack                          Throughput stack
─────────────────                       ──────────────────────────────────
docker-compose.yaml                     docker-compose.throughput.yaml
  db (postgres)                           db (postgres)
  infra  ← Dockerfile                     infra_throughput ← Dockerfile.throughput
            privacy config                  loadgen          ← same Dockerfile
                                            throughput config
                                              │
                                              ├─ storage JSON-RPC
                                              └─ FuelKeeper (client-side)
```

Both stacks share the same `infra.NewServer` library. Only default config path, compose wiring, and the loadgen service differ.

## Components

### 1. `cmd/infra` (existing, standard)

- Loads `infra-config.yaml` (unchanged).
- Docker image bakes/mounts `infra-config-docker.yaml` with **explicit** `utxo_management.strategy: privacy` (today it relies on code defaults; make the file self-documenting).

### 2. `cmd/infra_throughput` (new)

- Thin twin of `cmd/infra`.
- Default config path: `infra-config-throughput.yaml` (cwd).
- Same lifecycle: construct server, ListenAndServe, signal cleanup.
- No mode flags — config file is the switch.

### 3. Throughput Docker config

New file: `infra-config-docker-throughput.yaml`

Base: copy from `infra-config-docker.yaml` (testnet, postgres `host: db`, HTTP 8100, same services).

Overrides:

```yaml
fee_model:
  type: sat/kb
  value: 100

utxo_management:
  strategy: throughput
  throughput:
    expected_tx_size_bytes: 200
    expected_output_satoshis: 0
    denomination_satoshis: 0          # derive: ceil(200/1000*100)+0 = 20
    target_tps: 1000
    expected_confirmation_seconds: 300
    pool_headroom_factor: 1.5         # derived target_pool ≈ 450_000
    target_pool_size: 0
    low_water_percent: 60
    high_water_percent: 100
    spend_policy: prefer_mined
    pool_basket: fuel
    reserve_basket: reserve
    fanout_outputs_per_tx: 100
    fanout_max_txs_per_round: 12000   # ≥ 1000 × 10 × 1.2
    fanout_tree_depth: 2
    consolidation_inputs_per_tx: 1000
    top_up:
      enabled: true
      interval_seconds: 10
      start_immediately: true
```

Denomination invariant: derived 20 sats must exceed marginal fuel input fee at 100 sat/kb (`ceil(148/1000*100) = 15`). Validation already enforces this.

Optional local non-docker twin: `infra-config.example` remains generated from defaults (privacy); do not change generator defaults. Docker throughput file is hand-maintained like `infra-config-docker.yaml`.

### 4. `cmd/throughput_loadgen` (new)

Tiny Go program that:

1. Builds a wallet via remote storage client (`storage.NewClient(serverURL, userWallet)`).
2. Optionally bootstrap-funds via faucet internalize when `FAUCET_TXID` (or equivalent env) is set — reuse the BEEF fetch + `InternalizeAction` pattern from `examples/wallet_examples/internalize_tx_from_faucet` / `example_setup.InternalizeFromFaucet`.
3. Starts `fuelkeeper` with `fuelkeeper.FromThroughput(...)` using the **same** throughput numbers as the server config and the resolved denomination (must match server derivation).
4. After start (and optional warm-up wait), issues **1000 createAction requests per second**:
   - Rate limit with a ticker/token bucket (not a bare busy loop).
   - Concurrency: bounded worker pool so in-flight requests do not explode under latency (configurables via env with sane defaults, e.g. `TPS=1000`, `WORKERS=64`).
   - Each action has **exactly one** output: OP_RETURN locking script for `"the proof is in the pudding"` (`transaction.CreateOpReturnOutput`), `Satoshis: 0`.
5. Logs aggregate success/error rates periodically; exits cleanly on SIGINT/SIGTERM.

**Environment (loadgen):**

| Env | Default | Purpose |
|---|---|---|
| `SERVER_URL` | `http://infra:8100` | Remote storage endpoint |
| `BSV_NETWORK` | `test` | Network for wallet |
| `PRIVATE_KEY` | required | Operator wallet key (hex) |
| `TPS` | `1000` | Target createAction rate |
| `WORKERS` | `64` | Max concurrent createActions |
| `ORIGINATOR` | `throughput-loadgen.local` | createAction originator |
| `FAUCET_TXID` | empty | If set, fetch BEEF and internalize before load |
| `WARMUP_SECONDS` | `5` | Delay after keeper start before load |
| `DURATION_SECONDS` | `0` | If >0, stop after N seconds; 0 = run until signal |

Keys and secrets are never hardcoded in the image; compose passes them via env / `.env`.

### 5. Docker assets

#### Standard (existing, small edits)

- `Dockerfile` — continue building `./cmd/infra`.
- `docker-compose.yaml` — unchanged services; ensure mounted config has explicit privacy strategy.
- `infra-config-docker.yaml` — add:

```yaml
utxo_management:
  strategy: privacy
```

(full nested defaults optional; strategy alone is enough because code defaults fill throughput subsection when unused).

#### Throughput (new)

- `Dockerfile.throughput` multi-stage:
  - Build `cmd/infra_throughput` → `/app/infra`
  - Build `cmd/throughput_loadgen` → `/app/loadgen`
  - Runtime image can be multi-purpose: compose sets different `entrypoint`/`command` per service, **or** two final stages (`infra` / `loadgen`) from one builder. Prefer **one runtime image with two binaries** so compose is simple:

```dockerfile
# builder builds both
# runtime COPY both binaries + infra-config-docker-throughput.yaml as /app/infra-config-throughput.yaml
```

- `docker-compose.throughput.yaml`:
  - `db` — same postgres shape as standard compose
  - `infra` — build `Dockerfile.throughput`, command `/app/infra`, mount/bake throughput config, port 8100, depends on healthy db
  - `loadgen` — same image, command `/app/loadgen`, depends on infra, env for keys/TPS/SERVER_URL=`http://infra:8100`
  - Optional: document that operator supplies `PRIVATE_KEY` and optional `FAUCET_TXID` via `.env`

## Data flow (throughput live test)

1. Operator: `docker compose -f docker-compose.throughput.yaml up --build`
2. Infra starts with `strategy: throughput`, seeds fuel/reserve baskets for new users.
3. Loadgen starts wallet → optional faucet internalize into default basket → FuelKeeper fans default → reserve → fuel.
4. Loadgen fires 1000 createAction/s, each claiming fuel (exact-match path when pool healthy).
5. Operator observes logs/metrics; fuel runway gauges remain available if observability is enabled (optional in docker throughput config; not required for MVP).

## Error handling

| Failure | Behavior |
|---|---|
| Infra config invalid | Process exit non-zero at startup (existing Validate) |
| Loadgen missing `PRIVATE_KEY` | Fail fast with clear error |
| createAction errors under load | Count + log sample; do not crash process; keep rate limiter running |
| `ErrNotEnoughFunds` / contention | Counted as errors; FuelKeeper continues top-up |
| Infra unreachable at start | Retry with backoff for a short window, then exit |

## Testing

| Layer | What |
|---|---|
| Unit | Loadgen helpers: OP_RETURN script construction matches expected payload; rate-limiter fires ~TPS over a short window (table/unit tests in `cmd/throughput_loadgen` or a small internal package if needed for testability) |
| Build | `go build ./cmd/infra ./cmd/infra_throughput ./cmd/throughput_loadgen` |
| Config | Existing `defs.UTXOManagement.Validate` covers denomination/fanout identities for the live-test numbers (add a focused test case if not already covered for 200-byte / 1000 TPS shape) |
| Compose | Manual: `docker compose -f docker-compose.throughput.yaml config` validates YAML; full live 1000 TPS is operator-run, not CI-gated |

Do **not** require a full 1000 TPS integration test in CI (network, funds, duration). Prefer unit tests + build verification.

## File checklist

| Path | Action |
|---|---|
| `cmd/infra/main.go` | Keep as-is (or trivial comment only) |
| `cmd/infra_throughput/main.go` | Create |
| `cmd/throughput_loadgen/main.go` (+ small helpers if split) | Create |
| `infra-config-docker.yaml` | Add explicit `strategy: privacy` |
| `infra-config-docker-throughput.yaml` | Create |
| `Dockerfile` | Unchanged functionally |
| `Dockerfile.throughput` | Create |
| `docker-compose.yaml` | Unchanged unless privacy config needs mount tweak |
| `docker-compose.throughput.yaml` | Create |
| `pkg/defs/utxo_management_test.go` | Optional case for 200B / 1k TPS profile validation |
| README or short runbook snippet | Document how to run both stacks |

## Success criteria

1. `docker compose up --build` still runs the **privacy** standard stack.
2. `docker compose -f docker-compose.throughput.yaml up --build` runs throughput infra + loadgen.
3. Throughput config validates with derived denomination > marginal fee and fan-out capacity ≥ 1k TPS.
4. Loadgen issues createActions with a single OP_RETURN output containing exactly `the proof is in the pudding`.
5. Loadgen starts FuelKeeper and can optionally internalize a faucet tx before load.

## Implementation order

1. Standard docker config: explicit privacy strategy.
2. Throughput config YAML + `cmd/infra_throughput`.
3. `cmd/throughput_loadgen` (TDD for OP_RETURN + rate loop helpers).
4. `Dockerfile.throughput` + `docker-compose.throughput.yaml`.
5. Smoke build + config validation tests.
6. Short README/runbook notes for both modes.
