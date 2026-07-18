# Sync certificates, certificateFields, and commissions — processSyncChunk / getSyncChunk

**Issue:** [bsv-blockchain/go-wallet-toolbox#850](https://github.com/bsv-blockchain/go-wallet-toolbox/issues/850)
**Severity:** High for cross-deployment / multi-storage sync; latent for single-wallet yours-wallet (no certificates today). Certificates are a BRC-100 identity primitive.
**Prior attempt:** Closed (unmerged) implementation PR [#944](https://github.com/bsv-blockchain/go-wallet-toolbox/pull/944) on `fix/850-sync-cert-commission` — useful design sketch; re-validate against current `main` before reusing.

---

## Context for a fresh session

You are implementing full sync support for the three of twelve `SyncChunk` entity arrays that Go currently **silently drops**:

| Wire field | Entity name (`wdk.EntityName`) | Status on `main` |
|------------|--------------------------------|------------------|
| `certificates` | `certificate` | Contract + DB models exist; **no** chunker / upsert / Process branch |
| `certificateFields` | `certificateField` | Same |
| `commissions` | `commission` | Same |

The other nine entities (`outputBasket`, `provenTx`/`provenTxReq`, `transaction`, `output`, `txLabel`/`txLabelMap`, `outputTag`/`outputTagMap`) already round-trip through `getSyncChunk` → `processSyncChunk`.

**RPC-observable failure today:** a TypeScript client that sends certificates or commissions into a Go storage server sees them ignored; a Go `getSyncChunk` always returns empty arrays for those three fields regardless of stored rows. Cross-device / cross-store sync therefore loses identity certificates and commission state without error.

---

## Problem summary + file anchors (current `main`)

### Contract (already complete)

- `pkg/wdk/storage_request_sync_chunk_args.go` — `SyncChunk` fields `Certificates`, `CertificateFields`, `Commissions` (~L61–63); `NewSyncChunk` pre-inits empty slices (~L82–84) so TS never sees `undefined`.
- `pkg/wdk/table_certificate.go`, `table_certificate_field.go`, `table_commission.go` — wire table types.
- `pkg/wdk/entity_name.go` — `CertificateEntityName`, `CertificateFieldEntityName`, `CommissionEntityName` already in `AllEntityNames` (~L33–35).
- `pkg/wdk/sync_map.go` ~L59 — documents that **CertificateField has no idMap** (same class as `TxLabelMap` / `OutputTagMap`).

### DB models (already complete)

- `pkg/internal/storage/database/models/certificate.go` — `Certificate` (`gorm.Model` → soft-delete) with unique index `(type, serial_number, certifier, user_id)`; `CertificateField` with unique index `(field_name, certificate_id)`.
- `pkg/internal/storage/database/models/commission.go` — unique index `(user_id, transaction_id)`.

### Local CRUD (already complete; not sync)

- `pkg/internal/storage/repo/certificates.go` — `CreateCertificate`, `DeleteCertificate` (soft-delete via GORM `Delete`), `FindCertifiers`.
- Commission rows are created alongside transactions in the normal action path (see `models.Transaction.Commission`).

### Gaps (the bug)

| Layer | File | Missing piece |
|-------|------|---------------|
| processSyncChunk dispatch | `pkg/storage/internal/sync/chunk_processor.go` `Process` (~L90–142) | No loops for `chunk.Certificates` / `CertificateFields` / `Commissions` |
| emptyChunk | same file ~L702–711 | Does **not** count the three arrays → a chunk that only carries certs/commissions is treated as “done” and resets offsets |
| getSyncChunk chunkers | `pkg/storage/internal/sync/chunkers.go` | Only 8 chunkers registered; no certificate / field / commission chunkers |
| Repository interface | `pkg/storage/internal/sync/repos.interface.go` | No `Find*ForSync` / `Upsert*ForSync` for the three entities |
| syncrepo | `pkg/internal/storage/repo/syncrepo/` | No `sync_certificate.go` / `sync_certificate_field.go` / `sync_commission.go` |
| Sync composite | `pkg/internal/storage/repo/sync.go` | Does not embed the three new syncrepo types |
| Request offsets fixture | `pkg/internal/fixtures/default_request_sync_chunk_args.go` ~L52 | Explicit `// TODO: Add more offsets for other entities when implemented` |
| Entity bag for sync | `pkg/entity/certifier.go` | `Certificate` has no `IsDeleted`; `CertificateField` has no `UserID` / `CertificateID` (needed for upsert) |

Verified still present on `origin/main` as of this plan (2026-07-18): `chunkers.go` lists only baskets/knownTxs/transactions/outputs/labels/labelsMap/tags/tagsMap; `chunk_processor.Process` ends after tag maps.

---

## Root cause

Sync support was implemented entity-by-entity. The wire contract and entity-name list were extended for certificates/fields/commissions early, but neither the **reader** path (`getSyncChunk` chunkers + `Find*ForSync`) nor the **writer** path (`processSyncChunk` upsert branches + `Upsert*ForSync`) was ever filled in. There is no error path — the arrays are simply never iterated or populated — so the failure is silent and easy to miss in yours-wallet (which does not exercise certificates).

Secondary hazard: `emptyChunk()` ignoring the three arrays means a cert-only or commission-only chunk would be treated as the terminal empty chunk, resetting sync-map offsets and marking the cycle done while data is dropped.

---

## Reference TypeScript semantics

Mirror wallet-toolbox (ts-stack) merge entities:

- `EntityCertificate` — `mergeFind` on natural key **serialNumber + certifier + userId** (not the full unique index that also includes `type`). Soft-delete via `isDeleted` / `deleted_at`. `mergeExisting` gated by strict `incoming.updated_at > existing.updated_at` (BRC-40).
- `EntityCertificateField` — natural key **certificateId + fieldName (+ userId)**; **no** idMap; foreign `certificateId` resolved through `syncMap.certificate.idMap`.
- `EntityCommission` — natural key **transactionId + userId**; idMap present; foreign `transactionId` resolved through `syncMap.transaction.idMap`. `mergeExisting` updates **only `isRedeemed`** (plus `updated_at`) — do not rewrite satoshis / lockingScript / keyOffset on update.

Also match:

- `getSyncChunk.ts` entity order / offsets for `certificate`, `certificateField`, `commission`.
- Soft-deleted certificates must still appear in chunks with `isDeleted: true` so the peer can soft-delete (Unscoped find on read).

Go already applies BRC-40 guards in other syncrepo upserts (`sync_output.go`, `sync_transaction.go`, `sync_knowntx.go`); new upserts must follow the same strict-`>` pattern (see `plans/brc40-stale-chunk-guard.md`).

---

## Recommended fix

### 1. Entity extensions (`pkg/entity/certifier.go`)

- Add `IsDeleted bool` to `Certificate` (sync-only signal; not a DB column — maps to `gorm.Model.DeletedAt`).
- Add `UserID int` and `CertificateID uint` to `CertificateField` so the sync upsert can address the natural key without inventing a parallel type.

`pkg/entity/commission.go` already has the fields needed.

### 2. syncrepo (new files)

Follow patterns in `sync_output.go` / `sync_label.go` (existence check → BRC-40 skip → guarded UPDATE → INSERT).

| New file | Methods | Natural key | ID map? | Notes |
|----------|---------|-------------|---------|-------|
| `syncrepo/sync_certificate.go` | `FindCertificatesForSync`, `UpsertCertificateForSync` | `(user_id, serial_number, certifier)` | yes → return writer `certificateID` | **Unscoped** find/update so soft-deleted rows participate; set/clear `deleted_at` from `entity.IsDeleted`; on insert-then-deleted, create then soft-delete |
| `syncrepo/sync_certificate_field.go` | `FindCertificateFieldsForSync`, `UpsertCertificateFieldForSync` | `(user_id, certificate_id, field_name)` | **no** | `certificate_id` is **writer-local** (already translated by chunk_processor). Skip `BeforeCreate` OnConflict DoNothing hooks on insert (`Session{SkipHooks: true}`) so sync creates are real inserts |
| `syncrepo/sync_commission.go` | `FindCommissionsForSync`, `UpsertCommissionForSync` | `(user_id, transaction_id)` | yes → return writer `commissionID` | On update, only mutate `is_redeemed` + `updated_at` (TS parity). Wire `satoshis` is `int64`; model is `uint64` — convert carefully |

All three finds:

- Scope by `user_id`, apply `queryopts` (since + paging).
- Set `Since.TableName` when empty (same pattern as other Find*ForSync).
- Deterministic secondary order after paginate’s `created_at DESC` (e.g. `id` ASC for cert/commission; `field_name, certificate_id` for fields).

Wire both new types through `pkg/internal/storage/repo/sync.go` (embed + `NewSync` constructors).

### 3. Repository interface (`repos.interface.go`)

```go
FindCertificatesForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableCertificate, error)
UpsertCertificateForSync(ctx context.Context, entity *pkgentity.Certificate) (isNew bool, certificateID uint, err error)

FindCertificateFieldsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableCertificateField, error)
UpsertCertificateFieldForSync(ctx context.Context, entity *pkgentity.CertificateField) (isNew bool, err error)

FindCommissionsForSync(ctx context.Context, userID int, opts ...queryopts.Options) ([]*wdk.TableCommission, error)
UpsertCommissionForSync(ctx context.Context, entity *pkgentity.Commission) (isNew bool, commissionID uint, err error)
```

### 4. chunk_processor (`chunk_processor.go`)

In `Process`, **after** existing entity loops and **in this order** (FK dependencies):

1. `chunk.Certificates` → `upsertCertificate`
2. `chunk.CertificateFields` → `upsertCertificateField` (translates `certificateId` via `translateID(..., CertificateEntityName, ...)`)
3. `chunk.Commissions` → `upsertCommission` (translates `transactionId` via `translateID(..., TransactionEntityName, ...)`)

Each upsert:

- Validate chunk user ID vs `p.user` when `chunk.User` is set (same pattern as baskets/labels).
- Map wire table → entity (writer-side user ID = `p.user.ID`).
- Call repo; `incrementOperations(isNew)`.
- Certificate / commission: `updateSyncState(entityName, updatedAt, idDictionary{readerID, writerID})`.
- Certificate field: `updateSyncState(entityName, updatedAt)` **without** idDictionary.

Update `emptyChunk()` to also require:

```go
len(p.chunk.Certificates) == 0 &&
len(p.chunk.CertificateFields) == 0 &&
len(p.chunk.Commissions) == 0
```

### 5. getSyncChunk chunkers

- New `chunker_certificates.go` — `chunkerCertificates` + `chunkerCertificateFields` (mirror `chunker_labels.go`).
- New `chunker_commissions.go` — `chunkerCommissions`.
- Register in `chunkers.go` `all()` **after** tags/maps, certificates **before** certificate fields, commissions last (or anywhere after transactions exist for the same user in storage — chunkers only read, order among independent entities is less critical than Process order, but keep dependency-friendly order for consistency with `AllEntityNames`).

### 6. Relinquish must bump `updated_at` (`certificates.go` `DeleteCertificate`)

GORM’s default soft-delete only sets `deleted_at` and does **not** advance `updated_at`. Since-filter then **never** surfaces the delete on a subsequent sync cycle after `when` has moved past the original create time.

Change `DeleteCertificate` to an Unscoped update that sets both `deleted_at` and `updated_at` to `now` for the matching live row (`deleted_at IS NULL`). Keep “not found” when `RowsAffected == 0`.

### 7. Fixtures / test helpers

- `default_request_sync_chunk_args.go` — replace the TODO with offsets for `certificate`, `certificateField`, `commission` (offset 0).
- Extend empty-chunk / assertion helpers if they hard-code the nine-entity set (`pkg/storage/internal/testabilities/assertions_sync_chunk.go`, `provider_get_sync_chunk_test.go` expectations).

---

## Suggested implementation sketch (not committed code)

### BRC-40 upsert skeleton (certificate)

```go
// Pre-flight under Unscoped (include soft-deleted):
//   WHERE user_id = ? AND serial_number = ? AND certifier = ?
// if exists && !incoming.UpdatedAt.After(existing.UpdatedAt) → return existing.ID, isNew=false
// if exists && newer:
//   UPDATE ... WHERE id = ? AND updated_at < ?  -- belt-and-braces race guard
//   set deleted_at = incoming.UpdatedAt if IsDeleted else NULL
// if not found:
//   INSERT; if IsDeleted then soft-delete immediately
```

### Process branch order

```
baskets → provenTxReqs → provenTxs → transactions → outputs
→ labels → labelMaps → tags → tagMaps
→ certificates → certificateFields → commissions   // NEW
→ UpdateSyncState
```

Commissions **must** run after transactions in the same chunk (idMap for `transactionId`). Certificate fields **must** run after certificates (idMap for `certificateId`).

---

## Test strategy

### Unit (syncrepo)

New `syncrepo/sync_cert_commission_test.go` (or per-entity files):

**Certificate**

1. Insert new → `isNew=true`, row present, id non-zero.
2. Newer update mutates subject/signature/etc.
3. Stale / equal `updated_at` skips (fields unchanged) — BRC-40.
4. Soft-delete via `IsDeleted=true` on newer chunk → `deleted_at` set; Find*ForSync (Unscoped) returns `IsDeleted=true`.
5. Re-insert / restore: newer chunk with `IsDeleted=false` clears `deleted_at`.
6. Lookup by `(serial, certifier, user)` even when DB unique index also includes `type`.

**Certificate field**

1. Insert under writer certificate id.
2. Newer update changes `field_value` / `master_key`.
3. Stale skip.
4. Natural key conflict does not silently no-op on first insert (hooks skipped).

**Commission**

1. Insert under writer transaction id; idMap-relevant id returned.
2. Newer update flips `is_redeemed` only (satoshis/script unchanged even if entity carries different values).
3. Stale skip.

### Integration (storage provider)

New `pkg/storage/provider_sync_cert_commission_test.go`:

1. **getSyncChunk** — insert certificate (+ fields) via `InsertCertificateAuth`; request chunk with the three offsets; assert arrays non-empty and field values round-trip (`isDeleted=false`).
2. **getSyncChunk after relinquish** — after `DeleteCertificate` with bumped `updated_at`, a since-filtered chunk includes the cert with `isDeleted=true`.
3. **SyncToWriter e2e** — source storage with certs + fields (+ commission if easy to seed via createAction path) → `SyncToWriter` → backup storage lists the same certificates / fields / commission state; writer idMaps populated for certificate and commission.
4. **Cert-only chunk does not mark done** — process a chunk that only has certificates; `Done` must be false and inserts > 0 (guards the `emptyChunk` fix).

### Regression

- Existing `TestGetSyncChunk*`, `TestSyncProcess*`, certificate list/insert tests still pass.
- `go test ./pkg/internal/storage/repo/syncrepo/ ./pkg/storage/ -count=1` green.

---

## Acceptance criteria

- [ ] `getSyncChunk` returns stored certificates, certificate fields, and commissions when offsets request those entities.
- [ ] `processSyncChunk` persists all three; writer sync-map gains `certificate` and `commission` idMap entries; `certificateField` advances count/maxUpdatedAt only.
- [ ] Soft-deleted certificates round-trip (`isDeleted: true`) and peer applies soft-delete.
- [ ] Relinquish bumps `updated_at` so since-filter sees deletes after a completed sync cycle.
- [ ] BRC-40: stale and equal-timestamp chunks do not regress existing rows for all three entities.
- [ ] Commission update path only changes `is_redeemed` (+ timestamps).
- [ ] `emptyChunk` treats cert/field/commission-only payloads as non-empty.
- [ ] Default request-offset fixtures include the three entities.
- [ ] Unit + SyncToWriter integration tests cover the paths above; existing sync tests still pass.

---

## Risks, non-goals, dependencies

### Risks

- **Natural key vs unique index:** DB unique index includes `type`; TS mergeFind uses serial+certifier+user. If two rows differ only by type (should not happen under BRC-100), lookup could be ambiguous — document and follow TS.
- **CertificateField hooks:** `BeforeCreate` OnConflict DoNothing would hide insert failures; must skip hooks on sync insert.
- **Satoshis sign:** wire `int64` vs model `uint64` — reject / convert with the same helpers used elsewhere (`to.UInt64` / `must.ConvertToInt64FromUnsigned`).
- **Order bugs:** processing fields before certificates (or commissions before transactions) will fail `translateID` — keep Process order strict.
- **Prior closed PR #944:** may be cherry-pickable, but re-review against any main movement (BRC-40 guards, numeric_id patterns, provenTx idMap fixes) before reusing.

### Non-goals

- Changing the public RPC shape of `SyncChunk` / entity names.
- Implementing certificate **business** logic (issuance, prove, discover) — only storage sync.
- Promoting ts-stack conformance vectors (none currently tagged for #850); optional follow-up.
- Hard-delete of certificate fields when parent is relinquished (current GORM relation behavior is out of scope unless sync diverges from local delete).

### Dependencies

- BRC-40 stale-chunk guard pattern already on main for other entities (`#853` / `plans/brc40-stale-chunk-guard.md`) — reuse, do not invent a different comparison.
- Transaction idMap must already be populated before commission upserts in the same cycle (existing transaction upsert path).

---

## Estimated size

**M** (medium) — three entity pipelines end-to-end (syncrepo + chunker + process + relinquish tweak + tests), but each follows an established pattern. Roughly 800–1200 LOC including tests if structured like the closed #944 attempt.

---

## Useful cross-references

- Issue: https://github.com/bsv-blockchain/go-wallet-toolbox/issues/850
- Closed design sketch PR: https://github.com/bsv-blockchain/go-wallet-toolbox/pull/944
- Related plan: `plans/brc40-stale-chunk-guard.md` (upsert monotonicity)
- TS interfaces: `packages/wallet/wallet-toolbox/src/sdk/WalletStorage.interfaces.ts` (`SyncChunk`, `RequestSyncChunkArgs`)
- TS chunker: `.../storage/methods/getSyncChunk.ts`
- TS merge entities: `EntityCertificate`, `EntityCertificateField`, `EntityCommission` under `.../storage/schema/entities/`
- Entity name list: `pkg/wdk/entity_name.go`
- Sync orchestration: `pkg/storage/internal/sync/sync_to_writer.go`
)
