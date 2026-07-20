# listOutputs `includeLabels` — reliably return transaction labels

**Issue:** [bsv-blockchain/go-wallet-toolbox#820](https://github.com/bsv-blockchain/go-wallet-toolbox/issues/820)
**Prior closed fix (unmerged):** [#938](https://github.com/bsv-blockchain/go-wallet-toolbox/pull/938) on `fix/820-list-outputs-include-labels`
**Severity:** Medium — RPC/API parity / reliability gap; callers that depend on labels for filtering, display, or classification can get empty data with no error.

---

## Context for a fresh session

You are fixing `listOutputs` so that when `includeLabels: true`, each returned output carries the labels of its parent transaction (TS TxLabel / TxLabelMap model).

On current `origin/main` the flag is accepted and **partial label-loading code already exists**, but:

1. A stale `// TODO: Handle args.IncludeLabels` still claims the feature is unimplemented (adjacent to `// TODO: Handle args.KnownTxids` — **#821**, leave that alone).
2. `GetLabelsForTransactions` errors are **silently ignored** (`if err == nil { labelMap = labels }`), so any repo/DB failure produces empty labels with a successful response.
3. There is **no positive regression test** that labels are populated when the flag is true (storage only asserts labels empty under the minimal filter; wallet sets `IncludeLabels=true` in one suite but never asserts `Labels`).

**Issue-body nuance:** #820 says Go “always returns empty labels.” That was true when the TODO was the whole story. On current main the happy path **already attaches** labels when the fetch succeeds and rows exist — re-probe with the tests below before assuming a deeper persistence bug. The durable defects are: silent error swallowing, missing regression coverage, and the misleading TODO.

TypeScript returns a populated `labels` array per output when `includeLabels: true`. Go must match that and must fail loudly if labels cannot be loaded.

---

## Problem summary (file anchors on current main)

| Location | What is wrong |
|----------|----------------|
| `pkg/storage/internal/actions/list_outputs.go` ~L43–44 | Two TODOs; remove only `// TODO: Handle args.IncludeLabels` (keep KnownTxids / #821) |
| Same file ~L83–93 | Label load only applied when `err == nil`; errors swallowed |
| Same file ~L95–101 | Labels copied from `labelMap[m.TransactionID]` only if map is non-nil |
| `pkg/storage/provider_list_outputs_test.go` ~L56 | Minimal filter asserts `Labels` empty; no `IncludeLabels=true` happy path |
| `pkg/wallet/wallet_list_outputs_test.go` ~L227–256 | Basket-insertion suite sets `IncludeLabels=true` (~L237) but only asserts tags / custom instructions |
| `pkg/wallet/internal/mapping/mapping_list_outputs.go` ~L29, ~L90–92 | Args/result mapping already forwards labels correctly — **no change needed** |

### Data model (already correct)

- Labels are **transaction-level**, not per-output.
- Join table: `models.TransactionLabel` (`pkg/internal/storage/database/models/tx_labels.go`) — composite key `(transaction_id, label_name, label_user_id)`, soft-delete via `DeletedAt`.
- Repo: `Transactions.GetLabelsForTransactions(ctx, txIDs []uint) (map[uint][]string, error)` in `pkg/internal/storage/repo/transactions.go` ~L632–664 (uses `Model(&models.TransactionLabel{})` so GORM soft-delete applies).
- Create / internalize already persist labels via `AddLabels` / create-action params (`pkg/storage/internal/actions/create.go`, `internalize.go`).
- `wdk.WalletOutput.Labels` and `wdk.ListOutputsArgs.IncludeLabels` already exist (`pkg/wdk/storage_outputs_args.go` ~L17, ~L32).

### Fixture constants (use these in tests)

| Constant / value | Where |
|------------------|--------|
| `fixtures.CreateActionTestLabel` = `"test_label=true"` | `pkg/internal/fixtures/consts_and_values_fixtures.go` |
| Create-action default labels | `pkg/internal/fixtures/default_valid_create_action_args.go` (uses `CreateActionTestLabel`) |
| Storage internalize labels `label1`, `label2` | `pkg/internal/fixtures/default_internalize_action_args.go` → `DefaultInternalizeActionArgs` |
| Wallet internalize labels `label1`, `label2` | Same file → `DefaultWalletInternalizeActionArgs` |

### Working reference pattern

`listActions` already does this correctly:

```go
// pkg/storage/internal/actions/list_actions_mapping.go ~L90–101
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

Mirror that shape for `listOutputs` (plain `bool` flag instead of `*BooleanDefaultFalse`). Always return a non-nil empty map when the flag is false so the call site needs no nil check.

---

## Root cause

The feature is half-wired: happy-path code attaches labels, but the path is not treated as a first-class required behaviour.

- Silent error swallowing makes failures look like “empty labels” (the symptom reported in #820).
- Missing tests mean the behaviour can regress or never get exercised for labelled create/internalize flows.
- The leftover TODO misleads implementers and reviewers into thinking the work was never started.

Not a schema gap, not a wallet mapping bug, not a missing WDK field.

---

## Recommended fix

### 1. Harden `ListOutputs` label loading

In `pkg/storage/internal/actions/list_outputs.go`:

1. Remove `// TODO: Handle args.IncludeLabels` only (leave `// TODO: Handle args.KnownTxids` alone — that is #821 / separate scope; plan #950 / issue #821).
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

3. Call it after `ListAndCountOutputs` succeeds; on error return immediately (do not continue with empty labels):

```go
labelMap, err := l.loadLabelsIfNeeded(ctx, outputModels, args.IncludeLabels)
if err != nil {
	return nil, err
}
```

4. When mapping outputs, attach labels from the map when present (no outer `labelMap != nil` guard — helper always returns non-nil):

```go
if labels, ok := labelMap[m.TransactionID]; ok {
	out.Labels = slices.Map(labels, func(s string) primitives.StringUnder300 {
		return primitives.StringUnder300(s)
	})
}
```

5. When `includeLabels` is false / omitted, leave `Labels` nil/empty (`json:"labels,omitempty"`).

### 2. Tests (required)

**Storage** — append to `pkg/storage/provider_list_outputs_test.go` (names match #938 for easy reuse):

| Test | Setup | Assert |
|------|--------|--------|
| `TestListOutputs_IncludeLabels_True` | `given.Action(...).Processed()` | At least one output’s `Labels` contains `fixtures.CreateActionTestLabel` (`"test_label=true"`) |
| `TestListOutputs_IncludeLabels_False` | Same seed | Every output has empty `Labels` when flag is false/zero |
| `TestListOutputs_IncludeLabels_Internalize` | `fixtures.DefaultInternalizeActionArgs(t, wdk.BasketInsertionProtocol)` | With true: outpoint’s `Labels` contain each of `internalizeArgs.Labels` (`label1`, `label2`); with false: empty |

Suggested assertion helper for True: walk `result.Outputs` and `slices.Contains(output.Labels, fixtures.CreateActionTestLabel)` (import `"slices"`).

**Wallet** — `pkg/wallet/wallet_list_outputs_test.go`, basket-insertion subtest (already sets `IncludeLabels`/`IncludeTags`/`IncludeCustomInstructions` true after `DefaultWalletInternalizeActionArgs`):

- After existing tag assertions, assert:
  - `result.Outputs[0].Labels` non-empty
  - `Contains` `"label1"` and `"label2"`
- Also flip `IncludeLabels` to false and assert `Labels` empty on a second `ListOutputs` call (locks omit behaviour at the wallet mapping boundary).

**Error path (recommended, optional if hard without DI):** if the actions package already has stub `TransactionsRepo` patterns (see `process_unfail_wiring_test.go`), a unit test that forces `GetLabelsForTransactions` to return an error and expects `ListOutputs` to fail is ideal. Otherwise the storage happy/false paths + listActions parity review are enough for S-size.

### 3. No API / schema changes

- Do not change `wdk.ListOutputsArgs`, `WalletOutput`, SDK mapping, or GORM models.
- Do not change `GetLabelsForTransactions` signature unless a real bug is found while testing (e.g. soft-delete leak); prefer keep scope tight.
- Do not touch the KnownTxids / BEEF path (#821).

### 4. Patch reference (#938)

Closed unmerged PR #938 already has the intended shape:

- `pkg/storage/internal/actions/list_outputs.go` — helper + error return + TODO removal
- `pkg/storage/provider_list_outputs_test.go` — three storage tests above
- `pkg/wallet/wallet_list_outputs_test.go` — label asserts + false-path on basket-insertion suite

Re-implement / cherry-pick concepts onto latest `main`; do not force-push the old `fix/820-*` branch.

---

## Acceptance criteria

- [ ] `includeLabels: true` returns parent-transaction labels on each matching output (create-action and internalize paths).
- [ ] `includeLabels: false` / omitted leaves `labels` empty/omitted on every output (storage + wallet).
- [ ] Failures from `GetLabelsForTransactions` surface as errors from `ListOutputs` (no silent empty success).
- [ ] Stale `// TODO: Handle args.IncludeLabels` removed; KnownTxids TODO left for #821.
- [ ] Storage tests above pass; wallet basket-insertion suite asserts labels (true and false).
- [ ] Existing list-outputs / list-actions tests still pass.
- [ ] No unrelated refactors; no `.go` changes outside the list-outputs label path + tests.
- [ ] Implementation PR uses `Fixes #820` (this plan PR stays `Related to #820` only).

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
- Soft-deleted `TransactionLabel` rows: `GetLabelsForTransactions` uses `Model(&models.TransactionLabel{})`, so GORM’s soft-delete scope should exclude them. Do not switch to raw `Table(...)` without re-adding `deleted_at IS NULL` (compare `GetLabelsForTxIDs` / `GetLabelsForSelectedActions`, which set `deleted_at IS NULL` explicitly because they use `Table(...)`).
- If the new positive tests fail on current main before the code change, dig into persistence (`AddLabels` / create fixture) before rewriting the loader.

**Non-goals**

- `args.KnownTxids` BEEF optimization (#821 / plan #950) — separate issue/PR.
- Per-output labels (not in the TS model).
- Filtering listOutputs *by* labels (only include/return).
- Wallet SDK surface changes beyond asserting existing mapping.
- Closing #820 via this plan PR (plan only — use `Related to #820`).

**Dependencies**

- None. Label persistence and `GetLabelsForTransactions` already land on main.
- Prior unmerged work in #938 is a usable patch reference; re-implement on latest `main` rather than force-pushing that branch.

**Estimated size:** **S** (one helper + error propagation + a handful of tests).

---

## Useful cross-references

- Issue: https://github.com/bsv-blockchain/go-wallet-toolbox/issues/820
- Closed unmerged fix draft: https://github.com/bsv-blockchain/go-wallet-toolbox/pull/938
- Sibling plan for KnownTxids TODO: https://github.com/bsv-blockchain/go-wallet-toolbox/pull/950 (`plans/issue-821-list-outputs-known-txids.md`)
- Sibling list-actions label loader: `pkg/storage/internal/actions/list_actions_mapping.go` (`loadLabelsIfNeeded` ~L90–101)
- Label batch fetch: `pkg/internal/storage/repo/transactions.go` (`GetLabelsForTransactions` ~L632–664)
- Wallet arg/result mapping: `pkg/wallet/internal/mapping/mapping_list_outputs.go`
- Fixtures: `pkg/internal/fixtures/consts_and_values_fixtures.go` (`CreateActionTestLabel`); `default_internalize_action_args.go` (`label1`, `label2`); create-action args in `default_valid_create_action_args.go`
- Example client already passes includeLabels: `examples/wallet_examples/list_outputs/list_outputs.go`

---

## Notes / gotchas

- `IncludeLabels` on storage/WDK is a plain `bool` (default false); on the wallet SDK it is `*bool` mapped via `optional.OfPtr(...).OrZeroValue()`.
- Do not confuse **tags** (per-output, `IncludeTags`) with **labels** (per-transaction, `IncludeLabels`). The wallet basket-insertion test already covers tags; labels are the missing assertion.
- Balance spec-op path (`IsWalletBalanceSpecOp`) returns no outputs — labels are irrelevant there; leave that path alone.
- When preallocating `txIDs`, capacity `len(outputModels)` is enough; duplicates are fine for the `IN` query (or dedupe if you prefer — not required for correctness).
- Helper return value: always non-nil map so call sites can drop the historical `if labelMap != nil` guard.
- Implementation commit message style: Conventional Commits, e.g. `fix(storage): listOutputs includeLabels error handling and tests`.
