# Task 2 Report — Funding package harden

**Status:** complete  
**Commit:** `49b4199c` — `feat(dashboard): harden funding address and internalize`  
**Baseline:** `4758c367` — chore(dashboard): baseline throughput demo scaffold  
**Branch:** `feat/throughput-demo-dashboard`  
**Exclusive scope:** `cmd/throughput_dashboard/internal/funding/**`

## Summary

Hardened the BRC-29 deposit address derivation and WalletClient internalize path for operator top-ups. Production API used by `internal/api` remains compatible (`Internalize` gains optional `...InternalizeOption` only).

## Requirements checklist

| # | Requirement | Result |
|---|-------------|--------|
| 1 | Derive address with AnyoneKey sender + KeyID prefix `SfKxPIJNgdI=` / suffix `NaGLC6fMH50=` | Pass — constants + `brc29.AddressForSelf(anyonePub, keyID, priv, …)` |
| 2 | Mainnet vs testnet via `defs.BSVNetwork` | Pass — `network.Validate()` + `WithMainNet` / `WithTestNet` |
| 3 | Return locking script as **hex** (`hex.EncodeToString`) | Pass — `Info.LockingScriptHex` |
| 4 | Prefer `atomic_tx_hex`; else `txid` + services.GetBEEF | Pass — atomic first; txid path via `AtomicBeefSource` (default = services.GetBEEF) |
| 5 | Remittance = AnyoneKey wallet-payment protocol | Pass — `sdk.InternalizeProtocolWalletPayment` + `AnyonePaymentRemittance()` |
| 6 | Validate output pays operator address when parseable | Pass — `validateOutputPaysAddress`; skip if unparseable |
| 7 | Unit tests for DeriveInfo + AnyonePaymentRemittance; internalize with fakes | Pass — see tests below |
| 8 | `go test ./cmd/throughput_dashboard/internal/funding/... -count=1` | **PASS** (19 tests) |

## Changes

### `address.go`
- Nil private-key guard
- `network.Validate()` before derivation
- Explicit switch on `defs.NetworkTestnet` vs mainnet
- Documented default suggested satoshis (`100_000`)
- Unchanged public constants and `Info` JSON shape for the dashboard UI

### `internalize.go`
- Nil wallet guard
- `AtomicBeefSource` interface + `WithAtomicBeefSource` option (test injection; production still uses `services.New` + `GetBEEF` + `AtomicBytes`)
- Prefers `atomic_tx_hex` over `txid` even when both present (asserted in tests)
- Logs resolved txid from parsed atomic when request omits `txid`
- Shared `parseTx` (BEEF then raw) for validation and logging
- Variadic `opts ...InternalizeOption` — **backward compatible** with existing API caller

### Tests
**`address_test.go` (7):**
- `TestDeriveInfoMainnet` — hex lock script, AnyoneKey sender, BRC-29 cross-check
- `TestDeriveInfoTestnet` — test network + address differs from mainnet
- `TestDeriveInfoDefaultSuggested` — zero → 100_000
- `TestDeriveInfoNilPriv` / `TestDeriveInfoInvalidNetwork`
- `TestAnyonePaymentRemittance` — decoded prefix/suffix + AnyoneKey
- `TestDerivationConstantsMatchFaucet` — `SfKxPIJNgdI=` / `NaGLC6fMH50=`

**`internalize_test.go` (12):**
- Missing tx fields, nil wallet, bad hex
- Atomic path success: protocol, remittance, originator, bytes
- Prefer atomic over txid (beef source not called)
- TxID path with fake `AtomicBeefSource`
- Beef source error propagation
- Wrong address rejection, output index OOR
- Skip validation when unparseable
- Wallet error wrap, invalid expected address

## Test command

```bash
go test ./cmd/throughput_dashboard/internal/funding/... -count=1
# ok  .../funding  ~0.7–0.9s  (19 tests)
```

## Concerns / follow-ups

1. **Live GetBEEF path** is not integration-tested (by design: no live network). Covered via injectable `AtomicBeefSource`. Mainnet reliability still depends on ARC/WoC credentials at runtime.
2. **Unparseable atomic** still reaches `InternalizeAction` when validation is skipped — intentional so partial BEEF shapes the wallet understands can proceed; bad data fails at the wallet.
3. **API package** (`handleInternalize`) was not modified (out of ownership). Existing call site continues to work without options.
4. Parallel Task 1/4 agents may land other package commits on the same branch; this commit only touches `funding/**`.

## Files touched

- `cmd/throughput_dashboard/internal/funding/address.go`
- `cmd/throughput_dashboard/internal/funding/address_test.go`
- `cmd/throughput_dashboard/internal/funding/internalize.go`
- `cmd/throughput_dashboard/internal/funding/internalize_test.go` (new)
