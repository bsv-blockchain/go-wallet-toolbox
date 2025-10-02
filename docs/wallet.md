## WALLET: BSV Wallet Toolbox

Implements the BRC-100 wallet interface using the Go SDK, backed by the storage layer. Provides key ops (create/sign/internalize/abort), listing, basic auth checks, network info, and cryptographic primitives.

### Concepts

- Wallet wraps a `go-sdk` proto wallet plus a `WalletStorageManager` for persistence.
- Calls validate an `originator` string and translate arguments to storage types.
- Optional services enable network queries (height, headers) via `WithServices`.

### Quick start

```go
package main

import (
    "context"
    "log/slog"
    "os"

    sdk "github.com/bsv-blockchain/go-sdk/wallet"
    "github.com/bsv-blockchain/go-wallet-toolbox/pkg/defs"
    "github.com/bsv-blockchain/go-wallet-toolbox/pkg/storage"
    "github.com/bsv-blockchain/go-wallet-toolbox/pkg/wallet"
)

func main() {
    logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

    provider, err := storage.NewGORMProvider(
        defs.BSVNetworkMainnet,
        nil, // optional services; provide when using GetHeight/GetHeaderForHeight
        storage.WithLogger(logger),
        storage.WithDBConfig(defs.DefaultDBConfig()),
    )
    if err != nil { panic(err) }
    defer provider.Stop()

    ctx := context.Background()
    if _, err = provider.Migrate(ctx, "My Wallet Storage", "my-identity-key"); err != nil { panic(err) }
    if _, err = provider.MakeAvailable(ctx); err != nil { panic(err) }

    w, err := wallet.New(
        defs.BSVNetworkMainnet,
        wallet.WIF("<your WIF here>"), // or hex string, *ec.PrivateKey, *sdk.KeyDeriver
        provider,
        wallet.WithLogger(logger),
    )
    if err != nil { panic(err) }
    defer w.Close()

    const originator = "example.com"

    // Basic calls
    _, _ = w.GetVersion(ctx, nil, originator)
    _, _ = w.GetNetwork(ctx, nil, originator)
    _, _ = w.ListOutputs(ctx, sdk.ListOutputsArgs{}, originator)

    // Create then sign (skeleton)
    // createRes, _ := w.CreateAction(ctx, sdk.CreateActionArgs{ /* outputs, labels, options */ }, originator)
    // _, _ = w.SignAction(ctx, sdk.SignActionArgs{ Reference: createRes.SignableTransaction.Reference }, originator)
}
```

### Core operations

- Create/Sign: `CreateAction`, `SignAction`
- Internalize/Abort: `InternalizeAction`, `AbortAction`
- List: `ListActions`, `ListFailedActions(unfail)`, `ListOutputs`, `RelinquishOutput`
- Crypto: `GetPublicKey`, `CreateSignature`/`VerifySignature`, `Encrypt`/`Decrypt`, `CreateHMAC`/`VerifyHMAC`
- Keys linkage: `RevealCounterpartyKeyLinkage`, `RevealSpecificKeyLinkage`

### Options

- `WithIncludeAllSourceTransactions(bool)` default true
- `WithAutoKnownTxids(bool)` default false
- `WithTrustSelf(sdk.TrustSelf)` default `known`
- `WithServices(*services.WalletServices)` enables `GetHeight`/`GetHeaderForHeight`
- `WithPendingSignActionsRepository(repo)` cache for sign flow
- `WithLogger(*slog.Logger)` structured logging

### Utilities

- Network and version: `GetNetwork`, `GetVersion`
- Chain info (requires services): `GetHeight`, `GetHeaderForHeight`
- Auth stubs: `IsAuthenticated`, `WaitForAuthentication`
- Lifecycle: `Close`, `Destroy`

> **Note:** Certificate APIs (`AcquireCertificate`, `ListCertificates`, `ProveCertificate`, `RelinquishCertificate`, `Discover*`) are placeholders and not yet implemented.
