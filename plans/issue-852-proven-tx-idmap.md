# provenTx / provenTxReq idMap never populated in processSyncChunk

**Issue:** [bsv-blockchain/go-wallet-toolbox#852](https://github.com/bsv-blockchain/go-wallet-toolbox/issues/852)
**Severity:** High — silent data corruption of `transaction.provenTxId` on every TS↔Go sync round-trip.
**Prior attempt:** [#939](https://github.com/bsv-blockchain/go-wallet-toolbox/pull/939) (`fix/852-proven-tx-idmap`) implemented the fix but was closed unmerged; bug remains on `main` as of `b7e05a2`.
**Estimated size:** S

---

## Context for a fresh session

You are fixing a sync id-mapping bug. When the Go storage server processes a sync chunk, every entity upsert that carries a foreign key must record `readerID → writerID` in the writer's `SyncMap` via `updateSyncState(..., idDictionary{...})`. Transaction, output, basket, label, and tag already do this. **provenTx and provenTxReq do not.**

The TypeScript client (`wallet-toolbox`) relies on that map when merging transactions:

```ts
// EntityTransaction.mergeExisting
this.provenTxId = ei.provenTxId
  ? syncMap.provenTx.idMap[ei.provenTxId]
  : undefined
```

With an empty `idMap`, the lookup returns `undefined`, which is written to local storage. The transaction's proof linkage is silently severed: `EntityTransaction.getProvenTx()` returns `undefined` for mined txs that still have on-chain proofs.

This affects any deployment where a TS client syncs to a Go server (e.g. yours-wallet → api.1sat.app).

---

## Problem summary + file anchors (current main)

| File | Anchor | What's wrong |
|------|--------|--------------|
| `pkg/storage/internal/sync/chunk_processor.go` | `upsertProvenTxReqs` ~L234 | `p.updateSyncState(wdk.ProvenTxReqEntityName, chunkProvenTxReq.UpdatedAt)` — **no `idDictionary`** |
| `pkg/storage/internal/sync/chunk_processor.go` | `upsertProvenTx` ~L285 | `p.updateSyncState(wdk.ProvenTxEntityName, chunkProvenTx.UpdatedAt)` — **no `idDictionary`** |
| `pkg/storage/internal/sync/repos.interface.go` | `UpsertKnownTxForSync` ~L26 | Signature `(isNew bool, err error)` — no writer numeric ID returned |
| `pkg/internal/storage/repo/syncrepo/sync_knowntx.go` | `UpsertKnownTxForSync` ~L80–158 | Same signature; never creates/returns a `numeric_id_lookup` row on the write path |

Contrast with a correct path (basket ~L195–198):

```go
err = p.updateSyncState(wdk.OutputBasketEntityName, chunkBasket.UpdatedAt, idDictionary{
    readerID: chunkBasket.BasketID,
    writerID: basketNumID,
})
```

And with transaction (~L324–327):

```go
err = p.updateSyncState(wdk.TransactionEntityName, chunkTransaction.UpdatedAt, idDictionary{
    readerID: readerID,
    writerID: transactionID,
})
```

### Why the writer ID is not obvious

Go stores proven txs and proven-tx-reqs in a **single** `known_tx` table keyed by string `tx_id`. The BRC-40-facing numeric IDs (`ProvenTxID` / `ProvenTxReqID`) are **not** PK columns on `known_tx`; they come from `numeric_id_lookup` (`table_name` + `string_id` → `num_id`).

- **Read path is already correct:** `FindKnownTxsForSync` calls `upsertNumericIDLookup`, joins, and maps `model.NumID` → `TableProvenTx.ProvenTxID` / `TableProvenTxReq.ProvenTxReqID` (`sync_knowntx.go` ~L40–77, ~L203–250).
- **Write path never ensures or returns that numeric ID**, so the chunk processor has nothing to put in `idMap[readerID]`.

`updateSyncState` itself is fine (`chunk_processor.go` ~L671–698): when `ids` is empty it only bumps `Count` / `MaxUpdatedAt` and leaves `IDMap` empty — which is exactly what happens for provenTx* today.

---

## Root cause

1. `upsertProvenTx` / `upsertProvenTxReqs` call `updateSyncState` with only `(entityName, updatedAt)`.
2. `UpsertKnownTxForSync` returns only `(isNew, err)`, so even if the call sites wanted to pass an `idDictionary`, they have no writer-side numeric ID.
3. Unlike baskets/labels/tags (which always mint a `numeric_id_lookup` on upsert), the known-tx upsert path never touches `numeric_id_lookup` on write — only on find.

Net effect: after processSyncChunk, `syncState.SyncMap["provenTx"].IDMap` and `["provenTxReq"].IDMap` stay `{}` even when rows were inserted/updated.

---

## Reference TypeScript semantics

From `wallet-toolbox` (ts-stack):

- `packages/wallet/wallet-toolbox/src/storage/schema/entities/EntityTransaction.ts` — `mergeExisting` remaps `provenTxId` through `syncMap.provenTx.idMap`.
- Entity merge for provenTx / provenTxReq records their own idMap entries when the chunk is applied on the writer.
- TS storage uses real integer PKs for provenTx / provenTxReq tables; Go synthesizes the same wire-level IDs via `numeric_id_lookup` so cross-impl sync stays interoperable.

Wire types (already correct on Go):

- `wdk.TableProvenTx.ProvenTxID int` (`json:"provenTxId"`)
- `wdk.TableProvenTxReq.ProvenTxReqID int` (`json:"provenTxReqId"`)
- `wdk.SyncMapEntity.IDMap map[int]int` (`json:"idMap"`)

Intentional non-idMap entities (do **not** change): CertificateField, TxLabelMap, OutputTagMap — see comment on `wdk.SyncMapEntity` in `pkg/wdk/sync_map.go`.

---

## Recommended fix

Mirror the basket/label pattern: make `UpsertKnownTxForSync` return the writer numeric ID, always ensure a `numeric_id_lookup` row exists (including on BRC-40 stale/equal skip), and pass `idDictionary` from both provenTx call sites.

### 1. Extend `UpsertKnownTxForSync`

**Files:**

- `pkg/internal/storage/repo/syncrepo/sync_knowntx.go`
- `pkg/storage/internal/sync/repos.interface.go`
- Call-site updates in existing tests:
  - `pkg/internal/storage/repo/syncrepo/brc40_guard_test.go`
  - `pkg/internal/storage/repo/syncrepo/brc40_conformance_test.go`
  - any other `UpsertKnownTxForSync` callers (`rg UpsertKnownTxForSync`)

**New signature:**

```go
func (s *SyncKnownTx) UpsertKnownTxForSync(ctx context.Context, entity *entity.KnownTx) (isNew bool, knownTxNumID uint, err error)
```

**Inside the existing DB transaction, before the BRC-40 guard:**

```go
numID, numErr := s.saveNumericIDForKnownTx(ctx, tx, entity.TxID)
if numErr != nil {
    return numErr
}
knownTxNumID = numID
```

Helper (same shape as label/tag commons):

```go
func (s *SyncKnownTx) saveNumericIDForKnownTx(ctx context.Context, tx *gorm.DB, txID string) (uint, error) {
    if err := saveNumericIDLookup(ctx, tx, s.tableName(), txID); err != nil {
        return 0, fmt.Errorf("failed to save numeric ID lookup for known tx %q: %w", txID, err)
    }
    return findNumericIDLookup(ctx, tx, s.tableName(), txID)
}
```

`saveNumericIDLookup` / `findNumericIDLookup` already live in `syncrepo/numeric_id.go` and use `ON CONFLICT DO NOTHING`.

**Why mint the numeric ID before the BRC-40 guard:** a stale or equal-timestamp chunk must still contribute an idMap entry. The client needs `readerID → writerID` even when the row body is not applied, so later transaction merges can remap `provenTxId`. Skipping the map entry on stale-skip would reintroduce the corruption for partial re-syncs.

### 2. Pass `idDictionary` from both upserts

In `pkg/storage/internal/sync/chunk_processor.go`:

```go
// upsertProvenTxReqs
isNew, knownTxNumID, err := p.repo.UpsertKnownTxForSync(...)
// ...
err = p.updateSyncState(wdk.ProvenTxReqEntityName, chunkProvenTxReq.UpdatedAt, idDictionary{
    readerID: chunkProvenTxReq.ProvenTxReqID, // already int
    writerID: knownTxNumID,
})

// upsertProvenTx
isNew, knownTxNumID, err := p.repo.UpsertKnownTxForSync(...)
// ...
err = p.updateSyncState(wdk.ProvenTxEntityName, chunkProvenTx.UpdatedAt, idDictionary{
    readerID: chunkProvenTx.ProvenTxID, // already int
    writerID: knownTxNumID,
})
```

No `to.IntFromUnsigned` conversion needed — wire types are already `int` (unlike `TransactionID` which is unsigned).

### 3. getSyncChunk symmetry — no change

`FindKnownTxsForSync` already:

1. Upserts `numeric_id_lookup` for known txs in scope.
2. Joins and selects `num_id` into `KnownTxWithNum.NumID`.
3. Emits `ProvenTxID` / `ProvenTxReqID` from that value.

After the write-path fix, writer IDs in idMap **must equal** the IDs later emitted by the writer's `GetSyncChunk` for the same `tx_id`. That equality is the acceptance test.

---

## Test strategy

### Unit / integration (Go)

Add `TestProcessSyncChunkPopulatesProvenTxIDMap` in `pkg/storage/sync_test.go` (alongside `TestSyncProcess`):

1. Seed source storage for Alice with **one mined** tx (`OwnsMinedTransaction`) and **one unmined** tx (`OwnsTransaction`).
2. `GetSyncChunk` on source; capture `ProvenTxID` / `ProvenTxReqID` keyed by `txid` (reader IDs).
3. `SyncToWriter` into a clean backup provider.
4. On the writer, `FindOrInsertSyncStateAuth` → parse `SyncMap` JSON.
5. Assert:
   - `syncMap[provenTx].IDMap` is non-empty.
   - `syncMap[provenTxReq].IDMap` is non-empty.
   - Every source reader ID maps to a **positive** writer ID.
6. Writer `GetSyncChunk`: for each provenTx / provenTxReq, `idMap[readerID] == chunk.ProvenTxID` (resp. `ProvenTxReqID`).

### Repo-level signature churn

Update existing BRC-40 tests to ignore the new return value:

```go
isNew, _, err := repos.UpsertKnownTxForSync(...)
```

Optionally add a small repo test that `UpsertKnownTxForSync` returns a non-zero `knownTxNumID` on both insert and stale-skip paths, and that a second upsert for the same `tx_id` returns the **same** num ID.

### Suggested local commands

```bash
go test ./pkg/storage/ -run 'TestProcessSyncChunkPopulatesProvenTxIDMap|TestSyncProcess|TestGetSyncChunk' -count=1
go test ./pkg/internal/storage/repo/syncrepo/ -count=1
go test ./pkg/storage/ -run 'TestSync' -count=1
```

---

## Acceptance criteria

- [ ] `upsertProvenTx` passes `idDictionary{readerID: ProvenTxID, writerID: knownTxNumID}` into `updateSyncState`.
- [ ] `upsertProvenTxReqs` passes `idDictionary{readerID: ProvenTxReqID, writerID: knownTxNumID}` into `updateSyncState`.
- [ ] `UpsertKnownTxForSync` returns `(isNew, knownTxNumID uint, err)` and always ensures a `numeric_id_lookup` row for the `tx_id` (including BRC-40 stale/equal skip).
- [ ] Interface + all call sites compile with the new signature.
- [ ] After `SyncToWriter`, writer `SyncMap` has non-empty `provenTx.idMap` and `provenTxReq.idMap`.
- [ ] Writer idMap values match writer `GetSyncChunk` provenTx* IDs for the same txids.
- [ ] Existing syncrepo BRC-40 guards and storage sync tests still pass (no regression of happy path / stale skip).
- [ ] No schema migration beyond existing `numeric_id_lookup` usage.

---

## Risks, non-goals, dependencies

### Risks

- **Stale-skip without num ID:** if numeric ID is only created on insert/update branches, stale chunks leave idMap holes. Mitigated by minting **before** the BRC-40 guard.
- **ID type mismatch:** `idDictionary.writerID` is `uint`; `readerID` is `int`. provenTx wire IDs are `int` (fine for reader). `updateSyncState` already converts writer via `to.IntFromUnsigned`.
- **Dual entity, single table:** both provenTx and provenTxReq share `known_tx` + the same `numeric_id_lookup` table_name. A tx that moves from unmined→mined keeps the same num_id; idMap entries under both entity names may point at the same writer ID for different reader IDs across storage lifetimes — that is correct and matches find-side behaviour.
- **Closed PR #939:** may be reopened/cherry-picked; avoid conflicting force-pushes to `fix/852-proven-tx-idmap`. This plan branch is `plan/852-proven-tx-idmap`.

### Non-goals

- Changing BRC-40 `updated_at` merge semantics (already fixed elsewhere).
- Changing `FindKnownTxsForSync` / getSyncChunk mapping.
- Populating idMaps for CertificateField / TxLabelMap / OutputTagMap (intentionally empty).
- TS-side changes — TS already consumes the map correctly when present.
- Inventing notify / history behaviour (see #851).

### Dependencies

- Existing helpers: `saveNumericIDLookup`, `findNumericIDLookup` in `syncrepo/numeric_id.go`.
- Existing `idDictionary` / `updateSyncState` in `chunk_processor.go`.
- Test fixtures: `testabilities.GivenSyncFixture`, `OwnsMinedTransaction`, `OwnsTransaction`, `StorageManager.SyncToWriter`.

---

## Useful cross-references

- Issue: https://github.com/bsv-blockchain/go-wallet-toolbox/issues/852
- Closed (unmerged) fix PR with nearly the same diff + test: https://github.com/bsv-blockchain/go-wallet-toolbox/pull/939
- Sibling plan style: `plans/brc40-stale-chunk-guard.md` (#853)
- TS interfaces: `packages/wallet/wallet-toolbox/src/sdk/WalletStorage.interfaces.ts` (`ProcessSyncChunkResult`, sync map shapes)
- TS merge: `.../EntityTransaction.ts` (`provenTxId` remap via `syncMap.provenTx.idMap`)
- BRC-40 / known-tx upsert: `pkg/internal/storage/repo/syncrepo/sync_knowntx.go`
- Sync map types: `pkg/wdk/sync_map.go`, entity names in `pkg/wdk/entity_name.go`

---

## Notes / gotchas

- `KnownTx` has no integer PK usable as provenTxId; **always** go through `numeric_id_lookup` with `table_name = known_tx table` and `string_id = tx_id`.
- Do not use `chunkProvenTx.ProvenTxID` as the writer ID — that is the **reader** storage's ID. Writer ID must come from the local lookup after upsert.
- `Count` / `MaxUpdatedAt` already update today; only the missing `IDMap` entries are the bug. Keep count semantics unchanged.
- If reusing #939's test almost verbatim, prefer it: it already asserts source reader IDs, non-empty maps, positive writer IDs, and getSyncChunk equality.
- Issue has an inactivity close warning; landing this plan PR (`Related to #852`, not `Fixes`) keeps the issue open while documenting the approach for a follow-up code PR.
