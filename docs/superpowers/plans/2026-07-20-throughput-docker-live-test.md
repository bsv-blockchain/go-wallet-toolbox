# Throughput Docker Live-Test Stack Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a privacy (standard) Docker/cmd path with explicit privacy UTXO strategy and a throughput Docker/cmd path sized for 1000 TPS OP_RETURN createActions, including a loadgen binary that uses the wallet client, FuelKeeper, and optional faucet bootstrap.

**Architecture:** Two thin server mains (`cmd/infra`, `cmd/infra_throughput`) share `infra.NewServer` and differ only by default config path. Throughput Docker config sets `strategy: throughput` with `expected_tx_size_bytes: 200`, fee `100 sat/kb`, `target_tps: 1000`. A separate `cmd/throughput_loadgen` connects via `storage.NewClient`, optionally internalizes a faucet tx, runs FuelKeeper, and issues rate-limited createActions with one OP_RETURN output (`"the proof is in the pudding"`). `Dockerfile.throughput` builds both throughput binaries; `docker-compose.throughput.yaml` runs db + infra + loadgen.

**Tech Stack:** Go 1.26, Docker multi-stage alpine, docker compose, go-sdk wallet + transaction OP_RETURN helpers, existing `pkg/infra`, `pkg/wallet`, `pkg/wallet/fuelkeeper`, `pkg/storage` remote client.

## Global Constraints

- Work exclusively in the git worktree at `.worktrees/throughput-docker-live-test` on branch `feat/throughput-docker-live-test`.
- OP_RETURN payload must be exactly `the proof is in the pudding` (no extra punctuation/casing changes).
- Throughput profile: `expected_tx_size_bytes: 200`, `expected_output_satoshis: 0`, fee model `sat/kb` value `100`, `target_tps: 1000`.
- Standard path must use `utxo_management.strategy: privacy` explicitly in docker config.
- Derived denomination for the live-test profile must be `20` sats and must validate (`> MarginalFuelInputFee` of `15` at 100 sat/kb).
- Fan-out capacity must satisfy sustained-throughput identity at 1000 TPS (`fanout_outputs_per_tx × fanout_max_txs_per_round >= target_tps × top_up.interval_seconds × 1.2`).
- No hardcoded private keys or faucet secrets in source or images; env vars only.
- Do not change funder/FuelKeeper algorithms; packaging and loadgen only.
- Follow existing code style; use `testify/require` for tests; packages under `cmd/` are `package main`.
- Every task ends with a commit on `feat/throughput-docker-live-test`.

## File map

| Path | Responsibility |
|---|---|
| `infra-config-docker.yaml` | Standard docker config; explicit privacy strategy |
| `infra-config-docker-throughput.yaml` | Throughput live-test server config |
| `cmd/infra/main.go` | Unchanged privacy/standard server entry (loads `infra-config.yaml`) |
| `cmd/infra_throughput/main.go` | Throughput server entry (loads `infra-config-throughput.yaml`) |
| `cmd/throughput_loadgen/opreturn.go` | Build OP_RETURN locking script for fixed payload |
| `cmd/throughput_loadgen/opreturn_test.go` | Unit tests for OP_RETURN payload |
| `cmd/throughput_loadgen/config.go` | Env-based loadgen config |
| `cmd/throughput_loadgen/config_test.go` | Config parsing tests |
| `cmd/throughput_loadgen/runner.go` | Rate-limited createAction loop + stats |
| `cmd/throughput_loadgen/runner_test.go` | Rate loop unit tests with fake wallet |
| `cmd/throughput_loadgen/main.go` | Wire wallet, optional faucet, FuelKeeper, runner |
| `pkg/defs/utxo_management_test.go` | Add live-test profile validation case |
| `Dockerfile.throughput` | Build infra_throughput + loadgen |
| `docker-compose.throughput.yaml` | db + infra + loadgen stack |
| `docs/throughput-docker.md` | Short runbook for both stacks |

---

### Task 1: Explicit privacy in standard docker config + live-test profile validation test

**Files:**
- Modify: `infra-config-docker.yaml`
- Modify: `pkg/defs/utxo_management_test.go`
- Test: `pkg/defs/utxo_management_test.go`

**Interfaces:**
- Consumes: `defs.UTXOManagement.Validate`, `defs.Throughput.Denomination`, `defs.DefaultUTXOManagement`
- Produces: docker privacy strategy string `privacy`; test covering denomination `20` and Validate success for 200B/1000 TPS profile

- [ ] **Step 1: Write the failing validation test for the live-test profile**

Append to `pkg/defs/utxo_management_test.go`:

```go
func TestLiveTestThroughputProfile(t *testing.T) {
	cfg := defs.DefaultUTXOManagement()
	cfg.Strategy = defs.StrategyThroughput
	cfg.Throughput.ExpectedTxSizeBytes = 200
	cfg.Throughput.ExpectedOutputSatoshis = 0
	cfg.Throughput.TargetTPS = 1000
	// Keep other defaults from DefaultUTXOManagement (fanout, water marks, baskets, top_up).

	fee := feeModel(100)
	commission := defs.DefaultCommission()

	denom, err := cfg.Throughput.Denomination(fee, commission)
	require.NoError(t, err)
	require.Equal(t, uint64(20), denom)
	require.Greater(t, denom, defs.MarginalFuelInputFee(fee))

	require.NoError(t, cfg.Validate(fee, commission))
	require.Equal(t, uint64(450_000), cfg.Throughput.TargetPool()) // 1000 * 300 * 1.5
}
```

- [ ] **Step 2: Run test to verify it fails or passes**

Run: `go test ./pkg/defs/ -run TestLiveTestThroughputProfile -count=1`

If it fails, fix only test arithmetic or missing fields so validation matches current `Validate` rules (defaults already satisfy fan-out capacity: `100 * 12000 >= 1000 * 10 * 1.2`).

Expected: PASS once test matches real validation (no production code change required for this case if defaults are correct).

- [ ] **Step 3: Add explicit privacy strategy to docker config**

In `infra-config-docker.yaml`, after the `fee_model` block (or near other top-level keys), add:

```yaml
# --- UTXO management (standard / privacy) ---
utxo_management:
  strategy: privacy
```

Do not remove other sections. Do not set throughput overrides in the privacy file.

- [ ] **Step 4: Re-run tests**

Run: `go test ./pkg/defs/ -run 'TestLiveTest|TestDenomination|TestMarginal' -count=1`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add infra-config-docker.yaml pkg/defs/utxo_management_test.go
git commit -m "feat(docker): explicit privacy UTXO strategy and live-test profile test"
```

---

### Task 2: Throughput docker config + `cmd/infra_throughput`

**Files:**
- Create: `infra-config-docker-throughput.yaml`
- Create: `cmd/infra_throughput/main.go`

**Interfaces:**
- Consumes: `infra.NewServer`, `infra.WithConfigFile`
- Produces: binary entry that loads `infra-config-throughput.yaml` by default; docker config file content with locked live-test numbers

- [ ] **Step 1: Create throughput docker config**

Create `infra-config-docker-throughput.yaml` by copying `infra-config-docker.yaml` and applying these exact overrides (keep db/http/services/monitor blocks from the privacy docker file, including `server_private_key` and `bsv_network: test`):

```yaml
fee_model:
  type: sat/kb
  value: 100

utxo_management:
  strategy: throughput
  throughput:
    expected_tx_size_bytes: 200
    expected_output_satoshis: 0
    denomination_satoshis: 0
    target_tps: 1000
    expected_confirmation_seconds: 300
    pool_headroom_factor: 1.5
    target_pool_size: 0
    low_water_percent: 60
    high_water_percent: 100
    spend_policy: prefer_mined
    pool_basket: fuel
    reserve_basket: reserve
    fanout_outputs_per_tx: 100
    fanout_max_txs_per_round: 12000
    fanout_tree_depth: 2
    consolidation_inputs_per_tx: 1000
    top_up:
      enabled: true
      interval_seconds: 10
      start_immediately: true
```

Header comment at top of file:

```yaml
# =============================================================================
# Docker Compose configuration for throughput live-test infra server
# =============================================================================
# Usage:
#   docker compose -f docker-compose.throughput.yaml up -d --build
#
# Pairs with Dockerfile.throughput and cmd/throughput_loadgen.
# =============================================================================
```

Also set:

```yaml
name: go-storage-server-throughput
```

- [ ] **Step 2: Create `cmd/infra_throughput/main.go`**

```go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/infra"
)

func main() {
	server, err := infra.NewServer(
		context.Background(),
		infra.WithConfigFile("infra-config-throughput.yaml"),
	)
	if err != nil {
		panic(err)
	}

	go func() {
		if err = server.ListenAndServe(context.Background()); err != nil {
			panic(err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	server.Cleanup()
}
```

- [ ] **Step 3: Build the binary**

Run: `go build -o /tmp/infra_throughput ./cmd/infra_throughput`

Expected: exit 0, no compile errors.

- [ ] **Step 4: Commit**

```bash
git add infra-config-docker-throughput.yaml cmd/infra_throughput/main.go
git commit -m "feat(infra): add throughput server cmd and docker config"
```

---

### Task 3: Loadgen OP_RETURN helper + config (TDD)

**Files:**
- Create: `cmd/throughput_loadgen/opreturn.go`
- Create: `cmd/throughput_loadgen/opreturn_test.go`
- Create: `cmd/throughput_loadgen/config.go`
- Create: `cmd/throughput_loadgen/config_test.go`

**Interfaces:**
- Produces:
  - `const ProofPayload = "the proof is in the pudding"`
  - `func OpReturnLockingScript(payload string) ([]byte, error)`
  - `type Config struct { ServerURL, Network, PrivateKeyHex, Originator, FaucetTxID string; TPS, Workers, WarmupSeconds, DurationSeconds int }`
  - `func ConfigFromEnv() (Config, error)` — reads env vars listed below

Env map (exact names):

| Env | Default |
|---|---|
| `SERVER_URL` | `http://infra:8100` |
| `BSV_NETWORK` | `test` |
| `PRIVATE_KEY` | required (error if empty) |
| `TPS` | `1000` |
| `WORKERS` | `64` |
| `ORIGINATOR` | `throughput-loadgen.local` |
| `FAUCET_TXID` | `""` |
| `WARMUP_SECONDS` | `5` |
| `DURATION_SECONDS` | `0` |

- [ ] **Step 1: Write failing OP_RETURN tests**

`cmd/throughput_loadgen/opreturn_test.go`:

```go
package main

import (
	"testing"

	"github.com/bsv-blockchain/go-sdk/script"
	"github.com/stretchr/testify/require"
)

func TestOpReturnLockingScriptContainsPayload(t *testing.T) {
	locking, err := OpReturnLockingScript(ProofPayload)
	require.NoError(t, err)
	require.NotEmpty(t, locking)

	s := script.NewFromBytes(locking)
	// OP_FALSE OP_RETURN <payload>
	asm, err := s.ToASM()
	require.NoError(t, err)
	require.Contains(t, asm, "OP_RETURN")
	// payload appears as hex in ASM for pushdata — also check raw contains bytes
	require.Contains(t, string(locking), ProofPayload)
	require.Equal(t, "the proof is in the pudding", ProofPayload)
}

func TestOpReturnLockingScriptRejectsEmpty(t *testing.T) {
	_, err := OpReturnLockingScript("")
	require.Error(t, err)
}
```

If `script.NewFromBytes` / `ToASM` APIs differ in this SDK version, adjust assertions but keep: non-empty script, error on empty payload, exact `ProofPayload` constant.

- [ ] **Step 2: Run tests — expect fail**

Run: `go test ./cmd/throughput_loadgen/ -count=1`

Expected: FAIL (undefined symbols)

- [ ] **Step 3: Implement OP_RETURN helper**

`cmd/throughput_loadgen/opreturn.go`:

```go
package main

import (
	"fmt"

	"github.com/bsv-blockchain/go-sdk/transaction"
)

// ProofPayload is the fixed OP_RETURN message for live throughput tests.
const ProofPayload = "the proof is in the pudding"

// OpReturnLockingScript builds a standard OP_RETURN locking script for payload.
func OpReturnLockingScript(payload string) ([]byte, error) {
	if payload == "" {
		return nil, fmt.Errorf("opreturn payload must be non-empty")
	}
	out, err := transaction.CreateOpReturnOutput([][]byte{[]byte(payload)})
	if err != nil {
		return nil, fmt.Errorf("create opreturn output: %w", err)
	}
	return out.LockingScript.Bytes(), nil
}
```

- [ ] **Step 4: Write config tests + implement config**

`config_test.go` — use `t.Setenv` to set/clear vars, call `ConfigFromEnv()`, assert defaults and required PRIVATE_KEY error.

`config.go` — parse ints with `strconv.Atoi`, apply defaults when unset/empty, return error if `PRIVATE_KEY` empty or TPS/Workers <= 0.

- [ ] **Step 5: Run tests — expect pass**

Run: `go test ./cmd/throughput_loadgen/ -count=1`

Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/throughput_loadgen/
git commit -m "feat(loadgen): OP_RETURN helper and env config"
```

---

### Task 4: Loadgen runner + main (wallet client, FuelKeeper, faucet, rate loop)

**Files:**
- Create: `cmd/throughput_loadgen/runner.go`
- Create: `cmd/throughput_loadgen/runner_test.go`
- Create: `cmd/throughput_loadgen/main.go`
- Create: `cmd/throughput_loadgen/faucet.go` (optional bootstrap)

**Interfaces:**
- Consumes: `OpReturnLockingScript`, `Config`, `wallet.NewWithStorageFactory`, `storage.NewClient`, `fuelkeeper.New`, `fuelkeeper.FromThroughput`, `defs.DefaultUTXOManagement` (override fields to match server profile), `services.New` + `GetBEEF` for faucet path
- Produces:
  - `type ActionCreator interface { CreateAction(ctx context.Context, args sdk.CreateActionArgs, originator string) (*sdk.CreateActionResult, error) }`
  - `func RunLoad(ctx context.Context, w ActionCreator, cfg Config, lockingScript []byte) Stats`
  - `type Stats struct { Attempted, Succeeded, Failed uint64 }`

- [ ] **Step 1: Write runner test with fake wallet**

Fake implements `ActionCreator`, sleeps ~1ms, returns success. Configure `TPS=100`, `Workers=10`, `DurationSeconds=1`. Assert `Attempted` is roughly 80–150 (rate limiter window), `Succeeded == Attempted` if fake never fails.

- [ ] **Step 2: Implement runner**

```go
// Pseudocode structure — implement fully:
// - ticker every time.Second/TPS OR token bucket refill
// - worker pool size = cfg.Workers
// - each job: CreateAction with one output (LockingScript, Satoshis:0, OutputDescription:"throughput loadgen opreturn")
// - Description: "throughput loadgen"
// - Options: AcceptDelayedBroadcast true (prefer delayed under load)
// - atomic counters for stats
// - log every 1s: attempted/succeeded/failed + instantaneous rate
// - stop on ctx.Done() or DurationSeconds
```

Use `golang.org/x/time/rate` if already in go.mod; otherwise a simple ticker + buffered job channel is fine. Prefer dependencies already in the module.

- [ ] **Step 3: Implement faucet bootstrap**

When `cfg.FaucetTxID != ""`:
1. Parse network, create `services.New(logger, defs.DefaultServicesConfig(network))`
2. `GetBEEF(ctx, txid, nil)` → atomic bytes for that txid
3. `InternalizeAction` with wallet-payment protocol on output 0  
   Reuse derivation pattern from `examples/internal/example_setup/internalize_tx.go` (`utils.DerivationParts` is under examples — **do not import examples from cmd**).  
   Inline a minimal wallet-payment remittance using the same approach as production code, or copy the small remittance construction without importing `examples/...`.

If faucet bootstrap fails, return error from main (fail fast).

- [ ] **Step 4: Implement main wiring**

```go
// main flow:
// 1. ConfigFromEnv()
// 2. parse PRIVATE_KEY → ec.PrivateKeyFromHex
// 3. parse BSV_NETWORK
// 4. wallet.NewWithStorageFactory(network, priv, func(userWallet sdk.Interface) (wdk.WalletStorageProvider, func(), error) {
//        return storage.NewClient(cfg.ServerURL, userWallet)
//    })
// 5. optional faucet
// 6. build throughput defs matching server (ExpectedTxSizeBytes:200, TargetTPS:1000, ...)
// 7. denom, err := throughput.Denomination(defs.DefaultFeeModel(), defs.DefaultCommission())
// 8. fuelkeeper.New(wallet, fuelkeeper.FromThroughput(throughput, denom), logger); go keeper.Run(ctx)
// 9. sleep WarmupSeconds
// 10. locking := OpReturnLockingScript(ProofPayload)
// 11. RunLoad(...)
// 12. log final Stats; exit 0
```

Signal handling: `signal.NotifyContext` for SIGINT/SIGTERM.

- [ ] **Step 5: Build + unit tests**

Run:

```bash
go test ./cmd/throughput_loadgen/ -count=1
go build -o /tmp/throughput_loadgen ./cmd/throughput_loadgen
```

Expected: PASS and successful build.

- [ ] **Step 6: Commit**

```bash
git add cmd/throughput_loadgen/
git commit -m "feat(loadgen): rate-limited createAction runner with FuelKeeper"
```

---

### Task 5: Dockerfile.throughput + docker-compose.throughput.yaml + runbook

**Files:**
- Create: `Dockerfile.throughput`
- Create: `docker-compose.throughput.yaml`
- Create: `docs/throughput-docker.md`
- Modify: `README.md` (one short section or link only — do not rewrite README)

**Interfaces:**
- Consumes: binaries from Tasks 2–4; `infra-config-docker-throughput.yaml`
- Produces: compose stack with services `db`, `infra`, `loadgen`

- [ ] **Step 1: Create `Dockerfile.throughput`**

Mirror root `Dockerfile` builder pattern (golang:1.26-alpine → alpine:3.20):

```dockerfile
# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS builder
RUN apk add --no-cache git ca-certificates tzdata
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/infra ./cmd/infra_throughput
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/loadgen ./cmd/throughput_loadgen

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata curl postgresql-client
WORKDIR /app
COPY --from=builder /out/infra /app/infra
COPY --from=builder /out/loadgen /app/loadgen
COPY infra-config-docker-throughput.yaml /app/infra-config-throughput.yaml
RUN mkdir -p /app/data
EXPOSE 8100
# default entry is infra; loadgen service overrides command
ENTRYPOINT ["/app/infra"]
```

- [ ] **Step 2: Create `docker-compose.throughput.yaml`**

```yaml
services:
  db:
    image: postgres:17-alpine
    volumes:
      - db-data-throughput:/var/lib/postgresql/data:Z
    environment:
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=${POSTGRES_PASSWORD:-postgres}
      - POSTGRES_DB=storage
    ports:
      - "127.0.0.1:5433:5432"   # 5433 host to avoid clash with standard stack
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U postgres -d storage"]
      interval: 5s
      timeout: 5s
      retries: 5
      start_period: 10s

  infra:
    build:
      context: .
      dockerfile: Dockerfile.throughput
    depends_on:
      db:
        condition: service_healthy
    ports:
      - "8101:8100"             # 8101 host to avoid clash with standard stack
    volumes:
      - ./infra-config-docker-throughput.yaml:/app/infra-config-throughput.yaml:ro
    restart: unless-stopped

  loadgen:
    build:
      context: .
      dockerfile: Dockerfile.throughput
    depends_on:
      - infra
    entrypoint: ["/app/loadgen"]
    environment:
      - SERVER_URL=http://infra:8100
      - BSV_NETWORK=test
      - PRIVATE_KEY=${PRIVATE_KEY:?set PRIVATE_KEY for loadgen}
      - TPS=${TPS:-1000}
      - WORKERS=${WORKERS:-64}
      - FAUCET_TXID=${FAUCET_TXID:-}
      - WARMUP_SECONDS=${WARMUP_SECONDS:-5}
      - DURATION_SECONDS=${DURATION_SECONDS:-0}
    restart: "no"

volumes:
  db-data-throughput:
    driver: local
```

- [ ] **Step 3: Write `docs/throughput-docker.md`**

Document:

1. Standard stack: `docker compose up -d --build` (privacy).
2. Throughput stack: export `PRIVATE_KEY=...`, optional `FAUCET_TXID=...`, then `docker compose -f docker-compose.throughput.yaml up --build`.
3. Ports: standard 8100/5432; throughput 8101/5433.
4. OP_RETURN payload and TPS targets.
5. Note that loadgen needs funded wallet + FuelKeeper warm-up before sustained 1000 TPS.

- [ ] **Step 4: Link from README**

Add a short bullet under Docker/examples section pointing to `docs/throughput-docker.md`. Keep change minimal.

- [ ] **Step 5: Validate compose file + rebuild binaries**

Run:

```bash
docker compose -f docker-compose.throughput.yaml config >/dev/null
go build -o /tmp/infra ./cmd/infra
go build -o /tmp/infra_throughput ./cmd/infra_throughput
go build -o /tmp/loadgen ./cmd/throughput_loadgen
go test ./cmd/throughput_loadgen/ ./pkg/defs/ -run 'TestLiveTest|TestOpReturn|TestConfig|TestRunLoad' -count=1
```

Expected: all succeed. (`docker compose config` requires docker CLI; if unavailable, note in report and still pass binary builds/tests.)

- [ ] **Step 6: Commit**

```bash
git add Dockerfile.throughput docker-compose.throughput.yaml docs/throughput-docker.md README.md
git commit -m "feat(docker): throughput compose stack and loadgen image"
```

---

## Spec coverage checklist

| Spec requirement | Task |
|---|---|
| Explicit privacy in standard docker config | 1 |
| `cmd/infra` remains standard | 1–2 (unchanged) |
| `cmd/infra_throughput` | 2 |
| Throughput config 200B / 100 sat/kb / 1000 TPS | 2 + 1 validation test |
| `cmd/throughput_loadgen` @ 1000 TPS | 3–4 |
| OP_RETURN `"the proof is in the pudding"` | 3–4 |
| FuelKeeper in loadgen | 4 |
| Optional faucet bootstrap | 4 |
| `Dockerfile.throughput` + compose | 5 |
| Runbook | 5 |

## Execution notes for SDD

- Run all work in: `/Users/personal/git/go/go-wallet-toolbox/.worktrees/throughput-docker-live-test`
- After each task: commit exists, tests for that task pass, reviewer gates Critical/Important findings before next task.
- Final whole-branch review against merge-base `main`.
