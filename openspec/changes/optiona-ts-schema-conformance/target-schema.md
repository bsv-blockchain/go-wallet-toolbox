# Target schema contract (source of truth)

Every task in this change codes against THIS document, not against prose column-lists elsewhere. It is the migration-resolved final state of `ts-stack/wallet-toolbox` at the pinned commit (`ts-pin.md`).

**Extraction source:** `packages/wallet/wallet-toolbox/src/storage/schema/KnexMigrations.ts` @ `7a840ff97e1f685f778210818933e6da0dac22c2`.

**Extraction method (Wave 0 finalizes this file):** Knex runs migrations in **sorted-key order**, NOT file order. Final state = apply every `createTable` then every `alterTable`/`dropColumn` in key order. Columns added by one migration and dropped by a later one are **absent** in final state. The tables below are extracted from the `createTable` blocks; items flagged **⚠ RESOLVE** are touched by later `alterTable` migrations and Wave 0 must confirm net state before models are written.

## Naming convention (CRITICAL — the prose plan got this wrong)

1. **All columns are camelCase** (`provenTxId`, `identityKey`, `isDeleted`, `numberOfDesiredUTXOs`, `lockTime`, `outputDescription`) ...
2. **EXCEPT `created_at` and `updated_at`**, which are **snake_case** (TS `addTimeStamps` helper emits `created_at` / `updated_at`). A blanket "camelCase every column" pass would emit `createdAt`/`updatedAt` and **break conformance**.
3. **Primary keys are per-table named** (`provenTxId`, `transactionId`, `outputId`, `certificateId`, `commissionId`, `userId`, `basketId`, `outputTagId`, `txLabelId`, `syncStateId`), NOT a generic `id`. Only `monitor_events` uses `id`.
4. **No `deleted_at` anywhere.** TS has zero soft-delete timestamps. Soft delete is a boolean `isDeleted` on the six tables listed below — and nowhere else.

### Go modelling consequence

`gorm.Model` embeds `ID uint` (→ `id`), `CreatedAt`, `UpdatedAt`, **and `DeletedAt` (→ `deleted_at`)**. Using it violates rules 3 and 4 on every table. Replace it with:

```go
// models/base.go
// Timestamps matches TS addTimeStamps(): snake_case created_at / updated_at, no deleted_at.
type Timestamps struct {
    CreatedAt time.Time `gorm:"column:created_at"`
    UpdatedAt time.Time `gorm:"column:updated_at"`
}
```

Each model then declares its own named PK explicitly, e.g. `OutputID uint \`gorm:"column:outputId;primaryKey;autoIncrement"\`` plus an embedded `Timestamps`. Soft-deletable models add `IsDeleted bool \`gorm:"column:isDeleted;not null;default:false"\``.

---

## Shared tables (Go MUST match byte-for-byte)

Format: `column  type  flags`. `ts(3)` = `timestamp(3)`. Every table also has `created_at`, `updated_at` (snake_case, NOT NULL) — omitted per-row below, listed once here.

### proven_txs  ← split from `known_tx`
PK `provenTxId` auto-incr
- `txid` varchar(64) NOT NULL UNIQUE
- `height` int unsigned NOT NULL
- `index` int unsigned NOT NULL
- `merklePath` binary NOT NULL
- `rawTx` binary NOT NULL
- `blockHash` varchar(64) NOT NULL
- `merkleRoot` varchar(64) NOT NULL

### proven_tx_reqs  ← split from `known_tx`
PK `provenTxReqId` auto-incr
- `provenTxId` int unsigned NULL → FK `proven_txs.provenTxId`
- `status` varchar(16) NOT NULL default `'unknown'`  (index)
- `attempts` int unsigned NOT NULL default 0
- `notified` bool NOT NULL default false
- `txid` varchar(64) NOT NULL UNIQUE
- `batch` varchar(64) NULL  (index)
- `history` text/longtext NOT NULL default `'{}'`   ← absorbs Go `tx_notes`
- `notify` text/longtext NOT NULL default `'{}'`
- `rawTx` binary NOT NULL
- `inputBEEF` binary NULL
- ⚠ RESOLVE `wasBroadcast` bool + `rebroadcastAttempts` int — **added** by migration `2026-04-30-001`, **dropped** by a later migration. Net state appears **ABSENT**. Go currently HAS `rebroadcast_attempts` on `KnownTx`; if net-absent, Go must drop it. Confirm sorted-key order in Wave 0.

### users
PK `userId` auto-incr
- `identityKey` varchar(130) NOT NULL UNIQUE
- (NO `activeStorage` — added then dropped in TS; confirmed net-absent. Drop Go's.)

### certificates  (soft-delete)
PK `certificateId` auto-incr
- `userId` int unsigned NOT NULL → FK `users.userId`
- `serialNumber` varchar(100) NOT NULL
- `type` varchar(100) NOT NULL
- `certifier` varchar(100) NOT NULL
- `subject` varchar(100) NOT NULL
- `verifier` varchar(100) NULL
- `revocationOutpoint` varchar(100) NOT NULL
- `signature` varchar(255) NOT NULL
- `isDeleted` bool NOT NULL default false
- UNIQUE (`userId`,`type`,`certifier`,`serialNumber`)

### certificate_fields  (NO surrogate PK)
- `userId` int unsigned NOT NULL → FK `users.userId`
- `certificateId` int unsigned NOT NULL → FK `certificates.certificateId`
- `fieldName` varchar(100) NOT NULL
- `fieldValue` varchar NOT NULL
- `masterKey` varchar(255) NOT NULL default `''`
- UNIQUE (`fieldName`,`certificateId`)

### output_baskets  (soft-delete)
PK `basketId` auto-incr
- `userId` int unsigned NOT NULL → FK `users.userId`
- `name` varchar(300) NOT NULL
- `numberOfDesiredUTXOs` int NOT NULL default **6**   (Go currently 32)
- `minimumDesiredUTXOValue` int NOT NULL default **10000**  (Go currently 1000)
- `isDeleted` bool NOT NULL default false
- UNIQUE (`name`,`userId`)

### transactions
PK `transactionId` auto-incr
- `userId` int unsigned NOT NULL → FK `users.userId`
- `provenTxId` int unsigned NULL → FK `proven_txs.provenTxId`
- `status` varchar(64) NOT NULL  (index)
- `reference` varchar(64) NOT NULL UNIQUE
- `isOutgoing` bool NOT NULL
- `satoshis` bigint NOT NULL default 0
- `version` int unsigned NULL
- `lockTime` int unsigned NULL
- `description` varchar(2048) NOT NULL  (⚠ created at 500, altered to 2048)
- `txid` varchar(64) NULL
- `inputBEEF` binary NULL
- `rawTx` binary NULL

### commissions
PK `commissionId` auto-incr
- `userId` int unsigned NOT NULL → FK `users.userId`
- `transactionId` int unsigned NOT NULL UNIQUE → FK `transactions.transactionId`  (index)
- `satoshis` int NOT NULL
- `keyOffset` varchar(130) NOT NULL
- `isRedeemed` bool NOT NULL default false
- `lockingScript` binary NOT NULL

### outputs
PK `outputId` auto-incr
- `userId` int unsigned NOT NULL → FK `users.userId`
- `transactionId` int unsigned NOT NULL → FK `transactions.transactionId`
- `basketId` int unsigned NULL → FK `output_baskets.basketId`   ← replaces Go `basketName` string
- `spendable` bool NOT NULL default false
- `change` bool NOT NULL default false
- `vout` int NOT NULL
- `satoshis` bigint NOT NULL
- `providedBy` varchar(130) NOT NULL
- `purpose` varchar(20) NOT NULL
- `type` varchar(50) NOT NULL
- `outputDescription` varchar(2048)  (⚠ created 300, altered to 2048)  ← Go `Description`
- `txid` varchar(64) NULL
- `senderIdentityKey` varchar(130) NULL
- `derivationPrefix` varchar(32) NULL  (⚠ 200→32 final)
- `derivationSuffix` varchar(32) NULL
- `customInstructions` varchar(2500) NULL
- `spentBy` int unsigned NULL → FK `transactions.transactionId`
- `sequenceNumber` int unsigned NULL
- `spendingDescription` varchar(2048) NULL
- `scriptLength` bigint unsigned NULL
- `scriptOffset` bigint unsigned NULL
- `lockingScript` binary NULL
- UNIQUE (`transactionId`,`vout`,`userId`)
- Wave 1 index: partial/covering `(userId, spendable, basketId)` for UTXO selection (replaces `user_utxo`)

### output_tags  ← rename of Go `tags`  (soft-delete)
PK `outputTagId` auto-incr
- `userId` int unsigned NOT NULL → FK `users.userId`
- `tag` varchar(150) NOT NULL   ← value column is `tag`, NOT `name`
- `isDeleted` bool NOT NULL default false
- UNIQUE (`tag`,`userId`)

### output_tags_map  ← rename of Go `output_tags` (the M2M join)  (soft-delete, NO surrogate PK)
- `outputTagId` int unsigned NOT NULL → FK `output_tags.outputTagId`
- `outputId` int unsigned NOT NULL → FK `outputs.outputId`
- `isDeleted` bool NOT NULL default false
- UNIQUE (`outputTagId`,`outputId`); index (`outputId`)

### tx_labels  ← rename of Go `labels`  (soft-delete)
PK `txLabelId` auto-incr
- `userId` int unsigned NOT NULL → FK `users.userId`
- `label` varchar(300) NOT NULL   ← value column is `label`, NOT `name`
- `isDeleted` bool NOT NULL default false
- UNIQUE (`label`,`userId`)

### tx_labels_map  ← rename of Go `transaction_labels` (the M2M join)  (soft-delete, NO surrogate PK)
- `txLabelId` int unsigned NOT NULL → FK `tx_labels.txLabelId`
- `transactionId` int unsigned NOT NULL → FK `transactions.transactionId`
- `isDeleted` bool NOT NULL default false
- UNIQUE (`txLabelId`,`transactionId`); index (`transactionId`)

### monitor_events  ← NEW
PK `id` auto-incr  (the one table using generic `id`)
- `event` varchar(64) NOT NULL
- `details` text/longtext NULL

### settings
PK `storageIdentityKey` (no surrogate)
- `storageIdentityKey` varchar(130) NOT NULL
- `storageName` varchar(128) NOT NULL
- `chain` varchar(10) NOT NULL
- `dbtype` varchar(10) NOT NULL   ← Go must add
- `maxOutputScript` int NOT NULL

### sync_states
PK `syncStateId` auto-incr
- `userId` int unsigned NOT NULL → FK `users.userId`
- `storageIdentityKey` varchar(130) NOT NULL default `''`
- `storageName` varchar NOT NULL
- `status` varchar NOT NULL default `'unknown'`  (index)
- `init` bool NOT NULL default false   ← Go must add
- `refNum` varchar(100) NOT NULL UNIQUE  (index)
- `syncMap` text/longtext NOT NULL
- `when` dateTime NULL   ← **KEEP** (present in TS; Decision #2 overturned — do NOT drop)
- `satoshis` bigint NULL  ← **KEEP** (present in TS; Decision #2 overturned — do NOT drop)
- `errorLocal` text/longtext NULL
- `errorOther` text/longtext NULL

---

## Go extension tables (NOT in TS — retain, document as extensions)

- `chaintracks_live_headers` — already camelCase columns; spec-orthogonal. Keep.
- `chaintracks_bulk_files` — already camelCase columns. Keep.
- `key_value` — generic KV. Keep.

## Go tables removed by this change

| Go table | Disposition |
|----------|-------------|
| `known_txes` (`KnownTx`) | split → `proven_txs` + `proven_tx_reqs` |
| `tags` (`Tag`) | rename → `output_tags`, struct `OutputTag`, value col `tag` |
| `labels` (`Label`) | rename → `tx_labels`, struct `TxLabel`, value col `label` |
| `output_tags` (`OutputTag` join) | rename → `output_tags_map`, struct `OutputTagsMap` |
| `transaction_labels` (`TransactionLabel`) | rename → `tx_labels_map`, struct `TxLabelsMap` |
| `user_u_t_x_os` (`UserUTXO`) | **drop** — UTXO selection → `outputs WHERE spendable = true` |
| `tx_notes` (`TxNote`) | **drop** — fold → `proven_tx_reqs.history` JSON |
| `numeric_id_lookups` (`NumericIDLookup`) | **drop** — replaced by native surrogate PKs |

## JSON column shapes (proven_tx_reqs.history / .notify)

```
ProvenTxReqHistory { notes?: ReqHistoryNote[] }
ReqHistoryNote {
  when?: string            // ISO timestamp, optional
  what: string             // required event tag
  [k]: boolean|string|number|undefined   // extras are FLATTENED siblings, NOT nested under "attributes"
}
ProvenTxReqNotify { transactionIds?: number[] }
```

Go's existing `wdk.HistoryNote{ When time.Time; UserID *int; What string; Attributes map[string]any }` must serialize so that `What`→`what`, `When`→`when` (ISO), and **`Attributes` entries flatten to top-level siblings** (plus `userId` if retained). Do not emit an `attributes` sub-object. Verify with a TS-produced fixture (byte-identical modulo JSON key-order normalization).
