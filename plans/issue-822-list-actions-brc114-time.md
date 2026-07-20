# listActions BRC-114 action time label filtering

**Issue:** [bsv-blockchain/go-wallet-toolbox#822](https://github.com/bsv-blockchain/go-wallet-toolbox/issues/822)
**Spec / reference:** BRC-114 time control labels in [bsv-blockchain/ts-stack](https://github.com/bsv-blockchain/ts-stack) (`packages/wallet/wallet-toolbox/src/utility/brc114ActionTimeLabels.ts`, used by `listActionsKnex.ts` / `listActionsIdb.ts`)
**Prior art (closed, unmerged):** [PR #942](https://github.com/bsv-blockchain/go-wallet-toolbox/pull/942) / branch `fix/822-list-actions-brc114-time` — useful sketch; re-implement from this plan against current `main`
**Severity:** Medium — RPC-observable parity gap. Callers using BRC-114 time labels against a Go storage server get unfiltered results and no computed time labels.
**Estimated size:** S–M

---

## Context for a fresh session

You are implementing TypeScript parity for BRC-114 **action time control labels** on `listActions`.

The TypeScript wallet-toolbox treats certain strings inside `ListActionsArgs.labels` as **query controls**, not ordinary DB labels:

| Label form | Meaning |
|------------|---------|
| `"action time from <unix-ms>"` | Inclusive lower bound on transaction `created_at` |
| `"action time to <unix-ms>"` | Exclusive upper bound on transaction `created_at` |
| `"action time <unix-ms>"` (response only) | Computed creation-time label injected when a time filter is active and `includeLabels` is true |

On current Go `main`, these strings are passed straight into ordinary label filters (`labelFilterScope` / `TransactionLabel`). There is **no** parsing, **no** `created_at` range filter, and **no** response injection.

Verified still broken on `origin/main` (tip at plan time: `b7e05a2`): no `pkg/internal/brc114/` package; `toFilterParams` only strips `unfail` and otherwise keeps labels as-is; `ListActionsFilter` has no time fields; `ListAndCountActions` / `buildSelectedActionsSubQuery` / `selectedActionsSubquery` only filter by `user_id`, `status`, and labels.

---

## Problem summary + file anchors on current main

### Public API surface (unchanged)

`wdk.ListActionsArgs` already carries `Labels []primitives.StringUnder300` — BRC-114 piggybacks on that field; **no new JSON fields** are required.

- `pkg/wdk/storage_list_actions_args.go` — `ListActionsArgs` / `ListActionsResult` / `WalletAction.Labels`
- Wallet mapping already forwards labels: `pkg/wallet/internal/mapping/mapping_list_actions.go` (`MapListActionsArgs`)
- Validation: `pkg/internal/validate/validate_list_actions_args.go` — length-only label checks today (`validateLabel` 1–300 chars; no BRC-114 parse)

### Two list paths (important)

`ListActions` and `ListFailedActions` share mapping helpers but **load includes differently**:

| Path | Entry | Labels / I/O loading | BRC-114 impact |
|------|-------|----------------------|----------------|
| **ListActions** | `list_actions.go` `ListActions` (~L35–92) | `GetLabelsForSelectedActions` + `FindInputsAndOutputsForSelectedActions` — each re-applies the **full filter** via subqueries | **Must** add time predicates to all three builders (list/count, labels page, I/O page) |
| **ListFailedActions** | `list_failed_actions.go` (~L19–95) | `loadLabelsIfNeeded` / `fetchInputsOutputs` by **transaction IDs** already returned from `ListAndCountActions` | Today always passes `Labels: nil` into `toFilterParams`, so time controls never arrive on this path. Still update `mapInputsOutputsLabels` call site if signature changes |

Do not confuse with unused-looking helpers in `list_actions_mapping.go` (`fetchInputsOutputs` / `loadLabelsIfNeeded` using `FindInputsAndOutputsWithBaskets` / `GetLabelsForTransactions`) — those are used by **ListFailedActions**, not by `ListActions`.

### Storage list path (where the bug lives)

| File | Role today | Gap |
|------|------------|-----|
| `pkg/storage/internal/actions/list_actions.go` (~L35–92) | Orchestrates filter → list/count → SelectedActions includes → map | Passes filter through; no time awareness |
| `pkg/storage/internal/actions/list_actions_mapping.go` (`toFilterParams` ~L20–52) | Maps args → `entity.ListActionsFilter`; strips only `unfail` | Does not parse/strip BRC-114 control labels; `//nolint:unparam` on error return is reserved for future validation — remove when parse can fail |
| `pkg/storage/internal/actions/list_actions_mapping.go` (`mapInputsOutputsLabels` ~L116–135, `mapLabelsToAction` ~L137–143) | Copies DB labels onto the action; gates on `args.IncludeLabels` | Never injects `"action time {ms}"`; `mapLabelsToAction` only receives `txID`, not `*pkgentity.Transaction` |
| `pkg/internal/storage/entity/list_actions_model.go` | `ListActionsFilter` = user/labels/status/limit/offset | No `CreatedAtFrom` / `CreatedAtTo` / `TimeFilterRequested` |
| `pkg/internal/storage/repo/transactions.go` (`ListAndCountActions` ~L520–565, `buildSelectedActionsSubQuery` ~L570–583, `labelFilterScope` ~L746+) | Status + label scopes only | No `created_at` range |
| `pkg/internal/storage/repo/outputs.go` (`selectedActionsSubquery` ~L463–482) | Same page of action IDs for I/O joins | Must mirror the same time predicates or include maps drift from the list page |

`pkgentity.Transaction` already has `CreatedAt time.Time` (`pkg/entity/transaction.go` ~L12), and GORM models populate it via `mapModelToTransactionEntity` — response injection can use that field once the filter path is fixed. Do **not** confuse with `pkg/internal/storage/entity.ListActionsFilter` (filter DTO only).

### RPC-observable difference

```text
# TS (correct)
labels: ["run", "action time from 1700000000000", "action time to 1700001000000"]
includeLabels: true
→ only actions with created_at in [from, to)
→ remaining ordinary label filter still applies to "run"
→ each returned action includes a computed label "action time <created_at_ms>"

# Go main (broken)
same args
→ labels treated as three literal DB labels
→ no created_at filter
→ "action time from …" / "action time to …" never match real labels → often empty or wrong results under labelQueryMode=all
→ no computed "action time …" response labels
```

---

## Root cause

BRC-114 encodes time-range query controls **inside the existing `labels` array** rather than as dedicated request fields. The TypeScript stack peels those control labels off before ordinary label matching and applies them as SQL `created_at` predicates. Go never implemented that peel-and-filter step, so control labels fall through into `labelFilterScope` as if they were user-attached tag strings.

Secondary gap: even if filtering were added, TS also injects a **computed** per-action time label into the response when `includeLabels && timeFilterRequested`. Go’s `mapLabelsToAction` only clones stored labels.

---

## Reference TypeScript semantics

Source of truth (ts-stack):

- `packages/wallet/wallet-toolbox/src/utility/brc114ActionTimeLabels.ts` — `parseBrc114ActionTimeLabels`, `makeBrc114ActionTimeLabel`
- `packages/wallet/wallet-toolbox/src/storage/methods/listActionsKnex.ts` — applies bounds + response injection
- `packages/wallet/wallet-toolbox/src/storage/methods/listActionsIdb.ts` — same parse/inject for IDB
- Tests: `packages/wallet/wallet-toolbox/test/Wallet/list/listActions.brc114.test.ts`

### Parse rules (must match exactly)

For each label in `args.Labels` (order of processing):

1. Prefix `"action time from "` (note trailing space) → parse remainder as **unix milliseconds**
2. Else prefix `"action time to "` (trailing space) → parse remainder as **unix milliseconds**
3. Anything else → keep as ordinary label
4. **After** BRC-114 peel: existing `unfail` stripping runs on **remaining** labels only

Prefix matching must be **exact prefix**, not substring. A user label that merely contains those words without the control form stays ordinary.

Validation (reject with a clear error, TS uses `WERR_INVALID_PARAMETER`):

- Value must be digits-only (`/^[0-9]+$/`) — reject empty, signs, decimals, hex, whitespace
- Parsed number must be non-negative and ≤ `Number.MAX_SAFE_INTEGER` (`2^53 - 1` = `9007199254740991`) for JS parity
- Duplicate `from` or duplicate `to` → error
- If both present: require `from < to` (strict); equal bounds are invalid
- Presence of either control sets `timeFilterRequested = true` even if the other is absent
- Labels that look almost-control but fail digit parse (e.g. `"action time from abc"`) → error, not ordinary labels

Suggested error strings (keep close to TS for cross-impl logs):

- `"labels: valid. Duplicate action time from label"`
- `"labels: valid. Duplicate action time to label"`
- `"labels: valid. action time from must be less than action time to"`
- invalid number / out-of-range: mirror TS wording where practical

### SQL filter semantics

When `timeFilterRequested`:

```sql
created_at IS NOT NULL
-- if from set:
AND created_at >= :from   -- inclusive
-- if to set:
AND created_at <  :to     -- exclusive
```

Convert unix-ms → `time.Time` via `time.UnixMilli(ms).UTC()` before binding. Apply **before or after** status/label scopes — order among independent `Where` clauses does not change results, but apply in **all three** builders identically.

### Response label injection

When `includeLabels == true` **and** `timeFilterRequested`:

```ts
const timeLabel = `action time ${new Date(tx.created_at).getTime()}`
if (!action.labels.includes(timeLabel)) action.labels.push(timeLabel)
```

Do **not** inject when no time control labels were in the request. Do **not** store these computed labels in the DB. Injection is response-only; `totalActions` and pagination ignore the computed label.

### Interaction with ordinary labels / labelQueryMode

Control labels are **removed** from the set passed to label matching. So:

- `labels: ["run", "action time from 0"]` + `labelQueryMode: all` → require label `run` only (not the time string)
- `labels: ["action time from 0"]` alone → no ordinary label filter; only the time range (and status defaults)
- `labels: ["run"]` alone → unchanged today (no time filter, no injection)

`unfail` special-case continues to run on the remaining labels after BRC-114 stripping (used when wallet appends `"unfail"` for recovery flows).

Permissions manager (TS `WalletPermissionsManager`) also strips time labels before permission checks — **no Go equivalent** re-validates raw labels for grants today (`SeekPermission` is a simple bool). Leave as follow-up only if a permissions layer is added later.

---

## Recommended fix

### Files to touch (implementation PR checklist)

| File | Change |
|------|--------|
| `pkg/internal/brc114/action_time_labels.go` | **new** — parse / make / FromMillis |
| `pkg/internal/brc114/action_time_labels_test.go` | **new** — unit tests |
| `pkg/internal/storage/entity/list_actions_model.go` | add time filter fields |
| `pkg/internal/storage/repo/transactions.go` | apply time predicates in `ListAndCountActions` + `buildSelectedActionsSubQuery` |
| `pkg/internal/storage/repo/outputs.go` | apply same predicates in `selectedActionsSubquery` |
| `pkg/storage/internal/actions/list_actions_mapping.go` | parse in `toFilterParams`; inject in `mapLabelsToAction`; plumb `timeFilterRequested` |
| `pkg/storage/internal/actions/list_actions.go` | compile with any signature changes (filter already flows) |
| `pkg/storage/internal/actions/list_failed_actions.go` | update `mapInputsOutputsLabels` call if signature changes |
| `pkg/internal/validate/validate_list_actions_args.go` | early parse after length checks |
| `pkg/internal/validate/validate_list_actions_args_test.go` | validation cases |
| `pkg/storage/provider_list_actions_brc114_test.go` | **new** — storage integration |

### 1. New helper package `pkg/internal/brc114/`

Add pure parse/make helpers (no storage / GORM deps):

```go
// ParseActionTimeLabels(labels []string) (ParsedActionTimeLabels, error)
// MakeActionTimeLabel(unixMillis int64) string
// FromMillis(unixMillis int64) time.Time  // time.UnixMilli(ms).UTC()
```

Suggested shape:

```go
type ParsedActionTimeLabels struct {
    From                *int64
    To                  *int64
    TimeFilterRequested bool
    RemainingLabels     []string // preserve relative order of non-control labels
}

const MaxSafeInteger int64 = 9007199254740991 // 2^53 - 1
```

### 2. Extend `entity.ListActionsFilter`

In `pkg/internal/storage/entity/list_actions_model.go` (add `"time"` import):

```go
// BRC-114 action time filters (parsed from control labels).
// CreatedAtFrom is inclusive; CreatedAtTo is exclusive.
// Only consulted when TimeFilterRequested is true.
TimeFilterRequested bool
CreatedAtFrom       *time.Time
CreatedAtTo         *time.Time
```

### 3. Wire parse in `toFilterParams`

In `list_actions_mapping.go` (order matters):

1. Map `args.Labels` → `[]string`
2. `ParseActionTimeLabels` → remaining + bounds; **return error** on invalid controls (drop `//nolint:unparam`)
3. Run existing `unfail` stripping on **remaining** labels only
4. Populate filter time fields from `brc114.FromMillis` when From/To non-nil
5. Set `TimeFilterRequested` on the filter

Propagate `timeFilterRequested` into mapping for response injection. Suggested signature changes:

```go
func (l *listActions) mapInputsOutputsLabels(
    actions []wdk.WalletAction,
    txs []*pkgentity.Transaction,
    inputMap, outputMap map[uint][]*pkgentity.Output,
    labelMap map[uint][]string,
    rawTxMap map[string][]byte,
    args *wdk.ListActionsArgs,
    timeFilterRequested bool, // NEW
) error

func (l *listActions) mapLabelsToAction(
    action *wdk.WalletAction,
    tx *pkgentity.Transaction, // was txID uint — need CreatedAt
    labelMap map[uint][]string,
    timeFilterRequested bool,  // NEW
)
```

Update call sites in `list_actions.go` and `list_failed_actions.go`. For ListFailedActions, pass `timeFilterRequested=false` (or read from filter if labels are ever wired later).

### 4. Apply range in **all three** query builders

Keep list/count, labels-for-page, and inputs/outputs page aligned:

| Site | File | Used by |
|------|------|---------|
| `ListAndCountActions` | `pkg/internal/storage/repo/transactions.go` | both list paths |
| `buildSelectedActionsSubQuery` (feeds `GetLabelsForSelectedActions`) | same | **ListActions** only |
| `selectedActionsSubquery` | `pkg/internal/storage/repo/outputs.go` | **ListActions** only |

Prefer a small shared helper. Options (pick one; keep DRY):

1. Package-level helper in `repo` if both types live in same package — they do **not** (`Transactions` vs `Outputs` methods, same `repo` package) → **best**: unexported function in `pkg/internal/storage/repo/list_actions_time_filter.go`:

```go
func applyListActionsTimeFilters(query *gorm.DB, filter entity.ListActionsFilter) *gorm.DB {
    if !filter.TimeFilterRequested {
        return query
    }
    query = query.Where("created_at IS NOT NULL")
    if filter.CreatedAtFrom != nil {
        query = query.Where("created_at >= ?", *filter.CreatedAtFrom)
    }
    if filter.CreatedAtTo != nil {
        query = query.Where("created_at < ?", *filter.CreatedAtTo)
    }
    return query
}
```

Call from all three builders after `user_id` / before or after status+labels.

**Do not** only fix `ListAndCountActions` — otherwise label/include maps for ListActions can attach to the wrong page of IDs when limit/offset is non-zero under a time filter.

### 5. Early validation

In `validate.ListActionsArgs`, after per-label length checks, convert labels to `[]string` and call `ParseActionTimeLabels` so bad control labels fail at the wallet/API boundary before storage. Storage `toFilterParams` should still re-parse (defense in depth / direct storage RPC callers).

Do **not** require that ordinary labels fail if they equal a computed response form like `"action time 0"` — only the control prefixes `from` / `to` are special on input.

### 6. Response injection

In `mapLabelsToAction`, after cloning DB labels, when `timeFilterRequested && !tx.CreatedAt.IsZero()`:

```go
timeLabel := brc114.MakeActionTimeLabel(tx.CreatedAt.UnixMilli())
if !slices.Contains(action.Labels, timeLabel) {
    action.Labels = append(action.Labels, timeLabel)
}
```

Only when `includeLabels` is true (caller already gates via `mapInputsOutputsLabels`).

---

## Test strategy

### Unit — `pkg/internal/brc114/action_time_labels_test.go`

1. Happy parse: from only, to only, both, neither
2. Remaining labels preserved and ordered (controls removed in place)
3. Duplicate from / duplicate to → error
4. `from >= to` → error; `from == to` → error
5. Non-digits, empty value, leading `+`, negative (via `-`), whitespace in value → error
6. Value `> MAX_SAFE_INTEGER` → error; value `== MAX_SAFE_INTEGER` → ok
7. `MakeActionTimeLabel(0)` → `"action time 0"`
8. Ordinary label `"action time 123"` (no from/to) remains in `RemainingLabels` (not a control)
9. Mixed: `["a", "action time from 1", "b", "action time to 9"]` → remaining `["a","b"]`, from=1, to=9

### Validation — `validate_list_actions_args_test.go`

- Valid ordinary labels still pass
- Valid BRC-114 controls pass length + parse
- Invalid BRC-114 controls rejected by `ListActionsArgs`
- Label length still enforced (empty / >300)

### Integration — `pkg/storage/provider_list_actions_brc114_test.go` (new)

Seed several actions for one user with **explicit** `created_at` values (GORM `Updates` after create if needed — SQLite/MySQL/Postgres precision can make “now” flaky). Prefer millisecond-spaced timestamps that survive second-precision stores (e.g. 1s apart) for cross-driver CI, while still documenting ms semantics.

Cover:

| Case | Assert |
|------|--------|
| `from` only | Excludes actions strictly before from; includes boundary |
| `to` only | Excludes actions at/after to (exclusive); includes just below |
| from+to window | Only mid-range actions |
| Control labels stripped from DB label match | `labelQueryMode=all` with `["known-label", "action time from 0"]` still returns rows that have `known-label` |
| `includeLabels=true` + time filter | Each action has `"action time {ms}"` matching its `CreatedAt` |
| `includeLabels=false` + time filter | No labels (or empty); filter still applies |
| Pagination | limit/offset stable under time filter; `totalActions` matches count; includes (labels/outputs) belong only to returned page |
| No time controls | Behavior unchanged vs existing `TestListActions_*` baselines |
| Invalid control label via storage path | Error returned (not silent empty set) |
| `labelQueryMode=any` + ordinary + time | Intersection of time range and any-label match |

### Regression

```bash
gofmt -w pkg/internal/brc114/ pkg/storage/internal/actions/ pkg/internal/storage/ pkg/internal/validate/
go test ./pkg/internal/brc114/ -count=1
go test ./pkg/internal/validate/ -count=1 -run ListActionsArgs
go test ./pkg/storage/ -count=1 -run 'TestListActions'
go test ./pkg/wallet/ -count=1 -run ListActions
```

Optional later: port vectors from TS `listActions.brc114.test.ts` into `conformance/vectors/wallet/` if a shared BRC-114 listactions category is added in ts-stack — not required for the first fix PR.

---

## Acceptance criteria

- [ ] `"action time from <ms>"` / `"action time to <ms>"` are parsed as control labels, not DB labels
- [ ] `created_at >= from` (inclusive) and `created_at < to` (exclusive) applied consistently in list/count, label join subquery, and I/O subquery
- [ ] Invalid / duplicate / unordered time controls rejected with clear errors at validate **and** storage `toFilterParams`
- [ ] When time filter active and `includeLabels`, responses include computed `"action time {ms}"` from `tx.CreatedAt`
- [ ] Ordinary labels + `labelQueryMode` (`any` / `all`) continue to work on the remaining label set
- [ ] Existing listActions tests still pass; no API field additions to `ListActionsArgs`
- [ ] Unit + storage integration coverage as above
- [ ] `gofmt` clean; no schema migration

---

## Risks / gotchas

1. **Milliseconds, not seconds.** Issue text says “unix” generically; TS uses **unix milliseconds**. Using seconds will silently empty the result set for typical wall-clock values (~1.7e12 ms vs ~1.7e9 s).
2. **Exclusive `to`.** Off-by-one if implemented as `<=`. Boundary tests must pin this.
3. **Triple query alignment (ListActions).** Forgetting `outputs.selectedActionsSubquery` or `buildSelectedActionsSubQuery` causes wrong includes/labels for the page even when the main list looks correct — especially under `limit`/`offset`.
4. **DB datetime precision.** SQLite and some MySQL configs store second (or millisecond) precision. Prefer explicit `created_at` writes in tests; space fixtures by ≥1s if running under multi-driver CI.
5. **`labelQueryMode=all` false empty sets.** Today, if someone passes only time-control labels under `all`, Go may look for those literal labels and return empty. After the fix, remainingLabels is empty → no label scope → time-only filter. That is intentional TS parity, not a regression.
6. **MAX_SAFE_INTEGER.** Reject values above `2^53-1` even though Go `int64` can hold more — keeps RPC behavior identical to JS clients.
7. **Prior art branch.** `fix/822-list-actions-brc114-time` / [PR #942](https://github.com/bsv-blockchain/go-wallet-toolbox/pull/942) already sketched this design. Rebase or re-apply carefully against current `main`; do not re-close #822 from a plan PR. Prefer a fresh `fix/822-…` implementation PR that cites this plan and uses `Fixes #822`.
8. **gofmt.** Prior fix PR failed CI on gofmt around `50_000 + i*1_000` style in tests — run `gofmt` before push.
9. **`mapLabelsToAction` needs `CreatedAt`.** Passing only `txID` is insufficient for injection; change the signature to take `*pkgentity.Transaction` (or `createdAt time.Time`).
10. **ListFailedActions.** Currently `Labels: nil` — no BRC-114 time filtering on that RPC unless product later wires labels through. Signature/compile updates only.

---

## Non-goals

- Changing the public `ListActionsArgs` schema (no new `from`/`to` fields)
- Persisting `"action time …"` labels into `bsv_transaction_labels`
- Product behavior change for `listFailedActions` beyond compile-safe reuse of mapping helpers
- Permissions-manager parity (no Go path re-validates raw labels for grants today)
- Conformance-vector promotion in ts-stack (follow-up)

---

## Dependencies

- None on other open Go issues
- TypeScript reference already merged in ts-stack (`brc114ActionTimeLabels.ts` + knex/idb listActions)
- No schema migration — uses existing `transactions.created_at`

---

## Suggested implementation order

1. `pkg/internal/brc114` parse/make + unit tests
2. `ListActionsFilter` fields + `applyListActionsTimeFilters` on all three repo builders
3. `toFilterParams` + label injection + validate wiring + call-site signatures
4. Storage integration tests (explicit `created_at`, boundary + pagination + label strip)
5. Full package regression commands above

**Commit message for the eventual fix PR (not this plan PR):**
`fix: implement BRC-114 action time label filtering in listActions`

**This plan PR only** adds this document under `plans/` — no application code. Does **not** close #822 (`Related to #822` only).
