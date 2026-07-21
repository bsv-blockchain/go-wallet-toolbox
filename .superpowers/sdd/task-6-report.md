# Task 6 Report — Config + connect

## Status
**DONE**

## Commit
- `4aa3b631` — `feat(dashboard): config defaults and connect retries`

## Scope
`cmd/throughput_dashboard/internal/config/**` and `cmd/throughput_dashboard/internal/connect/**` only.

## What was verified / changed

### `config` (implementation already on branch; tests expanded)
- **ConfigFromEnv** mainnet-first defaults:
  - `BSV_NETWORK=main`
  - `SERVER_URL=http://127.0.0.1:8101`
  - `HTTP_ADDR=127.0.0.1:8200`
  - `TPS=10`, `WORKERS=8`, `SAMPLE_SECONDS=1`
  - `ORIGINATOR=throughput-dashboard.local`
- `PRIVATE_KEY` required
- Positive int validation for TPS / WORKERS / SAMPLE_SECONDS; `BSV_NETWORK` parsed via `defs.ParseBSVNetworkStr`
- **DemoThroughput()**: `ExpectedTxSizeBytes=200`, `ExpectedOutputSatoshis=0`, `TargetTPS=1000` on top of `DefaultUTXOManagement().Throughput`
- Denomination with `DefaultFeeModel()` (100 sat/kb) + `DefaultCommission()` (disabled) → **20 sats**

### `connect` (implementation already on branch; pure-helper tests added)
- `Wallet(...)`: `wallet.NewWithStorageFactory` + `storage.NewClient`; Balance probe as first storage RPC
- Retry window **30s**, exponential backoff 1s → max 5s; closes partial wallets between attempts; ctx cancel aborts
- Unit tests for `retryWithBackoff` only (no network): success after failures, window exhaust, cancel, onRetry callback

## Test summary
```text
go test ./cmd/throughput_dashboard/internal/config/... ./cmd/throughput_dashboard/internal/connect/... -count=1
```
**PASS** (config: 9 tests; connect: 4 tests). Also `-race` **PASS**.

## Requirements checklist
1. Mainnet defaults (SERVER_URL, BSV_NETWORK, HTTP_ADDR, TPS, WORKERS) — yes  
2. PRIVATE_KEY required — yes  
3. DemoThroughput shape + denom 20 — yes (asserted)  
4. Connect NewWithStorageFactory + NewClient + Balance retry ~30s — yes  
5. Config tests: defaults + overrides + DemoThroughput — yes (+ validation edges)  
6. Optional connect unit tests for pure helpers — yes (`retryWithBackoff`)  

## Concerns
- None for this package scope. Full wallet connect still needs live infra; only backoff helper is unit-tested (by design).
- `DemoThroughput` inherits other DefaultUTXOManagement throughput knobs (pool headroom, fanout, top-up interval); only size/output/TPS are overridden — matches mainnet yaml profile intent.
