# Task 3 — Metrics sampler + tests

## Owns exclusively
`cmd/throughput_dashboard/internal/metrics/**` only.

## Goal
Reliable sampling of stream TPS + basket inventory + top-up events; **unit tests** (currently missing).

## Requirements
1. Poll: default Balance, fuel/reserve ListOutputs TotalOutputs, stream Stats deltas.
2. Fuel runway_seconds ≈ fuel_count / target_tps when target_tps > 0.
3. Emit `tick` events each sample; `topup` when fuel or reserve count increases.
4. Event ring (max ~200) + Subscribe/Unsubscribe for SSE (non-blocking send).
5. **Add tests** with fake WalletAPI + controllable stream.Controller (or interface if you must inject — prefer fakes against existing types without changing other packages; if Controller is concrete, construct with fake ActionCreator).
6. Thread-safe.
7. `go test ./cmd/throughput_dashboard/internal/metrics/... -count=1` PASS.

## Baseline
`sampler.go` exists without tests at `4758c367`.

## Commit
`feat(dashboard): metrics sampler tests and harden`

## Report
`.superpowers/sdd/task-3-report.md`

## Note
Package currently imports `stream.Controller` concretely. You may only edit metrics package — work with exported stream APIs. If a pure interface would help but requires stream changes, document as concern and use fake ActionCreator + real Controller for tests instead.
