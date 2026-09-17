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

### Network values and compatibility

`GetNetwork` returns the BRC-100 values `mainnet` or `testnet`, using
`sdk.NetworkMainnet` and `sdk.NetworkTestnet` respectively:

| Configured internal chain | `GetNetwork().Network` |
|---|---|
| `main` | `mainnet` |
| `test` | `testnet` |
| `ttn` | `testnet` |
| `tstn` | `testnet` |

Earlier versions returned the internal chain identifier directly. Applications that
compare the result against `main`, `test`, `ttn`, or `tstn` must update those comparisons.
For Go callers, compare against the SDK network constants rather than casting `defs`
constants. JSON consumers must expect `mainnet`/`testnet`. This also corrects mainnet
responses through the SDK binary serializer, which treated the old `main` value as testnet.

Existing wallets require no database migration, key regeneration, or movement of funds.
Configuration and stored chain values remain `main`, `test`, `ttn`, and `tstn`; do not
rename them to the BRC-100 values. Do not cast a `GetNetwork` result back to
`defs.BSVNetwork` or pass it directly to `defs.ParseBSVNetworkStr`.

Keep the configured internal chain separately when selecting service endpoints or
distinguishing public testnet from TTN/TSTN. A `testnet` result indicates the network
family, not that two wallets necessarily use the same test blockchain.

### Certificates and identity

`AcquireCertificate` (both `issuance` and `direct` acquisition protocols),
`ListCertificates`, `ProveCertificate`, `RelinquishCertificate`, `DiscoverByIdentityKey`,
and `DiscoverByAttributes` are implemented in `pkg/wallet/wallet.go`. The discovery
methods query overlay services and cache results; they do not persist certificates.

> **Note:** The privileged keyring is not yet wired up. Methods accepting `privileged`
> derive through the standard key deriver regardless; see the TODOs in
> `pkg/wallet/wallet.go` referencing `PrivilegedKeyManager`.

### Transaction lifecycle

For the full UTXO lifecycle — repository calls per method, the two-phase reservation, the
release compensation machine, status vocabularies, and known parity gaps against the
TypeScript implementation — see [`docs/utxo-lifecycle.md`](./utxo-lifecycle.md).
