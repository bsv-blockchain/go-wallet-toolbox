# Aborted Transaction Status — Design

**Date:** 2026-07-20
**Branch:** `feat/aborted-tx-status-959` (base: `main` @ `5cd687e5`)
**Issue:** [#959 — Ambiguous status: failed broadcast or cancelled ctx?](https://github.com/bsv-blockchain/go-wallet-toolbox/issues/959)
**Sources:** issue #959, PR #929 (immediate input release on pre-broadcast abort), diagnostic sweep of `pkg/wdk`, `pkg/storage/internal/actions`, `pkg/monitor`, `pkg/wallet`, `conformance/`.

## Problem

A transaction can leave `createAction`/`processAction` in the terminal status
`failed` for two semantically opposite reasons that are today indistinguishable:

1. **Aborted before broadcast** — ctx cancellation (upstream HTTP timeout), a
   token-swap detail fetch timing out, or the Monitor `fail_abandoned` sweep. The
   transaction **never reached a broadcast endpoint**; its inputs are released
   immediately (PR #929) and it is safe to rebuild and retry.
2. **Broadcast rejection** — the transaction **was posted** and rejected
   (double spend / invalidTx). Retrying is pointless; this is permanent.

Both write the single value `wdk.TxStatusFailed = "failed"` onto the per-user
`Transaction` row, which carries **no** reason/kind column. The only discriminator
(the shared `KnownTx.Status` = `doubleSpend`/`invalidTx`) is deliberately **not**
set on the abort path (`abort.go:136`) and is never surfaced through the listing
APIs. The Enterprise Wallet labels transactions with an idempotency-id and, on
retry, looks up a prior attempt by that label — but it cannot tell a retryable
abort from a permanent rejection, so it cannot decide between "rebuild & rebroadcast"
and "report permanent failure".

### Convergence (verified)

| Path | Write site | Class |
|---|---|---|
| ctx-cancel / token-swap / build-abort (PR #929) | `process.go:935` `abortTxByStringID` | abort → retryable |
| user `AbortAction` + Monitor `fail_abandoned` sweep | `abort.go:131` `abortTx` (shared) | abort → retryable |
| broadcast double spend | `process.go:878` `singleTxBroadcastResult` | rejection → permanent |
| broadcast invalidTx | `process.go:886` `singleTxBroadcastResult` | rejection → permanent |
| Arcade SSE confirmed rejection | `known_tx.go:150` `FailKnownTxAsDoubleSpend` | rejection → permanent |
| Monitor reconciliation cascade | `synchronize_tx_statuses.go:498` | rejection → permanent |
| UnFail re-check no-proof | `process_unfail.go:114` `markAsInvalid` | rejection → permanent |

## Goal

Add a distinct terminal transaction status `aborted` written **only** by the
pre-broadcast abort paths, leaving every broadcast-rejection path on `failed`. This
single divergence lets the Enterprise Wallet's idempotency-label lookup read the raw
status and branch retryable-vs-permanent. Go-only for this PR; TypeScript upstream
(`ts-stack`) parity is a coordinated follow-up.

## Decisions (settled during brainstorming)

- **D1 — Read path:** the Enterprise Wallet reads the raw status string via the
  toolbox failed-actions listing surface (`ListFailedActions` / the `unfail`
  magic-label path), **not** a new BRC-100 `ActionStatus` value. `aborted` therefore
  flows through as a raw string; the failed-family query bucket is widened to carry it.
- **D2 — TS parity:** ship Go-only now; file a `ts-stack` follow-up to add `aborted`
  to the TS `TransactionStatus` union plus a conformance vector. `TxStatus` crosses
  the BRC-40 sync wire verbatim, so this is tracked, not skipped.
- **D3 — Abort scope:** **all** pre-broadcast aborts become `aborted` — the automatic
  ctx-cancel path (`process.go:935`), the user-initiated `AbortAction`, and the
  Monitor `fail_abandoned` sweep. The latter two share the single `abortTx` choke
  point, so both flip with one write-site change. All three are pre-broadcast and
  retryable, so a uniform status is correct and requires no `abortTx` status-param split.

## Design

### 1. New status value

`pkg/wdk/tx_status.go`:

```go
TxStatusAborted TxStatus = "aborted"
```

No DB migration: `Transaction.Status` is an unconstrained free-form string column
(`models/transaction.go:13`, GORM `AutoMigrate`, no CHECK/enum on SQLite/Postgres/MySQL).
Existing `failed` rows that were really aborts cannot be retroactively reclassified —
accepted.

### 2. Write-sites flipped `failed` → `aborted`

Both pre-broadcast abort writers emit `aborted`; nothing else changes about their
input-release behavior:

- `abort.go:131` `abortTx` — the shared choke point for **user `AbortAction`** and the
  **Monitor `fail_abandoned` sweep** (`abort_abandoned.go:76` → `abortTx`). CAS
  allowed-from set `{unprocessed, unsigned, nosend, nonfinal, unfail}` is unchanged;
  it now transitions those pre-broadcast states to `aborted`.
- `process.go:935` `abortTxByStringID` (called by `abortTxsBeforeBroadcast`) — the
  ctx-cancel / token-swap / build-abort path from PR #929.

### 3. Write-sites that stay `failed` (broadcast rejection)

`process.go:878`, `process.go:886`, `known_tx.go:150`, `synchronize_tx_statuses.go:498`,
`process_unfail.go:114`. This is the whole point of the ticket — the rejection family
must remain `failed` so the divergence is meaningful.

### 4. Enum plumbing (exhaustive-linter-forced)

`.golangci.json` sets `default-signifies-exhaustive:false`, so every `TxStatus` switch
without `//nolint:exhaustive` must gain an explicit `aborted` case or the build fails:

| Site | Case for `aborted` | Rationale |
|---|---|---|
| `tx_status.go:44` `ToStandardizedStatus` | `→ TxUpdateStatusFailed` | terminal-dead; **must not** be `InvalidTx` (that is permanent-rejection semantics for monitor/sync consumers). The retryable nuance lives on the raw `TxStatus`, not the standardized surface. |
| `tx_status.go:30` `ToUTXOStatus` (`//nolint` + default) | explicit `→ UTXOStatusUnknown` | inputs released, no live UTXO — same as `failed`. Explicit case for clarity. |
| `sync_output.go:283` `utxoStatusByTxStatus` | no-UTXO group `→ UTXOStatusUnknown` | no `UserUTXO` row created (`sync_output.go:122`); matches released-inputs intent. |
| `internalize.go:659` `isAllowedMergeStatus` | `→ false` | an aborted tx is not a valid internalize-merge target. |
| `internalize.go:670` `utxoStatusByTxStatusForMerge` | error group | "unsupported transaction status for UTXO". |
| `abort.go:186` `validateTxStatusForAbort` | **refuse** group | cannot re-abort an already-aborted tx (idempotent, matches how `failed` is refused today). |

### 5. Listing / read surface (D1)

Preserve set-compatibility while adding precision. Before this change, an aborted tx
was `failed` and appeared in the failed-actions bucket; keeping the returned **row set
identical** avoids a silent regression in the Enterprise Wallet's existing lookup, and
the raw status string supplies the new distinction.

- `list_failed_actions.go:45` `ListFailedActions` → filter `Status IN [failed, aborted]`.
- `list_actions_mapping.go:48` (`unfail` magic-label path) → same widening.
- `list_actions_mapping.go:86` already sets `Status: string(tx.Status)` — the raw
  `"aborted"` flows through with no mapping change.
- Default `listActions` allowlists (`outputs.go:212`, `list_actions_mapping.go:42`)
  already omit `failed`; `aborted` is auto-omitted the same way — no change, aborted
  stays out of ordinary listings.

Additionally, guard the unfail side-effect: `markActionsForUnfail`
(`list_actions_mapping.go:107`) flips each listed action's `KnownTx` to `unfail` so the
UnFail cron re-verifies it on-chain. Now that the failed-family bucket also returns
`aborted` rows, an unguarded `Unfail:true` list call could mark an aborted tx's shared
`KnownTx` for re-verification → the cron finds no proof → `markAsInvalid` cascades it to
`failed`, **re-erasing the distinction**. Add a status guard so only `failed` actions
are unfailed:

```go
if a.Status != string(wdk.TxStatusFailed) {
    continue
}
```

An aborted tx was never broadcast, so there is nothing on-chain to re-verify — the
Enterprise Wallet rebuilds a fresh tx instead.

### 6. BRC-100 boundary safety

`mapActionStatus` (`mapping_list_actions.go:129`) has a `default: error` branch that
would fail the **entire** `ListActions` response for any `aborted` row reachable via
the `unfail`-label path. Add a local temp constant mirroring the existing
`ActionStatusFailed` workaround (the go-sdk `ActionStatus` union lacks even `failed`):

```go
const ActionStatusAborted sdk.ActionStatus = "aborted" // non-standard; JSON transport only
```

Map `aborted → ActionStatusAborted`. This repo uses the JSON transport
(`client.go:206`), so the non-standard string round-trips; the go-sdk binary serializer
(unused here) is unaffected. Documented as a known BRC-100 wire-parity ceiling.

### 7. Output-liveness / failure-review sweep — verified no change needed

`FindKnownTxIDsByStatusesNeedingFailureReview` (`known_tx.go:461`) drives the Monitor
reconciliation net that cascades terminal-failure `KnownTx` rows to `Transaction=failed`
and restores outputs. Its outer filter selects only `KnownTx` rows already in
`{invalidTx, doubleSpend}`; its `EXISTS` predicate keys `transaction.status <> 'failed'`
(`known_tx.go:494`).

A normally-aborted tx is **never broadcast**, so no `KnownTx` reaches
`{invalidTx, doubleSpend}` for its txid → the outer filter excludes it. Its spent inputs
are already restored inside the abort UoW (`RecreateSpentOutputs`), so the `EXISTS`
output-liveness `OR` clause does not fire either. In the rare multi-user case where an
aborted tx shares a txid with a genuinely network-rejected `KnownTx`, cascading it to
`failed` is **correct** — that txid is dead and must not be retried. Therefore the
predicate is left **unchanged**; a regression test locks the common-case behavior
(aborted tx not swept, stays `aborted`).

### 8. Non-goals / out of scope

- **Mid-`PostFromBEEF` ctx-cancel** (`process.go:566-569`): a cancellation *during* the
  network POST returns the error without routing through `abortTxsBeforeBroadcast`,
  leaving the tx in `submitting/sending`. The tx **may** have reached the network, so it
  is **not safe** to classify as `aborted`. Left as-is (SendWaiting retries). Documented.
- **`ts-stack` parity:** follow-up to add `aborted` to TS `TransactionStatus` and a
  `parity_class:'intended'` conformance vector, then promote to `required` once both
  impls pass. The Go inbound sync path (`chunk_processor.go:329`) has no status
  whitelist, so a Go receiver already tolerates an unknown peer status; the hazard is a
  Go wallet **emitting** `aborted` to an older TS peer — noted for the follow-up.
- **Backfill** of historical `failed`-that-were-aborts: impossible (no discriminator was
  persisted), accepted.
- **A new `StandardizedTxStatus` value / BRC-100 `ActionStatus` upstream change:** not
  needed for D1's read path; deferred with the `ts-stack` follow-up.

## Invariants

- The set of transactions returned by `ListFailedActions` / the `unfail` path is a
  superset-equal of the pre-change set (no aborted tx disappears from where callers
  expect it).
- Exactly the two pre-broadcast abort writers emit `aborted`; every rejection writer
  emits `failed`.
- A never-broadcast aborted tx does not transition `aborted → failed`: re-abort is
  refused, `Unfail:true` skips it (§5 guard), and the failure-review sweep
  (`known_tx.go:461`) does not select it because its `KnownTx` is not in
  `{invalidTx, doubleSpend}`. (A genuinely network-rejected txid may still cascade a
  co-owned aborted tx to `failed` — intended, §7.)
- No DB schema change; behavior identical on SQLite/Postgres/MySQL.

## Testing

- `provider_abort_action_test.go`, `provider_abort_abandoned_test.go`: flip expected
  status `failed → aborted`.
- `mapActionStatus`: round-trip `"aborted" → ActionStatusAborted` (no error).
- `ToStandardizedStatus` table (`tx_status_test.go`): `aborted → TxUpdateStatusFailed`.
- `ListFailedActions` visibility: seed one rejected (`failed`) and one aborted
  (`aborted`) tx under the same label → both returned, raw statuses distinct.
- `Unfail:true` guard: an aborted action in the result is **not** unfailed
  (`markActionsForUnfail` skips non-`failed` status).
- Broadcast-rejection regression: doubleSpend/invalidTx assert status **stays** `failed`.
- Failure-review sweep: assert a never-broadcast `aborted` row is **not** selected by
  `FindKnownTxIDsByStatusesNeedingFailureReview` (`known_tx.go:461`) and stays `aborted`.
- `ListTransactions` terminal reporting: a `nosend`-then-aborted tx reports
  `TxUpdateStatusFailed` (terminal) on the standardized surface, not `TxUpdateStatusWaiting`.
- Exhaustive-switch compile coverage for each new case.

## Change surface (file-by-file)

| File | Change |
|---|---|
| `pkg/wdk/tx_status.go` | add `TxStatusAborted`; cases in `ToStandardizedStatus`, `ToUTXOStatus` |
| `pkg/storage/internal/actions/abort.go` | `abortTx` writes `aborted`; `validateTxStatusForAbort` refuses `aborted` |
| `pkg/storage/internal/actions/process.go` | `abortTxByStringID` writes `aborted` |
| `pkg/storage/internal/actions/internalize.go` | 2 switch cases |
| `pkg/storage/internal/actions/list_failed_actions.go` | widen status filter to `{failed, aborted}` |
| `pkg/storage/internal/actions/list_actions_mapping.go` | widen `unfail`-path filter; guard `markActionsForUnfail` to `failed`-only |
| `pkg/internal/storage/repo/syncrepo/sync_output.go` | 1 switch case |
| `pkg/wallet/internal/mapping/mapping_list_actions.go` | `ActionStatusAborted` const + case |
| `pkg/storage/provider.go` | `ListTransactions` standardized-status override treats `aborted` as terminal (`TxUpdateStatusFailed`), matching `ToStandardizedStatus`. The override bases status on the KnownTx status and previously special-cased only `failed`, so an `aborted` tx with a lingering non-terminal KnownTx (e.g. `nosend`, since `AbortAction` never touches KnownTx) would otherwise report `waiting` on this surface. Found in whole-branch review. |
| tests | as above |
| `ts-stack` follow-up (separate) | TS `TransactionStatus` + conformance vector |
