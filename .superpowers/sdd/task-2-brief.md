# Task 2 — Funding package harden

## Owns exclusively
`cmd/throughput_dashboard/internal/funding/**` only.

## Goal
Solid BRC-29 deposit address + internalize path for browser WalletClient top-ups.

## Requirements
1. Derive address with AnyoneKey sender + KeyID prefix `SfKxPIJNgdI=` / suffix `NaGLC6fMH50=`.
2. Mainnet vs testnet via `defs.BSVNetwork`.
3. Return locking script as **hex** (`hex.EncodeToString`) for WalletClient.
4. `Internalize`: prefer `atomic_tx_hex`; else `txid` + services.GetBEEF.
5. Remittance = AnyoneKey wallet-payment protocol (`sdk.InternalizeProtocolWalletPayment`).
6. Validate output pays operator address when parseable.
7. Unit tests for DeriveInfo + AnyonePaymentRemittance; add internalize unit tests with fakes if practical (no live network).
8. `go test ./cmd/throughput_dashboard/internal/funding/... -count=1` PASS.

## Baseline
Scaffold at `4758c367`. Harden/complete/tests.

## Commit
`feat(dashboard): harden funding address and internalize`

## Report
`.superpowers/sdd/task-2-report.md`
