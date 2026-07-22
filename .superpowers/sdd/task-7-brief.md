# Task 7 — API + main wiring + API tests

## Owns exclusively
- `cmd/throughput_dashboard/internal/api/**`
- `cmd/throughput_dashboard/main.go`

Do not edit stream/funding/metrics/config/connect/web packages (consume as libraries).

## Requirements
1. Routes: GET /api/status, POST /api/stream/start|stop, GET /api/events (SSE), GET /api/funding, POST /api/funding/internalize, static embed /
2. main wires: config, connect wallet, FuelKeeper, stream.Controller, metrics.Sampler, api.Server, graceful shutdown
3. //go:embed web/*
4. **HTTP tests** for at least start/stop/status JSON (use fakes/interfaces or httptest for handler)
5. If funding Internalize signature changed in Task 2, adapt call sites in api only
6. `go test ./cmd/throughput_dashboard/... -count=1` PASS
7. `go build ./cmd/throughput_dashboard` PASS

## Commit
`feat(dashboard): API control plane and main wiring tests`

## Report
`.superpowers/sdd/task-7-report.md`
