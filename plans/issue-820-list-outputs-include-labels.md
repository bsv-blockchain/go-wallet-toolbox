# listOutputs `includeLabels` — reliably return transaction labels

**Issue:** [bsv-blockchain/go-wallet-toolbox#820](https://github.com/bsv-blockchain/go-wallet-toolbox/issues/820)
**Prior closed fix (unmerged):** [#938](https://github.com/bsv-blockchain/go-wallet-toolbox/pull/938) on `fix/820-list-outputs-include-labels`
**Severity:** Medium — RPC/API parity gap; callers that depend on labels for filtering, display, or classification get empty data with no error.

---

## Context for a fresh session

You are fixing `listOutputs` so that when `includeLabels: true`, each returned output carries the labels of its parent transaction (TS TxLabel / TxLabelMap model).

On current `origin/main` the flag is accepted and label-loading code exists, but:

1. A stale `// TODO: Handle args.IncludeLabels` still claims the feature is unimplemented.
2. `GetLabelsForTransactions` errors are **silently ignored** (`if err == nil { labelMap = labels }`), so any repo/DB failure produces empty labels with a successful response.
3. There is **no positive regression test** that labels are populated when the flag is true (storage only asserts labels empty under the minimal filter; wallet sets `IncludeLabels=true` in one suite but never asserts `Labels`).

TypeScript returns a populated `labels` array per output when `includeLabels: true`. Go must match that.

---

## Problem summary (file anchors on current main)

| Location | What is wrong |
|----------|----------------|
| `pkg/storage/internal/actions/list_outputs.go` ~L43–44 | Stale TODO: `// TODO: Handle args.IncludeLabels` |
| Same file ~L83–93 | Label load only applied when `err == nil`; errors swallowed |
| Same file ~L95–101 | Labels copied from `labelMap[m.TransactionID]` only if map is non-nil |
| `pkg/storage/provider_list_outputs_test.go` ~L56 | Minimal filter asserts `Labels` empty; no `IncludeLabels=true` happy path |
| `pkg/wallet/wallet_list_outputs_test.go` ~L237–253 | Basket-insertion suite sets `IncludeLabels=true` but only asserts tags / custom instructions |
| `pkg/wallet/internal/mapping/mapping_list_outputs.go` ~L29, ~L90–92 | Args/result mapping already forwards labels correctly — **no change needed** |

### Data model (already correct)

- Labels are **transaction-level**, not per-output.
- Join table: `models.TransactionLabel` (`pkg/internal/storage/database/models/tx_labels.go`) — composite key `(transaction_id, label_name, label_user_id)`, soft-delete via `DeletedAt`.
- Repo: `Transactions.GetLabelsForTransactions(ctx, txIDs []uint) (map[uint][]string, error)` in `pkg/internal/storage/repo/transactions.go` ~L590–622.
- Create / internalize already persist labels via `AddLabels` / create-action params (`pkg/storage/internal/actions/create.go`, `internalize.go`).
- `wdk.WalletOutput.Labels` and `wdk.ListOutputsArgs.IncludeLabels` already exist (`pkg/wdk/storage_outputs_args.go`).

### Working reference pattern

`listActions` already does this correctly:

```go
// pkg/storage/internal/actions/list_actions_mapping.go ~L89–100
func (l *listActions) loadLabelsIfNeeded(ctx context.Context, txIDs []uint, include *primitives.BooleanDefaultFalse) (map[uint][]string, error) {
	if !include.Value() {
		return map[uint][]string{}, nil
	}
	labelMap, err := l.transactionsRepo.GetLabelsForTransactions(ctx, txIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load labels: %w", err)
	}
	return labelMap, nil
}
```

Mirror that shape for `listOutputs` (bool flag instead of `*BooleanDefaultFalse`).

---

## Root cause

The feature is half-wired: happy-path code attaches labels, but the path is not treated as a first-class required behaviour.

- Silent error swallowing makes failures look like “empty labels” (the symptom in #820).
- Missing tests mean the behaviour can regress or never get exercised for labelled create/internalize flows.
- The leftover TODO misleads implementers and reviewers into thinking the work was never started.

Not a schema gap, not a wallet mapping bug, not a missing WDK field.

---

## Recommended fix

### 1. Harden `ListOutputs` label loading

In `pkg/storage/internal/actions/list_outputs.go`:

1. Remove `// TODO: Handle args.IncludeLabels` only (leave `// TODO: Handle args.KnownTxids` alone — that is #821 / separate scope unless you confirm it is already wired).
2. Extract a helper (same package, unexported):

```go
// loadLabelsIfNeeded fetches transaction labels when includeLabels is true.
// Labels live on the parent transaction (TxLabel / transaction_labels);
// each output inherits its parent tx's labels.
func (l *listOutputs) loadLabelsIfNeeded(
	ctx context.Context,
	outputModels []*pkgentity.Output,
	includeLabels bool,
) (map[uint][]string, error) {
	if !includeLabels {
		return map[uint][]string{}, nil
	}
	txIDs := make([]uint, 0, len(outputModels))
	for _, m := range outputModels {
		txIDs = append(txIDs, m.TransactionID)
	}
	labelMap, err := l.transactionsRepo.GetLabelsForTransactions(ctx, txIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to load labels: %w", err)
	}
	return labelMap, nil
}
```

3. Call it after `ListAndCountOutputs` succeeds; on error return immediately (do not continue with empty labels).
4. When mapping outputs, attach labels from the map when present:

```go
if labels, ok := labelMap[m.TransactionID]; ok {
	out.Labels = slices.Map(labels, func(s string) primitives.StringUnder300 {
		return primitives.StringUnder300(s)
	})
}
```

5. When `includeLabels` is false / omitted, leave `Labels` nil/empty (`json:"labels,omitempty"`).

### 2. Tests (required)

**Storage** — `pkg/storage/provider_list_outputs_test.go` (or adjacent):

| Test | Setup | Assert |
|------|--------|--------|
| `TestListOutputs_IncludeLabels_True` | `given.Action(...).Processed()` (create-action fixture labels, e.g. `fixtures.CreateActionTestLabel`) | At least one output’s `Labels` contains the expected create-action label |
| `TestListOutputs_IncludeLabels_False` | Same seed | Every output has empty `Labels` when flag is false/zero |
| `TestListOutputs_IncludeLabels_Internalize` | `DefaultInternalizeActionArgs` with basket-insertion labels (`label1`, `label2`) | With true: those labels present on the internalized outpoint; with false: empty |

**Wallet** — `pkg/wallet/wallet_list_outputs_test.go`, basket-insertion suite (already sets `IncludeLabels=true`):

- Assert `result.Outputs[0].Labels` is non-empty and contains the internalize fixture labels (`label1`, `label2` from wallet/internalize fixtures).

### 3. No API / schema changes

- Do not change `wdk.ListOutputsArgs`, `WalletOutput`, SDK mapping, or GORM models.
- Do not change `GetLabelsForTransactions` signature unless a real bug is found while testing (e.g. soft-delete leak); prefer keep scope tight.

---

## Acceptance criteria

- [ ] `includeLabels: true` returns parent-transaction labels on each matching output (create-action and internalize paths).
- [ ] `includeLabels: false` / omitted leaves `labels` empty/omitted on every output.
- [ ] Failures from `GetLabelsForTransactions` surface as errors from `ListOutputs` (no silent empty success).
- [ ] Stale `// TODO: Handle args.IncludeLabels` removed.
- [ ] Storage tests above pass; wallet basket-insertion suite asserts labels.
- [ ] Existing list-outputs / list-actions tests still pass.
- [ ] No unrelated refactors; no `.go` changes outside the list-outputs label path + tests.

---

## Verification commands

```bash
go test ./pkg/storage/ -run 'TestListOutputs_IncludeLabels|TestListOutputs_MinimalFilter|TestListOutputs_IncludeTags' -count=1
go test ./pkg/wallet/ -run 'ListOutputs' -count=1
go test ./pkg/storage/internal/actions/ -count=1
```

Optional: exercise `examples/wallet_examples/list_outputs` with `DefaultIncludeLabels = true` against a wallet that has labelled internalized outputs.

---

## Risks / non-goals / dependencies

**Risks**

- Returning errors where labels previously failed silently is a behaviour change: callers may see new errors instead of empty arrays. That is correct and matches `listActions`.
- Labels are shared across all outputs of a transaction — document in test comments so reviewers do not expect per-output label rows.
- Soft-deleted `TransactionLabel` rows: `GetLabelsForTransactions` uses `Model(&models.TransactionLabel{})`, so GORM’s soft-delete scope should exclude them. Do not switch to raw `Table(...)` without re-adding `deleted_at IS NULL`.

**Non-goals**

- `args.KnownTxids` BEEF optimization (#821) — separate issue/PR.
- Per-output labels (not in the TS model).
- Filtering listOutputs *by* labels (only include/return).
- Wallet SDK surface changes beyond asserting existing mapping.
- Closing #820 via this plan PR (plan only).

**Dependencies**

- None. Label persistence and `GetLabelsForTransactions` already land on main.
- Prior unmerged work in #938 is a usable patch reference; re-implement on latest `main` rather than force-pushing that branch.

**Estimated size:** **S** (one helper + error propagation + a handful of tests).

---

## Useful cross-references

- Issue: https://github.com/bsv-blockchain/go-wallet-toolbox/issues/820
- Closed unmerged fix draft: https://github.com/bsv-blockchain/go-wallet-toolbox/pull/938
- Sibling list-actions label loader: `pkg/storage/internal/actions/list_actions_mapping.go` (`loadLabelsIfNeeded`)
- Label batch fetch: `pkg/internal/storage/repo/transactions.go` (`GetLabelsForTransactions`)
- Wallet arg/result mapping: `pkg/wallet/internal/mapping/mapping_list_outputs.go`
- Fixtures with labels: `pkg/internal/fixtures/default_internalize_action_args.go` (`label1`, `label2`); create-action test label constant in wallet fixtures
- Example client already passes includeLabels: `examples/wallet_examples/list_outputs/list_outputs.go`

---

## Notes / gotchas

- `IncludeLabels` on storage/WDK is a plain `bool` (default false); on the wallet SDK it is `*bool` mapped via `optional.OfPtr(...).OrZeroValue()`.
- Do not confuse **tags** (per-output, `IncludeTags`) with **labels** (per-transaction, `IncludeLabels`). The wallet basket-insertion test already covers tags; labels are the missing assertion.
- Balance spec-op path (`IsWalletBalanceSpecOp`) returns no outputs — labels are irrelevant there; leave that path alone.
- When preallocating `txIDs`, capacity `len(outputModels)` is enough; duplicates are fine for the `IN` query (or dedupe if you prefer — not required for correctness).
