## STORAGE SERVER: BSV Wallet Toolbox

Exposes the wallet storage provider over JSON-RPC with Authrite authentication. Use this to host a remote storage backend and connect via the `StorageClient`.

### Concepts

- Server wraps a `wdk.WalletStorageProvider` and publishes RPC endpoints under the name `remote_storage`.
- Requests are authenticated using a wallet (`sdk.Interface`) via Authrite middleware.
- CORS is permissive by default to enable cross-origin API usage.

### Quick start

```go
package main

import (
    "log/slog"
    "os"

    sdk "github.com/bsv-blockchain/go-sdk/wallet"
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

    var wallet sdk.Interface

    server := storage.NewServer(logger, provider, wallet, storage.ServerOptions{Port: 8080})
    if err := server.Start(); err != nil { panic(err) }
}
```

### Client usage

```go
client, cleanup, err := storage.NewClient("https://localhost:8080", wallet)
defer func() { if cleanup != nil { cleanup() } }()
if err != nil { /* handle */ }
_ = client // implements wdk.WalletStorageProvider
```
