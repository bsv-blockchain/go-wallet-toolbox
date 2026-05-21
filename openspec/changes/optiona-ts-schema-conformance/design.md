# Design — Option A schema conformance

## Goals

1. Go storage schema is byte-identical to `ts-stack/wallet-toolbox` schema where applicable.
2. Storage dumps round-trip between Go and TS without translation.
3. Cross-impl conformance fixtures work without per-impl forks.
4. Performance at 10 tps target is unaffected; 100 tps degradation bounded under 25%.

## Non-goals

1. Online migration of existing production state — explicitly out of scope; no prod users exist.
2. Eliminating every Go-specific table — `chaintracks_*`, `key_value`, `tx_notes` retained as documented extensions.
3. Backwards-compatible RPC — major version bump expected.

## Schema mapping reference

Pinned ts-stack commit recorded in `ts-pin.md`. Schema source of truth:
`packages/wallet/wallet-toolbox/src/storage/schema/KnexMigrations.ts`

| Capability | TS table | Go table (post-change) | Notes |
|------------|----------|------------------------|-------|
| Proof facts | `proven_txs` | `proven_txs` | split from `known_tx` |
| Pursuit state | `proven_tx_reqs` | `proven_tx_reqs` | split from `known_tx` |
| Wallet txs | `transactions` | `transactions` | gain `provenTxId`, `rawTx` |
| Outputs | `outputs` | `outputs` | gain `sequenceNumber`, etc.; lose `basketName` string |
| UTXO selection | `outputs WHERE spendable` | `outputs WHERE spendable` | `user_utxo` dropped unconditionally |
| Baskets | `output_baskets` | `output_baskets` | surrogate `basketId` |
| Tags | `output_tags` + `output_tags_map` | `output_tags` + `output_tags_map` | rename + surrogate id |
| Labels | `tx_labels` + `tx_labels_map` | `tx_labels` + `tx_labels_map` | rename + surrogate id |
| Users | `users` | `users` | drop `activeStorage`; auto-incr `userId` |
| Certificates | `certificates` + `certificate_fields` | same | `isDeleted` flag |
| Commissions | `commissions` | `commissions` | unchanged |
| Sync state | `sync_states` | `sync_states` | drop `when`/`satoshis`, add `init` |
| Settings | `settings` | `settings` | add `dbtype` |
| Monitor log | `monitor_events` | `monitor_events` | new |
| Chaintracks | (TS: separate package) | `chaintracks_live_header` + `chaintracks_bulk_file` | Go extension; retained |
| KV | (none in TS) | `key_value` | Go extension; retained |
| Tx notes | `proven_tx_reqs.history` JSON column | `proven_tx_reqs.history` JSON column | Go table dropped; folded into TS-conformant JSON shape |
| Numeric ID lookup | (none in TS) | (removed) | replaced by native auto-increment surrogates |

## Sequencing rationale

16 phases. Each independently mergeable. Sequence is bottom-up: convention pass (low risk, mechanical) before structural changes (high risk, semantic).

**Why renames (Phase 4) before tx_notes fold (Phase 5) before split (Phase 6):** Renames are mechanical. Folding `tx_notes` adds the `history`/`notify` JSON columns to the still-merged `KnownTx` model. The Phase 6 split then carries those columns into the new `ProvenTxReq` model. Doing tx_notes first means the split phase doesn't have to reason about both the table split AND the JSON column introduction simultaneously.

**Why `numeric_id_lookup` drop (Phase 7) comes after the split:** the sync layer for `known_tx` still depends on `NumericIDLookup` until Phase 6 introduces `provenTxReqId` as the native surrogate. Dropping the table earlier would break sync. Phase 3 already removes the basket/tag/label consumers, so after Phase 6 the table has zero consumers and can be deleted cleanly.

**Why a partial index is required on `outputs.spendable` (Phase 11):** UTXO selection latency is the only schema-driven perf risk that scales with history length. Dropping `user_utxo` unconditionally is acceptable at 10 tps; the partial/covering index on `(userId, spendable, basketId)` keeps the hot scan narrow as history grows. Phase 15 re-bench validates.

**Why Phase 15 re-bench:** Catch latent regressions that didn't show up in unit tests (lock contention, query planner shifts).

## Risk register

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Join cost on `transactions ⋈ proven_tx_reqs ⋈ proven_txs` in `ListTransactions` exceeds budget | Low | Medium | Index on `transactions.provenTxId` + `proven_tx_reqs.txid` + `proven_txs.txid`. Cached query plan |
| UTXO selection slow after `user_utxo` drop | Medium | Medium | Partial index on `outputs (userId, spendable, basketId)`; Phase 14 re-bench. No fallback path — table will not be reintroduced |
| `history` / `notify` JSON columns store unbounded data on `proven_tx_reqs` | Low | Low | Mirror TS retention policy; cap size if TS does |
| `isDeleted` boolean lacks query-planner stats vs nullable timestamp | Low | Low | Add partial index `WHERE is_deleted = false` |
| Composite-key → surrogate-id migration touches every join | High | High | Phase 3 done in isolation; full test suite gate before next phase |
| Conformance fixtures drift between TS pin and live TS | Medium | Medium | Pin commit; refresh once per phase; final refresh in Phase 14 |
| RPC clients depend on old field names | High | Medium | Major version bump; deprecation note in changelog |

## Decision log

### D1: Drop `user_utxo` unconditionally

Rationale: TS has no equivalent and the table is purely a denormalization for UTXO selection speed. Schema conformance is the priority; perf at 10 tps target is acceptable on `outputs.spendable` index alone. If higher-tps issues surface in production they will be addressed via indexing strategy (partial / covering indexes) rather than reintroducing the divergent table. No benchmark gate — committed.

### D2: Keep `chaintracks_*` tables as documented Go extensions

Rationale: TS keeps chaintracks in a separate package, not in wallet storage. Go bundles them. This isn't a schema-conformance concern because conformance fixtures don't exercise chaintracks tables — they're internal to header tracking. Cheap to retain, expensive to relocate.

### D3: Migrate `users.userId` to native auto-increment

Rationale: TS uses auto-increment. `NumericIDLookup` was Go's workaround for a per-table id-allocator pattern. Switching simplifies code and removes a whole table.

### D4: Soft-delete via `isDeleted` boolean

Rationale: TS pattern. Gorm's `DeletedAt` adds an implicit scope that conformance tests would have to be aware of. Explicit boolean is simpler and aligns. Cost: every query against soft-deletable tables needs explicit WHERE.

### D5: Use openspec, not free-form `plans/` markdown

Rationale: This change is multi-phase with delta specs. OpenSpec's structure (proposal + tasks + design + specs/) gives clear gates between phases and a path to archive. Free-form markdown in `plans/` works for single-PR fixes (BRC-40 guard) but doesn't scale to a 16-phase refactor.

### D6: Drop `tx_notes` table; fold into `proven_tx_reqs.history` JSON column

Rationale: TS stores audit/history notes as a JSON-serialized `ProvenTxReqHistory` blob on the `proven_tx_reqs.history` text column, never as a separate table. Go's `tx_notes` table is functionally equivalent but diverges in shape. To meet the conformance bar, Go drops the dedicated table and adopts the TS column + struct layout byte-for-byte. The `notify` column (`ProvenTxReqNotify`) is added at the same time for parity. Helper methods on the entity layer mirror TS API (`AddHistoryNote`, `HistorySince`, `HistoryPretty`, `GetHistorySummary`).

### D7: Drop `numeric_id_lookup` after Phase 6, not before

Rationale: `numeric_id_lookup` is a sync-layer workaround for Go's lack of native surrogate IDs on entities that TS keys by auto-incr int. The four consumers are baskets, tags, labels, and known_tx. Phase 3 removes the first three by adding surrogates to those models and rewiring sync. Phase 6 removes the last by splitting `known_tx` into `proven_tx_reqs` with a native `provenTxReqId`. Only after both phases does the table have zero consumers and can be deleted. Phase 7 is the dedicated drop.
