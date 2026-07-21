# Aborted Transaction Status Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a distinct terminal transaction status `aborted`, written only by the pre-broadcast abort paths, so the Enterprise Wallet's idempotency-label lookup can tell a retryable abort from a permanent broadcast rejection.

**Architecture:** `failed` today means both "aborted before broadcast" (retryable) and "broadcast then rejected" (permanent). Introduce `wdk.TxStatusAborted = "aborted"`; flip only the two pre-broadcast abort writers to emit it; leave every rejection writer on `failed`. Surface it raw through the existing failed-actions listing bucket (widened to `{failed, aborted}`). Go-only; TypeScript `ts-stack` parity is a tracked follow-up.

**Tech Stack:** Go, GORM (SQLite/Postgres/MySQL), `go-sdk` wallet interfaces (BRC-100), `golangci-lint` with `exhaustive` (`default-signifies-exhaustive:false`), testify.

**Spec:** `docs/superpowers/specs/2026-07-20-aborted-tx-status-design.md`

## Global Constraints

- **No DB migration.** `Transaction.Status` is a free-form string column (`models/transaction.go:13`); the new value stores without schema change on all three engines.
- **Exhaustive linter is ON** with `default-signifies-exhaustive:false` (`.golangci.json`). Every `switch` over `wdk.TxStatus` that lacks `//nolint:exhaustive` must enumerate the new `TxStatusAborted` case explicitly, even when a `default` exists, or the build fails.
- **Only two writers emit `aborted`:** `abort.go` `abortTx` and `process.go` `abortTxByStringID`. Every broadcast-rejection writer stays on `failed` — this divergence is the entire point.
- **Ignore `.claude/worktrees/`** — stale copies, never edit.
- **Run from repo root.** Build check: `go build ./...`. Lint check: `golangci-lint run` (or `make lint` if present). Tests: `go test ./pkg/...`.
- End every commit message with: `Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>`.

---

### Task 1: Introduce `TxStatusAborted` across the type system

Adds the enum value and every exhaustive-switch case it forces, so the whole tree builds and lints green and the enum mappings are unit-tested. No writer emits it yet.

**Files:**
- Modify: `pkg/wdk/tx_status.go` (const block; `ToUTXOStatus`; `ToStandardizedStatus`)
- Modify: `pkg/internal/storage/repo/syncrepo/sync_output.go:282-295` (`utxoStatusByTxStatus`)
- Modify: `pkg/storage/internal/actions/internalize.go:659-683` (`isAllowedMergeStatus`, `utxoStatusByTxStatusForMerge`)
- Modify: `pkg/storage/internal/actions/abort.go:186-195` (`validateTxStatusForAbort`)
- Test: `pkg/wdk/tx_status_test.go` (new `TestTxStatusMappings`)

**Interfaces:**
- Produces: `wdk.TxStatusAborted wdk.TxStatus = "aborted"`. Mappings: `TxStatusAborted.ToUTXOStatus() == wdk.UTXOStatusUnknown`; `TxStatusAborted.ToStandardizedStatus() == wdk.TxUpdateStatusFailed`. `validateTxStatusForAbort(TxStatusAborted)` returns a `wdk.ErrNotAbortableAction` error (non-abortable).

- [ ] **Step 1: Write the failing test** — append to `pkg/wdk/tx_status_test.go`:

```go
// TestTxStatusMappings pins ToUTXOStatus and ToStandardizedStatus for every TxStatus
// value. The coverage guard forces this table to be extended whenever a new TxStatus
// is added to the enum.
func TestTxStatusMappings(t *testing.T) {
	type expectation struct {
		status       wdk.TxStatus
		utxo         wdk.UTXOStatus
		standardized wdk.StandardizedTxStatus
	}

	table := []expectation{
		{wdk.TxStatusCompleted, wdk.UTXOStatusMined, wdk.TxUpdateStatusMined},
		{wdk.TxStatusFailed, wdk.UTXOStatusUnknown, wdk.TxUpdateStatusInvalidTx},
		{wdk.TxStatusUnprocessed, wdk.UTXOStatusUnknown, wdk.TxUpdateStatusWaiting},
		{wdk.TxStatusSending, wdk.UTXOStatusSending, wdk.TxUpdateStatusWaiting},
		{wdk.TxStatusUnproven, wdk.UTXOStatusUnproven, wdk.TxUpdateStatusBroadcasted},
		{wdk.TxStatusUnsigned, wdk.UTXOStatusUnknown, wdk.TxUpdateStatusWaiting},
		{wdk.TxStatusNoSend, wdk.UTXOStatusUnknown, wdk.TxUpdateStatusWaiting},
		{wdk.TxStatusNonFinal, wdk.UTXOStatusUnknown, wdk.TxUpdateStatusWaiting},
		{wdk.TxStatusUnfail, wdk.UTXOStatusUnknown, wdk.TxUpdateStatusWaiting},
		{wdk.TxStatusAborted, wdk.UTXOStatusUnknown, wdk.TxUpdateStatusFailed},
	}

	allStatuses := []wdk.TxStatus{
		wdk.TxStatusCompleted, wdk.TxStatusFailed, wdk.TxStatusUnprocessed,
		wdk.TxStatusSending, wdk.TxStatusUnproven, wdk.TxStatusUnsigned,
		wdk.TxStatusNoSend, wdk.TxStatusNonFinal, wdk.TxStatusUnfail,
		wdk.TxStatusAborted,
	}
	assert.Len(t, table, len(allStatuses), "mapping table must cover all TxStatus values")

	for _, row := range table {
		t.Run(string(row.status), func(t *testing.T) {
			assert.Equalf(t, row.utxo, row.status.ToUTXOStatus(), "ToUTXOStatus(%q)", row.status)
			assert.Equalf(t, row.standardized, row.status.ToStandardizedStatus(), "ToStandardizedStatus(%q)", row.status)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./pkg/wdk/ -run TestTxStatusMappings -v`
Expected: FAIL to compile — `wdk.TxStatusAborted` undefined.

- [ ] **Step 3: Add the const** — in `pkg/wdk/tx_status.go`, extend the const block (after `TxStatusUnfail`, line 20):

```go
	TxStatusUnfail      TxStatus = "unfail"
	// TxStatusAborted marks a transaction that was built but never broadcast (ctx
	// cancellation, token-swap timeout, user AbortAction, or the fail_abandoned sweep).
	// Its inputs are released and it is safe to rebuild and retry — distinct from
	// TxStatusFailed, which marks a broadcast that the network rejected (permanent).
	TxStatusAborted     TxStatus = "aborted"
```

- [ ] **Step 4: Add the `ToUTXOStatus` case** — `pkg/wdk/tx_status.go:29-40`, add an explicit case (the `//nolint` default already yields Unknown, but be explicit):

```go
func (s TxStatus) ToUTXOStatus() UTXOStatus {
	switch s { //nolint:exhaustive // default case handles remaining statuses
	case TxStatusCompleted:
		return UTXOStatusMined
	case TxStatusSending:
		return UTXOStatusSending
	case TxStatusUnproven:
		return UTXOStatusUnproven
	case TxStatusAborted:
		return UTXOStatusUnknown
	default:
		return UTXOStatusUnknown
	}
}
```

- [ ] **Step 5: Add the `ToStandardizedStatus` case** — `pkg/wdk/tx_status.go:43-56`. `aborted` maps to the terminal generic `failed`, NOT `InvalidTx` (which is permanent-rejection semantics):

```go
func (s TxStatus) ToStandardizedStatus() StandardizedTxStatus {
	switch s {
	case TxStatusCompleted:
		return TxUpdateStatusMined
	case TxStatusUnproven:
		return TxUpdateStatusBroadcasted
	case TxStatusSending, TxStatusUnprocessed, TxStatusNoSend, TxStatusNonFinal, TxStatusUnsigned, TxStatusUnfail:
		return TxUpdateStatusWaiting
	case TxStatusFailed:
		return TxUpdateStatusInvalidTx
	case TxStatusAborted:
		return TxUpdateStatusFailed
	default:
		return TxUpdateStatusUnknown
	}
}
```

- [ ] **Step 6: Add the `utxoStatusByTxStatus` case** — `pkg/internal/storage/repo/syncrepo/sync_output.go:290`, add `wdk.TxStatusAborted` to the no-UTXO fallthrough group:

```go
	case wdk.TxStatusFailed, wdk.TxStatusAborted, wdk.TxStatusUnprocessed, wdk.TxStatusUnsigned, wdk.TxStatusNoSend, wdk.TxStatusNonFinal, wdk.TxStatusUnfail:
		fallthrough
	default:
		return wdk.UTXOStatusUnknown
```

- [ ] **Step 7: Add the two `internalize.go` cases** — `pkg/storage/internal/actions/internalize.go`. Line 663 (`isAllowedMergeStatus`, the `false` group) and line 678 (`utxoStatusByTxStatusForMerge`, the error group):

```go
	// isAllowedMergeStatus (line ~663)
	case wdk.TxStatusFailed, wdk.TxStatusAborted, wdk.TxStatusUnprocessed, wdk.TxStatusUnsigned, wdk.TxStatusNonFinal, wdk.TxStatusUnfail:
		fallthrough
	default:
		return false
```

```go
	// utxoStatusByTxStatusForMerge (line ~678)
	case wdk.TxStatusFailed, wdk.TxStatusAborted, wdk.TxStatusUnprocessed, wdk.TxStatusUnsigned, wdk.TxStatusNoSend, wdk.TxStatusNonFinal, wdk.TxStatusUnfail:
		fallthrough
	default:
		return "", fmt.Errorf("unsupported transaction status for UTXO: %s", txStatus)
```

- [ ] **Step 8: Add the `validateTxStatusForAbort` refuse case** — `pkg/storage/internal/actions/abort.go:186-195`, add `wdk.TxStatusAborted` to the refuse group so re-aborting an aborted tx is rejected:

```go
func validateTxStatusForAbort(txStatus wdk.TxStatus) error {
	switch txStatus {
	case wdk.TxStatusCompleted, wdk.TxStatusFailed, wdk.TxStatusAborted, wdk.TxStatusSending, wdk.TxStatusUnproven:
		return fmt.Errorf("%w: action with status %s cannot be aborted", wdk.ErrNotAbortableAction, txStatus)
	case wdk.TxStatusUnprocessed, wdk.TxStatusUnsigned, wdk.TxStatusNoSend, wdk.TxStatusNonFinal, wdk.TxStatusUnfail:
		return nil
	default:
		return fmt.Errorf("%w: unexpected transaction status %s", wdk.ErrNotAbortableAction, txStatus)
	}
}
```

- [ ] **Step 9: Run the test and the build/lint**

Run: `go test ./pkg/wdk/ -run TestTxStatusMappings -v && go build ./... && golangci-lint run ./pkg/wdk/... ./pkg/storage/... ./pkg/internal/...`
Expected: test PASS; build clean; no `exhaustive` lint errors.

- [ ] **Step 10: Commit**

```bash
git add pkg/wdk/tx_status.go pkg/wdk/tx_status_test.go pkg/internal/storage/repo/syncrepo/sync_output.go pkg/storage/internal/actions/internalize.go pkg/storage/internal/actions/abort.go
git commit -m "feat: add TxStatusAborted enum value and its type-system mappings (#959)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 2: Pre-broadcast abort writers emit `aborted`

Flip the two abort write-sites from `failed` to `aborted`. This covers all three sources: ctx-cancel/build-abort (`abortTxByStringID`), user `AbortAction`, and the Monitor `fail_abandoned` sweep (last two share `abortTx`).

**Files:**
- Modify: `pkg/storage/internal/actions/abort.go:131` (`abortTx`)
- Modify: `pkg/storage/internal/actions/process.go:935` (`abortTxByStringID`)
- Test: `pkg/storage/provider_abort_action_test.go` (2 assertions), `pkg/storage/provider_abort_abandoned_test.go` (1 assertion)

**Interfaces:**
- Consumes: `wdk.TxStatusAborted` (Task 1).
- Produces: after `AbortAction`, `AbortAbandoned`, or a pre-broadcast `abortTxByStringID`, the transaction row has `Status == wdk.TxStatusAborted`.

- [ ] **Step 1: Update the failing assertions** — in `pkg/storage/provider_abort_action_test.go`, change the two existing status assertions (currently at lines 44 and 73) from `WithStatus(wdk.TxStatusFailed)` to `WithStatus(wdk.TxStatusAborted)`. In `pkg/storage/provider_abort_abandoned_test.go`, change the assertion at line 61 from `WithStatus(wdk.TxStatusFailed)` to `WithStatus(wdk.TxStatusAborted)`.

Also update the CAS-race assertion in `pkg/storage/provider_abort_action_test.go` (the block around line 311 that expects `WithStatus(wdk.TxStatusFailed)` after a successful abort) to `WithStatus(wdk.TxStatusAborted)`. Leave any assertion that expects `TxStatusSending`/`TxStatusUnprocessed` for a NON-aborted tx unchanged (e.g. the "sending tx should remain unaffected" and the KnownTx-status assertions).

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/storage/ -run 'TestAbortAction|TestAbortAbandoned' -v`
Expected: FAIL — abort still writes `failed`, assertions now expect `aborted`.

- [ ] **Step 3: Flip `abortTx`** — `pkg/storage/internal/actions/abort.go:131-132`. Change the target status; the CAS allowed-from set is unchanged:

```go
		if err := repos.TransactionsRepo().UpdateTransactionStatusByID(txCtx, id, wdk.TxStatusAborted,
			wdk.TxStatusUnprocessed, wdk.TxStatusUnsigned, wdk.TxStatusNoSend, wdk.TxStatusNonFinal, wdk.TxStatusUnfail); err != nil {
			return fmt.Errorf("failed to update transaction status: %w", err)
		}
```

Also update the adjacent debug log line (`abort.go:126`) text `"Updating transaction status to 'failed'"` → `"Updating transaction status to 'aborted'"`.

- [ ] **Step 4: Flip `abortTxByStringID`** — `pkg/storage/internal/actions/process.go:935`:

```go
			if err := repos.TransactionsRepo().UpdateTransactionStatusByID(txCtx, id, wdk.TxStatusAborted); err != nil {
				return fmt.Errorf("failed to update transaction status to aborted: %w", err)
			}
```

- [ ] **Step 5: Run the tests to verify they pass**

Run: `go test ./pkg/storage/ -run 'TestAbortAction|TestAbortAbandoned' -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/storage/internal/actions/abort.go pkg/storage/internal/actions/process.go pkg/storage/provider_abort_action_test.go pkg/storage/provider_abort_abandoned_test.go
git commit -m "feat: pre-broadcast abort paths write TxStatusAborted (#959)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 3: Regression lock — broadcast rejection stays `failed`

Prove the divergence: doubleSpend / invalidTx broadcasts still land on `failed`, not `aborted`. No production code changes — this is a guard test that fails loudly if a future edit blurs the two paths again.

**Files:**
- Test: `pkg/storage/provider_broadcast_rejection_status_test.go` (new)

**Interfaces:**
- Consumes: `activeStorage.ProcessAction`, `provider.ARC().WhenQueryingTx(...).WillReturnDoubleSpending(...)` (existing test harness, see `provider_list_failed_actions_test.go`).

- [ ] **Step 1: Write the test** — new file `pkg/storage/provider_broadcast_rejection_status_test.go`:

```go
package storage_test

import (
	"testing"

	testvectors "github.com/bsv-blockchain/universal-test-vectors/pkg/testabilities"
	"github.com/go-softwarelab/common/pkg/to"
	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/randomizer"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// TestBroadcastRejectionStaysFailed locks the #959 divergence: a transaction that
// reached the broadcast endpoint and was rejected (double spend) must land on
// TxStatusFailed, never TxStatusAborted. Only pre-broadcast aborts use 'aborted'.
func TestBroadcastRejectionStaysFailed(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	provider := given.Provider()
	activeStorage := provider.WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	createResult, signedTx := given.Action(activeStorage).Created()
	txID := signedTx.TxID().String()
	competingTxID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID().String()
	provider.ARC().WhenQueryingTx(txID).WillReturnDoubleSpending(competingTxID)

	// when: the tx is broadcast and rejected as a double spend
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx:   true,
		Reference: to.Ptr(createResult.Reference),
		TxID:      to.Ptr(primitives.TXIDHexString(txID)),
		RawTx:     signedTx.Bytes(),
		SendWith:  []primitives.TXIDHexString{},
	})
	require.NoError(t, err)

	// then: it is 'failed' (permanent), NOT 'aborted' (retryable)
	testabilities.ThenDBState(t, activeStorage).
		HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusFailed)
}
```

- [ ] **Step 2: Run the test to verify it passes**

Run: `go test ./pkg/storage/ -run TestBroadcastRejectionStaysFailed -v`
Expected: PASS (rejection writers were untouched). If it FAILS, a rejection writer was wrongly changed — fix that, do not weaken this test.

- [ ] **Step 3: Commit**

```bash
git add pkg/storage/provider_broadcast_rejection_status_test.go
git commit -m "test: lock broadcast rejection to TxStatusFailed, not aborted (#959)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 4: Widen the failed-actions listing bucket and guard unfail

Make the read path work: `aborted` transactions must appear where `failed` ones do (they used to, when aborts were `failed`), carrying their raw status so the caller can distinguish. Guard the unfail side-effect so listing aborted rows can never re-fail them.

**Files:**
- Modify: `pkg/storage/internal/actions/list_failed_actions.go:45`
- Modify: `pkg/storage/internal/actions/list_actions_mapping.go` (`toFilterParams` failed-query set ~line 48; `markActionsForUnfail` ~line 108)
- Test: `pkg/storage/provider_list_failed_actions_test.go` (new test in existing file)

**Interfaces:**
- Consumes: `wdk.TxStatusAborted` (Task 1), abort writers (Task 2), `specops.ListActionsSpecOpFailedActionsLabel`, `wdk.TxStatusUnfail` label.
- Produces: `ListFailedActions` / the failed-actions spec-op returns transactions with `Status IN {failed, aborted}`, each `WalletAction.Status` carrying the raw value. `Unfail:true` flips only `failed` actions' KnownTx to `unfail`.

- [ ] **Step 1: Write the failing test** — append to `pkg/storage/provider_list_failed_actions_test.go`:

```go
// TestListFailedActions_IncludesAbortedWithDistinctStatus proves an aborted tx
// (never broadcast) surfaces in the failed-actions bucket alongside genuinely failed
// txs, with its raw status preserved so the caller can distinguish retryable-abort
// from permanent-rejection. It also proves Unfail does not re-fail the aborted tx.
func TestListFailedActions_IncludesAbortedWithDistinctStatus(t *testing.T) {
	// given:
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	provider := given.Provider()
	activeStorage := provider.WithRandomizer(randomizer.NewTestRandomizer()).GORM()

	// and: a rejected (failed) tx
	failedCreate, failedSigned := given.Action(activeStorage).Created()
	failedTxID := failedSigned.TxID().String()
	competingTxID := testvectors.GivenTX().WithInput(2).WithP2PKHOutput(1).ID().String()
	provider.ARC().WhenQueryingTx(failedTxID).WillReturnDoubleSpending(competingTxID)
	_, err := activeStorage.ProcessAction(t.Context(), testusers.Alice.AuthID(), wdk.ProcessActionArgs{
		IsNewTx: true, Reference: to.Ptr(failedCreate.Reference),
		TxID: to.Ptr(primitives.TXIDHexString(failedTxID)), RawTx: failedSigned.Bytes(),
		SendWith: []primitives.TXIDHexString{},
	})
	require.NoError(t, err)

	// and: an aborted tx (never broadcast)
	abortedCreate, _ := given.Action(activeStorage).Created()
	_, err = activeStorage.AbortAction(t.Context(), testusers.Alice.AuthID(), wdk.AbortActionArgs{
		Reference: primitives.Base64String(abortedCreate.Reference),
	})
	require.NoError(t, err)

	// when: list the failed-actions bucket (spec-op route), with unfail requested
	result, err := activeStorage.ListActions(t.Context(), testusers.Alice.AuthID(), wdk.ListActionsArgs{
		Labels: []primitives.StringUnder300{
			primitives.StringUnder300(wdk.TxStatusUnfail),
			primitives.StringUnder300(specops.ListActionsSpecOpFailedActionsLabel),
		},
		Limit: 10,
	})

	// then: both are returned, with distinct raw statuses
	require.NoError(t, err)
	require.EqualValues(t, 2, result.TotalActions)
	statuses := map[string]int{}
	for _, a := range result.Actions {
		statuses[a.Status]++
	}
	require.Equal(t, 1, statuses[string(wdk.TxStatusFailed)], "one failed action expected")
	require.Equal(t, 1, statuses[string(wdk.TxStatusAborted)], "one aborted action expected")

	// and: the aborted tx was NOT re-failed by the unfail side-effect
	testabilities.ThenDBState(t, activeStorage).
		HasUserTransactionByReference(testusers.Alice, abortedCreate.Reference).
		WithStatus(wdk.TxStatusAborted)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pkg/storage/ -run TestListFailedActions_IncludesAbortedWithDistinctStatus -v`
Expected: FAIL — bucket still `[failed]` only, so `TotalActions` is 1 and the aborted action is missing.

- [ ] **Step 3: Widen the `ListFailedActions` filter** — `pkg/storage/internal/actions/list_failed_actions.go:45`:

```go
	filter.Status = []wdk.TxStatus{wdk.TxStatusFailed, wdk.TxStatusAborted}
```

Also update the doc comment at line 19 (`// ListFailedActions lists only actions with status 'failed'.`) to `// ListFailedActions lists actions with a terminal-unsuccessful status ('failed' or 'aborted').`

- [ ] **Step 4: Widen the `isFailedQuery` set** — `pkg/storage/internal/actions/list_actions_mapping.go` (the `if isFailedQuery` branch, ~line 48):

```go
	if isFailedQuery {
		statuses = []wdk.TxStatus{wdk.TxStatusFailed, wdk.TxStatusAborted}
	}
```

- [ ] **Step 5: Guard `markActionsForUnfail`** — `pkg/storage/internal/actions/list_actions_mapping.go`, inside the loop in `markActionsForUnfail` (~line 108), skip anything not `failed` (an aborted tx was never broadcast, so there is nothing on-chain to re-verify):

```go
	for _, a := range actions {
		if a.Status != string(wdk.TxStatusFailed) {
			continue
		}
		if a.TxID == "" {
			continue
		}
		// ... existing UpdateKnownTxStatus / ErrStatusUpdateSkipped handling unchanged ...
	}
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `go test ./pkg/storage/ -run 'TestListFailedActions' -v`
Expected: PASS (both the new test and the existing `...ToleratesTransactionWithoutKnownTxRow`).

- [ ] **Step 7: Commit**

```bash
git add pkg/storage/internal/actions/list_failed_actions.go pkg/storage/internal/actions/list_actions_mapping.go pkg/storage/provider_list_failed_actions_test.go
git commit -m "feat: surface aborted txs in the failed-actions bucket with raw status (#959)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 5: BRC-100 boundary — `mapActionStatus` handles `aborted`

Prevent `ListActions` (BRC-100) from erroring when an `aborted` row reaches the mapper via the failed-actions spec-op route. The go-sdk `ActionStatus` union lacks `aborted` (it lacks even `failed`), so add a local temp constant mirroring the existing `ActionStatusFailed` workaround.

**Files:**
- Modify: `pkg/wallet/internal/mapping/mapping_list_actions.go` (const ~line 126; `mapActionStatus` ~line 129)
- Test: `pkg/wallet/internal/mapping/mapping_list_actions_status_test.go` (new)

**Interfaces:**
- Consumes: `sdk.ActionStatus`, existing `ActionStatusFailed`.
- Produces: `mapActionStatus("aborted")` returns `(ActionStatusAborted, nil)` where `ActionStatusAborted sdk.ActionStatus = "aborted"`.

- [ ] **Step 1: Write the failing test** — new file `pkg/wallet/internal/mapping/mapping_list_actions_status_test.go`:

```go
package mapping

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapActionStatus_Aborted(t *testing.T) {
	got, err := mapActionStatus("aborted")
	require.NoError(t, err)
	assert.Equal(t, ActionStatusAborted, got)
}

func TestMapActionStatus_Unknown(t *testing.T) {
	_, err := mapActionStatus("bogus")
	require.Error(t, err)
}
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `go test ./pkg/wallet/internal/mapping/ -run TestMapActionStatus -v`
Expected: FAIL to compile — `ActionStatusAborted` undefined.

- [ ] **Step 3: Add the const** — `pkg/wallet/internal/mapping/mapping_list_actions.go`, next to `ActionStatusFailed` (line 126):

```go
// TODO: Temporary constant - "aborted" ActionStatus is missing in sdk.ActionStatus, adjust this once go-sdk is updated.
// Non-standard over BRC-100; safe on the JSON transport this repo uses. Tracks the #959 ts-stack follow-up.
const ActionStatusAborted sdk.ActionStatus = "aborted"
```

- [ ] **Step 4: Add the case** — in `mapActionStatus` (`mapping_list_actions.go:129`), before the `default`:

```go
	case "aborted":
		return ActionStatusAborted, nil
```

- [ ] **Step 5: Run the test to verify it passes**

Run: `go test ./pkg/wallet/internal/mapping/ -run TestMapActionStatus -v`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add pkg/wallet/internal/mapping/mapping_list_actions.go pkg/wallet/internal/mapping/mapping_list_actions_status_test.go
git commit -m "feat: map aborted status across the BRC-100 listActions boundary (#959)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 6: Regression lock — failure-review sweep does not re-fail aborted

Prove that a never-broadcast aborted tx is not swept into `failed` by the Monitor reconciliation net (`FindKnownTxIDsByStatusesNeedingFailureReview`), which selects only `KnownTx` rows already in `{invalidTx, doubleSpend}`. No production change — verification lock.

**Files:**
- Test: extend `pkg/storage/provider_abort_action_test.go` (or a new `pkg/storage/provider_abort_sweep_regression_test.go`)

**Interfaces:**
- Consumes: `activeStorage.AbortAction`, `activeStorage.SynchronizeTxStatuses` if directly callable; otherwise assert via DB state after the sweep entry point used by existing monitor tests.

- [ ] **Step 1: Write the test** — new file `pkg/storage/provider_abort_sweep_regression_test.go`:

```go
package storage_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/internal/fixtures/testusers"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage/internal/testabilities"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk"
	"github.com/bsv-blockchain/go-wallet-toolbox/pkg/wdk/primitives"
)

// TestAbortedTxNotSweptByFailureReview locks that a never-broadcast aborted tx is not
// reclassified to 'failed' by the Monitor failure-review reconciliation, which only
// targets KnownTx rows already in {invalidTx, doubleSpend}. An aborted tx has no such
// KnownTx, so it must stay 'aborted'.
func TestAbortedTxNotSweptByFailureReview(t *testing.T) {
	// given: an aborted tx
	given, cleanup := testabilities.Given(t)
	defer cleanup()
	activeStorage := given.Provider().GORM()
	createResult, _ := given.Action(activeStorage).Created()
	_, err := activeStorage.AbortAction(t.Context(), testusers.Alice.AuthID(), wdk.AbortActionArgs{
		Reference: primitives.Base64String(createResult.Reference),
	})
	require.NoError(t, err)

	// when: the failure-review reconciliation runs (CheckForProofs / SynchronizeTxStatuses).
	// NOTE for implementer: invoke the same entry point existing monitor tests use to
	// drive SynchronizeTxStatuses (search pkg/storage for SynchronizeTxStatuses test
	// callers). If it takes ProvenTxReqStatus args, pass wdk.ProvenTxReqProblematicStatuses.
	_, err = activeStorage.SynchronizeTxStatuses(t.Context(), wdk.ProvenTxReqProblematicStatuses, 100)
	require.NoError(t, err)

	// then: the aborted tx is untouched
	testabilities.ThenDBState(t, activeStorage).
		HasUserTransactionByReference(testusers.Alice, createResult.Reference).
		WithStatus(wdk.TxStatusAborted)
}
```

- [ ] **Step 2: Adjust the sweep entry point to the real signature**

Run: `rg -n "SynchronizeTxStatuses\(" --glob "*_test.go" pkg/storage | grep -v worktree` to find the exact call shape existing tests use; align the `when:` call (name, args, return arity) to match. Then run:
`go test ./pkg/storage/ -run TestAbortedTxNotSweptByFailureReview -v`
Expected: PASS. If it FAILS with the aborted tx flipped to `failed`, that is a real defect in the sweep interaction — stop and investigate before weakening the test.

- [ ] **Step 3: Commit**

```bash
git add pkg/storage/provider_abort_sweep_regression_test.go
git commit -m "test: lock aborted txs against the failure-review sweep (#959)

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>"
```

---

### Task 7: Full-suite verification and issue wiring

**Files:** none (verification + docs)

- [ ] **Step 1: Full build, lint, and test sweep**

Run: `go build ./... && golangci-lint run && go test ./pkg/...`
Expected: all green. Investigate and fix any exhaustive-switch or broken-test fallout (e.g. other fixtures asserting `failed` on an abort path that this change relocated to `aborted`).

- [ ] **Step 2: Grep for stragglers**

Run: `rg -n "TxStatusFailed" pkg/ | grep -v _test | grep -v worktree`
Confirm every remaining `TxStatusFailed` writer is a broadcast-rejection site (`process.go:878/886`, `known_tx.go:150`, `synchronize_tx_statuses.go:498`, `process_unfail.go:114`) and that no abort path still writes `failed`.

- [ ] **Step 3: Update `manual_tests` / examples if they assert status strings** — `rg -n "\"failed\"|TxStatusFailed" manual_tests/ examples/ | grep -v worktree`; adjust any that exercise an abort flow. Commit if changed.

## Follow-ups (out of this PR)

- **ts-stack parity:** open a follow-up issue to add `aborted` to the TypeScript `TransactionStatus` union and a `parity_class:'intended'` BRC-40 conformance vector, then promote to `'required'` once both impls pass. Link issue #959. Note the outbound-sync hazard: a Go wallet emitting `aborted` to an older TS peer relies on TS tolerating an unknown status.
- **go-sdk `ActionStatus`:** the temp consts `ActionStatusFailed` / `ActionStatusAborted` are non-standard over BRC-100; track their removal once go-sdk's `ActionStatus` union (and its binary serializer) gain the values.

## Self-Review

- **Spec coverage:** §1 new value → T1. §2 abort writers → T2. §3 rejection stays failed → T3. §4 enum switches → T1. §5 listing widen + unfail guard → T4. §6 BRC-100 mapper → T5. §7 sweep no-change + regression → T6. §8 out-of-scope → Follow-ups. Testing bullets → T1/T3/T4/T5/T6. All covered.
- **Placeholder scan:** the only deferred detail is the `SynchronizeTxStatuses` call signature in T6 (real function, signature confirmed at runtime via the given rg command) — acceptable because the exact test-harness caller must be matched to existing tests; every code step shows concrete code.
- **Type consistency:** `TxStatusAborted` (T1) used verbatim in T2/T4/T6; `ActionStatusAborted` defined and used in T5; `TxUpdateStatusFailed`/`UTXOStatusUnknown` mappings consistent between the T1 table and the switch edits.
