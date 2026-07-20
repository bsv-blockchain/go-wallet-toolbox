# ProcessActionError missing tx / noSendChange on review-required broadcast

**Issue:** [bsv-blockchain/go-wallet-toolbox#819](https://github.com/bsv-blockchain/go-wallet-toolbox/issues/819)
**TS reference:** TypeScript `WERR_REVIEW_ACTIONS` (wallet-toolbox) carries `txid`, `tx`, `sendWithResults`, `reviewActionResults`, and `noSendChange`
**Severity:** Medium — callers lose AtomicBEEF and noSend change outpoints when undelayed broadcast requires review; chained noSend → sendWith workflows cannot recover.

---

## Context for a fresh session

You are fixing a wallet-layer recovery gap. When undelayed `CreateAction` / `SignAction` broadcast outcomes are not all `unproven`, storage still returns a successful `ProcessActionResult` with `SendWithResults` and `NotDelayedResults`. The wallet then converts that into a `ProcessActionError` and returns `nil` for the happy-path result. Today that error only carries:

- `SendWithResults`
- `ReviewResults` (mapped from `NotDelayedResults`)
- `Cause`

It does **not** carry the AtomicBEEF `tx` bytes or `noSendChange` outpoints that a successful result would have returned. TypeScript throws `WERR_REVIEW_ACTIONS` with those fields so callers can treat the broadcast as “succeeded with caveats.” Go callers currently cannot rebroadcast or continue a noSend chain from the error alone.

Bug verified still present on `origin/main` (post closed implementation PR #943, which was intentionally closed as non-plan-only).

---

## Files to change

| File | Role | Notes |
|------|------|-------|
| `pkg/errors/action_error.go` | Extend `ProcessActionError` | Add optional `TxID`, `Tx`, `NoSendChange` + fluent setters; keep constructor signature stable |
| `pkg/errors/action_error_test.go` | Unit tests for new fields | Population, `Error()` includes txid, `errors.As` through `TransactionError` |
| `pkg/wallet/internal/actions/process_action_error.go` | **New** shared helper | Build error from process result + optional new-tx recovery data |
| `pkg/wallet/internal/actions/wallet_create_action.go` | Construction sites | New-tx path attaches AtomicBEEF + noSendChange; sendWith-only leaves them empty |
| `pkg/wallet/internal/actions/wallet_sign_action.go` | Construction sites | Review path + mapping-failure path attach AtomicBEEF |
| `pkg/wallet/wallet_create_action_test.go` | Integration coverage | Review-required createAction asserts `Tx` / `NoSendChange` via `errors.As` |

Optional (only if needed for SignAction noSend recovery):

| File | Role |
|------|------|
| `pkg/wallet/wallet_sign_action_test.go` | Mirror createAction review tests for sign flow |

**Out of scope for this issue:** storage-layer `NewProcessActionError` in `pkg/storage/internal/actions/process.go` (~L613) — mid-broadcast update failure has no assembled new-tx BEEF in scope; leave as-is.

---

## Problem summary + file anchors (current `main`)

### Error type lacks recovery fields

`pkg/errors/action_error.go` ~L130–143:

```go
type ProcessActionError struct {
	SendWithResults []wdk.SendWithResult
	ReviewResults   []wdk.ReviewActionResult
	Cause           error
}

func NewProcessActionError(sendWithResults []wdk.SendWithResult, reviewResults []wdk.ReviewActionResult) *ProcessActionError {
	return &ProcessActionError{
		SendWithResults: sendWithResults,
		ReviewResults:   reviewResults,
	}
}
```

### CreateAction drops tx / noSendChange on review

`pkg/wallet/internal/actions/wallet_create_action.go`:

- **sendWith-only** (`handleNotNewTX` ~L51–53): builds bare `ProcessActionError` — correct; no new tx exists.
- **new-tx path** (`handleProcessAction` ~L141–145): builds bare `ProcessActionError` even though `tx`, `txID`, and `createActionResult.NoSendChangeOutputVouts` are in scope.
- Outer wrap (`handleCreatedNewTx` ~L95–97): wraps the process error in `TransactionError(*tx.TxID())`, so callers use `errors.As` — fields on the inner `ProcessActionError` remain recoverable if populated.

Happy-path mapping that *does* attach these fields on success (never reached on review failure):

`pkg/wallet/internal/mapping/mapping_create_action_result.go` ~L14–36 — `MapIndexesToOutpoints` + `tx.AtomicBEEF(true)`.

### SignAction same gap

`pkg/wallet/internal/actions/wallet_sign_action.go`:

- `handleProcessAction` ~L124–128: bare `ProcessActionError` when `NotDelayedProcessActionResult` fails; `s.tx` / `s.txID` available.
- Mapping failure path ~L81–84: also bare (no AtomicBEEF on the nested process error).

### Validator that triggers the error

`pkg/internal/validate/validate_not_delayed_process_action_result.go`:

Returns error when `NotDelayedResults` is non-empty and not every `SendWithResult` is `unproven`. Storage already succeeded; wallet-level policy converts to error.

---

## Root cause

The wallet treats “review required” as a hard error but constructs that error from only the storage process summary. The local assembled transaction (AtomicBEEF) and derived noSend change outpoints live one stack frame above the constructor and are never threaded in. Result mapping code already knows how to produce both fields for success; the error path does not reuse that knowledge.

This is a **parity** gap with TypeScript `WERR_REVIEW_ACTIONS`, not a storage broadcast bug.

---

## Reference TypeScript semantics

TypeScript wallet-toolbox throws `WERR_REVIEW_ACTIONS` when undelayed broadcast outcomes need review. The exception carries:

| Field | Purpose |
|-------|---------|
| `txid` | New transaction id (when a new tx was created) |
| `tx` | Transaction bytes (BEEF / AtomicBEEF equivalent) |
| `sendWithResults` | Per-txid send status |
| `reviewActionResults` | Diagnostics (double-spend, service error, competing txs, …) |
| `noSendChange` | Change outpoints for noSend batch chaining (optional) |

Optional fields stay empty when the path has no new transaction (sendWith-only). Go should match that shape so cross-language wallet clients can share recovery logic.

---

## Recommended fix

### 1. Extend `ProcessActionError` (keep constructor stable)

Add optional fields and fluent setters; do **not** change `NewProcessActionError` parameter list (many call sites; storage path stays bare intentionally).

```go
type ProcessActionError struct {
	SendWithResults []wdk.SendWithResult
	ReviewResults   []wdk.ReviewActionResult
	// TxID is the hex transaction id of the new transaction, when available.
	TxID string
	// Tx is AtomicBEEF bytes of the new transaction, when available.
	Tx []byte
	// NoSendChange is outpoint strings ("txid.vout") for change from noSend batches.
	NoSendChange []string
	Cause        error
}

func (p *ProcessActionError) WithTx(txID string, tx []byte) *ProcessActionError {
	p.TxID = txID
	p.Tx = tx
	return p
}

func (p *ProcessActionError) WithNoSendChange(noSendChange []string) *ProcessActionError {
	p.NoSendChange = noSendChange
	return p
}
```

Update `Error()` to include `txID: …` when `TxID != ""` (aids logs without dumping BEEF). Preserve existing `Wrap` / `Unwrap` / `Is` behavior so `errors.As` / `errors.Is` keep working through `TransactionError` wrappers.

**Type note:** `NoSendChange` as `[]string` (`"txid.vout"`) matches TS wire-style outpoints and `primitives.OutpointString`. Alternatively store `[]transaction.Outpoint` — prefer strings for error-value simplicity and JSON-friendly diagnostics; document the format.

### 2. Shared wallet helper

New file `pkg/wallet/internal/actions/process_action_error.go`:

```go
func newProcessActionError(
	processActionResult *wdk.ProcessActionResult,
	txID *chainhash.Hash,
	tx *assembler.AssembledTransaction,
	noSendChangeOutputVouts []int,
) *pkgerrors.ProcessActionError {
	processErr := pkgerrors.NewProcessActionError(
		processActionResult.SendWithResults,
		processActionResult.NotDelayedResults,
	)

	if txID != nil {
		var beef []byte
		if tx != nil {
			// Best-effort: if AtomicBEEF fails, still attach txid.
			if bytes, err := tx.AtomicBEEF(true); err == nil {
				beef = bytes
			}
		}
		processErr = processErr.WithTx(txID.String(), beef)
	}

	if txID != nil && len(noSendChangeOutputVouts) > 0 {
		if outpoints, err := mapping.MapIndexesToOutpoints(txID, noSendChangeOutputVouts); err == nil {
			processErr = processErr.WithNoSendChange(outpointsToStrings(outpoints))
		}
	}

	return processErr
}
```

Reuse `mapping.MapIndexesToOutpoints` (same as success path). Convert via `primitives.NewOutpointString` / equivalent `"%s.%d"` so format matches the rest of the toolbox.

### 3. Wire construction sites

| Site | Change |
|------|--------|
| `CreateAction.handleProcessAction` (~L141–145) | `newProcessActionError(processActionResult, txID, tx, createActionResult.NoSendChangeOutputVouts).Wrap(broadcastErr)` |
| `CreateAction.handleNotNewTX` (~L51–53) | Leave bare `NewProcessActionError` (no new tx / noSendChange) |
| `SignAction.handleProcessAction` (~L124–128) | `newProcessActionError(processActionResult, s.txID, s.tx, nil).Wrap(err)` — attach Tx only; pending cache has no noSendChange vouts |
| `SignAction` mapping-failure wrap (~L81–84) | Same helper for consistency (`TransactionError` already wraps this path) |

**Wrap asymmetry (important for tests / callers):**

| Path | Outer wrap today |
|------|------------------|
| CreateAction new-tx review (`handleCreatedNewTx` ~L95–97) | `TransactionError(*tx.TxID()).Wrap(processErr)` |
| CreateAction sendWith-only (~L51–53) | bare `ProcessActionError` |
| SignAction review (`SignAction` ~L74–77) | bare `ProcessActionError` (no `TransactionError`) |
| SignAction mapping failure (~L81–84) | `TransactionError` wrapping `ProcessActionError` |

Do **not** invent new wraps for parity — keep existing wrap behavior. Recovery always uses `errors.As` to `*ProcessActionError` (works with or without outer `TransactionError`):

```go
var processErr *errors.ProcessActionError
if errors.As(err, &processErr) {
	_ = processErr.TxID
	_ = processErr.Tx
	_ = processErr.NoSendChange
	_ = processErr.SendWithResults
	_ = processErr.ReviewResults
}
```

### 4. SignAction + noSendChange (optional stretch / follow-up)

`pending.SignAction` (`pkg/wallet/pending/pending_sign_action.interface.go`) stores `Tx`, `InputBEEF`, `CreateActionArgs` — **not** `NoSendChangeOutputVouts`. Create-signable already returns `NoSendChange` on the **success** path via `SignableTransactionResult` (`mapping_signable_transaction_result.go` ~L30–35) when `IsNoSend`, so callers typically already hold those outpoints before `SignAction`.

If product still requires noSendChange on **sign** review errors:

1. Persist vouts (or full outpoints) on `pending.SignAction` at create-signable time, **or**
2. Re-derive from storage / pending tx outputs when building the error.

**Default scope for #819:** CreateAction and SignAction attach **tx** bytes; **noSendChange** at least on CreateAction new-tx path where `NoSendChangeOutputVouts` is already available. SignAction noSendChange is follow-up unless pending cache is extended in the same PR.

---

## Settled design decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Constructor signature | Keep `NewProcessActionError(send, review)` | Avoid churn at storage + existing call sites |
| Recovery API | Fluent `WithTx` / `WithNoSendChange` | Optional fields; chainable with existing `Wrap` |
| `NoSendChange` type on error | `[]string` (`txid.vout`) | Matches TS wire style + `primitives.OutpointString`; simpler than `[]transaction.Outpoint` on error values (success result still uses Outpoints) |
| AtomicBEEF failure | Best-effort; still set `TxID`, leave `Tx` nil | Never mask the original review `Cause` |
| `ReturnTXIDOnly` on errors | Prefer still attaching AtomicBEEF when `tx` is available | Recovery is the purpose of the error; gate only if product insists |
| Review policy | Unchanged | Still error + nil result; enrichment only |
| Storage mid-broadcast path | Leave bare | No assembled BEEF in scope at `process.go` ~L613 |
| SignAction noSendChange | Out of default scope | Available earlier via create-signable; pending cache lacks vouts |

---

## Implementation order

1. Extend `ProcessActionError` + `WithTx` / `WithNoSendChange` + `Error()` txid part in `pkg/errors/action_error.go`.
2. Unit tests in `pkg/errors/action_error_test.go` (population + `errors.As` through `TransactionError`).
3. Add `newProcessActionError` helper (+ `outpointsToStrings`) in `pkg/wallet/internal/actions/process_action_error.go`.
4. Wire CreateAction new-tx path; leave sendWith-only bare.
5. Wire SignAction review + mapping-failure paths (Tx only).
6. Extend wallet integration tests (see below; model after closed PR #943 test bodies).
7. `go test` + `golangci-lint` on touched packages.

Reference patch shape (do not merge as-is; re-validate on current main): closed PR #943 files list matches this plan exactly.

---

## Test strategy

### Unit (`pkg/errors`)

1. **WithTx / WithNoSendChange** — fields set; `Error()` contains txid; `Unwrap`/`Is` still work with cause.
2. **errors.As through TransactionError** — wrap `ProcessActionError` in `NewTransactionError(...).Wrap(processErr)`; recover all three recovery fields.

### Integration (`pkg/wallet`)

Extend `TestWalletCreateAction_NoSend_SendWith_BroadcastErrorForOne` (~L863) with ARC double-spend fixtures (existing pattern in that suite). Closed PR #943 already drafted two subtests you can port:

1. **Review-required createAction with new tx** — prior noSend, ARC double-spend on that txid, then createAction with `sendWith` that txid + a new output; assert:
   - `result == nil`, `err != nil`
   - `require.ErrorAs` → `*ProcessActionError`
   - `TxID` non-empty, `Tx` non-empty AtomicBEEF
   - `SendWithResults` / `ReviewResults` non-empty
   - `Error()` contains `TxID`
2. **Review-required createAction with noSend** — new noSend tx + sendWith of a double-spending prior noSend; assert `NoSendChange` non-empty and each outpoint has prefix `TxID + "."`.
3. **sendWith-only path** (no new tx) — still errors without requiring `Tx` / `NoSendChange` (empty optional fields). Existing bare-constructor behavior; add assertion only if a dedicated case exists or is cheap to add.
4. **SignAction** (recommended) — sign undelayed path with review-required process result; assert AtomicBEEF + TxID on error (noSendChange may be empty).

Ensure faucet top-ups leave enough UTXOs when the test creates multiple actions (prior flaky cases needed a second `TopUp`).

Fixture helpers already used by that suite: `given.Services().ARC().WhenQueryingTx(...).WillReturnDoubleSpending()`, `walletargs.WithNoSend`, `walletargs.WithSendWith`.

### Commands

```bash
go test ./pkg/errors/ -count=1
go test ./pkg/wallet/ -count=1 -run 'TestWalletWithSQLiteStorage/TestWalletCreateAction_NoSend_SendWith_BroadcastErrorForOne' -timeout 180s
# plus any new SignAction review test name
golangci-lint run ./pkg/errors/... ./pkg/wallet/internal/actions/... ./pkg/wallet/...
```

Watch for `testifylint` `error-is-as`: prefer `require.ErrorAs` / `assert.ErrorIs` over `assert.True(t, errors.Is(...))` / `assert.True(t, errors.As(...))`.

---

## Acceptance criteria

- [ ] `ProcessActionError` exposes optional `TxID`, `Tx` (AtomicBEEF), and `NoSendChange` (`txid.vout` strings).
- [ ] Constructor `NewProcessActionError(send, review)` signature unchanged; fluent setters populate recovery fields.
- [ ] Undelayed **CreateAction** new-tx review path attaches AtomicBEEF + noSendChange when available.
- [ ] Undelayed **SignAction** review path attaches AtomicBEEF (txid + tx).
- [ ] sendWith-only CreateAction path still returns process error **without** inventing tx/noSendChange.
- [ ] Callers recover fields with `errors.As` even when wrapped in `TransactionError`.
- [ ] `Error()` string includes txid when set; does not dump full BEEF.
- [ ] Unit + integration tests cover population and recovery; existing process-action tests still pass.
- [ ] Storage-layer process update failure path left unchanged (documented non-goal).

---

## Risks, non-goals, dependencies

### Risks

- **AtomicBEEF best-effort:** serialization failure must not mask the original review error — attach txid, leave `Tx` nil, keep `Cause` as the review validation error.
- **Memory / log size:** BEEF on the error value is intentional for recovery; avoid logging `processErr.Tx` wholesale.
- **Outpoint format drift:** use the same string form as the rest of the wallet (`txid.vout` via `primitives.OutpointString`) so callers can feed values back into noSend options without reformatting.
- **SignAction noSendChange incompleteness:** default scope is Tx-only on SignAction; document so implementers do not claim full WERR_REVIEW_ACTIONS parity until follow-up (create-signable already returns NoSendChange to callers before sign).
- **Integration flakiness:** double-spend ARC fixtures need two faucet top-ups when two createActions spend change; reuse the pattern already in `TestWalletCreateAction_NoSend_SendWith_BroadcastErrorForOne`.

### Non-goals

- Changing when review is required (`NotDelayedProcessActionResult` policy).
- Returning a non-nil success result on review (keep error + nil result; recovery is on the error).
- Storage `process.go` mid-broadcast `NewProcessActionError` enrichment.
- Public SDK API changes beyond richer typed error fields.
- Closing #819 from a plan PR (`Related to #819` only).

### Dependencies

- None blocked. Reuses existing `assembler.AssembledTransaction.AtomicBEEF`, `mapping.MapIndexesToOutpoints`, and ARC test doubles already used by wallet tests.
- Prior draft implementation existed on closed PR #943 (`fix/819-process-action-error-fields`) — useful reference for implementers; do not merge that branch as-is without re-validating against current main.

---

## Estimated size

**S–M** — small API surface on one error type, one helper, 2–3 call sites, focused tests. Most complexity is integration test setup for review-required broadcast, which already exists in outline form.

---

## Useful cross-references

- Issue: <https://github.com/bsv-blockchain/go-wallet-toolbox/issues/819>
- Closed code draft (reference only): <https://github.com/bsv-blockchain/go-wallet-toolbox/pull/943>
- Happy-path result mapping: `pkg/wallet/internal/mapping/mapping_create_action_result.go`, `mapping_sign_action_result.go`, `mapping_outpoints.go`
- Validator: `pkg/internal/validate/validate_not_delayed_process_action_result.go`
- WDK result types: `pkg/wdk/storage_process_action_result.go` (`SendWithResult`, `ReviewActionResult`, `ProcessActionResult`)
- Outpoint string type: `pkg/wdk/primitives/outpoint.go`
- Assembler BEEF: `pkg/internal/assembler/assembled_transaction.go` (`AtomicBEEF`, `ToAtomicBEEF`)
- Related noSend example flow: `examples/wallet_examples/no_send_send_with/`

---

## Notes / gotchas

- CreateAction wraps process failures as `TransactionError{TxID}.Wrap(processErr)` — implementers must not put recovery fields only on the outer type; keep them on `ProcessActionError` so both `errors.As` targets remain useful. SignAction review path does **not** add that outer wrap today.
- `ReviewResults` on the Go error maps from `processActionResult.NotDelayedResults` (field name differs from storage JSON `notDelayedResults` / TS `reviewActionResults`).
- `ReturnTXIDOnly` success path skips BEEF; for review errors prefer always attaching AtomicBEEF when `tx` is available (recovery is the point of the error). If product wants to honor `ReturnTXIDOnly` on errors, gate BEEF but still set `TxID`.
- Success `CreateActionResult.NoSendChange` is `[]transaction.Outpoint`; error field is `[]string` — document conversion if callers bridge both (`primitives.NewOutpointString` / `OutpointString.Get`).
- Helper lives in package `actions` (unexported `newProcessActionError`) so wallet tests assert via public `*pkgerrors.ProcessActionError` only.
- Do not use `Fixes #819` on a plan PR; implementation PR may close the issue after merge (`Related to #819` on plan; `Fixes #819` only on the code PR).
