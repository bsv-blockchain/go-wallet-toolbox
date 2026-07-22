# Task 5 Report — Packaging + docs

## status
**done**

## commits
- `c1925196` — `docs(dashboard): mainnet compose packaging and runbook`

## validation result
- `PRIVATE_KEY=00 docker compose -f docker-compose.throughput-dashboard.yaml config` → **exit 0**
- Without `PRIVATE_KEY` → exit 1 (required-variable interpolation)
- Asserts: `bsv_network: main`, strategy `throughput`, `expected_tx_size_bytes: 200`, `target_tps: 1000`, fee `sat/kb` 100
- Host ports: db `127.0.0.1:5433`, infra `127.0.0.1:8101`, dashboard `127.0.0.1:8200`
- Dockerfile builds `./cmd/infra_throughput` + `./cmd/throughput_dashboard`

## concerns
- ARC token in mainnet yaml remains `mainnet_placeholder`; operators must set `INFRA_WALLET_SERVICES_ARC_TOKEN` for reliable mainnet broadcast (documented in runbook).
- Did not build/run full image stack (compose config-only validation as required).
- Infra host publish was tightened to `127.0.0.1:8101` (baseline was `8101:8100`); matches dashboard/db localhost-only safety. Intra-compose traffic still uses `http://infra:8100`.
