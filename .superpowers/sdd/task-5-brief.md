# Task 5 — Packaging + docs

## Owns exclusively
- `Dockerfile.dashboard`
- `docker-compose.throughput-dashboard.yaml`
- `infra-config-docker-throughput-mainnet.yaml`
- `docs/throughput-dashboard.md`
- Cross-links only: `docs/throughput-docker.md`, `README.md` (docs list line only)

Do not edit `cmd/throughput_dashboard/**`.

## Goal
Mainnet demo stack packaging + operator runbook ready to run.

## Requirements
1. Mainnet infra yaml: `bsv_network: main`, throughput strategy, expected_tx_size 200, target_tps 1000, fee 100 sat/kb.
2. Compose: db (5433 host), infra (8101 host), dashboard (127.0.0.1:8200); PRIVATE_KEY required.
3. Dockerfile builds `infra_throughput` + `throughput_dashboard`.
4. Runbook: fund → FuelKeeper → start stream; safety checklist.
5. Validate: `PRIVATE_KEY=00 docker compose -f docker-compose.throughput-dashboard.yaml config` exits 0.

## Baseline
Files exist at `4758c367`. Verify completeness, fix gaps, improve docs clarity.

## Commit
`docs(dashboard): mainnet compose packaging and runbook`

## Report
`.superpowers/sdd/task-5-report.md`
