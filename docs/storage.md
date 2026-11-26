## STORAGE: BSV Wallet Toolbox

This page covers persistent storage of wallet data: transactions, outputs, proofs, and related metadata. It explains the components, data model, typical lifecycle, and provides minimal code examples.

### Concepts and components

- **Provider (`storage.Provider`)**: Database-backed storage that implements wallet operations and exposes helper methods used by the monitor. Handles schema, CRUD, fees/commission, background broadcasting integration, and proof merging.
- **WalletStorageManager**: Orchestrates one active storage and optional backups, provides authenticated access, and supports replication from active to backups.
- **Remote client/server**: A JSON-RPC client and server enable remote storage. See `StorageClient` in code and the separate `storage_server.md` for hosting details.

The storage layer is the authoritative system of record for a wallet. It persists users, transactions, outputs, proofs, and metadata; implements wallet workflows (create/process/internalize/abort); coordinates with services and the monitor to converge transaction status to the on-chain truth; intelligently broadcasts/marks abandoned; and exposes chunked synchronization for backups or remote providers.

### What it is

- Authoritative store for a wallet’s state: transactions, outputs, proofs, labels/tags, and settings.
- Operational layer that implements wallet workflows (create/process/internalize/abort) against a durable database.
- Integration hub that coordinates with services (mempool, headers, proofs) and the monitor to converge on-chain truth.

### What it does

- Persists wallet data: durable CRUD for users, transactions, outputs, baskets, proofs, requests, commissions, metadata.
- Manages lifecycle: transitions transaction statuses, allocates/relinquishes outputs, and updates derived state.
- Merges proofs: ingests Merkle proofs and updates related transactions and requests atomically.
- Broadcasts intelligently: queues and retries sending, integrates with background broadcaster, and marks abandoned.
- Replicates state: exposes chunked sync for backup/remote stores; the manager coordinates push/pull.
- Provides queries: efficient listing/filtering of actions, outputs, labels/tags for application UX.


### Data model overview

- **Transactions**: Status lifecycle (unsigned → unprocessed/sending → unproven/completed/failed/...); optional `rawTx`, `provenTxId`.
- **Outputs and baskets**: Spendable/change outputs and user-defined baskets (including the default change basket).
- **ProvenTx / ProvenTxReq**: Mined proofs and outstanding proof requests with history/attempts.
- **Certificates, labels, tags**: User metadata for governance and organization.
- **Settings and SyncState**: Storage configuration and replication progress tracking.

### Quick start

```go
package main

import (
    "context"
    "log/slog"
    "os"

    chainsdk "github.com/bsv-blockchain/go-sdk/transaction/chaintracker"
    "github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
    "github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
)

func main() {
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
    services := chainsdk.New()

    provider, err := storage.NewGORMProvider(
        defs.BSVNetworkMainnet,
        services,
        storage.WithLogger(logger),
        storage.WithDBConfig(defs.DefaultDBConfig()),
    )
    if err != nil { panic(err) }
    defer provider.Stop(context.Background())

    ctx := context.Background()
    _, err = provider.Migrate(ctx, "My Storage", "my-storage-identity-key")
    if err != nil { panic(err) }

    _, err = provider.MakeAvailable(ctx)
    if err != nil { panic(err) }
}
```

### Lifecycle and flows

#### Initialize storage
- **Migrate**: create/update schema and persist initial settings.
- **MakeAvailable**: load settings and prepare for use.

```go
version, _ := provider.Migrate(ctx, "My Storage", "my-storage-identity-key")
_ = version
_, _ = provider.MakeAvailable(ctx)
```

#### Users and auth
- Use `FindOrInsertUser` to ensure a user exists; `WalletStorageManager.GetAuth` provides an `AuthID` for user-scoped calls.

```go
resp, _ := provider.FindOrInsertUser(ctx, "user-identity-key")
mgr := storage.NewWalletStorageManager("user-identity-key", provider)
auth, _ := mgr.GetAuth(ctx)
_ = resp; _ = auth
```

#### Actions pipeline (high level)
- **CreateAction** builds a transaction, **ProcessAction** finalizes/sends, **InternalizeAction** imports external outputs, **AbortAction** cancels queued work.

```go
_ = auth
_, _ = provider.CreateAction(ctx, auth, args)
_, _ = provider.ProcessAction(ctx, auth, pArgs)
_, _ = provider.InternalizeAction(ctx, auth, iArgs)
_, _ = provider.AbortAction(ctx, auth, aArgs)
```

#### Listing and organization
- Query outputs and manage baskets; tag/label as needed.

```go
_, _ = provider.ListOutputs(ctx, auth, lArgs)
_ = provider.ConfigureBasket(ctx, auth, basketCfg)
```

#### Monitor helpers
- Storage exposes the same primitives used by the monitor for background health: sync statuses, send queued, mark abandoned, and unfail.

```go
_ = provider.SynchronizeTransactionStatuses(ctx)
_ = provider.SendWaitingTransactions(ctx, 5*time.Minute)
_ = provider.AbortAbandoned(ctx)
_ = provider.UnFail(ctx)
```

#### Replication and backups
- `WalletStorageManager` can push from the active storage to backups; Providers also expose chunked sync helpers.

```go
mgr := storage.NewWalletStorageManager("user-identity-key", provider)
_, _ = mgr.MakeAvailable(ctx)
// inserts, updates, err := mgr.SyncToWriter(ctx, backupProvider)
```

### Database configuration

- Defaults use SQLite. Swap to Postgres/MySQL by setting `defs.Database`.

```go
db := defs.DefaultDBConfig()
db.Engine = defs.DBTypePostgres
db.PostgreSQL.SQLCommon = defs.SQLCommon{
    Host: "localhost", Port: "5432", User: "postgres", Password: "postgres", DBName: "storage", TimeZone: "UTC",
}
db.PostgreSQL.Schema = "my_custom_schema"
provider, _ := storage.NewGORMProvider(
    defs.BSVNetworkMainnet, services, storage.WithDBConfig(db),
)
```

- **MySQL minimum version: 8.0.1.** UTXO selection uses `SELECT ... FOR UPDATE SKIP LOCKED`, which MySQL only supports from 8.0.1 onward; older MySQL rejects it with a syntax error on the first `CreateAction`.

- **Connection pool defaults**: `MaxOpenConnections` and `MaxIdleConnections` default to 5 for every engine (`defs.DefaultDBConfig`, `pkg/defs/dbs.go`). Each `CreateAction` holds one connection for the lifetime of its funding transaction, so under concurrent load raise `MaxOpenConnections` for Postgres/MySQL production deployments.

- **Prepared statements (`prepare_statements`, default `false`)**: sets GORM `PrepareStmt`, so each distinct SQL text is prepared once per connection and reused (the cache is bounded to `defs.DBPreparedStatementCacheSize` entries, because variable-length `IN (...)` lists and variable-row inserts each produce a new SQL text). On SQLite it saves the per-statement parse; it works with `_txlock=immediate`, inside transactions (commit and rollback) and across schema changes (SQLite re-prepares a statement whose schema changed). On Postgres the toolbox runs pgx in simple-protocol mode (`PreferSimpleProtocol`), which never prepares statements, so turning this on switches the connection to server-side prepared statements for the repository queries; it is useful for chatty workloads but do not enable it behind a transaction-pooling proxy such as pgbouncer in transaction mode, which does not support session-level prepared statements. Run migrations before enabling it on a live Postgres (Postgres can reject a cached `SELECT *` plan with "cached plan must not change result type" if a column is added underneath it; restart the process after a migration). Not used when you hand the toolbox your own `*gorm.DB`: set `PrepareStmt` there yourself.

- **Batch inserts (`insert_batch_size`, default `0`)**: sets GORM `CreateBatchSize`. `0` keeps today's behaviour. A value above 0 caps the rows per `INSERT` for every slice or association insert on the connection (transactions with many outputs, change UTXOs, history notes, ...), which also keeps very large fan-out transactions below the engines' bind-parameter limits, and switches the slice-taking repository helpers (`UpsertKnownTxs`, `CreateTransactions`, and the `Upsert*sForSync` sync helpers) to multi-row statements. Negative values are rejected. A few hundred is a good starting point.

- **Group commit (`group_commit`, default off)**: concurrent `createAction` and `processAction` calls share database transactions instead of each opening its own. The write phases of those calls (funding and creating the action, recording the signed transaction, claiming it for broadcast together with the attempts counter, and applying the broadcast result) are queued and run, one unit per caller, as savepoints inside one transaction per batch. A caller returns only after the transaction holding its work committed; a unit that fails (not enough funds, UTXO contention, a validation error, ...) is rolled back to its savepoint and returns the same error it would without grouping, while the rest of its batch commits. Contention retries and their backoff run outside the shared transaction, and no network call (broadcast included) ever runs inside one. A unit the engine aborts to break a deadlock, or a batch that is rolled back as a whole, runs again on its own, ungrouped path. Settings: `enabled` (default `false`), `max_batch` (units per transaction, default 16, which keeps a batch inside Postgres's 64 cached subtransactions), `max_wait` (how long a batch that is not full waits for more work, default `0s`: take whatever queued up while the previous batch ran, so a lone call is never delayed) and `workers` (batches in flight, default 1). Group commit is meant for **SQLite**, where it replaces the per-call write-lock handoff (one writer, `busy_timeout` polling) with one lock and one commit per batch; use it with `_txlock=immediate` and keep `workers` at 1. On **Postgres** leave it off: Postgres already runs concurrent transactions in parallel and groups WAL flushes itself, and funnelling the calls through a few connections makes them slower. `storage.WithGroupCommit` sets it when you pass your own `*gorm.DB`.

- **SQLite defaults**: file-based DSNs get `_journal_mode=WAL` and `_busy_timeout=5000` filled in **independently** — each pragma is appended only when the DSN doesn't already set it, so an operator who pins only `_busy_timeout` still receives the WAL default (and vice versa). Explicit operator-provided `_journal_mode`/`_busy_timeout` params always win, and in-memory DSNs (`:memory:`, `mode=memory`) are left untouched. Two operational notes: WAL mode creates `-wal`/`-shm` sibling files next to the database file, so persist/mount the containing directory rather than the single database file; and `journal_mode` is stored inside the database file itself, so it persists across reopens even without the DSN pragma.

- **Migration note**: upgrading adds a composite `idx_user_utxos_selection` index (covering `user_id, basket_name, reserved_by_id, utxo_status, satoshis`) plus a single-column index on `reserved_by_id` to `bsv_user_utxos`, built automatically by `AutoMigrate`. On large existing tables, expect the index build to take noticeable time during the first migration after upgrade.
