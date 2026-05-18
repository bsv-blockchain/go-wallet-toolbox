# BRC-40 stale-chunk guard — `updated_at` monotonicity on sync upserts

**Issue:** [bsv-blockchain/go-wallet-toolbox#853](https://github.com/bsv-blockchain/go-wallet-toolbox/issues/853)
**Conformance vectors:** `conformance/vectors/sync/brc40-user-state.json` in [bsv-blockchain/ts-stack](https://github.com/bsv-blockchain/ts-stack) (added in main branch; spec ref BRC-40)
**Severity:** High — data regression with double-spend hazard on `output.spendable`.

---

## Context for a fresh session

You are picking up a fix for a sync correctness bug. The Go upsert paths in `syncrepo` unconditionally apply incoming sync chunks to existing rows. The TypeScript reference impl (`wallet-toolbox`) gates the UPDATE on strict `incoming.updated_at > existing.updated_at`. Without that guard, an older chunk arriving after a newer local write silently regresses mutable fields.

A new BRC-40 conformance vector set in ts-stack now pins the expected behaviour. After fixing this issue you must wire the Go runner at those vectors so the regression cannot return.

---

## Files to change

All three sit in `pkg/internal/storage/repo/syncrepo/`:

| File | Function | Composite key used | Mutable fields at risk |
|------|----------|--------------------|-----------------------|
| `sync_output.go` | `upsertOutput` (~L123–191) | `(user_id, transaction_id, vout)` | `spendable`, `spent_by`, `basket_name`, `description`, `custom_instructions` |
| `sync_transaction.go` | `UpsertTransactionForSync` (~L85–146) | `(user_id, reference)` | `status`, `proven_tx_id` (via known_tx join), `is_outgoing`, `description` |
| `sync_knowntx.go` | `UpsertKnownTxForSync` (~L79–135) | `tx_id` | `status`, `block_height`, `merkle_path`, `merkle_root`, `block_hash`, `was_broadcast` |

---

## Reference TypeScript semantics

From `wallet-toolbox` (ts-stack):
- `packages/wallet/wallet-toolbox/src/storage/schema/entities/EntityTransaction.ts` — `mergeExisting`
- `.../EntityOutput.ts` — `mergeExisting`
- `.../EntityProvenTx.ts` — `mergeExisting`

Each follows the pattern:

```ts
if (ei.updated_at > this.updated_at) {
  // apply update
}
```

Strict greater-than. Equal timestamps → **no update**. This is intentional: equal `updated_at` means the incoming chunk is not strictly newer than what we already have, and the local write wins on a tie.

---

## Recommended fix

Add `WHERE updated_at < ?` to the UPDATE branch of each upsert. On `RowsAffected == 0` you must distinguish two cases:

1. **No row exists** → fall through to `INSERT` (current behaviour).
2. **Row exists but is newer-or-equal than incoming** → return success without inserting.

You need a second lookup (or a `RETURNING` trick) to tell these apart. Suggested shape using `gorm`:

```go
// Pre-flight: check if row exists at all under the composite key
var existing models.Output
existsErr := tx.Model(&models.Output{}).
    Where("user_id = ? AND transaction_id = ? AND vout = ?",
        model.UserID, model.TransactionID, model.Vout).
    Select("id, updated_at").
    First(&existing).Error

if existsErr == nil {
    // Row exists — only update if incoming is strictly newer.
    if !model.UpdatedAt.After(existing.UpdatedAt) {
        // Stale chunk: skip, preserve local state. Return existing ID.
        outputID = existing.ID
        return false, outputID, nil
    }
    // Apply guarded update
    updateTx := tx.Model(&model).
        Clauses(clause.Returning{Columns: []clause.Column{{Name: "id"}}}).
        Where("id = ? AND updated_at < ?", existing.ID, model.UpdatedAt).
        Select("*").
        Updates(&model)
    // ... existing success path ...
    return false, existing.ID, nil
}

if !errors.Is(existsErr, gorm.ErrRecordNotFound) {
    return false, 0, fmt.Errorf("lookup failed: %w", existsErr)
}

// No row → INSERT (existing branch unchanged)
```

The `WHERE updated_at < ?` clause on the UPDATE itself is **belt-and-braces** — it prevents a TOCTOU race between the existence check and the UPDATE when two sync workers process chunks concurrently for the same user.

### Subtle: strict `<` vs `<=`

TypeScript uses strict `>`. The mirror in SQL is `WHERE existing.updated_at < incoming.updated_at`. **Do not** use `<=` — equal timestamps must not trigger an update. The conformance vector `sync.brc40.merge.tx.error.regression.2` directly asserts this boundary.

### Same-second timestamp collisions

DB column precision matters. If `updated_at` is stored at second precision but the application generates sub-second timestamps, two genuine writes can compare equal after truncation. Confirm the schema precision (milliseconds in `gorm.Model` defaults vs `DATETIME(3)` on MySQL). If you find truncation, raise it as a separate issue — the fix here assumes column precision matches the application's clock precision.

---

## Conformance vectors that pin this fix

File: `conformance/vectors/sync/brc40-user-state.json` (in ts-stack `main`)
Dispatcher: `conformance/runner/ts/dispatchers/sync.ts` (category `brc40-user-state`, channel `brc40/mergeExisting`, channel `brc40/flow` with `finalState`)

The vectors that fail today against the unguarded Go code and must pass after the fix:

| Vector ID | Asserts |
|-----------|---------|
| `sync.brc40.merge.tx.1` | newer incoming → UPDATE (happy-path baseline; must still work) |
| `sync.brc40.merge.tx.error.regression.1` | older incoming MUST NOT regress `status` completed→unsigned and MUST NOT clear `proven_tx_id` |
| `sync.brc40.merge.tx.error.regression.2` | equal `updated_at` → SKIP (strict `>` boundary) |
| `sync.brc40.merge.output.error.regression.1` | output MUST NOT flip `spendable` false→true via stale chunk (double-spend hazard) |
| `sync.brc40.merge.output.error.regression.2` | output MUST NOT clear `spent_by` via stale chunk |
| `sync.brc40.merge.proventx.error.regression.1` | proven tx (knownTx) MUST NOT be overwritten by stale chunk |
| `sync.brc40.flow.regression.1` | two-chunk replay: newer chunk arrives first then stale chunk; final consumer state must reflect the newer one |

Each vector has `tags` including `go-wallet-toolbox-853` for easy filtering.

The vector file is currently `parity_class: intended` in ts-stack — that means CI skips it. Once both TS and Go pass, promote to `required` in a coordinated PR.

---

## Test strategy

### Unit tests (Go side, before wiring the conformance runner)

Add tests in `pkg/internal/storage/repo/syncrepo/sync_output_test.go`, `sync_transaction_test.go`, `sync_knowntx_test.go`:

1. **Happy update** — seed row with `updated_at = T`, upsert with `updated_at = T+1m`, assert mutable fields updated.
2. **Stale skip** — seed row with `updated_at = T+1m`, upsert with `updated_at = T`, assert mutable fields **unchanged** and `isNew == false`.
3. **Equal-timestamp skip** — seed row with `updated_at = T`, upsert with `updated_at = T`, assert mutable fields **unchanged**.
4. **Output spendable regression** — specific test: existing `spendable=false, spent_by=42, updated_at=T+1m`; incoming `spendable=true, spent_by=null, updated_at=T`; assert post-state still has `spendable=false, spent_by=42`.
5. **Transaction status regression** — existing `status=completed, proven_tx_id=1001, updated_at=T+1m`; incoming `status=unsigned, proven_tx_id=null, updated_at=T`; assert post-state still has `status=completed, proven_tx_id=1001`.
6. **No-row insert** — empty table, upsert anything, assert `isNew == true` and row inserted.
7. **Concurrent upsert** (optional, integration) — two goroutines upsert the same composite key with different `updated_at`; final state must equal the newer row regardless of arrival order. Mirrors vector `sync.brc40.flow.regression.1`.

### Cross-impl conformance run

After the unit tests pass, wire the Go conformance runner at the new vector file:

- Mirror the dispatcher contract from `conformance/runner/ts/dispatchers/sync.ts` — channels: `brc40/requestSyncChunk`, `brc40/syncChunk`, `brc40/flow`, `brc40/mergeExisting`.
- For `brc40/mergeExisting`, parse `existing` and `incoming`, compute the merge action against the Go upsert (`update` vs `skip`), and assert it matches `expected.action`.
- For `brc40/flow` with `messages[]` and `expected.finalState`, replay the chunks against an in-memory `syncrepo` and assert the final DB state matches the expected rows.
- Go runner location (planned, per `conformance/VECTOR-FORMAT.md`): `conformance/runner/runner.go`.

---

## Acceptance criteria

- [ ] All three upsert paths in `syncrepo` reject stale chunks under their respective composite keys.
- [ ] Equal-timestamp incoming chunk is treated as stale (no update).
- [ ] Brand-new rows still insert when no existing row matches the composite key.
- [ ] Unit tests cover happy update, stale skip, equal-skip, and the explicit field-regression cases listed above.
- [ ] Go conformance runner wired at `sync.brc40` vectors; all `merge.*` and `flow.regression.*` vectors pass.
- [ ] Vector file in ts-stack promoted from `parity_class: intended` to `required` once both impls green (separate coordinated PR).
- [ ] Existing tests for `syncrepo` still pass (no regression of the happy path).

---

## Useful cross-references

- TS reference impl shape: `packages/wallet/wallet-toolbox/src/sdk/WalletStorage.interfaces.ts` (lines ~480-575 define `RequestSyncChunkArgs`, `SyncChunk`, `ProcessSyncChunkResult`, `SyncProtocolVersion = '0.1.0'`).
- TS sync chunker: `packages/wallet/wallet-toolbox/src/storage/methods/getSyncChunk.ts`.
- TS merge entities: `packages/wallet/wallet-toolbox/src/storage/schema/entities/Entity{Transaction,Output,ProvenTx,ProvenTxReq,OutputBasket,...}.ts`.
- BRC-40 spec: <https://bsv.brc.dev/outpoints/0040>.
- ts-stack conformance META.json `regression_index` carries the entry `"brc40-stale-chunk-regress": "go-wallet-toolbox#853"`.

---

## Notes / gotchas

- The output upsert uses `Select("*")` precisely so that zero-valued fields (e.g. cleared `basket_name`) are applied. After the fix, that semantic only applies when the incoming row is strictly newer — which is correct.
- The transaction upsert composite key is `(user_id, reference)`, not `(user_id, tx_id)`. Newly created (unsigned) transactions have `reference` but no `tx_id` yet, which is why `reference` is the natural key.
- `sync_knowntx.go` also deletes and re-inserts `TxNote` rows on every successful update. After the fix, those side-effects must only fire when the UPDATE actually applies — keep them inside the post-guard branch.
