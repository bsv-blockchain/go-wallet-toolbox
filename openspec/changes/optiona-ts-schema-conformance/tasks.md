# Tasks — Option A schema conformance

Sequenced for review-ability. Each phase is an independently mergeable PR. Do **not** combine phases into one PR.

## Phase 0 — Audit and baseline

- [ ] Snapshot current ts-stack schema at a pinned commit; record commit SHA in `openspec/changes/optiona-ts-schema-conformance/ts-pin.md`.
- [ ] Diff TS schema against Go models; produce machine-readable diff file (`schema-diff.json`).
- [ ] Capture baseline benchmark: load 1M txs, run mixed workload (createAction / internalizeAction / listOutputs / listActions / listTransactions) at 10/50/100 tps. Save results to `bench/baseline-pre-optionA.md`.
- [ ] Capture baseline schema dump and conformance vector results.

## Phase 1 — Column naming convention (snake_case → camelCase)

- [ ] Mechanical pass: add explicit `gorm:"column:<camelCase>"` tags on every column across all models in `pkg/internal/storage/database/models/`.
- [ ] Update column references in all repo queries.
- [ ] Update column references in `genquery/` generators; regenerate.
- [ ] Update sync repos in `pkg/internal/storage/repo/syncrepo/`.
- [ ] Update entity layer in `pkg/internal/storage/entity/`.
- [ ] Run existing test suite; fix breakages.
- [ ] Migration: rewrite `pkg/internal/storage/repo/migrator.go` to recreate tables from scratch (no preservation of old data, per breaking-change policy).

## Phase 2 — Soft delete: DeletedAt → isDeleted boolean

- [ ] Remove `gorm.DeletedAt` from: `Certificate`, `OutputBasket`, `Label`, `Tag`, `OutputTag`, `TransactionLabel`.
- [ ] Add `IsDeleted bool` column to each.
- [ ] Replace gorm scope-based soft-delete with explicit `WHERE is_deleted = false` in every query against these tables.
- [ ] Update repo helpers / query builders to default-filter on `isDeleted`.
- [ ] Verify no implicit gorm scope leaks remain (grep for `Unscoped()`).

## Phase 3 — Surrogate IDs on composite-key tables

- [ ] `OutputBasket`: replace composite (Name, UserID) PK with auto-incr `BasketID`. Keep (Name, UserID) as unique index.
- [ ] `Tag` (rename later): replace composite (Name, UserID) PK with auto-incr `OutputTagID`. Keep (Name, UserID) unique.
- [ ] `Label` (rename later): replace composite (Name, UserID) PK with auto-incr `TxLabelID`. Keep (Name, UserID) unique.
- [ ] `User`: switch `UserID` from manual to `autoIncrement`. Update all FKs.
- [ ] `OutputTag` (join, rename later): use `OutputTagID` + `OutputID` instead of (TagName, TagUserID, OutputID).
- [ ] `TransactionLabel` (join, rename later): use `TxLabelID` + `TransactionID` instead of (LabelName, LabelUserID, TransactionID).
- [ ] Update all FK definitions in `Output`, `Transaction`, `Certificate`, etc.
- [ ] **Sync rewire**: in `pkg/internal/storage/repo/syncrepo/sync_basket.go`, `label_tag_commons.go`, `label_tag_map_commons.go`, `sync_output.go`, `sync_transaction.go`, replace `upsertNumericIDLookup` / `joinWithNumericIDLookupScope` / `findNumericIDLookup` / `saveNumericIDLookup` calls for baskets, tags, labels with direct lookups against the new surrogate ID columns.
- [ ] Verify `NumericIDLookup` remains in use **only** for `known_tx` after this phase (becomes the sole consumer until Phase 6).

## Phase 4 — Table renames

- [ ] Rename `tags` → `output_tags`.
- [ ] Rename `output_tags` (M2M) → `output_tags_map`.
- [ ] Rename `labels` → `tx_labels`.
- [ ] Rename `transaction_labels` → `tx_labels_map`.
- [ ] Update all gorm model TableName() overrides if needed.
- [ ] Update genquery references.
- [ ] Update repo file names and Go package contents (`pkg/internal/storage/repo/labels.go`, etc.).

## Phase 5 — Fold `tx_notes` into `proven_tx_reqs.history` (TS-conformant)

- [ ] Define Go structs matching TS exactly:
  - `ProvenTxReqHistory { Notes []ReqHistoryNote }`
  - `ReqHistoryNote { When *string; What string; Extras map[string]any }` — JSON marshalled with `what` required, `when` optional, all extras flattened to siblings
  - `ProvenTxReqNotify { TransactionIDs []int64 }`
- [ ] Add `History string` (default `"{}"`) and `Notify string` (default `"{}"`) columns to the `KnownTx` model (becomes `ProvenTxReq` in Phase 6).
- [ ] Implement entity helpers in `pkg/entity/`:
  - `AddHistoryNote(note ReqHistoryNote, noDupes bool)` — append, optional dedup by `what`
  - `HistorySince(t time.Time) ProvenTxReqHistory`
  - `HistoryPretty(since *time.Time, indent int) string`
  - `GetHistorySummary() ProvenTxReqHistorySummary` — derive `SetToCompleted`, `SetToCallback`, etc.
- [ ] Rewrite all existing `tx_notes` write sites (monitor failures, broadcast events, etc.) as `AddHistoryNote` calls.
- [ ] Update sync merge logic to dedup notes by `(what, when)` and union-sort, matching TS `mergeExisting`.
- [ ] Drop `tx_notes` table + `models/tx_note.go` + `entity/tx_note.go` + `repo/tx_notes.go` + `genquery/tx_notes.gen.go`.
- [ ] Verify byte-identical JSON serialization against TS fixtures (round-trip parse/stringify a TS-generated `history` blob).

## Phase 6 — Split `known_tx` → `proven_tx_reqs` + `proven_txs`

- [ ] Create `ProvenTx` model: auto-incr `provenTxId` PK, `height`, `index`, `merklePath`, `rawTx`, `blockHash`, `merkleRoot`, `txid` unique.
- [ ] Refactor `KnownTx` → `ProvenTxReq`: auto-incr `provenTxReqId` PK, nullable FK `provenTxId` → `proven_txs`, `status`, `attempts`, `rebroadcastAttempts`, `notified`, `txid` unique, `batch`, `history` (from Phase 5), `notify` (from Phase 5), `rawTx`, `inputBEEF`.
- [ ] Update `Transaction` model: add `provenTxId` int FK (nullable), add `rawTx` blob.
- [ ] Rename sync repo `sync_knowntx.go` → `sync_proven_tx_req.go` and add `sync_proven_tx.go` for proven_tx upserts.
- [ ] Update monitor / sync tasks that promote a req to proven state — must INSERT into proven_txs and UPDATE proven_tx_reqs.provenTxId.
- [ ] Update `ListTransactions` provider method to join `transactions` ⋈ `proven_tx_reqs` ⋈ `proven_txs`.
- [ ] Update `get_beef` and all proof-fetching code paths.
- [ ] Verify all status enum transitions still semantically correct after split.
- [ ] **Sync rewire**: replace `NumericIDLookup` usage for known_tx in `sync_proven_tx_req.go` with direct `provenTxReqId` reference. After this phase, `numeric_id_lookup` has zero consumers.

## Phase 7 — Drop `numeric_id_lookup`

- [ ] Confirm zero remaining consumers (grep for `NumericIDLookup`, `numeric_id_lookup`, `upsertNumericIDLookup`, `joinWithNumericIDLookupScope`, `findNumericIDLookup`, `saveNumericIDLookup`).
- [ ] Delete `pkg/internal/storage/database/models/numeric_id_lookup.go`.
- [ ] Delete `pkg/internal/storage/database/genquery/numeric_id_lookups.gen.go`.
- [ ] Delete `pkg/internal/storage/repo/syncrepo/numeric_id.go`.
- [ ] Remove `models.NumericIDLookup{}` from `pkg/internal/storage/repo/migrator.go`.
- [ ] Update any RPC layer that previously translated string IDs via this table.
- [ ] Regenerate gorm/genquery output.

## Phase 8 — Add `monitor_events` table

- [ ] Add `MonitorEvent` model: `id` auto-incr PK, `event` string, `details` text.
- [ ] Add repo helpers for append.
- [ ] Wire into monitor task lifecycle (event emission aligned to TS reference).

## Phase 9 — Column additions on existing tables

- [ ] `outputs`: add `sequenceNumber`, `spendingDescription`, `scriptLength`, `scriptOffset`, `txid`.
- [ ] `outputs`: replace `basketName` string with `basketId` int FK to `output_baskets`.
- [ ] `transactions`: add `provenTxId` FK (done in Phase 6), add `rawTx` blob.
- [ ] `settings`: add `dbtype` column.
- [ ] `sync_states`: add `init` boolean.
- [ ] Update default values: `output_baskets.numberOfDesiredUTXOs` 32→6, `minimumDesiredUTXOValue` 1000→10000.

## Phase 10 — Column removals (breaking)

- [ ] `users`: drop `ActiveStorage` (no TS equivalent). Update any code that references it.
- [ ] `sync_states`: drop `When`, `Satoshis`. Update sync code paths.

## Phase 11 — Drop `user_utxo`

- [ ] Delete `user_utxo` model and table.
- [ ] Rewrite UTXO selection to query `outputs WHERE spendable = true AND user_id = ?` (filter by basket where required).
- [ ] Add partial/covering index on `outputs (userId, spendable, basketId)` to keep selection latency low.
- [ ] Update all UTXO selection code paths in `pkg/storage/internal/actions/` and coin-pick logic.
- [ ] Capture post-change UTXO selection latency for Phase 15 comparison.

## Phase 12 — Conformance test alignment

- [ ] Point `conformance/` runner at the TS-aligned schema.
- [ ] Verify all existing BRC conformance vectors still pass.
- [ ] Add new conformance vectors that exercise the split tables (`proven_txs` lifecycle, `monitor_events` emission, `history` JSON shape).
- [ ] Verify byte-identical storage dump can be loaded by TS implementation (round-trip test).

## Phase 13 — RPC layer

- [ ] Regenerate RPC DTOs to use new field names.
- [ ] Update RPC server stubs in `pkg/storage/rpcserver/`.
- [ ] Update `v1adapter` if it exposes old field names.
- [ ] Pin RPC schema version bump (major version increment).

## Phase 14 — Documentation

- [ ] Update `docs/wallet.md` to reflect new tables/columns.
- [ ] Add migration note in CHANGELOG: this is a breaking schema change; existing storage is incompatible.
- [ ] Update any architecture diagrams referencing the old `known_tx` table.

## Phase 15 — Post-migration validation

- [ ] Re-run baseline benchmark; compare against pre-Option-A numbers. Document any regression > 20%.
- [ ] Full conformance test sweep against ts-stack pinned commit.
- [ ] Storage dump round-trip test (Go → TS → Go).
- [ ] Update `ts-pin.md` to latest ts-stack commit; rerun conformance.

## Phase 16 — Archive change

- [ ] Verify all phases merged.
- [ ] Run `openspec archive` to merge deltas into main specs.
- [ ] Tag release.
