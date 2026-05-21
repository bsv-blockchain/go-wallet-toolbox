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

All wallet-scoped tables with composite natural keys SHALL adopt auto-incrementing integer surrogate primary keys, matching TS conventions:

- `output_baskets`: `basketId`
- `output_tags`: `outputTagId`
- `tx_labels`: `txLabelId`
- `users`: `userId` (no longer manually allocated via `NumericIDLookup`)

#### Scenario: Insert tag, look up by name+user

- **GIVEN** a new tag name + userId pair
- **WHEN** a caller upserts the tag
- **THEN** an auto-incr `outputTagId` SHALL be assigned
- **AND** subsequent inserts into `output_tags_map` SHALL reference the surrogate id

### Requirement: Soft delete via `isDeleted` flag

Tables that support soft delete SHALL use a boolean `isDeleted` column instead of gorm's `DeletedAt` timestamp:

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

All gorm column tags SHALL use camelCase to match TS column names exactly. Snake_case column tags SHALL NOT be used.

#### Scenario: Schema diff column comparison

- **WHEN** Go's gorm migration is applied
- **THEN** every column name in every shared table SHALL match the equivalent TS column name byte-for-byte

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

### Requirement: `sync_states` table shape

The `sync_states` table SHALL include an `init` boolean column. The Go-only `when` and `satoshis` columns SHALL be removed.

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

### Requirement: `users.activeStorage` column

**Reason:** No TS equivalent. If multi-storage routing is needed in TS, propose addition there first.

### Requirement: `sync_states.when` and `sync_states.satoshis` columns

**Reason:** No TS equivalent. Sync semantics in TS do not require these fields.
