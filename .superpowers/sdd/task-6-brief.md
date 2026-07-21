# Task 6 — Config + connect

## Owns exclusively
- `cmd/throughput_dashboard/internal/config/**`
- `cmd/throughput_dashboard/internal/connect/**`

## Requirements
1. Env config mainnet defaults: BSV_NETWORK=main, SERVER_URL http://127.0.0.1:8101, HTTP_ADDR 127.0.0.1:8200, TPS=10, WORKERS=8
2. PRIVATE_KEY required
3. `DemoThroughput()` → expected_tx_size 200, output sats 0, target_tps 1000; denom 20 with DefaultFeeModel+DefaultCommission
4. Connect: wallet.NewWithStorageFactory + storage.NewClient; retry Balance probe with backoff (~30s window)
5. Config tests cover defaults + overrides + DemoThroughput
6. Optional connect unit test only if pure helpers are extractable without network

## Done
`go test ./cmd/throughput_dashboard/internal/config/... ./cmd/throughput_dashboard/internal/connect/... -count=1` PASS
Commit: `feat(dashboard): config defaults and connect retries`
Report: `.superpowers/sdd/task-6-report.md`
