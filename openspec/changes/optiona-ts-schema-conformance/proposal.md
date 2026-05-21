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
| `numeric_id_lookup` | Drop — replaced by native auto-increment surrogate IDs |
| `key_value` | Keep — generic KV used by non-spec subsystems |
| `tx_notes` | Keep — Go-specific debug/audit, no TS conflict |
| `user_utxo` | **Drop** — fold back into `outputs` via `spendable` flag |

### Column convention

- snake_case → camelCase across every gorm column tag.
- Move from gorm soft-delete (`DeletedAt` timestamp) to TS-style `isDeleted` boolean where TS uses one.

### ID strategy

- Composite natural keys → surrogate auto-incrementing integer PKs on:
  - `output_baskets` → add `basketId`
  - `tags`/`output_tags` (after rename) → add `outputTagId`
  - `labels`/`tx_labels` (after rename) → add `txLabelId`
  - `users` → switch from manual `NumericIDLookup` assignment to native auto-increment
- All join tables key on surrogate FKs, not name+user composite.

### Column additions

| Table | Columns | Notes |
|-------|---------|-------|
| `users` | drop `activeStorage` (no TS equivalent) | breaking |
| `outputs` | add `sequenceNumber`, `spendingDescription`, `scriptLength`, `scriptOffset`, `txid` | |
| `outputs` | replace `basketName` string with `basketId` int FK | |
| `transactions` | add `provenTxId` int FK (nullable until proven), `rawTx` blob | |
| `settings` | add `dbtype` | |
| `sync_states` | add `init` boolean | |
| `sync_states` | drop `when`, `satoshis` (Go extras) | breaking |
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

At target load of 10 tps, no significant impact expected. Estimated 5-15% added latency on hot read paths (list calls) from joins; write paths neutral. The one watch item is removal of `user_utxo` — UTXO selection moves to scanning `outputs` filtered by `spendable=true`. Acceptable at 10 tps; benchmark before declaring scalable to 100+ tps.

### Breaking changes

- All storage state created with pre-Option-A schema is incompatible. No migration path is provided since no production users exist.
- All storage RPC clients must re-pin to new DTO field names.
- Any external tooling reading the SQLite/Postgres directly must be updated.

## Open questions

1. Does TS plan to gain `activeStorage` on `users`? If yes, keep it in Go and lobby TS to add.
2. Does TS plan to gain `when`/`satoshis` on `sync_states`? Same call.
3. Should `tx_notes` be promoted to spec or stay Go extension? Tokenovate may want similar debug surface in TS.
4. Bench `outputs.spendable` index at simulated 100 tps × 6 months before committing to drop `user_utxo` — fallback is keep `user_utxo` as Go-extension.
5. `numeric_id_lookup` removal requires confirming all consumers can switch to direct auto-increment IDs.

## Alternatives considered

**Option B — Keep Go schema, port methods logically.** Cheaper short-term, preserves Go's read-path simplicity, but locks in divergence and per-method translation tax forever. Already analysed in this branch's prior discussion; rejected on grounds that "no prod users yet" is a one-time window.

**Option C — Migrate TS to Go schema.** Discussed and rejected: TS has production deployments; migration cost is weeks of dual-write + cutover plus FK-widening risk; net schema win is small once you observe TS already dedups by unique `txid`.

**Option D — Hybrid schema match logically but keep one materialized read view.** Defer until benchmarks prove join cost is real.
