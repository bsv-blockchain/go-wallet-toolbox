# Task 4 — Web UI polish

## Owns exclusively
`cmd/throughput_dashboard/web/**` only. No Go files.

## Goal
Polished demo UI for start/stop stream, gauges, charts, top-ups, WalletClient funding.

## Requirements
1. Start/Stop stream; TPS and workers inputs; POST `/api/stream/start|stop`.
2. Charts: TPS succeeded/failed and fuel/reserve (Chart.js CDN OK).
3. Balances from status/SSE ticks; mainnet warning banner.
4. Top-up event log from SSE `topup`.
5. Funding: load `/api/funding`, pay with `@bsv/sdk` WalletClient (esm.sh), POST `/api/funding/internalize` with atomic_tx_hex or txid.
6. Graceful errors if WalletClient/CDN missing.
7. Dark, readable demo aesthetic; mobile-ish responsive.

## Baseline
`index.html`, `app.js`, `style.css` at `4758c367`. Polish UX, robustness of funding parsing, accessibility of controls.

## Commit
`feat(dashboard): polish throughput demo web UI`

## Report
`.superpowers/sdd/task-4-report.md`
