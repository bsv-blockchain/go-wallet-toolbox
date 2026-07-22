# Task 7 Report — API + main wiring + API tests

## Status
**PASS**

## What was done

### Verified existing control plane (no production edits required)
- `cmd/throughput_dashboard/main.go` already wires:
  - `config.ConfigFromEnv` + `DemoThroughput` denomination/pool
  - `connect.Wallet` → operator wallet
  - `fuelkeeper.New` + `keeper.Run` (background)
  - `stream.NewController`
  - `metrics.NewSampler` + `sampler.Run` (background)
  - `//go:embed web/*` + `api.New` + `http.Server`
  - graceful shutdown: stream `Stop` + HTTP `Shutdown` on SIGINT/SIGTERM
- `cmd/throughput_dashboard/internal/api/server.go` already exposes:
  - `GET /api/status`
  - `POST /api/stream/start` / `POST /api/stream/stop`
  - `GET /api/events` (SSE)
  - `GET /api/funding`
  - `POST /api/funding/internalize`
  - static `GET /` from embedded `WebFS`
  - CORS preflight

### Funding.Internalize adaptation
Current signature (Task 2) matches the call site already in `handleInternalize`:

```go
funding.Internalize(ctx, wallet, network, expectedAddress, req, originator, logger, opts...)
```

No api package changes required.

### Added HTTP handler tests
New file: `cmd/throughput_dashboard/internal/api/server_test.go`

| Test | Coverage |
|------|----------|
| `TestStatus_JSONShape` | network/mainnet/server_url/originator/tick/events + CORS header |
| `TestStatus_IncludesTickAfterSample` | tick gauges after sampler `Run` |
| `TestStreamStartStop_StatusRunning` | start with tps/workers, double-start 409, stop, stop-idempotent |
| `TestStreamStart_EmptyBodyUsesDefaults` | empty body keeps controller defaults |
| `TestFunding_ReturnsDepositAddress` | BRC-29 deposit JSON shape |
| `TestInternalize_AtomicPath` | atomic_tx_hex → wallet InternalizeAction |
| `TestInternalize_InvalidJSON` | 400 |
| `TestInternalize_MissingFields` | 400 when neither atomic nor txid |
| `TestCORS_Options` | OPTIONS → 204 |
| `TestStatic_Index` | embedded-style MapFS index |
| `TestEvents_SSEHeadersAndInitialTick` | `text/event-stream`, initial tick event, client cancel exit |

Fakes: `stream.ActionCreator`, `metrics.WalletAPI`, `funding.ActionInternalizer`; real `stream.Controller` + `metrics.Sampler` under `httptest`.

## Commits
- `feat(dashboard): API control plane and main wiring tests` (this task)

## Test summary
```
go test ./cmd/throughput_dashboard/... -count=1
  ok  .../internal/api
  ok  .../internal/config
  ok  .../internal/funding
  ok  .../internal/metrics
  ok  .../internal/stream

go build ./cmd/throughput_dashboard  → PASS
```

## Concerns
1. **No main.go unit tests** — wiring is integration-level (needs PRIVATE_KEY + live storage). Covered by compile + structural review of `run()`.
2. **SSE test uses httptest** — adequate for headers/first event/cancel; not a long-lived multi-subscriber stress test.
3. **Internalize test uses unparseable atomic hex** — exercises wallet path after decode (validation skipped when unparseable); address-strict path remains covered in `funding` package tests.
4. **Task 6 (config/connect)** was still open in progress ledger when this ran; api/main already consume those packages as libraries.
