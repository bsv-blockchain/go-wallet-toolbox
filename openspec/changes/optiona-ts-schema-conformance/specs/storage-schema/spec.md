# Spec delta — `storage-schema` capability

Adds and modifies requirements under the `storage-schema` capability to encode TS conformance.

## ADDED Requirements

### Requirement: Schema parity with `ts-stack/wallet-toolbox`

Go's storage schema SHALL match the schema defined in `ts-stack/wallet-toolbox/src/storage/schema/KnexMigrations.ts` at the pinned commit, with the following narrow exceptions documented as Go extensions:

- `chaintracks_live_header` (Go-only)
- `chaintracks_bulk_file` (Go-only)
- `key_value` (Go-only)
- `tx_notes` (Go-only)

#### Scenario: Cross-impl storage dump round-trip

- **GIVEN** a wallet storage instance backed by Go
- **WHEN** its state is exported via `pg_dump` / equivalent and imported into a TS-backed wallet storage instance
- **THEN** the TS instance SHALL successfully load the dump without translation
- **AND** the TS instance SHALL pass all conformance vectors against the loaded state

#### Scenario: TS-pinned schema diff

- **GIVEN** the pinned ts-stack commit in `ts-pin.md`
- **WHEN** a schema diff is computed between Go's AutoMigrate output and TS's KnexMigrations
- **THEN** the diff SHALL contain only documented Go extensions

### Requirement: `proven_tx_reqs.history` JSON column

The `proven_tx_reqs.history` column SHALL be a JSON-serialized text column conforming exactly to the TS `ProvenTxReqHistory` shape:

```
ProvenTxReqHistory {
  notes?: ReqHistoryNote[]
}
ReqHistoryNote {
  when?: string   // ISO timestamp
  what: string    // required event tag
  ...extras: boolean | string | number | undefined
}
```

The `proven_tx_reqs.notify` column SHALL be a JSON-serialized text column conforming exactly to the TS `ProvenTxReqNotify` shape:

```
ProvenTxReqNotify {
  transactionIds?: number[]
}
```

Both columns SHALL default to `"{}"` and be `NOT NULL`.

#### Scenario: Append history note

- **GIVEN** a `proven_tx_reqs` row
- **WHEN** the application appends a note `{ what: "broadcast", when: "2026-05-21T10:00:00Z" }`
- **THEN** the `history` JSON SHALL parse as `{ notes: [{ what: "broadcast", when: "2026-05-21T10:00:00Z" }] }`

#### Scenario: Cross-impl history round-trip

- **GIVEN** a `proven_tx_reqs.history` blob written by TS
- **WHEN** Go reads and parses it
- **THEN** the parsed `ProvenTxReqHistory` SHALL be structurally equal to the TS value
- **AND** re-serializing in Go SHALL produce a byte-identical blob (modulo key ordering normalization per JSON spec)

#### Scenario: Note dedup by `what`

- **GIVEN** a history with note `{ what: "callback", when: "T1" }`
- **WHEN** a duplicate note `{ what: "callback", when: "T2" }` is added with `noDupes = true`
- **THEN** only the later note SHALL remain
- **AND** the order SHALL match TS `addHistoryNote(noDupes=true)` semantics

### Requirement: `proven_txs` and `proven_tx_reqs` split

The merged `known_tx` table SHALL be split into:

- `proven_txs` — stores immutable proof facts (rawTx, merklePath, blockHash, blockHeight, merkleRoot, txid). Keyed by auto-incr `provenTxId`. `txid` SHALL be unique.
- `proven_tx_reqs` — stores mutable pursuit state (status, attempts, rebroadcastAttempts, notified, batch, history, notify). Keyed by auto-incr `provenTxReqId`. `txid` SHALL be unique. References `proven_txs.provenTxId` via nullable FK.

#### Scenario: New tx broadcast pursuit

- **WHEN** a transaction is queued for broadcast
- **THEN** a row SHALL be inserted into `proven_tx_reqs` with status `unsent` and `provenTxId = NULL`

#### Scenario: Proof received

- **WHEN** a merkle proof is received for a txid
- **THEN** a row SHALL be inserted into `proven_txs` (or reused if present)
- **AND** the corresponding `proven_tx_reqs.provenTxId` SHALL be updated to the new `provenTxId`
- **AND** the corresponding `proven_tx_reqs.status` SHALL be updated to `unmined` / `completed` per BRC-40 lifecycle

### Requirement: Surrogate auto-incrementing primary keys

Every shared table SHALL use a per-table camelCase auto-incrementing integer surrogate primary key matching TS, replacing gorm's generic `id`:

- `proven_txs`: `provenTxId`
- `proven_tx_reqs`: `provenTxReqId`
- `users`: `userId` (no longer manually allocated via `NumericIDLookup`)
- `certificates`: `certificateId`
- `output_baskets`: `basketId`
- `transactions`: `transactionId`
- `commissions`: `commissionId`
- `outputs`: `outputId`
- `output_tags`: `outputTagId`
- `tx_labels`: `txLabelId`
- `sync_states`: `syncStateId`

`monitor_events` SHALL use `id`. `settings`, `certificate_fields`, `output_tags_map`, and `tx_labels_map` SHALL have no surrogate PK (matching TS). Tables that previously used composite natural keys SHALL retain those as UNIQUE indexes.

#### Scenario: Insert tag, look up by name+user

- **GIVEN** a new tag name + userId pair
- **WHEN** a caller upserts the tag
- **THEN** an auto-incr `outputTagId` SHALL be assigned
- **AND** subsequent inserts into `output_tags_map` SHALL reference the surrogate id

### Requirement: Soft delete via `isDeleted` flag

Tables that support soft delete SHALL use a boolean `isDeleted` column instead of gorm's `DeletedAt` timestamp. No table SHALL carry a `deleted_at` column (i.e. `gorm.Model` SHALL NOT be embedded). The soft-deletable tables are:

- `certificates`
- `output_baskets`
- `output_tags`
- `tx_labels`
- `output_tags_map`
- `tx_labels_map`

Default value SHALL be `false`. All queries against these tables SHALL filter `WHERE is_deleted = false` unless explicitly inclusive of deleted rows.

#### Scenario: Soft-deleted basket excluded from listBaskets

- **GIVEN** a basket with `isDeleted = true`
- **WHEN** `listOutputBaskets` is called for the owning user
- **THEN** the basket SHALL NOT appear in results

### Requirement: Column naming convention

All gorm column tags SHALL use camelCase to match TS column names exactly, **with the sole exception of `created_at` and `updated_at`**, which SHALL remain snake_case to match the TS `addTimeStamps` helper. The generic `id` and `deleted_at` columns injected by `gorm.Model` SHALL NOT appear on any table.

#### Scenario: Schema diff column comparison

- **WHEN** Go's gorm migration is applied
- **THEN** every column name in every shared table SHALL match the equivalent TS column name byte-for-byte
- **AND** `created_at`/`updated_at` SHALL be snake_case while all other columns are camelCase
- **AND** no table SHALL contain a `deleted_at` column

### Requirement: `monitor_events` table

A `monitor_events` table SHALL exist with columns `id` (auto-incr PK), `event` (varchar), `details` (text). The monitor task lifecycle SHALL emit events to this table aligned with TS monitor semantics.

#### Scenario: Monitor task emits event

- **WHEN** the monitor task transitions to a tracked lifecycle step
- **THEN** a row SHALL be appended to `monitor_events` with the appropriate `event` string and `details` payload

## MODIFIED Requirements

### Requirement: `users` table shape

The `users` table SHALL contain columns `userId` (auto-incr PK), `identityKey` (varchar 130, unique), and standard timestamps. The Go-only `activeStorage` column SHALL be removed.

#### Scenario: User identity uniqueness

- **WHEN** a user is registered with an existing identity key
- **THEN** the insert SHALL fail on the unique constraint on `identityKey`

### Requirement: `transactions` table shape

The `transactions` table SHALL contain a `provenTxId` nullable integer FK to `proven_txs.provenTxId` and a `rawTx` binary column, mirroring TS.

#### Scenario: Proven transaction linkage

- **GIVEN** a proven tx exists in `proven_txs`
- **WHEN** the corresponding `transactions` row is read
- **THEN** `transactions.provenTxId` SHALL reference the `proven_txs.provenTxId`

### Requirement: `outputs` table extensions

The `outputs` table SHALL include `sequenceNumber`, `spendingDescription`, `scriptLength`, `scriptOffset`, and `txid` columns. The `basketName` string column SHALL be replaced by a `basketId` integer FK to `output_baskets.basketId`.

#### Scenario: Output basket lookup

- **GIVEN** an output row
- **WHEN** the output's basket is needed
- **THEN** the join SHALL be `outputs.basketId = output_baskets.basketId`, not by name

### Requirement: `settings` table extensions

The `settings` table SHALL include a `dbtype` column.

#### Scenario: Settings reports the active backend

- **WHEN** the `settings` row is read
- **THEN** the `dbtype` column SHALL report the active database backend (e.g. `sqlite` / `postgres`)

### Requirement: `sync_states` table shape

The `sync_states` table SHALL include an `init` boolean column. The `when` (dateTime) and `satoshis` (bigint) columns SHALL be retained, matching the TS schema at the pinned commit.

#### Scenario: Sync state matches TS columns

- **WHEN** a `sync_states` row is created under the TS-conformant schema
- **THEN** it SHALL expose an `init` boolean defaulting to `false`
- **AND** it SHALL retain the `when` and `satoshis` columns present in TS

## REMOVED Requirements

### Requirement: `known_tx` merged table

**Reason:** Replaced by the `proven_txs` + `proven_tx_reqs` split (see ADDED Requirement above).

**Migration:** No data migration; pre-Option-A storage state is incompatible.

### Requirement: `numeric_id_lookup` allocator

**Reason:** Replaced by native auto-incrementing primary keys per TS convention.

**Migration:** No data migration; all IDs reissued under new schema.

### Requirement: `user_utxo` denormalized table

**Reason:** TS does not maintain a separate UTXO table; UTXO selection runs against `outputs WHERE spendable = true`. Dropped unconditionally for schema conformance.

**Migration:** UTXO selection rewritten against `outputs.spendable` with a partial/covering index on `(userId, spendable, basketId)`.

### Requirement: `tx_notes` table

**Reason:** TS has no separate notes table; equivalent semantics live in `proven_tx_reqs.history` JSON column. Go's `tx_notes` table is removed and its data model collapsed into the TS-conformant `ProvenTxReqHistory` shape.

**Migration:** Every `tx_notes` write site is rewritten as `proven_tx_req.AddHistoryNote(...)`. Existing rows are not migrated (no production data).

### Requirement: `users.activeStorage` column

**Reason:** No TS equivalent. If multi-storage routing is needed in TS, propose addition there first.

### Requirement: `sync_states.when` and `sync_states.satoshis` columns

**RESCINDED 2026-06-08.** These columns ARE present in TS at the pinned commit `7a840ff97e1f685f778210818933e6da0dac22c2` and are retained (see the MODIFIED `sync_states` table shape requirement). The original removal rested on a wrong premise.
