# Schema diff — Go current vs target (Wave 1 work list)

Generated 2026-07-08 from Go models in `pkg/internal/storage/database/models/` vs finalized `target-schema.md`.

**Legend:** 🔴 = must change, 🟡 = minor/rename, 🟢 = already correct, 🗑️ = delete, 🆕 = create new

---

## Structural changes (affect ALL or MANY tables)

| Issue | Affected tables | Work |
|-------|----------------|------|
| 🔴 `gorm.Model` embeds `deleted_at` + generic `id` | `certificates`, `commissions`, `outputs`, `sync_states`, `transactions` | Replace with named PK + `Timestamps` embed |
| 🔴 Column names default to snake_case | Nearly all tables | Add explicit `column:camelCase` gorm tags for every non-timestamp column |
| 🔴 `DeletedAt gorm.DeletedAt` → `IsDeleted bool` | 6 tables: `certificates`, `output_baskets`, `output_tags`(map), `tx_labels`(map), `output_tags`(lookup), `tx_labels`(lookup) | Replace soft-delete mechanism |
| 🔴 Wrong soft-delete on tables that shouldn't have it | `commissions`, `transactions`, `outputs`, `sync_states` | Remove soft delete entirely (via dropping `gorm.Model`) |

---

## Per-table deltas

### `proven_txs` 🆕 — agent M1
**Go current:** Does not exist. Proof fields are embedded in `KnownTx`.
**Target:** New table with `provenTxId` auto-incr PK, columns: `txid`, `height`, `index`, `merklePath`, `rawTx`, `blockHash`, `merkleRoot`.
**Work:** Create `proven_tx.go` with `ProvenTx` struct.

---

### `proven_tx_reqs` 🆕 (refactor from `known_tx`) — agent M1
**Go current:** `KnownTx` struct → table `known_txes`. PK = `TxID` (string, natural key). Merges proof + pursuit state. Has `rebroadcast_attempts` (snake_case), `WasBroadcast`. Missing `history`, `notify` text columns.
**Target:** `provenTxReqId` auto-incr PK. Pursuit-only columns. `history`/`notify` text NOT NULL default `'{}'`. `wasBroadcast` bool, `rebroadcastAttempts` int.

| Delta | Current | Target |
|-------|---------|--------|
| 🔴 PK | `TxID` string (natural) | `provenTxReqId` auto-incr |
| 🔴 Table name | `known_txes` | `proven_tx_reqs` |
| 🔴 Split | Contains proof fields (`BlockHeight`, `MerklePath`, `MerkleRoot`, `BlockHash`) | Proof fields moved to `proven_txs` |
| 🔴 FK to proven_txs | N/A | `provenTxId` int unsigned NULL → FK |
| 🔴 Missing cols | — | `history` text default `'{}'`, `notify` text default `'{}'` |
| 🟡 Rename col | `rebroadcast_attempts` | `rebroadcastAttempts` (camelCase) |
| 🟡 All cols | snake_case | camelCase gorm tags |

---

### `users` — agent M1
**Go current:** `User` struct. PK = `UserID int` (manual, not auto-incr). Has `ActiveStorage varchar(255)`.

| Delta | Current | Target |
|-------|---------|--------|
| 🔴 PK | `UserID int` (manual) | `userId` auto-incr |
| 🟡 ActiveStorage size | `varchar(255)` | `varchar(130)` |
| 🟡 Column tags | Missing camelCase tags | Need `column:userId`, `column:identityKey`, `column:activeStorage` |

---

### `certificates` — agent M1
**Go current:** Embeds `gorm.Model` → generic `id`, has `deleted_at`.

| Delta | Current | Target |
|-------|---------|--------|
| 🔴 PK | generic `ID uint` (gorm.Model) | `certificateId` auto-incr |
| 🔴 Soft delete | `DeletedAt gorm.DeletedAt` | `isDeleted` bool default false |
| 🔴 Missing `column:` tags | `UserID`, `SerialNumber`, etc. use Go defaults | Need camelCase gorm column tags |

---

### `certificate_fields` — agent M1
**Go current:** No explicit PK declared (GORM implicit ID). Has `FieldValue varchar(100)`.

| Delta | Current | Target |
|-------|---------|--------|
| 🔴 PK | Implicit GORM `id` | No surrogate PK (match TS) |
| 🟡 FieldValue size | `varchar(100)` | `varchar` (unbounded/default 255) |
| 🟡 MasterKey default | none | `default:''` |
| 🟡 Column tags | Missing camelCase | Need `column:userId`, `column:certificateId`, `column:fieldName`, `column:fieldValue`, `column:masterKey` |

---

### `output_baskets` — agent M1
**Go current:** Composite PK (`Name`, `UserID`). `DeletedAt gorm.DeletedAt`. Defaults: `numberOfDesiredUTXOs=32`, `minimumDesiredUTXOValue=1000`.

| Delta | Current | Target |
|-------|---------|--------|
| 🔴 PK | Composite (`Name`, `UserID`) | `basketId` auto-incr |
| 🔴 Soft delete | `DeletedAt gorm.DeletedAt` | `isDeleted` bool default false |
| 🔴 Defaults | `numberOfDesiredUTXOs=32`, `minimumDesiredUTXOValue=1000` | `numberOfDesiredUTXOs=6`, `minimumDesiredUTXOValue=10000` |
| 🟡 Column tags | Missing camelCase | Need `column:basketId`, `column:userId`, `column:name`, `column:numberOfDesiredUTXOs`, `column:minimumDesiredUTXOValue`, `column:isDeleted` |
| 🟡 Unique | Implicit from composite PK | Explicit `UNIQUE(name, userId)` |

---

### `transactions` — agent M1
**Go current:** Embeds `gorm.Model`. Missing `provenTxId` FK, `rawTx`. `Description` is `type:string` (unbounded). `Version`/`LockTime` are `uint32` (not nullable).

| Delta | Current | Target |
|-------|---------|--------|
| 🔴 PK | generic `ID uint` | `transactionId` auto-incr |
| 🔴 Soft delete | `DeletedAt` (gorm.Model) | None |
| 🔴 Missing cols | — | `provenTxId` int unsigned NULL FK, `rawTx` binary NULL |
| 🟡 Description size | unbounded `type:string` | `varchar(2048)` |
| 🟡 Version/LockTime | `uint32` (not nullable) | `int unsigned NULL` |
| 🟡 Column tags | Missing camelCase | All columns need `column:` tags |

---

### `commissions` — agent M1
**Go current:** Embeds `gorm.Model`. `Satoshis` is `uint64`. Missing explicit column tags.

| Delta | Current | Target |
|-------|---------|--------|
| 🔴 PK | generic `ID uint` | `commissionId` auto-incr |
| 🔴 Soft delete | `DeletedAt` (gorm.Model) | None |
| 🟡 Satoshis type | `uint64` | `int` |
| 🟡 KeyOffset | `type:string` | `varchar(130)` |
| 🟡 TransactionID constraint | `uniqueIndex:idx_commission_user_tx` | `UNIQUE` (standalone) + index |
| 🟡 Column tags | Missing camelCase | Need `column:commissionId`, `column:userId`, `column:transactionId`, `column:satoshis`, `column:keyOffset`, `column:isRedeemed`, `column:lockingScript` |

---

### `outputs` — agent M1
**Go current:** Embeds `gorm.Model`. Uses `BasketName` string FK. Missing several TS columns. `Description` maps to `outputDescription`.

| Delta | Current | Target |
|-------|---------|--------|
| 🔴 PK | generic `ID uint` | `outputId` auto-incr |
| 🔴 Soft delete | `DeletedAt` (gorm.Model) | None |
| 🔴 Basket FK | `BasketName *string` (name-based) | `basketId` int unsigned NULL FK |
| 🔴 Missing cols | — | `txid`, `sequenceNumber`, `spendingDescription`, `scriptLength`, `scriptOffset` |
| 🟡 Description | `Description string` | `outputDescription varchar(2048)` |
| 🟡 DerivationPrefix/Suffix | unbounded `*string` | `varchar(200)` |
| 🟡 Column tags | Missing camelCase | All columns need `column:` tags |
| 🟡 Unique | None explicit | `UNIQUE(transactionId, vout, userId)` |

---

### `output_tags` (lookup table) → rename to `output_tags` — agent M2
**Go current:** `Tag` struct → table `tags`. Composite PK (`Name`, `UserID`). `DeletedAt gorm.DeletedAt`. Value column is `Name`.

| Delta | Current | Target |
|-------|---------|--------|
| 🔴 Struct rename | `Tag` | `OutputTag` |
| 🔴 Table rename | `tags` | `output_tags` |
| 🔴 PK | Composite (`Name`, `UserID`) | `outputTagId` auto-incr |
| 🔴 Value col rename | `Name` | `tag` (varchar 150) |
| 🔴 Soft delete | `DeletedAt gorm.DeletedAt` | `isDeleted` bool default false |

---

### `output_tags_map` (M2M join) → rename from `output_tags` — agent M2
**Go current:** `OutputTag` struct → table `output_tags`. Composite PK (`OutputID`, `TagName`, `TagUserID`). `DeletedAt gorm.DeletedAt`.

| Delta | Current | Target |
|-------|---------|--------|
| 🔴 Struct rename | `OutputTag` | `OutputTagsMap` |
| 🔴 Table stays | `output_tags` → `output_tags_map` |
| 🔴 PK | Composite (`OutputID`, `TagName`, `TagUserID`) | No surrogate PK |
| 🔴 FK cols | `TagName`, `TagUserID` | `outputTagId` int FK, `outputId` int FK |
| 🔴 Soft delete | `DeletedAt gorm.DeletedAt` | `isDeleted` bool default false |

---

### `tx_labels` (lookup table) → rename from `labels` — agent M2
**Go current:** `Label` struct → table `labels`. Composite PK (`Name`, `UserID`). `DeletedAt gorm.DeletedAt`. Value column is `Name`.

| Delta | Current | Target |
|-------|---------|--------|
| 🔴 Struct rename | `Label` | `TxLabel` |
| 🔴 Table rename | `labels` | `tx_labels` |
| 🔴 PK | Composite (`Name`, `UserID`) | `txLabelId` auto-incr |
| 🔴 Value col rename | `Name` | `label` (varchar 300) |
| 🔴 Soft delete | `DeletedAt gorm.DeletedAt` | `isDeleted` bool default false |

---

### `tx_labels_map` (M2M join) → rename from `transaction_labels` — agent M2
**Go current:** `TransactionLabel` struct → table `transaction_labels`. Composite PK (`TransactionID`, `LabelName`, `LabelUserID`). `DeletedAt gorm.DeletedAt`.

| Delta | Current | Target |
|-------|---------|--------|
| 🔴 Struct rename | `TransactionLabel` | `TxLabelsMap` |
| 🔴 Table rename | `transaction_labels` | `tx_labels_map` |
| 🔴 PK | Composite (`TransactionID`, `LabelName`, `LabelUserID`) | No surrogate PK |
| 🔴 FK cols | `LabelName`, `LabelUserID` | `txLabelId` int FK, `transactionId` int FK |
| 🔴 Soft delete | `DeletedAt gorm.DeletedAt` | `isDeleted` bool default false |

---

### `monitor_events` 🆕 — agent M2
**Go current:** Does not exist.
**Target:** `id` auto-incr PK (the one table using generic `id`), `event` varchar(64) NOT NULL, `details` text NULL.
**Work:** Create `monitor_events.go` with `MonitorEvent` struct.

---

### `settings` — agent M1
**Go current:** `Setting` struct. PK = `StorageIdentityKey`. Missing `dbtype`.

| Delta | Current | Target |
|-------|---------|--------|
| 🔴 Missing col | — | `dbtype` varchar(10) NOT NULL |
| 🟡 Column tags | Missing camelCase | Need `column:storageIdentityKey`, `column:storageName`, `column:chain`, `column:dbtype`, `column:maxOutputScript` |

---

### `sync_states` — agent M1
**Go current:** Embeds `gorm.Model`. Missing `init` col. Has `When`/`Satoshis` (✅ keep).

| Delta | Current | Target |
|-------|---------|--------|
| 🔴 PK | generic `ID uint` | `syncStateId` auto-incr |
| 🔴 Soft delete | `DeletedAt` (gorm.Model) | None |
| 🔴 Missing col | — | `init` bool NOT NULL default false |
| 🟡 Missing cols | — | `errorLocal` text NULL, `errorOther` text NULL |
| 🟡 Column tags | Missing camelCase | All columns need `column:` tags |
| 🟢 When/Satoshis | Present | Present (keep) |

---

## Tables to DELETE — agent M2

| Go file | Struct | Table | Reason |
|---------|--------|-------|--------|
| 🗑️ `user_utxo.go` | `UserUTXO` | `user_utxos` | UTXO selection → `outputs WHERE spendable = true` |
| 🗑️ `tx_note.go` | `TxNote` | `tx_notes` | Folded into `proven_tx_reqs.history` JSON |
| 🗑️ `numeric_id_lookup.go` | `NumericIDLookup` | `numeric_id_lookups` | Replaced by native surrogate PKs |

---

## Go extension tables (no changes needed)

| Table | Status |
|-------|--------|
| 🟢 `chaintracks_bulk_files` | Already camelCase columns, custom PK — no work |
| 🟢 `chaintracks_live_headers` | Already camelCase columns, custom PK — no work |
| 🟢 `key_values` | Custom PK, no gorm.Model — no work |

---

## New file: `models/base.go` — agent M-base

```go
// Timestamps matches TS addTimeStamps(): snake_case created_at / updated_at, no deleted_at.
type Timestamps struct {
    CreatedAt time.Time `gorm:"column:created_at"`
    UpdatedAt time.Time `gorm:"column:updated_at"`
}
```

Every model replaces `gorm.Model` with: explicit named PK + embedded `Timestamps`.
