# Option A — Total schema conformance with `ts-stack/wallet-toolbox`

**Status:** Proposed
**Branch:** `feat/schema-conformance-optiona`
**Author:** Deggen
**Date:** 2026-05-21

## Why

`go-wallet-toolbox` currently ships a storage schema that diverges from the reference TypeScript impl in `ts-stack/wallet-toolbox` in many small and several large ways. Divergence costs:

- Cross-impl conformance fixtures cannot be shared byte-for-byte.
- Storage dumps cannot be moved between Go and TS backends without translation.
- Backporting features in either direction (e.g. `ListTransactions`) requires per-method schema mapping rather than direct lift.
- New BRC specs implicitly assume TS shapes, so Go drifts further every release.
- Engineers context-switching between codebases pay a translation tax on every query.

No production wallet runs against `go-wallet-toolbox` storage yet, so the cost of breaking changes is one-time and bounded. The window to align is now.

This change adopts the TS schema as the canonical shape for both implementations, mutating Go to match. Go-only tables (chaintracks, key/value, numeric-id lookup, user_utxo, tx_notes) are evaluated case by case for retention as extensions or removal in favour of TS-equivalent shapes.

## What changes

### Tables — split

- `known_tx` → `proven_tx_reqs` (pursuit state) + `proven_txs` (proof facts), 1:1 keyed by txid.

### Tables — added

- `monitor_events` — append-only monitor event log.
- `proven_txs` — proof facts table (from split).

### Tables — renamed

| Current (Go) | Target (TS) |
|--------------|-------------|
| `tags` | `output_tags` |
| `output_tags` | `output_tags_map` |
| `labels` | `tx_labels` |
| `transaction_labels` | `tx_labels_map` |
| `known_tx` | `proven_tx_reqs` (+ split to `proven_txs`) |

### Tables — evaluated for retention

| Go-only table | Decision |
|---------------|----------|
| `chaintracks_live_header` | Keep — chaintracks lives in storage; spec-orthogonal |
| `chaintracks_bulk_file` | Keep — chaintracks lives in storage; spec-orthogonal |
| `numeric_id_lookup` | **Drop** — workaround for missing surrogate IDs; eliminated by Phase 3 + Phase 6 |
| `key_value` | Keep — generic KV used by non-spec subsystems |
| `tx_notes` | **Drop** — fold semantics into `proven_tx_reqs.history` JSON column matching TS shape exactly |
| `user_utxo` | **Drop unconditionally** — UTXO selection moves to `outputs WHERE spendable = true` |

### tx_notes → `proven_tx_reqs.history` (TS-conformant)

Go's `tx_notes` table is dropped. Its semantics move into `proven_tx_reqs.history`, a JSON-serialized text column matching the TS `ProvenTxReqHistory` shape exactly:

```
interface ProvenTxReqHistory {
  notes?: ReqHistoryNote[]
}

interface ReqHistoryNote {
  when?: string  // ISO timestamp
  what: string   // required event tag
  [key: string]: boolean | string | number | undefined  // extra attributes
}
```

Plus a parallel `notify` text column for `{ transactionIds?: number[] }`, matching TS `ProvenTxReqNotify`.

Helper methods on the Go entity layer mirror TS:
- `AddHistoryNote(note, noDupes)` — append, optionally dedup by `what`.
- `HistorySince(date)` — filter by `when > date`.
- `HistoryPretty(since, indent)` — human-readable render.
- `GetHistorySummary()` — derive flags from `notes[].what`.

Sync merge dedups notes by `(what, when)` and union-sorts, matching TS `mergeExisting`.

### `numeric_id_lookup` removal

`numeric_id_lookup` is a sync-layer workaround that mints synthetic integer IDs for entities whose Go schema currently uses composite natural keys. The table is dropped once every consumer can reference a native auto-increment surrogate ID:

| Entity | Surrogate gained in | Consumer rewired in |
|--------|---------------------|---------------------|
| `output_baskets` | Phase 3 (`basketId`) | Phase 3 sync rewire |
| `tags` / `output_tags` | Phase 3 (`outputTagId`) | Phase 3 sync rewire |
| `labels` / `tx_labels` | Phase 3 (`txLabelId`) | Phase 3 sync rewire |
| `known_tx` (becomes `proven_tx_reqs`) | Phase 6 (`provenTxReqId`) | Phase 6 sync rewire |

After Phase 3 + Phase 6, `numeric_id_lookup` and every wrapper in `pkg/internal/storage/repo/syncrepo/numeric_id.go` is deleted in a dedicated drop phase.

### Column convention

- camelCase across every gorm column tag — **except** `created_at` / `updated_at`, which TS emits as snake_case (the `addTimeStamps` helper). A blanket camelCase pass would break conformance on these two.
- Soft delete moves from gorm `DeletedAt` timestamp to TS-style `isDeleted` boolean on the six tables TS marks deletable (`certificates`, `output_baskets`, `output_tags`, `output_tags_map`, `tx_labels`, `tx_labels_map`).
- **Drop `gorm.Model` everywhere** — it injects a non-conformant `deleted_at` (and a generic `id`). Replace with an explicit named PK + a `Timestamps` embed. TS has no `deleted_at` on any table. See `target-schema.md`.

### ID strategy

- **Every table** adopts a per-table camelCase surrogate PK matching TS, replacing gorm's generic `id`: `provenTxId`, `provenTxReqId`, `transactionId`, `outputId`, `certificateId`, `commissionId`, `userId`, `basketId`, `outputTagId`, `txLabelId`, `syncStateId`. (`monitor_events` keeps `id`; `settings`, `certificate_fields`, and the `_map` join tables have no surrogate, matching TS.)
- Composite natural keys become UNIQUE indexes, not PKs (e.g. `(name,userId)` on baskets, `(tag,userId)` on output_tags, `(label,userId)` on tx_labels).
- `users.userId` switches from manual `NumericIDLookup` assignment to native auto-increment.
- All join tables key on surrogate FKs, not name+user composite.
- Value-column renames: `tags.name`→`output_tags.tag`, `labels.name`→`tx_labels.label`.

### Column additions

| Table | Columns | Notes |
|-------|---------|-------|
| `users` | drop `activeStorage` (no TS equivalent) | breaking |
| `outputs` | add `sequenceNumber`, `spendingDescription`, `scriptLength`, `scriptOffset`, `txid` | |
| `outputs` | replace `basketName` string with `basketId` int FK | |
| `transactions` | add `provenTxId` int FK (nullable until proven), `rawTx` blob | |
| `settings` | add `dbtype` | |
| `sync_states` | add `init` boolean | |
| `sync_states` | **keep** `when`, `satoshis` | present in TS — Decision #2 revised |
| `certificates` | replace `DeletedAt` with `isDeleted` boolean | |
| `output_baskets` | replace `DeletedAt` with `isDeleted` boolean | |
| `output_tags` / `tx_labels` | replace `DeletedAt` with `isDeleted` boolean | |

### Default values aligned to TS

- `output_baskets.numberOfDesiredUTXOs`: 32 → 6
- `output_baskets.minimumDesiredUTXOValue`: 1000 → 10000

(Verify against current TS defaults before finalizing; values above are TS migration defaults.)

## Impact

### Affected capabilities

- `storage-schema` (this change's primary spec)
- `storage-rpc` (RPC DTOs reference renamed fields)
- `storage-sync` (BRC-40 sync uses table names + column names)
- `wallet-actions` (createAction/internalizeAction code paths)
- `chaintracks-storage` (unaffected; tables retained)

### Affected code surfaces

- All gorm models in `pkg/internal/storage/database/models/`
- All repo queries in `pkg/internal/storage/repo/`
- Generated query layer in `pkg/internal/storage/database/genquery/` (regen)
- Entity layer in `pkg/internal/storage/entity/`
- Action handlers in `pkg/storage/internal/actions/`
- Sync handlers in `pkg/internal/storage/repo/syncrepo/`
- Provider methods in `pkg/storage/provider.go`
- RPC server stubs in `pkg/storage/rpcserver/`
- Conformance fixtures in `conformance/`

### Performance

At target load of 10 tps, no significant impact expected. Estimated 5-15% added latency on hot read paths (list calls) from joins; write paths neutral. UTXO selection moves to `outputs WHERE spendable = true` after `user_utxo` removal — acceptable at 10 tps. Phase 14 re-bench captures regressions; if `outputs.spendable` index proves inadequate at higher tps in production, address via partial/covering index rather than reintroducing the denormalized table.

### Breaking changes

- All storage state created with pre-Option-A schema is incompatible. No migration path is provided since no production users exist.
- All storage RPC clients must re-pin to new DTO field names.
- Any external tooling reading the SQLite/Postgres directly must be updated.

## Decisions confirmed by author

1. **Drop `users.activeStorage`** — strict conformance. Multi-storage routing reintroduced via spec only if TS adds it upstream.
2. **Keep `sync_states.when` and `sync_states.satoshis`** — **REVISED 2026-06-08** (was "drop"): both columns exist in TS at the pinned commit `7a840ff97e1f685f778210818933e6da0dac22c2`, so conformance requires retaining them. The original drop rested on a wrong premise.
3. **Drop `user_utxo` unconditionally** — no benchmark gate. UTXO selection moves to `outputs.spendable` index. Perf at 10 tps target accepted; higher-tps issues addressed via indexing later.
4. **ts-stack pin: latest main HEAD** at Phase 0 start. Refresh policy in `ts-pin.md`.
5. **Perf validation deferred to a follow-up** — **REVISED 2026-06-08** (was "block on baseline benchmark"): no benchmark harness exists in-repo. Gate the sitting on build + unit tests + conformance + lint, apply the design's indexes, and file a perf-bench follow-up issue. See `tasks.md` Wave 4.
6. **Drop `tx_notes`** — fold into `proven_tx_reqs.history` JSON column matching TS `ProvenTxReqHistory` shape exactly. Also add `proven_tx_reqs.notify` matching `ProvenTxReqNotify`.
7. **Drop `numeric_id_lookup` after Phase 6** — sync layer rewired to use native auto-increment surrogate IDs from Phase 3 (baskets/tags/labels) and Phase 6 (`provenTxReqId`).

## Open questions

None outstanding — all originally-open items are now locked decisions.

## Alternatives considered

**Option B — Keep Go schema, port methods logically.** Cheaper short-term, preserves Go's read-path simplicity, but locks in divergence and per-method translation tax forever. Already analysed in this branch's prior discussion; rejected on grounds that "no prod users yet" is a one-time window.

**Option C — Migrate TS to Go schema.** Discussed and rejected: TS has production deployments; migration cost is weeks of dual-write + cutover plus FK-widening risk; net schema win is small once you observe TS already dedups by unique `txid`.

**Option D — Hybrid schema match logically but keep one materialized read view.** Defer until benchmarks prove join cost is real.
