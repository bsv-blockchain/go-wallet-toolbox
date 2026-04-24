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
    defer provider.Stop()

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
