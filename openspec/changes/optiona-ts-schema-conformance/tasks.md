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
- [ ] Update upsert paths in sync repos to look up surrogate IDs by (name, userId) before inserting map rows.

## Phase 4 — Drop NumericIDLookup

- [ ] Confirm all consumers of `NumericIDLookup` can switch to native auto-increment IDs.
- [ ] Remove `models/numeric_id_lookup.go`.
- [ ] Remove all repo code referencing it (`pkg/internal/storage/repo/`).
- [ ] Update RPC layer that previously translated string IDs via this table.

## Phase 5 — Table renames

- [ ] Rename `tags` → `output_tags`.
- [ ] Rename `output_tags` (M2M) → `output_tags_map`.
- [ ] Rename `labels` → `tx_labels`.
- [ ] Rename `transaction_labels` → `tx_labels_map`.
- [ ] Update all gorm model TableName() overrides if needed.
- [ ] Update genquery references.
- [ ] Update repo file names and Go package contents (`pkg/internal/storage/repo/labels.go`, etc.).

## Phase 6 — Split `known_tx` → `proven_tx_reqs` + `proven_txs`

- [ ] Create `ProvenTx` model: txid PK proxy, `provenTxId` auto-incr, `height`, `index`, `merklePath`, `rawTx`, `blockHash`, `merkleRoot`. Unique on `txid`.
- [ ] Refactor `KnownTx` → `ProvenTxReq`: `provenTxReqId` auto-incr PK, `provenTxId` nullable FK to `ProvenTx`, `status`, `attempts`, `rebroadcastAttempts`, `notified`, `txid` unique, `batch`, `history` JSON, `notify` JSON, `rawTx`, `inputBEEF`.
- [ ] Update `Transaction` model: add `provenTxId` int FK (nullable), add `rawTx` blob.
- [ ] Update sync repo `sync_knowntx.go` → `sync_proven_tx_req.go` and add `sync_proven_tx.go` for proven_tx upserts.
- [ ] Update monitor / sync tasks that promote a req to proven state — must INSERT into proven_txs and UPDATE proven_tx_reqs.provenTxId.
- [ ] Update `ListTransactions` provider method to join `transactions` ⋈ `proven_tx_reqs` ⋈ `proven_txs`.
- [ ] Update `get_beef` and all proof-fetching code paths.
- [ ] Verify all status enum transitions still semantically correct after split.

## Phase 7 — Add `monitor_events` table

- [ ] Add `MonitorEvent` model: `id` auto-incr PK, `event` string, `details` text.
- [ ] Add repo helpers for append.
- [ ] Wire into monitor task lifecycle (event emission TBD against TS reference).

## Phase 8 — Column additions on existing tables

- [ ] `outputs`: add `sequenceNumber`, `spendingDescription`, `scriptLength`, `scriptOffset`, `txid`.
- [ ] `outputs`: replace `basketName` string with `basketId` int FK to `output_baskets`.
- [ ] `transactions`: add `provenTxId` FK (done in Phase 6), add `rawTx` blob.
- [ ] `settings`: add `dbtype` column.
- [ ] `sync_states`: add `init` boolean.
- [ ] Update default values: `output_baskets.numberOfDesiredUTXOs` 32→6, `minimumDesiredUTXOValue` 1000→10000.

## Phase 9 — Column removals (breaking)

- [ ] `users`: drop `ActiveStorage` (no TS equivalent). Update any code that references it.
- [ ] `sync_states`: drop `When`, `Satoshis`. Update sync code paths.

## Phase 10 — Drop `user_utxo`

- [ ] Benchmark: at simulated 100 tps × 6 months of history, measure UTXO selection latency using `outputs.spendable` index alone.
- [ ] If acceptable: delete `user_utxo` model and table; rewrite UTXO selection to query `outputs WHERE spendable = true AND user_id = ?`.
- [ ] If unacceptable: retain `user_utxo` as Go extension; document deviation from TS in `proposal.md`.

## Phase 11 — Conformance test alignment

- [ ] Point `conformance/` runner at the TS-aligned schema.
- [ ] Verify all existing BRC conformance vectors still pass.
- [ ] Add new conformance vectors that exercise the split tables (`proven_txs` lifecycle, `monitor_events` emission).
- [ ] Verify byte-identical storage dump can be loaded by TS implementation (round-trip test).

## Phase 12 — RPC layer

- [ ] Regenerate RPC DTOs to use new field names.
- [ ] Update RPC server stubs in `pkg/storage/rpcserver/`.
- [ ] Update `v1adapter` if it exposes old field names.
- [ ] Pin RPC schema version bump (major version increment).

## Phase 13 — Documentation

- [ ] Update `docs/wallet.md` to reflect new tables/columns.
- [ ] Add migration note in CHANGELOG: this is a breaking schema change; existing storage is incompatible.
- [ ] Update any architecture diagrams referencing the old `known_tx` table.

## Phase 14 — Post-migration validation

- [ ] Re-run baseline benchmark; compare against pre-Option-A numbers. Document any regression > 20%.
- [ ] Full conformance test sweep against ts-stack pinned commit.
- [ ] Storage dump round-trip test (Go → TS → Go).
- [ ] Update `ts-pin.md` to latest ts-stack commit; rerun conformance.

## Phase 15 — Archive change

- [ ] Verify all phases merged.
- [ ] Run `openspec archive` to merge deltas into main specs.
- [ ] Tag release.
